package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"

	"github.com/jylitalo/mystats/api"
)

type Config struct {
	Strava api.Config `yaml:"strava"`
}

func Get() (api.Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return api.Config{}, fmt.Errorf("error in UserHomeDir: %w", err)
	}
	vip := viper.GetViper()
	vip.AddConfigPath(home)
	vip.SetConfigName(".mystats")
	if err = vip.ReadInConfig(); err != nil {
		return api.Config{}, fmt.Errorf("error in getConfig: %w", err)
	}
	return api.Config{
		ClientID:     vip.GetInt("strava.clientID"),
		ClientSecret: vip.GetString("strava.clientSecret"),
		AccessToken:  vip.GetString("strava.accessToken"),
		RefreshToken: vip.GetString("strava.refreshToken"),
	}, nil
}
