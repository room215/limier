package cmd

import (
	"fmt"
	"strings"

	"github.com/oneslash/limier/internal/render"
	"github.com/spf13/cobra"
)

type renderOptions struct {
	format     string
	inputPath  string
	outputPath string
}

func newRenderCommand() *cobra.Command {
	options := renderOptions{}

	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render a Limier report for a downstream review surface",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRender(options)
		},
	}

	cmd.Flags().StringVar(&options.format, "format", "", "Render format: github-comment, gitlab-note, build-summary")
	cmd.Flags().StringVar(&options.inputPath, "input", "", "Path to an existing report.json")
	cmd.Flags().StringVar(&options.outputPath, "output", "", "Optional path to write the rendered output")

	_ = cmd.MarkFlagRequired("format")
	_ = cmd.MarkFlagRequired("input")

	return cmd
}

func runRender(options renderOptions) error {
	runReport, err := loadReport(options.inputPath)
	if err != nil {
		return err
	}

	rendered, err := render.Render(runReport, strings.TrimSpace(options.format))
	if err != nil {
		return err
	}

	if err := writeCommandOutput(options.outputPath, rendered); err != nil {
		return fmt.Errorf("emit rendered output: %w", err)
	}

	return nil
}
