package analysis

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/room215/limier/internal/collector"
	"github.com/room215/limier/internal/report"
	"github.com/room215/limier/internal/rules"
	"github.com/room215/limier/internal/verdict"
)

type Result struct {
	Findings               []report.Finding
	RuleHits               []report.RuleHit
	TechnicalVerdict       verdict.Technical
	OperatorRecommendation verdict.Recommendation
	Diagnostic             *report.Diagnostic
}

func AssessSide(runs []report.Run) (report.SideSummary, bool) {
	summary := report.SideSummary{
		RunCount: len(runs),
	}

	if len(runs) == 0 {
		return summary, false
	}

	reference := signature(runs[0])
	summary.StepCount = len(runs[0].Steps)
	summary.ExitCode = runs[0].ExitCode
	summary.EventCounts = map[string]int{}
	for _, event := range runs[0].Events {
		summary.EventCounts[event.Kind]++
	}

	for _, run := range runs[1:] {
		if signature(run) != reference {
			return summary, false
		}
	}

	return summary, true
}

func Evaluate(baseline report.Side, candidate report.Side, expectedExitCode int, ruleSet rules.File) Result {
	if !baseline.Stable {
		return inconclusive(
			report.DiagnosticCategoryStability,
			"baseline_repeats_unstable",
			"baseline output changed between repeated runs",
			"Rerun after making the baseline scenario deterministic enough to produce the same output on repeated runs.",
		)
	}

	if !candidate.Stable {
		return inconclusive(
			report.DiagnosticCategoryStability,
			"candidate_repeats_unstable",
			"candidate output changed between repeated runs",
			"Rerun after making the candidate scenario deterministic enough to produce the same output on repeated runs.",
		)
	}

	if len(baseline.Runs) == 0 {
		return inconclusive(
			report.DiagnosticCategoryExecution,
			"baseline_produced_no_runs",
			"baseline produced no runs",
			"Inspect the baseline evidence and confirm the scenario actually executed inside the fixture.",
		)
	}

	if len(candidate.Runs) == 0 {
		return inconclusive(
			report.DiagnosticCategoryExecution,
			"candidate_produced_no_runs",
			"candidate produced no runs",
			"Inspect the candidate evidence and confirm the scenario actually executed inside the fixture.",
		)
	}

	baselineRepresentative := baseline.Runs[0]
	candidateRepresentative := candidate.Runs[0]

	if baselineRepresentative.ExitCode != expectedExitCode {
		return inconclusive(
			report.DiagnosticCategoryExecution,
			"baseline_diverged_from_success_criteria",
			fmt.Sprintf("baseline diverged from the scenario success criteria: expected exit code %d, got %d", expectedExitCode, baselineRepresentative.ExitCode),
			"Fix the baseline scenario or its success criteria before trusting the comparison result.",
		)
	}

	findings := compareRuns(baselineRepresentative, candidateRepresentative, expectedExitCode)
	if len(findings) == 0 {
		return Result{
			TechnicalVerdict:       verdict.TechnicalNoDiff,
			OperatorRecommendation: verdict.RecommendationGoodToGo,
		}
	}

	findings, ruleHits := applyRules(findings, ruleSet)

	hasUnsuppressed := false
	hasHardBlock := false
	for _, finding := range findings {
		if !finding.Suppressed {
			hasUnsuppressed = true
		}
	}

	for _, hit := range ruleHits {
		if hit.Category == "hard_block" {
			hasHardBlock = true
			break
		}
	}

	switch {
	case hasHardBlock:
		return Result{
			Findings:               findings,
			RuleHits:               ruleHits,
			TechnicalVerdict:       verdict.TechnicalSuspiciousDiff,
			OperatorRecommendation: verdict.RecommendationBlock,
		}
	case hasUnsuppressed:
		return Result{
			Findings:               findings,
			RuleHits:               ruleHits,
			TechnicalVerdict:       verdict.TechnicalUnexpectedDiff,
			OperatorRecommendation: verdict.RecommendationNeedsReview,
		}
	default:
		return Result{
			Findings:               findings,
			RuleHits:               ruleHits,
			TechnicalVerdict:       verdict.TechnicalExpectedDiff,
			OperatorRecommendation: verdict.RecommendationGoodToGo,
		}
	}
}

