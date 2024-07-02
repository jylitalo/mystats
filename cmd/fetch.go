package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jylitalo/mystats/api"
	"github.com/jylitalo/mystats/config"
	"github.com/jylitalo/mystats/storage"
)

// fetchCmd fetches activity data from Strava
func fetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch activity data from Strava to JSON files",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			be, _ := flags.GetBool("best_efforts")
			return fetch(be)
		},
	}
	cmd.Flags().Bool("best_efforts", false, "Fetch activities best efforts")
	return cmd
}

func fetch(best_efforts bool) error {
	after, prior, err := getEpoch()
	if err != nil {
		return err
	}
	client, err := getClient()
	if err != nil {
		return err
	}
	call, err := callListActivities(client, after)
	if err != nil {
		return err
	}
	_, err = saveActivities(call, prior)
	if err == nil && best_efforts {
		err = fetchBestEfforts(client)
	}
	if err != nil && api.IsRateLimitExceeded(err) {
		slog.Warn("Strava API Rate Limit Exceeded")
		return nil
	}
	return err
}

func getClient() (*api.Client, error) {
	cfg, err := config.Get(true)
	if err != nil {
		return nil, err
	}
	strava := cfg.Strava
	api.ClientId = strava.ClientID
	api.ClientSecret = strava.ClientSecret
	client := api.NewClient(strava.AccessToken)
	return client, err
}

func fetchBestEfforts(client *api.Client) error {
	db := &storage.Sqlite3{}
	if err := db.Open(); err != nil {
		return err
	}
	rows, err := db.QuerySummary(
		[]string{"StravaID"},
		storage.SummaryConditions{},
		&storage.Order{OrderBy: []string{"StravaID desc"}},
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	stravaIDs := []int64{}
	for rows.Next() {
		var stravaID int64
		err = rows.Scan(&stravaID)
		if err != nil {
			return err
		}
		stravaIDs = append(stravaIDs, stravaID)
	}
	if len(stravaIDs) < 1 {
		return errors.New("no stravaIDs found from database")
	}
	fetched := 0
	_ = os.Mkdir("activities", 0750)
	actFiles, err := activitiesFiles()
	if err != nil {
		return err
	}
	alreadyFetched := []int64{}
	for _, actFile := range actFiles {
		intStr := strings.Split(strings.Split(actFile, "_")[1], ".")[0]
		i, _ := strconv.Atoi(intStr)
		alreadyFetched = append(alreadyFetched, int64(i))
	}
	service := api.NewActivitiesService(client)
	for idx, stravaID := range stravaIDs {
		if slices.Contains[[]int64, int64](alreadyFetched, stravaID) {
			continue
		}
		call := service.Get(stravaID)
		activity, err := call.Do()
		if err != nil {
			return err
		}
		j, err := json.Marshal(activity)
		if err != nil {
			return err
		}
		fmt.Printf("%d => activities/activity_%d.json ...\n", stravaID, stravaID)
		if err = os.WriteFile(fmt.Sprintf("activities/activity_%d.json", stravaID), j, 0600); err != nil {
			return err
		}
		fetched++
		if fetched >= 100 {
			slog.Info("Already fetched 100 activities", "left", len(stravaIDs)-idx)
			return nil
		}
	}
	slog.Info("Activity details fetched", "fetched", fetched)
	return err
}

func activitiesFiles() ([]string, error) {
	return filepath.Glob("activities/activity_*.json")
}

func pageFiles() ([]string, error) {
	return filepath.Glob("pages/page*.json")
}

func getEpoch() (time.Time, int, error) {
	epoch := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	fnames, err := pageFiles()
	switch {
	case err != nil:
		return epoch, 0, err
	case len(fnames) == 0:
		if _, err = os.Stat("pages"); os.IsNotExist(err) {
			if err = os.Mkdir("pages", 0750); err != nil {
				return epoch, 0, err
			}
		}
	}
	offset := len(fnames)
	activities, err := api.ReadSummaryJSONs(fnames)
	if err != nil {
		return epoch, offset, err
	}
	for _, act := range activities {
		if act.StartDateLocal.After(epoch) {
			epoch = act.StartDateLocal
		}
	}
	return epoch, offset, nil
}

func callListActivities(client *api.Client, after time.Time) (*api.CurrentAthleteListActivitiesCall, error) {
	current := api.NewCurrentAthleteService(client)
	call := current.ListActivities()
	return call.After(int(after.Unix())), nil
}

func saveActivities(call *api.CurrentAthleteListActivitiesCall, prior int) (int, error) {
	page := 1
	for {
		call = call.Page(page)
		activities, err := call.Do()
		if err != nil {
			if api.IsRateLimitExceeded(err) {
				return page, err
			}
			if page == 1 {
				return page, fmt.Errorf("run mystats configure --client_id=... --client_secret=... first. err=%w", err)
			}
			return page, err
		}
		if len(activities) == 0 {
			return page - 1, err
		}
		j, err := json.Marshal(activities)
		if err != nil {
			return page, err
		}
		fmt.Printf("%d => pages/page%d.json ...\n", page, page+prior)
		if err = os.WriteFile(fmt.Sprintf("pages/page%d.json", page+prior), j, 0600); err != nil {
			return page, err
		}
		if len(activities) < 30 {
			return page, nil
		}
		page++
	}
}
