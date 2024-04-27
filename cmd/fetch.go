package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/jylitalo/mystats/api"
	"github.com/jylitalo/mystats/config"
)

// fetchCmd fetches activity data from Strava
func fetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch activity data from Strava",
		RunE: func(cmd *cobra.Command, args []string) error {
			epoch := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
			offset := 0
			fnames, err := filepath.Glob("pages/page*.json")
			switch {
			case err != nil:
				return err
			case len(fnames) == 0:
				if _, err = os.Stat("pages"); os.IsNotExist(err) {
					if err = os.Mkdir("pages", 0755); err != nil {
						return err
					}
				}
			default:
				offset = len(fnames)
				activities, err := api.ReadJSONs(fnames)
				if err != nil {
					return err
				}
				for _, act := range activities {
					if act.StartDateLocal.After(epoch) {
						epoch = act.StartDateLocal
					}
				}
			}
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			api.ClientId = cfg.ClientID
			api.ClientSecret = cfg.ClientSecret
			client := api.NewClient(cfg.AccessToken)
			current := api.NewCurrentAthleteService(client)
			call := current.ListActivities()
			call = call.After(int(epoch.Unix()))
			stay := true
			for page := 1; stay; page++ {
				call = call.Page(page)
				activities, err := call.Do()
				if err != nil {
					if page == 1 {
						return fmt.Errorf("run mystats configure --client_id=... --client_secret=... first. err=%w", err)
					}
					return err
				}
				j, err := json.Marshal(activities)
				if err != nil || len(activities) == 0 {
					return err
				}
				fmt.Printf("%d => pages/page%d.json ...\n", page, page+offset)
				os.WriteFile(fmt.Sprintf("pages/page%d.json", page+offset), j, 0644)
				stay = (len(activities) == 30)
			}
			return nil
		},
	}
	return cmd
}
