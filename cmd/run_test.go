package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/room215/limier/internal/verdict"
)

func TestPolicyExitCodePreservesDefaultWhenFailOnEmpty(t *testing.T) {
	t.Parallel()

	exitCode, err := policyExitCode(verdict.RecommendationNeedsReview, 1, "")
	if err != nil {
		t.Fatalf("policyExitCode() error = %v", err)
	}
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
}

func TestPolicyExitCodeAllowsNeedsReviewWhenNotListed(t *testing.T) {
	t.Parallel()

	exitCode, err := policyExitCode(verdict.RecommendationNeedsReview, 1, "block,rerun")
	if err != nil {
		t.Fatalf("policyExitCode() error = %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestPolicyExitCodeFailsListedRecommendation(t *testing.T) {
	t.Parallel()

	exitCode, err := policyExitCode(verdict.RecommendationBlock, 1, "block,rerun")
	if err != nil {
		t.Fatalf("policyExitCode() error = %v", err)
	}
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
}

func TestPolicyExitCodeFailsRerunWithInfrastructureExitCode(t *testing.T) {
	t.Parallel()

	exitCode, err := policyExitCode(verdict.RecommendationRerun, 0, "block,rerun")
	if err != nil {
		t.Fatalf("policyExitCode() error = %v", err)
	}
	if exitCode != 2 {
		t.Fatalf("exitCode = %d, want 2", exitCode)
	}
}

func TestPolicyExitCodeRejectsUnknownFailOnRecommendation(t *testing.T) {
	t.Parallel()

	exitCode, err := policyExitCode(verdict.RecommendationBlock, 1, "block,unknown")
	if err == nil {
		t.Fatal("policyExitCode() error = nil, want error")
	}
	if exitCode != 2 {
		t.Fatalf("exitCode = %d, want 2", exitCode)
	}
	if !strings.Contains(err.Error(), "unsupported --fail-on recommendation") {
		t.Fatalf("policyExitCode() error = %q, want unsupported recommendation", err)
	}
}

func TestResolveRunPresetsMaterializesNPMDefaults(t *testing.T) {
	t.Parallel()

	resolved, cleanup, err := resolveRunPresets(runOptions{
		ecosystem:    "npm",
		packageName:  "left-pad",
		evidencePath: filepath.Join(t.TempDir(), "evidence"),
	})
	if err != nil {
		t.Fatalf("resolveRunPresets() error = %v", err)
	}
	defer cleanup()

	for _, path := range []string{
		resolved.fixturePath,
		resolved.scenarioPath,
		resolved.rulesPath,
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Stat(%q) error = %v", path, err)
		}
	}
}
