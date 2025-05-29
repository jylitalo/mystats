package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
	"github.com/labstack/echo/v4"
)

type ListFormData struct {
	Name     string
	Sports   map[string]bool
	Workouts map[string]bool
	Years    map[int]bool
	Limit    int
}

func newListFormData(years []int, sports, workouts map[string]bool) ListFormData {
	yearSelection := map[int]bool{}
	currentYear := time.Now().Year()
	for _, y := range years {
		yearSelection[y] = false
	}
	if _, ok := yearSelection[currentYear]; ok {
		yearSelection[currentYear] = true
	}
	return ListFormData{
		Name:     "list",
		Sports:   sports,
		Workouts: workouts,
		Years:    yearSelection,
		Limit:    1000,
	}
}

type ListEventData struct {
	Name string
	Date string
	TableData
}

type listStatsFn func(
	ctx context.Context, db stats.Storage, sports, workouts []string,
	years []int, limit int, name string) ([]string, [][]string, error)

type ListPage struct {
	Form  ListFormData
	Data  TableData
	Event ListEventData
	stats listStatsFn
}

func newListPage(
	ctx context.Context, db Storage, years []int,
	sports, workouts map[string]bool, stats listStatsFn,
) (*ListPage, error) {
	var err error

	form := newListFormData(years, sports, workouts)
	data := newTableData()
	data.Headers, data.Rows, err = stats(
		ctx, db, selectedSports(sports), selectedWorkouts(workouts),
		selectedYears(form.Years), form.Limit, "",
	)
	if err != nil {
		return nil, err
	}
	return &ListPage{
		Form: form,
		Data: data,
		Event: ListEventData{
			Name:      "",
			TableData: newTableData(),
		},
		stats: stats,
	}, err
}

func listPost(ctx context.Context, page *ListPage, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		var err error

		ctx, span := telemetry.NewSpan(ctx, "listPOST")
		defer span.End()

		values, errV := c.FormParams()
		sports, errT := sportsValues(values)
		workouts, errW := workoutsValues(values)
		years, errY := yearValues(values)
		limit, errL := strconv.Atoi(c.FormValue("limit"))
		name := c.FormValue("name")
		if err = errors.Join(errV, errT, errW, errY, errL); err != nil {
			slog.Error("server.listPost()", "err", err)
			_ = telemetry.Error(span, err)
		}
		slog.Info("POST /list", "values", values)
		page.Form.Years = years
		page.Data.Headers, page.Data.Rows, err = page.stats(
			ctx, db, selectedSports(sports), selectedWorkouts(workouts),
			selectedYears(years), limit, name,
		)
		return telemetry.Error(span, errors.Join(err, c.Render(200, "list-data", page.Data)))
	}
}

func listEvent(ctx context.Context, page *ListPage, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		ctx, span := telemetry.NewSpan(ctx, "eventPOST")
		defer span.End()

		id, err := strconv.Atoi(c.FormValue("id"))
		if err != nil {
			return telemetry.Error(span, err)
		}
		rows, err := db.Query(
			[]string{"name", "year", "month", "day"},
			storage.WithTable(storage.SummaryTable),
			storage.WithStravaID(int64(id)),
		)
		if err != nil {
			return telemetry.Error(span, err)
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			return fmt.Errorf("listEvent was unable to find activity %d", id)
		}
		var year, month, day int
		if err = rows.Scan(&page.Event.Name, &year, &month, &day); err != nil {
			return telemetry.Error(span, err)
		}
		page.Event.Date = fmt.Sprintf("%d.%d.%d", day, month, year)
		page.Event.Headers, page.Event.Rows, err = stats.Split(ctx, db, int64(id))
		return errors.Join(err, c.Render(200, "list-event", page.Event))
	}
}
