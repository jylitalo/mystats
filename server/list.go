package server

import (
	"errors"
	"log"
	"log/slog"
	"strconv"

	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/labstack/echo/v4"
)

type ListFormData struct {
	Name         string
	Types        map[string]bool
	WorkoutTypes map[string]bool
	Years        map[int]bool
	Limit        int
}

func newListFormData() ListFormData {
	return ListFormData{
		Name:         "list",
		Types:        map[string]bool{},
		WorkoutTypes: map[string]bool{},
		Years:        map[int]bool{},
		Limit:        100,
	}
}

type ListPage struct {
	Form  ListFormData
	Data  TableData
	Event TableData
}

func newListPage() *ListPage {
	return &ListPage{
		Form:  newListFormData(),
		Data:  newTableData(),
		Event: newTableData(),
	}
}

func listPost(page *Page, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		var err error

		values, errV := c.FormParams()
		types, errT := typeValues(values)
		workoutTypes, errW := workoutTypeValues(values)
		years, errY := yearValues(values)
		limit, errL := strconv.Atoi(c.FormValue("limit"))
		name := c.FormValue("name")
		if err = errors.Join(errV, errT, errW, errY, errL); err != nil {
			log.Fatal(err)
		}
		slog.Info("POST /list", "values", values)
		page.List.Form.Years = years
		page.List.Data.Headers, page.List.Data.Rows, err = stats.List(
			db, selectedTypes(types), selectedWorkoutTypes(workoutTypes), selectedYears(years), limit, name,
		)
		return errors.Join(err, c.Render(200, "list-data", page.List.Data))
	}
}

func listEvent(page *Page, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.FormValue("id"))
		if err != nil {
			log.Fatal(err)
		}
		page.List.Event.Headers, page.List.Event.Rows, err = stats.Split(db, int64(id))
		return errors.Join(err, c.Render(200, "list-event", page.List.Event))
	}
}
