package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jylitalo/mystats/storage"
	_ "github.com/mattn/go-sqlite3"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// queryYears creates list of distinct years from which have records
func queryYears(db *storage.Sqlite3, cond storage.Conditions) ([]int, error) {
	years := []int{}
	rows, err := db.Query(
		[]string{"distinct(year)"}, cond,
		&storage.Order{GroupBy: []string{"year"}, OrderBy: []string{"year desc"}},
	)
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
func printCSV(period, measurement string, years []int, results [][]string, totals []string) {
	fmt.Printf("%-5s", period)
	for _, year := range years {
		fmt.Printf(",%d", year)
	}
	fmt.Println()
	for idx := range results {
		if strings.TrimSpace(strings.Join(results[idx], "")) != "" {
			fmt.Printf("%5d,%s\n", idx+1, strings.Join(results[idx], ","))
		}
	}
	fmt.Printf("TOTAL,%s\n", strings.Join(totals, ","))
}

// printTable outputs results in CSV format
func printTable(period, measurement string, years []int, results [][]string, totals []string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetFooterAlignment(tablewriter.ALIGN_RIGHT)
	header := []string{period}
	for _, year := range years {
		header = append(header, strconv.Itoa(year))
	}
	table.SetHeader(header)
	for idx := range results {
		if strings.TrimSpace(strings.Join(results[idx], "")) != "" {
			table.Append(append([]string{strconv.Itoa(idx + 1)}, results[idx]...))
		}
	}
	table.SetFooter(append([]string{"total"}, totals...))
	table.Render()
}

// statsCmd turns sqlite db into table or csv by week/month/...
func statsCmd(types []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Create year to year comparisons",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			format, _ := flags.GetString("format")
			measurement, _ := flags.GetString("measure")
			period, _ := flags.GetString("period")
			types, _ := flags.GetStringSlice("type")
			month, _ := flags.GetInt("month")
			day, _ := flags.GetInt("day")
			inYear := map[string]int{
				"month": 12,
				"week":  53,
			}
			if _, ok := inYear[period]; !ok {
				return fmt.Errorf("unknown period: %s", period)
			}
			cond := storage.Conditions{Types: types, Month: month, Day: day}
			formatFn := map[string]func(period string, measurement string, years []int, results [][]string, totals []string){
				"csv":   printCSV,
				"table": printTable,
			}
			if _, ok := formatFn[format]; !ok {
				return fmt.Errorf("unknown format: %s", format)
			}
			results := make([][]string, inYear[period])
			db := storage.Sqlite3{}
			if err := db.Open(); err != nil {
				return err
			}
			defer db.Close()
			years, err := queryYears(&db, cond)
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
			rows, err := db.Query(
				[]string{"year", period, measurement}, cond,
				&storage.Order{GroupBy: []string{period, "year"}, OrderBy: []string{period, "year"}},
			)
			if err != nil {
				return fmt.Errorf("select caused: %w", err)
			}
			defer rows.Close()
			totalsAbs := make([]float64, len(years))
			modifier := float64(1)
			if strings.Contains(measurement, "distance") && !strings.Contains(measurement, "count") {
				modifier = 1000
			}
			for rows.Next() {
				var year, periodValue int
				var measureValue float64
				if err = rows.Scan(&year, &periodValue, &measureValue); err != nil {
					return err
				}
				totalsAbs[yearIndex[year]] += measureValue / modifier
				results[periodValue-1][yearIndex[year]] = fmt.Sprintf("%4.0f", measureValue/modifier)
			}
			totals := make([]string, len(years))
			for idx := range totalsAbs {
				totals[idx] = fmt.Sprintf("%4.0f", totalsAbs[idx])
			}

			formatFn[format](period, measurement, years, results, totals)
			return nil
		},
	}
	cmd.Flags().String("format", "csv", "output format (csv, table)")
	cmd.Flags().String("measure", "sum(distance)", "measurement type (sum(distance), max(elevation), ...)")
	cmd.Flags().String("period", "week", "time period (week, month)")
	cmd.Flags().StringSlice("type", types, "sport types (run, trail run, ...)")
	cmd.Flags().Int("month", 12, "only search number of months")
	cmd.Flags().Int("day", 31, "only search number of days from last --month")
	return cmd
}
