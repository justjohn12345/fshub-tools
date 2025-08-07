package fswebhook

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// PilotFlightDetails struct to hold pilot's landing rate
type PilotFlightDetails struct {
	PilotName    string  `json:"pilot_name"`
	LandingRate  float64 `json:"landing_rate"`
	AircraftName string  `json:"aircraft_name"`
	Rank         int     `json:"rank"` // Rank based on landing rate
}

// GroupFlight struct to hold data for a group flight event
type GroupFlight struct {
	DepartureICAO   string               `json:"departure_icao"`
	ArrivalICAO     string               `json:"arrival_icao"`
	FlightCount     int                  `json:"flight_count"`
	StartTime       time.Time            `json:"start_time"`
	TotalPilots     int                  `json:"total_pilots"`
	TopLandingRates []PilotFlightDetails `json:"top_landing_rates"`
}

// Intermediate struct to scan results from the DB
type groupFlightScanResult struct {
	DepartureICAO string
	ArrivalICAO   string
	ArrivalTime   string
	LandingRate   float64
	AircraftName  string
	PilotName     string
	FlightID      string
	FlightNumber  int
	Rank          int
	TotalPilots   int
}

const flightLeaderDefault = "KipOnTheGround"

func GroupFlightHandler(w http.ResponseWriter, r *http.Request) {

	pilotName := flightLeaderDefault // Default leader
	sinceTime := time.Now().UTC().Add(-24 * time.Hour)

	query := `
	select * from (
		select 
			f.departure_icao, 
			f.arrival_icao, 
			f.arrival_time,
			f.landing_rate, 
			f.aircraft_name,
			f.pilotname,
			lf.flightid,
			lf.flight_number,
			row_number() OVER (PARTITION BY lf.flightid ORDER BY f.landing_rate desc) AS rn,
            count(*) OVER (PARTITION BY lf.flightid) AS totalPilots
		FROM flights AS f
		JOIN ( 
			SELECT departure_icao, 
					arrival_icao, 
					arrival_time,
					landing_rate, 
					aircraft_name,
					flightid,
					row_number() OVER () AS flight_number
			FROM flights
			WHERE pilotname = ? 
				AND arrival_time >= ?
			ORDER BY arrival_time DESC) AS lf 
		ON f.departure_icao = lf.departure_icao
		AND f.arrival_icao = lf.arrival_icao
		AND datetime(f.arrival_time) >= datetime(lf.arrival_time, '-30 minutes')
		AND datetime(f.arrival_time) <= datetime(lf.arrival_time, '+30 minutes')
	)
	where rn <= 5 OR pilotname = ?`

	rows, err := db.Query(query, pilotName, sinceTime.Format(time.RFC3339), pilotName)
	if err != nil {
		log.Printf("Error querying for group flights: %v", err)
		http.Error(w, "Error querying for group flights", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	flightNumber := -1

	var allGroupFlights []GroupFlight
	var currentGroupFlight GroupFlight

	for rows.Next() {
		var res groupFlightScanResult
		if err := rows.Scan(
			&res.DepartureICAO,
			&res.ArrivalICAO,
			&res.ArrivalTime,
			&res.LandingRate,
			&res.AircraftName,
			&res.PilotName,
			&res.FlightID,
			&res.FlightNumber,
			&res.Rank,
			&res.TotalPilots,
		); err != nil {
			log.Printf("Error scanning group flight row: %v", err)
			continue
		}

		if res.FlightNumber != flightNumber {
			if flightNumber != -1 {
				// Not the first iteration, append the completed group flight
				allGroupFlights = append(allGroupFlights, currentGroupFlight)
			}
			// Reset flight number to the current one
			flightNumber = res.FlightNumber
			currentGroupFlight = GroupFlight{
				DepartureICAO:   res.DepartureICAO,
				ArrivalICAO:     res.ArrivalICAO,
				TotalPilots:     res.TotalPilots,
				TopLandingRates: []PilotFlightDetails{},
			}

		}

		if res.PilotName == flightLeaderDefault {
			// If the pilot is the flight leader, set the start time to the parsed time
			parsedTime, err := time.Parse(time.RFC3339, res.ArrivalTime)
			if err != nil {
				log.Printf("Error parsing arrival time for group flight: %v", err)
				continue
			}
			currentGroupFlight.StartTime = parsedTime
		}

		// Add to top landing rates if they are in the top 5
		pfd := PilotFlightDetails{
			PilotName:    res.PilotName,
			LandingRate:  res.LandingRate,
			AircraftName: res.AircraftName,
			Rank:         res.Rank,
		}
		currentGroupFlight.TopLandingRates = append(currentGroupFlight.TopLandingRates, pfd)
	}

	if len(currentGroupFlight.TopLandingRates) > 0 {
		allGroupFlights = append(allGroupFlights, currentGroupFlight)
	}

	fmt.Println(allGroupFlights)
	if len(allGroupFlights) > 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(allGroupFlights)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}
