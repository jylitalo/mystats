package server

import (
	"errors"
	"log"
	"log/slog"
	"strconv"

	"github.com/jylitalo/mystats/pkg/stats"
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
	Data    TableData
	Measure string
	Period  string
}

func newTopData() TopData {
	return TopData{
		Data:    newTableData(),
		Measure: "distance",
		Period:  "week",
	}
}

type TopPage struct {
	Form TopFormData
	Data TopData
}

func newTopPage() *TopPage {
	return &TopPage{
		Form: newTopFormData(),
		Data: newTopData(),
	}
}

func topPost(page *Page, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		var err error

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
			log.Fatal(err)
		}
		page.Top.Form.Limit = limit
		slog.Info("POST /top", "values", values)
		tf := &page.Top.Form
		tf.Years = years
		td := &page.Top.Data
		td.Data.Headers, td.Data.Rows, err = stats.Top(
			db, tf.Measure, tf.Period, selectedTypes(types),
			selectedWorkoutTypes(workoutTypes), tf.Limit, selectedYears(years),
		)
		return errors.Join(err, c.Render(200, "top-data", td))
	}
}
