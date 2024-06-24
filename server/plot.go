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

type PlotData struct {
	Years       []int
	Measurement string
	Stats       [][]string
	Totals      []string
	Filename    string
	plot        func(db plot.Storage, types []string, measurement string, month, day int, years []int, filename string) error
	stats       func(db stats.Storage, measurement, period string, types []string, month, day int, years []int) ([]int, [][]string, []string, error)
}

func newPlotData() PlotData {
	return PlotData{
		Measurement: "sum(distance)",
		plot:        plot.Plot,
		stats:       stats.Stats,
	}
}

type PlotFormData struct {
	EndMonth int
	EndDay   int
	Years    map[int]bool
}

func newPlotFormData() PlotFormData {
	t := time.Now()
	return PlotFormData{
		EndMonth: int(t.Month()),
		EndDay:   t.Day(),
		Years:    map[int]bool{},
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

func (p *PlotPage) render(db Storage, types []string, month, day int, years map[int]bool) error {
	p.Form.EndMonth = month
	p.Form.EndDay = day
	p.Form.Years = years
	checked := selectedYears(years)
	d := &p.Data
	d.Filename = "cache/plot-" + uuid.NewString() + ".png"
	err := d.plot(db, types, "distance", month, day, checked, "server/"+d.Filename)
	if err != nil {
		slog.Error("failed to plot", "err", err)
		return err
	}
	d.Years, d.Stats, d.Totals, err = d.stats(db, d.Measurement, "month", types, month, day, checked)
	if err != nil {
		slog.Error("failed to calculate stats", "err", err)
	}
	return err
}

func plotPost(page *Page, db Storage, types []string) func(c echo.Context) error {
	return func(c echo.Context) error {
		month, errM := strconv.Atoi(c.FormValue("EndMonth"))
		day, errD := strconv.Atoi(c.FormValue("EndDay"))
		values, errV := c.FormParams()
		years, errY := yearValues(values)
		if err := errors.Join(errM, errD, errV, errY); err != nil {
			log.Fatal(err)
		}
		slog.Info("POST /plot", "values", values)
		return errors.Join(
			page.Plot.render(db, types, month, day, years),
			c.Render(200, "plot-data", page.Plot.Data),
		)
	}
}