func inconclusive(category report.DiagnosticCategory, code string, message string, suggestedAction string) Result {
	finding := report.Finding{
		ID:      "finding-1",
		Kind:    "inconclusive_run",
		Message: message,
	}

	return Result{
		Findings:               []report.Finding{finding},
		TechnicalVerdict:       verdict.TechnicalInconclusive,
		OperatorRecommendation: verdict.RecommendationRerun,
		Diagnostic: report.NewDiagnostic(
			category,
			code,
			message,
			suggestedAction,
		),
	}
}

func compareRuns(baseline report.Run, candidate report.Run, expectedExitCode int) []report.Finding {
	var findings []report.Finding

	candidateDiverged := candidate.ExitCode != expectedExitCode
	exitCodeChanged := baseline.ExitCode != candidate.ExitCode

	if candidateDiverged {
		findings = append(findings, report.Finding{
			ID:             nextFindingID(len(findings)),
			Kind:           "candidate_failed_or_diverged",
			Message:        fmt.Sprintf("candidate diverged from the scenario success criteria: expected exit code %d, got %d", expectedExitCode, candidate.ExitCode),
			BaselineValue:  fmt.Sprintf("%d", baseline.ExitCode),
			CandidateValue: fmt.Sprintf("%d", candidate.ExitCode),
		})
	}

	if exitCodeChanged && !candidateDiverged {
		findings = append(findings, report.Finding{
			ID:             nextFindingID(len(findings)),
			Kind:           "scenario_exit_code_changed",
			Message:        fmt.Sprintf("scenario exit code changed from %d to %d", baseline.ExitCode, candidate.ExitCode),
			BaselineValue:  fmt.Sprintf("%d", baseline.ExitCode),
			CandidateValue: fmt.Sprintf("%d", candidate.ExitCode),
		})
	}

	maxSteps := len(baseline.Steps)
	if len(candidate.Steps) > maxSteps {
		maxSteps = len(candidate.Steps)
	}

	if len(baseline.Steps) != len(candidate.Steps) {
		findings = append(findings, report.Finding{
			ID:             nextFindingID(len(findings)),
			Kind:           "step_count_changed",
			Message:        fmt.Sprintf("scenario step count changed from %d to %d", len(baseline.Steps), len(candidate.Steps)),
			BaselineValue:  fmt.Sprintf("%d", len(baseline.Steps)),
			CandidateValue: fmt.Sprintf("%d", len(candidate.Steps)),
		})
	}

	for index := 0; index < maxSteps; index++ {
		if index >= len(baseline.Steps) || index >= len(candidate.Steps) {
			continue
		}

		baselineStep := baseline.Steps[index]
		candidateStep := candidate.Steps[index]

		if baselineStep.ExitCode != candidateStep.ExitCode {
			findings = append(findings, report.Finding{
				ID:             nextFindingID(len(findings)),
				Kind:           "step_exit_code_changed",
				Step:           candidateStep.Name,
				Phase:          candidateStep.Intent,
				Message:        fmt.Sprintf("step %q exit code changed from %d to %d", candidateStep.Name, baselineStep.ExitCode, candidateStep.ExitCode),
				BaselineValue:  fmt.Sprintf("%d", baselineStep.ExitCode),
				CandidateValue: fmt.Sprintf("%d", candidateStep.ExitCode),
				Evidence:       evidencePaths(baselineStep.Evidence.Stdout.Path, baselineStep.Evidence.Stderr.Path, candidateStep.Evidence.Stdout.Path, candidateStep.Evidence.Stderr.Path),
			})
		}

		if outputChanged(baselineStep.Stdout, candidateStep.Stdout) {
			findings = append(findings, report.Finding{
				ID:             nextFindingID(len(findings)),
				Kind:           "step_stdout_changed",
				Step:           candidateStep.Name,
				Phase:          candidateStep.Intent,
				Message:        fmt.Sprintf("step %q stdout changed", candidateStep.Name),
				BaselineValue:  describeOutput(baselineStep.Stdout),
				CandidateValue: describeOutput(candidateStep.Stdout),
				Evidence:       evidencePaths(baselineStep.Evidence.Stdout.Path, candidateStep.Evidence.Stdout.Path),
			})
		}

		if outputChanged(baselineStep.Stderr, candidateStep.Stderr) {
			findings = append(findings, report.Finding{
				ID:             nextFindingID(len(findings)),
				Kind:           "step_stderr_changed",
				Step:           candidateStep.Name,
				Phase:          candidateStep.Intent,
				Message:        fmt.Sprintf("step %q stderr changed", candidateStep.Name),
				BaselineValue:  describeOutput(baselineStep.Stderr),
				CandidateValue: describeOutput(candidateStep.Stderr),
				Evidence:       evidencePaths(baselineStep.Evidence.Stderr.Path, candidateStep.Evidence.Stderr.Path),
			})
		}
	}

	findings = append(findings, compareExecEvents(baseline, candidate, len(findings))...)

	return findings
}

