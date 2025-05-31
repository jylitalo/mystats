package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/jylitalo/mystats/pkg/data"
	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
)

type PlotFormData struct {
	Name           string
	EndMonth       int
	EndDay         int
	Measure        string
	MeasureOptions []string
	Period         string
	PeriodOptions  []string
	Sports         map[string]bool
	Workouts       map[string]bool
	Years          map[int]bool
}

func newPlotFormData(years []int, sports, workouts map[string]bool) PlotFormData {
	yearSelection := map[int]bool{}
	for _, y := range years {
		yearSelection[y] = true
	}
	t := time.Now()
	return PlotFormData{
		Name:           "plot",
		EndMonth:       int(t.Month()),
		EndDay:         t.Day(),
		Measure:        "distance",
		MeasureOptions: []string{"distance", "elevation", "time"},
		Period:         "month",
		PeriodOptions:  []string{"month", "week"},
		Sports:         sports,
		Workouts:       workouts,
		Years:          yearSelection,
	}
}

type plotStatsFn func(
	ctx context.Context, db stats.Storage, measure, period string, sports, workouts []string,
	month, day int, years []int,
) ([]int, [][]string, []string, error)

type PlotData struct {
	Years         []int
	Measure       string
	Stats         [][]string
	Totals        []string
	ScriptColumns []int
	ScriptRows    template.JS
	ScriptColors  template.JS
	Period        string
	stats         plotStatsFn
}

func newPlotData(stats plotStatsFn, period string) PlotData {
	return PlotData{
		Measure: "distance",
		Period:  period,
		stats:   stats,
	}
}

type PlotPage struct {
	Data PlotData
	Form PlotFormData
}

func newPlotPage(
	ctx context.Context, db Storage, years []int,
	sports, workouts map[string]bool, stats plotStatsFn,
) (*PlotPage, error) {
	form := newPlotFormData(years, sports, workouts)
	page := &PlotPage{
		Form: form,
		Data: newPlotData(stats, form.Period),
	}
	return page, page.render(
		ctx, db, selectedSports(sports), selectedWorkouts(workouts),
		form.EndMonth, form.EndDay, form.Years, form.Period,
	)
}

func (p *PlotPage) render(
	ctx context.Context, db Storage, sports, workouts []string, month, day int,
	years map[int]bool, period string,
) error {
	ctx, span := telemetry.NewSpan(ctx, "plot.render")
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
	measured, err := getNumbers(ctx, db, sports, workouts, d.Measure, month, day, checkedYears)
	if err != nil {
		slog.Error("failed to plot", "err", err)
		return err
	}
	foundYears := data.Intersection(slices.Values(checkedYears), maps.Keys(measured))
	if len(foundYears) == 0 {
		slog.Error("No years found in plot.render()")
		return nil
	}
	slices.Sort(foundYears)
	refTime, err := time.Parse(time.DateOnly, fmt.Sprintf("%d-01-01", slices.Max(foundYears)))
	if err != nil {
		return err
	}
	scriptRows := [][]interface{}{}
	for day := range measured[foundYears[0]] {
		scriptRows = append(scriptRows, make([]interface{}, len(foundYears)+1))
		index0 := refTime.Add(24 * time.Duration(day) * time.Hour)
		// Month in JavaScript's Date is 0-indexed
		newDate := fmt.Sprintf("new Date(%d, %d, %d)", index0.Year(), index0.Month()-1, index0.Day())
		scriptRows[day][0] = template.JS(newDate) // #nosec G203
		for idx, year := range foundYears {
			scriptRows[day][idx+1] = measured[year][day]
		}
	}
	byteRows, _ := json.Marshal(scriptRows)
	byteColors, _ := json.Marshal(colors[0:len(foundYears)])
	p.Data.ScriptColumns = foundYears
	p.Data.ScriptRows = template.JS(strings.ReplaceAll(string(byteRows), `"`, ``)) // #nosec G203
	p.Data.ScriptColors = template.JS(byteColors)                                  // #nosec G203
	measure := d.Measure
	if measure == "time" {
		measure = "elapsedtime"
	}
	d.Years, d.Stats, d.Totals, err = d.stats(
		ctx, db, "sum("+measure+")", period, sports, workouts, month, day, foundYears,
	)
	if err != nil {
		slog.Error("failed to calculate stats", "err", err)
	}
	return err
}

