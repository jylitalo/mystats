package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/jylitalo/mystats/api"
)

type Config struct {
	Strava  api.Config `yaml:"strava"`
	Default struct {
		Types []string `yaml:"types"`
	} `yaml:"default"`
}

func configFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error in UserHomeDir: %w", err)
	}
	return home + "/.mystats.yaml", nil
}

func Get(refresh bool) (*Config, error) {
	setLogger()
	fname, err := configFile()
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(filepath.Clean(fname))
	if err != nil {
		return nil, fmt.Errorf("error in reading .mystats.yaml")
	}
	cfg := Config{}
	if err = yaml.Unmarshal(body, &cfg); err != nil {
		return nil, fmt.Errorf("error in parsing .mystats.yaml")
	}
	if !refresh {
		return &cfg, nil
	}
	tokens, changes, err := cfg.Strava.Refresh()
	if err == nil && changes {
		cfg.Strava = *tokens
		_, err = cfg.Write()
	}
	return &cfg, err
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
