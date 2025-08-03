package garmin

import (
	"fmt"

	garmin "github.com/jylitalo/go-garmin"
)

func NewAPI(username, password string) (*garmin.API, error) {
	client := garmin.NewClient()
	if err := client.Login(username, password); err != nil {
		return nil, fmt.Errorf("Garmin login returned: %w", err) //nolint:staticcheck // Garmin is name
	}
	return garmin.NewAPI(client), nil
}
