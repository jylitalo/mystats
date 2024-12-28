package server

import (
	"context"
	"database/sql"
	"errors"
	"html/template"
	"io"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
)

type Storage interface {
	QueryBestEffort(fields []string, name string, order *storage.Order) (*sql.Rows, error)
	QueryBestEffortDistances() ([]string, error)
	QuerySplit(fields []string, id int64) (*sql.Rows, error)
	QuerySteps(fields []string, cond storage.SummaryConditions, order *storage.Order) (*sql.Rows, error)
	QuerySummary(fields []string, cond storage.SummaryConditions, order *storage.Order) (*sql.Rows, error)
	QueryTypes(cond storage.SummaryConditions) ([]string, error)
	QueryWorkoutTypes(cond storage.SummaryConditions) ([]string, error)
	QueryYears(cond storage.SummaryConditions) ([]int, error)
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
	Plot  *PlotPage
	Best  *BestPage
	List  *ListPage
	Top   *TopPage
	Steps *StepsPage
}

func newPage() *Page {
	return &Page{
		Plot:  newPlotPage(),
		Best:  newBestPage(),
		List:  newListPage(),
		Top:   newTopPage(),
		Steps: newStepsPage(),
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
		"esc": func(s string) string {
			return strings.ReplaceAll(strings.ReplaceAll(s, " ", "_"), "/", "X")
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
		tmpl: template.Must(template.New("index").Funcs(funcMap).ParseGlob(path)),
	}
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.tmpl.ExecuteTemplate(w, name, data)
}

func selectedTypes(types map[string]bool) []string {
	checked := []string{}
	for k, v := range types {
		if v {
			checked = append(checked, k)
		}
	}
	return checked
}

func selectedWorkoutTypes(types map[string]bool) []string {
	checked := []string{}
	for k, v := range types {
		if v {
			checked = append(checked, k)
		}
	}
	return checked
}

func selectedYears(years map[int]bool) []int {
	checked := []int{}
	for k, v := range years {
		if v {
			checked = append(checked, k)
		}
	}
	slices.Sort(checked)
	return checked
}

func typeValues(values url.Values) (map[string]bool, error) {
	if values == nil {
		return nil, errors.New("no type values given")
	}
	types := map[string]bool{}
	for k, v := range values {
		if strings.HasPrefix(k, "type_") {
			tv := strings.ReplaceAll(k[5:], "_", " ")
			types[tv] = (len(tv) > 0 && v[0] == "on")
		}
	}
	return types, nil
}

func workoutTypeValues(values url.Values) (map[string]bool, error) {
	if values == nil {
		return nil, errors.New("no workoutType values given")
	}
	types := map[string]bool{}
	for k, v := range values {
		if strings.HasPrefix(k, "wt_") {
			tv := strings.ReplaceAll(k[3:], "_", " ")
			types[tv] = (len(tv) > 0 && v[0] == "on")
		}
	}
	return types, nil
}

func yearValues(values url.Values) (map[int]bool, error) {
	if values == nil {
		return nil, errors.New("no year values given")
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
	// slog.Info("server.yearValues", "years", years)
	return years, nil
}

func Start(ctx context.Context, db Storage, selectedTypes []string, port int) error {
	_, span := telemetry.NewSpan(ctx, "server.start")
	e := echo.New()
	e.Renderer = newTemplate("server/views/*.html")
	e.Use(middleware.Logger())
	e.Static("/css", "server/css")

	page := newPage()
	types, errT := db.QueryTypes(storage.SummaryConditions{})
	workoutTypes, errW := db.QueryWorkoutTypes(storage.SummaryConditions{})
	years, errY := db.QueryYears(storage.SummaryConditions{})
	bestEfforts, errBE := db.QueryBestEffortDistances()
	if err := errors.Join(errT, errW, errY, errBE); err != nil {
		return telemetry.Error(span, err)
	}
	// it is faster to first mark everything false and afterwards change selected one to true,
	// instead of going through all types and checking on every type, if it is contained in selectedTypes or not.
	for _, t := range types {
		page.List.Form.Types[t] = false
		page.Plot.Form.Types[t] = false
		page.Top.Form.Types[t] = false
	}
	for _, t := range selectedTypes {
		page.List.Form.Types[t] = true
		page.Plot.Form.Types[t] = true
		page.Top.Form.Types[t] = true
	}
	for _, t := range workoutTypes {
		page.List.Form.WorkoutTypes[t] = true
		page.Plot.Form.WorkoutTypes[t] = true
		page.Top.Form.WorkoutTypes[t] = true
	}
	currentYear := time.Now().Year()
	for _, y := range years {
		page.List.Form.Years[y] = y == currentYear
		page.Plot.Form.Years[y] = true
		page.Top.Form.Years[y] = true
		page.Steps.Form.Years[y] = true
	}
	value := true
	page.Best.Form.InOrder = bestEfforts
	for _, be := range bestEfforts {
		page.Best.Form.Distances[be] = value
		value = false
	}
	span.End()

	e.GET("/", indexGet(ctx, page, db))
	e.POST("/best", bestPost(ctx, page, db))
	e.POST("/event", listEvent(ctx, page, db))
	e.POST("/list", listPost(ctx, page, db))
	e.POST("/plot", plotPost(ctx, page, db))
	e.POST("/top", topPost(ctx, page, db))
	e.POST("/steps", stepsPost(ctx, page, db))
	e.Logger.Fatal(e.Start(":" + strconv.FormatInt(int64(port), 10)))
	return nil
}

func indexGet(ctx context.Context, page *Page, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		var errL, errT error

		ctx, span := telemetry.NewSpan(ctx, "indexGET")
		defer span.End()
		pf := &page.Plot.Form
		errP := page.Plot.render(
			ctx, db, pf.Types, pf.WorkoutTypes, pf.EndMonth, pf.EndDay, pf.Years, pf.Period,
		)
		// init List tab
		types := selectedTypes(pf.Types)
		workoutTypes := selectedWorkoutTypes(pf.WorkoutTypes)
		years := selectedYears(pf.Years)
		sp := &page.Steps.Form
		errS := page.Steps.render(ctx, db, sp.EndMonth, sp.EndDay, sp.Years, sp.Period)
		plfYears := selectedYears(page.List.Form.Years)
		pld := &page.List.Data
		pld.Headers, pld.Rows, errL = stats.List(ctx, db, types, workoutTypes, plfYears, page.List.Form.Limit, "")
		// init Top tab
		tf := &page.Top.Form
		td := &page.Top.Data
		td.Headers, td.Rows, errT = stats.Top(ctx, db, tf.Measure, tf.Period, types, workoutTypes, tf.Limit, years)
		// init Best tab
		for _, be := range selectedBestEfforts(page.Best.Form.Distances) {
			if headers, rows, err := stats.Best(ctx, db, be, 10); err != nil {
				return telemetry.Error(span, err)
			} else {
				page.Best.Data.Data = append(page.Best.Data.Data, TableData{Headers: headers, Rows: rows})
			}
		}
		return telemetry.Error(span, errors.Join(errP, errL, errT, errS, c.Render(200, "index", page)))
	}
}
