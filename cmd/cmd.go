package cmd

import (
	"github.com/jylitalo/mystats/config"
	"github.com/spf13/cobra"
)

func Execute() error {
	rootCmd := &cobra.Command{
		Use:   "mystats",
		Short: "mystats is tool for fetching your Strava results to your machine",
	}
	types := []string{"Run", "Trail Run"}
	if cfg, err := config.Get(false); err == nil {
		types = cfg.Default.Types
	}
	rootCmd.AddCommand(configureCmd(), fetchCmd(), makeCmd(), listCmd(types), plotCmd(types), statsCmd(types), topCmd(types))
	return rootCmd.Execute()
}
