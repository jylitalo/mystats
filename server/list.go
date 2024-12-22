package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
	"github.com/labstack/echo/v4"
)

type ListFormData struct {
	Name         string
	Types        map[string]bool
	WorkoutTypes map[string]bool
	Years        map[int]bool
	Limit        int
}

func newListFormData() ListFormData {
	return ListFormData{
		Name:         "list",
		Types:        map[string]bool{},
		WorkoutTypes: map[string]bool{},
		Years:        map[int]bool{},
		Limit:        1000,
	}
}

type ListEventData struct {
	Name string
	Date string
	TableData
}

type ListPage struct {
	Form  ListFormData
	Data  TableData
	Event ListEventData
}

func newListPage() *ListPage {
	return &ListPage{
		Form: newListFormData(),
		Data: newTableData(),
		Event: ListEventData{
			Name:      "",
			TableData: newTableData(),
		},
	}
}

func listPost(ctx context.Context, page *Page, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		var err error

		ctx, span := telemetry.NewSpan(ctx, "listPOST")
		defer span.End()

		values, errV := c.FormParams()
		types, errT := typeValues(values)
		workoutTypes, errW := workoutTypeValues(values)
		years, errY := yearValues(values)
		limit, errL := strconv.Atoi(c.FormValue("limit"))
		name := c.FormValue("name")
		if err = errors.Join(errV, errT, errW, errY, errL); err != nil {
			slog.Error("server.listPost()", "err", err)
			_ = telemetry.Error(span, err)
		}
		slog.Info("POST /list", "values", values)
		page.List.Form.Years = years
		page.List.Data.Headers, page.List.Data.Rows, err = stats.List(
			ctx, db, selectedTypes(types), selectedWorkoutTypes(workoutTypes), selectedYears(years), limit, name,
		)
		return telemetry.Error(span, errors.Join(err, c.Render(200, "list-data", page.List.Data)))
	}
}

func listEvent(ctx context.Context, page *Page, db Storage) func(c echo.Context) error {
	return func(c echo.Context) error {
		ctx, span := telemetry.NewSpan(ctx, "eventPOST")
		defer span.End()

		id, err := strconv.Atoi(c.FormValue("id"))
		if err != nil {
			return telemetry.Error(span, err)
		}
		rows, err := db.QuerySummary([]string{"name", "year", "month", "day"}, storage.SummaryConditions{StravaID: int64(id)}, nil)
		if err != nil {
			return telemetry.Error(span, err)
		}
		defer rows.Close()
		if !rows.Next() {
			return fmt.Errorf("listEvent was unable to find activity %d", id)
		}
		var year, month, day int
		if err = rows.Scan(&page.List.Event.Name, &year, &month, &day); err != nil {
			return telemetry.Error(span, err)
		}
		page.List.Event.Date = fmt.Sprintf("%d.%d.%d", day, month, year)
		page.List.Event.Headers, page.List.Event.Rows, err = stats.Split(ctx, db, int64(id))
		return errors.Join(err, c.Render(200, "list-event", page.List.Event))
	}
}
