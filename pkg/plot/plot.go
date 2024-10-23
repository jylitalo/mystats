package plot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/text"
	"gonum.org/v1/plot/vg"
)

type Numbers map[int][]float64
type Storage interface {
	QuerySummary(fields []string, cond storage.SummaryConditions, order *storage.Order) (*sql.Rows, error)
	QueryYears(cond storage.SummaryConditions) ([]int, error)
}

func scan(rows *sql.Rows, years []int) (Numbers, error) {
	tz, _ := time.LoadLocation("Europe/Helsinki")
	day1 := map[int]time.Time{}
	ys := map[int][]float64{}
	for _, year := range years {
		day1[year] = time.Date(year, time.January, 1, 6, 0, 0, 0, tz)
		ys[year] = []float64{}
	}
	xmax := 0
	for rows.Next() {
		var year, month, day int
		var value float64
		if err := rows.Scan(&year, &month, &day, &value); err != nil {
			return ys, err
		}
		now := time.Date(year, time.Month(month), day, 6, 0, 0, 0, tz)
		days := int(now.Sub(day1[year]).Hours() / 24)
		yslen := len(ys[year])
		y := float64(0)
		if yslen > 0 {
			y = ys[year][yslen-1]
		}
		for x := yslen; x < days-1; x++ {
			ys[year] = append(ys[year], y)
		}
		xmax = max(xmax, days)
		ys[year] = append(ys[year], y+value)
	}
	for _, year := range years {
		yslen := len(ys[year])
		y := float64(0)
		if yslen > 0 {
			y = ys[year][yslen-1]
		}
		for x := yslen; x < xmax; x++ {
			ys[year] = append(ys[year], y)
		}
	}
	return ys, nil
}

func GetNumbers(ctx context.Context, db Storage, types, workoutTypes []string, measure string,
	month, day int, years []int,
) (Numbers, error) {
	_, span := telemetry.NewSpan(ctx, "plot.GetNumbers")
	defer span.End()

	cond := storage.SummaryConditions{
		Types: types, WorkoutTypes: workoutTypes, Month: month, Day: day, Years: years,
	}
	years, err := db.QueryYears(cond)
	if err != nil {
		return nil, err
	}
	m := "sum(" + measure + ")"
	switch measure {
	case "time":
		m = "sum(elapsedtime)/3600"
	case "distance":
		m = "sum(distance)/1000"
	}
	o := []string{"year", "month", "day"}
	rows, err := db.QuerySummary(append(o, m), cond, &storage.Order{GroupBy: o, OrderBy: o})
	if err != nil {
		return nil, fmt.Errorf("select caused: %w", err)
	}
	defer rows.Close()
	return scan(rows, years)
}

func Plot(ctx context.Context, db Storage, types, workoutTypes []string, measure string,
	month, day int, years []int, filename string,
) error {
	_, span := telemetry.NewSpan(ctx, "plot.Plot")
	defer span.End()

	p := plot.New()
	p.X.Label.Text = "date"
	ticks := []plot.Tick{
		{Value: 0, Label: "January"},
		{Value: 31, Label: "February"},
		{Value: 59, Label: "March"},
		{Value: 90, Label: "April"},
		{Value: 121, Label: "May"},
		{Value: 152, Label: "June"},
		{Value: 182, Label: "July"},
		{Value: 213, Label: "August"},
		{Value: 244, Label: "September"},
		{Value: 274, Label: "October"},
		{Value: 305, Label: "November"},
		{Value: 335, Label: "December"},
	}
	p.X.Tick.Marker = plot.ConstantTicks(ticks)
	p.Title.Text = fmt.Sprintf("year to day (to %s %d)", ticks[month-1].Label, day)
	p.X.Tick.Label.XAlign = text.XLeft
	p.Y.Label.Text = measure
	p.X.Min = 0
	p.Y.Min = 0

	cond := storage.SummaryConditions{
		Types: types, WorkoutTypes: workoutTypes,
		Month: month, Day: day, Years: years,
	}
	yearLines := []interface{}{}
	m := measure
	if m == "time" {
		m = "elapsedtime"
	}
	f := []string{"year", "month", "day", "sum(" + m + ")"}
	o := []string{"year", "month", "day"}
	rows, err := db.QuerySummary(f, cond, &storage.Order{GroupBy: o, OrderBy: o})
	if err != nil {
		return fmt.Errorf("select caused: %w", err)
	}
	defer rows.Close()
	if err = plotutil.AddLines(p, yearLines...); err != nil {
		return telemetry.Error(span, errors.New("failed to plot years"))
	} else if err = p.Save(40*vg.Centimeter, 20*vg.Centimeter, filename); err != nil {
		return telemetry.Error(span, errors.New("failed to save image"))
	}
	return nil
}
