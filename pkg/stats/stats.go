package stats

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"

	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
)

type Storage interface {
	QueryYears(ctx context.Context, opts ...storage.QueryOption) ([]int, error)
	Query(ctx context.Context, fields []string, opts ...storage.QueryOption) (*sql.Rows, error)
}

func Stats(
	ctx context.Context, db Storage, measure, period string, sports, workouts []string,
	month, day int, years []int,
) ([]int, [][]string, []string, error) {
	_, span := telemetry.NewSpan(ctx, "stats.Stats")
	defer span.End()

	if years == nil {
		var err error
		if years, err = db.QueryYears(ctx); err != nil {
			return nil, nil, nil, telemetry.Error(span, err)
		}
	}
	yearIndex := map[int]int{}
	for idx, year := range years {
		yearIndex[year] = idx
	}
	inYear := map[string]int{
		"month": 12,
		"week":  53,
	}
	if _, ok := inYear[period]; !ok {
		return nil, nil, nil, telemetry.Error(span, fmt.Errorf("unknown period: %s", period))
	}
	results := make([][]string, inYear[period])
	columns := len(years)
	for idx := range results {
		results[idx] = slices.Repeat([]string{"    "}, columns) // helps CSV formatting
	}
	measure = strings.ReplaceAll(measure, "(time)", "(elapsedtime)")
	o := []string{period, "year"}
	opts := []storage.QueryOption{
		storage.WithTable(storage.SummaryTable),
		storage.WithDayOfYear(day, month),
		storage.WithOrder(storage.OrderConfig{GroupBy: o, OrderBy: o}),
		storage.WithSports(sports...),
		storage.WithWorkouts(workouts...),
		storage.WithYears(years...),
	}
	rows, err := db.Query(ctx, []string{"Year", period, measure}, opts...)
	if err != nil {
		return nil, nil, nil, telemetry.Error(span, fmt.Errorf("select caused: %w", err))
	}
	defer func() { _ = rows.Close() }()
	totalsAbs := make([]float64, len(years))
	modifier, unit := getModifier(measure)
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

func getModifier(measure string) (float64, string) {
	modifier := float64(1)
	unit := "%4.0fm"
	switch {
	case strings.Contains(measure, "count"):
	case strings.Contains(measure, "distance"):
		modifier = 1000
		unit = "%4.1fkm"
	case strings.Contains(measure, "time"):
		modifier = 3600
		unit = "%4.1fh"
	}
	return modifier, unit
}
