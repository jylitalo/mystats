package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type Config struct {
	ClientID     int    `yaml:"clientID" json:"client_id"`
	ClientSecret string `yaml:"clientSecret" json:"client_secret"`
	AccessToken  string `yaml:"accessToken" json:"access_token"`
	RefreshToken string `yaml:"refreshToken" json:"refresh_token"`
	ExpiresAt    int64  `yaml:"expiresAt" json:"expires_at"`
}

const tokenURL string = "https://www.strava.com/oauth/token"

func (cfg *Config) AuthorizationCode(code string) (*Config, error) {
	url := fmt.Sprintf(
		"%s?client_id=%d&client_secret=%s&code=%s&grant_type=authorization_code",
		tokenURL, cfg.ClientID, cfg.ClientSecret, code,
	)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	slog.Debug("body from token", "body", string(body))
	tokens := Config{}
	if err = json.Unmarshal(body, &tokens); err != nil {
		return nil, err
	}
	tokens.ClientID = cfg.ClientID
	tokens.ClientSecret = cfg.ClientSecret
	return &tokens, nil
}

func (cfg *Config) Refresh() (*Config, bool, error) {
	now := time.Now().Unix()
	if cfg.ExpiresAt > now {
		return nil, false, nil
	}
	url := fmt.Sprintf(
		"%s?client_id=%d&client_secret=%s&refresh_token=%s&grant_type=refresh_token",
		tokenURL, cfg.ClientID, cfg.ClientSecret, cfg.RefreshToken,
	)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, false, err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	slog.Debug("body from token", "body", string(body))
	tokens := Config{}
	if err = json.Unmarshal(body, &tokens); err != nil {
		return nil, false, err
	}
	tokens.ClientID = cfg.ClientID
	tokens.ClientSecret = cfg.ClientSecret
	return &tokens, true, nil
}
