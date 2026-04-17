package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/room215/limier/internal/limier"
	"github.com/room215/limier/internal/report"
	"github.com/spf13/cobra"
)

type runOptions struct {
	ecosystem        string
	packageName      string
	currentVersion   string
	candidateVersion string
	fixturePath      string
	scenarioPath     string
	rulesPath        string
	reportPath       string
	summaryPath      string
	evidencePath     string
}

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err == nil {
		return ""
	}

	return e.err.Error()
}

func (e *exitError) Unwrap() error {
	return e.err
}

func (e *exitError) ExitCode() int {
	return e.code
}

func newRunCommand() *cobra.Command {
	options := runOptions{
		reportPath:   "report.json",
		summaryPath:  "summary.md",
		evidencePath: "evidence",
	}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Compare one dependency upgrade in an isolated fixture",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLimier(cmd.Context(), options)
		},
	}

	cmd.Flags().StringVar(&options.ecosystem, "ecosystem", "", "Dependency ecosystem adapter to use")
	cmd.Flags().StringVar(&options.packageName, "package", "", "Dependency to compare")
	cmd.Flags().StringVar(&options.currentVersion, "current", "", "Baseline dependency version")
	cmd.Flags().StringVar(&options.candidateVersion, "candidate", "", "Candidate dependency version")
	cmd.Flags().StringVar(&options.fixturePath, "fixture", "", "Path to the sample application fixture")
	cmd.Flags().StringVar(&options.scenarioPath, "scenario", "", "Path to the scenario manifest")
	cmd.Flags().StringVar(&options.rulesPath, "rules", "", "Path to the rules file")
	cmd.Flags().StringVar(&options.reportPath, "report", options.reportPath, "Path to write report.json")
	cmd.Flags().StringVar(&options.summaryPath, "summary", options.summaryPath, "Path to write summary.md")
	cmd.Flags().StringVar(&options.evidencePath, "evidence", options.evidencePath, "Path to write evidence files")

	_ = cmd.MarkFlagRequired("ecosystem")
	_ = cmd.MarkFlagRequired("package")
	_ = cmd.MarkFlagRequired("current")
	_ = cmd.MarkFlagRequired("candidate")
	_ = cmd.MarkFlagRequired("fixture")
	_ = cmd.MarkFlagRequired("scenario")
	_ = cmd.MarkFlagRequired("rules")

	return cmd
}

func runLimier(ctx context.Context, options runOptions) error {
	result := limier.Run(ctx, limier.Options{
		LimierVersion:    version,
		Ecosystem:        strings.TrimSpace(options.ecosystem),
		PackageName:      strings.TrimSpace(options.packageName),
		CurrentVersion:   strings.TrimSpace(options.currentVersion),
		CandidateVersion: strings.TrimSpace(options.candidateVersion),
		FixturePath:      strings.TrimSpace(options.fixturePath),
		ScenarioPath:     strings.TrimSpace(options.scenarioPath),
		RulesPath:        strings.TrimSpace(options.rulesPath),
		EvidencePath:     strings.TrimSpace(options.evidencePath),
	})

	if err := report.WriteAll(options.reportPath, options.summaryPath, result.Report); err != nil {
		return &exitError{
			code: 2,
			err:  fmt.Errorf("write outputs: %w", err),
		}
	}

	slog.Info(
		"run completed",
		"ecosystem", result.Report.Input.Ecosystem,
		"package", result.Report.Input.Package,
		"technical_verdict", result.Report.TechnicalVerdict,
		"operator_recommendation", result.Report.OperatorRecommendation,
		"exit_code", result.Report.ExitCode,
		"report_path", options.reportPath,
		"summary_path", options.summaryPath,
		"evidence_path", result.Report.Evidence.RootPath,
	)

	if result.Report.ExitCode == 0 {
		return nil
	}

	if result.Report.ExitCode == 2 && result.Report.Diagnostic != nil && strings.TrimSpace(result.Report.Diagnostic.Summary) != "" {
		return &exitError{
			code: result.Report.ExitCode,
			err:  errors.New(result.Report.Diagnostic.Summary),
		}
	}

	return &exitError{code: result.Report.ExitCode}
}
