package cmd

import (
	"database/sql"
	"errors"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"

	"github.com/jylitalo/mystats/api"
	"github.com/jylitalo/mystats/storage"
)

type Storage interface {
	Query(fields []string, cond storage.Conditions, order *storage.Order) (*sql.Rows, error)
	QueryYears(cond storage.Conditions) ([]int, error)
	Close() error
}

// fetchCmd fetches activity data from Strava
func makeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "make",
		Short: "Turn fetched JSON files into Sqlite database",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := makeDB()
			return err
		},
	}
	return cmd
}

func skipDB(db *storage.Sqlite3, fnames []string) bool {
	dbMtime, _ := db.LastModified()
	pagesMtime := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	for _, fname := range fnames {
		fi, err := os.Stat(fname)
		if err != nil {
			continue
		}
		if m := fi.ModTime().UTC(); m.After(pagesMtime) {
			pagesMtime = m
		}
	}
	return dbMtime.After(pagesMtime)
}

func makeDB() (Storage, error) {
	if err := fetch(); err != nil {
		return nil, err
	}
	fnames, err := pageFiles()
	if err != nil {
		return nil, err
	}
	db := &storage.Sqlite3{}
	if skipDB(db, fnames) {
		return db, nil
	}
	activities, err := api.ReadJSONs(fnames)
	if err != nil {
		return nil, err
	}
	dbActivities := []storage.Record{}
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
	errR := db.Remove()
	errO := db.Open()
	defer db.Close()
	errC := db.Create()
	errI := db.Insert(dbActivities)
	return db, errors.Join(errR, errO, errC, errI)
}
