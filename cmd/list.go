package cmd

import (
	"os"

	_ "github.com/mattn/go-sqlite3"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/jylitalo/mystats/pkg/stats"
)

// listCmd turns sqlite db into table or csv by week/month/...
func listCmd(types []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List races or long runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			limit, _ := flags.GetInt("limit")
			types, _ := flags.GetStringSlice("type")
			update, _ := flags.GetBool("update")
			workouts, _ := flags.GetStringSlice("workout")
			db, err := makeDB(update)
			if err != nil {
				return err
			}
			defer db.Close()
			table := tablewriter.NewWriter(os.Stdout)
			headers, results, err := stats.List(db, types, workouts, nil, limit)
			if err != nil {
				return err
			}
			table.SetHeader(headers)
			table.AppendBulk(results)
			table.Render()
			return nil
		},
	}
	cmd.Flags().Int("limit", 100, "number of activities")
	cmd.Flags().StringSlice("type", types, "sport types (run, trail run, ...)")
	cmd.Flags().Bool("update", true, "update database")
	cmd.Flags().StringSlice("workout", []string{}, "workout type")
	return cmd
}
