package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jylitalo/mystats/pkg/data"
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

func newStepsFormData(years []int) StepsFormData {
	yearSelection := map[int]bool{}
	for _, y := range years {
		yearSelection[y] = true
	}
	t := time.Now()
	return StepsFormData{
		Name:          "steps",
		EndMonth:      int(t.Month()),
		EndDay:        t.Day(),
		Period:        "month",
		PeriodOptions: []string{"month", "week"},
		Years:         yearSelection,
	}
}

type stepStatsFn func(
	ctx context.Context, db Storage, period string, month, day int, years []int,
) ([]int, [][]string, []string, error)

type StepsData struct {
	Years         []int
	Stats         [][]string
	Totals        []string
	ScriptColumns []int
	ScriptRows    template.JS
	ScriptColors  template.JS
	Period        string
	stats         stepStatsFn
}

func stepsStats(
	ctx context.Context, db Storage, period string, month, day int, years []int,
) ([]int, [][]string, []string, error) {
	_, span := telemetry.NewSpan(ctx, "server.stepsStats")
	defer span.End()
	o := []string{period, "year"}
	opts := []storage.QueryOption{
		storage.WithTable(storage.DailyStepsTable),
		storage.WithDayOfYear(day, month),
		storage.WithOrder(storage.OrderConfig{GroupBy: o, OrderBy: o}),
	}
	opts = append(opts, storage.WithYears(years...))
	inYear := map[string]int{
		"month": 12,
		"week":  53,
	}
	if _, ok := inYear[period]; !ok {
		return nil, nil, nil, telemetry.Error(span, fmt.Errorf("unknown period: %s", period))
	}
	results := make([][]string, inYear[period])
	years, err := db.QueryYears(ctx, opts...)
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
	rows, err := db.Query(ctx, []string{"year", period, "sum(totalsteps)"}, opts...)
	if err != nil {
		return nil, nil, nil, telemetry.Error(span, fmt.Errorf("select caused: %w", err))
	}
	defer func() { _ = rows.Close() }()
	totalsAbs := make([]float64, len(years))
	modifier := float64(1000)
	unit := "%6.1fk"
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

func newStepsData(stats stepStatsFn, period string) StepsData {
	return StepsData{
		Period: period,
		stats:  stats,
	}
}

type StepsPage struct {
	Data StepsData
	Form StepsFormData
}

func newStepsPage(ctx context.Context, db Storage, years []int, stats stepStatsFn) (*StepsPage, error) {
	form := newStepsFormData(years)
	data := newStepsData(stats, form.Period)
	page := &StepsPage{Data: data, Form: form}
	return page, page.render(ctx, db, form.EndMonth, form.EndDay, form.Years, form.Period)
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
	stepCounts, err := getSteps(ctx, db, month, day, checkedYears)
	if err != nil {
		slog.Error("failed to steps", "err", err)
		return err
	}
	foundYears := data.Intersection(slices.Values(checkedYears), maps.Keys(stepCounts))
	if len(foundYears) == 0 {
		slog.Error("No years found in steps.render()")
		return nil
	}
	slices.Sort(foundYears)
	refTime, err := time.Parse(time.DateOnly, fmt.Sprintf("%d-01-01", slices.Max(foundYears)))
	if err != nil {
		return err
	}
	scriptRows := [][]interface{}{}
	for day := range stepCounts[foundYears[0]] {
		scriptRows = append(scriptRows, make([]interface{}, len(foundYears)+1))
		index0 := refTime.Add(24 * time.Duration(day) * time.Hour)
		// Month in JavaScript's Date is 0-indexed
		newDate := fmt.Sprintf("new Date(%d, %d, %d)", index0.Year(), index0.Month()-1, index0.Day())
		scriptRows[day][0] = template.JS(newDate) // #nosec G203
		for idx, year := range foundYears {
			scriptRows[day][idx+1] = stepCounts[year][day]
		}
	}
	byteRows, _ := json.Marshal(scriptRows)
	byteColors, _ := json.Marshal(colors[0:len(foundYears)])
	p.Data.ScriptColumns = foundYears
	p.Data.ScriptRows = template.JS(strings.ReplaceAll(string(byteRows), `"`, ``)) // #nosec G203
	p.Data.ScriptColors = template.JS(byteColors)                                  // #nosec G203
	if d.stats == nil {
		return errors.New("stats is nil in StepsPage.render")
	}
	d.Years, d.Stats, d.Totals, err = d.stats(ctx, db, period, month, day, foundYears)
	if err != nil {
		return fmt.Errorf("failed to calculate stats: %w", err)
	}
	return nil
}

func stepsPost(ctx context.Context, renderer *Template, page *StepsPage, db Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(ctx, "stepsPOST")
		defer span.End()
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			_ = telemetry.Error(span, err)
			return
		}
		month, errM := strconv.Atoi(r.FormValue("EndMonth"))
		day, errD := strconv.Atoi(r.FormValue("EndDay"))
		page.Data.Period = r.FormValue("Period")
		page.Form.Period = page.Data.Period
		values := r.Form
		years, errY := yearValues(values)
		if err := errors.Join(errM, errD, errY); err != nil {
			_ = telemetry.Error(span, err)
			return
		}
		slog.Info("POST /steps", "values", values)
		err := page.render(ctx, db, month, day, years, page.Data.Period)
		if err != nil {
			_ = telemetry.Error(span, err)
			return
		}
		if err := renderer.tmpl.ExecuteTemplate(w, "step-data", page.Data); err != nil {
			_ = telemetry.Error(span, err)
			http.Error(w, "Template rendering failed", http.StatusInternalServerError)
			return
		}
	}
}

func getSteps(ctx context.Context, db Storage, month, day int, years []int) (numbers, error) {
	_, span := telemetry.NewSpan(ctx, "server.getSteps")
	defer span.End()

	years, rows, err := yearToDateQuery(ctx, db, day, month, years, storage.DailyStepsTable, "TotalSteps")
	if err != nil {
		return nil, telemetry.Error(span, err)
	}
	return cumulativeScan(rows, years)
}
