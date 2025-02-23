package garmin

type Config struct {
	Username   string `yaml:"username" json:"username"`
	Password   string `yaml:"password" json:"password"`
	DailySteps string `yaml:"daily_steps" json:"daily_steps"`
	HeartRate  string `yaml:"heart_rate" json:"heart_rate"`
}
