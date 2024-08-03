package stats

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/jylitalo/mystats/storage"
)

type Storage interface {
	QueryBestEffort(fields []string, name string, order *storage.Order) (*sql.Rows, error)
	QuerySplit(fields []string, id int64) (*sql.Rows, error)
	QuerySummary(fields []string, cond storage.SummaryConditions, order *storage.Order) (*sql.Rows, error)
	QueryYears(cond storage.SummaryConditions) ([]int, error)
}

func Stats(db Storage, measure, period string, types, workoutTypes []string, month, day int, years []int) ([]int, [][]string, []string, error) {
	inYear := map[string]int{
		"month": 12,
		"week":  53,
	}
	if _, ok := inYear[period]; !ok {
		return nil, nil, nil, fmt.Errorf("unknown period: %s", period)
	}
	cond := storage.SummaryConditions{
		Types: types, WorkoutTypes: workoutTypes, Month: month, Day: day, Years: years,
	}
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
	if strings.Contains(measure, "(time)") {
		measure = strings.ReplaceAll(measure, "(time)", "(elapsedtime)")
	}
	rows, err := db.QuerySummary(
		[]string{"year", period, measure}, cond,
		&storage.Order{GroupBy: []string{period, "year"}, OrderBy: []string{period, "year"}},
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("select caused: %w", err)
	}
	defer rows.Close()
	totalsAbs := make([]float64, len(years))
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
	for rows.Next() {
		var year, periodValue int
		var measureValue float64
		if err = rows.Scan(&year, &periodValue, &measureValue); err != nil {
			return nil, nil, nil, err
		}
		totalsAbs[yearIndex[year]] += measureValue / modifier
		results[periodValue-1][yearIndex[year]] = fmt.Sprintf(unit, measureValue/modifier)
	}
	totals := make([]string, len(years))
	for idx := range totalsAbs {
		totals[idx] = fmt.Sprintf(unit, totalsAbs[idx])
	}
	return years, results, totals, nil
}
