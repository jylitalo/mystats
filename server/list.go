package server

import (
	"errors"
	"log"
	"log/slog"

	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/labstack/echo/v4"
)

type ListFormData struct {
	Name         string
	Types        map[string]bool
	WorkoutTypes map[string]bool
	Years        map[int]bool
}

func newListFormData() ListFormData {
	return ListFormData{
		Name:         "list",
		Types:        map[string]bool{},
		WorkoutTypes: map[string]bool{},
		Years:        map[int]bool{},
	}
}

type ListPage struct {
	Form ListFormData
	Data TableData
}

func newListPage() *ListPage {
	return &ListPage{
		Form: newListFormData(),
		Data: newTableData(),
	}
}

func listPost(page *Page, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		var err error

		values, errV := c.FormParams()
		types, errT := typeValues(values)
		workoutTypes, errW := workoutTypeValues(values)
		years, errY := yearValues(values)
		if err = errors.Join(errV, errT, errW, errY); err != nil {
			log.Fatal(err)
		}
		slog.Info("POST /list", "values", values)
		page.List.Form.Years = years
		page.List.Data.Headers, page.List.Data.Rows, err = stats.List(
			db, selectedTypes(types), selectedWorkoutTypes(workoutTypes), selectedYears(years),
		)
		return errors.Join(err, c.Render(200, "list-data", page.List.Data))
	}
}
