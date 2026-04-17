package main

import (
	"errors"
	"log/slog"
	"os"

	"github.com/room215/limier/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		var exitErr interface {
			ExitCode() int
		}
		if errors.As(err, &exitErr) {
			if err.Error() != "" {
				slog.Error("command failed", "error", err)
			}
			os.Exit(exitErr.ExitCode())
		}

		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}
