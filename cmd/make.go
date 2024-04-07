package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
	strava "github.com/strava/go.strava"
)

type dbEntry struct {
	Year       int
	Month      int
	Day        int
	Week       int
	StravaID   int64
	Type       string
	Distance   float64
	Elevation  float64
	MovingTime int
}

// fetchCmd fetches activity data from Strava
func makeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "make",
		Short: "Turn fetched JSON files into Sqlite database",
		RunE: func(cmd *cobra.Command, args []string) error {
			fnames, err := filepath.Glob("pages/page*.json")
			if err != nil {
				log.Fatal(err)
			}
			ids := map[int64]string{}
			activities := []strava.ActivitySummary{}
			for _, fname := range fnames {
				body, err := os.ReadFile(fname)
				if err != nil {
					log.Fatal(err)
				}
				page := []strava.ActivitySummary{}
				if err = json.Unmarshal(body, &page); err != nil {
					log.Fatal(err)
				}
				for _, p := range page {
					if val, ok := ids[p.Id]; ok {
						slog.Error("id exists in multiple pages", "id", p.Id, "current", fname, "previos", val)
					} else {
						ids[p.Id] = fname
						activities = append(activities, p)
					}
				}
			}
			dbActivities := []dbEntry{}
			for _, activity := range activities {
				t := activity.StartDateLocal
				year, week := t.ISOWeek()
				dbActivities = append(dbActivities, dbEntry{
					StravaID:   activity.Id,
					Year:       year,
					Month:      int(t.Month()),
					Day:        t.Day(),
					Week:       week,
					Type:       activity.Type.String(),
					Distance:   activity.Distance,
					Elevation:  activity.TotalElevationGain,
					MovingTime: activity.MovingTime,
				})
			}
			dbFile := "mystats.sql"
			os.Remove(dbFile)
			db, err := sql.Open("sqlite3", dbFile)
			if err != nil {
				log.Fatal(err)
			}
			defer db.Close()
			_, err = db.Exec(`create table mystats (
				Year       integer,
				Month      integer,
				Day        integer,
				Week       integer,
				StravaID   integer,
				Type       text,
				Distance   real,
				Elevation  real,
				MovingTime integer
			)`)
			if err != nil {
				log.Fatal(fmt.Errorf("create table caused: %w", err))
			}
			tx, err := db.Begin()
			if err != nil {
				log.Fatal(err)
			}
			stmt, err := tx.Prepare(`insert into mystats(Year,Month,Day,Week,StravaID,Type,Distance,Elevation,MovingTime) values (?,?,?,?,?,?,?,?,?)`)
			if err != nil {
				log.Fatal(fmt.Errorf("insert caused %w", err))
			}
			defer stmt.Close()
			for _, dbAct := range dbActivities {
				_, err = stmt.Exec(
					dbAct.Year, dbAct.Month, dbAct.Day, dbAct.Week, dbAct.StravaID, dbAct.Type,
					dbAct.Distance, dbAct.Elevation, dbAct.MovingTime,
				)
				if err != nil {
					log.Fatal(fmt.Errorf("statement execution caused: %w", err))
				}
			}
			if err = tx.Commit(); err != nil {
				log.Fatal(fmt.Errorf("commit caused %w", err))
			}
			return nil
		},
	}
	return cmd
}
