package analysis

import (
	"fmt"
	"testing"

	"github.com/oneslash/limier/internal/collector"
	"github.com/oneslash/limier/internal/report"
	"github.com/oneslash/limier/internal/rules"
	"github.com/oneslash/limier/internal/verdict"
)

func TestEvaluateMapsRulesToVerdicts(t *testing.T) {
	t.Parallel()

	baseline := stableSide("1.0.0")
	candidate := stableSide("2.0.0")

	testCases := []struct {
		name               string
		ruleSet            rules.File
		wantTechnical      verdict.Technical
		wantRecommendation verdict.Recommendation
	}{
		{
			name: "suppressed",
			ruleSet: rules.File{
				Version: 1,
				Suppress: []rules.Rule{
					{ID: "s1", Finding: "step_stdout_changed", Step: "exercise"},
				},
			},
			wantTechnical:      verdict.TechnicalExpectedDiff,
			wantRecommendation: verdict.RecommendationGoodToGo,
		},
		{
			name: "review",
			ruleSet: rules.File{
				Version: 1,
				Review: []rules.Rule{
					{ID: "r1", Finding: "step_stdout_changed", Step: "exercise"},
				},
			},
			wantTechnical:      verdict.TechnicalUnexpectedDiff,
			wantRecommendation: verdict.RecommendationNeedsReview,
		},
		{
			name: "hard block",
			ruleSet: rules.File{
				Version: 1,
				HardBlock: []rules.Rule{
					{ID: "h1", Finding: "step_stdout_changed", Step: "exercise"},
				},
			},
			wantTechnical:      verdict.TechnicalSuspiciousDiff,
			wantRecommendation: verdict.RecommendationBlock,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := Evaluate(baseline, candidate, 0, testCase.ruleSet)
			if result.TechnicalVerdict != testCase.wantTechnical {
				t.Fatalf("technical verdict = %q, want %q", result.TechnicalVerdict, testCase.wantTechnical)
			}

			if result.OperatorRecommendation != testCase.wantRecommendation {
				t.Fatalf("operator recommendation = %q, want %q", result.OperatorRecommendation, testCase.wantRecommendation)
			}
		})
	}
}

func TestEvaluateSuppressionPreventsEscalationForSameFinding(t *testing.T) {
	t.Parallel()

	baseline := stableSide("1.0.0")
	candidate := stableSide("2.0.0")

	result := Evaluate(baseline, candidate, 0, rules.File{
		Version: 1,
		Suppress: []rules.Rule{
			{ID: "s1", Finding: "step_stdout_changed", Step: "exercise"},
		},
		Review: []rules.Rule{
			{ID: "r1", Finding: "step_stdout_changed", Step: "exercise"},
		},
		HardBlock: []rules.Rule{
			{ID: "h1", Finding: "step_stdout_changed", Step: "exercise"},
		},
	})

	if result.TechnicalVerdict != verdict.TechnicalExpectedDiff {
		t.Fatalf("technical verdict = %q, want %q", result.TechnicalVerdict, verdict.TechnicalExpectedDiff)
	}

	if result.OperatorRecommendation != verdict.RecommendationGoodToGo {
		t.Fatalf("operator recommendation = %q, want %q", result.OperatorRecommendation, verdict.RecommendationGoodToGo)
	}

	if len(result.RuleHits) != 1 {
		t.Fatalf("rule hits = %d, want 1", len(result.RuleHits))
	}

	if result.RuleHits[0].Category != "suppress" {
		t.Fatalf("rule hit category = %q, want suppress", result.RuleHits[0].Category)
	}
}

func TestEvaluateKeepsUnsuppressedFindingsActionable(t *testing.T) {
	t.Parallel()

	baseline := sideWithOutputs("baseline stdout", "baseline stderr")
	candidate := sideWithOutputs("candidate stdout", "candidate stderr")

	result := Evaluate(baseline, candidate, 0, rules.File{
		Version: 1,
		Suppress: []rules.Rule{
			{ID: "s1", Finding: "step_stdout_changed", Step: "exercise"},
		},
		Review: []rules.Rule{
			{ID: "r1", Finding: "step_stderr_changed", Step: "exercise"},
		},
	})

	if result.TechnicalVerdict != verdict.TechnicalUnexpectedDiff {
		t.Fatalf("technical verdict = %q, want %q", result.TechnicalVerdict, verdict.TechnicalUnexpectedDiff)
	}

	if result.OperatorRecommendation != verdict.RecommendationNeedsReview {
		t.Fatalf("operator recommendation = %q, want %q", result.OperatorRecommendation, verdict.RecommendationNeedsReview)
	}

	if len(result.Findings) != 2 {
		t.Fatalf("findings = %d, want 2", len(result.Findings))
	}

	if !result.Findings[0].Suppressed {
		t.Fatal("stdout finding Suppressed = false, want true")
	}

	if result.Findings[1].Suppressed {
		t.Fatal("stderr finding Suppressed = true, want false")
	}
}

