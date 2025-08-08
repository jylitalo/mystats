package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/jylitalo/mystats/pkg/telemetry"
)

type TopFormData struct {
	Name           string
	Measure        string
	MeasureOptions []string
	Period         string
	PeriodOptions  []string
	Sports         map[string]bool
	Workouts       map[string]bool
	Years          map[int]bool
	Limit          int
}

func newTopFormData(years []int, sports, workouts map[string]bool) TopFormData {
	yearSelection := map[int]bool{}
	for _, y := range years {
		yearSelection[y] = true
	}
	return TopFormData{
		Name:           "top",
		Measure:        "distance",
		MeasureOptions: []string{"distance", "elevation", "time"},
		Period:         "week",
		PeriodOptions:  []string{"week", "month"},
		Sports:         sports,
		Workouts:       workouts,
		Years:          yearSelection,
		Limit:          10,
	}
}

type topStatsFn func(
	ctx context.Context, db stats.Storage, measure, period string, sports, workouts []string,
	limit int, years []int,
) ([]string, [][]string, error)

type TopData struct {
	Measure string
	Period  string
	stats   topStatsFn
	TableData
}

func newTopData(
	ctx context.Context, db Storage, measure, period string,
	sports, workouts []string, limit int, years []int, stats topStatsFn,
) (*TopData, error) {
	var err error
	data := &TopData{
		Measure: measure,
		Period:  period,
		stats:   stats,
	}
	data.Headers, data.Rows, err = data.stats(ctx, db, measure, period, sports, workouts, limit, years)
	return data, err
}

type TopPage struct {
	Form TopFormData
	Data *TopData
}

func newTopPage(
	ctx context.Context, db Storage, years []int,
	sports, workouts map[string]bool, stats topStatsFn,
) (*TopPage, error) {
	form := newTopFormData(years, sports, workouts)
	data, err := newTopData(
		ctx, db, form.Measure, form.Period, selectedSports(sports),
		selectedWorkouts(workouts), form.Limit, years, stats,
	)
	return &TopPage{Form: form, Data: data}, err
}

func topPost(ctx context.Context, renderer *Template, page *TopPage, db Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err, errL, errS, errW, errY error

		ctx, span := telemetry.NewSpan(ctx, "topPOST")
		defer span.End()
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			_ = telemetry.Error(span, err)
			return
		}
		values := r.Form
		slog.Info("POST /top", "values", values)
		page.Form.Sports, errS = sportsValues(values)
		page.Form.Workouts, errW = workoutsValues(values)
		page.Form.Years, errY = yearValues(values)
		page.Form.Limit, errL = strconv.Atoi(r.FormValue("limit"))
		if err = errors.Join(err, errS, errW, errY, errL); err != nil {
			_ = telemetry.Error(span, err)
			return
		}
		page.Form.Measure = r.FormValue("Measure")
		page.Form.Period = r.FormValue("Period")
		page.Data, err = newTopData(
			ctx, db, page.Form.Measure, page.Form.Period,
			selectedSports(page.Form.Sports),
			selectedWorkouts(page.Form.Workouts), page.Form.Limit,
			selectedYears(page.Form.Years),
			page.Data.stats,
		)
		if err != nil {
			_ = telemetry.Error(span, err)
			return
		}
		if err := renderer.tmpl.ExecuteTemplate(w, "top-data", page.Data); err != nil {
			_ = telemetry.Error(span, err)
			http.Error(w, "Template rendering failed", http.StatusInternalServerError)
			return
		}
	}
}
