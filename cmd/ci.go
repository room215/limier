package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/room215/limier/internal/adapters"
	"github.com/room215/limier/internal/render"
	"github.com/room215/limier/internal/report"
	"github.com/room215/limier/internal/verdict"
	"github.com/spf13/cobra"
)

type githubCIOptions struct {
	outputDir              string
	failOn                 string
	ecosystem              string
	packageName            string
	currentVersion         string
	candidateVersion       string
	fixturePath            string
	scenarioPath           string
	rulesPath              string
	telemetryMode          string
	metadataOutcome        string
	dependencyFilesChanged string
	prAuthor               string
	getenv                 func(string) string
}

type dependencyUpgrade struct {
	Ecosystem        string
	Package          string
	CurrentVersion   string
	CandidateVersion string
}

type ciStatus struct {
	Status                 string `json:"status"`
	OperatorRecommendation string `json:"operator_recommendation"`
	Message                string `json:"message,omitempty"`
	PRNumber               int    `json:"pr_number,omitempty"`
	Ecosystem              string `json:"ecosystem,omitempty"`
	Package                string `json:"package,omitempty"`
	CurrentVersion         string `json:"current_version,omitempty"`
	CandidateVersion       string `json:"candidate_version,omitempty"`
	ReportPath             string `json:"report_path,omitempty"`
	SummaryPath            string `json:"summary_path,omitempty"`
	BuildSummaryPath       string `json:"build_summary_path,omitempty"`
	CommentPath            string `json:"comment_path,omitempty"`
	StatusPath             string `json:"status_path,omitempty"`
	PolicyExitCode         int    `json:"policy_exit_code"`
}

func newCICommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ci",
		Short: "Run CI-oriented Limier integrations",
	}

	cmd.AddCommand(newGitHubCICommand())

	return cmd
}

func newGitHubCICommand() *cobra.Command {
	options := githubCIOptions{
		outputDir: "out/limier",
		failOn:    "block,rerun",
	}

	cmd := &cobra.Command{
		Use:   "github",
		Short: "Run a GitHub Actions dependency review integration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGitHubCI(cmd.Context(), options)
		},
	}

	cmd.Flags().StringVar(&options.outputDir, "output-dir", options.outputDir, "Directory for CI outputs")
	cmd.Flags().StringVar(&options.failOn, "fail-on", options.failOn, "Comma-separated recommendations that should fail this command")
	cmd.Flags().StringVar(&options.ecosystem, "ecosystem", "", "Dependency ecosystem override")
	cmd.Flags().StringVar(&options.packageName, "package", "", "Dependency package override")
	cmd.Flags().StringVar(&options.currentVersion, "current", "", "Baseline dependency version override")
	cmd.Flags().StringVar(&options.candidateVersion, "candidate", "", "Candidate dependency version override")
	cmd.Flags().StringVar(&options.fixturePath, "fixture", "", "Path or preset for the sample application fixture")
	cmd.Flags().StringVar(&options.scenarioPath, "scenario", "", "Path or preset for the scenario manifest")
	cmd.Flags().StringVar(&options.rulesPath, "rules", "", "Path or preset for the rules file")
	cmd.Flags().StringVar(&options.telemetryMode, "telemetry-mode", "", "Telemetry mode override: required or off")
	cmd.Flags().StringVar(&options.dependencyFilesChanged, "dependency-files-changed", "", "Whether dependency-relevant files changed: true, false, or unknown")

	return cmd
}