func TestCompareRunsDetectsProcessExecCountChanges(t *testing.T) {
	t.Parallel()

	findings := compareRuns(
		runWithEvents(processExecEvent("exercise", "curl https://example.test")),
		runWithEvents(
			processExecEvent("exercise", "curl https://example.test"),
			processExecEvent("exercise", "curl https://example.test"),
		),
		0,
	)

	finding := requireFinding(t, findings, "process_exec_count_changed")
	if finding.Step != "exercise" {
		t.Fatalf("finding.Step = %q, want %q", finding.Step, "exercise")
	}
	if finding.Phase != "exercise" {
		t.Fatalf("finding.Phase = %q, want %q", finding.Phase, "exercise")
	}
	if finding.BaselineValue != "1" || finding.CandidateValue != "2" {
		t.Fatalf("finding counts = %q -> %q, want 1 -> 2", finding.BaselineValue, finding.CandidateValue)
	}
	if len(finding.Evidence) != 2 {
		t.Fatalf("finding evidence = %#v, want baseline and candidate events paths", finding.Evidence)
	}
}

func TestCompareRunsDetectsMissingProcessExec(t *testing.T) {
	t.Parallel()

	findings := compareRuns(
		runWithEvents(processExecEvent("exercise", "curl https://example.test")),
		runWithEvents(),
		0,
	)

	finding := requireFinding(t, findings, "missing_process_exec")
	if finding.Phase != "exercise" {
		t.Fatalf("finding.Phase = %q, want %q", finding.Phase, "exercise")
	}
	if finding.BaselineValue != "1" || finding.CandidateValue != "0" {
		t.Fatalf("finding counts = %q -> %q, want 1 -> 0", finding.BaselineValue, finding.CandidateValue)
	}
}

func TestCompareRunsDetectsNewProcessExec(t *testing.T) {
	t.Parallel()

	findings := compareRuns(
		runWithEvents(),
		runWithEvents(processExecEvent("exercise", "curl https://example.test")),
		0,
	)

	finding := requireFinding(t, findings, "new_process_exec")
	if finding.Phase != "exercise" {
		t.Fatalf("finding.Phase = %q, want %q", finding.Phase, "exercise")
	}
	if finding.BaselineValue != "0" || finding.CandidateValue != "1" {
		t.Fatalf("finding counts = %q -> %q, want 0 -> 1", finding.BaselineValue, finding.CandidateValue)
	}
}

func TestCompareRunsIgnoresMatchingProcessExecCounts(t *testing.T) {
	t.Parallel()

	findings := compareRuns(
		runWithEvents(processExecEvent("exercise", "curl https://example.test")),
		runWithEvents(processExecEvent("exercise", "curl https://example.test")),
		0,
	)

	if len(findings) != 0 {
		t.Fatalf("findings = %#v, want no exec findings", findings)
	}
}

func TestEvaluateMarksProcessExecDiffsAsNeedsReview(t *testing.T) {
	t.Parallel()

	baseline := sideWithOutputsAndEvents("stable output", "", []collector.Event{
		processExecEvent("exercise", "curl https://example.test"),
	})
	candidate := sideWithOutputsAndEvents("stable output", "", []collector.Event{
		processExecEvent("exercise", "curl https://example.test"),
		processExecEvent("exercise", "curl https://example.test"),
	})

	result := Evaluate(baseline, candidate, 0, rules.File{Version: 1})
	if result.TechnicalVerdict != verdict.TechnicalUnexpectedDiff {
		t.Fatalf("technical verdict = %q, want %q", result.TechnicalVerdict, verdict.TechnicalUnexpectedDiff)
	}
	if result.OperatorRecommendation != verdict.RecommendationNeedsReview {
		t.Fatalf("operator recommendation = %q, want %q", result.OperatorRecommendation, verdict.RecommendationNeedsReview)
	}

	requireFinding(t, result.Findings, "process_exec_count_changed")
}

