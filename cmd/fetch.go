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
)

type jsonStatus struct {
	latest time.Time
	pages  int
	ids    []int64
}

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
	status, err := getJsonStatus()
	if err != nil {
		return err
	}
	client, err := getClient()
	if err != nil {
		return err
	}
	call, err := callListActivities(client, status.latest)
	if err != nil {
		return err
	}
	ids, err := saveActivities(call, status.pages)
	if err == nil && best_efforts {
		ids = append(ids, status.ids...)
		err = fetchBestEfforts(client, ids)
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

func fetchBestEfforts(client *api.Client, ids []int64) error {
	if len(ids) == 0 {
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
	for idx, id := range ids {
		if slices.Contains[[]int64, int64](alreadyFetched, id) {
			continue
		}
		call := service.Get(id)
		activity, err := call.Do()
		if err != nil {
			return err
		}
		j, err := json.Marshal(activity)
		if err != nil {
			return err
		}
		fmt.Printf("%s => activities/activity_%d.json ...\n", activity.StartDateLocal, id)
		if err = os.WriteFile(fmt.Sprintf("activities/activity_%d.json", id), j, 0600); err != nil {
			return err
		}
		fetched++
		if fetched >= 80 {
			slog.Info("Already fetched 80 activities", "left", len(ids)-idx)
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

func getJsonStatus() (jsonStatus, error) {
	status := jsonStatus{
		latest: time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC),
		pages:  0,
		ids:    []int64{},
	}
	fnames, err := pageFiles()
	switch {
	case err != nil:
		return status, err
	case len(fnames) == 0:
		if _, err = os.Stat("pages"); os.IsNotExist(err) {
			if err = os.Mkdir("pages", 0750); err != nil {
				return status, err
			}
		}
	}
	status.pages = len(fnames)
	activities, err := api.ReadSummaryJSONs(fnames)
	if err != nil {
		return status, err
	}
	for _, act := range activities {
		if act.StartDateLocal.After(status.latest) {
			status.latest = act.StartDateLocal
		}
		status.ids = append(status.ids, act.Id)
	}
	return status, nil
}

func callListActivities(client *api.Client, after time.Time) (*api.CurrentAthleteListActivitiesCall, error) {
	current := api.NewCurrentAthleteService(client)
	call := current.ListActivities()
	return call.After(int(after.Unix())), nil
}

func saveActivities(call *api.CurrentAthleteListActivitiesCall, prior int) ([]int64, error) {
	page := 1
	newIds := []int64{}
	for {
		call = call.Page(page)
		activities, err := call.Do()
		if err != nil {
			if api.IsRateLimitExceeded(err) {
				return newIds, err
			}
			if page == 1 {
				return newIds, fmt.Errorf("run mystats configure --client_id=... --client_secret=... first. err=%w", err)
			}
			return newIds, err
		}
		if len(activities) == 0 {
			return newIds, err
		}
		content, err := json.Marshal(activities)
		if err != nil {
			return newIds, err
		}
		for _, act := range activities {
			newIds = append(newIds, act.Id)
		}
		fmt.Printf("%d activities => pages/page%d.json ...\n", len(activities), page+prior)
		if err = os.WriteFile(fmt.Sprintf("pages/page%d.json", page+prior), content, 0600); err != nil {
			return newIds, err
		}
		if len(activities) < 30 {
			return newIds, nil
		}
		page++
	}
}
