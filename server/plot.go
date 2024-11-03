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

	"github.com/jylitalo/mystats/pkg/plot"
	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/jylitalo/mystats/pkg/telemetry"
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
	plot          func(
		ctx context.Context, db plot.Storage, types, workoutTypes []string, measure string,
		month, day int, years []int, filename string,
	) error
	stats func(
		ctx context.Context, db stats.Storage, measure, period string, types, workoutTypes []string,
		month, day int, years []int,
	) ([]int, [][]string, []string, error)
}

func newPlotData() PlotData {
	return PlotData{
		Measure: "distance",
		Period:  "month",
		plot:    plot.Plot,
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
	numbers, err := plot.GetNumbers(ctx, db, checkedTypes, checkedWorkoutTypes, d.Measure, month, day, checkedYears)
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
	refTime, err := time.Parse(time.DateOnly, fmt.Sprintf("%d-01-01", slices.Max(foundYears)))
	if err != nil {
		return err
	}
	scriptRows := [][]interface{}{}
	for day := range numbers[foundYears[0]] {
		scriptRows = append(scriptRows, make([]interface{}, len(foundYears)+1))
		delta, _ := time.ParseDuration(fmt.Sprintf("%dh", 24*day))
		index0 := refTime.Add(delta)
		// Month in JavaScript's Date is 0-indexed
		scriptRows[day][0] = template.JS(fmt.Sprintf("new Date(%d, %d, %d)", index0.Year(), index0.Month()-1, index0.Day()))
		for idx, year := range foundYears {
			scriptRows[day][idx+1] = numbers[year][day]
		}
	}
	byteRows, _ := json.Marshal(scriptRows)
	byteColors, _ := json.Marshal(colors[0:len(foundYears)])
	p.Data.ScriptColumns = foundYears
	p.Data.ScriptRows = template.JS(strings.ReplaceAll(string(byteRows), `"`, ``))
	p.Data.ScriptColors = template.JS(byteColors)
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
