package limier

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/room215/limier/internal/adapter"
	"github.com/room215/limier/internal/adapters"
	"github.com/room215/limier/internal/analysis"
	"github.com/room215/limier/internal/collector"
	"github.com/room215/limier/internal/env/docker"
	"github.com/room215/limier/internal/fsutil"
	"github.com/room215/limier/internal/report"
	"github.com/room215/limier/internal/rules"
	"github.com/room215/limier/internal/scenario"
	"github.com/room215/limier/internal/verdict"
)

type Options struct {
	LimierVersion    string
	Ecosystem        string
	PackageName      string
	CurrentVersion   string
	CandidateVersion string
	FixturePath      string
	ScenarioPath     string
	RulesPath        string
	EvidencePath     string
}

type Result struct {
	Report report.Report
}

func Run(ctx context.Context, options Options) Result {
	runReport := report.Report{
		LimierVersion:          options.LimierVersion,
		GeneratedAt:            time.Now().UTC(),
		TechnicalVerdict:       verdict.TechnicalInconclusive,
		OperatorRecommendation: verdict.RecommendationRerun,
		ExitCode:               verdict.ExitCode(verdict.RecommendationRerun),
	}

	normalized, err := normalizeOptions(options)
	if err != nil {
		return resultWithDiagnostic(runReport, report.NewDiagnostic(
			report.DiagnosticCategoryInput,
			"input_invalid",
			err.Error(),
			"Fix the CLI inputs and rerun Limier.",
		))
	}

	runReport.Input = report.Input{
		Ecosystem:        normalized.Ecosystem,
		Package:          normalized.PackageName,
		CurrentVersion:   normalized.CurrentVersion,
		CandidateVersion: normalized.CandidateVersion,
		FixturePath:      normalized.FixturePath,
		ScenarioPath:     normalized.ScenarioPath,
		RulesPath:        normalized.RulesPath,
	}
	runReport.Evidence = report.Evidence{RootPath: normalized.EvidencePath}

	manifest, err := scenario.Load(normalized.ScenarioPath)
	if err != nil {
		return resultWithDiagnostic(runReport, classifyLoadDiagnostic("scenario", normalized.ScenarioPath, err))
	}

	ruleSet, err := rules.Load(normalized.RulesPath)
	if err != nil {
		return resultWithDiagnostic(runReport, classifyLoadDiagnostic("rules", normalized.RulesPath, err))
	}

	adapter, err := adapters.Lookup(normalized.Ecosystem)
	if err != nil {
		return resultWithDiagnostic(runReport, report.NewDiagnostic(
			report.DiagnosticCategoryInput,
			"unsupported_ecosystem",
			err.Error(),
			fmt.Sprintf("Choose one of the supported ecosystem adapters (%s) and rerun Limier.", strings.Join(adapters.Supported(), ", ")),
		))
	}

	selectedImage := manifest.Image
	if strings.TrimSpace(selectedImage) == "" {
		selectedImage = adapter.DefaultImage()
	}

	runReport.Scenario = report.ScenarioIdentity{
		Name:               manifest.Name,
		Path:               normalized.ScenarioPath,
		Repeats:            manifest.Repeats,
		Image:              selectedImage,
		Workdir:            manifest.Workdir,
		Steps:              manifest.StepNames(),
		CaptureHostSignals: manifest.Evidence.HostSignalsEnabled(),
	}
	runReport.Rules = report.RulesIdentity{
		Path:           normalized.RulesPath,
		HardBlockCount: len(ruleSet.HardBlock),
		ReviewCount:    len(ruleSet.Review),
		SuppressCount:  len(ruleSet.Suppress),
	}

	if err := os.MkdirAll(normalized.EvidencePath, 0o755); err != nil {
		return resultWithDiagnostic(runReport, report.NewDiagnostic(
			report.DiagnosticCategoryInternal,
			"evidence_directory_create_failed",
			fmt.Sprintf("create evidence directory %q: %v", normalized.EvidencePath, err),
			"Fix the evidence output path or permissions and rerun Limier.",
			normalized.EvidencePath,
		))
	}

	var collectorFactory collector.Factory
	if manifest.Evidence.HostSignalsEnabled() {
		collectorFactory = collector.NewFactory()
	}
	manager := docker.NewManager("docker")
	mounts := resolveMounts(filepath.Dir(normalized.ScenarioPath), manifest.Mounts)

	baseline, baselineFiles, baselineDiagnostic := runSide(ctx, sideRequest{
		Name:             "baseline",
		Version:          normalized.CurrentVersion,
		FixturePath:      normalized.FixturePath,
		PackageName:      normalized.PackageName,
		Scenario:         manifest,
		Image:            selectedImage,
		EvidenceRoot:     normalized.EvidencePath,
		Mounts:           mounts,
		Adapter:          adapter,
		CollectorFactory: collectorFactory,
		Manager:          manager,
	})
	runReport.Baseline = baseline
	runReport.Evidence.Files = append(runReport.Evidence.Files, baselineFiles...)
	if baselineDiagnostic != nil {
		sort.Strings(runReport.Evidence.Files)
		return resultWithDiagnostic(runReport, baselineDiagnostic)
	}

	candidate, candidateFiles, candidateDiagnostic := runSide(ctx, sideRequest{
		Name:             "candidate",
		Version:          normalized.CandidateVersion,
		FixturePath:      normalized.FixturePath,
		PackageName:      normalized.PackageName,
		Scenario:         manifest,
		Image:            selectedImage,
		EvidenceRoot:     normalized.EvidencePath,
		Mounts:           mounts,
		Adapter:          adapter,
		CollectorFactory: collectorFactory,
		Manager:          manager,
	})
	runReport.Candidate = candidate
	runReport.Evidence.Files = append(runReport.Evidence.Files, candidateFiles...)
	sort.Strings(runReport.Evidence.Files)
	if candidateDiagnostic != nil {
		return resultWithDiagnostic(runReport, candidateDiagnostic)
	}

	evaluation := analysis.Evaluate(baseline, candidate, manifest.Success.ExitCode, ruleSet)
	runReport.Findings = evaluation.Findings
	runReport.RuleHits = evaluation.RuleHits
	runReport.TechnicalVerdict = evaluation.TechnicalVerdict
	runReport.OperatorRecommendation = evaluation.OperatorRecommendation
	runReport.ExitCode = verdict.ExitCode(evaluation.OperatorRecommendation)
	runReport.Diagnostic = evaluation.Diagnostic

	return Result{Report: runReport}
}

