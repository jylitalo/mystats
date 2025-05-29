package strava

// Copied from https://github.com/strava/go.strava/blob/99ebe972ba16ef3e1b1e5f62003dae3ac06f3adb/service.go
// as dependency for current_athlete.go
import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	strava "github.com/strava/go.strava"
)

// ClientID is the Strava application client ID.
var ClientID int

// ClientSecret is the Strava application client secret.
var ClientSecret string

const basePath = "https://www.strava.com/api/v3"

type Client struct {
	token      string
	httpClient *http.Client
}

type ErrorHandler func(*http.Response) error

var defaultErrorHandler ErrorHandler = func(resp *http.Response) error {
	// check status code, could be 500, or most likely the client_secret is incorrect
	if resp.StatusCode/100 == 5 {
		return errors.New("server error")
	}

	if resp.StatusCode/100 == 4 {
		var response strava.Error
		contents, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(contents, &response); err != nil {
			return err
		}

		return response
	}

	if resp.StatusCode/100 == 3 {
		return errors.New("redirect error")
	}
	return nil
}

// NewClient builds a normal client for making requests to the strava api.
// a http.Client can be passed in if http.DefaultClient can not be used.
func NewClient(token string, client ...*http.Client) *Client {
	c := &Client{token: token}
	if len(client) != 0 {
		c.httpClient = client[0]
	} else {
		c.httpClient = http.DefaultClient
	}
	return c
}

// NewStubResponseClient can be used for testing
func NewStubResponseClient(content string, statusCode ...int) *Client {
	c := NewClient("")
	t := &stubResponseTransport{content: content}

	if len(statusCode) != 0 {
		t.statusCode = statusCode[0]
	}

	c.httpClient = &http.Client{Transport: t}

	return c
}

type stubResponseTransport struct {
	http.Transport
	content    string
	statusCode int
}

func (t *stubResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{
		Status:     http.StatusText(t.statusCode),
		StatusCode: t.statusCode,
	}
	resp.Body = io.NopCloser(strings.NewReader(t.content))

	return resp, nil
}

func (client *Client) run(method, path string, params map[string]interface{}) ([]byte, error) {
	var err error

	values := make(url.Values)
	for k, v := range params {
		values.Set(k, fmt.Sprintf("%v", v))
	}

	var req *http.Request
	if method == "POST" {
		req, err = http.NewRequest(http.MethodPost, basePath+path, strings.NewReader(values.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req, err = http.NewRequest(method, basePath+path+"?"+values.Encode(), nil)
		if err != nil {
			return nil, err
		}
	}

	return client.runRequest(req)
}

func (client *Client) runRequestWithErrorHandler(req *http.Request, errorHandler ErrorHandler) ([]byte, error) {
	req.Header.Set("Authorization", "Bearer "+client.token)
	req.Header.Set("User-Agent", "Strava Golang Library v1")
	resp, err := client.httpClient.Do(req)
	// this was a poor request, maybe strava servers down?
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	RateLimiting.updateRateLimits(resp)

	return checkResponseForErrorsWithErrorHandler(resp, errorHandler)
}

func (client *Client) runRequest(req *http.Request) ([]byte, error) {
	return client.runRequestWithErrorHandler(req, defaultErrorHandler)
}

func checkResponseForErrorsWithErrorHandler(resp *http.Response, errorHandler ErrorHandler) ([]byte, error) {
	if resp.StatusCode/100 > 2 {
		return nil, errorHandler(resp)
	} else {
		return io.ReadAll(resp.Body)
	}
}
