package garmin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	garmin "github.com/jylitalo/go-garmin"
)

func DailySteps(user *garmin.UserSummaryService, all bool) ([]garmin.DailySteps, error) {
	steps := []garmin.DailySteps{}
	end := time.Now()
	for {
		start := end.Add(-26 * 24 * time.Hour)
		resp, err := user.DailySteps(start, end)
		if resp != nil {
			steps = append(steps, *resp)
		}
		if !all || len(resp.Values) == 0 || err != nil {
			return steps, err
		}
		end = start
	}
}

func ReadDailyStepsJSONs(ctx context.Context, fnames []string) (map[string]garmin.DailyStepsStat, error) {
	values := map[string]garmin.DailyStepsStat{}
	for _, fname := range fnames {
		content, err := os.ReadFile(filepath.Clean(fname))
		if err != nil {
			return values, err
		}
		oneSet := garmin.DailySteps{}
		if err = json.Unmarshal(content, &oneSet); err != nil {
			return values, err
		}
		for _, oneDay := range oneSet.Values {
			day := oneDay.CalendarDate
			if val, ok := values[day]; ok {
				val.StepGoal = max(val.StepGoal, oneDay.Values.StepGoal)
				val.TotalSteps = max(val.TotalSteps, oneDay.Values.TotalSteps)
				values[day] = val
			} else {
				values[day] = garmin.DailyStepsStat{
					StepGoal:   oneDay.Values.StepGoal,
					TotalSteps: oneDay.Values.TotalSteps,
				}
			}
		}
	}
	return values, nil
}
