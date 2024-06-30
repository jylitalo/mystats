package stats

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jylitalo/mystats/storage"
)

func Top(db Storage, measurement, period string, types, workoutTypes []string, limit int, years []int) ([]string, [][]string, error) {
	results := [][]string{}
	rows, err := db.QuerySummary(
		[]string{measurement + " as total", "year", period},
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
		value := ""
		if strings.Contains(measurement, "distance") && !strings.Contains(measurement, "count") {
			value = fmt.Sprintf("%4.1fkm", measureValue/1000)
		} else {
			value = fmt.Sprintf("%4.0f", measureValue)
		}
		results = append(
			results,
			[]string{value, strconv.FormatInt(int64(year), 10), strconv.FormatInt(int64(periodValue), 10)},
		)
	}
	return []string{measurement, "year", period}, results, nil
}
