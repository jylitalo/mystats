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

	"github.com/jylitalo/mystats/pkg/stats"
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
	Best      *BestPage
	HeartRate *HeartRatePage
	List      *ListPage
	Plot      *PlotPage
	Steps     *StepsPage
	Top       *TopPage
}

type pageConfig struct {
	bestStats  bestStatsFn
	listStats  listStatsFn
	plotStats  plotStatsFn
	stepsStats stepStatsFn
	topStats   topStatsFn
	sports     []string
}
type pageOptions func(po *pageConfig)

func newPage(ctx context.Context, db Storage, opts ...pageOptions) (*Page, error) {
	ctx, span := telemetry.NewSpan(ctx, "server.newPage")
	defer span.End()
	allSports, err := db.QuerySports()
	if err != nil {
		return nil, telemetry.Error(span, err)
	}
	sports := map[string]bool{}
	for _, s := range allSports {
		sports[s] = false
	}
	cfg := pageConfig{
		bestStats:  stats.Best,
		listStats:  stats.List,
		plotStats:  stats.Stats,
		stepsStats: stepsStats,
		topStats:   stats.Top,
		sports:     allSports,
	}
	for _, o := range opts {
		o(&cfg)
	}
	for _, t := range cfg.sports {
		if _, ok := sports[t]; ok {
			sports[t] = true
		}
	}
	workoutTypes, errW := db.QueryWorkouts()
	selectedWT := map[string]bool{}
	for _, wt := range workoutTypes {
		selectedWT[wt] = true
	}
	dailyStepsYears, errDS := db.QueryYears(storage.WithTable(storage.DailyStepsTable))
	heartRateYears, errHR := db.QueryYears(storage.WithTable(storage.HeartRateTable))
	if err := errors.Join(errDS, errHR); err != nil {
		return nil, err
	}
	stravaYears, errS := db.QueryYears()
	be, errBE := newBestPage(ctx, db, cfg.bestStats)
	hr, errHR := newHeartRatePage(ctx, db, heartRateYears)
	steps, errS := newStepsPage(ctx, db, dailyStepsYears, cfg.stepsStats)
	list, errL := newListPage(ctx, db, stravaYears, maps.Clone(sports), maps.Clone(selectedWT), cfg.listStats)
	plot, errP := newPlotPage(ctx, db, stravaYears, maps.Clone(sports), maps.Clone(selectedWT), cfg.plotStats)
	top, errTop := newTopPage(ctx, db, stravaYears, maps.Clone(sports), maps.Clone(selectedWT), cfg.topStats)
	if err := errors.Join(errW, errS, errBE, errHR, errL, errP, errS, errTop); err != nil {
		return nil, err
	}
	return &Page{
		Best:      be,
		HeartRate: hr,
		List:      list,
		Plot:      plot,
		Steps:     steps,
		Top:       top,
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
		"multiply": func(i, j int) int {
			return i * j
		},
	}
	return &Template{
		tmpl: template.Must(template.New("index").Funcs(funcMap).ParseGlob(path)),
	}
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.tmpl.ExecuteTemplate(w, name, data)
}

func selectedSports(sports map[string]bool) []string {
	checked := []string{}
	for k, v := range sports {
		if v {
			checked = append(checked, k)
		}
	}
	return checked
}

func selectedWorkouts(workouts map[string]bool) []string {
	checked := []string{}
	for k, v := range workouts {
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

func sportsValues(values url.Values) (map[string]bool, error) {
	if values == nil {
		return nil, errors.New("no type values given")
	}
	sports := map[string]bool{}
	for k, v := range values {
		if strings.HasPrefix(k, "sport_") {
			tv := strings.ReplaceAll(k[6:], "_", " ")
			sports[tv] = (len(tv) > 0 && v[0] == "on")
		}
	}
	return sports, nil
}

func workoutsValues(values url.Values) (map[string]bool, error) {
	if values == nil {
		return nil, errors.New("no workoutType values given")
	}
	sports := map[string]bool{}
	for k, v := range values {
		if strings.HasPrefix(k, "wt_") {
			tv := strings.ReplaceAll(k[3:], "_", " ")
			sports[tv] = (len(tv) > 0 && v[0] == "on")
		}
	}
	return sports, nil
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

func Start(ctx context.Context, db Storage, sports []string, port int) error {
	ctx, span := telemetry.NewSpan(ctx, "server.start")
	defer span.End()
	e := echo.New()
	e.Renderer = newTemplate("server/views/*.html")
	e.Use(middleware.Logger())
	e.Static("/css", "server/css")

	page, err := newPage(ctx, db, func(pc *pageConfig) { pc.sports = sports })
	if err != nil {
		return err
	}
	e.GET("/", indexGet(ctx, page))
	e.POST("/best", bestPost(ctx, page.Best, db))
	e.POST("/event", listEvent(ctx, page.List, db))
	e.POST("/heartrate", heartRatePost(ctx, page.HeartRate, db))
	e.POST("/list", listPost(ctx, page.List, db))
	e.POST("/plot", plotPost(ctx, page.Plot, db))
	e.POST("/top", topPost(ctx, page.Top, db))
	e.POST("/steps", stepsPost(ctx, page.Steps, db))
	e.Logger.Fatal(e.Start(":" + strconv.FormatInt(int64(port), 10)))
	return nil
}

func indexGet(ctx context.Context, page *Page) func(c echo.Context) error {
	return func(c echo.Context) error {
		_, span := telemetry.NewSpan(ctx, "indexGET")
		defer span.End()
		return telemetry.Error(span, c.Render(200, "index", page))
	}
}
