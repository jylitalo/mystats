package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
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
				log.Fatal(err)
			}
			fmt.Printf("cfg=%#v\n", cfg)
			strava.ClientId = cfg.ClientID
			strava.ClientSecret = cfg.ClientSecret
			client := strava.NewClient(cfg.AccessToken)
			fmt.Printf("%#v\n", *client)
			current := strava.NewCurrentAthleteService(client)
			fmt.Printf("%#v\n", *current)
			call := current.ListActivities()
			fmt.Printf("%#v\n", *call)
			activities, err := call.Do()
			if err != nil {
				slog.Error("err from activities.Do", "err", err)
				log.Fatal("Run mystats configure --client_id=... --client_secret=... first.")
			}
			j, err := json.Marshal(activities)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(string(j))
			os.WriteFile("page0.json", j, 0644)
			return nil
		},
	}
	return cmd
}
