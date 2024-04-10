package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

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
			flags := cmd.Flags()
			clientID, _ := flags.GetInt("client_id")
			if clientID == 0 {
				return errors.New("client_id argument missing")
			}
			clientSecret, _ := flags.GetString("client_secret")
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
				return err
			}
			stravaArgs := stravaURL.Query()
			code := stravaArgs["code"]
			if len(code) == 0 {
				return errors.New("code missing from authorize request")
			}
			slog.Debug("code from authorize", "code", code[0])
			tokens := &api.Config{ClientID: clientID, ClientSecret: clientSecret}
			if tokens, err = tokens.AuthorizationCode(code[0]); err != nil {
				return err
			}
			cfg := config.Config{Strava: *tokens}
			fname, err := cfg.Write()
			if err != nil {
				return err
			}
			fmt.Printf("Wrote configuration file into " + fname)
			return nil
		},
	}
	cmd.Flags().Int("client_id", 0, "Client ID from Strava")
	cmd.Flags().String("client_secret", "", "Client Secret from Strava")
	return cmd
}
