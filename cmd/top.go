package cmd

import (
	"fmt"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/jylitalo/mystats/pkg/stats"
)

// printCSV outputs results in CSV format
func printTopCSV(headers []string, results [][]string) {
	fmt.Print(strings.Join(headers, ","))
	fmt.Println()
	for idx := range results {
		fmt.Println(strings.Join(results[idx], ","))
	}
}

// printTable outputs results in CSV format
func printTopTable(headers []string, results [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
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
			update, _ := flags.GetBool("update")
			inYear := map[string]int{
				"month": 12,
				"week":  53,
			}
			if _, ok := inYear[period]; !ok {
				return fmt.Errorf("unknown period: %s", period)
			}
			formatFn := map[string]func(headers []string, results [][]string){
				"csv":   printTopCSV,
				"table": printTopTable,
			}
			if _, ok := formatFn[format]; !ok {
				return fmt.Errorf("unknown format: %s", format)
			}
			db, err := makeDB(update)
			if err != nil {
				return err
			}
			defer db.Close()
			headers, results, err := stats.Top(db, measurement, period, types, nil, limit, nil)
			if err != nil {
				return err
			}
			formatFn[format](headers, results)
			return nil
		},
	}
	cmd.Flags().String("format", "csv", "output format (csv, table)")
	cmd.Flags().Int("limit", 10, "number of entries")
	cmd.Flags().String("measure", "distance", "measurement type (distance, elevation, time)")
	cmd.Flags().String("period", "week", "time period (week, month)")
	cmd.Flags().StringSlice("type", types, "sport types (run, trail run, ...)")
	cmd.Flags().Bool("update", true, "update database")
	return cmd
}
