import os
import requests
import sqlite3
from datetime import datetime, timezone

API_BASE_URL = "https://fshub.io/api/v3"
DB_FILE = "fshub.db"

def get_db_connection():
    """Establishes a connection to the SQLite database."""
    conn = sqlite3.connect(DB_FILE)
    conn.row_factory = sqlite3.Row
    return conn

def get_max_flight_id(conn):
    """Gets the maximum flight ID from the database."""
    cursor = conn.cursor()
    cursor.execute("SELECT MAX(flightid) FROM flights")
    max_id = cursor.fetchone()[0]
    return max_id if max_id else 0

def validate_flight_data(flight):
    """Validates the presence and basic type of required flight data fields."""
    required_fields = {
        "id": int,
        "user": dict,
        "landing_rate": (int, float),
        "distance": dict,
        "time": int,
        "aircraft": dict,
        "departure": dict,
        "arrival": dict,
        "fuel_used": (int, float),
    }
    
    for field, expected_type in required_fields.items():
        if field not in flight or not isinstance(flight[field], expected_type):
            return False, f"Missing or invalid type for field: {field}"

    # Nested validations
    if "id" not in flight.get("user", {}) or not isinstance(flight["user"]["id"], int):
        return False, "Missing or invalid user ID"
    if "name" not in flight.get("user", {}) or not isinstance(flight["user"]["name"], str):
        return False, "Missing or invalid user name"
    if "nm" not in flight.get("distance", {}) or not isinstance(flight["distance"]["nm"], (int, float)):
        return False, "Missing or invalid distance in nautical miles"
    if "icao" not in flight.get("aircraft", {}) or not isinstance(flight["aircraft"]["icao"], str):
        return False, "Missing or invalid aircraft ICAO"
    if "time" not in flight.get("departure", {}) or not isinstance(flight["departure"]["time"], str):
        return False, "Missing or invalid departure time"
    if "time" not in flight.get("arrival", {}) or not isinstance(flight["arrival"]["time"], str):
        return False, "Missing or invalid arrival time"

    return True, ""

def insert_flight_data(conn, flight):
    """Inserts or updates a flight record in the database after validation."""
    is_valid, error_message = validate_flight_data(flight)
    if not is_valid:
        print(f"Skipping invalid flight data: {error_message} (Flight ID: {flight.get('id')})")
        return

    cursor = conn.cursor()
    
    # Check if flight already exists
    cursor.execute("SELECT 1 FROM flights WHERE flightid = ?", (flight['id'],))
    if cursor.fetchone():
        print(f"Skipping duplicate flight: {flight['id']})")
        return

    # Prepare data for insertion
    data_to_insert = {
        "flightid": flight.get("id"),
        "pilotid": flight.get("user", {}).get("id"),
        "pilotname": flight.get("user", {}).get("name"),
        "landing_rate": flight.get("landing_rate"),
        "ts": flight.get("departure", {}).get("time"),
        "distance": flight.get("distance", {}).get("nm"),
        "time": flight.get("time"),
        "aircraft_icao": flight.get("aircraft", {}).get("icao"),
        "aircraft_name": flight.get("aircraft", {}).get("name"),
        "departure_icao": flight.get("departure", {}).get("icao"),
        "arrival_icao": flight.get("arrival", {}).get("icao"),
        "fuel_used": flight.get("fuel_used"),
        "departure_time": flight.get("departure", {}).get("time"),
        "arrival_time": flight.get("arrival", {}).get("time"),
    }
    
    columns = ', '.join(data_to_insert.keys())
    placeholders = ', '.join('?' for _ in data_to_insert)
    sql = f"INSERT INTO flights ({columns}) VALUES ({placeholders})"
    
    cursor.execute(sql, tuple(data_to_insert.values()))
    print(f"Inserted flight: {flight['id']}")

def get_auth_token():
    """Retrieves the FSHub API token from an environment variable."""
    token = os.environ.get("FSHUB_API_TOKEN")
    if not token:
        raise ValueError("FSHUB_API_TOKEN environment variable not set.")
    return token

def get_airline_flights(token, airline_id, cursor_id=None):
    """Fetches all flight data for a given airline, handling pagination."""
    headers = {
        "X-Pilot-Token": token,
        "Content-Type": "application/json",
    }
    all_flights = []
    params = {"limit": 100}
    if cursor_id:
        params["cursor"] = cursor_id

    url = f"{API_BASE_URL}/airline/{airline_id}/flight"

    while True:
        response = requests.get(url, headers=headers, params=params)

        if response.status_code == 404:
            print("INFO: Received 404 on flights endpoint, considering it the end of data.")
            break

        response.raise_for_status()
        data = response.json()
        
        meta = data.get("meta", {})
        print(f"DEBUG: Fetched flight page with meta: {meta}")

        all_flights.extend(data.get("data", []))
        
        next_cursor = meta.get("cursor", {}).get("next")
        if not next_cursor:
            break
        params["cursor"] = next_cursor

    return all_flights

if __name__ == "__main__":
    try:
        auth_token = get_auth_token()
        print("Successfully retrieved API token.")
        
        airline_id = "6076"
        if airline_id:
            # 1. Establish DB connection
            conn = get_db_connection()
            
            # 2. Get the max flight ID from the database
            max_flight_id = get_max_flight_id(conn)
            print(f"Starting flight import from ID: {max_flight_id}")

            # 3. Fetch all flights
            flights_data = get_airline_flights(auth_token, airline_id, max_flight_id)
            print(f"\nFetched {len(flights_data)} total flights for Airline {airline_id}.")

            # 4. Insert flights into the database
            for flight in flights_data:
                insert_flight_data(conn, flight)
            
            conn.commit()
            conn.close()
            print("\nFlight data successfully saved to the database.")

    except (ValueError, requests.exceptions.RequestException, sqlite3.Error) as e:
        print(f"Error: {e}")