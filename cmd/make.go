package cmd

import (
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"

	"github.com/jylitalo/mystats/api"
	"github.com/jylitalo/mystats/storage"
)

type Storage interface {
	QueryBestEffort(fields []string, name string, order *storage.Order) (*sql.Rows, error)
	QueryBestEffortDistances() ([]string, error)
	QuerySummary(fields []string, cond storage.SummaryConditions, order *storage.Order) (*sql.Rows, error)
	QueryTypes(cond storage.SummaryConditions) ([]string, error)
	QueryWorkoutTypes(cond storage.SummaryConditions) ([]string, error)
	QueryYears(cond storage.SummaryConditions) ([]int, error)
	Close() error
}

// fetchCmd fetches activity data from Strava
func makeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "make",
		Short: "Turn fetched JSON files into Sqlite database",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			update, _ := flags.GetBool("update")
			db, err := makeDB(update)
			if err != nil {
				return err
			}
			defer db.Close()
			return err
		},
	}
	cmd.Flags().Bool("update", true, "update database")
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

func makeDB(update bool) (Storage, error) {
	slog.Info("Fetch activities from Strava")
	if update {
		if err := fetch(false); err != nil {
			return nil, err
		}
	}
	fnames, err := pageFiles()
	if err != nil {
		return nil, err
	}
	db := &storage.Sqlite3{}
	if skipDB(db, fnames) {
		slog.Info("Database is uptodate")
		db.Open()
		return db, nil
	}
	slog.Info("Making database")
	activities, err := api.ReadSummaryJSONs(fnames)
	if err != nil {
		return nil, err
	}
	dbActivities := []storage.SummaryRecord{}
	for _, activity := range activities {
		t := activity.StartDateLocal
		year, week := t.ISOWeek()
		dbActivities = append(dbActivities, storage.SummaryRecord{
			StravaID:    activity.Id,
			Year:        year,
			Month:       int(t.Month()),
			Day:         t.Day(),
			Week:        week,
			Name:        activity.Name,
			Type:        activity.Type.String(),
			SportType:   activity.SportType,
			WorkoutType: activity.WorkoutType(),
			Distance:    activity.Distance,
			Elevation:   activity.TotalElevationGain,
			MovingTime:  activity.MovingTime,
		})
	}
	fnames, err = activitiesFiles()
	if err != nil {
		return nil, err
	}
	detailed, err := api.ReadBestEffortJSONs(fnames)
	if err != nil {
		return nil, err
	}
	dbEfforts := []storage.BestEffortRecord{}
	for _, activity := range detailed {
		dbEfforts = append(dbEfforts, storage.BestEffortRecord{
			StravaID:    activity.EffortSummary.Activity.Id,
			Name:        activity.EffortSummary.Name,
			MovingTime:  activity.EffortSummary.MovingTime,
			ElapsedTime: activity.EffortSummary.ElapsedTime,
			Distance:    int(activity.Distance),
		})
	}
	errR := db.Remove()
	errO := db.Open()
	errC := db.Create()
	errI := db.InsertSummary(dbActivities)
	errBE := db.InsertBestEffort(dbEfforts)
	return db, errors.Join(errR, errO, errC, errI, errBE)
}
