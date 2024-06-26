package server

import (
	"errors"
	"log"
	"log/slog"

	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/labstack/echo/v4"
)

type TopFormData struct {
	Name         string
	Types        map[string]bool
	WorkoutTypes map[string]bool
	Years        map[int]bool
	measurement  string
	period       string
	limit        int
}

func newTopFormData() TopFormData {
	return TopFormData{
		Name:         "top",
		Types:        map[string]bool{},
		WorkoutTypes: map[string]bool{},
		Years:        map[int]bool{},
		measurement:  "sum(distance)",
		period:       "week",
		limit:        100,
	}
}

type TopPage struct {
	Form TopFormData
	Data TableData
}

func newTopPage() *TopPage {
	return &TopPage{
		Form: newTopFormData(),
		Data: newTableData(),
	}
}

func topPost(page *Page, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		var err error

		values, errV := c.FormParams()
		types, errT := typeValues(values)
		workoutTypes, errW := workoutTypeValues(values)
		years, errY := yearValues(values)
		if err = errors.Join(errV, errT, errW, errY); err != nil {
			log.Fatal(err)
		}
		slog.Info("POST /top", "values", values)
		tf := &page.Top.Form
		tf.Years = years
		td := &page.Top.Data
		td.Headers, td.Rows, err = stats.Top(
			db, tf.measurement, tf.period, selectedTypes(types), selectedWorkoutTypes(workoutTypes),
			tf.limit, selectedYears(years),
		)
		return errors.Join(err, c.Render(200, "top-data", td))
	}
}
