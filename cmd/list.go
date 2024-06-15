package cmd

import (
	"fmt"
	"math"
	"os"

	"github.com/jylitalo/mystats/storage"
	_ "github.com/mattn/go-sqlite3"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// listCmd turns sqlite db into table or csv by week/month/...
func listCmd(types []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List races or long runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			types, _ := flags.GetStringSlice("type")
			workouts, _ := flags.GetStringSlice("workout")
			makeDB()
			db := storage.Sqlite3{}
			if err := db.Open(); err != nil {
				return err
			}
			defer db.Close()
			o := []string{"year", "month", "day"}
			rows, err := db.Query(
				[]string{"year", "month", "day", "name", "distance", "elevation", "movingtime"},
				storage.Conditions{Workouts: workouts, Types: types}, &storage.Order{GroupBy: o, OrderBy: o},
			)
			if err != nil {
				return fmt.Errorf("query caused: %w", err)
			}
			defer rows.Close()
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Date", "Name", "Distance (km)", "Elevation (m)", "Time"})
			for rows.Next() {
				var year, month, day, movingTime int
				var distance, elevation float64
				var name string
				err = rows.Scan(&year, &month, &day, &name, &distance, &elevation, &movingTime)
				if err != nil {
					return err
				}
				table.Append([]string{
					fmt.Sprintf("%2d.%2d.%d", day, month, year), name,
					fmt.Sprintf("%.0f", math.Round(distance/1000)), fmt.Sprintf("%.0f", elevation),
					fmt.Sprintf("%2d:%02d:%02d", movingTime/3600, movingTime/60%60, movingTime%60),
				})
			}
			table.Render()
			return nil
		},
	}
	cmd.Flags().StringSlice("type", types, "sport types (run, trail run, ...)")
	cmd.Flags().StringSlice("workout", []string{}, "workout type")
	return cmd
}
