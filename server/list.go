package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
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

func listPost(ctx context.Context, renderer *Template, page *ListPage, db Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := telemetry.NewSpan(ctx, "listPOST")
		defer span.End()

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			_ = telemetry.Error(span, err)
			return
		}
		values := r.Form
		sports, errT := sportsValues(values)
		workouts, errW := workoutsValues(values)
		years, errY := yearValues(values)
		limit, errL := strconv.Atoi(r.FormValue("limit"))
		name := r.FormValue("name")
		if err = errors.Join(errT, errW, errY, errL); err != nil {
			http.Error(w, "Error with arguments", http.StatusBadRequest)
			slog.Error("server.listPost()", "err", err)
			_ = telemetry.Error(span, err)
		}
		slog.Info("POST /list", "values", values)
		page.Form.Years = years
		page.Data.Headers, page.Data.Rows, err = page.stats(
			ctx, db, selectedSports(sports), selectedWorkouts(workouts),
			selectedYears(years), limit, name,
		)
		if err != nil {
			http.Error(w, "Failed to build page", http.StatusInternalServerError)
			_ = telemetry.Error(span, err)
		}
		if err := renderer.tmpl.ExecuteTemplate(w, "list-data", page.Data); err != nil {
			http.Error(w, "Template rendering failed", http.StatusInternalServerError)
			_ = telemetry.Error(span, err)
		}
	}
}

func listEvent(ctx context.Context, renderer *Template, page *ListPage, db Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(ctx, "eventPOST")
		defer span.End()

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			_ = telemetry.Error(span, err)
			return
		}

		id, err := strconv.Atoi(r.FormValue("id"))
		if err != nil {
			_ = telemetry.Error(span, err)
			return
		}
		rows, err := db.Query(
			ctx,
			[]string{"name", "year", "month", "day"},
			storage.WithTable(storage.SummaryTable),
			storage.WithStravaID(int64(id)),
		)
		if err != nil {
			_ = telemetry.Error(span, err)
			return
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			http.Error(w, "Unable to find activity", http.StatusBadRequest)
			_ = telemetry.Error(span, fmt.Errorf("listEvent was unable to find activity %d", id))
			return
		}
		var year, month, day int
		if err = rows.Scan(&page.Event.Name, &year, &month, &day); err != nil {
			http.Error(w, "Unable to find activity", http.StatusBadRequest)
			_ = telemetry.Error(span, err)
			return
		}
		page.Event.Date = fmt.Sprintf("%d.%d.%d", day, month, year)
		page.Event.Headers, page.Event.Rows, err = stats.Split(ctx, db, int64(id))
		if err != nil {
			http.Error(w, "Failed to build page", http.StatusInternalServerError)
			_ = telemetry.Error(span, err)
		}
		if err := renderer.tmpl.ExecuteTemplate(w, "list-event", page.Event); err != nil {
			http.Error(w, "Template rendering failed", http.StatusInternalServerError)
			_ = telemetry.Error(span, err)
		}
	}
}
