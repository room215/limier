package cmd

import (
	"log/slog"

	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the Limier version",
		Run: func(cmd *cobra.Command, args []string) {
			slog.Debug("reporting version", "version", version)
			cmd.Printf("%s %s\n", appName, version)
		},
	}
}
