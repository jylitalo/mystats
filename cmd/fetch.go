package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jylitalo/mystats/api"
	"github.com/jylitalo/mystats/config"
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

	status, errS := getJsonStatus(ctx)
	ctx, client, errC := getClient(ctx)
	if err := errors.Join(errS, errC); err != nil {
		return telemetry.Error(span, err)
	}
	call, err := callListActivities(ctx, client, status.latest)
	if err != nil {
		return telemetry.Error(span, err)
	}
	ids, apiCalls, err := saveActivities(ctx, call, status.pages)
	if err == nil && best_efforts {
		ids = append(ids, status.ids...)
		err = fetchBestEfforts(ctx, client, ids, apiCalls)
	}
	if err != nil && api.IsRateLimitExceeded(err) {
		slog.Warn("Strava API Rate Limit Exceeded")
		return nil
	}
	return telemetry.Error(span, err)
}

func getClient(ctx context.Context) (context.Context, *api.Client, error) {
	ctx, errR := config.Read(ctx, true)
	cfg, errG := config.Get(ctx)
	if err := errors.Join(errR, errG); err != nil {
		return ctx, nil, err
	}
	strava := cfg.Strava
	api.ClientId = strava.ClientID
	api.ClientSecret = strava.ClientSecret
	client := api.NewClient(strava.AccessToken)
	return ctx, client, nil
}

func fetchBestEfforts(ctx context.Context, client *api.Client, ids []int64, apiCalls int) error {
	ctx, span := telemetry.NewSpan(ctx, "fetchfetchBestEfforts")
	defer span.End()
	if len(ids) == 0 {
		return telemetry.Error(span, errors.New("no stravaIDs found from database"))
	}
	cfg, err := config.Get(ctx)
	if err != nil {
		return err
	}
	path := cfg.Strava.Activities
	actFiles, err := activitiesFiles(path)
	switch {
	case err != nil:
		return err
	case len(actFiles) == 0:
		if _, err = os.Stat(path); os.IsNotExist(err) {
			if err = os.Mkdir(path, 0750); err != nil {
				return telemetry.Error(span, err)
			}
		}
	}
	alreadyFetched := []int64{}
	for _, actFile := range actFiles {
		intStr := strings.Split(strings.Split(actFile, "_")[1], ".")[0]
		i, _ := strconv.Atoi(intStr)
		alreadyFetched = append(alreadyFetched, int64(i))
	}
	service := api.NewActivitiesService(ctx, client)
	for idx, id := range ids {
		if slices.Contains[[]int64, int64](alreadyFetched, id) {
			continue
		}
		if activity, err := service.Get(id).Do(); err != nil {
			return telemetry.Error(span, err)
		} else if data, err := json.Marshal(activity); err != nil {
			return telemetry.Error(span, err)
		} else {
			fname := fmt.Sprintf("%s/activity_%d.json ...\n", path, id)
			fmt.Printf("%s => %s ...\n", activity.StartDateLocal, fname)
			if err = os.WriteFile(fname, data, 0600); err != nil {
				return telemetry.Error(span, err)
			}
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
			if err = os.Mkdir(path, 0750); err != nil {
				return status, telemetry.Error(span, err)
			}
		}
	}
	status.pages = len(fnames)
	activities, err := api.ReadSummaryJSONs(fnames)
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

func callListActivities(ctx context.Context, client *api.Client, after time.Time) (*api.CurrentAthleteListActivitiesCall, error) {
	_, span := telemetry.NewSpan(ctx, "callListActivities")
	defer span.End()
	current := api.NewCurrentAthleteService(client)
	call := current.ListActivities()
	return call.After(int(after.Unix())), nil
}

func saveActivities(ctx context.Context, call *api.CurrentAthleteListActivitiesCall, prior int) ([]int64, int, error) {
	_, span := telemetry.NewSpan(ctx, "saveActivities")
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
			if api.IsRateLimitExceeded(err) {
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
		fname := fmt.Sprintf("%s/page%d.json", path, page+prior)
		fmt.Printf("%d activities => %s ...\n", len(activities), fname)
		if err = os.WriteFile(fname, content, 0600); err != nil {
			return newIds, page, telemetry.Error(span, err)
		}
		if len(activities) < 30 {
			return newIds, page, nil
		}
		page++
	}
}
