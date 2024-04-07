package cmd

import (
	"github.com/spf13/cobra"
)

func Execute() error {
	rootCmd := &cobra.Command{
		Use:   "mystats",
		Short: "mystats is tool for fetching your Strava results to your machine",
	}
	rootCmd.AddCommand(configureCmd(), fetchCmd(), makeCmd())
	return rootCmd.Execute()
}
