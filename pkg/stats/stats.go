package stats

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/jylitalo/mystats/storage"
)

type Storage interface {
	Query(fields []string, cond storage.Conditions, order *storage.Order) (*sql.Rows, error)
	QueryYears(cond storage.Conditions) ([]int, error)
}

func Stats(db Storage, measurement, period string, types, workoutTypes []string, month, day int, years []int) ([]int, [][]string, []string, error) {
	inYear := map[string]int{
		"month": 12,
		"week":  53,
	}
	if _, ok := inYear[period]; !ok {
		return nil, nil, nil, fmt.Errorf("unknown period: %s", period)
	}
	cond := storage.Conditions{Types: types, WorkoutTypes: workoutTypes, Month: month, Day: day, Years: years}
	results := make([][]string, inYear[period])
	years, err := db.QueryYears(cond)
	if err != nil {
		return nil, nil, nil, err
	}
	yearIndex := map[int]int{}
	for idx, year := range years {
		yearIndex[year] = idx
	}
	columns := len(years)
	for idx := range results {
		results[idx] = make([]string, columns)
		for year := range columns { // helps CSV formatting
			results[idx][year] = "    "
		}
	}
	rows, err := db.Query(
		[]string{"year", period, measurement}, cond,
		&storage.Order{GroupBy: []string{period, "year"}, OrderBy: []string{period, "year"}},
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("select caused: %w", err)
	}
	defer rows.Close()
	totalsAbs := make([]float64, len(years))
	modifier := float64(1)
	if strings.Contains(measurement, "distance") && !strings.Contains(measurement, "count") {
		modifier = 1000
	}
	for rows.Next() {
		var year, periodValue int
		var measureValue float64
		if err = rows.Scan(&year, &periodValue, &measureValue); err != nil {
			return nil, nil, nil, err
		}
		totalsAbs[yearIndex[year]] += measureValue / modifier
		results[periodValue-1][yearIndex[year]] = fmt.Sprintf("%4.0f", measureValue/modifier)
	}
	totals := make([]string, len(years))
	for idx := range totalsAbs {
		totals[idx] = fmt.Sprintf("%4.0f", totalsAbs[idx])
	}
	return years, results, totals, nil
}