func runGitHubCI(ctx context.Context, options githubCIOptions) error {
	paths := ciOutputPaths(options.outputDir)
	if err := os.MkdirAll(options.outputDir, 0o755); err != nil {
		return &exitError{
			code: 2,
			err:  fmt.Errorf("create output directory %q: %w", options.outputDir, err),
		}
	}

	prNumber := githubPullRequestNumber(options.getEnv)
	prAuthor := firstNonEmpty(options.prAuthor, githubPullRequestAuthor(options.getEnv))
	upgrade, status, ok, err := resolveDependencyUpgrade(options, prNumber, prAuthor)
	if err != nil {
		status.PolicyExitCode = 2
		finishErr := finishCIStatus(status, paths)
		if finishErr != nil {
			err = errors.Join(err, finishErr)
		}
		return &exitError{
			code: 2,
			err:  err,
		}
	}
	if !ok {
		return finishSkippedCIStatus(status, paths, options.failOn)
	}

	runOptions := runOptions{
		ecosystem:        upgrade.Ecosystem,
		packageName:      upgrade.Package,
		currentVersion:   upgrade.CurrentVersion,
		candidateVersion: upgrade.CandidateVersion,
		fixturePath:      options.fixturePath,
		scenarioPath:     options.scenarioPath,
		rulesPath:        options.rulesPath,
		telemetryMode:    options.telemetryMode,
		reportPath:       paths.reportPath,
		summaryPath:      paths.summaryPath,
		evidencePath:     paths.evidencePath,
		failOn:           options.failOn,
	}

	runReport, err := executeRun(ctx, runOptions)
	if err != nil {
		finishErr := finishCIStatus(ciErrorStatus(err, paths, prNumber, upgrade), paths)
		if finishErr != nil {
			err = errors.Join(err, finishErr)
		}
		return &exitError{
			code: 2,
			err:  err,
		}
	}

	if err := writeRenderedCIOutputs(runReport, paths); err != nil {
		finishErr := finishCIStatus(ciErrorStatus(err, paths, prNumber, upgrade), paths)
		if finishErr != nil {
			err = errors.Join(err, finishErr)
		}
		return &exitError{
			code: 2,
			err:  err,
		}
	}

	exitCode, err := policyExitCode(runReport.OperatorRecommendation, runReport.ExitCode, options.failOn)
	if err != nil {
		status = ciStatus{
			Status:                 "ran",
			OperatorRecommendation: string(runReport.OperatorRecommendation),
			Message:                err.Error(),
			PRNumber:               prNumber,
			Ecosystem:              upgrade.Ecosystem,
			Package:                upgrade.Package,
			CurrentVersion:         upgrade.CurrentVersion,
			CandidateVersion:       upgrade.CandidateVersion,
			ReportPath:             paths.reportPath,
			SummaryPath:            paths.summaryPath,
			BuildSummaryPath:       paths.buildSummaryPath,
			CommentPath:            paths.commentPath,
			StatusPath:             paths.statusPath,
			PolicyExitCode:         2,
		}
		finishErr := finishCIStatus(status, paths)
		if finishErr != nil {
			err = errors.Join(err, finishErr)
		}
		return &exitError{
			code: 2,
			err:  err,
		}
	}

	status = ciStatus{
		Status:                 "ran",
		OperatorRecommendation: string(runReport.OperatorRecommendation),
		PRNumber:               prNumber,
		Ecosystem:              upgrade.Ecosystem,
		Package:                upgrade.Package,
		CurrentVersion:         upgrade.CurrentVersion,
		CandidateVersion:       upgrade.CandidateVersion,
		ReportPath:             paths.reportPath,
		SummaryPath:            paths.summaryPath,
		BuildSummaryPath:       paths.buildSummaryPath,
		CommentPath:            paths.commentPath,
		StatusPath:             paths.statusPath,
		PolicyExitCode:         exitCode,
	}
	if err := finishCIStatus(status, paths); err != nil {
		return err
	}

	if exitCode == 0 {
		return nil
	}
	return &exitError{code: exitCode}
}

func (options githubCIOptions) getEnv(name string) string {
	if options.getenv != nil {
		return options.getenv(name)
	}
	return os.Getenv(name)
}

func ciErrorStatus(err error, paths ciPaths, prNumber int, upgrade dependencyUpgrade) ciStatus {
	return ciStatus{
		Status:                 "rerun",
		OperatorRecommendation: string(verdict.RecommendationRerun),
		Message:                fmt.Sprintf("Limier CI did not complete: %v", err),
		PRNumber:               prNumber,
		Ecosystem:              upgrade.Ecosystem,
		Package:                upgrade.Package,
		CurrentVersion:         upgrade.CurrentVersion,
		CandidateVersion:       upgrade.CandidateVersion,
		ReportPath:             paths.reportPath,
		SummaryPath:            paths.summaryPath,
		BuildSummaryPath:       paths.buildSummaryPath,
		CommentPath:            paths.commentPath,
		StatusPath:             paths.statusPath,
		PolicyExitCode:         2,
	}
}

