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
	stravaapi "github.com/strava/go.strava"

	"github.com/jylitalo/mystats/api/garmin"
	"github.com/jylitalo/mystats/api/strava"
	"github.com/jylitalo/mystats/config"
	"github.com/jylitalo/mystats/pkg/telemetry"
	"github.com/jylitalo/mystats/storage"
)

type Storage interface {
	QueryBestEffortDistances() ([]string, error)
	QuerySports() ([]string, error)
	QueryWorkouts() ([]string, error)
	QueryYears(opts ...storage.QueryOption) ([]int, error)
	Query(fields []string, opts ...storage.QueryOption) (*sql.Rows, error)
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
			defer func() { _ = db.Close() }()
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
	stepsFiles, errS := stepsFiles(cfg.Garmin.DailySteps)
	heartRateFiles, errHR := heartRateFiles(cfg.Garmin.HeartRate)
	if err := errors.Join(errP, errF, errS, errHR); err != nil {
		return nil, telemetry.Error(span, err)
	}
	db := &storage.Sqlite3{}
	if skipDB(db, append(pageFnames, actFnames...)) {
		slog.Info("Database is uptodate")
		return db, telemetry.Error(span, db.Open())
	}
	slog.Info("Making database")
	summaries, errS := strava.ReadSummaryJSONs(pageFnames)
	acts, errA := strava.ReadActivityJSONs(ctx, actFnames)
	dbDailySteps, errDS := garmin.ReadDailyStepsJSONs(ctx, stepsFiles)
	dbHeartRate, errHR := garmin.ReadHeartRateJSONs(ctx, heartRateFiles)
	if err := errors.Join(errS, errA, errDS, errHR); err != nil {
		return nil, telemetry.Error(span, err)
	}
	ctx, spanDB := telemetry.NewSpan(ctx, "rebuildDB")
	defer spanDB.End()
	return db, telemetry.Error(spanDB, errors.Join(
		db.Remove(), db.Open(), db.Create(),
		db.InsertSummary(ctx, getDbActivities(summaries)),
		db.InsertBestEffort(ctx, getDbBestEfforts(acts)),
		db.InsertSplit(ctx, getDbSplits(acts)),
		db.InsertDailySteps(ctx, dbDailySteps),
		db.InsertHeartRate(ctx, dbHeartRate),
	))
}

func getDbActivities(activities []strava.ActivitySummary) []storage.SummaryRecord {
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
			ElapsedTime: activity.ElapsedTime,
		})
	}
	return dbActivities
}

func getDbBestEfforts(activities []stravaapi.ActivityDetailed) []storage.BestEffortRecord {
	dbEfforts := []storage.BestEffortRecord{}
	for _, activity := range activities {
		for _, be := range activity.BestEfforts {
			dbEfforts = append(dbEfforts, storage.BestEffortRecord{
				StravaID:    activity.Id,
				Name:        be.Name,
				MovingTime:  be.MovingTime,
				ElapsedTime: be.ElapsedTime,
				Distance:    int(be.Distance),
			})
		}
	}
	return dbEfforts
}

func getDbSplits(activities []stravaapi.ActivityDetailed) []storage.SplitRecord {
	dbSplits := []storage.SplitRecord{}
	for _, activity := range activities {
		for _, split := range activity.SplitsMetric {
			dbSplits = append(dbSplits, storage.SplitRecord{
				StravaID:      activity.Id,
				Split:         split.Split,
				MovingTime:    split.MovingTime,
				ElapsedTime:   split.ElapsedTime,
				ElevationDiff: split.ElevationDifference,
				Distance:      split.Distance,
			})
		}
	}
	return dbSplits
}
