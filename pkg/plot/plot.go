package plot

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jylitalo/mystats/storage"
	"go-hep.org/x/hep/hplot"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

type Storage interface {
	QuerySummary(fields []string, cond storage.SummaryConditions, order *storage.Order) (*sql.Rows, error)
	QueryYears(cond storage.SummaryConditions) ([]int, error)
}

func Plot(db Storage, types, workoutTypes []string, measurement string, month, day int, years []int, filename string) error {
	tz, _ := time.LoadLocation("Europe/Helsinki")
	cond := storage.SummaryConditions{
		Types: types, WorkoutTypes: workoutTypes, Month: month, Day: day, Years: years,
	}
	years, err := db.QueryYears(cond)
	if err != nil {
		return err
	}
	xs := map[int][]float64{}
	ys := map[int][]float64{}
	totals := map[int]float64{}
	for _, year := range years {
		xs[year] = []float64{}
		ys[year] = []float64{}
		totals[year] = 0
	}
	o := []string{"year", "month", "day"}
	rows, err := db.QuerySummary(
		[]string{"year", "month", "day", "sum(" + measurement + ")"},
		cond, &storage.Order{GroupBy: o, OrderBy: o},
	)
	if err != nil {
		return fmt.Errorf("select caused: %w", err)
	}
	defer rows.Close()
	var xmax float64
	for rows.Next() {
		var year, month, day int
		var value float64
		err = rows.Scan(&year, &month, &day, &value)
		if err != nil {
			return err
		}
		if measurement == "distance" {
			value = value / 1000
		}
		totals[year] = totals[year] + value
		day1 := time.Date(year, time.January, 1, 6, 0, 0, 0, tz)
		now := time.Date(year, time.Month(month), day, 6, 0, 0, 0, tz)
		days := now.Sub(day1).Hours() / 24
		xmax = max(xmax, days)
		xs[year] = append(xs[year], days)
		ys[year] = append(ys[year], totals[year])
	}
	p := plot.New()
	p.Title.Text = fmt.Sprintf("year to day (month=%d, day=%d)", month, day)
	p.X.Label.Text = "days"
	p.Y.Label.Text = "distance"
	p.X.Min = 0
	p.X.Max = xmax
	p.Y.Min = 0
	yearLines := []interface{}{}
	for _, year := range years {
		yearLines = append(yearLines, strconv.FormatInt(int64(year), 10), hplot.ZipXY(xs[year], ys[year]))
	}
	err = plotutil.AddLines(p, yearLines...)
	if err != nil {
		return errors.New("failed to plot years")
	}
	err = p.Save(40*vg.Centimeter, 20*vg.Centimeter, filename)
	if err != nil {
		return errors.New("failed to save image")
	}
	slog.Info("Plat created", "filename", filename)
	return nil
}
