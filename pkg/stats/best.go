package stats

import (
	"context"
	"fmt"

	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
)

func Best(ctx context.Context, db Storage, distance string, limit int) ([]string, [][]string, error) {
	_, span := telemetry.NewSpan(ctx, "stats.Best")
	defer span.End()

	o := []string{storage.BestEffortTable + ".Movingtime", "Year", "Month", "Day"}
	rows, err := db.Query(
		[]string{
			"Year", "Month", "Day",
			storage.SummaryTable + ".Name",
			storage.SummaryTable + ".Distance",
			storage.SummaryTable + ".Elapsedtime",
			storage.BestEffortTable + ".Movingtime",
			storage.BestEffortTable + ".Elapsedtime",
			storage.SummaryTable + ".StravaID",
		},
		storage.WithName(distance),
		storage.WithTable(storage.SummaryTable), storage.WithTable(storage.BestEffortTable),
		storage.WithOrder(storage.OrderConfig{OrderBy: o, Limit: limit}),
	)
	if err != nil {
		return nil, nil, telemetry.Error(span, fmt.Errorf("query caused: %w", err))
	}
	defer rows.Close()
	results := [][]string{}
	for rows.Next() {
		var year, month, day, movingTime, elapsedTime, totalTime, stravaID int
		var distance float64
		var name string
		err = rows.Scan(&year, &month, &day, &name, &distance, &totalTime, &movingTime, &elapsedTime, &stravaID)
		if err != nil {
			return nil, nil, telemetry.Error(span, err)
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
