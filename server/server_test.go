package server

import (
	"bufio"
	"bytes"
	"database/sql"
	"testing"

	"github.com/jylitalo/mystats/pkg/plot"
	"github.com/jylitalo/mystats/pkg/stats"
	"github.com/jylitalo/mystats/storage"
)

type testDB struct{}

func (t *testDB) Query(fields []string, cond storage.Conditions, order *storage.Order) (*sql.Rows, error) {
	return nil, nil
}

func (t *testDB) QueryTypes(cond storage.Conditions) ([]string, error) {
	return nil, nil
}

func (t *testDB) QueryYears(cond storage.Conditions) ([]int, error) {
	return nil, nil
}

func TestTemplateRender(t *testing.T) {
	p := newPage()
	p.Plot.Data.plot = func(db plot.Storage, types []string, measurement string, month, day int, years []int, filename string) error {
		return nil
	}
	p.Plot.Data.stats = func(db stats.Storage, measurement, period string, types []string, month, day int, years []int) ([]int, [][]string, []string, error) {
		return nil, nil, nil, nil
	}
	err := p.Plot.render(&testDB{}, map[string]bool{"Run": true}, 6, 12, map[int]bool{2024: true})
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
