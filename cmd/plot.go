package cmd

import (
	"fmt"
	"log"
	"log/slog"
	"strconv"
	"time"

	"github.com/jylitalo/mystats/storage"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
	"go-hep.org/x/hep/hplot"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
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
			tz, _ := time.LoadLocation("Europe/Helsinki")
			db := storage.Sqlite3{}
			if err := db.Open(); err != nil {
				return err
			}
			defer db.Close()
			cond := storage.Conditions{Types: types, Month: month, Day: day}
			years, err := queryYears(&db, cond)
			if err != nil {
				return err
			}
			xs := map[int][]float64{}
			ys := map[int][]float64{}
			totals := map[int]float64{}
			for _, year := range years {
				xs[year] = []float64{}
				ys[year] = []float64{}
				totals[year] = 0
			}
			o := []string{"year", "month", "day"}
			rows, err := db.Query(
				[]string{"year", "month", "day", "sum(" + measurement + ")"},
				cond, &storage.Order{GroupBy: o, OrderBy: o},
			)
			if err != nil {
				return fmt.Errorf("select caused: %w", err)
			}
			defer rows.Close()
			var xmax float64
			for rows.Next() {
				var year, month, day int
				var value float64
				err = rows.Scan(&year, &month, &day, &value)
				if err != nil {
					return err
				}
				if measurement == "distance" {
					value = value / 1000
				}
				totals[year] = totals[year] + value
				day1 := time.Date(year, time.January, 1, 6, 0, 0, 0, tz)
				now := time.Date(year, time.Month(month), day, 6, 0, 0, 0, tz)
				days := now.Sub(day1).Hours() / 24
				xmax = max(xmax, days)
				xs[year] = append(xs[year], days)
				ys[year] = append(ys[year], totals[year])
			}
			p := plot.New()
			p.Title.Text = fmt.Sprintf("year to day (month=%d, day=%d)", month, day)
			p.X.Label.Text = "days"
			p.Y.Label.Text = "distance"
			p.X.Min = 0
			p.X.Max = xmax
			p.Y.Min = 0
			yearLines := []interface{}{}
			for _, year := range years {
				yearLines = append(yearLines, strconv.FormatInt(int64(year), 10), hplot.ZipXY(xs[year], ys[year]))
			}
			err = plotutil.AddLines(p, yearLines...)
			if err != nil {
				log.Fatal("Failed to plot years")
			}
			err = p.Save(40*vg.Centimeter, 20*vg.Centimeter, output)
			if err != nil {
				log.Fatal("failed to save image")
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
