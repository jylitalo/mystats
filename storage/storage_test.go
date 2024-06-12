package storage

import "testing"

func TestSqlQuery(t *testing.T) {
	values := []struct {
		name   string
		fields []string
		cond   Conditions
		order  *Order
		query  string
	}{
		{"none", []string{"field"}, Conditions{}, nil, "select field from mystats"},
		{"simple", []string{"field"}, Conditions{Types: []string{"Run"}}, nil, "select field from mystats where (type='Run')"},
		{
			"multi-field", []string{"f1", "f2"},
			Conditions{Types: []string{"r1", "r2"}},
			&Order{GroupBy: []string{"f3"}, OrderBy: []string{"f3 desc"}},
			"select f1,f2 from mystats where (type='r1' or type='r2') group by f3 order by f3 desc",
		},
		{
			"order", []string{"k1", "k2"}, Conditions{Types: []string{"c1"}, Workouts: []string{"c3"}},
			&Order{GroupBy: []string{"k3", "k4"}, OrderBy: []string{"k5", "k6"}, Limit: 7},
			"select k1,k2 from mystats where (workouttype='c3') and (type='c1') group by k3,k4 order by k5,k6 limit 7",
		},
		{"one_year", []string{"field"}, Conditions{Types: []string{"Run"}, Years: []int{2023}}, nil, "select field from mystats where (type='Run') and (year=2023)"},
		{"multiple_years", []string{"field"}, Conditions{Types: []string{"Run"}, Years: []int{2019, 2023}}, nil, "select field from mystats where (type='Run') and (year=2019 or year=2023)"},
	}
	for _, value := range values {
		t.Run(value.name, func(t *testing.T) {
			cmd := sqlQuery(value.fields, value.cond, value.order)
			if cmd != value.query {
				t.Errorf("mismatch got '%s' vs. expected '%s'", cmd, value.query)
			}
		})
	}
}
