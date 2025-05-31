package server

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/labstack/echo/v4"
)

type BestFormData struct {
	Distances map[string]bool
	InOrder   []string
	Limit     int
}

func newBestFormData(ctx context.Context, db Storage) (*BestFormData, error) {
	_, span := telemetry.NewSpan(ctx, "server.newBestFormData")
	defer span.End()
	inOrder, err := db.QueryBestEffortDistances()
	if err != nil {
		return nil, telemetry.Error(span, err)
	}
	distances := map[string]bool{}
	for _, d := range inOrder {
		distances[d] = false
	}
	if len(inOrder) > 0 {
		distances[inOrder[0]] = true
	}
	return &BestFormData{
		Distances: distances,
		InOrder:   inOrder,
		Limit:     10,
	}, nil
}

type bestStatsFn func(ctx context.Context, db stats.Storage, distance string, limit int) ([]string, [][]string, error)

type BestData struct {
	Data  []TableData
	stats bestStatsFn
}

func newBestData(ctx context.Context, db Storage, selected []string, limit int, stats bestStatsFn) (*BestData, error) {
	ctx, span := telemetry.NewSpan(ctx, "server.newBestData")
	defer span.End()
	data := []TableData{}
	if stats == nil {
		panic("stats is nil in server.newBestData()")
	}
	for _, distance := range selected {
		if headers, rows, err := stats(ctx, db, distance, limit); err != nil {
			return nil, telemetry.Error(span, err)
		} else {
			data = append(data, TableData{Headers: headers, Rows: rows})
		}
	}
	return &BestData{Data: data, stats: stats}, nil
}

type BestPage struct {
	Form *BestFormData
	Data *BestData
}

func newBestPage(ctx context.Context, db Storage, stats bestStatsFn) (*BestPage, error) {
	form, err := newBestFormData(ctx, db)
	if err != nil {
		return nil, err
	}
	data, err := newBestData(ctx, db, selectedBestEfforts(form.Distances), form.Limit, stats)
	if err != nil {
		return nil, err
	}
	return &BestPage{
		Form: form,
		Data: data,
	}, nil
}

func selectedBestEfforts(distances map[string]bool) []string {
	checked := []string{}
	for k, v := range distances {
		if v {
			checked = append(checked, k)
		}
	}
	return checked
}

func bestEffortValues(values url.Values) (map[string]bool, error) {
	if values == nil {
		return nil, errors.New("no bename values given")
	}
	bestEfforts := map[string]bool{}
	for k, v := range values {
		if strings.HasPrefix(k, "be_") {
			tv := strings.ReplaceAll(strings.ReplaceAll(k[3:], "_", " "), "X", "/")
			bestEfforts[tv] = (len(tv) > 0 && v[0] == "on")
		}
	}
	return bestEfforts, nil
}

func bestPost(ctx context.Context, page *BestPage, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		var err, errB, errL error

		ctx, span := telemetry.NewSpan(ctx, "bestPOST")
		defer span.End()
		values, err := c.FormParams()
		page.Form.Distances, errB = bestEffortValues(values)
		page.Form.Limit, errL = strconv.Atoi(c.FormValue("limit"))
		if err := errors.Join(err, errB, errL); err != nil {
			return telemetry.Error(span, err)
		}
		slog.Info("POST /best", "values", values)
		selected := selectedBestEfforts(page.Form.Distances)
		page.Data.Data = []TableData{}
		for _, distance := range page.Form.InOrder {
			if !slices.Contains(selected, distance) {
				continue
			}
			if headers, rows, err := page.Data.stats(ctx, db, distance, page.Form.Limit); err != nil {
				return telemetry.Error(span, err)
			} else {
				page.Data.Data = append(page.Data.Data, TableData{
					Headers: headers,
					Rows:    rows,
				})
			}
		}
		return telemetry.Error(span, c.Render(200, "best-data", page.Data))
	}
}
