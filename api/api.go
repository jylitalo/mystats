package api

type Config struct {
	ClientID     int    `yaml:"clientID" json:"client_id"`
	ClientSecret string `yaml:"clientSecret" json:"client_secret"`
	AccessToken  string `yaml:"accessToken" json:"access_token"`
	RefreshToken string `yaml:"refreshToken" json:"refresh_token"`
}
