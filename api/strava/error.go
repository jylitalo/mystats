package strava

import (
	"encoding/json"
)

type StravaError struct {
	Message string      `json:"message"`
	Errors  interface{} `json:"errors"`
}

func (e StravaError) Error() string {
	return e.Message
}

func GetStravaError(err error) (*StravaError, error) {
	errString := error.Error(err)
	serr := StravaError{}
	if errMarshal := json.Unmarshal([]byte(errString), &serr); errMarshal != nil {
		return nil, err
	}
	return &serr, nil
}

func IsRateLimitExceeded(err error) bool {
	serr, ok := GetStravaError(err)
	if ok == nil && serr.Message == "Rate Limit Exceeded" {
		return true
	}
	return false
}