func resultWithDiagnostic(runReport report.Report, diagnostic *report.Diagnostic) Result {
	runReport.Diagnostic = diagnostic
	return Result{Report: runReport}
}

type normalizedOptions struct {
	LimierVersion    string
	Ecosystem        string
	PackageName      string
	CurrentVersion   string
	CandidateVersion string
	FixturePath      string
	ScenarioPath     string
	RulesPath        string
	EvidencePath     string
}

func normalizeOptions(options Options) (normalizedOptions, error) {
	var problems []string

	required := map[string]string{
		"--ecosystem": options.Ecosystem,
		"--package":   options.PackageName,
		"--current":   options.CurrentVersion,
		"--candidate": options.CandidateVersion,
		"--fixture":   options.FixturePath,
		"--scenario":  options.ScenarioPath,
		"--rules":     options.RulesPath,
		"--evidence":  options.EvidencePath,
	}

	for flag, value := range required {
		if strings.TrimSpace(value) == "" {
			problems = append(problems, flag+" is required")
		}
	}

	if len(problems) > 0 {
		return normalizedOptions{}, fmt.Errorf("%s", strings.Join(problems, "; "))
	}

	fixturePath, err := filepath.Abs(options.FixturePath)
	if err != nil {
		return normalizedOptions{}, fmt.Errorf("resolve fixture path: %w", err)
	}

	fixtureInfo, err := os.Stat(fixturePath)
	if err != nil {
		return normalizedOptions{}, fmt.Errorf("stat fixture path %q: %w", fixturePath, err)
	}

	if !fixtureInfo.IsDir() {
		return normalizedOptions{}, fmt.Errorf("fixture path %q must be a directory", fixturePath)
	}

	scenarioPath, err := filepath.Abs(options.ScenarioPath)
	if err != nil {
		return normalizedOptions{}, fmt.Errorf("resolve scenario path: %w", err)
	}

	rulesPath, err := filepath.Abs(options.RulesPath)
	if err != nil {
		return normalizedOptions{}, fmt.Errorf("resolve rules path: %w", err)
	}

	evidencePath, err := filepath.Abs(options.EvidencePath)
	if err != nil {
		return normalizedOptions{}, fmt.Errorf("resolve evidence path: %w", err)
	}

	return normalizedOptions{
		LimierVersion:    options.LimierVersion,
		Ecosystem:        options.Ecosystem,
		PackageName:      options.PackageName,
		CurrentVersion:   options.CurrentVersion,
		CandidateVersion: options.CandidateVersion,
		FixturePath:      fixturePath,
		ScenarioPath:     scenarioPath,
		RulesPath:        rulesPath,
		EvidencePath:     evidencePath,
	}, nil
}

