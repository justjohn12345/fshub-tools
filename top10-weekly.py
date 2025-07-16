# FSHub CLI Tool

import os
import requests
from datetime import datetime, timedelta, timezone

API_BASE_URL = "https://fshub.io/api/v3"

def get_auth_token():
    """Retrieves the FSHub API token from an environment variable."""
    token = os.environ.get("FSHUB_API_TOKEN")
    if not token:
        raise ValueError("FSHUB_API_TOKEN environment variable not set.")
    return token

def get_airline_pilots(token, airline_id):
    """Fetches all pilots for a given airline, handling pagination."""
    headers = {
        "X-Pilot-Token": token,
        "Content-Type": "application/json",
    }
    all_pilots = []
    params = {"limit": 100}
    url = f"{API_BASE_URL}/airline/{airline_id}/pilot"

    while True:
        response = requests.get(url, headers=headers, params=params)

        if response.status_code == 404:
            print("INFO: Received 404, considering it the end of data and returning collected pilots.")
            break

        response.raise_for_status()
        data = response.json()

        meta = data.get("meta", {})
        print(f"DEBUG: Fetched page with meta: {meta}")

        all_pilots.extend(data.get("data", []))
        
        next_cursor = data.get("meta", {}).get("cursor", {}).get("next")
        if not next_cursor:
            break
        params["cursor"] = next_cursor

    return all_pilots

def get_airline_flights(token, airline_id):
    """Fetches all flight data for a given airline, handling pagination."""
    headers = {
        "X-Pilot-Token": token,
        "Content-Type": "application/json",
    }
    all_flights = []
    params = {"limit": 100}
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
            # 1. Fetch all flights
            flights_data = get_airline_flights(auth_token, airline_id)
            print(f"\nFetched {len(flights_data)} total flights for Airline {airline_id}.")

            # 2. Filter flights from the last 7 days
            one_week_ago = datetime.now(timezone.utc) - timedelta(days=7)
            recent_flights = []
            for f in flights_data:
                departure_info = f.get("departure")
                if departure_info and departure_info.get("time"):
                    try:
                        departure_time = datetime.fromisoformat(departure_info["time"].replace("Z", "+00:00"))
                        if departure_time > one_week_ago:
                            recent_flights.append(f)
                    except ValueError:
                        print(f"WARNING: Skipping flight {f.get('id')} due to invalid time format: {departure_info.get('time')}")

            print(f"Found {len(recent_flights)} flights in the last 7 days.")

            # 3. Group flights by pilot
            pilot_flights = {}
            for flight in recent_flights:
                user_info = flight.get("user")
                if user_info and user_info.get("id"):
                    pilot_id = user_info["id"]
                    if pilot_id not in pilot_flights:
                        pilot_flights[pilot_id] = []
                    pilot_flights[pilot_id].append(flight)

            # 4. Calculate average landing rate and other stats for each pilot
            pilot_stats = []
            for pilot_id, flights in pilot_flights.items():
                if not flights:
                    continue
                
                # Filter out flights with no landing rate data
                flights_with_rate = [f for f in flights if f.get('landing_rate') is not None]

                # Check if the pilot has at least 10 flights to qualify
                if len(flights_with_rate) < 10:
                    continue

                total_landing_rate = sum(f['landing_rate'] for f in flights_with_rate)
                average_landing_rate = total_landing_rate / len(flights_with_rate)
                pilot_name = flights[0].get("user", {}).get("name", "Unknown")
                
                # Calculate additional stats
                total_flights = len(flights_with_rate)
                total_distance_nm = sum(f.get('distance', {}).get('nm', 0) for f in flights_with_rate)
                total_time_seconds = sum(f.get('time', 0) for f in flights_with_rate)
                total_hours_flown = total_time_seconds / 3600

                pilot_stats.append({
                    "name": pilot_name,
                    "id": pilot_id,
                    "average_landing_rate": average_landing_rate,
                    "total_flights": total_flights,
                    "total_distance_nm": total_distance_nm,
                    "total_hours_flown": total_hours_flown
                })

            # 5. Sort pilots by average landing rate (descending) and get top 10
            sorted_pilots = sorted(pilot_stats, key=lambda p: p['average_landing_rate'], reverse=True)
            top_10_pilots = sorted_pilots[:10]

            # 6. Display the report
            print("\n--- Top 10 Weekly Landing Rate Report (Highest to Lowest) ---")
            for pilot in top_10_pilots:
                print(f"Pilot: {pilot['name']} ({pilot['id']})\n"
                      f"  Avg. Landing Rate: {pilot['average_landing_rate']:.2f} fpm\n"
                      f"  Total Flights: {pilot['total_flights']}\n"
                      f"  Total Distance: {pilot['total_distance_nm']:.0f} nm\n"
                      f"  Total Hours: {pilot['total_hours_flown']:.2f} hrs\n")

    except (ValueError, requests.exceptions.RequestException) as e:
        print(f"Error: {e}")