func applyRules(findings []report.Finding, ruleSet rules.File) ([]report.Finding, []report.RuleHit) {
	var hits []report.RuleHit

	for index, finding := range findings {
		for _, rule := range ruleSet.Suppress {
			if matches(rule, finding) {
				findings[index].Suppressed = true
				hits = append(hits, report.RuleHit{
					RuleID:    rule.ID,
					Category:  "suppress",
					FindingID: finding.ID,
					Reason:    rule.Reason,
				})
			}
		}
	}

	for index, finding := range findings {
		if findings[index].Suppressed {
			continue
		}

		for _, rule := range ruleSet.Review {
			if matches(rule, findings[index]) {
				hits = append(hits, report.RuleHit{
					RuleID:    rule.ID,
					Category:  "review",
					FindingID: finding.ID,
					Reason:    rule.Reason,
				})
			}
		}

		for _, rule := range ruleSet.HardBlock {
			if matches(rule, findings[index]) {
				hits = append(hits, report.RuleHit{
					RuleID:    rule.ID,
					Category:  "hard_block",
					FindingID: finding.ID,
					Reason:    rule.Reason,
				})
			}
		}
	}

	return findings, hits
}

func matches(rule rules.Rule, finding report.Finding) bool {
	if rule.Finding != finding.Kind {
		return false
	}

	if strings.TrimSpace(rule.Step) != "" && rule.Step != finding.Step {
		return false
	}

	if strings.TrimSpace(rule.MessageContains) != "" && !strings.Contains(finding.Message, rule.MessageContains) {
		return false
	}

	return true
}

type execEventKey struct {
	Step    string
	Command string
}

