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
		ts DATETIME,
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
