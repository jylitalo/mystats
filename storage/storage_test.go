package storage

import "testing"

func TestSqlQuery(t *testing.T) {
	values := []struct {
		name   string
		tables []string
		fields []string
		cond   conditions
		order  *Order
		query  string
	}{
		{
			"none", []string{"summary"}, []string{"field"}, conditions{}, nil,
			"select field from summary"},
		{
			"simple", []string{"summary"}, []string{"field"}, conditions{Types: []string{"Run"}}, nil,
			"select field from summary where (type='Run')",
		},
		{
			"multi-field", []string{"summary"}, []string{"f1", "f2"},
			conditions{Types: []string{"r1", "r2"}},
			&Order{GroupBy: []string{"f3"}, OrderBy: []string{"f3 desc"}},
			"select f1,f2 from summary where (type='r1' or type='r2') group by f3 order by f3 desc",
		},
		{
			"order", []string{"summary"}, []string{"k1", "k2"},
			conditions{Types: []string{"c1"}, WorkoutTypes: []string{"c3"}},
			&Order{GroupBy: []string{"k3", "k4"}, OrderBy: []string{"k5", "k6"}, Limit: 7},
			"select k1,k2 from summary where (workouttype='c3') and (type='c1') group by k3,k4 order by k5,k6 limit 7",
		},
		{
			"one_year", []string{"summary"}, []string{"field"},
			conditions{Types: []string{"Run"}, Years: []int{2023}}, nil,
			"select field from summary where (type='Run') and (year=2023)",
		},
		{
			"multiple_years", []string{"summary"}, []string{"field"},
			conditions{Types: []string{"Run"}, Years: []int{2019, 2023}}, nil,
			"select field from summary where (type='Run') and (year=2019 or year=2023)",
		},
		{
			"ids", []string{"summary"},
			[]string{"StravaID"},
			conditions{Types: []string{"Run"}},
			&Order{OrderBy: []string{"StravaID desc"}},
			"select StravaID from summary where (type='Run') order by StravaID desc",
		},
		{
			"besteffort", []string{"summary", "besteffort"}, []string{"summary.Name"}, conditions{BEName: "400m"}, nil,
			"select summary.Name from summary,besteffort where summary.StravaID=besteffort.StravaID and besteffort.name='400m'",
		},
	}
	for _, value := range values {
		t.Run(value.name, func(t *testing.T) {
			cmd := sqlQuery(value.tables, value.fields, value.cond, value.order)
			if cmd != value.query {
				t.Errorf("mismatch got '%s' vs. expected '%s'", cmd, value.query)
			}
		})
	}
}
