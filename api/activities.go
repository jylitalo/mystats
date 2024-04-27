package api

// Copied from https://github.com/strava/go.strava/blob/99ebe972ba16ef3e1b1e5f62003dae3ac06f3adb/activities.go
// so that we were able to add WorkoutType into ActivitySummary struct.
// workout_type is documented attribute in Strava API v3, but for some reason it is missing from go.strava
// workout_type is integer value that needs separate transformation into string.
import (
	"fmt"
	"time"

	strava "github.com/strava/go.strava"
)

type ActivitySummary struct {
	Id                 int64                 `json:"id"`
	ExternalId         string                `json:"external_id"`
	UploadId           int64                 `json:"upload_id"`
	Athlete            strava.AthleteSummary `json:"athlete"`
	Name               string                `json:"name"`
	Distance           float64               `json:"distance"`
	MovingTime         int                   `json:"moving_time"`
	ElapsedTime        int                   `json:"elapsed_time"`
	TotalElevationGain float64               `json:"total_elevation_gain"`
	Type               strava.ActivityType   `json:"type"`

	StartDate      time.Time `json:"start_date"`
	StartDateLocal time.Time `json:"start_date_local"`

	TimeZone         string          `json:"time_zone"`
	StartLocation    strava.Location `json:"start_latlng"`
	EndLocation      strava.Location `json:"end_latlng"`
	City             string          `json:"location_city"`
	State            string          `json:"location_state"`
	Country          string          `json:"location_country"`
	AchievementCount int             `json:"achievement_count"`
	KudosCount       int             `json:"kudos_count"`
	CommentCount     int             `json:"comment_count"`
	AthleteCount     int             `json:"athlete_count"`
	PhotoCount       int             `json:"photo_count"`
	Map              struct {
		Id              string          `json:"id"`
		Polyline        strava.Polyline `json:"polyline"`
		SummaryPolyline strava.Polyline `json:"summary_polyline"`
	} `json:"map"`
	Trainer              bool    `json:"trainer"`
	Commute              bool    `json:"commute"`
	Manual               bool    `json:"manual"`
	Private              bool    `json:"private"`
	Flagged              bool    `json:"flagged"`
	GearId               string  `json:"gear_id"` // bike or pair of shoes
	AverageSpeed         float64 `json:"average_speed"`
	MaximunSpeed         float64 `json:"max_speed"`
	AverageCadence       float64 `json:"average_cadence"`
	AverageTemperature   float64 `json:"average_temp"`
	AveragePower         float64 `json:"average_watts"`
	WeightedAveragePower int     `json:"weighted_average_watts"`
	Kilojoules           float64 `json:"kilojoules"`
	DeviceWatts          bool    `json:"device_watts"`
	AverageHeartrate     float64 `json:"average_heartrate"`
	MaximumHeartrate     float64 `json:"max_heartrate"`
	Truncated            int     `json:"truncated"` // only present if activity is owned by authenticated athlete, returns 0 if not truncated by privacy zones
	HasKudoed            bool    `json:"has_kudoed"`

	WorkoutTypeId int `json:"workout_type"`
}

func (as *ActivitySummary) WorkoutType() string {
	options := []string{"", "Race", "Long Run", "Workout"}
	if as.WorkoutTypeId < len(options) {
		return options[as.WorkoutTypeId]
	}
	return fmt.Sprintf("Unknown (%d)", as.WorkoutTypeId)
}
