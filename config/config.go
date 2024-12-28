package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/jylitalo/mystats/api/garmin"
	"github.com/jylitalo/mystats/api/strava"
)

type Config struct {
	Garmin  *garmin.Config `yaml:"garmin"`
	Strava  *strava.Config `yaml:"strava"`
	Default struct {
		Types []string `yaml:"types"`
	} `yaml:"default"`
}

type configCtxKey string

const configKey configCtxKey = "mystats.config"

func configFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error in UserHomeDir: %w", err)
	}
	return home + "/.mystats.yaml", nil
}

func Get(ctx context.Context) (*Config, error) {
	cfg := ctx.Value(configKey)
	if cfg == nil {
		return nil, errors.New("config not found from context")
	}
	return cfg.(*Config), nil
}
func Read(ctx context.Context, refresh bool) (context.Context, error) {
	setLogger()
	fname, err := configFile()
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(filepath.Clean(fname))
	if err != nil {
		return nil, fmt.Errorf("error in reading .mystats.yaml")
	}
	cfg := Config{Strava: &strava.Config{}}
	if err = yaml.Unmarshal(body, &cfg); err != nil {
		return nil, fmt.Errorf("error in parsing .mystats.yaml")
	}
	if cfg.Garmin.DailySteps == "" {
		cfg.Garmin.DailySteps = "daily_steps"
	}
	if cfg.Strava.Activities == "" {
		cfg.Strava.Activities = "activities"
	}
	if cfg.Strava.Summaries == "" {
		cfg.Strava.Summaries = "pages"
	}
	ctx = context.WithValue(ctx, configKey, &cfg)
	if !refresh {
		return ctx, nil
	}
	tokens, changes, err := cfg.Strava.Refresh()
	if err == nil && changes {
		cfg.Strava = tokens
		_, err = cfg.Write()
	}
	return ctx, err
}

func setLogger() {
	lvl := &slog.LevelVar{}
	// lvl.Set(slog.LevelDebug)
	lvl.Set(slog.LevelInfo)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: lvl,
	}))
	slog.SetDefault(logger)
}

func (cfg *Config) Write() (string, error) {
	fname, err := configFile()
	if err != nil {
		return "", err
	}
	text, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return fname, os.WriteFile(fname, text, 0600)
}
