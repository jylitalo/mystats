package server

import (
	"database/sql"
	"errors"
	"html/template"
	"io"
	"log"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/jylitalo/mystats/pkg/plot"
	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/jylitalo/mystats/storage"
)

type Storage interface {
	Query(fields []string, cond storage.Conditions, order *storage.Order) (*sql.Rows, error)
	QueryYears(cond storage.Conditions) ([]int, error)
}

type Data struct {
	Years       []int
	Measurement string
	Stats       [][]string
	Totals      []string
	Filename    string
}

type FormData struct {
	EndMonth int
	EndDay   int
	Years    map[int]bool
}

func newFormData() FormData {
	t := time.Now()
	return FormData{
		EndMonth: int(t.Month()),
		EndDay:   t.Day(),
		Years:    map[int]bool{},
	}
}

type Page struct {
	Data Data
	Form FormData
}

func newPage() *Page {
	return &Page{
		Data: Data{Measurement: "sum(distance)"},
		Form: newFormData(),
	}
}

type Template struct {
	tmpl *template.Template
}

func newTemplate() *Template {
	funcMap := template.FuncMap{
		"N": func(start, end int) (stream chan int) {
			stream = make(chan int)
			go func() {
				for i := start; i < end; i++ {
					stream <- i
				}
				close(stream)
			}()
			return
		},
		"dec": func(i int) int {
			return i - 1
		},
		"inc": func(i int) int {
			return i + 1
		},
		"joined": func(s []string) string {
			return strings.TrimSpace(strings.Join(s, ""))
		},
		"month": func(i int) time.Month {
			return time.Month(i)
		},
	}
	return &Template{
		tmpl: template.Must(template.New("plot").Funcs(funcMap).ParseGlob("server/views/*.html")),
	}
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.tmpl.ExecuteTemplate(w, name, data)
}

func render(db Storage, page Page, types []string, month, day int, years map[int]bool) (*Page, error) {
	page.Form.EndMonth = month
	page.Form.EndDay = day
	page.Form.Years = years
	page.Data.Filename = "cache/plot-" + uuid.NewString() + ".png"
	checked := []int{}
	for k, v := range years {
		if v {
			checked = append(checked, k)
		}
	}
	err := plot.Plot(db, types, "distance", month, day, checked, "server/"+page.Data.Filename)
	if err != nil {
		slog.Error("failed to plot", "err", err)
		return nil, err
	}
	page.Data.Years, page.Data.Stats, page.Data.Totals, err = stats.Stats(db, page.Data.Measurement, "month", types, month, day, checked)
	if err != nil {
		slog.Error("failed to calculate stats", "err", err)
		return nil, err
	}
	return &page, nil
}

func Start(db Storage, types []string, port int) error {
	var err error
	e := echo.New()
	e.Renderer = newTemplate()
	e.Use(middleware.Logger())
	e.Static("/cache", "server/cache")
	e.Static("/css", "server/css")

	page := newPage()
	page.Data.Measurement = "sum(distance)"
	years, err := db.QueryYears(storage.Conditions{})
	if err != nil {
		return err
	}
	for _, y := range years {
		page.Form.Years[y] = true
	}
	slog.Info("starting things", "page", page)

	e.GET("/", func(c echo.Context) error {
		respPage, err := render(db, *page, types, page.Form.EndMonth, page.Form.EndDay, page.Form.Years)
		if err != nil {
			log.Fatal(err)
		}
		page = respPage
		// slog.Info("GET /", "page", page)
		if err = c.Render(200, "index", page); err != nil {
			log.Fatal(err)
		}
		return nil
	})

	e.POST("/plot", func(c echo.Context) error {
		month, errM := strconv.Atoi(c.FormValue("EndMonth"))
		day, errD := strconv.Atoi(c.FormValue("EndDay"))
		values, errV := c.FormParams()
		if err := errors.Join(errM, errD, errV); err != nil {
			log.Fatal(err)
		}
		slog.Info("POST", "values", values)
		years := map[int]bool{}
		for k, v := range values {
			if strings.HasPrefix(k, "year_") {
				y, err := strconv.Atoi(k[5:])
				if err != nil {
					return err
				}
				years[y] = (len(v) > 0 && v[0] == "on")
			}
		}
		respPage, err := render(db, *page, types, month, day, years)
		if err != nil {
			return err
		}
		page = respPage
		// slog.Info("POST /plot", "page", page)
		return c.Render(200, "data", page.Data)
	})
	e.Logger.Fatal(e.Start(":" + strconv.FormatInt(int64(port), 10)))
	return nil
}
