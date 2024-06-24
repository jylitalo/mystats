package server

import (
	"errors"
	"log"
	"log/slog"

	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/labstack/echo/v4"
)

type TopFormData struct {
	Years       map[int]bool
	measurement string
	period      string
	limit       int
}

func newTopFormData() TopFormData {
	return TopFormData{
		Years:       map[int]bool{},
		measurement: "sum(distance)",
		period:      "week",
		limit:       100,
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

func topPost(page *Page, db Storage, types []string) func(c echo.Context) error {
	return func(c echo.Context) error {
		var err error

		values, errV := c.FormParams()
		years, errY := yearValues(values)
		if err = errors.Join(errV, errY); err != nil {
			log.Fatal(err)
		}
		slog.Info("POST /top", "values", values)
		tf := &page.Top.Form
		tf.Years = years
		td := &page.Top.Data
		td.Headers, td.Rows, err = stats.Top(db, tf.measurement, tf.period, types, tf.limit, selectedYears(years))
		return errors.Join(err, c.Render(200, "top-data", td))
	}
}
