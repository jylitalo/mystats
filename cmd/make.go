package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jylitalo/mystats/api"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
)

type dbEntry struct {
	Year        int
	Month       int
	Day         int
	Week        int
	StravaID    int64
	Name        string
	Type        string
	WorkoutType string
	Distance    float64
	Elevation   float64
	MovingTime  int
}

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
			dbActivities := []dbEntry{}
			activities, err := api.ReadJSONs(fnames)
			if err != nil {
				return err
			}
			for _, activity := range activities {
				t := activity.StartDateLocal
				year, week := t.ISOWeek()
				dbActivities = append(dbActivities, dbEntry{
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
			dbFile := "mystats.sql"
			os.Remove(dbFile)
			db, err := sql.Open("sqlite3", dbFile)
			if err != nil {
				return err
			}
			defer db.Close()
			_, err = db.Exec(`create table mystats (
				Year        integer,
				Month       integer,
				Day         integer,
				Week        integer,
				StravaID    integer,
				Name        text,
				Type        text,
				WorkoutType text,
				Distance    real,
				Elevation   real,
				MovingTime  integer
			)`)
			if err != nil {
				return fmt.Errorf("create table caused: %w", err)
			}
			tx, err := db.Begin()
			if err != nil {
				return err
			}
			stmt, err := tx.Prepare(`insert into mystats(Year,Month,Day,Week,StravaID,Name,Type,WorkoutType,Distance,Elevation,MovingTime) values (?,?,?,?,?,?,?,?,?,?,?)`)
			if err != nil {
				return fmt.Errorf("insert caused %w", err)
			}
			defer stmt.Close()
			for _, dbAct := range dbActivities {
				_, err = stmt.Exec(
					dbAct.Year, dbAct.Month, dbAct.Day, dbAct.Week, dbAct.StravaID,
					dbAct.Name, dbAct.Type, dbAct.WorkoutType,
					dbAct.Distance, dbAct.Elevation, dbAct.MovingTime,
				)
				if err != nil {
					return fmt.Errorf("statement execution caused: %w", err)
				}
			}
			if err = tx.Commit(); err != nil {
				return fmt.Errorf("commit caused %w", err)
			}
			return nil
		},
	}
	return cmd
}
