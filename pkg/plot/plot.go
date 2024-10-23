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
	"gonum.org/v1/plot/text"
	"gonum.org/v1/plot/vg"
)

type Storage interface {
	QuerySummary(fields []string, cond storage.SummaryConditions, order *storage.Order) (*sql.Rows, error)
	QueryYears(cond storage.SummaryConditions) ([]int, error)
}

type Numbers struct {
	Xs     map[int][]int
	Ys     map[int][]float64
	totals map[int]float64
	xmax   int
}

func scan(rows *sql.Rows, years []int, measure string) (*Numbers, error) {
	var modifier float64
	var xmax int

	tz, _ := time.LoadLocation("Europe/Helsinki")
	xs := map[int][]int{}
	ys := map[int][]float64{}
	totals := map[int]float64{}
	for _, year := range years {
		xs[year] = []int{}
		ys[year] = []float64{}
		totals[year] = 0
	}
	modifier = 1
	if measure == "distance" {
		modifier = 1000
	}
	for rows.Next() {
		var year, month, day int
		var value float64
		err := rows.Scan(&year, &month, &day, &value)
		if err != nil {
			return nil, err
		}
		value = value / modifier
		totals[year] = totals[year] + value
		day1 := time.Date(year, time.January, 1, 6, 0, 0, 0, tz)
		now := time.Date(year, time.Month(month), day, 6, 0, 0, 0, tz)
		days := int(now.Sub(day1).Hours() / 24)
		if len(ys[year]) == 0 {
			for x := range days - 1 {
				xs[year] = append(xs[year], x)
				ys[year] = append(ys[year], 0)
			}
		} else {
			y := ys[year][len(ys[year])-1]
			for x := len(xs[year]); x < days-1; x++ {
				xs[year] = append(xs[year], x)
				ys[year] = append(ys[year], y)
			}
		}
		xmax = max(xmax, days)
		xs[year] = append(xs[year], days)
		ys[year] = append(ys[year], totals[year])
	}
	for _, year := range years {
		idx := len(xs[year]) - 1
		if idx > 0 && xs[year][idx] == xmax {
			continue
		}
		y := float64(0)
		if len(ys[year]) > 0 {
			y = ys[year][len(ys[year])-1]
		}
		for x := len(xs[year]); x < xmax; x++ {
			xs[year] = append(xs[year], x)
			ys[year] = append(ys[year], y)
		}
	}
	return &Numbers{Xs: xs, Ys: ys, totals: totals, xmax: xmax}, nil
}

func GetNumbers(db Storage, types, workoutTypes []string, measure string, month, day int, years []int) (*Numbers, error) {
	cond := storage.SummaryConditions{
		Types: types, WorkoutTypes: workoutTypes, Month: month, Day: day, Years: years,
	}
	years, err := db.QueryYears(cond)
	if err != nil {
		return nil, err
	}
	o := []string{"year", "month", "day"}
	m := measure
	if m == "time" {
		m = "elapsedtime"
	}
	rows, err := db.QuerySummary(
		[]string{"year", "month", "day", "sum(" + m + ")"},
		cond, &storage.Order{GroupBy: o, OrderBy: o},
	)
	if err != nil {
		return nil, fmt.Errorf("select caused: %w", err)
	}
	defer rows.Close()
	return scan(rows, years, measure)
}

func Plot(db Storage, types, workoutTypes []string, measure string, month, day int, years []int, filename string) error {
	numbers, err := GetNumbers(db, types, workoutTypes, measure, month, day, years)
	if err != nil {
		return err
	}
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
	p.X.Max = float64(numbers.xmax)
	p.Y.Min = 0
	yearLines := []interface{}{}
	for _, year := range years {
		floatX := make([]float64, len(numbers.Xs[year]))
		for i := range len(numbers.Xs[year]) {
			floatX[i] = float64(numbers.Xs[year][i])
		}
		yearLines = append(yearLines, strconv.FormatInt(int64(year), 10), hplot.ZipXY(floatX, numbers.Ys[year]))
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