type numbers map[int][]float64

func plotPost(ctx context.Context, page *PlotPage, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		_, span := telemetry.NewSpan(ctx, "plotPOST")
		defer span.End()
		month, errM := strconv.Atoi(c.FormValue("EndMonth"))
		day, errD := strconv.Atoi(c.FormValue("EndDay"))
		page.Form.Measure = c.FormValue("Measure")
		page.Data.Measure = page.Form.Measure
		page.Data.Period = c.FormValue("Period")
		page.Form.Period = page.Data.Period
		values, errV := c.FormParams()
		sports, errS := sportsValues(values)
		workouts, errW := workoutsValues(values)
		years, errY := yearValues(values)
		if err := errors.Join(errM, errD, errV, errS, errW, errY); err != nil {
			return telemetry.Error(span, err)
		}
		slog.Info("POST /plot", "values", values)
		return telemetry.Error(span, errors.Join(
			page.render(
				ctx, db, selectedSports(sports), selectedWorkouts(workouts),
				month, day, years, page.Data.Period,
			),
			c.Render(200, "plot-data", page.Data),
		))
	}
}

func cumulativeScan(rows *sql.Rows, years []int) (numbers, error) {
	tz, _ := time.LoadLocation("Europe/Helsinki")
	day1 := map[int]time.Time{}
	// ys is map, where key is year and array has entry for each day of the year
	ys := map[int][]float64{}
	previous_y := map[int]float64{}
	for _, year := range years {
		day1[year] = time.Date(year, time.January, 1, 6, 0, 0, 0, tz)
		ys[year] = []float64{}
		previous_y[year] = 0
	}
	max_acts := 0
	if rows == nil {
		return ys, nil
	}
	for rows.Next() { // scan through database rows
		var year, month, day int
		var value float64
		if err := rows.Scan(&year, &month, &day, &value); err != nil {
			return ys, err
		}
		now := time.Date(year, time.Month(month), day, 6, 0, 0, 0, tz) // time when activity happened
		days := int(now.Sub(day1[year]).Hours()/24) + 1                // day within a year (1-365)
		if days > 366 {
			log.Fatalf(
				"days got impossible number %d (year=%d, month=%d, day=%d, now=%#v, day1=%#v)",
				days, year, month, day, now, day1[year],
			)
		}
		yslen := len(ys[year])
		for x := yslen; x < days-1; x++ { // fill the gaps on days that didn't have activities
			ys[year] = append(ys[year], previous_y[year])
		}
		max_acts = max(max_acts, days)
		previous_y[year] += value
		ys[year] = append(ys[year], previous_y[year])
	}
	for _, year := range years { // fill the end of year
		yslen := len(ys[year])
		for x := yslen; x < max_acts; x++ {
			ys[year] = append(ys[year], previous_y[year])
		}
	}
	return ys, nil
}

func getNumbers(
	ctx context.Context, db Storage, sports, workouts []string, measure string,
	month, day int, years []int,
) (numbers, error) {
	_, span := telemetry.NewSpan(ctx, "server.getNumbers")
	defer span.End()
	o := []string{"year", "month", "day"}
	opts := []storage.QueryOption{
		storage.WithTable(storage.SummaryTable),
		storage.WithDayOfYear(day, month),
		storage.WithOrder(storage.OrderConfig{GroupBy: o, OrderBy: o}),
	}
	opts = append(opts, storage.WithSports(sports...))
	opts = append(opts, storage.WithWorkouts(workouts...))
	opts = append(opts, storage.WithYears(years...))
	var m string
	switch measure {
	case "time":
		m = "sum(elapsedtime)/3600"
	case "distance":
		m = "sum(distance)/1000"
	case "elevation":
		m = "sum(elevation)"
	}
	rows, err := db.Query(append(o, m), opts...)
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
	foundYears, err := db.QueryYears(opts...)
	if err != nil {
		return nil, err
	}
	return cumulativeScan(rows, foundYears)
}
