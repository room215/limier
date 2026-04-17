package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/room215/limier/internal/report"
)

func loadReport(path string) (report.Report, error) {
	return report.ReadJSON(strings.TrimSpace(path))
}

func writeCommandOutput(path string, contents string) error {
	if !strings.HasSuffix(contents, "\n") {
		contents += "\n"
	}

	if strings.TrimSpace(path) == "" {
		_, err := os.Stdout.WriteString(contents)
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory for %q: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		return fmt.Errorf("write output %q: %w", path, err)
	}

	return nil
}
