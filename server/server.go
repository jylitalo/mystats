package server

import (
	"context"
	"database/sql"
	"errors"
	"html/template"
	"io"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
)

type Storage interface {
	QueryBestEffortDistances() ([]string, error)
	QuerySports() ([]string, error)
	QueryWorkouts() ([]string, error)
	QueryYears(opts ...storage.QueryOption) ([]int, error)
	Query(fields []string, opts ...storage.QueryOption) (*sql.Rows, error)
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
	Best  *BestPage
	List  *ListPage
	Plot  *PlotPage
	Steps *StepsPage
	Top   *TopPage
}

func newPage(ctx context.Context, db Storage, selectedTypes []string) (*Page, error) {
	ctx, span := telemetry.NewSpan(ctx, "server.newPage")
	defer span.End()
	allTypes, err := db.QuerySports()
	if err != nil {
		return nil, telemetry.Error(span, err)
	}
	types := map[string]bool{}
	for _, t := range allTypes {
		types[t] = false
	}
	for _, t := range selectedTypes {
		if _, ok := types[t]; ok {
			types[t] = true
		}
	}
	workoutTypes, errW := db.QueryWorkouts()
	selectedWT := map[string]bool{}
	for _, wt := range workoutTypes {
		selectedWT[wt] = true
	}
	dailyStepsYears, errD := db.QueryYears(storage.WithTable(storage.DailyStepsTable))
	stravaYears, errS := db.QueryYears()
	be, errBE := newBestPage(ctx, db)
	list, errL := newListPage(ctx, db, stravaYears, maps.Clone(types), maps.Clone(selectedWT))
	steps, errS := newStepsPage(ctx, db, dailyStepsYears)
	top, errTop := newTopPage(
		ctx, db, stravaYears, maps.Clone(types), maps.Clone(selectedWT),
	)
	if err := errors.Join(errW, errD, errS, errBE, errL, errS, errTop); err != nil {
		return nil, err
	}
	return &Page{
		Best:  be,
		List:  list,
		Plot:  newPlotPage(stravaYears, maps.Clone(types), maps.Clone(selectedWT)),
		Steps: steps,
		Top:   top,
	}, nil
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
	ctx, span := telemetry.NewSpan(ctx, "server.start")
	defer span.End()
	e := echo.New()
	e.Renderer = newTemplate("server/views/*.html")
	e.Use(middleware.Logger())
	e.Static("/css", "server/css")

	page, err := newPage(ctx, db, selectedTypes)
	if err != nil {
		return err
	}
	e.GET("/", indexGet(ctx, page.Plot, db))
	e.POST("/best", bestPost(ctx, page.Best, db))
	e.POST("/event", listEvent(ctx, page.List, db))
	e.POST("/list", listPost(ctx, page.List, db))
	e.POST("/plot", plotPost(ctx, page.Plot, db))
	e.POST("/top", topPost(ctx, page.Top, db))
	e.POST("/steps", stepsPost(ctx, page.Steps, db))
	e.Logger.Fatal(e.Start(":" + strconv.FormatInt(int64(port), 10)))
	return nil
}

func indexGet(ctx context.Context, page *PlotPage, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		ctx, span := telemetry.NewSpan(ctx, "indexGET")
		defer span.End()
		pf := &page.Form
		err := page.render(
			ctx, db, pf.Types, pf.WorkoutTypes, pf.EndMonth, pf.EndDay, pf.Years, pf.Period,
		)
		return telemetry.Error(span, errors.Join(err, c.Render(200, "index", page)))
	}
}
