package server

import (
	"errors"
	"log"
	"log/slog"

	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/labstack/echo/v4"
)

type ListFormData struct {
	Workouts []string
	Years    map[int]bool
}

func newListFormData() ListFormData {
	return ListFormData{
		Workouts: []string{},
		Years:    map[int]bool{},
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

func listPost(page *Page, db Storage, types []string) func(c echo.Context) error {
	return func(c echo.Context) error {
		var err error

		values, errV := c.FormParams()
		years, errY := yearValues(values)
		if err = errors.Join(errV, errY); err != nil {
			log.Fatal(err)
		}
		slog.Info("POST /list", "values", values)
		page.List.Form.Years = years
		page.List.Data.Headers, page.List.Data.Rows, err = stats.List(db, types, page.List.Form.Workouts, selectedYears(years))
		return errors.Join(err, c.Render(200, "list-data", page.List.Data))
	}
}
