package server

import (
	"database/sql"
	"errors"
	"html/template"
	"io"
	"log"
	"log/slog"
	"net/url"
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

type PlotData struct {
	Years       []int
	Measurement string
	Stats       [][]string
	Totals      []string
	Filename    string
	plot        func(db plot.Storage, types []string, measurement string, month, day int, years []int, filename string) error
	stats       func(db stats.Storage, measurement, period string, types []string, month, day int, years []int) ([]int, [][]string, []string, error)
}

func newPlotData() PlotData {
	return PlotData{
		Measurement: "sum(distance)",
		plot:        plot.Plot,
		stats:       stats.Stats,
	}
}

type PlotFormData struct {
	EndMonth int
	EndDay   int
	Years    map[int]bool
}

func newPlotFormData() PlotFormData {
	t := time.Now()
	return PlotFormData{
		EndMonth: int(t.Month()),
		EndDay:   t.Day(),
		Years:    map[int]bool{},
	}
}

type PlotPage struct {
	Data PlotData
	Form PlotFormData
}

func newPlotPage() *PlotPage {
	return &PlotPage{
		Data: newPlotData(),
		Form: newPlotFormData(),
	}
}

func (p *PlotPage) render(db Storage, types []string, month, day int, years map[int]bool) error {
	p.Form.EndMonth = month
	p.Form.EndDay = day
	p.Form.Years = years
	checked := selectedYears(years)
	d := &p.Data
	d.Filename = "cache/plot-" + uuid.NewString() + ".png"
	err := d.plot(db, types, "distance", month, day, checked, "server/"+d.Filename)
	if err != nil {
		slog.Error("failed to plot", "err", err)
		return err
	}
	d.Years, d.Stats, d.Totals, err = d.stats(db, d.Measurement, "month", types, month, day, checked)
	if err != nil {
		slog.Error("failed to calculate stats", "err", err)
	}
	return err
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

type ListFormData struct {
	Name     string
	Workouts []string
	Years    map[int]bool
}

func newListFormData() ListFormData {
	return ListFormData{
		Name:     "list",
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

type TopFormData struct {
	Name        string
	Years       map[int]bool
	measurement string
	period      string
	limit       int
}

func newTopFormData() TopFormData {
	return TopFormData{
		Name:        "top",
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

	e.GET("/", func(c echo.Context) error {
		var errL, errT error
		pf := &page.Plot.Form
		errP := page.Plot.render(db, types, pf.EndMonth, pf.EndDay, pf.Years)
		page.List.Data.Headers, page.List.Data.Rows, errL = stats.List(db, types, page.List.Form.Workouts, nil)
		tf := &page.Top.Form
		td := &page.Top.Data
		td.Headers, td.Rows, errT = stats.Top(db, tf.measurement, tf.period, types, tf.limit, nil)
		errR := c.Render(200, "index", page)
		if err := errors.Join(errP, errL, errT, errR); err != nil {
			log.Fatal(err)
		}
		return nil
	})

	e.POST("/list", func(c echo.Context) error {
		values, errV := c.FormParams()
		years, errY := yearValues(values)
		if err := errors.Join(errV, errY); err != nil {
			log.Fatal(err)
		}
		slog.Info("POST /list", "values", values)
		workouts := []string{}
		page.List.Form.Years = years
		page.List.Data.Headers, page.List.Data.Rows, err = stats.List(db, types, workouts, selectedYears(years))
		if err != nil {
			log.Fatal(err)
		}
		if err = c.Render(200, "list-data", page.List.Data); err != nil {
			log.Fatal(err)
		}
		return nil
	})

	e.POST("/top", func(c echo.Context) error {
		values, errV := c.FormParams()
		years, errY := yearValues(values)
		if err := errors.Join(errV, errY); err != nil {
			log.Fatal(err)
		}
		slog.Info("POST /top", "values", values)
		tf := &page.Top.Form
		tf.Years = years
		td := &page.Top.Data
		td.Headers, td.Rows, err = stats.Top(db, tf.measurement, tf.period, types, tf.limit, selectedYears(years))
		if err = c.Render(200, "top-data", td); err != nil {
			log.Fatal(err)
		}
		return nil
	})

	e.POST("/plot", func(c echo.Context) error {
		month, errM := strconv.Atoi(c.FormValue("EndMonth"))
		day, errD := strconv.Atoi(c.FormValue("EndDay"))
		values, errV := c.FormParams()
		years, errY := yearValues(values)
		if err := errors.Join(errM, errD, errV, errY); err != nil {
			log.Fatal(err)
		}
		slog.Info("POST /plot", "values", values)
		if err := page.Plot.render(db, types, month, day, years); err != nil {
			return err
		}
		// slog.Info("POST /plot", "page", page)
		return c.Render(200, "plot-data", page.Plot.Data)
	})
	e.Logger.Fatal(e.Start(":" + strconv.FormatInt(int64(port), 10)))
	return nil
}
