package server

import (
	"context"
	"database/sql"
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
	Types          map[string]bool
	WorkoutTypes   map[string]bool
	Years          map[int]bool
}

func newPlotFormData() PlotFormData {
	t := time.Now()
	return PlotFormData{
		Name:           "plot",
		EndMonth:       int(t.Month()),
		EndDay:         t.Day(),
		Measure:        "distance",
		MeasureOptions: []string{"distance", "elevation", "time"},
		Period:         "month",
		PeriodOptions:  []string{"month", "week"},
		Types:          map[string]bool{},
		WorkoutTypes:   map[string]bool{},
		Years:          map[int]bool{},
	}
}

type PlotData struct {
	Years         []int
	Measure       string
	Stats         [][]string
	Totals        []string
	ScriptColumns []int
	ScriptRows    template.JS
	ScriptColors  template.JS
	Period        string
	stats         func(
		ctx context.Context, db stats.Storage, measure, period string, types, workoutTypes []string,
		month, day int, years []int,
	) ([]int, [][]string, []string, error)
}

func newPlotData() PlotData {
	return PlotData{
		Measure: "distance",
		Period:  "month",
		stats:   stats.Stats,
	}
}

type PlotPage struct {
	Data PlotData
	Form PlotFormData
}

func newPlotPage() *PlotPage {
	return &PlotPage{
		Data: newPlotData(),
		Form: newPlotFormData(),
	}
}

func (p *PlotPage) render(
	ctx context.Context, db Storage, types, workoutTypes map[string]bool, month, day int,
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
	checkedTypes := selectedTypes(types)
	checkedWorkoutTypes := selectedWorkoutTypes(workoutTypes)
	checkedYears := selectedYears(years)
	d := &p.Data
	numbers, err := getNumbers(ctx, db, checkedTypes, checkedWorkoutTypes, d.Measure, month, day, checkedYears)
	if err != nil {
		slog.Error("failed to plot", "err", err)
		return err
	}
	foundYears := []int{}
	for _, year := range checkedYears {
		if _, ok := numbers[year]; ok {
			foundYears = append(foundYears, year)
		}
	}
	if len(foundYears) == 0 {
		slog.Error("No years found in plot.render()")
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
	measure := d.Measure
	if measure == "time" {
		measure = "elapsedtime"
	}
	d.Years, d.Stats, d.Totals, err = d.stats(
		ctx, db, "sum("+measure+")", period, checkedTypes, checkedWorkoutTypes,
		month, day, foundYears,
	)
	if err != nil {
		slog.Error("failed to calculate stats", "err", err)
	}
	return err
}

type numbers map[int][]float64

func plotPost(ctx context.Context, page *Page, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		_, span := telemetry.NewSpan(ctx, "plotPOST")
		defer span.End()
		month, errM := strconv.Atoi(c.FormValue("EndMonth"))
		day, errD := strconv.Atoi(c.FormValue("EndDay"))
		page.Plot.Form.Measure = c.FormValue("Measure")
		page.Plot.Data.Measure = page.Plot.Form.Measure
		page.Plot.Data.Period = c.FormValue("Period")
		page.Plot.Form.Period = page.Plot.Data.Period
		values, errV := c.FormParams()
		types, errT := typeValues(values)
		workoutTypes, errW := workoutTypeValues(values)
		years, errY := yearValues(values)
		if err := errors.Join(errM, errD, errV, errT, errW, errY); err != nil {
			return telemetry.Error(span, err)
		}
		slog.Info("POST /plot", "values", values)
		return telemetry.Error(span, errors.Join(
			page.Plot.render(ctx, db, types, workoutTypes, month, day, years, page.Plot.Data.Period),
			c.Render(200, "plot-data", page.Plot.Data),
		))
	}
}

func scan(rows *sql.Rows, years []int) (numbers, error) {
	tz, _ := time.LoadLocation("Europe/Helsinki")
	day1 := map[int]time.Time{}
	ys := map[int][]float64{}
	for _, year := range years {
		day1[year] = time.Date(year, time.January, 1, 6, 0, 0, 0, tz)
		ys[year] = []float64{}
	}
	xmax := 0
	if rows == nil {
		return ys, nil
	}
	for rows.Next() {
		var year, month, day int
		var value float64
		if err := rows.Scan(&year, &month, &day, &value); err != nil {
			return ys, err
		}
		now := time.Date(year, time.Month(month), day, 6, 0, 0, 0, tz)
		days := int(now.Sub(day1[year]).Hours() / 24)
		yslen := len(ys[year])
		y := float64(0)
		if yslen > 0 {
			y = ys[year][yslen-1]
		}
		for x := yslen; x < days-1; x++ {
			ys[year] = append(ys[year], y)
		}
		xmax = max(xmax, days)
		ys[year] = append(ys[year], y+value)
	}
	for _, year := range years {
		yslen := len(ys[year])
		y := float64(0)
		if yslen > 0 {
			y = ys[year][yslen-1]
		}
		for x := yslen; x < xmax; x++ {
			ys[year] = append(ys[year], y)
		}
	}
	return ys, nil
}

func getNumbers(
	ctx context.Context, db Storage, types, workoutTypes []string, measure string,
	month, day int, years []int,
) (numbers, error) {
	cond := storage.SummaryConditions{
		Types: types, WorkoutTypes: workoutTypes, Month: month, Day: day, Years: years,
	}
	_, span := telemetry.NewSpan(ctx, "server.getNumbers")
	defer span.End()

	years, err := db.QueryYears(cond)
	if err != nil {
		return nil, err
	}
	var m string
	switch measure {
	case "time":
		m = "sum(elapsedtime)/3600"
	case "distance":
		m = "sum(distance)/1000"
	case "elevation":
		m = "sum(elevation)"
	}
	o := []string{"year", "month", "day"}
	rows, err := db.QuerySummary(append(o, m), cond, &storage.Order{GroupBy: o, OrderBy: o})
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
