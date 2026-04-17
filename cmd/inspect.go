package cmd

import (
	"fmt"

	"github.com/oneslash/limier/internal/render"
	"github.com/spf13/cobra"
)

type inspectOptions struct {
	inputPath  string
	outputPath string
}

func newInspectCommand() *cobra.Command {
	options := inspectOptions{}

	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Explain a completed Limier report without rerunning it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInspect(options)
		},
	}

	cmd.Flags().StringVar(&options.inputPath, "input", "", "Path to an existing report.json")
	cmd.Flags().StringVar(&options.outputPath, "output", "", "Optional path to write the inspection output")

	_ = cmd.MarkFlagRequired("input")

	return cmd
}

func runInspect(options inspectOptions) error {
	runReport, err := loadReport(options.inputPath)
	if err != nil {
		return err
	}

	if err := writeCommandOutput(options.outputPath, render.Inspect(runReport)); err != nil {
		return fmt.Errorf("emit inspect output: %w", err)
	}

	return nil
}
