package garmin

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"time"

	garmin "github.com/jylitalo/go-garmin"
)

func HeartRate(user *garmin.UserSummaryService, all bool) (map[string]garmin.HeartRateStat, error) {
	hrs := map[string]garmin.HeartRateStat{}
	end := time.Now()
	for {
		start := end.Add(-26 * 24 * time.Hour)
		resp, err := user.DailyHeartRate(start, end)
		for _, hr := range resp {
			hrs[hr.CalendarDate] = hr.Values
		}
		if !all || len(resp) == 0 || err != nil {
			return hrs, err
		}
		end = start
	}
}

func ReadHeartRateJSONs(ctx context.Context, fnames []string) (map[string]garmin.HeartRateStat, error) {
	slog.Info("ReadHeartRateJSONs", "fnames", fnames)
	values := map[string]garmin.HeartRateStat{}
	for _, fname := range fnames {
		content, err := os.ReadFile(filepath.Clean(fname))
		if err != nil {
			return values, err
		}
		oneSet := map[string]garmin.HeartRateStat{}
		if err = json.Unmarshal(content, &oneSet); err != nil {
			return values, err
		}
		maps.Copy(values, oneSet)
	}
	return values, nil
}
