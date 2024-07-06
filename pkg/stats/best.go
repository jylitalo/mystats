package stats

import (
	"fmt"

	"github.com/jylitalo/mystats/storage"
)

func Best(db Storage, distance string, limit int) ([]string, [][]string, error) {
	o := []string{"besteffort.movingtime", "year", "month", "day"}
	rows, err := db.QueryBestEffort(
		[]string{
			"year", "month", "day", "summary.name", "summary.distance", "summary.elapsedtime",
			"besteffort.movingtime", "besteffort.elapsedtime",
			"summary.StravaID",
		},
		distance, &storage.Order{OrderBy: o, Limit: limit},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("query caused: %w", err)
	}
	defer rows.Close()
	results := [][]string{}
	for rows.Next() {
		var year, month, day, movingTime, elapsedTime, totalTime, stravaID int
		var distance float64
		var name string
		err = rows.Scan(&year, &month, &day, &name, &distance, &totalTime, &movingTime, &elapsedTime, &stravaID)
		if err != nil {
			return nil, nil, err
		}
		results = append(results, []string{
			fmt.Sprintf("%2d.%2d.%d", day, month, year), name,
			fmt.Sprintf("%2d:%02d:%02d", elapsedTime/3600, elapsedTime/60%60, elapsedTime%60),
			fmt.Sprintf("%.2f", distance/1000),
			fmt.Sprintf("%2d:%02d:%02d", totalTime/3600, totalTime/60%60, totalTime%60),
			fmt.Sprintf("https://strava.com/activities/%d", stravaID),
		})
	}
	return []string{"Date", distance, "Time", "Total (km)", "Total (time)", "Link"}, results, nil
}