type sideRequest struct {
	Name             string
	Version          string
	FixturePath      string
	PackageName      string
	Scenario         scenario.Manifest
	Image            string
	EvidenceRoot     string
	Mounts           []docker.Mount
	Adapter          adapter.Adapter
	CollectorFactory collector.Factory
	Manager          runManager
}

type runManager interface {
	Run(context.Context, docker.RunRequest) (docker.RunResult, error)
}

func runSide(ctx context.Context, request sideRequest) (report.Side, []string, *report.Diagnostic) {
	sideReport := report.Side{
		RequestedVersion: request.Version,
	}

	var evidenceFiles []string
	var installedVersion string
	for runIndex := 1; runIndex <= request.Scenario.Repeats; runIndex++ {
		workspace, cleanup, err := cloneFixture(request.FixturePath)
		if err != nil {
			var symlinkErr *fsutil.SymlinkError
			if errors.As(err, &symlinkErr) {
				return sideReport, evidenceFiles, sideDiagnostic(
					request.Name,
					report.DiagnosticCategoryValidation,
					"fixture_contains_symlink",
					fmt.Sprintf("prepare %s workspace: %v", request.Name, err),
					"Remove symlinked fixture entries or disable the unsafe fixture layout, then rerun Limier.",
					symlinkErr.Path,
				)
			}
			return sideReport, evidenceFiles, sideDiagnostic(
				request.Name,
				report.DiagnosticCategoryInternal,
				"workspace_prepare_failed",
				fmt.Sprintf("prepare %s workspace: %v", request.Name, err),
				"Fix the fixture contents or local filesystem permissions and rerun Limier.",
			)
		}

		prepared, err := request.Adapter.PrepareWorkspace(ctx, workspace, adapter.DependencySpec{
			Package: request.PackageName,
			Version: request.Version,
		})
		if err != nil {
			cleanup()
			return sideReport, evidenceFiles, sideDiagnostic(
				request.Name,
				report.DiagnosticCategoryExecution,
				"dependency_materialization_failed",
				fmt.Sprintf("prepare %s dependency materialization: %v", request.Name, err),
				"Inspect the fixture and dependency coordinates, then rerun Limier.",
			)
		}

		if len(sideReport.AdapterMetadata) == 0 {
			sideReport.AdapterMetadata = prepared.ReportMetadata()
		}

		steps, err := resolveSteps(request.Scenario, request.Adapter, prepared)
		if err != nil {
			cleanup()
			return sideReport, evidenceFiles, sideDiagnostic(
				request.Name,
				report.DiagnosticCategoryValidation,
				"step_resolution_failed",
				fmt.Sprintf("resolve %s steps: %v", request.Name, err),
				"Fix the scenario steps or adapter-specific step configuration and rerun Limier.",
			)
		}

		runEvidenceDir := filepath.Join(request.EvidenceRoot, request.Name, fmt.Sprintf("run-%d", runIndex))
		var runCollector collector.RunCollector
		if request.CollectorFactory != nil {
			runCollector, err = request.CollectorFactory.Start(collector.RunContext{
				Side:     request.Name,
				RunIndex: runIndex,
			})
			if err != nil {
				cleanup()
				return sideReport, evidenceFiles, sideDiagnostic(
					request.Name,
					report.DiagnosticCategoryExecution,
					"host_signal_capture_failed",
					fmt.Sprintf("start %s host signal capture: %v", request.Name, err),
					"Run Limier on Linux with host signal capture support available, or disable capture_host_signals and rerun Limier.",
				)
			}
		}

		runResult, err := request.Manager.Run(ctx, docker.RunRequest{
			Side:        request.Name,
			RunIndex:    runIndex,
			Image:       request.Image,
			Workdir:     request.Scenario.Workdir,
			Workspace:   workspace,
			Env:         sideEnv(request, runIndex),
			Mounts:      request.Mounts,
			NetworkMode: request.Scenario.Network.Mode,
			Steps:       steps,
			EvidenceDir: runEvidenceDir,
			Collector:   runCollector,
		})
		if err != nil {
			cleanup()
			return sideReport, evidenceFiles, classifyRuntimeDiagnostic(request.Name, runEvidenceDir, err)
		}

		sideReport.Runs = append(sideReport.Runs, convertRun(runResult))
		evidenceFiles = append(evidenceFiles, runResult.EvidenceFiles...)

		if runResult.ExitCode != request.Scenario.Success.ExitCode {
			cleanup()
			continue
		}

		readVersion, err := request.Adapter.ReadInstalledVersion(workspace, adapter.DependencySpec{
			Package: request.PackageName,
			Version: request.Version,
		}, prepared)
		if err != nil {
			cleanup()
			return sideReport, evidenceFiles, sideDiagnostic(
				request.Name,
				report.DiagnosticCategoryExecution,
				"installed_version_read_failed",
				fmt.Sprintf("read %s installed version: %v", request.Name, err),
				"Inspect the fixture output and adapter metadata, then rerun Limier.",
				runEvidenceDir,
			)
		}

		versionPath := filepath.Join(runEvidenceDir, "installed-version.txt")
		if err := os.WriteFile(versionPath, []byte(readVersion+"\n"), 0o644); err != nil {
			cleanup()
			return sideReport, evidenceFiles, sideDiagnostic(
				request.Name,
				report.DiagnosticCategoryInternal,
				"installed_version_evidence_write_failed",
				fmt.Sprintf("write installed version evidence %q: %v", versionPath, err),
				"Fix the evidence output path or permissions and rerun Limier.",
				versionPath,
			)
		}
		evidenceFiles = append(evidenceFiles, versionPath)

		if installedVersion == "" {
			installedVersion = readVersion
		} else if installedVersion != readVersion {
			cleanup()
			return sideReport, evidenceFiles, sideDiagnostic(
				request.Name,
				report.DiagnosticCategoryStability,
				"installed_version_changed_between_runs",
				fmt.Sprintf("%s installed version changed between runs: %q vs %q", request.Name, installedVersion, readVersion),
				"Rerun after making dependency installation deterministic enough to produce the same installed version each time.",
				versionPath,
			)
		}

		cleanup()
	}

	sideReport.InstalledVersion = installedVersion
	sideReport.Summary, sideReport.Stable = analysis.AssessSide(sideReport.Runs)

	return sideReport, evidenceFiles, nil
}

