package storage

import (
	"fmt"
	"strings"
	"testing"
)

func TestSqlQuery(t *testing.T) {
	values := []struct {
		name    string
		fields  []string
		options []QueryOption
		query   string
		values  []string
	}{
		{
			"none", []string{"field"}, []QueryOption{WithTable(SummaryTable)},
			"select field from Summary", []string{},
		},
		{
			"simple", []string{"field"}, []QueryOption{WithTable(SummaryTable), WithSport("Run")},
			"select field from Summary where (Type=?)", []string{"Run"},
		},
		{
			"multi-field", []string{"f1", "f2"}, []QueryOption{
				WithTable(SummaryTable), WithSport("r1"), WithSport("r2"),
				WithOrder(OrderConfig{GroupBy: []string{"f3"}, OrderBy: []string{"f3 desc"}}),
			},
			"select f1,f2 from Summary where (Type=? or Type=?) group by f3 order by f3 desc", []string{"r1", "r2"},
		},
		{
			"order", []string{"k1", "k2"}, []QueryOption{
				WithTable(SummaryTable), WithWorkout("c3"), WithSport("c1"),
				WithOrder(OrderConfig{GroupBy: []string{"k3", "k4"}, OrderBy: []string{"k5", "k6"}, Limit: 7}),
			},
			"select k1,k2 from Summary where (Workouttype=?) and (Type=?) group by k3,k4 order by k5,k6 limit 7",
			[]string{"c3", "c1"},
		},
		{
			"one_year", []string{"field"}, []QueryOption{WithTable(SummaryTable), WithSport("Run"), WithYear(2023)},
			"select field from Summary where (Type=?) and (Year=?)", []string{"Run", "2023"},
		},
		{
			"multiple_years", []string{"field"}, []QueryOption{
				WithTable(SummaryTable), WithSport("Run"), WithYear(2019), WithYear(2023),
			},
			"select field from Summary where (Type=?) and (Year=? or Year=?)", []string{"Run", "2019", "2023"},
		},
		{
			"ids", []string{"StravaID"}, []QueryOption{
				WithTable(SummaryTable), WithSport("Run"), WithOrder(OrderConfig{OrderBy: []string{"StravaID desc"}}),
			},
			"select StravaID from Summary where (Type=?) order by StravaID desc", []string{"Run"},
		},
		{
			"besteffort", []string{"Summary.Name"}, []QueryOption{
				WithTable(SummaryTable), WithTable(BestEffortTable), WithName("400m"),
			},
			"select Summary.Name from Summary,BestEffort where Summary.StravaID=BestEffort.StravaID and BestEffort.Name=?",
			[]string{"400m"},
		},
	}
	for _, value := range values {
		t.Run(value.name, func(t *testing.T) {
			cmd, values := sqlQuery(value.fields, value.options...)
			if cmd != value.query {
				t.Errorf("query mismatch got '%s' vs. expected '%s'", cmd, value.query)
			}
			if fmt.Sprintf("%v", values) != "["+strings.Join(value.values, " ")+"]" {
				t.Errorf("values mismatch got '%s' vs. expected '%s'", fmt.Sprintf("%v", values), strings.Join(value.values, " "))
			}
		})
	}
}
