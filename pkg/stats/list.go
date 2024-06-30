package stats

import (
	"fmt"
	"math"

	"github.com/jylitalo/mystats/storage"
)

func List(db Storage, types, workouts []string, years []int, limit int) ([]string, [][]string, error) {
	o := []string{"year", "month", "day"}
	rows, err := db.QuerySummary(
		[]string{"year", "month", "day", "name", "distance", "elevation", "movingtime", "type", "workouttype", "stravaid"},
		storage.SummaryConditions{WorkoutTypes: workouts, Types: types, Years: years},
		&storage.Order{GroupBy: o, OrderBy: o, Limit: limit},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("query caused: %w", err)
	}
	defer rows.Close()
	results := [][]string{}
	for rows.Next() {
		var year, month, day, movingTime, stravaID int
		var distance, elevation float64
		var name, typeName, workoutType string
		err = rows.Scan(&year, &month, &day, &name, &distance, &elevation, &movingTime, &typeName, &workoutType, &stravaID)
		if err != nil {
			return nil, nil, err
		}
		results = append(results, []string{
			fmt.Sprintf("%2d.%2d.%d", day, month, year), name,
			fmt.Sprintf("%.0f", math.Round(distance/1000)), fmt.Sprintf("%.0f", elevation),
			fmt.Sprintf("%2d:%02d:%02d", movingTime/3600, movingTime/60%60, movingTime%60),
			typeName, workoutType, fmt.Sprintf("https://strava.com/activities/%d", stravaID),
		})
	}
	return []string{"Date", "Name", "Distance (km)", "Elevation (m)", "Time", "Type", "Workout Type", "Link"}, results, nil
}
