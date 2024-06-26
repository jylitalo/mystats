package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/jylitalo/mystats/pkg/stats"
)

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
	first := func(i int) string {
		return time.Month(i).String()
	}
	if period == "week" {
		first = func(i int) string {
			return strconv.Itoa(i)
		}
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetFooterAlignment(tablewriter.ALIGN_RIGHT)
	header := []string{period}
	for _, year := range years {
		header = append(header, strconv.Itoa(year))
	}
	table.SetHeader(header)
	for idx := range results {
		if strings.TrimSpace(strings.Join(results[idx], "")) != "" {
			table.Append(append([]string{first(idx + 1)}, results[idx]...))
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
			update, _ := flags.GetBool("update")
			month, _ := flags.GetInt("month")
			day, _ := flags.GetInt("day")
			formatFn := map[string]func(period string, measurement string, years []int, results [][]string, totals []string){
				"csv":   printCSV,
				"table": printTable,
			}
			if _, ok := formatFn[format]; !ok {
				return fmt.Errorf("unknown format: %s", format)
			}
			db, err := makeDB(update)
			if err != nil {
				return err
			}
			defer db.Close()
			years, results, totals, err := stats.Stats(db, measurement, period, types, nil, month, day, nil)
			if err != nil {
				return err
			}
			formatFn[format](period, measurement, years, results, totals)
			return nil
		},
	}
	cmd.Flags().String("format", "csv", "output format (csv, table)")
	cmd.Flags().String("measure", "sum(distance)", "measurement type (sum(distance), max(elevation), ...)")
	cmd.Flags().String("period", "week", "time period (week, month)")
	cmd.Flags().StringSlice("type", types, "sport types (run, trail run, ...)")
	cmd.Flags().Bool("update", true, "update database")
	cmd.Flags().Int("month", 12, "only search number of months")
	cmd.Flags().Int("day", 31, "only search number of days from last --month")
	return cmd
}
