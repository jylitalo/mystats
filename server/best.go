package server

import (
	"context"
	"errors"
	"log"
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

func newBestFormData() BestFormData {
	return BestFormData{
		Distances: map[string]bool{},
		InOrder:   []string{},
		Limit:     10,
	}
}

type BestData struct {
	Data []TableData
}

func newBestData() BestData {
	return BestData{
		Data: []TableData{},
	}
}

type BestPage struct {
	Form BestFormData
	Data BestData
}

func newBestPage() *BestPage {
	return &BestPage{
		Form: newBestFormData(),
		Data: newBestData(),
	}
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

func bestPost(ctx context.Context, page *Page, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		_, span := telemetry.NewSpan(ctx, "bestPost")
		defer span.End()
		values, errV := c.FormParams()
		bestEfforts, errB := bestEffortValues(values)
		limit, errL := strconv.Atoi(c.FormValue("limit"))
		if err := errors.Join(errV, errB, errL); err != nil {
			log.Fatal(err)
		}
		slog.Info("POST /best", "values", values)
		page.Best.Form.Distances = bestEfforts
		page.Best.Data = newBestData()
		selected := selectedBestEfforts(bestEfforts)
		for _, be := range page.Best.Form.InOrder {
			if !slices.Contains[[]string, string](selected, be) {
				continue
			}
			headers, rows, err := stats.Best(ctx, db, be, limit)
			if err != nil {
				return err
			}
			page.Best.Data.Data = append(page.Best.Data.Data, TableData{
				Headers: headers,
				Rows:    rows,
			})
		}
		return c.Render(200, "best-data", page.Best.Data)
	}
}
