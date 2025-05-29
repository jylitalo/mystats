package storage //nolint:testpackage

import (
	"fmt"
	"strings"
	"testing"
)

func TestSqlQuery(t *testing.T) { //nolint:funlen
	values := []struct {
		name    string
		fields  []string
		options []QueryOption
		query   string
		values  []string
	}{
		{
			name:    "none",
			fields:  []string{"field"},
			options: []QueryOption{WithTable(SummaryTable)},
			query:   "select field from Summary",
		},
		{
			name:    "simple",
			fields:  []string{"field"},
			options: []QueryOption{WithTable(SummaryTable), WithSports("Run")},
			query:   "select field from Summary where (Type=?)",
			values:  []string{"Run"},
		},
		{
			name:   "multi-field",
			fields: []string{"f1", "f2"},
			options: []QueryOption{
				WithTable(SummaryTable), WithSports("r1"), WithSports("r2"),
				WithOrder(OrderConfig{GroupBy: []string{"f3"}, OrderBy: []string{"f3 desc"}}),
			},
			query:  "select f1,f2 from Summary where (Type=? or Type=?) group by f3 order by f3 desc",
			values: []string{"r1", "r2"},
		},
		{
			name:   "order",
			fields: []string{"k1", "k2"},
			options: []QueryOption{
				WithTable(SummaryTable), WithWorkouts("c3"), WithSports("c1"),
				WithOrder(OrderConfig{GroupBy: []string{"k3", "k4"}, OrderBy: []string{"k5", "k6"}, Limit: 7}),
			},
			query:  "select k1,k2 from Summary where (Workouttype=?) and (Type=?) group by k3,k4 order by k5,k6 limit 7",
			values: []string{"c3", "c1"},
		},
		{
			name:    "one_year",
			fields:  []string{"field"},
			options: []QueryOption{WithTable(SummaryTable), WithSports("Run"), WithYears(2023)},
			query:   "select field from Summary where (Type=?) and (Year=?)",
			values:  []string{"Run", "2023"},
		},
		{
			name:   "multiple_years",
			fields: []string{"field"},
			options: []QueryOption{
				WithTable(SummaryTable), WithSports("Run"), WithYears(2019), WithYears(2023),
			},
			query:  "select field from Summary where (Type=?) and (Year=? or Year=?)",
			values: []string{"Run", "2019", "2023"},
		},
		{
			name:   "ids",
			fields: []string{"StravaID"},
			options: []QueryOption{
				WithTable(SummaryTable), WithSports("Run"), WithOrder(OrderConfig{OrderBy: []string{"StravaID desc"}}),
			},
			query:  "select StravaID from Summary where (Type=?) order by StravaID desc",
			values: []string{"Run"},
		},
		{
			name:   "besteffort",
			fields: []string{"Summary.Name"},
			options: []QueryOption{
				WithTable(SummaryTable), WithTable(BestEffortTable), WithName("400m"),
			},
			query: "select Summary.Name from Summary,BestEffort " +
				"where Summary.StravaID=BestEffort.StravaID and BestEffort.Name=?",
			values: []string{"400m"},
		},
	}
	for _, value := range values {
		t.Run(value.name, func(t *testing.T) {
			cmd, values := sqlQuery(value.fields, value.options...)
			if cmd != value.query {
				t.Errorf("query mismatch got '%s' vs. expected '%s'", cmd, value.query)
			}
			if fmt.Sprintf("%v", values) != "["+strings.Join(value.values, " ")+"]" {
				t.Errorf(
					"values mismatch got '%s' vs. expected '%s'",
					fmt.Sprintf("%v", values), strings.Join(value.values, " "),
				)
			}
		})
	}
}
