package cmd

import (
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"

	"github.com/jylitalo/mystats/pkg/stats"
)

// topCmd turns sqlite db into table or csv by week/month/...
func bestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "best",
		Short: "Best Run Efforts based on Strava",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			format, _ := flags.GetString("format")
			limit, _ := flags.GetInt("limit")
			distance, _ := flags.GetString("distance")
			update, _ := flags.GetBool("update")
			formatFn := map[string]func(headers []string, results [][]string){
				"csv":   printTopCSV,
				"table": printTopTable,
			}
			if _, ok := formatFn[format]; !ok {
				return fmt.Errorf("unknown format: %s", format)
			}
			ctx := cmd.Context()
			db, err := makeDB(ctx, update)
			if err != nil {
				return err
			}
			defer db.Close()
			headers, results, err := stats.Best(ctx, db, distance, limit)
			if err != nil {
				return err
			}
			formatFn[format](headers, results)
			return nil
		},
	}
	cmd.Flags().String("format", "csv", "output format (csv, table)")
	cmd.Flags().String("distance", "Marathon", "Best Efforts distance")
	cmd.Flags().Int("limit", 10, "number of entries")
	cmd.Flags().Bool("update", true, "update database")
	return cmd
}
