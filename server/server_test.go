package server

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"testing"

	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
)

type testDB struct{}

func (t *testDB) QueryBestEffortDistances() ([]string, error) {
	return nil, nil
}

func (t *testDB) Query(fields []string, opts ...storage.QueryOption) (*sql.Rows, error) {
	return nil, nil
}

func (t *testDB) QuerySports() ([]string, error) {
	return nil, nil
}

func (t *testDB) QueryWorkouts() ([]string, error) {
	return nil, nil
}

func (t *testDB) QueryYears(opts ...storage.QueryOption) ([]int, error) {
	return nil, nil
}

func TestTemplateRender(t *testing.T) {
	ctx, _, _ := telemetry.Setup(context.TODO(), "test")
	db := &testDB{}
	p, err := newPage(ctx, db, []string{"Race"})
	if err != nil {
		t.Error(err)
	}
	p.Plot.Data.stats = func(
		ctx context.Context, db stats.Storage, measurement, period string, types, workoutTypes []string,
		month, day int, years []int) ([]int, [][]string, []string, error,
	) {
		return nil, nil, nil, nil
	}
	err = p.Plot.render(ctx, &testDB{}, map[string]bool{"Run": true}, nil, 6, 12, map[int]bool{2024: true}, "month")
	if err != nil {
		t.Error(err)
	}
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	tmpl := newTemplate("views/*.html")
	err = tmpl.Render(w, "index", p, nil)
	if err != nil {
		t.Error(err)
	}
	err = tmpl.Render(w, "plot-data", p.Plot.Data, nil)
	if err != nil {
		t.Error(err)
	}
	err = tmpl.Render(w, "plot-form", p.Plot.Form, nil)
	if err != nil {
		t.Error(err)
	}
}
