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
			flags := cmd.Flags()
			measurement, _ := flags.GetString("measure")
			period, _ := flags.GetString("period")
			inYear := map[string]int{
				"month": 12,
				"week":  53,
			}
			if _, ok := inYear[period]; !ok {
				return fmt.Errorf("unknown period: %s", period)
			}
			results := make([][]string, inYear[period])
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
			columns := len(years)
			for idx := range results {
				results[idx] = make([]string, columns)
				for year := range columns {
					results[idx][year] = "    "
				}
			}
			query := fmt.Sprintf(
				`select year,%s,%s from mystats where type="Run" group by year,%s order by %s,year`,
				period, measurement, period, period,
			)
			rows, err := db.Query(query)
			if err != nil {
				return fmt.Errorf("%s caused: %w", query, err)
			}
			defer rows.Close()
			for rows.Next() {
				var year, periodValue int
				var measureValue float64
				value := ""
				err = rows.Scan(&year, &periodValue, &measureValue)
				if err != nil {
					return err
				}
				if strings.Contains(measurement, "distance") {
					value = fmt.Sprintf("%4.0f", measureValue/1000)
				} else {
					value = fmt.Sprintf("%4.0f", measureValue)
				}
				results[periodValue-1][yearIndex[year]] = value
			}
			fmt.Printf("%-5s", period)
			for _, year := range years {
				fmt.Printf(",%d", year)
			}
			fmt.Println()
			for idx := range results {
				fmt.Printf("%5d,%s\n", idx+1, strings.Join(results[idx], ","))
			}
			return nil
		},
	}
	cmd.Flags().String("period", "week", "time period (week, month)")
	cmd.Flags().String("measure", "sum(distance)", "measurement type (sum(distance), max(elevation), ...)")
	return cmd
}
