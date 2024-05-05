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
		{"simple", []string{"field"}, Conditions{Types: []string{"Run"}}, nil, "select field from mystats where (type='Run')"},
		{
			"multi-field", []string{"f1", "f2"}, Conditions{Types: []string{"r1", "r2"}}, &Order{Fields: []string{"f3"}},
			"select f1,f2 from mystats where (type='r1' or type='r2') group by f3 order by f3 desc",
		},
		{
			"order", []string{"k1", "k2"}, Conditions{Types: []string{"c1"}, Workouts: []string{"c3"}}, &Order{Fields: []string{"k3", "k4"}, Ascend: true},
			"select k1,k2 from mystats where (workouttype='c3') and (type='c1') group by k3,k4 order by k3,k4",
		},
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
