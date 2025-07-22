package fswebhook

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
	ICAO string `json:"icao"`
	Name string `json:"name"`
	Time string `json:"time"`
}

type Distance struct {
	NM int `json:"nm"`
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
