package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/viper"
	strava "github.com/strava/go.strava"
)

type StravaConfig struct {
	ClientID     int
	ClientSecret string
	AccessToken  string
	Code         string
	Token        string
}

func getConfig() (StravaConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return StravaConfig{}, fmt.Errorf("error in UserHomeDir: %w", err)
	}
	vip := viper.GetViper()
	vip.AddConfigPath(home)
	vip.SetConfigName(".mystats")
	if err = vip.ReadInConfig(); err != nil {
		return StravaConfig{}, fmt.Errorf("error in getConfig: %w", err)
	}
	return StravaConfig{
		ClientID:     vip.GetInt("strava.clientID"),
		ClientSecret: vip.GetString("strava.clientSecret"),
		AccessToken:  vip.GetString("strava.accessToken"),
		Code:         vip.GetString("strava.code"),
		Token:        vip.GetString("strava.token"),
	}, nil
}

func main() {
	// followup instructions from https://yizeng.me/2017/01/11/get-a-strava-api-access-token-with-write-permission/
	// to get activity:read_all and with that code, ...
	scfg, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}
	if scfg.Code == "" {
		// http://www.strava.com/oauth/authorize?client_id=[REPLACE_WITH_YOUR_CLIENT_ID]&response_type=code&redirect_uri=http://localhost/exchange_token&approval_prompt=force&scope=read_all,profile:read_all,activity:read_all
		fmt.Printf(
			"http://www.strava.com/oauth/authorize?client_id=%d&response_type=code&redirect_uri=http://localhost/exchange_token&approval_prompt=force&scope=activity:read_all\n",
			scfg.ClientID,
		)
		// $ curl -X POST https://www.strava.com/oauth/token \
		// -F client_id=5 \
		// -F client_secret=[REPLACE_WITH_YOUR_CLIENT_SECRET] \
		// -F code=c498932e64136c8991a3fb31e3d1dfdf2f859357
		// -F grant_type=authorization_code
	}
	if scfg.Token == "" {
		fmt.Printf(
			"curl -X POST https://www.strava.com/oauth/token?client_id=%d&client_secret=%s&code=%s&grant_type=authorization_code\n",
			scfg.ClientID,
			scfg.ClientSecret,
			scfg.Code,
		)
		os.Exit(0)
	}
	client := strava.NewClient(scfg.Token)
	fmt.Printf("%#v\n", *client)
	current := strava.NewCurrentAthleteService(client)
	fmt.Printf("%#v\n", *current)
	call := current.ListActivities()
	fmt.Printf("%#v\n", *call)
	activities, err := call.Do()
	if err != nil {
		// http://www.strava.com/oauth/authorize?client_id=[REPLACE_WITH_YOUR_CLIENT_ID]&response_type=code&redirect_uri=http://localhost/exchange_token&approval_prompt=force&scope=read_all,profile:read_all,activity:read_all
		fmt.Printf(
			"http://www.strava.com/oauth/authorize?client_id=%d&response_type=code&redirect_uri=http://localhost/exchange_token&approval_prompt=force&scope=activity:read_all\n",
			scfg.ClientID,
		)
		// $ curl -X POST https://www.strava.com/oauth/token \
		// -F client_id=5 \
		// -F client_secret=[REPLACE_WITH_YOUR_CLIENT_SECRET] \
		// -F code=c498932e64136c8991a3fb31e3d1dfdf2f859357
		// -F grant_type=authorization_code
		fmt.Printf(
			"curl -X POST 'https://www.strava.com/oauth/token?client_id=%d&client_secret=%s&code=%s&grant_type=authorization_code'\n",
			scfg.ClientID,
			scfg.ClientSecret,
			scfg.Code,
		)
		log.Fatal(err)
	}
	j, err := json.Marshal(activities)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(j))
	os.WriteFile("page0.json", j, 0644)
}
