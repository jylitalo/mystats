package server

import (
	"errors"
	"log"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/jylitalo/mystats/pkg/plot"
	"github.com/jylitalo/mystats/pkg/stats"
)

type PlotFormData struct {
	Name          string
	EndMonth      int
	EndDay        int
	Period        string
	PeriodOptions []string
	Types         map[string]bool
	WorkoutTypes  map[string]bool
	Years         map[int]bool
}

func newPlotFormData() PlotFormData {
	t := time.Now()
	return PlotFormData{
		Name:          "plot",
		EndMonth:      int(t.Month()),
		EndDay:        t.Day(),
		Period:        "month",
		PeriodOptions: []string{"month", "week"},
		Types:         map[string]bool{},
		WorkoutTypes:  map[string]bool{},
		Years:         map[int]bool{},
	}
}

type PlotData struct {
	Years       []int
	Measurement string
	Stats       [][]string
	Totals      []string
	Filename    string
	Period      string
	plot        func(db plot.Storage, types, workoutTypes []string, measurement string, month, day int, years []int, filename string) error
	stats       func(db stats.Storage, measurement, period string, types, workoutTypes []string, month, day int, years []int) ([]int, [][]string, []string, error)
}

func newPlotData() PlotData {
	return PlotData{
		Measurement: "distance",
		plot:        plot.Plot,
		stats:       stats.Stats,
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
	db Storage, types, workoutTypes map[string]bool, month, day int, years map[int]bool,
	period string,
) error {
	p.Form.EndMonth = month
	p.Form.EndDay = day
	p.Form.Years = years
	checkedTypes := selectedTypes(types)
	checkedWorkoutTypes := selectedWorkoutTypes(workoutTypes)
	checkedYears := selectedYears(years)
	d := &p.Data
	d.Filename = "cache/plot-" + uuid.NewString() + ".png"
	err := d.plot(
		db, checkedTypes, checkedWorkoutTypes, d.Measurement, month, day, checkedYears,
		"server/"+d.Filename,
	)
	if err != nil {
		slog.Error("failed to plot", "err", err)
		return err
	}
	d.Years, d.Stats, d.Totals, err = d.stats(
		db, "sum("+d.Measurement+")", period, checkedTypes, checkedWorkoutTypes,
		month, day, checkedYears,
	)
	if err != nil {
		slog.Error("failed to calculate stats", "err", err)
	}
	return err
}

func plotPost(page *Page, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		month, errM := strconv.Atoi(c.FormValue("EndMonth"))
		day, errD := strconv.Atoi(c.FormValue("EndDay"))
		page.Plot.Data.Period = c.FormValue("Period")
		values, errV := c.FormParams()
		types, errT := typeValues(values)
		workoutTypes, errW := workoutTypeValues(values)
		years, errY := yearValues(values)
		if err := errors.Join(errM, errD, errV, errT, errW, errY); err != nil {
			log.Fatal(err)
		}
		slog.Info("POST /plot", "values", values)
		return errors.Join(
			page.Plot.render(db, types, workoutTypes, month, day, years, page.Plot.Data.Period),
			c.Render(200, "plot-data", page.Plot.Data),
		)
	}
}
