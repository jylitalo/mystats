package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"

	"github.com/jylitalo/mystats/api"
)

type Config struct {
	Strava api.Config `yaml:"strava"`
}

func Get() (*api.Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("error in UserHomeDir: %w", err)
	}
	vip := viper.GetViper()
	vip.AddConfigPath(home)
	vip.SetConfigName(".mystats")
	if err = vip.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error in getConfig: %w", err)
	}
	tokens := &api.Config{
		ClientID:     vip.GetInt("strava.clientID"),
		ClientSecret: vip.GetString("strava.clientSecret"),
		AccessToken:  vip.GetString("strava.accessToken"),
		RefreshToken: vip.GetString("strava.refreshToken"),
		ExpiresAt:    int64(vip.GetInt("strava.expiresAt")),
	}
	tokens, changes, err := tokens.Refresh()
	if err == nil && changes {
		cfg := Config{Strava: *tokens}
		if _, err := cfg.Write(); err != nil {
			return nil, err
		}
	}
	return tokens, nil
}

func (cfg *Config) Write() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	fname := filepath.Join(home, ".mystats.yaml")
	text, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return fname, os.WriteFile(fname, text, 0600)
}
