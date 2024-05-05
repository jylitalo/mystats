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
func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List races or long runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			workoutArg, _ := flags.GetString("workout")
			db := storage.Sqlite3{}
			if err := db.Open(); err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.Query(
				[]string{"year", "month", "day", "name", "distance", "elevation", "movingtime"},
				storage.Conditions{Workouts: []string{workoutArg}, Types: []string{"Run"}},
				&storage.Order{Fields: []string{"year", "month", "day"}, Ascend: true},
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
	cmd.Flags().String("workout", "Race", "workout type")
	return cmd
}