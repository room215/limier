package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGitHubCIWritesNotApplicableStatusWithoutMetadata(t *testing.T) {
	t.Parallel()

	outputDir := filepath.Join(t.TempDir(), "limier")
	if err := runGitHubCI(t.Context(), githubCIOptions{
		outputDir: outputDir,
		failOn:    "block,rerun",
		getenv:    emptyCIEnv,
	}); err != nil {
		t.Fatalf("runGitHubCI() error = %v", err)
	}

	status := readCIStatus(t, filepath.Join(outputDir, "status.json"))
	if status.Status != "not_applicable" {
		t.Fatalf("status.Status = %q, want not_applicable", status.Status)
	}
	if status.OperatorRecommendation != "good_to_go" {
		t.Fatalf("status.OperatorRecommendation = %q, want good_to_go", status.OperatorRecommendation)
	}

	data, err := os.ReadFile(filepath.Join(outputDir, "build-summary.md"))
	if err != nil {
		t.Fatalf("ReadFile(build-summary.md) error = %v", err)
	}
	if !strings.Contains(string(data), "No dependency metadata was available") {
		t.Fatalf("build summary = %q, want no metadata message", string(data))
	}
}

func TestRunGitHubCIFailOnNeedsReviewFailsGroupedUpdate(t *testing.T) {
	t.Parallel()

	outputDir := filepath.Join(t.TempDir(), "limier")
	err := runGitHubCI(t.Context(), githubCIOptions{
		outputDir:        outputDir,
		failOn:           "needs_review,block,rerun",
		ecosystem:        "npm",
		packageName:      "a,b",
		currentVersion:   "1.0.0",
		candidateVersion: "1.1.0",
		getenv:           emptyCIEnv,
	})
	if err == nil {
		t.Fatal("runGitHubCI() error = nil, want exit error")
	}

	var exitErr interface {
		ExitCode() int
	}
	if !errors.As(err, &exitErr) {
		t.Fatalf("runGitHubCI() error = %T, want exit error", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("ExitCode() = %d, want 1", exitErr.ExitCode())
	}

	status := readCIStatus(t, filepath.Join(outputDir, "status.json"))
	if status.Status != "needs_review" {
		t.Fatalf("status.Status = %q, want needs_review", status.Status)
	}
}

func TestRunGitHubCIFailsClosedWhenDependabotMetadataFails(t *testing.T) {
	t.Parallel()

	outputDir := filepath.Join(t.TempDir(), "limier")
	err := runGitHubCI(t.Context(), githubCIOptions{
		outputDir:       outputDir,
		failOn:          "block,rerun",
		metadataOutcome: "failure",
		prAuthor:        "dependabot[bot]",
		getenv:          emptyCIEnv,
	})
	if err == nil {
		t.Fatal("runGitHubCI() error = nil, want exit error")
	}

	var exitErr interface {
		ExitCode() int
	}
	if !errors.As(err, &exitErr) {
		t.Fatalf("runGitHubCI() error = %T, want exit error", err)
	}
	if exitErr.ExitCode() != 2 {
		t.Fatalf("ExitCode() = %d, want 2", exitErr.ExitCode())
	}

	status := readCIStatus(t, filepath.Join(outputDir, "status.json"))
	if status.Status != "rerun" {
		t.Fatalf("status.Status = %q, want rerun", status.Status)
	}
	if status.OperatorRecommendation != "rerun" {
		t.Fatalf("status.OperatorRecommendation = %q, want rerun", status.OperatorRecommendation)
	}
}

func TestRunGitHubCIWritesStatusWhenRunFails(t *testing.T) {
	t.Parallel()

	outputDir := filepath.Join(t.TempDir(), "limier")
	err := runGitHubCI(t.Context(), githubCIOptions{
		outputDir:        outputDir,
		failOn:           "block,rerun",
		ecosystem:        "npm",
		packageName:      "left-pad",
		currentVersion:   "1.1.0",
		candidateVersion: "1.3.0",
		rulesPath:        "preset:missing",
		getenv:           emptyCIEnv,
	})
	if err == nil {
		t.Fatal("runGitHubCI() error = nil, want exit error")
	}

	var exitErr interface {
		ExitCode() int
	}
	if !errors.As(err, &exitErr) {
		t.Fatalf("runGitHubCI() error = %T, want exit error", err)
	}
	if exitErr.ExitCode() != 2 {
		t.Fatalf("ExitCode() = %d, want 2", exitErr.ExitCode())
	}

	status := readCIStatus(t, filepath.Join(outputDir, "status.json"))
	if status.Status != "rerun" {
		t.Fatalf("status.Status = %q, want rerun", status.Status)
	}
	if status.OperatorRecommendation != "rerun" {
		t.Fatalf("status.OperatorRecommendation = %q, want rerun", status.OperatorRecommendation)
	}
	if !strings.Contains(status.Message, "unsupported rules preset") {
		t.Fatalf("status.Message = %q, want unsupported rules preset", status.Message)
	}
}

func emptyCIEnv(string) string {
	return ""
}

func readCIStatus(t *testing.T, path string) ciStatus {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var status ciStatus
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	return status
}
