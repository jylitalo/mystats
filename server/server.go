package server

import (
	"database/sql"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/jylitalo/mystats/storage"
)

type Storage interface {
	Query(fields []string, cond storage.Conditions, order *storage.Order) (*sql.Rows, error)
	QueryYears(cond storage.Conditions) ([]int, error)
}

type TableData struct {
	Headers []string
	Rows    [][]string
}

func newTableData() TableData {
	return TableData{
		Headers: []string{},
		Rows:    [][]string{},
	}
}

type Page struct {
	Plot *PlotPage
	List *ListPage
	Top  *TopPage
}

func newPage() *Page {
	return &Page{
		Plot: newPlotPage(),
		List: newListPage(),
		Top:  newTopPage(),
	}
}

type Template struct {
	tmpl *template.Template
}

func newTemplate(path string) *Template {
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
		tmpl: template.Must(template.New("plot").Funcs(funcMap).ParseGlob(path)),
	}
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.tmpl.ExecuteTemplate(w, name, data)
}

func selectedYears(years map[int]bool) []int {
	checked := []int{}
	for k, v := range years {
		if v {
			checked = append(checked, k)
		}
	}
	return checked
}

func yearValues(values url.Values) (map[int]bool, error) {
	if values == nil {
		return nil, errors.New("no values given")
	}
	years := map[int]bool{}
	for k, v := range values {
		if strings.HasPrefix(k, "year_") {
			y, err := strconv.Atoi(k[5:])
			if err != nil {
				return nil, err
			}
			years[y] = (len(v) > 0 && v[0] == "on")
		}
	}
	return years, nil
}

func Start(db Storage, types []string, port int) error {
	var err error
	e := echo.New()
	e.Renderer = newTemplate("server/views/*.html")
	e.Use(middleware.Logger())
	e.Static("/cache", "server/cache")
	e.Static("/css", "server/css")

	page := newPage()
	years, err := db.QueryYears(storage.Conditions{})
	if err != nil {
		return err
	}
	for _, y := range years {
		page.List.Form.Years[y] = true
		page.Plot.Form.Years[y] = true
		page.Top.Form.Years[y] = true
	}
	slog.Info("starting things", "page", page)

	e.GET("/", indexGet(page, db, types))
	e.POST("/list", listPost(page, db, types))
	e.POST("/plot", plotPost(page, db, types))
	e.POST("/top", topPost(page, db, types))
	e.Logger.Fatal(e.Start(":" + strconv.FormatInt(int64(port), 10)))
	return nil
}

func indexGet(page *Page, db Storage, types []string) func(c echo.Context) error {
	return func(c echo.Context) error {
		var errL, errT error
		pf := &page.Plot.Form
		errP := page.Plot.render(db, types, pf.EndMonth, pf.EndDay, pf.Years)
		page.List.Data.Headers, page.List.Data.Rows, errL = stats.List(db, types, page.List.Form.Workouts, nil)
		tf := &page.Top.Form
		td := &page.Top.Data
		td.Headers, td.Rows, errT = stats.Top(db, tf.measurement, tf.period, types, tf.limit, nil)
		return errors.Join(errP, errL, errT, c.Render(200, "index", page))
	}
}