func finishSkippedCIStatus(status ciStatus, paths ciPaths, failOn string) error {
	exitCode, err := policyExitCode(verdict.Recommendation(status.OperatorRecommendation), 0, failOn)
	if err != nil {
		return &exitError{code: 2, err: err}
	}
	status.PolicyExitCode = exitCode

	if err := finishCIStatus(status, paths); err != nil {
		return err
	}
	if exitCode == 0 {
		return nil
	}
	return &exitError{code: exitCode}
}

type ciPaths struct {
	outputDir        string
	reportPath       string
	summaryPath      string
	buildSummaryPath string
	commentPath      string
	statusPath       string
	evidencePath     string
	prPath           string
}

func ciOutputPaths(outputDir string) ciPaths {
	return ciPaths{
		outputDir:        outputDir,
		reportPath:       filepath.Join(outputDir, "report.json"),
		summaryPath:      filepath.Join(outputDir, "summary.md"),
		buildSummaryPath: filepath.Join(outputDir, "build-summary.md"),
		commentPath:      filepath.Join(outputDir, "comment.md"),
		statusPath:       filepath.Join(outputDir, "status.json"),
		evidencePath:     filepath.Join(outputDir, "evidence"),
		prPath:           filepath.Join(outputDir, "pr.txt"),
	}
}

func resolveDependencyUpgrade(options githubCIOptions, prNumber int, prAuthor string) (dependencyUpgrade, ciStatus, bool, error) {
	rawEcosystem := firstNonEmpty(
		options.ecosystem,
		options.getEnv("LIMIER_CI_ECOSYSTEM"),
		options.getEnv("DEPENDABOT_PACKAGE_ECOSYSTEM"),
	)
	rawPackage := firstNonEmpty(
		options.packageName,
		options.getEnv("LIMIER_CI_PACKAGE"),
		options.getEnv("LIMIER_CI_DEPENDENCY_NAMES"),
		options.getEnv("DEPENDABOT_DEPENDENCY_NAMES"),
	)
	current := firstNonEmpty(
		options.currentVersion,
		options.getEnv("LIMIER_CI_CURRENT"),
		options.getEnv("LIMIER_CI_PREVIOUS_VERSION"),
		options.getEnv("DEPENDABOT_PREVIOUS_VERSION"),
	)
	candidate := firstNonEmpty(
		options.candidateVersion,
		options.getEnv("LIMIER_CI_CANDIDATE"),
		options.getEnv("LIMIER_CI_NEW_VERSION"),
		options.getEnv("DEPENDABOT_NEW_VERSION"),
	)

	dependencyFilesChanged, rawDependencyFilesChanged, signalOK := resolveDependencyFilesChangeSignal(options)
	if !signalOK {
		message := fmt.Sprintf("Unsupported dependency file change signal %q; use true, false, or unknown.", rawDependencyFilesChanged)
		return dependencyUpgrade{}, skippedStatus("rerun", verdict.RecommendationRerun, message, prNumber), false, errors.New(message)
	}

	if metadataLookupFailed(options, prAuthor) && (rawEcosystem == "" || rawPackage == "" || current == "" || candidate == "") {
		return dependencyUpgrade{}, skippedStatus("rerun", verdict.RecommendationRerun, "Dependabot metadata lookup failed, so Limier could not safely determine the dependency update.", prNumber), false, nil
	}

	if strings.TrimSpace(rawEcosystem) == "" && strings.TrimSpace(rawPackage) == "" && strings.TrimSpace(current) == "" && strings.TrimSpace(candidate) == "" {
		switch dependencyFilesChanged {
		case dependencyFilesChangeNo:
			return dependencyUpgrade{}, skippedStatus("not_applicable", verdict.RecommendationGoodToGo, "No dependency-relevant files changed, so Limier did not run.", prNumber), false, nil
		case dependencyFilesChangeYes:
			return dependencyUpgrade{}, skippedStatus("needs_review", verdict.RecommendationNeedsReview, "Dependency-relevant files changed, but no dependency metadata was available; review this update manually.", prNumber), false, nil
		default:
			return dependencyUpgrade{}, skippedStatus("needs_review", verdict.RecommendationNeedsReview, "No dependency metadata was available and dependency-file changes were not classified; review this update manually.", prNumber), false, nil
		}
	}

	ecosystem := normalizeDependabotEcosystem(rawEcosystem)
	if ecosystem == "" || rawPackage == "" || current == "" || candidate == "" {
		return dependencyUpgrade{}, skippedStatus("needs_review", verdict.RecommendationNeedsReview, "Dependency metadata was incomplete; review this update manually.", prNumber), false, nil
	}

	packages := dependencyNames(rawPackage)
	if len(packages) != 1 {
		return dependencyUpgrade{}, skippedStatus("needs_review", verdict.RecommendationNeedsReview, fmt.Sprintf("Limier needs exactly one dependency; got %d (%s).", len(packages), strings.Join(packages, ", ")), prNumber), false, nil
	}

	if _, err := adapters.Lookup(ecosystem); err != nil {
		return dependencyUpgrade{}, skippedStatus("needs_review", verdict.RecommendationNeedsReview, fmt.Sprintf("Ecosystem %q is not supported by this Limier integration.", rawEcosystem), prNumber), false, nil
	}

	return dependencyUpgrade{
		Ecosystem:        ecosystem,
		Package:          packages[0],
		CurrentVersion:   strings.TrimSpace(current),
		CandidateVersion: strings.TrimSpace(candidate),
	}, ciStatus{}, true, nil
}

