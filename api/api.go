package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jylitalo/mystats/pkg/telemetry"
	strava "github.com/strava/go.strava"
)

type Config struct {
	ClientID     int    `yaml:"clientID" json:"client_id"`
	ClientSecret string `yaml:"clientSecret" json:"client_secret"`
	AccessToken  string `yaml:"accessToken" json:"access_token"`
	RefreshToken string `yaml:"refreshToken" json:"refresh_token"`
	ExpiresAt    int64  `yaml:"expiresAt" json:"expires_at"`
}

const tokenURL string = "https://www.strava.com/oauth/token" // #nosec G101

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
		return cfg, false, nil
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

func ReadActivityJSONs(ctx context.Context, fnames []string) ([]strava.ActivityDetailed, error) {
	_, span := telemetry.NewSpan(ctx, "api.ReadActivityJSONs")
	defer span.End()

	acts := []strava.ActivityDetailed{}
	for _, fname := range fnames {
		body, err := os.ReadFile(filepath.Clean(fname))
		if err != nil {
			return acts, telemetry.Error(span, err)
		}
		activity := strava.ActivityDetailed{}
		if err = json.Unmarshal(body, &activity); err != nil {
			return acts, telemetry.Error(span, err)
		}
		acts = append(acts, activity)
	}
	return acts, nil
}

func ReadSummaryJSONs(fnames []string) ([]ActivitySummary, error) {
	ids := map[int64]string{}
	activities := []ActivitySummary{}
	for _, fname := range fnames {
		body, err := os.ReadFile(filepath.Clean(fname))
		if err != nil {
			return activities, err
		}
		page := []ActivitySummary{}
		if err = json.Unmarshal(body, &page); err != nil {
			return activities, err
		}
		for _, p := range page {
			if val, ok := ids[p.Id]; ok {
				slog.Error("id exists in multiple pages", "id", p.Id, "current", fname, "previous", val)
			} else {
				ids[p.Id] = fname
				activities = append(activities, p)
			}
		}
	}
	return activities, nil
}
