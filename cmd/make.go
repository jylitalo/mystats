package cmd

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"

	"github.com/jylitalo/mystats/api"
	"github.com/jylitalo/mystats/config"
	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
)

type Storage interface {
	QueryBestEffort(fields []string, name string, order *storage.Order) (*sql.Rows, error)
	QueryBestEffortDistances() ([]string, error)
	QuerySplit(fields []string, id int64) (*sql.Rows, error)
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
			db, err := makeDB(cmd.Context(), update)
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

func makeDB(ctx context.Context, update bool) (Storage, error) {
	ctx, span := telemetry.NewSpan(ctx, "make")
	defer span.End()

	slog.Info("Fetch activities from Strava")
	if update {
		if err := fetch(ctx, true); err != nil {
			return nil, telemetry.Error(span, err)
		}
	}
	cfg, err := config.Get(ctx)
	if err != nil {
		return nil, telemetry.Error(span, err)
	}
	pageFnames, errP := pageFiles(cfg.Strava.Summaries)
	actFnames, errF := activitiesFiles(cfg.Strava.Activities)
	if err := errors.Join(errP, errF); err != nil {
		return nil, telemetry.Error(span, err)
	}
	db := &storage.Sqlite3{}
	if skipDB(db, append(pageFnames, actFnames...)) {
		slog.Info("Database is uptodate")
		if err := db.Open(); err != nil {
			return nil, telemetry.Error(span, err)
		}
		return db, nil
	}
	slog.Info("Making database")
	dbActivities := []storage.SummaryRecord{}
	if activities, err := api.ReadSummaryJSONs(pageFnames); err != nil {
		return nil, telemetry.Error(span, err)
	} else {
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
				ElapsedTime: activity.ElapsedTime,
			})
		}
	}
	dbEfforts := []storage.BestEffortRecord{}
	dbSplits := []storage.SplitRecord{}
	if acts, err := api.ReadActivityJSONs(ctx, actFnames); err != nil {
		return nil, telemetry.Error(span, err)
	} else {
		for _, activity := range acts {
			id := activity.Id
			for _, be := range activity.BestEfforts {
				dbEfforts = append(dbEfforts, storage.BestEffortRecord{
					StravaID:    id,
					Name:        be.EffortSummary.Name,
					MovingTime:  be.EffortSummary.MovingTime,
					ElapsedTime: be.EffortSummary.ElapsedTime,
					Distance:    int(be.Distance),
				})
			}
			for _, split := range activity.SplitsMetric {
				dbSplits = append(dbSplits, storage.SplitRecord{
					StravaID:      id,
					Split:         split.Split,
					MovingTime:    split.MovingTime,
					ElapsedTime:   split.ElapsedTime,
					ElevationDiff: split.ElevationDifference,
					Distance:      split.Distance,
				})

			}
		}
	}
	ctx, spanDB := telemetry.NewSpan(ctx, "rebuildDB")
	defer spanDB.End()
	errR := db.Remove()
	errO := db.Open()
	errC := db.Create()
	errI := db.InsertSummary(ctx, dbActivities)
	errBE := db.InsertBestEffort(ctx, dbEfforts)
	errSplit := db.InsertSplit(ctx, dbSplits)
	return db, telemetry.Error(spanDB, errors.Join(errR, errO, errC, errI, errBE, errSplit))
}
