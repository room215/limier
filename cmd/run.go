package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/room215/limier/internal/limier"
	"github.com/room215/limier/internal/preset"
	"github.com/room215/limier/internal/report"
	"github.com/room215/limier/internal/verdict"
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
	failOn           string
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
	cmd.Flags().StringVar(&options.failOn, "fail-on", "", "Comma-separated recommendations that should fail this command; empty preserves Limier defaults")

	_ = cmd.MarkFlagRequired("ecosystem")
	_ = cmd.MarkFlagRequired("package")
	_ = cmd.MarkFlagRequired("current")
	_ = cmd.MarkFlagRequired("candidate")

	return cmd
}

func runLimier(ctx context.Context, options runOptions) error {
	runReport, err := executeRun(ctx, options)
	if err != nil {
		return &exitError{
			code: 2,
			err:  err,
		}
	}

	exitCode, err := policyExitCode(runReport.OperatorRecommendation, runReport.ExitCode, options.failOn)
	if err != nil {
		return &exitError{
			code: 2,
			err:  err,
		}
	}

	if exitCode == 0 {
		return nil
	}

	if exitCode == 2 && runReport.Diagnostic != nil && strings.TrimSpace(runReport.Diagnostic.Summary) != "" {
		return &exitError{
			code: exitCode,
			err:  errors.New(runReport.Diagnostic.Summary),
		}
	}

	return &exitError{code: exitCode}
}

func executeRun(ctx context.Context, options runOptions) (report.Report, error) {
	resolved, cleanup, err := resolveRunPresets(options)
	if err != nil {
		return report.Report{}, err
	}
	defer cleanup()

	result := limier.Run(ctx, limier.Options{
		LimierVersion:    version,
		Ecosystem:        strings.TrimSpace(resolved.ecosystem),
		PackageName:      strings.TrimSpace(resolved.packageName),
		CurrentVersion:   strings.TrimSpace(resolved.currentVersion),
		CandidateVersion: strings.TrimSpace(resolved.candidateVersion),
		FixturePath:      strings.TrimSpace(resolved.fixturePath),
		ScenarioPath:     strings.TrimSpace(resolved.scenarioPath),
		RulesPath:        strings.TrimSpace(resolved.rulesPath),
		EvidencePath:     strings.TrimSpace(resolved.evidencePath),
	})

	if err := report.WriteAll(options.reportPath, options.summaryPath, result.Report); err != nil {
		return report.Report{}, fmt.Errorf("write outputs: %w", err)
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

	return result.Report, nil
}

func resolveRunPresets(options runOptions) (runOptions, func(), error) {
	var cleanups []preset.Cleanup
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			if cleanups[i] != nil {
				cleanups[i]()
			}
		}
	}

	resolved := options
	var err error
	var cleanupOne preset.Cleanup

	resolved.rulesPath, cleanupOne, err = preset.ResolveRules(options.rulesPath)
	if err != nil {
		cleanup()
		return runOptions{}, nil, err
	}
	cleanups = append(cleanups, cleanupOne)

	resolved.scenarioPath, cleanupOne, err = preset.ResolveScenario(options.scenarioPath, options.ecosystem)
	if err != nil {
		cleanup()
		return runOptions{}, nil, err
	}
	cleanups = append(cleanups, cleanupOne)

	resolved.fixturePath, cleanupOne, err = preset.ResolveFixture(options.fixturePath, options.ecosystem)
	if err != nil {
		cleanup()
		return runOptions{}, nil, err
	}
	cleanups = append(cleanups, cleanupOne)

	return resolved, cleanup, nil
}

func policyExitCode(recommendation verdict.Recommendation, defaultExitCode int, failOn string) (int, error) {
	trimmed := strings.TrimSpace(failOn)
	if trimmed == "" {
		return defaultExitCode, nil
	}

	shouldFail, err := recommendationListed(recommendation, trimmed)
	if err != nil {
		return 2, err
	}
	if !shouldFail {
		return 0, nil
	}

	if recommendation == verdict.RecommendationRerun {
		return 2, nil
	}

	return 1, nil
}

func recommendationListed(recommendation verdict.Recommendation, value string) (bool, error) {
	valid := map[string]verdict.Recommendation{
		string(verdict.RecommendationGoodToGo):    verdict.RecommendationGoodToGo,
		string(verdict.RecommendationNeedsReview): verdict.RecommendationNeedsReview,
		string(verdict.RecommendationBlock):       verdict.RecommendationBlock,
		string(verdict.RecommendationRerun):       verdict.RecommendationRerun,
	}

	for _, part := range strings.Split(value, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		if _, ok := valid[name]; !ok {
			return false, fmt.Errorf("unsupported --fail-on recommendation %q", name)
		}
		if verdict.Recommendation(name) == recommendation {
			return true, nil
		}
	}

	return false, nil
}
