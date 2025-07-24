package fswebhook

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// PilotStats struct to hold aggregated pilot data
type PilotStats struct {
	PilotID            int     `json:"pilotid"`
	PilotName          string  `json:"pilotname"`
	AverageLandingRate float64 `json:"average_landing_rate"`
	TotalFlights       int     `json:"total_flights"`
	TotalDistance      int     `json:"total_distance_nm"`
	TotalHoursFlown    float64 `json:"total_hours_flown"`
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
