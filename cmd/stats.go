package cmd

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
)

// queryYears creates list of distinct years from which have records
func queryYears(db *sql.DB) ([]int, error) {
	years := []int{}
	rows, err := db.Query(`select distinct(year) from mystats where type="Run" order by year desc`)
	if err != nil {
		return years, fmt.Errorf("select caused: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var year int
		if err = rows.Scan(&year); err != nil {
			return years, err
		}
		years = append(years, year)
	}
	return years, nil
}

// fetchCmd fetches activity data from Strava
func statsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Create year to year comparisons",
		RunE: func(cmd *cobra.Command, args []string) error {
			dbFile := "mystats.sql"
			db, err := sql.Open("sqlite3", dbFile)
			if err != nil {
				return err
			}
			defer db.Close()
			years, err := queryYears(db)
			if err != nil {
				return err
			}
			yearIndex := map[int]int{}
			for idx, year := range years {
				yearIndex[year] = idx
			}
			results := [53][]string{}
			for idx := range 53 {
				results[idx] = make([]string, len(years))
				for year := range len(years) {
					results[idx][year] = "    "
				}
			}
			rows, err := db.Query(`select year,week,sum(distance) from mystats where type="Run" group by year,week order by week,year`)
			if err != nil {
				return fmt.Errorf("select caused: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				var year, week int
				var distance float64
				err = rows.Scan(&year, &week, &distance)
				if err != nil {
					return err
				}
				results[week-1][yearIndex[year]] = fmt.Sprintf("%4.0f", distance/1000)
			}
			fmt.Print("week")
			for _, year := range years {
				fmt.Printf(",%d", year)
			}
			fmt.Println()
			for idx := range results {
				fmt.Printf("%4d,%s\n", idx+1, strings.Join(results[idx], ","))
			}
			return nil
		},
	}
	return cmd
}
