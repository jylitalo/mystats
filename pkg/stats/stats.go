package stats

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
)

type Storage interface {
	QueryYears(opts ...storage.QueryOption) ([]int, error)
	Query(fields []string, opts ...storage.QueryOption) (*sql.Rows, error)
}

func Stats(
	ctx context.Context, db Storage, measure, period string, sports, workouts []string,
	month, day int, years []int,
) ([]int, [][]string, []string, error) {
	_, span := telemetry.NewSpan(ctx, "stats.Stats")
	defer span.End()

	inYear := map[string]int{
		"month": 12,
		"week":  53,
	}
	if _, ok := inYear[period]; !ok {
		return nil, nil, nil, telemetry.Error(span, fmt.Errorf("unknown period: %s", period))
	}
	results := make([][]string, inYear[period])
	years, err := db.QueryYears()
	if err != nil {
		return nil, nil, nil, telemetry.Error(span, err)
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
	o := []string{period, "year"}
	opts := []storage.QueryOption{
		storage.WithTable(storage.SummaryTable),
		storage.WithDayOfYear(day, month),
		storage.WithOrder(storage.OrderConfig{GroupBy: o, OrderBy: o}),
	}
	for _, s := range sports {
		opts = append(opts, storage.WithSport(s))
	}
	for _, w := range workouts {
		opts = append(opts, storage.WithWorkout(w))
	}
	for _, y := range years {
		opts = append(opts, storage.WithYear(y))
	}
	rows, err := db.Query([]string{"Year", period, measure}, opts...)
	if err != nil {
		return nil, nil, nil, telemetry.Error(span, fmt.Errorf("select caused: %w", err))
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
