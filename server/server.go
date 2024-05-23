package server

import (
	"html/template"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/jylitalo/mystats/pkg/plot"
	"github.com/jylitalo/mystats/pkg/stats"
)

type Data struct {
	Years       []int
	Measurement string
	Stats       [][]string
	Totals      []string
	Filename    string
}

type FormData struct {
	Month int
	Day   int
}

func newFormData() FormData {
	t := time.Now()
	return FormData{
		Month: int(t.Month()),
		Day:   t.Day(),
	}
}

type Page struct {
	Data Data
	Form FormData
}

func newPage() Page {
	return Page{
		Data: Data{},
		Form: newFormData(),
	}
}

type Template struct {
	tmpl *template.Template
}

func newTemplate() *Template {
	funcMap := template.FuncMap{
		"joined": func(s []string) string {
			return strings.TrimSpace(strings.Join(s, ""))
		},
		"month": func(i int) time.Month {
			return time.Month(i + 1)
		},
	}
	return &Template{
		tmpl: template.Must(template.New("plot").Funcs(funcMap).ParseGlob("server/views/*.html")),
	}
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.tmpl.ExecuteTemplate(w, name, data)
}

func Start(types []string, port int) {
	e := echo.New()
	e.Renderer = newTemplate()
	e.Use(middleware.Logger())
	e.Static("/cache", "server/cache")
	e.Static("/css", "server/css")

	page := newPage()
	slog.Info("starting things", "page", page)

	e.GET("/", func(c echo.Context) error {
		return c.Render(200, "index", page)
	})

	e.POST("/plot", func(c echo.Context) error {
		month, _ := strconv.Atoi(c.FormValue("month"))
		day, _ := strconv.Atoi(c.FormValue("day"))
		page.Data.Filename = "cache/plot-" + uuid.NewString() + ".png"
		err := plot.Plot(types, "distance", month, day, "server/"+page.Data.Filename)
		if err != nil {
			slog.Error("failed to plot", "err", err)
			return err
		}
		measurement := "sum(distance)"
		page.Data.Measurement = measurement
		page.Data.Years, page.Data.Stats, page.Data.Totals, err = stats.Stats(measurement, "month", types, month, day)
		if err != nil {
			slog.Error("failed to calculate stats", "err", err)
			return err
		}
		return c.Render(200, "data", page.Data)
	})
	e.Logger.Fatal(e.Start(":" + strconv.FormatInt(int64(port), 10)))
}
