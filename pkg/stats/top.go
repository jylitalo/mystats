package stats

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
)

func Top(
	ctx context.Context, db Storage, measure, period string, sports, workouts []string, limit int, years []int,
) ([]string, [][]string, error) {
	_, span := telemetry.NewSpan(ctx, "stats.Top")
	defer span.End()

	var m, unit string
	switch measure {
	case "time":
		m = "sum(elapsedtime)/3600"
		unit = "%4.1fh"
	case "distance":
		m = "sum(distance)/1000"
		unit = "%4.1fkm"
	case "elevation":
		m = "sum(elevation)"
		unit = "%4.0fm"
	}
	if !slices.Contains([]string{"month", "week", "day"}, period) {
		return nil, nil, fmt.Errorf("valid values for top query are month, week and day (not %s)", period)
	}
	results := [][]string{}
	opts := []storage.QueryOption{
		storage.WithOrder(storage.OrderConfig{
			GroupBy: []string{"year", period},
			OrderBy: []string{"total desc", "year desc", period + " desc"},
			Limit:   limit,
		}),
		storage.WithTable(storage.SummaryTable),
		storage.WithSports(sports...),
		storage.WithWorkouts(workouts...),
		storage.WithYears(years...),
	}
	rows, err := db.Query([]string{m + " as total", "year", period}, opts...)
	if err != nil {
		return nil, nil, telemetry.Error(span, fmt.Errorf("select caused: %w", err))
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var year, periodValue int
		var measureValue float64
		if err = rows.Scan(&measureValue, &year, &periodValue); err != nil {
			return nil, nil, telemetry.Error(span, err)
		}
		value := fmt.Sprintf(unit, measureValue)
		periodStr := strconv.FormatInt(int64(periodValue), 10)
		if period == "month" {
			periodStr = time.Month(periodValue).String()
		}
		results = append(
			results, []string{value, strconv.FormatInt(int64(year), 10), periodStr},
		)
	}
	return []string{measure, "year", period}, results, nil
}
