package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oneslash/limier/internal/report"
	"github.com/oneslash/limier/internal/verdict"
)

func TestRunRenderWritesRenderedOutput(t *testing.T) {
	t.Parallel()

	inputPath := writeSampleReport(t, report.Report{
		Input: report.Input{
			Ecosystem:        "npm",
			Package:          "left-pad",
			CurrentVersion:   "1.0.0",
			CandidateVersion: "1.1.0",
		},
		TechnicalVerdict:       verdict.TechnicalUnexpectedDiff,
		OperatorRecommendation: verdict.RecommendationNeedsReview,
		ExitCode:               1,
		Evidence: report.Evidence{
			RootPath: "/tmp/limier/evidence",
		},
	})
	outputPath := filepath.Join(t.TempDir(), "rendered.md")

	if err := runRender(renderOptions{
		format:     "build-summary",
		inputPath:  inputPath,
		outputPath: outputPath,
	}); err != nil {
		t.Fatalf("runRender() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(data)
	if !strings.Contains(contents, "# Limier Build Summary") {
		t.Fatalf("rendered output = %q, want build summary heading", contents)
	}
}

func TestRunInspectWritesDiagnosticOutput(t *testing.T) {
	t.Parallel()

	inputPath := writeSampleReport(t, report.Report{
		Input: report.Input{
			Ecosystem:        "pip",
			Package:          "requests",
			CurrentVersion:   "2.31.0",
			CandidateVersion: "2.32.0",
		},
		TechnicalVerdict:       verdict.TechnicalInconclusive,
		OperatorRecommendation: verdict.RecommendationRerun,
		ExitCode:               2,
		Evidence: report.Evidence{
			RootPath: "/tmp/limier/evidence",
		},
		Diagnostic: report.NewDiagnostic(
			report.DiagnosticCategoryDocker,
			"candidate_docker_run_failed",
			"run candidate scenario: docker exec failed",
			"Confirm Docker is available and healthy on the runner, then rerun Limier.",
		),
	})
	outputPath := filepath.Join(t.TempDir(), "inspect.md")

	if err := runInspect(inspectOptions{
		inputPath:  inputPath,
		outputPath: outputPath,
	}); err != nil {
		t.Fatalf("runInspect() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(data)
	if !strings.Contains(contents, "structured inconclusive diagnostic available") {
		t.Fatalf("inspect output = %q, want structured diagnostic text", contents)
	}
}

func writeSampleReport(t *testing.T, runReport report.Report) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "report.json")
	if err := report.WriteJSON(path, runReport); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	return path
}