func compareExecEvents(baseline report.Run, candidate report.Run, existingFindings int) []report.Finding {
	baselineCounts := countExecEvents(baseline.Events)
	candidateCounts := countExecEvents(candidate.Events)
	stepPhases := stepPhases(baseline, candidate)

	keys := make([]execEventKey, 0, len(baselineCounts)+len(candidateCounts))
	seen := map[execEventKey]struct{}{}
	for key := range baselineCounts {
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for key := range candidateCounts {
		if _, ok := seen[key]; ok {
			continue
		}
		keys = append(keys, key)
	}

	sort.Slice(keys, func(i int, j int) bool {
		if keys[i].Step != keys[j].Step {
			return keys[i].Step < keys[j].Step
		}
		return keys[i].Command < keys[j].Command
	})

	findings := make([]report.Finding, 0, len(keys))
	for _, key := range keys {
		baselineCount := baselineCounts[key]
		candidateCount := candidateCounts[key]

		switch {
		case baselineCount == 0 && candidateCount > 0:
			findings = append(findings, report.Finding{
				ID:             nextFindingID(existingFindings + len(findings)),
				Kind:           "new_process_exec",
				Step:           key.Step,
				Phase:          stepPhases[key.Step],
				Message:        fmt.Sprintf("candidate executed a new process during step %q: %s", key.Step, key.Command),
				BaselineValue:  fmt.Sprintf("%d", baselineCount),
				CandidateValue: fmt.Sprintf("%d", candidateCount),
				Evidence:       evidencePaths(baseline.EventsPath, candidate.EventsPath),
			})
		case baselineCount > 0 && candidateCount == 0:
			findings = append(findings, report.Finding{
				ID:             nextFindingID(existingFindings + len(findings)),
				Kind:           "missing_process_exec",
				Step:           key.Step,
				Phase:          stepPhases[key.Step],
				Message:        fmt.Sprintf("candidate no longer executed a process during step %q: %s", key.Step, key.Command),
				BaselineValue:  fmt.Sprintf("%d", baselineCount),
				CandidateValue: fmt.Sprintf("%d", candidateCount),
				Evidence:       evidencePaths(baseline.EventsPath, candidate.EventsPath),
			})
		case baselineCount != candidateCount:
			findings = append(findings, report.Finding{
				ID:             nextFindingID(existingFindings + len(findings)),
				Kind:           "process_exec_count_changed",
				Step:           key.Step,
				Phase:          stepPhases[key.Step],
				Message:        fmt.Sprintf("process execution count changed during step %q: %s", key.Step, key.Command),
				BaselineValue:  fmt.Sprintf("%d", baselineCount),
				CandidateValue: fmt.Sprintf("%d", candidateCount),
				Evidence:       evidencePaths(baseline.EventsPath, candidate.EventsPath),
			})
		}
	}

	return findings
}

func countExecEvents(events []collector.Event) map[execEventKey]int {
	counts := map[execEventKey]int{}
	for _, event := range events {
		if event.Kind != "process.exec" {
			continue
		}
		counts[execEventKey{
			Step:    event.Step,
			Command: event.Command,
		}]++
	}

	return counts
}

func stepPhases(baseline report.Run, candidate report.Run) map[string]string {
	phases := map[string]string{}

	for _, step := range candidate.Steps {
		if strings.TrimSpace(step.Name) == "" || strings.TrimSpace(step.Intent) == "" {
			continue
		}
		phases[step.Name] = step.Intent
	}

	for _, step := range baseline.Steps {
		if strings.TrimSpace(step.Name) == "" || strings.TrimSpace(step.Intent) == "" {
			continue
		}
		if _, ok := phases[step.Name]; ok {
			continue
		}
		phases[step.Name] = step.Intent
	}

	return phases
}

func nextFindingID(existing int) string {
	return fmt.Sprintf("finding-%d", existing+1)
}

func outputChanged(baseline report.Output, candidate report.Output) bool {
	return baseline.TotalBytes != candidate.TotalBytes || baseline.SHA256 != candidate.SHA256
}

func describeOutput(output report.Output) string {
	if !output.Truncated {
		return output.Preview
	}

	return fmt.Sprintf("%s\n[truncated preview: %d total bytes, sha256=%s]", output.Preview, output.TotalBytes, output.SHA256)
}

func evidencePaths(paths ...string) []string {
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		filtered = append(filtered, path)
	}

	return filtered
}

func signature(run report.Run) string {
	type eventSignature struct {
		Kind     string `json:"kind"`
		Step     string `json:"step,omitempty"`
		Command  string `json:"command,omitempty"`
		ExitCode *int   `json:"exit_code,omitempty"`
	}

	type stepSignature struct {
		Name         string `json:"name"`
		Intent       string `json:"intent"`
		Command      string `json:"command"`
		ExitCode     int    `json:"exit_code"`
		StdoutSHA256 string `json:"stdout_sha256"`
		StdoutBytes  int64  `json:"stdout_total_bytes"`
		StderrSHA256 string `json:"stderr_sha256"`
		StderrBytes  int64  `json:"stderr_total_bytes"`
	}

	type runSignature struct {
		ExitCode int              `json:"exit_code"`
		Steps    []stepSignature  `json:"steps"`
		Events   []eventSignature `json:"events"`
	}

	signature := runSignature{
		ExitCode: run.ExitCode,
	}

	for _, step := range run.Steps {
		signature.Steps = append(signature.Steps, stepSignature{
			Name:         step.Name,
			Intent:       step.Intent,
			Command:      step.Command,
			ExitCode:     step.ExitCode,
			StdoutSHA256: step.Stdout.SHA256,
			StdoutBytes:  step.Stdout.TotalBytes,
			StderrSHA256: step.Stderr.SHA256,
			StderrBytes:  step.Stderr.TotalBytes,
		})
	}

	for _, event := range run.Events {
		signature.Events = append(signature.Events, eventSignature{
			Kind:     event.Kind,
			Step:     event.Step,
			Command:  event.Command,
			ExitCode: event.ExitCode,
		})
	}

	data, _ := json.Marshal(signature)
	return string(data)
}
