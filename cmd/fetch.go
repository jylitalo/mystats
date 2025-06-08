package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	gogarmin "github.com/jylitalo/go-garmin"
	"github.com/jylitalo/mystats/api/garmin"
	"github.com/jylitalo/mystats/api/strava"
	"github.com/jylitalo/mystats/config"
	"github.com/jylitalo/mystats/pkg/data"
	"github.com/jylitalo/mystats/pkg/telemetry"
)

type jsonStatus struct {
	latest time.Time
	pages  int
	ids    []int64
}

// fetchCmd fetches activity data from Strava
func fetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch activity data from Strava to JSON files",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			be, _ := flags.GetBool("best_efforts")
			return fetch(cmd.Context(), be)
		},
	}
	cmd.Flags().Bool("best_efforts", true, "Fetch activities best efforts")
	return cmd
}

func fetch(ctx context.Context, best_efforts bool) error {
	ctx, span := telemetry.NewSpan(ctx, "fetch")
	defer span.End()

	ctx, err := config.Read(ctx, true)
	if err != nil {
		return telemetry.Error(span, err)
	}
	cfg, err := config.Get(ctx)
	if err != nil {
		return telemetry.Error(span, err)
	}
	garminClient, err := garmin.NewAPI(cfg.Garmin.Username, cfg.Garmin.Password)
	if err != nil {
		return telemetry.Error(span, fmt.Errorf("garmin.NewAPI returned %w", err))
	}
	errSteps := getDailySteps(ctx, garminClient, cfg.Garmin.DailySteps)
	errHR := getHeartRate(ctx, garminClient, cfg.Garmin.HeartRate)
	status, errStatus := getJsonStatus(ctx)
	ctx, stravaClient, errC := getStravaClient(ctx)
	if err := errors.Join(errSteps, errHR, errStatus, errC); err != nil {
		return telemetry.Error(span, err)
	}
	call, err := callListActivities(ctx, stravaClient, status.latest)
	if err != nil {
		return telemetry.Error(span, err)
	}
	ids, apiCalls, err := saveStravaSummaries(ctx, call, status.pages)
	if err == nil && best_efforts {
		ids = append(ids, status.ids...)
		err = fetchActivityDetails(ctx, stravaClient, ids, apiCalls)
	}
	if err != nil && strava.IsRateLimitExceeded(err) {
		slog.Warn("Strava API Rate Limit Exceeded")
		return nil
	}
	return telemetry.Error(span, err)
}

func getHeartRate(ctx context.Context, client *gogarmin.API, path string) error {
	_, span := telemetry.NewSpan(ctx, "getHRs")
	defer span.End()
	all := false
	if path == "" {
		return telemetry.Error(span, errors.New("path is empty"))
	}
	hrFiles, err := heartRateFiles(path)
	switch {
	case err != nil:
		return err
	case len(hrFiles) == 0:
		if _, err = os.Stat(path); os.IsNotExist(err) {
			if err = os.Mkdir(path, 0o750); err != nil {
				err = fmt.Errorf("mkdir '%s' failed due to %w", path, err)
				return telemetry.Error(span, err)
			}
		}
		all = true
	}
	data, err := garmin.HeartRate(client.UserSummary, all)
	if err != nil {
		return err
	}
	fname := fmt.Sprintf("%s/hr_%d.json", path, len(hrFiles)+1)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(fname, jsonData, 0o600)
}

func mkdir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err = os.Mkdir(path, 0o750); err != nil {
			return fmt.Errorf("mkdir '%s' failed due to %w", path, err)
		}
	}
	return nil
}

