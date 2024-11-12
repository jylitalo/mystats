package stats

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
)

func Top(ctx context.Context, db Storage, measure, period string, types, workoutTypes []string, limit int, years []int) ([]string, [][]string, error) {
	_, span := telemetry.NewSpan(ctx, "stats.Top")
	defer span.End()

	var m, unit, p string
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
	switch period {
	case "month":
		p = "month"
	case "week":
		p = "week"
	case "day":
		p = "day"
	}
	results := [][]string{}
	rows, err := db.QuerySummary(
		[]string{m + " as total", "year", p},
		storage.SummaryConditions{Types: types, WorkoutTypes: workoutTypes, Years: years},
		&storage.Order{
			GroupBy: []string{"year", p},
			OrderBy: []string{"total desc", "year desc", p + " desc"},
			Limit:   limit},
	)
	if err != nil {
		return nil, nil, telemetry.Error(span, fmt.Errorf("select caused: %w", err))
	}
	defer rows.Close()
	for rows.Next() {
		var year, periodValue int
		var measureValue float64
		if err = rows.Scan(&measureValue, &year, &periodValue); err != nil {
			return nil, nil, telemetry.Error(span, err)
		}
		value := fmt.Sprintf(unit, measureValue)
		periodStr := strconv.FormatInt(int64(periodValue), 10)
		if p == "month" {
			periodStr = time.Month(periodValue).String()
		}
		results = append(
			results, []string{value, strconv.FormatInt(int64(year), 10), periodStr},
		)
	}
	return []string{measure, "year", p}, results, nil
}