func TestEvaluateDoesNotDoubleCountCandidateExitCodeDivergence(t *testing.T) {
	t.Parallel()

	baseline := sideWithRun(report.Run{
		RunIndex: 1,
		ExitCode: 0,
		Steps: []report.Step{
			{
				Name:     "exercise",
				Intent:   "exercise",
				Command:  "echo version",
				ExitCode: 0,
				Stdout: report.Output{
					Preview:    "stable output",
					TotalBytes: int64(len("stable output")),
					SHA256:     "stable output",
				},
				Stderr: report.Output{
					Preview:    "",
					TotalBytes: 0,
					SHA256:     "",
				},
			},
		},
	})
	candidate := sideWithRun(report.Run{
		RunIndex: 1,
		ExitCode: 1,
		Steps: []report.Step{
			{
				Name:     "exercise",
				Intent:   "exercise",
				Command:  "echo version",
				ExitCode: 1,
				Stdout: report.Output{
					Preview:    "stable output",
					TotalBytes: int64(len("stable output")),
					SHA256:     "stable output",
				},
				Stderr: report.Output{
					Preview:    "",
					TotalBytes: 0,
					SHA256:     "",
				},
			},
		},
	})

	result := Evaluate(baseline, candidate, 0, rules.File{Version: 1})

	var scenarioLevelExitFindings []string
	for _, finding := range result.Findings {
		switch finding.Kind {
		case "candidate_failed_or_diverged", "scenario_exit_code_changed":
			scenarioLevelExitFindings = append(scenarioLevelExitFindings, finding.Kind)
		}
	}

	if len(scenarioLevelExitFindings) != 1 {
		t.Fatalf("scenario-level exit findings = %#v, want exactly one", scenarioLevelExitFindings)
	}
	if scenarioLevelExitFindings[0] != "candidate_failed_or_diverged" {
		t.Fatalf("scenario-level exit finding = %q, want %q", scenarioLevelExitFindings[0], "candidate_failed_or_diverged")
	}
}

func stableSide(version string) report.Side {
	return sideWithOutputs(version, "")
}

func sideWithOutputs(stdout string, stderr string) report.Side {
	return sideWithOutputsAndEvents(stdout, stderr, nil)
}

func sideWithRun(run report.Run) report.Side {
	summary, stable := AssessSide([]report.Run{run})

	return report.Side{
		RequestedVersion: run.Steps[0].Stdout.Preview,
		InstalledVersion: run.Steps[0].Stdout.Preview,
		Stable:           stable,
		Summary:          summary,
		Runs:             []report.Run{run},
	}
}

func sideWithOutputsAndEvents(stdout string, stderr string, events []collector.Event) report.Side {
	step := report.Step{
		Name:                 "exercise",
		Intent:               "exercise",
		Command:              "echo version",
		ExitCode:             0,
		DurationMilliseconds: 1,
		Stdout: report.Output{
			Preview:    stdout,
			TotalBytes: int64(len(stdout)),
			SHA256:     stdout,
		},
		Stderr: report.Output{
			Preview:    stderr,
			TotalBytes: int64(len(stderr)),
			SHA256:     stderr,
		},
	}
	run := report.Run{
		RunIndex:   1,
		ExitCode:   0,
		Steps:      []report.Step{step},
		Events:     events,
		EventsPath: fmt.Sprintf("/tmp/events-%d.json", len(events)),
	}

	summary, stable := AssessSide([]report.Run{run})

	return report.Side{
		RequestedVersion: stdout,
		InstalledVersion: stdout,
		Stable:           stable,
		Summary:          summary,
		Runs:             []report.Run{run},
	}
}

func runWithEvents(events ...collector.Event) report.Run {
	return sideWithOutputsAndEvents("stable output", "", events).Runs[0]
}

func processExecEvent(step string, command string) collector.Event {
	return collector.Event{
		Kind:    "process.exec",
		Step:    step,
		Command: command,
	}
}

func requireFinding(t *testing.T, findings []report.Finding, kind string) report.Finding {
	t.Helper()

	for _, finding := range findings {
		if finding.Kind == kind {
			return finding
		}
	}

	t.Fatalf("finding kind %q not found in %#v", kind, findings)
	return report.Finding{}
}
