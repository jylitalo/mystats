package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/jylitalo/mystats/api"
	"github.com/jylitalo/mystats/config"
)

// configureCmd is based on instructions from
// https://yizeng.me/2017/01/11/get-a-strava-api-access-token-with-write-permission/
// Basic idea is to elevate priviledges from `read` to `activity:read_all`
func configureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configure --client_id=[int] --client_secret=[string]",
		Short: "Create config file for mystat",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientID, _ := cmd.Flags().GetInt("client_id")
			if clientID == 0 {
				return errors.New("client_id argument missing")
			}
			clientSecret, _ := cmd.Flags().GetString("client_secret")
			if clientSecret == "" {
				return errors.New("client_secret argument missing")
			}
			fmt.Printf(
				"Go to https://www.strava.com/oauth/authorize?client_id=%d&response_type=code&redirect_uri=http://localhost/exchange_token&approval_prompt=force&scope=activity:read_all\n",
				clientID,
			)
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Strava redirected you to: ")
			text, _ := reader.ReadString('\n')
			stravaURL, err := url.ParseRequestURI(strings.TrimSpace(text))
			if err != nil {
				log.Fatal(err)
			}
			stravaArgs := stravaURL.Query()
			code := stravaArgs["code"]
			if len(code) == 0 {
				return errors.New("code missing from authorize request")
			}
			slog.Debug("code from authorize", "code", code[0])
			url := fmt.Sprintf(
				"https://www.strava.com/oauth/token?client_id=%d&client_secret=%s&code=%s&grant_type=authorization_code",
				clientID, clientSecret, code[0],
			)
			req, err := http.NewRequest("POST", url, nil)
			if err != nil {
				log.Fatal(err)
			}
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				log.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Fatal(err)
			}
			slog.Debug("body from token", "body", string(body))
			tokens := api.Config{}
			if err = json.Unmarshal(body, &tokens); err != nil {
				log.Fatal(err)
			}
			tokens.ClientID = clientID
			tokens.ClientSecret = clientSecret
			cfgText, err := yaml.Marshal(config.Config{
				Strava: tokens,
			})
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(string(cfgText))
			return nil
		},
	}
	cmd.Flags().Int("client_id", 0, "Client ID from Strava")
	cmd.Flags().String("client_secret", "", "Client Secret from Strava")
	return cmd
}
