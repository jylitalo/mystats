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

// printCSV outputs results in CSV format
func printTopCSV(period, measurement string, results [][]string) {
	fmt.Printf("%s,year,%-5s", measurement, period)
	fmt.Println()
	for idx := range results {
		fmt.Println(strings.Join(results[idx], ","))
	}
}

// printTable outputs results in CSV format
func printTopTable(period, measurement string, results [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{measurement, "year", period})
	table.AppendBulk(results)
	table.Render()
}

// topCmd turns sqlite db into table or csv by week/month/...
func topCmd(types []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "top",
		Short: "Create top list",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			format, _ := flags.GetString("format")
			limit, _ := flags.GetInt("limit")
			measurement, _ := flags.GetString("measure")
			period, _ := flags.GetString("period")
			types, _ := flags.GetStringSlice("type")
			inYear := map[string]int{
				"month": 12,
				"week":  53,
			}
			if _, ok := inYear[period]; !ok {
				return fmt.Errorf("unknown period: %s", period)
			}
			formatFn := map[string]func(period string, measurement string, results [][]string){
				"csv":   printTopCSV,
				"table": printTopTable,
			}
			if _, ok := formatFn[format]; !ok {
				return fmt.Errorf("unknown format: %s", format)
			}
			results := [][]string{}
			db := storage.Sqlite3{}
			if err := db.Open(); err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.Query(
				[]string{measurement + " as total", "year", period},
				storage.Conditions{Types: types},
				&storage.Order{
					GroupBy: []string{"year", period},
					OrderBy: []string{"total desc", "year desc", period + " desc"},
					Limit:   limit},
			)
			if err != nil {
				return fmt.Errorf("select caused: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				var year, periodValue int
				var measureValue float64
				err = rows.Scan(&measureValue, &year, &periodValue)
				if err != nil {
					return err
				}
				value := ""
				if strings.Contains(measurement, "distance") && !strings.Contains(measurement, "count") {
					value = fmt.Sprintf("%4.1fkm", measureValue/1000)
				} else {
					value = fmt.Sprintf("%4.0f", measureValue)
				}
				results = append(
					results,
					[]string{value, strconv.FormatInt(int64(year), 10), strconv.FormatInt(int64(periodValue), 10)},
				)
			}
			formatFn[format](period, measurement, results)
			return nil
		},
	}
	cmd.Flags().String("format", "csv", "output format (csv, table)")
	cmd.Flags().Int("limit", 10, "number of entries")
	cmd.Flags().String("measure", "sum(distance)", "measurement type (sum(distance), max(elevation), ...)")
	cmd.Flags().String("period", "week", "time period (week, month)")
	cmd.Flags().StringSlice("type", types, "sport types (run, trail run, ...)")
	return cmd
}
