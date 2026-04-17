package render

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oneslash/limier/internal/report"
	"github.com/oneslash/limier/internal/verdict"
)

func TestRenderGoldenOutputs(t *testing.T) {
	t.Parallel()

	runReport := sampleReviewReport()
	cases := []struct {
		name   string
		format string
		golden string
	}{
		{
			name:   "github comment",
			format: FormatGitHubComment,
			golden: "github-comment.golden.md",
		},
		{
			name:   "gitlab note",
			format: FormatGitLabNote,
			golden: "gitlab-note.golden.md",
		},
		{
			name:   "build summary",
			format: FormatBuildSummary,
			golden: "build-summary.golden.md",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := Render(runReport, tc.format)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			want := readGoldenFile(t, tc.golden)
			if got != want {
				t.Fatalf("Render() output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

func TestInspectGoldenOutputs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		report report.Report
		golden string
	}{
		{
			name:   "conclusive report",
			report: sampleReviewReport(),
			golden: "inspect-conclusive.golden.md",
		},
		{
			name:   "inconclusive report",
			report: sampleInconclusiveReport(),
			golden: "inspect-inconclusive.golden.md",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := Inspect(tc.report)
			want := readGoldenFile(t, tc.golden)
			if got != want {
				t.Fatalf("Inspect() output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

func TestRenderRejectsUnknownFormat(t *testing.T) {
	t.Parallel()

	if _, err := Render(sampleReviewReport(), "unknown"); err == nil {
		t.Fatal("Render() error = nil, want unsupported format error")
	}
}

func readGoldenFile(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	return string(data)
}

func sampleReviewReport() report.Report {
	return report.Report{
		Input: report.Input{
			Ecosystem:        "npm",
			Package:          "left-pad",
			CurrentVersion:   "1.0.0",
			CandidateVersion: "1.1.0",
		},
		Findings: []report.Finding{
			{
				ID:      "finding-1",
				Kind:    "step_stdout_changed",
				Step:    "exercise",
				Phase:   "exercise",
				Message: `step "exercise" stdout changed`,
			},
			{
				ID:      "finding-2",
				Kind:    "new_process_exec",
				Step:    "exercise",
				Phase:   "exercise",
				Message: `candidate executed a new process during step "exercise": curl https://example.test`,
			},
		},
		RuleHits: []report.RuleHit{
			{
				RuleID:    "review-network-activity",
				Category:  "review",
				FindingID: "finding-2",
				Reason:    "new outbound network tool execution",
			},
		},
		TechnicalVerdict:       verdict.TechnicalUnexpectedDiff,
		OperatorRecommendation: verdict.RecommendationNeedsReview,
		ExitCode:               1,
		Baseline: report.Side{
			Stable: true,
			Summary: report.SideSummary{
				RunCount: 2,
			},
		},
		Candidate: report.Side{
			Stable: true,
			Summary: report.SideSummary{
				RunCount: 2,
			},
		},
		Evidence: report.Evidence{
			RootPath: "/tmp/limier/evidence",
		},
	}
}

func sampleInconclusiveReport() report.Report {
	return report.Report{
		Input: report.Input{
			Ecosystem:        "pip",
			Package:          "requests",
			CurrentVersion:   "2.31.0",
			CandidateVersion: "2.32.0",
		},
		TechnicalVerdict:       verdict.TechnicalInconclusive,
		OperatorRecommendation: verdict.RecommendationRerun,
		ExitCode:               2,
		Baseline: report.Side{
			Stable: true,
			Summary: report.SideSummary{
				RunCount: 1,
			},
		},
		Candidate: report.Side{
			Summary: report.SideSummary{
				RunCount: 0,
			},
		},
		Evidence: report.Evidence{
			RootPath: "/tmp/limier/evidence",
		},
		Diagnostic: report.NewDiagnostic(
			report.DiagnosticCategoryDocker,
			"candidate_docker_run_failed",
			`run candidate scenario: docker exec "exercise": exit status 1`,
			"Confirm Docker is available and healthy on the runner, then rerun Limier.",
			"/tmp/limier/evidence/candidate/run-1",
		),
	}
}
