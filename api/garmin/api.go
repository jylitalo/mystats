package garmin

type Config struct {
	Username   string `json:"username"    yaml:"username"`
	Password   string `json:"password"    yaml:"password"`
	DailySteps string `json:"daily_steps" yaml:"daily_steps"`
	HeartRate  string `json:"heart_rate"  yaml:"heart_rate"`
}