func sideDiagnostic(side string, category report.DiagnosticCategory, codeSuffix string, summary string, suggestedAction string, evidence ...string) *report.Diagnostic {
	return report.NewDiagnostic(category, side+"_"+codeSuffix, summary, suggestedAction, evidence...)
}

func classifyLoadDiagnostic(subject string, path string, err error) *report.Diagnostic {
	lower := strings.ToLower(err.Error())
	codePrefix := subject

	switch {
	case strings.Contains(lower, "invalid "+subject), strings.Contains(lower, "parse "+subject):
		return report.NewDiagnostic(
			report.DiagnosticCategoryValidation,
			codePrefix+"_invalid",
			err.Error(),
			fmt.Sprintf("Fix the %s file contents and rerun Limier.", subject),
			path,
		)
	default:
		return report.NewDiagnostic(
			report.DiagnosticCategoryInput,
			codePrefix+"_load_failed",
			err.Error(),
			fmt.Sprintf("Fix the %s path or permissions and rerun Limier.", subject),
			path,
		)
	}
}

func classifyRuntimeDiagnostic(side string, runEvidenceDir string, err error) *report.Diagnostic {
	var captureErr *collector.CaptureError
	if errors.As(err, &captureErr) {
		return report.NewDiagnostic(
			report.DiagnosticCategoryExecution,
			side+"_host_signal_capture_failed",
			fmt.Sprintf("run %s scenario: %v", side, err),
			"Run Limier on Linux with host signal capture support available, or disable capture_host_signals and rerun Limier.",
			runEvidenceDir,
		)
	}

	summary := fmt.Sprintf("run %s scenario: %v", side, err)
	lower := strings.ToLower(err.Error())

	if strings.Contains(lower, "docker ") || strings.Contains(lower, "container") {
		return report.NewDiagnostic(
			report.DiagnosticCategoryDocker,
			side+"_docker_run_failed",
			summary,
			"Confirm Docker is available and healthy on the runner, then rerun Limier.",
			runEvidenceDir,
		)
	}

	return report.NewDiagnostic(
		report.DiagnosticCategoryExecution,
		side+"_scenario_execution_failed",
		summary,
		"Inspect the fixture, scenario commands, and evidence for the failing run, then rerun Limier.",
		runEvidenceDir,
	)
}

