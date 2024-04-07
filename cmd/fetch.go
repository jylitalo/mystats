package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	strava "github.com/strava/go.strava"

	"github.com/jylitalo/mystats/config"
)

// fetchCmd fetches activity data from Strava
func fetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch activity data from Strava",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			strava.ClientId = cfg.ClientID
			strava.ClientSecret = cfg.ClientSecret
			client := strava.NewClient(cfg.AccessToken)
			current := strava.NewCurrentAthleteService(client)
			call := current.ListActivities()
			stay := true
			if err = os.Mkdir("pages", 0755); err != nil {
				return err
			}
			for page := 0; stay; page++ {
				if page > 0 {
					call = call.Page(page)
				}
				activities, err := call.Do()
				if err != nil {
					if page == 0 {
						return fmt.Errorf("run mystats configure --client_id=... --client_secret=... first. err=%w", err)
					}
					return err
				}
				j, err := json.Marshal(activities)
				if err != nil {
					return err
				}
				fmt.Printf("page %d ...\n", page+1)
				os.WriteFile(fmt.Sprintf("pages/page%d.json", page), j, 0644)
				stay = (len(activities) == 30)
			}
			return nil
		},
	}
	return cmd
}
