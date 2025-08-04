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
	// Set the webhook secret for the test
	os.Setenv("WEBHOOK_SECRET", "test-secret")
	defer os.Unsetenv("WEBHOOK_SECRET")

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
	req, err := http.NewRequest("POST", "/flight-completed?secret=test-secret", bytes.NewBuffer(jsonData))
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
		 fuel_used, departure_time, arrival_time FROM flights WHERE flightid = ?`, 3901328).Scan(
		&flightID, &pilotID, &pilotName, &landingRate,
		&distance, &flightTime, &aircraftICAO, &aircraftName, &departureICAO, &arrivalICAO,
		&fuelUsed, &departureTime, &arrivalTime)
	if err != nil {
		t.Fatalf("Failed to read row from database: %v", err)
	}

	if flightID != 3901328 {
		t.Errorf("expected flightID to be 3901328, got %d", flightID)
	}
	if pilotID != 25104 {
		t.Errorf("expected pilotID to be 25104, got %d", pilotID)
	}
	if pilotName != "Inode" {
		t.Errorf("expected pilotName to be 'Inode', got '%s'", pilotName)
	}
	if landingRate != -196 {
		t.Errorf("expected landingRate to be -196, got %d", landingRate)
	}
	if distance != 352 {
		t.Errorf("expected distance to be 352, got %d", distance)
	}
	if flightTime != 3469 {
		t.Errorf("expected flightTime to be 3469, got %d", flightTime)
	}
	if aircraftICAO != "B38M" {
		t.Errorf("expected aircraftICAO to be 'B38M', got '%s'", aircraftICAO)
	}
	if aircraftName != "737 Max 8 Passengers" {
		t.Errorf("expected aircraftName to be '737 Max 8 Passengers', got '%s'", aircraftName)
	}
	if departureICAO != "KMYR" {
		t.Errorf("expected departureICAO to be 'KMYR', got '%s'", departureICAO)
	}
	if arrivalICAO != "KATL" {
		t.Errorf("expected arrivalICAO to be 'KATL', got '%s'", arrivalICAO)
	}
	if fuelUsed != 2390 {
		t.Errorf("expected fuelUsed to be 2390, got %f", fuelUsed)
	}
	if departureTime != "2025-07-24T21:32:49Z" {
		t.Errorf("expected departureTime to be '2025-07-24T21:32:49Z', got '%s'", departureTime)
	}
	if arrivalTime != "2025-07-24T22:30:38Z" {
		t.Errorf("expected arrivalTime to be '2025-07-24T22:30:38Z', got '%s'", arrivalTime)
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
	if event.Data.ID != 3901328 {
		t.Errorf("expected flight ID to be 3901328, got '%d'", event.Data.ID)
	}
	if event.Data.User.Name != "Inode" {
		t.Errorf("expected user name to be 'Inode', got '%s'", event.Data.User.Name)
	}
	if event.Data.Aircraft.ICAO != "B38M" {
		t.Errorf("expected aircraft ICAO to be 'B38M', got '%s'", event.Data.Aircraft.ICAO)
	}
	if event.Data.Departure.Airport.ICAO != "KMYR" {
		t.Errorf("expected departure ICAO to be 'KMYR', got '%s'", event.Data.Departure.Airport.ICAO)
	}
	if event.Data.Arrival.Airport.ICAO != "KATL" {
		t.Errorf("expected arrival ICAO to be 'KATL', got '%s'", event.Data.Arrival.Airport.ICAO)
	}
	if event.Data.Arrival.LandingRate != -196 {
		t.Errorf("expected landing rate to be -196, got '%d'", event.Data.Arrival.LandingRate)
	}
}

func TestFlightCompletedHandler_EdgeCases(t *testing.T) {
	os.Setenv("WEBHOOK_SECRET", "test-secret")
	defer os.Unsetenv("WEBHOOK_SECRET")

	InitDB()

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

	_, err = db.Exec(`DELETE FROM flights`)
	if err != nil {
		t.Fatalf("Failed to clear flights table: %v", err)
	}

	baseJSON, err := os.ReadFile(filepath.Join("testdata", "flight.completed.example.json"))
	if err != nil {
		t.Fatalf("Failed to read example JSON file: %v", err)
	}

	var baseEvent FlightCompletedEvent
	if err := json.Unmarshal(baseJSON, &baseEvent); err != nil {
		t.Fatalf("Failed to unmarshal base JSON: %v", err)
	}

	testCases := []struct {
		name          string
		modifier      func(event *FlightCompletedEvent)
		expectInDB    bool
		expectedCount int
	}{
		{
			name: "empty departure icao",
			modifier: func(event *FlightCompletedEvent) {
				event.Data.Departure.Airport.ICAO = ""
			},
			expectInDB: false,
		},
		{
			name: "empty arrival icao",
			modifier: func(event *FlightCompletedEvent) {
				event.Data.Arrival.Airport.ICAO = ""
			},
			expectInDB: false,
		},
		{
			name: "duration less than 5 minutes",
			modifier: func(event *FlightCompletedEvent) {
				event.Data.Arrival.DateTime = event.Data.Departure.DateTime
			},
			expectInDB: false,
		},
		{
			name: "invalid arrival time",
			modifier: func(event *FlightCompletedEvent) {
				event.Data.Arrival.DateTime = "invalid-time"
			},
			expectInDB: false,
		},
		{
			name: "invalid departure time",
			modifier: func(event *FlightCompletedEvent) {
				event.Data.Departure.DateTime = "invalid-time"
			},
			expectInDB: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event := baseEvent
			tc.modifier(&event)

			body, err := json.Marshal(event)
			if err != nil {
				t.Fatalf("Failed to marshal test case JSON: %v", err)
			}

			req, err := http.NewRequest("POST", "/flight-completed?secret=test-secret", bytes.NewBuffer(body))
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(FlightCompletedHandler)
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != http.StatusOK {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, http.StatusOK)
			}

			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM flights WHERE flightid = ?", event.Data.ID).Scan(&count)
			if err != nil {
				t.Fatalf("Failed to query database: %v", err)
			}

			if tc.expectInDB && count == 0 {
				t.Errorf("expected flight to be in database, but it wasn't")
			}
			if !tc.expectInDB && count > 0 {
				t.Errorf("expected flight not to be in database, but it was")
			}
			// Clear the table for the next test
			_, err = db.Exec(`DELETE FROM flights`)
			if err != nil {
				t.Fatalf("Failed to clear flights table: %v", err)
			}
		})
	}
}
