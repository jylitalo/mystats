package cmd

import (
	"log/slog"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"

	"github.com/jylitalo/mystats/server"
)

// serverCmd turns sqlite db into table or csv by week/month/...
func serverCmd(types []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start web service",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			port, _ := flags.GetInt("port")
			db, err := makeDB()
			if err != nil {
				return err
			}
			defer db.Close()
			slog.Info("start service", "port", port)
			return server.Start(db, types, port)
		},
	}
	cmd.Flags().Int("port", 8000, "Port number for service")
	return cmd
}
