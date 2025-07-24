package fswebhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFlightCompletedHandler(t *testing.T) {
	// Initialize the database for testing
	InitDB()

	// Create the flights table for testing
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS flights (
		flightid INTEGER PRIMARY KEY,
		pilotid INTEGER,
		pilotname TEXT,
		landing_rate INTEGER,
		distance INTEGER,
		"time" INTEGER,
		aircraft_icao TEXT,
		aircraft_name TEXT,
		departure_icao TEXT,
		arrival_icao TEXT,
		fuel_used INTEGER,
		departure_time DATETIME,
		arrival_time DATETIME
	);
	`)
	if err != nil {
		t.Fatalf("Failed to create flights table: %v", err)
	}

	// Clear the flights table before each test
	_, err = db.Exec(`DELETE FROM flights`)
	if err != nil {
		t.Fatalf("Failed to clear flights table: %v", err)
	}

	// Read the example JSON data from the file
	jsonData, err := os.ReadFile(filepath.Join("testdata", "flight.completed.example.json"))
	if err != nil {
		t.Fatalf("Failed to read example JSON file: %v", err)
	}

	// Create a request with the example JSON data
	req, err := http.NewRequest("POST", "/flight-completed", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(FlightCompletedHandler)

	// Serve the HTTP request
	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Verify the data was written to the database correctly
	var (
		flightID      int
		pilotID       int
		pilotName     string
		landingRate   int
		distance      int
		flightTime    int
		aircraftICAO  string
		aircraftName  string
		departureICAO string
		arrivalICAO   string
		fuelUsed      float64
		departureTime string
		arrivalTime   string
	)

	err = db.QueryRow(`SELECT flightid, pilotid, pilotname, landing_rate,
		 distance, time, aircraft_icao, aircraft_name, departure_icao, arrival_icao, 
		 fuel_used, departure_time, arrival_time FROM flights WHERE flightid = ?`, 1796723).Scan(
		&flightID, &pilotID, &pilotName, &landingRate,
		&distance, &flightTime, &aircraftICAO, &aircraftName, &departureICAO, &arrivalICAO,
		&fuelUsed, &departureTime, &arrivalTime)
	if err != nil {
		t.Fatalf("Failed to read row from database: %v", err)
	}

	if flightID != 1796723 {
		t.Errorf("expected flightID to be 1796723, got %d", flightID)
	}
	if pilotID != 2 {
		t.Errorf("expected pilotID to be 2, got %d", pilotID)
	}
	if pilotName != "Bobby Allen" {
		t.Errorf("expected pilotName to be 'Bobby Allen', got '%s'", pilotName)
	}
	if landingRate != -125 {
		t.Errorf("expected landingRate to be -125, got %d", landingRate)
	}
	if distance != 322 {
		t.Errorf("expected distance to be 322, got %d", distance)
	}
	if flightTime != 3857 {
		t.Errorf("expected flightTime to be 3857, got %d", flightTime)
	}
	if aircraftICAO != "A20N" {
		t.Errorf("expected aircraftICAO to be 'A20N', got '%s'", aircraftICAO)
	}
	if aircraftName != "British Airways Dirty Op" {
		t.Errorf("expected aircraftName to be 'British Airways Dirty Op', got '%s'", aircraftName)
	}
	if departureICAO != "EGPH" {
		t.Errorf("expected departureICAO to be 'EGPH', got '%s'", departureICAO)
	}
	if arrivalICAO != "EGLL" {
		t.Errorf("expected arrivalICAO to be 'EGLL', got '%s'", arrivalICAO)
	}
	if fuelUsed != 0 {
		t.Errorf("expected fuelUsed to be 0, got %f", fuelUsed)
	}
	if departureTime != "2022-02-23T11:39:10Z" {
		t.Errorf("expected departureTime to be '2022-02-23T11:39:10Z', got '%s'", departureTime)
	}
	if arrivalTime != "2022-02-23T12:43:27Z" {
		t.Errorf("expected arrivalTime to be '2022-02-23T12:43:27Z', got '%s'", arrivalTime)
	}
}

func TestUnmarshalFlightCompletedEvent(t *testing.T) {
	// Read the example JSON data from the file
	jsonData, err := os.ReadFile(filepath.Join("testdata", "flight.completed.example.json"))
	if err != nil {
		t.Fatalf("Failed to read example JSON file: %v", err)
	}

	// Attempt to unmarshal the JSON data into our struct
	var event FlightCompletedEvent
	err = json.Unmarshal(jsonData, &event)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Perform some basic checks to ensure the data was unmarshalled correctly
	if event.Data.ID != 1796723 {
		t.Errorf("expected flight ID to be 1796723, got '%d'", event.Data.ID)
	}
	if event.Data.User.Name != "Bobby Allen" {
		t.Errorf("expected user name to be 'Bobby Allen', got '%s'", event.Data.User.Name)
	}
	if event.Data.Aircraft.ICAO != "A20N" {
		t.Errorf("expected aircraft ICAO to be 'A20N', got '%s'", event.Data.Aircraft.ICAO)
	}
	if event.Data.Departure.Airport.ICAO != "EGPH" {
		t.Errorf("expected departure ICAO to be 'EGPH', got '%s'", event.Data.Departure.Airport.ICAO)
	}
	if event.Data.Departure.Arrival.Airport.ICAO != "EGLL" {
		t.Errorf("expected arrival ICAO to be 'EGLL', got '%s'", event.Data.Departure.Arrival.Airport.ICAO)
	}
	if event.Data.Departure.Arrival.LandingRate != -125 {
		t.Errorf("expected landing rate to be -125, got '%d'", event.Data.Departure.Arrival.LandingRate)
	}
}
