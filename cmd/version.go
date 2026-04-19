package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the Limier version",
		Run: func(cmd *cobra.Command, args []string) {
			slog.Debug("reporting version", "version", version)
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", appName, version)
		},
	}
}
