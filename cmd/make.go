package cmd

import (
	"errors"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"

	"github.com/jylitalo/mystats/api"
	"github.com/jylitalo/mystats/storage"
)

// fetchCmd fetches activity data from Strava
func makeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "make",
		Short: "Turn fetched JSON files into Sqlite database",
		RunE: func(cmd *cobra.Command, args []string) error {
			fnames, err := filepath.Glob("pages/page*.json")
			if err != nil {
				return err
			}
			dbActivities := []storage.Record{}
			activities, err := api.ReadJSONs(fnames)
			if err != nil {
				return err
			}
			for _, activity := range activities {
				t := activity.StartDateLocal
				year, week := t.ISOWeek()
				dbActivities = append(dbActivities, storage.Record{
					StravaID:    activity.Id,
					Year:        year,
					Month:       int(t.Month()),
					Day:         t.Day(),
					Week:        week,
					Name:        activity.Name,
					Type:        activity.Type.String(),
					WorkoutType: activity.WorkoutType(),
					Distance:    activity.Distance,
					Elevation:   activity.TotalElevationGain,
					MovingTime:  activity.MovingTime,
				})
			}
			db := storage.Sqlite3{}
			errR := db.Remove()
			errO := db.Open()
			errC := db.Create()
			if err = errors.Join(errR, errO, errC); err != nil {
				return err
			}
			defer db.Close()
			return db.Insert(dbActivities)
		},
	}
	return cmd
}
