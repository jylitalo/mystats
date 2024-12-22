package cmd

import (
	"context"
	"errors"

	"github.com/jylitalo/mystats/config"
	"github.com/spf13/cobra"
)

func Execute(ctx context.Context) error {
	rootCmd := &cobra.Command{
		Use:   "mystats",
		Short: "mystats is tool for fetching your Strava results to your machine",
	}
	types := []string{"Run", "Trail Run"}
	ctx, errR := config.Read(ctx, false)
	cfg, errG := config.Get(ctx)
	if err := errors.Join(errR, errG); err == nil {
		types = cfg.Default.Types
	}
	rootCmd.AddCommand(
		configureCmd(), fetchCmd(), makeCmd(),
		bestCmd(), listCmd(types), statsCmd(types), topCmd(types),
		serverCmd(types),
	)
	return rootCmd.ExecuteContext(ctx)
}
