package stats

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jylitalo/mystats/storage"
)

func Top(db Storage, measure, period string, types, workoutTypes []string, limit int, years []int) ([]string, [][]string, error) {
	modifier := float64(1)
	unit := "%4.0fm"
	switch {
	case strings.Contains(measure, "distance") && !strings.Contains(measure, "count"):
		modifier = 1000
		unit = "%4.1fkm"
	case strings.Contains(measure, "time") && !strings.Contains(measure, "count"):
		modifier = 3600
		unit = "%4.1fh"
	}
	if measure == "time" {
		measure = "elapsedtime"
	}
	results := [][]string{}
	rows, err := db.QuerySummary(
		[]string{"sum(" + measure + ") as total", "year", period},
		storage.SummaryConditions{Types: types, WorkoutTypes: workoutTypes, Years: years},
		&storage.Order{
			GroupBy: []string{"year", period},
			OrderBy: []string{"total desc", "year desc", period + " desc"},
			Limit:   limit},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("select caused: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var year, periodValue int
		var measureValue float64
		if err = rows.Scan(&measureValue, &year, &periodValue); err != nil {
			return nil, nil, err
		}
		value := fmt.Sprintf(unit, measureValue/modifier)
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
