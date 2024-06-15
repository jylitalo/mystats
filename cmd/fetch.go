package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jylitalo/mystats/api"
	"github.com/jylitalo/mystats/config"
	"github.com/spf13/cobra"
)

// fetchCmd fetches activity data from Strava
func fetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch activity data from Strava to JSON files",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fetch()
		},
	}
	return cmd
}

func fetch() error {
	after, prior, err := getEpoch()
	if err != nil {
		return err
	}
	call, err := callListActivities(after)
	if err != nil {
		return err
	}
	_, err = saveActivities(call, prior)
	return err
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
	activities, err := api.ReadJSONs(fnames)
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

func callListActivities(after time.Time) (*api.CurrentAthleteListActivitiesCall, error) {
	cfg, err := config.Get(true)
	if err != nil {
		return nil, err
	}
	strava := cfg.Strava
	api.ClientId = strava.ClientID
	api.ClientSecret = strava.ClientSecret
	client := api.NewClient(strava.AccessToken)
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
