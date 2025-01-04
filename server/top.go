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

func newTopFormData(years []int, types, workouts map[string]bool) TopFormData {
	yearSelection := map[int]bool{}
	for _, y := range years {
		yearSelection[y] = true
	}
	return TopFormData{
		Name:           "top",
		Measure:        "distance",
		MeasureOptions: []string{"distance", "elevation", "time"},
		Period:         "week",
		PeriodOptions:  []string{"week", "month"},
		Types:          types,
		WorkoutTypes:   workouts,
		Years:          yearSelection,
		Limit:          10,
	}
}

type TopData struct {
	Measure string
	Period  string
	TableData
}

func newTopData(ctx context.Context, db Storage, measure, period string, types, workouts []string, limit int, years []int) (*TopData, error) {
	var err error
	data := &TopData{
		Measure: measure,
		Period:  period,
	}
	data.Headers, data.Rows, err = stats.Top(ctx, db, measure, period, types, workouts, limit, years)
	return data, err
}

type TopPage struct {
	Form TopFormData
	Data *TopData
}

func newTopPage(ctx context.Context, db Storage, years []int, types, workouts map[string]bool) (*TopPage, error) {
	form := newTopFormData(years, types, workouts)
	data, err := newTopData(
		ctx, db, form.Measure, form.Period, selectedTypes(types),
		selectedWorkoutTypes(workouts), form.Limit, years,
	)
	return &TopPage{Form: form, Data: data}, err
}

func topPost(ctx context.Context, page *TopPage, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		var err, errL, errT, errW, errY error

		ctx, span := telemetry.NewSpan(ctx, "topPOST")
		defer span.End()
		values, err := c.FormParams()
		slog.Info("POST /top", "values", values)
		page.Form.Types, errT = typeValues(values)
		page.Form.WorkoutTypes, errW = workoutTypeValues(values)
		page.Form.Years, errY = yearValues(values)
		page.Form.Limit, errL = strconv.Atoi(c.FormValue("limit"))
		if err = errors.Join(err, errT, errW, errY, errL); err != nil {
			return telemetry.Error(span, err)
		}
		page.Form.Measure = c.FormValue("Measure")
		page.Form.Period = c.FormValue("Period")
		page.Data, err = newTopData(
			ctx, db, page.Form.Measure, page.Form.Period,
			selectedTypes(page.Form.Types),
			selectedWorkoutTypes(page.Form.WorkoutTypes), page.Form.Limit,
			selectedYears(page.Form.Years),
		)
		if err != nil {
			return telemetry.Error(span, err)
		}
		return telemetry.Error(span, errors.Join(err, c.Render(200, "top-data", page.Data)))
	}
}