type dependencyFilesChangeSignal int

const (
	dependencyFilesChangeUnknown dependencyFilesChangeSignal = iota
	dependencyFilesChangeNo
	dependencyFilesChangeYes
)

func resolveDependencyFilesChangeSignal(options githubCIOptions) (dependencyFilesChangeSignal, string, bool) {
	raw := firstNonEmpty(
		options.dependencyFilesChanged,
		options.getEnv("LIMIER_CI_DEPENDENCY_FILES_CHANGED"),
	)
	if raw == "" {
		return dependencyFilesChangeUnknown, "", true
	}

	switch strings.ToLower(raw) {
	case "1", "true", "yes":
		return dependencyFilesChangeYes, raw, true
	case "0", "false", "no":
		return dependencyFilesChangeNo, raw, true
	case "unknown":
		return dependencyFilesChangeUnknown, raw, true
	default:
		return dependencyFilesChangeUnknown, raw, false
	}
}

func metadataLookupFailed(options githubCIOptions, prAuthor string) bool {
	outcome := strings.ToLower(firstNonEmpty(options.metadataOutcome, options.getEnv("DEPENDABOT_METADATA_OUTCOME")))
	if outcome == "" || outcome == "success" {
		return false
	}

	return isDependabotLogin(prAuthor)
}

func isDependabotLogin(login string) bool {
	return strings.EqualFold(strings.TrimSpace(login), "dependabot[bot]")
}

func skippedStatus(status string, recommendation verdict.Recommendation, message string, prNumber int) ciStatus {
	return ciStatus{
		Status:                 status,
		OperatorRecommendation: string(recommendation),
		Message:                message,
		PRNumber:               prNumber,
	}
}

