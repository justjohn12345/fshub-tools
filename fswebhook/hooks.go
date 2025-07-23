package fswebhook

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

// PilotStats struct to hold aggregated pilot data
type PilotStats struct {
	PilotID            int     `json:"pilotid"`
	PilotName          string  `json:"pilotname"`
	AverageLandingRate float64 `json:"average_landing_rate"`
	TotalFlights       int     `json:"total_flights"`
	TotalDistance      int     `json:"total_distance_nm"`
	TotalHoursFlown    float64 `json:"total_hours_flown"`
}

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

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "hello world")
}

// getWeeklyDateRange calculates the start and end dates for the weekly report.
func getWeeklyDateRanges(numWeeks int) [][2]time.Time {
	var dateRanges [][2]time.Time
	now := time.Now().UTC()

	// Find the most recent Saturday (end of the current week)
	endOfWeek := now
	for endOfWeek.Weekday() != time.Saturday {
		endOfWeek = endOfWeek.AddDate(0, 0, -1)
	}
	endOfWeek = time.Date(endOfWeek.Year(), endOfWeek.Month(), endOfWeek.Day(), 0, 0, 0, 0, time.UTC)

	for i := 0; i < numWeeks; i++ {
		currentEnd := endOfWeek.AddDate(0, 0, -7*i)
		currentStart := currentEnd.AddDate(0, 0, -7)
		dateRanges = append(dateRanges, [2]time.Time{currentStart, currentEnd})
	}

	return dateRanges
}

// WeeklyReport struct to hold data for each week, categorized
type WeeklyReport struct {
	StartDate      time.Time    `json:"start_date"`
	EndDate        time.Time    `json:"end_date"`
	TopLandingRate []PilotStats `json:"top_landing_rate"`
	TopDistance    []PilotStats `json:"top_distance"`
	TopFlights     []PilotStats `json:"top_flights"`
	TopHours       []PilotStats `json:"top_hours"`
}

// getTopPilots is a helper function to query the database for top pilots based on a specific ordering.
func getTopPilots(start, end time.Time, orderBy string) ([]PilotStats, error) {
	baseQuery := `
		SELECT
			pilotname,
			pilotid,
			AVG(landing_rate) AS avg_landing_rate,
			COUNT(flightid) AS total_flights,
			SUM(distance) AS total_distance,
			SUM(time) / 3600.0 AS total_hours
		FROM flights
		WHERE ts >= ? AND ts < ?
		GROUP BY pilotid, pilotname
		HAVING COUNT(flightid) >= 10
	`
	query := fmt.Sprintf("%s ORDER BY %s LIMIT 10", baseQuery, orderBy)

	rows, err := db.Query(query, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []PilotStats
	for rows.Next() {
		var ps PilotStats
		err := rows.Scan(&ps.PilotName, &ps.PilotID, &ps.AverageLandingRate, &ps.TotalFlights, &ps.TotalDistance, &ps.TotalHoursFlown)
		if err != nil {
			log.Printf("Error scanning pilot stats: %v", err)
			return nil, err
		}
		stats = append(stats, ps)
	}
	return stats, nil
}

// FlightsHandler calculates and returns categorized top 10 pilot reports for the last few weeks.
func FlightsHandler(w http.ResponseWriter, r *http.Request) {
	weeklyReports := []WeeklyReport{}
	dateRanges := getWeeklyDateRanges(3) // Get data for the last 3 weeks

	for _, dr := range dateRanges {
		start, end := dr[0], dr[1]

		topLandingRate, err := getTopPilots(start, end, "avg_landing_rate DESC")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		topDistance, err := getTopPilots(start, end, "total_distance DESC")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		topFlights, err := getTopPilots(start, end, "total_flights DESC")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		topHours, err := getTopPilots(start, end, "total_hours DESC")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		weeklyReports = append(weeklyReports, WeeklyReport{
			StartDate:      start,
			EndDate:        end,
			TopLandingRate: topLandingRate,
			TopDistance:    topDistance,
			TopFlights:     topFlights,
			TopHours:       topHours,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(weeklyReports)
}

type FlightCompletedWebhook struct {
	Data []FlightData `json:"data"`
}

type FlightData struct {
	ID          int      `json:"id"`
	User        User     `json:"user"`
	Airline     Airline  `json:"airline"`
	Aircraft    Aircraft `json:"aircraft"`
	Departure   Airport  `json:"departure"`
	Arrival     Airport  `json:"arrival"`
	Distance    Distance `json:"distance"`
	Time        int      `json:"time"`
	FuelUsed    int      `json:"fuel_used"`
	LandingRate int      `json:"landing_rate"`
}

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Airline struct {
	ID int `json:"id"`
}

type Aircraft struct {
	ICAO string `json:"icao"`
	Name string `json:"name"`
}

type Airport struct {
	ICAO string    `json:"icao"`
	Name string    `json:"name"`
	Time time.Time `json:"time"`
}

type Distance struct {
	NM int `json:"nm"`
}

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

func FlightCompletedHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Read the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	// Restore the body so it can be read again
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	log.Printf("Received FlightCompleted request: %s", string(body))

	var webhook FlightCompletedWebhook
	if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
		http.Error(w, "Error decoding request body", http.StatusBadRequest)
		return
	}

	for _, flight := range webhook.Data {
		if flight.Airline.ID != 6076 {
			continue
		}

		stmt, err := db.Prepare(`
			INSERT INTO flights (
				flightid, pilotid, pilotname, landing_rate, ts, distance, "time",
				aircraft_icao, aircraft_name, departure_icao, arrival_icao, fuel_used,
				departure_time, arrival_time
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			log.Printf("Error preparing statement: %v", err)
			continue
		}
		defer stmt.Close()

		_, err = stmt.Exec(
			flight.ID,
			flight.User.ID,
			flight.User.Name,
			flight.LandingRate,
			time.Now().UTC(),
			flight.Distance.NM,
			flight.Time,
			flight.Aircraft.ICAO,
			flight.Aircraft.Name,
			flight.Departure.ICAO,
			flight.Arrival.ICAO,
			flight.FuelUsed,
			flight.Departure.Time,
			flight.Arrival.Time,
		)
		if err != nil {
			log.Printf("Error inserting flight data: %v", err)
		} else {
			log.Printf("Successfully inserted flight data for flight ID %d", flight.ID)
		}
	}

	w.WriteHeader(http.StatusOK)
}
