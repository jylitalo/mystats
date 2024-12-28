package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
)

type StepsFormData struct {
	Name          string
	EndMonth      int
	EndDay        int
	Period        string
	PeriodOptions []string
	Years         map[int]bool
}

func newStepsFormData() StepsFormData {
	t := time.Now()
	return StepsFormData{
		Name:          "steps",
		EndMonth:      int(t.Month()),
		EndDay:        t.Day(),
		Period:        "month",
		PeriodOptions: []string{"month", "week"},
		Years:         map[int]bool{},
	}
}

type StepsData struct {
	Years         []int
	Stats         [][]string
	Totals        []string
	ScriptColumns []int
	ScriptRows    template.JS
	ScriptColors  template.JS
	Period        string
	stats         func(
		ctx context.Context, db Storage, period string, month, day int, years []int,
	) ([]int, [][]string, []string, error)
}

func stepsStats(
	ctx context.Context, db Storage, period string, month, day int, years []int,
) ([]int, [][]string, []string, error) {
	_, span := telemetry.NewSpan(ctx, "server.stepsStats")
	defer span.End()

	inYear := map[string]int{
		"month": 12,
		"week":  53,
	}
	if _, ok := inYear[period]; !ok {
		return nil, nil, nil, telemetry.Error(span, fmt.Errorf("unknown period: %s", period))
	}
	cond := storage.SummaryConditions{Month: month, Day: day, Years: years}
	results := make([][]string, inYear[period])
	years, err := db.QueryYears(cond)
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
	rows, err := db.QuerySteps(
		[]string{"year", period, "totalsteps"}, cond,
		&storage.Order{GroupBy: []string{period, "year"}, OrderBy: []string{period, "year"}},
	)
	if err != nil {
		return nil, nil, nil, telemetry.Error(span, fmt.Errorf("select caused: %w", err))
	}
	defer rows.Close()
	totalsAbs := make([]float64, len(years))
	modifier := float64(1)
	unit := "%4.0fm"
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

func newStepsData() StepsData {
	return StepsData{
		Period: "month",
		stats:  stepsStats,
	}
}

type StepsPage struct {
	Data StepsData
	Form StepsFormData
}

func newStepsPage() *StepsPage {
	return &StepsPage{
		Data: newStepsData(),
		Form: newStepsFormData(),
	}
}

func (p *StepsPage) render(
	ctx context.Context, db Storage, month, day int, years map[int]bool, period string,
) error {
	ctx, span := telemetry.NewSpan(ctx, "steps.render")
	defer span.End()

	colors := []string{
		"#0000ff", // 1
		"#00ff00", // 2
		"#ff0000", // 3
		"#00ffff", // 4
		"#ffff00", // 5
		"#ff00ff", // 6
		"#000088", // 7
		"#008800", // 8
		"#880000", // 9
		"#00f000",
		"#0000f0",
	}

	p.Form.EndMonth = month
	p.Form.EndDay = day
	p.Form.Years = years
	checkedYears := selectedYears(years)
	d := &p.Data
	numbers, err := getSteps(ctx, db, month, day, checkedYears)
	if err != nil {
		slog.Error("failed to steps", "err", err)
		return err
	}
	foundYears := []int{}
	for _, year := range checkedYears {
		if _, ok := numbers[year]; ok {
			foundYears = append(foundYears, year)
		}
	}
	if len(foundYears) == 0 {
		slog.Error("No years found in steps.render()")
		return nil
	}
	refTime, err := time.Parse(time.DateOnly, fmt.Sprintf("%d-01-01", slices.Max(foundYears)))
	if err != nil {
		return err
	}
	scriptRows := [][]interface{}{}
	for day := range numbers[foundYears[0]] {
		scriptRows = append(scriptRows, make([]interface{}, len(foundYears)+1))
		index0 := refTime.Add(24 * time.Duration(day) * time.Hour)
		// Month in JavaScript's Date is 0-indexed
		newDate := fmt.Sprintf("new Date(%d, %d, %d)", index0.Year(), index0.Month()-1, index0.Day())
		scriptRows[day][0] = template.JS(newDate) // #nosec G203
		for idx, year := range foundYears {
			scriptRows[day][idx+1] = numbers[year][day]
		}
	}
	byteRows, _ := json.Marshal(scriptRows)
	byteColors, _ := json.Marshal(colors[0:len(foundYears)])
	p.Data.ScriptColumns = foundYears
	p.Data.ScriptRows = template.JS(strings.ReplaceAll(string(byteRows), `"`, ``)) // #nosec G203
	p.Data.ScriptColors = template.JS(byteColors)                                  // #nosec G203
	d.Years, d.Stats, d.Totals, err = d.stats(ctx, db, period, month, day, foundYears)
	if err != nil {
		slog.Error("failed to calculate stats", "err", err)
	}
	return err
}

func stepsPost(ctx context.Context, page *Page, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		_, span := telemetry.NewSpan(ctx, "stepsPOST")
		defer span.End()
		month, errM := strconv.Atoi(c.FormValue("EndMonth"))
		day, errD := strconv.Atoi(c.FormValue("EndDay"))
		page.Steps.Data.Period = c.FormValue("Period")
		page.Steps.Form.Period = page.Steps.Data.Period
		values, errV := c.FormParams()
		years, errY := yearValues(values)
		if err := errors.Join(errM, errD, errV, errY); err != nil {
			return telemetry.Error(span, err)
		}
		slog.Info("POST /steps", "values", values)
		return telemetry.Error(span, errors.Join(
			page.Steps.render(ctx, db, month, day, years, page.Steps.Data.Period),
			c.Render(200, "steps-data", page.Steps.Data),
		))
	}
}

func getSteps(ctx context.Context, db Storage, month, day int, years []int) (numbers, error) {
	cond := storage.SummaryConditions{Month: month, Day: day, Years: years}
	_, span := telemetry.NewSpan(ctx, "server.getSteps")
	defer span.End()

	years, err := db.QueryYears(cond)
	if err != nil {
		return nil, err
	}
	o := []string{"year", "month", "day"}
	rows, err := db.QuerySteps(append(o, "TotalSteps"), cond, &storage.Order{GroupBy: o, OrderBy: o})
	if err != nil {
		return nil, fmt.Errorf("select caused: %w", err)
	}
	defer func() {
		if rows != nil {
			if err := rows.Close(); err != nil {
				_ = telemetry.Error(span, err)
			}
		}
	}()
	return scan(rows, years)
}