func getDailySteps(ctx context.Context, client *gogarmin.API, path string) error {
	_, span := telemetry.NewSpan(ctx, "getDailySteps")
	defer span.End()
	if path == "" {
		return telemetry.Error(span, errors.New("path is empty"))
	}
	stepsFiles, errS := stepsFiles(path)
	if err := errors.Join(errS, mkdir(path)); err != nil {
		return telemetry.Error(span, err)
	}
	steps, err := garmin.DailySteps(client.UserSummary, len(stepsFiles) == 0)
	if err != nil {
		return err
	}
	for idx, val := range steps {
		fname := fmt.Sprintf("%s/steps_%d.json", path, len(stepsFiles)+idx+1)
		data, err := json.Marshal(val)
		if err != nil {
			return err
		}
		if err := os.WriteFile(fname, data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func heartRateFiles(path string) ([]string, error) {
	return filepath.Glob(path + "/hr*.json")
}

func stepsFiles(path string) ([]string, error) {
	return filepath.Glob(path + "/steps*.json")
}

func getStravaClient(ctx context.Context) (context.Context, *strava.Client, error) {
	ctx, span := telemetry.NewSpan(ctx, "getStravaClient")
	defer span.End()
	cfg, err := config.Get(ctx)
	if err != nil {
		return ctx, nil, err
	}
	stravaCfg := cfg.Strava
	strava.ClientID = stravaCfg.ClientID
	strava.ClientSecret = stravaCfg.ClientSecret
	client := strava.NewClient(stravaCfg.AccessToken)
	return ctx, client, nil
}

func fetchActivityDetails(ctx context.Context, client *strava.Client, ids []int64, apiCalls int) error {
	ctx, span := telemetry.NewSpan(ctx, "fetchBestEfforts")
	defer span.End()
	if len(ids) == 0 {
		return telemetry.Error(span, errors.New("no stravaIDs found from database"))
	}
	cfg, err := config.Get(ctx)
	if err != nil {
		return err
	}
	path := cfg.Strava.Activities
	if path == "" {
		return telemetry.Error(span, errors.New("path is empty"))
	}
	errPath := mkdir(path)
	alreadyFetched, errAct := alreadyFetchedDetails(path)
	if err = errors.Join(errPath, errAct); err != nil {
		return telemetry.Error(span, err)
	}
	service := strava.NewActivitiesService(ctx, client)
	for idx, id := range data.Reduce(ids, alreadyFetched) {
		activity, err := service.Get(id).Do()
		if err != nil {
			return telemetry.Error(span, err)
		}
		data, err := json.Marshal(activity)
		if err != nil {
			return telemetry.Error(span, err)
		}
		if err = os.WriteFile(fmt.Sprintf("%s/activity_%d.json", path, id), data, 0o600); err != nil {
			return telemetry.Error(span, err)
		}
		if apiCalls++; apiCalls >= 90 {
			slog.Info("Already fetched 90 activities", "left", len(ids)-idx)
			return nil
		}
	}
	slog.Info("Activity details fetched", "fetched", apiCalls)
	return nil
}

func activitiesFiles(path string) ([]string, error) {
	return filepath.Glob(path + "/activity_*.json")
}

func alreadyFetchedDetails(path string) ([]int64, error) {
	files, err := activitiesFiles(path)
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	for _, actFile := range files {
		i, err := strconv.Atoi(strings.Split(strings.Split(actFile, "_")[1], ".")[0])
		if err != nil {
			return nil, err
		}
		ids = append(ids, int64(i))
	}
	return ids, nil
}

func pageFiles(path string) ([]string, error) {
	return filepath.Glob(path + "/page*.json")
}

func getJsonStatus(ctx context.Context) (jsonStatus, error) {
	ctx, span := telemetry.NewSpan(ctx, "getJsonStatus")
	defer span.End()
	status := jsonStatus{
		latest: time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC),
		pages:  0,
		ids:    []int64{},
	}
	cfg, err := config.Get(ctx)
	if err != nil {
		return status, telemetry.Error(span, err)
	}
	path := cfg.Strava.Summaries
	fnames, err := pageFiles(path)
	switch {
	case err != nil:
		return status, err
	case len(fnames) == 0:
		if _, err = os.Stat(path); os.IsNotExist(err) {
			if err = os.Mkdir(path, 0o750); err != nil {
				return status, telemetry.Error(span, err)
			}
		}
	}
	status.pages = len(fnames)
	activities, err := strava.ReadSummaryJSONs(fnames)
	if err != nil {
		return status, err
	}
	for _, act := range activities {
		if act.StartDateLocal.After(status.latest) {
			status.latest = act.StartDateLocal
		}
		status.ids = append(status.ids, act.Id)
	}
	return status, nil
}

func callListActivities(
	ctx context.Context, client *strava.Client, after time.Time,
) (*strava.CurrentAthleteListActivitiesCall, error) {
	_, span := telemetry.NewSpan(ctx, "callListActivities")
	defer span.End()
	current := strava.NewCurrentAthleteService(client)
	call := current.ListActivities()
	return call.After(int(after.Unix())), nil
}

func saveStravaSummaries( //nolint:cyclop
	ctx context.Context, call *strava.CurrentAthleteListActivitiesCall, prior int,
) ([]int64, int, error) {
	_, span := telemetry.NewSpan(ctx, "saveStravaSummaries")
	defer span.End()
	page := 1
	newIds := []int64{}
	cfg, err := config.Get(ctx)
	if err != nil {
		return newIds, page, telemetry.Error(span, err)
	}
	path := cfg.Strava.Summaries
	for {
		call = call.Page(page)
		activities, err := call.Do()
		if err != nil {
			if strava.IsRateLimitExceeded(err) {
				return newIds, page, telemetry.Error(span, err)
			}
			if page == 1 {
				return newIds, page, fmt.Errorf("run mystats configure --client_id=... --client_secret=... first. err=%w", err)
			}
			return newIds, page, err
		}
		if len(activities) == 0 {
			return newIds, page, telemetry.Error(span, err)
		}
		content, err := json.Marshal(activities)
		if err != nil {
			return newIds, page, telemetry.Error(span, err)
		}
		for _, act := range activities {
			newIds = append(newIds, act.Id)
		}
		if err = os.WriteFile(fmt.Sprintf("%s/page%d.json", path, page+prior), content, 0o600); err != nil {
			return newIds, page, telemetry.Error(span, err)
		}
		if len(activities) < 30 {
			return newIds, page, nil
		}
		page++
	}
}
