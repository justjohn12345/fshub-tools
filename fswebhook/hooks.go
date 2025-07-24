package fswebhook

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func InitDB() {
	var err error
	db, err = sql.Open("sqlite3", "./fshub.db")
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	fmt.Println("Successfully connected to the database.")
}

type FlightCompletedEvent struct {
	Data FlightData `json:"_data"`
}

type FlightData struct {
	ID        int       `json:"id"`
	User      User      `json:"user"`
	Aircraft  Aircraft  `json:"aircraft"`
	Departure Departure `json:"departure"`
	Arrival   Arrival   `json:"arrival"`
	Distance  Distance  `json:"distance"`
	FuelBurnt float64   `json:"fuel_burnt"`
}

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Aircraft struct {
	ICAO string `json:"icao"`
	Name string `json:"name"`
}

type Departure struct {
	Airport  Airport `json:"airport"`
	DateTime string  `json:"datetime"`
}

type Airport struct {
	ICAO string `json:"icao"`
}

type Arrival struct {
	Airport     Airport `json:"airport"`
	LandingRate int     `json:"landing_rate"`
	DateTime    string  `json:"datetime"`
}

type Distance struct {
	NM int `json:"nm"`
}

func FlightCompletedHandler(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("WEBHOOK_SECRET")
	if secret != "" {
		if r.URL.Query().Get("secret") != secret {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	log.Println("Received flight completed event")

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	log.Printf("Received flight completed event: %s", bodyBytes)

	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var event FlightCompletedEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Error decoding request body", http.StatusBadRequest)
		return
	}

	flight := event.Data

	stmt, err := db.Prepare(`
		INSERT OR REPLACE INTO flights (
			flightid, pilotid, pilotname, landing_rate, distance, "time",
			aircraft_icao, aircraft_name, departure_icao, arrival_icao, fuel_used,
			departure_time, arrival_time
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		log.Printf("Error preparing statement: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	departureTime, _ := time.Parse(time.RFC3339, flight.Departure.DateTime)
	arrivalTime, _ := time.Parse(time.RFC3339, flight.Arrival.DateTime)
	duration := arrivalTime.Sub(departureTime).Seconds()

	_, err = stmt.Exec(
		flight.ID,
		flight.User.ID,
		flight.User.Name,
		flight.Arrival.LandingRate,
		flight.Distance.NM,
		duration,
		flight.Aircraft.ICAO,
		flight.Aircraft.Name,
		flight.Departure.Airport.ICAO,
		flight.Arrival.Airport.ICAO,
		flight.FuelBurnt,
		flight.Departure.DateTime,
		flight.Arrival.DateTime,
	)
	if err != nil {
		log.Printf("Error inserting flight data: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully inserted flight data for flight ID %d", flight.ID)
	w.WriteHeader(http.StatusOK)
}
