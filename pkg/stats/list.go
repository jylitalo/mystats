package stats

import (
	"fmt"
	"math"

	"github.com/jylitalo/mystats/storage"
)

func List(db Storage, types, workouts []string) ([]string, [][]string, error) {
	o := []string{"year", "month", "day"}
	rows, err := db.Query(
		[]string{"year", "month", "day", "name", "distance", "elevation", "movingtime"},
		storage.Conditions{Workouts: workouts, Types: types}, &storage.Order{GroupBy: o, OrderBy: o},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("query caused: %w", err)
	}
	defer rows.Close()
	results := [][]string{}
	for rows.Next() {
		var year, month, day, movingTime int
		var distance, elevation float64
		var name string
		err = rows.Scan(&year, &month, &day, &name, &distance, &elevation, &movingTime)
		if err != nil {
			return nil, nil, err
		}
		results = append(results, []string{
			fmt.Sprintf("%2d.%2d.%d", day, month, year), name,
			fmt.Sprintf("%.0f", math.Round(distance/1000)), fmt.Sprintf("%.0f", elevation),
			fmt.Sprintf("%2d:%02d:%02d", movingTime/3600, movingTime/60%60, movingTime%60),
		})
	}
	return []string{"Date", "Name", "Distance (km)", "Elevation (m)", "Time"}, results, nil
}
