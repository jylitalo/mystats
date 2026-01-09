package cmd

import (
	"context"
	"fmt"

	"github.com/jylitalo/mystats/config"
	"github.com/spf13/cobra"
)

func Execute(ctx context.Context) error {
	rootCmd := &cobra.Command{
		Use:   "mystats",
		Short: "mystats is tool for fetching your Strava results to your machine",
	}
	types := []string{"Run", "Trail Run"}
	ctx, err := config.Read(ctx, false)
	if err != nil {
		return fmt.Errorf("config.Read due to %w", err)
	}
	cfg, err := config.Get(ctx)
	if err != nil {
		return fmt.Errorf("config.Get due to %w", err)
	}
	types = cfg.Default.Types
	rootCmd.AddCommand(
		configureCmd(), fetchCmd(), makeCmd(),
		bestCmd(), listCmd(types), statsCmd(types), topCmd(types),
		serverCmd(types),
	)
	return rootCmd.ExecuteContext(ctx)
}