func normalizeDependabotEcosystem(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "npm", "npm_and_yarn":
		return "npm"
	case "pip", "pip-compile", "pipenv", "poetry":
		return "pip"
	case "cargo":
		return "cargo"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func dependencyNames(value string) []string {
	var names []string
	for _, part := range strings.Split(value, ",") {
		name := strings.TrimSpace(part)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func writeRenderedCIOutputs(runReport report.Report, paths ciPaths) error {
	buildSummary, err := render.Render(runReport, render.FormatBuildSummary)
	if err != nil {
		return fmt.Errorf("render build summary: %w", err)
	}
	if err := writeCommandOutput(paths.buildSummaryPath, buildSummary); err != nil {
		return err
	}

	comment, err := render.Render(runReport, render.FormatGitHubComment)
	if err != nil {
		return fmt.Errorf("render GitHub comment: %w", err)
	}
	if err := writeCommandOutput(paths.commentPath, comment); err != nil {
		return err
	}

	return nil
}

func finishCIStatus(status ciStatus, paths ciPaths) error {
	status.StatusPath = paths.statusPath
	if status.BuildSummaryPath == "" {
		status.BuildSummaryPath = paths.buildSummaryPath
	}
	if status.CommentPath == "" {
		status.CommentPath = paths.commentPath
	}
	if status.SummaryPath == "" {
		status.SummaryPath = paths.summaryPath
	}

	if status.PRNumber != 0 {
		if err := os.WriteFile(paths.prPath, []byte(strconv.Itoa(status.PRNumber)+"\n"), 0o644); err != nil {
			return &exitError{code: 2, err: fmt.Errorf("write PR number: %w", err)}
		}
	}

	if _, err := os.Stat(paths.buildSummaryPath); err != nil {
		if err := writeCommandOutput(paths.buildSummaryPath, skippedMarkdown(status)); err != nil {
			return &exitError{code: 2, err: err}
		}
	}
	if _, err := os.Stat(paths.summaryPath); err != nil {
		if err := writeCommandOutput(paths.summaryPath, skippedMarkdown(status)); err != nil {
			return &exitError{code: 2, err: err}
		}
	}
	if _, err := os.Stat(paths.commentPath); err != nil {
		if err := writeCommandOutput(paths.commentPath, skippedMarkdown(status)); err != nil {
			return &exitError{code: 2, err: err}
		}
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return &exitError{code: 2, err: fmt.Errorf("encode CI status: %w", err)}
	}
	data = append(data, '\n')
	if err := os.WriteFile(paths.statusPath, data, 0o644); err != nil {
		return &exitError{code: 2, err: fmt.Errorf("write CI status: %w", err)}
	}

	if err := writeGitHubStepOutputs(status); err != nil {
		return &exitError{code: 2, err: err}
	}

	return nil
}

func skippedMarkdown(status ciStatus) string {
	var lines []string
	lines = append(lines, "# Limier Build Summary", "")
	lines = append(lines, fmt.Sprintf("- Status: `%s`", status.Status))
	lines = append(lines, fmt.Sprintf("- Operator recommendation: `%s`", status.OperatorRecommendation))
	if strings.TrimSpace(status.Message) != "" {
		lines = append(lines, fmt.Sprintf("- Why: %s", status.Message))
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func writeGitHubStepOutputs(status ciStatus) error {
	outputPath := strings.TrimSpace(os.Getenv("GITHUB_OUTPUT"))
	if outputPath == "" {
		return nil
	}

	var lines []string
	lines = append(lines, "status="+status.Status)
	lines = append(lines, "operator-recommendation="+status.OperatorRecommendation)
	lines = append(lines, "report-path="+status.ReportPath)
	lines = append(lines, "summary-path="+status.SummaryPath)
	lines = append(lines, "build-summary-path="+status.BuildSummaryPath)
	lines = append(lines, "comment-path="+status.CommentPath)
	lines = append(lines, "status-path="+status.StatusPath)
	lines = append(lines, "policy-exit-code="+strconv.Itoa(status.PolicyExitCode))

	file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open GITHUB_OUTPUT %q: %w", outputPath, err)
	}
	defer file.Close()

	_, err = file.WriteString(strings.Join(lines, "\n") + "\n")
	return err
}

func githubPullRequestNumber(getenv func(string) string) int {
	event := readGitHubPullRequestEvent(getenv)
	if event.PullRequest.Number != 0 {
		return event.PullRequest.Number
	}
	return event.Number
}

func githubPullRequestAuthor(getenv func(string) string) string {
	event := readGitHubPullRequestEvent(getenv)
	return event.PullRequest.User.Login
}

type githubPullRequestEvent struct {
	Number      int `json:"number"`
	PullRequest struct {
		Number int `json:"number"`
		User   struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"pull_request"`
}

func readGitHubPullRequestEvent(getenv func(string) string) githubPullRequestEvent {
	if getenv == nil {
		getenv = os.Getenv
	}

	eventPath := strings.TrimSpace(getenv("GITHUB_EVENT_PATH"))
	if eventPath == "" {
		return githubPullRequestEvent{}
	}

	data, err := os.ReadFile(eventPath)
	if err != nil {
		return githubPullRequestEvent{}
	}

	var event githubPullRequestEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return githubPullRequestEvent{}
	}

	return event
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
