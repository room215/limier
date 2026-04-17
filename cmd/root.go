package cmd

import (
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const appName = "limier"

var version = "dev"

func Execute() error {
	slog.SetDefault(newLogger())

	return newRootCommand().Execute()
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           appName,
		Short:         "Dependency behavior diff tool for fixture-based upgrade reviews",
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			slog.Debug("starting command", "command", cmd.CommandPath())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newRunCommand())
	cmd.AddCommand(newRenderCommand())
	cmd.AddCommand(newInspectCommand())

	return cmd
}

func newLogger() *slog.Logger {
	options := &slog.HandlerOptions{
		Level: parseLogLevel(os.Getenv("LIMIER_LOG_LEVEL")),
	}

	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LIMIER_LOG_FORMAT"))) {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, options)
	default:
		handler = slog.NewTextHandler(os.Stderr, options)
	}

	return slog.New(handler).With("app", appName)
}

func parseLogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
