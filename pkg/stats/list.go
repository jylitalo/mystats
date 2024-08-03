package stats

import (
	"fmt"
	"math"
	"strconv"

	"github.com/jylitalo/mystats/storage"
)

func List(db Storage, types, workouts []string, years []int, limit int, name string) ([]string, [][]string, error) {
	o := []string{"year", "month", "day"}
	rows, err := db.QuerySummary(
		[]string{"year", "month", "day", "name", "distance", "elevation", "elapsedtime", "type", "workouttype", "stravaid"},
		storage.SummaryConditions{WorkoutTypes: workouts, Types: types, Years: years, Name: name},
		&storage.Order{GroupBy: o, OrderBy: o, Limit: limit},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("query caused: %w", err)
	}
	defer rows.Close()
	results := [][]string{}
	for rows.Next() {
		var year, month, day, elapsedTime, stravaID int
		var distance, elevation float64
		var name, typeName, workoutType string
		err = rows.Scan(&year, &month, &day, &name, &distance, &elevation, &elapsedTime, &typeName, &workoutType, &stravaID)
		if err != nil {
			return nil, nil, err
		}
		results = append(results, []string{
			strconv.Itoa(stravaID),
			fmt.Sprintf("%2d.%2d.%d", day, month, year), name,
			fmt.Sprintf("%.1f", math.Round(distance/1000)), fmt.Sprintf("%.0f", elevation),
			fmt.Sprintf("%2d:%02d:%02d", elapsedTime/3600, elapsedTime/60%60, elapsedTime%60),
			typeName, workoutType, fmt.Sprintf("https://strava.com/activities/%d", stravaID),
		})
	}
	return []string{"ID", "Date", "Name", "Distance (km)", "Elevation (m)", "Time", "Type", "Workout Type", "Link"}, results, nil
}

func Split(db Storage, id int64) ([]string, [][]string, error) {
	var totalTime int
	var totalDistance, ascent, descent float64
	rows, err := db.QuerySplit([]string{"split", "elapsedtime", "distance", "elevationdiff"}, id)
	if err != nil {
		return nil, nil, fmt.Errorf("query caused: %w", err)
	}
	defer rows.Close()
	results := [][]string{}
	for rows.Next() {
		var split, elapsedTime int
		var distance, elevationDiff float64
		err = rows.Scan(&split, &elapsedTime, &distance, &elevationDiff)
		if err != nil {
			return nil, nil, err
		}
		totalTime += elapsedTime
		totalDistance += distance
		if elevationDiff < 0 {
			descent += -elevationDiff
		} else {
			ascent += elevationDiff
		}
		results = append(results, []string{
			strconv.Itoa(split),
			fmt.Sprintf("%02d:%02d", elapsedTime/60%60, elapsedTime%60),
			fmt.Sprintf("%.1f", math.Round(distance/1000)), fmt.Sprintf("%.0f", elevationDiff),
			fmt.Sprintf("%2d:%02d:%02d", totalTime/3600, totalTime/60%60, totalTime%60),
			fmt.Sprintf("%.1f", math.Round(totalDistance/1000)),
			fmt.Sprintf("%.0f", ascent), fmt.Sprintf("%.0f", descent),
		})
	}
	return []string{
		"Split", "Time", "Distance (km)", "Elevation (m)",
		"Total Time", "Total Distance (km)", "Total Ascent (m)", "Total Descent (m)",
	}, results, nil
}
