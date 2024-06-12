package cmd

import (
	"log/slog"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"

	"github.com/jylitalo/mystats/pkg/plot"
	"github.com/jylitalo/mystats/storage"
)

// plotCmd makes graphs from sqlite data
func plotCmd(types []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plot",
		Short: "Create graphics",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			measurement, _ := flags.GetString("measure")
			output, _ := flags.GetString("output")
			types, _ := flags.GetStringSlice("type")
			month, _ := flags.GetInt("month")
			day, _ := flags.GetInt("day")
			db := storage.Sqlite3{}
			if err := db.Open(); err != nil {
				return err
			}
			defer db.Close()
			err := plot.Plot(&db, types, measurement, month, day, nil, output)
			if err != nil {
				return err
			}
			slog.Info("Plat created", "output", output)
			return nil
		},
	}
	cmd.Flags().String("output", "ytd.png", "output file")
	cmd.Flags().String("measure", "distance", "measurement type (distance, elevation, ...)")
	cmd.Flags().StringSlice("type", types, "sport types (run, trail run, ...)")
	cmd.Flags().Int("month", 12, "only search number of months")
	cmd.Flags().Int("day", 31, "only search number of days from last --month")
	return cmd
}
