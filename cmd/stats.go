package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/olekukonko/tablewriter"
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

// printCSV outputs results in CSV format
func printCSV(period, measurement string, years []int, results [][]string) {
	fmt.Printf("%-5s", period)
	for _, year := range years {
		fmt.Printf(",%d", year)
	}
	fmt.Println()
	for idx := range results {
		fmt.Printf("%5d,%s\n", idx+1, strings.Join(results[idx], ","))
	}
}

// printTable outputs results in CSV format
func printTable(period, measurement string, years []int, results [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	header := []string{period}
	for _, year := range years {
		header = append(header, strconv.Itoa(year))
	}
	table.SetHeader(header)
	for idx := range results {
		table.Append(append([]string{strconv.Itoa(idx + 1)}, results[idx]...))
	}
	table.Render()
}

// fetchCmd fetches activity data from Strava
func statsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Create year to year comparisons",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			format, _ := flags.GetString("format")
			measurement, _ := flags.GetString("measure")
			period, _ := flags.GetString("period")
			inYear := map[string]int{
				"month": 12,
				"week":  53,
			}
			if _, ok := inYear[period]; !ok {
				return fmt.Errorf("unknown period: %s", period)
			}
			formatFn := map[string]func(period string, measurement string, years []int, results [][]string){
				"csv":   printCSV,
				"table": printTable,
			}
			if _, ok := formatFn[format]; !ok {
				return fmt.Errorf("unknown format: %s", format)
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
				for year := range columns { // helps CSV formatting
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
				if strings.Contains(measurement, "distance") && !strings.Contains(measurement, "count") {
					value = fmt.Sprintf("%4.0f", measureValue/1000)
				} else {
					value = fmt.Sprintf("%4.0f", measureValue)
				}
				results[periodValue-1][yearIndex[year]] = value
			}
			formatFn[format](period, measurement, years, results)
			return nil
		},
	}
	cmd.Flags().String("format", "csv", "output format (csv, table)")
	cmd.Flags().String("measure", "sum(distance)", "measurement type (sum(distance), max(elevation), ...)")
	cmd.Flags().String("period", "week", "time period (week, month)")
	return cmd
}
