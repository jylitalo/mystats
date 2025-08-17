package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
)

type HeartRateFormData struct {
	Name     string
	EndMonth int
	EndDay   int
	Average  int
	Years    map[int]bool
}

func newHeartRateFormData(years []int) HeartRateFormData {
	yearSelection := map[int]bool{}
	for _, y := range years {
		yearSelection[y] = true
	}
	t := time.Now()
	return HeartRateFormData{
		Name:     "heartrate",
		EndMonth: int(t.Month()),
		EndDay:   t.Day(),
		Years:    yearSelection,
	}
}

type HeartRateData struct {
	Years         []int
	Stats         [][]string
	ScriptColumns []int
	ScriptRows    template.JS
	ScriptColors  template.JS
}

func newHeartRateData() HeartRateData {
	return HeartRateData{}
}

type HeartRatePage struct {
	Data HeartRateData
	Form HeartRateFormData
}

func newHeartRatePage(ctx context.Context, db Storage, years []int) (*HeartRatePage, error) {
	form := newHeartRateFormData(years)
	data := newHeartRateData()
	page := &HeartRatePage{Data: data, Form: form}
	return page, page.render(ctx, db, form.EndMonth, form.EndDay, form.Years, form.Average)
}

func average(values []float64) float64 {
	s := float64(0)
	for _, v := range values {
		s += v
	}
	return s / float64(len(values))
}

func (p *HeartRatePage) render(
	ctx context.Context, db Storage, month, day int, years map[int]bool, avg int,
) error {
	ctx, span := telemetry.NewSpan(ctx, "heartrate.render")
	defer span.End()

	colors := []string{
		"#0000ff", // 1
		"#00ff00", // 2
		"#ff0000", // 3
		"#00ffff", // 4
		"#ffff00", // 5
		"#ff00ff", // 6
		"#000088", // 7
		"#008800", // 8
		"#880000", // 9
		"#00f000",
		"#0000f0",
	}
	p.Form.EndMonth = month
	p.Form.EndDay = day
	p.Form.Years = years
	checkedYears := selectedYears(years)
	numbers, err := getHeartRate(ctx, db, month, day, checkedYears)
	if err != nil {
		slog.Error("failed to heartrate", "err", err)
		return err
	}
	foundYears := []int{}
	for _, year := range checkedYears {
		if _, ok := numbers[year]; ok {
			foundYears = append(foundYears, year)
		}
	}
	if len(foundYears) == 0 {
		slog.Error("No years found in heartrate.render()")
		return nil
	}
	refTime, err := time.Parse(time.DateOnly, fmt.Sprintf("%d-01-01", slices.Max(foundYears)))
	if err != nil {
		return err
	}
	scriptRows := [][]interface{}{}
	for day := range numbers[foundYears[0]] {
		scriptRows = append(scriptRows, make([]interface{}, len(foundYears)+1))
		index0 := refTime.Add(24 * time.Duration(day) * time.Hour)
		// Month in JavaScript's Date is 0-indexed
		newDate := fmt.Sprintf("new Date(%d, %d, %d)", index0.Year(), index0.Month()-1, index0.Day())
		scriptRows[day][0] = template.JS(newDate) // #nosec G203
		start := max(0, day-avg)                  // avg is reference to how many days average we use as measurement
		for idx, year := range foundYears {
			end := min(day+avg, len(numbers[year])-1)
			scriptRows[day][idx+1] = average(numbers[year][start : end+1])
		}
	}
	byteRows, _ := json.Marshal(scriptRows)
	byteColors, _ := json.Marshal(colors[0:len(foundYears)])
	p.Data.ScriptColumns = foundYears
	p.Data.ScriptRows = template.JS(strings.ReplaceAll(string(byteRows), `"`, ``)) // #nosec G203
	p.Data.ScriptColors = template.JS(byteColors)                                  // #nosec G203
	return err
}

func heartratePost(ctx context.Context, renderer *Template, page *HeartRatePage, db Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(ctx, "heartratePOST")
		defer span.End()
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			_ = telemetry.Error(span, err)
			return
		}

		month, errM := strconv.Atoi(r.FormValue("EndMonth"))
		day, errD := strconv.Atoi(r.FormValue("EndDay"))
		avg, errA := strconv.Atoi(r.FormValue("Average"))
		values := r.Form
		years, errY := yearValues(values)
		if err := errors.Join(errA, errM, errD, errY); err != nil {
			_ = telemetry.Error(span, err)
			return
		}
		if avg < 0 {
			avg = -avg
		}
		page.Form.Average = avg
		slog.Info("POST /heartrate", "values", values)
		err := page.render(ctx, db, month, day, years, avg)
		_ = telemetry.Error(span, err)
		if err := renderer.tmpl.ExecuteTemplate(w, "heartrate-data", page.Data); err != nil {
			_ = telemetry.Error(span, err)
			http.Error(w, "Template rendering failed", http.StatusInternalServerError)
		}
	}
}

// getHeartRate fetches year to date values from storage.
func getHeartRate(ctx context.Context, db Storage, month, day int, years []int) (numbers, error) {
	_, span := telemetry.NewSpan(ctx, "server.getHeartRate")
	defer span.End()

	years, rows, err := yearToDateQuery(ctx, db, day, month, years, storage.HeartRateTable, "RestingHR")
	if err != nil {
		return nil, telemetry.Error(span, err)
	}
	return absoluteScan(rows, years)
}

// absoluteScan tries to ensure that we have some value for each year-to-day for all years.
func absoluteScan(rows *sql.Rows, years []int) (numbers, error) {
	tz, _ := time.LoadLocation("Europe/Helsinki")
	day1 := map[int]time.Time{}
	// ys is map, where key is year and array has entry for each day of the year
	ys := map[int][]float64{}
	previous_y := map[int]float64{}
	for _, year := range years {
		day1[year] = time.Date(year, time.January, 1, 6, 0, 0, 0, tz)
		ys[year] = []float64{}
		previous_y[year] = 0
	}
	max_acts := 0
	if rows == nil {
		return ys, nil
	}
	for rows.Next() { // scan through database rows
		var year, month, day int
		var value float64
		if err := rows.Scan(&year, &month, &day, &value); err != nil {
			return ys, err
		}
		now := time.Date(year, time.Month(month), day, 6, 0, 0, 0, tz) // time when activity happened
		days := int(now.Sub(day1[year]).Hours()/24) + 1                // day within a year (1-365)
		if days > 366 {
			log.Fatalf(
				"days got impossible number %d (year=%d, month=%d, day=%d, now=%#v, day1=%#v)",
				days, year, month, day, now, day1[year],
			)
		}
		yslen := len(ys[year])
		for x := yslen; x < days-1; x++ { // fill the gaps on days that didn't have activities
			ys[year] = append(ys[year], previous_y[year])
		}
		ys[year] = append(ys[year], value)
		max_acts = max(max_acts, len(ys[year]))
		previous_y[year] = value
	}
	for _, year := range years { // fill the end of year
		yslen := len(ys[year])
		for x := yslen; x < max_acts; x++ {
			ys[year] = append(ys[year], previous_y[year])
		}
	}
	return ys, nil
}
