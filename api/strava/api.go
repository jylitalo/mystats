package strava

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

// Config requires that you modify Refresh() if you add new fields into struct
type Config struct {
	ClientID     int    `json:"client_id"     yaml:"clientID"`
	ClientSecret string `json:"client_secret" yaml:"clientSecret"`
	AccessToken  string `json:"access_token"  yaml:"accessToken"`
	RefreshToken string `json:"refresh_token" yaml:"refreshToken"`
	ExpiresAt    int64  `json:"expires_at"    yaml:"expiresAt"`
	Summaries    string `json:"summaries"     yaml:"summaries"`
	Activities   string `json:"activities"    yaml:"activities"`
}

const tokenURL string = "https://www.strava.com/oauth/token" // #nosec G101

func (cfg *Config) AuthorizationCode(code string) (*Config, error) {
	url := fmt.Sprintf(
		"%s?client_id=%d&client_secret=%s&code=%s&grant_type=authorization_code",
		tokenURL, cfg.ClientID, cfg.ClientSecret, code,
	)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
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
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return nil, false, err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = resp.Body.Close() }()
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
	tokens.Activities = cfg.Activities
	tokens.Summaries = cfg.Summaries
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

// ReadSummaryJSONs reads on pages JSON files
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
