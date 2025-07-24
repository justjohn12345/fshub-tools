package fswebhook

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"time"
)

// GroupFlight struct to hold data for a group flight event
// PilotLandingRate struct to hold pilot's landing rate
type PilotFlightDetails struct {
	PilotName    string  `json:"pilot_name"`
	LandingRate  float64 `json:"landing_rate"`
	AircraftName string  `json:"aircraft_name"`
}

type GroupFlight struct {
	DepartureICAO   string               `json:"departure_icao"`
	ArrivalICAO     string               `json:"arrival_icao"`
	Pilots          []string             `json:"pilots"`
	FlightCount     int                  `json:"flight_count"`
	StartTime       time.Time            `json:"start_time"`
	TopLandingRates []PilotFlightDetails `json:"top_landing_rates"`
}

func GroupFlightHandler(w http.ResponseWriter, r *http.Request) {
	leader := "KipOnTheGround"
	twentyFourHoursAgo := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)

	// 1. Find all of the leader's flights in the last 24 hours
	leaderFlightsQuery := `
        SELECT departure_icao, arrival_icao, arrival_time, landing_rate, aircraft_name
        FROM flights
        WHERE pilotname = ? AND arrival_time >= ?
        ORDER BY arrival_time DESC;
    `
	rows, err := db.Query(leaderFlightsQuery, leader, twentyFourHoursAgo)
	if err != nil {
		http.Error(w, "Error querying leader's flights", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type LeaderFlight struct {
		DepartureICAO string
		ArrivalICAO   string
		ArrivalTime   time.Time
		LandingRate   float64
		AircraftName  string
	}
	var leaderFlights []LeaderFlight

	for rows.Next() {
		var lf LeaderFlight
		var arrivalTimeStr string
		if err := rows.Scan(&lf.DepartureICAO, &lf.ArrivalICAO, &arrivalTimeStr, &lf.LandingRate, &lf.AircraftName); err != nil {
			log.Printf("Error scanning leader flight: %v", err)
			continue
		}
		lf.ArrivalTime, err = time.Parse(time.RFC3339, arrivalTimeStr)
		if err != nil {
			log.Printf("Error parsing arrival time for leader flight: %v", err)
			continue
		}
		leaderFlights = append(leaderFlights, lf)
	}

	var allGroupFlights []GroupFlight

	// 2. For each of the leader's flights, find other flights within the time window
	for _, lf := range leaderFlights {
		timeWindowStart := lf.ArrivalTime.Add(-30 * time.Minute)
		timeWindowEnd := lf.ArrivalTime.Add(30 * time.Minute)

		groupQuery := `
		    SELECT pilotname, landing_rate, aircraft_name
		    FROM flights
		    WHERE departure_icao = ? AND arrival_icao = ? AND arrival_time BETWEEN ? AND ?;
		`
		// Use the time window to find other flights in the group
		groupRows, err := db.Query(groupQuery,
			lf.DepartureICAO, lf.ArrivalICAO,
			//timeWindowStart.UTC(), timeWindowEnd.UTC()
			timeWindowStart.Format(time.RFC3339), timeWindowEnd.Format(time.RFC3339))
		if err != nil {
			log.Printf("Error querying for group flights: %v", err)
			continue
		}
		defer groupRows.Close()

		var pilots []string
		var allFlightDetails []PilotFlightDetails
		var leaderInGroup bool

		for groupRows.Next() {
			var pfd PilotFlightDetails
			if err := groupRows.Scan(&pfd.PilotName, &pfd.LandingRate, &pfd.AircraftName); err != nil {
				log.Printf("Error scanning group flight pilot: %v", err)
				continue
			}
			pilots = append(pilots, pfd.PilotName)
			allFlightDetails = append(allFlightDetails, pfd)
			if pfd.PilotName == leader {
				leaderInGroup = true
			}
		}

		if len(allFlightDetails) < 5 {
			// If not enough pilots, we can skip this group flight
			continue
		}

		if !leaderInGroup {
			// This case should ideally not happen based on the query logic, but as a safeguard
			pilots = append(pilots, leader)
			allFlightDetails = append(allFlightDetails, PilotFlightDetails{PilotName: leader, LandingRate: lf.LandingRate, AircraftName: lf.AircraftName})
		}

		// 3. Produce a top 5 landing rate, ensuring the leader is included
		sort.Slice(allFlightDetails, func(i, j int) bool {
			return allFlightDetails[i].LandingRate > allFlightDetails[j].LandingRate
		})

		var topLandingRates []PilotFlightDetails
		var leaderInTop5 bool
		for i, pfd := range allFlightDetails {
			if i < 5 {
				topLandingRates = append(topLandingRates, pfd)
				if pfd.PilotName == leader {
					leaderInTop5 = true
				}
			} else {
				break
			}
		}

		if !leaderInTop5 {
			// Find the leader's landing rate and add it
			for _, pfd := range allFlightDetails {
				if pfd.PilotName == leader {
					topLandingRates = append(topLandingRates, pfd)
					break
				}
			}
		}

		groupFlight := GroupFlight{
			DepartureICAO:   lf.DepartureICAO,
			ArrivalICAO:     lf.ArrivalICAO,
			Pilots:          pilots,
			FlightCount:     len(pilots),
			StartTime:       lf.ArrivalTime,
			TopLandingRates: topLandingRates,
		}
		allGroupFlights = append(allGroupFlights, groupFlight)
	}

	if len(allGroupFlights) > 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(allGroupFlights)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}
