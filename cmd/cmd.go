package cmd

import (
	"context"

	"github.com/jylitalo/mystats/config"
	"github.com/spf13/cobra"
)

func Execute(ctx context.Context) error {
	rootCmd := &cobra.Command{
		Use:   "mystats",
		Short: "mystats is tool for fetching your Strava results to your machine",
	}
	types := []string{"Run", "Trail Run"}
	if cfg, err := config.Get(false); err == nil {
		types = cfg.Default.Types
	}
	rootCmd.AddCommand(
		configureCmd(), fetchCmd(), makeCmd(),
		bestCmd(), listCmd(types), plotCmd(types), statsCmd(types), topCmd(types),
		serverCmd(types),
	)
	return rootCmd.ExecuteContext(ctx)
}