func cloneFixture(fixturePath string) (string, func(), error) {
	workspace, err := os.MkdirTemp("", "limier-workspace-")
	if err != nil {
		return "", nil, fmt.Errorf("create temp workspace: %w", err)
	}

	if err := fsutil.CopyTree(fixturePath, workspace); err != nil {
		_ = os.RemoveAll(workspace)
		return "", nil, fmt.Errorf("copy fixture into temp workspace: %w", err)
	}

	return workspace, func() {
		_ = os.RemoveAll(workspace)
	}, nil
}

func resolveSteps(manifest scenario.Manifest, adapter adapter.Adapter, prepared adapter.PreparedWorkspace) ([]docker.Step, error) {
	steps := make([]docker.Step, 0, len(manifest.Steps))
	for _, step := range manifest.Steps {
		resolved, err := adapter.ResolveStep(step, prepared)
		if err != nil {
			return nil, err
		}

		steps = append(steps, docker.Step{
			Name:    resolved.Name,
			Intent:  resolved.Intent,
			Command: resolved.Command,
		})
	}

	return steps, nil
}

func convertRun(run docker.RunResult) report.Run {
	converted := report.Run{
		RunIndex:             run.RunIndex,
		ExitCode:             run.ExitCode,
		DurationMilliseconds: run.DurationMilliseconds,
		Events:               run.Events,
		EventsPath:           run.EventsPath,
	}

	for _, step := range run.Steps {
		converted.Steps = append(converted.Steps, report.Step{
			Name:                 step.Name,
			Intent:               step.Intent,
			Command:              step.Command,
			ExitCode:             step.ExitCode,
			DurationMilliseconds: step.DurationMilliseconds,
			Stdout: report.Output{
				Preview:    step.Stdout.Preview,
				TotalBytes: step.Stdout.TotalBytes,
				SHA256:     step.Stdout.SHA256,
				Truncated:  step.Stdout.Truncated,
			},
			Stderr: report.Output{
				Preview:    step.Stderr.Preview,
				TotalBytes: step.Stderr.TotalBytes,
				SHA256:     step.Stderr.SHA256,
				Truncated:  step.Stderr.Truncated,
			},
			Evidence: report.StepEvidence{
				Stdout: report.OutputFile{
					Path:        step.Evidence.Stdout.Path,
					StoredBytes: step.Evidence.Stdout.StoredBytes,
					Truncated:   step.Evidence.Stdout.Truncated,
				},
				Stderr: report.OutputFile{
					Path:        step.Evidence.Stderr.Path,
					StoredBytes: step.Evidence.Stderr.StoredBytes,
					Truncated:   step.Evidence.Stderr.Truncated,
				},
			},
		})
	}

	return converted
}

func sideEnv(request sideRequest, runIndex int) map[string]string {
	env := map[string]string{}
	for key, value := range request.Scenario.Env {
		env[key] = value
	}

	env["LIMIER_SIDE"] = request.Name
	env["LIMIER_RUN_INDEX"] = fmt.Sprintf("%d", runIndex)
	env["LIMIER_PACKAGE"] = request.PackageName
	env["LIMIER_VERSION_UNDER_TEST"] = request.Version

	return env
}

func resolveMounts(baseDir string, mounts []scenario.Mount) []docker.Mount {
	resolved := make([]docker.Mount, 0, len(mounts))
	for _, mount := range mounts {
		source := mount.Source
		if !filepath.IsAbs(source) {
			source = filepath.Join(baseDir, source)
		}

		resolved = append(resolved, docker.Mount{
			Source:   source,
			Target:   mount.Target,
			ReadOnly: mount.ReadOnly,
		})
	}

	return resolved
}
