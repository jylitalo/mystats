package server

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/labstack/echo/v4"
)

type TopFormData struct {
	Name           string
	Measure        string
	MeasureOptions []string
	Period         string
	PeriodOptions  []string
	Types          map[string]bool
	WorkoutTypes   map[string]bool
	Years          map[int]bool
	Limit          int
}

func newTopFormData() TopFormData {
	return TopFormData{
		Name:           "top",
		Measure:        "distance",
		MeasureOptions: []string{"distance", "elevation", "time"},
		Period:         "week",
		PeriodOptions:  []string{"week", "month"},
		Types:          map[string]bool{},
		WorkoutTypes:   map[string]bool{},
		Years:          map[int]bool{},
		Limit:          10,
	}
}

type TopData struct {
	Measure string
	Period  string
	TableData
}

type TopPage struct {
	Form TopFormData
	Data TopData
}

func newTopPage() *TopPage {
	return &TopPage{
		Form: newTopFormData(),
		Data: TopData{
			Measure:   "distance",
			Period:    "week",
			TableData: newTableData(),
		},
	}
}

func topPost(ctx context.Context, page *Page, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		var err error

		ctx, span := telemetry.NewSpan(ctx, "topPOST")
		defer span.End()
		values, errV := c.FormParams()
		types, errT := typeValues(values)
		workoutTypes, errW := workoutTypeValues(values)
		years, errY := yearValues(values)
		limit, errL := strconv.Atoi(c.FormValue("limit"))
		page.Top.Data.Measure = c.FormValue("Measure")
		page.Top.Form.Measure = page.Top.Data.Measure
		page.Top.Data.Period = c.FormValue("Period")
		page.Top.Form.Period = page.Top.Data.Period
		if err = errors.Join(errV, errT, errW, errY, errL); err != nil {
			return telemetry.Error(span, err)
		}
		page.Top.Form.Limit = limit
		slog.Info("POST /top", "values", values)
		tf := &page.Top.Form
		tf.Years = years
		td := &page.Top.Data
		td.Headers, td.Rows, err = stats.Top(
			ctx, db, tf.Measure, tf.Period, selectedTypes(types),
			selectedWorkoutTypes(workoutTypes), tf.Limit, selectedYears(years),
		)
		return telemetry.Error(span, errors.Join(err, c.Render(200, "top-data", td)))
	}
}
