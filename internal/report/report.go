package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oneslash/limier/internal/collector"
	"github.com/oneslash/limier/internal/verdict"
)

type DiagnosticCategory string

const (
	DiagnosticCategoryInput      DiagnosticCategory = "invalid_input"
	DiagnosticCategoryValidation DiagnosticCategory = "validation_failure"
	DiagnosticCategoryDocker     DiagnosticCategory = "docker_failure"
	DiagnosticCategoryExecution  DiagnosticCategory = "execution_failure"
	DiagnosticCategoryStability  DiagnosticCategory = "unstable_behavior"
	DiagnosticCategoryInternal   DiagnosticCategory = "internal_failure"
)

type Report struct {
	LimierVersion          string                 `json:"limier_version"`
	GeneratedAt            time.Time              `json:"generated_at"`
	Input                  Input                  `json:"input"`
	Scenario               ScenarioIdentity       `json:"scenario"`
	Rules                  RulesIdentity          `json:"rules"`
	Baseline               Side                   `json:"baseline"`
	Candidate              Side                   `json:"candidate"`
	Findings               []Finding              `json:"findings,omitempty"`
	RuleHits               []RuleHit              `json:"rule_hits,omitempty"`
	TechnicalVerdict       verdict.Technical      `json:"technical_verdict"`
	OperatorRecommendation verdict.Recommendation `json:"operator_recommendation"`
	ExitCode               int                    `json:"exit_code"`
	Evidence               Evidence               `json:"evidence"`
	Diagnostic             *Diagnostic            `json:"diagnostic,omitempty"`
}

type Input struct {
	Ecosystem        string `json:"ecosystem"`
	Package          string `json:"package"`
	CurrentVersion   string `json:"current_version"`
	CandidateVersion string `json:"candidate_version"`
	FixturePath      string `json:"fixture_path"`
	ScenarioPath     string `json:"scenario_path"`
	RulesPath        string `json:"rules_path"`
}

type ScenarioIdentity struct {
	Name               string   `json:"name"`
	Path               string   `json:"path"`
	Repeats            int      `json:"repeats"`
	Image              string   `json:"image"`
	Workdir            string   `json:"workdir"`
	Steps              []string `json:"steps"`
	CaptureHostSignals bool     `json:"capture_host_signals"`
}

type RulesIdentity struct {
	Path           string `json:"path"`
	HardBlockCount int    `json:"hard_block_count"`
	ReviewCount    int    `json:"review_count"`
	SuppressCount  int    `json:"suppress_count"`
}

type Side struct {
	RequestedVersion string            `json:"requested_version"`
	InstalledVersion string            `json:"installed_version,omitempty"`
	Stable           bool              `json:"stable"`
	AdapterMetadata  map[string]string `json:"adapter_metadata,omitempty"`
	Summary          SideSummary       `json:"summary"`
	Runs             []Run             `json:"runs,omitempty"`
}

type SideSummary struct {
	RunCount    int            `json:"run_count"`
	StepCount   int            `json:"step_count"`
	ExitCode    int            `json:"exit_code"`
	EventCounts map[string]int `json:"event_counts,omitempty"`
}

type Run struct {
	RunIndex             int               `json:"run_index"`
	ExitCode             int               `json:"exit_code"`
	DurationMilliseconds int64             `json:"duration_ms"`
	Steps                []Step            `json:"steps"`
	Events               []collector.Event `json:"events,omitempty"`
	EventsPath           string            `json:"events_path,omitempty"`
}

type Step struct {
	Name                 string       `json:"name"`
	Intent               string       `json:"intent"`
	Command              string       `json:"command"`
	ExitCode             int          `json:"exit_code"`
	DurationMilliseconds int64        `json:"duration_ms"`
	Stdout               Output       `json:"stdout"`
	Stderr               Output       `json:"stderr"`
	Evidence             StepEvidence `json:"evidence"`
}

type Output struct {
	Preview    string `json:"preview,omitempty"`
	TotalBytes int64  `json:"total_bytes"`
	SHA256     string `json:"sha256"`
	Truncated  bool   `json:"truncated,omitempty"`
}

type StepEvidence struct {
	Stdout OutputFile `json:"stdout,omitempty"`
	Stderr OutputFile `json:"stderr,omitempty"`
}

type OutputFile struct {
	Path        string `json:"path,omitempty"`
	StoredBytes int64  `json:"stored_bytes"`
	Truncated   bool   `json:"truncated,omitempty"`
}

type Finding struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Step           string   `json:"step,omitempty"`
	Phase          string   `json:"phase,omitempty"`
	Message        string   `json:"message"`
	BaselineValue  string   `json:"baseline_value,omitempty"`
	CandidateValue string   `json:"candidate_value,omitempty"`
	Suppressed     bool     `json:"suppressed,omitempty"`
	Evidence       []string `json:"evidence,omitempty"`
}

type RuleHit struct {
	RuleID    string `json:"rule_id"`
	Category  string `json:"category"`
	FindingID string `json:"finding_id"`
	Reason    string `json:"reason,omitempty"`
}

type Evidence struct {
	RootPath string   `json:"root_path"`
	Files    []string `json:"files,omitempty"`
}

type Diagnostic struct {
	Category        DiagnosticCategory `json:"category"`
	Code            string             `json:"code"`
	Summary         string             `json:"summary"`
	SuggestedAction string             `json:"suggested_action,omitempty"`
	Evidence        []string           `json:"evidence,omitempty"`
}

func NewDiagnostic(category DiagnosticCategory, code string, summary string, suggestedAction string, evidence ...string) *Diagnostic {
	trimmedEvidence := make([]string, 0, len(evidence))
	seen := map[string]struct{}{}
	for _, path := range evidence {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		trimmedEvidence = append(trimmedEvidence, path)
	}
	sort.Strings(trimmedEvidence)

	return &Diagnostic{
		Category:        category,
		Code:            strings.TrimSpace(code),
		Summary:         strings.TrimSpace(summary),
		SuggestedAction: strings.TrimSpace(suggestedAction),
		Evidence:        trimmedEvidence,
	}
}

func (r Report) Inconclusive() bool {
	return r.TechnicalVerdict == verdict.TechnicalInconclusive || r.ExitCode == verdict.ExitCode(verdict.RecommendationRerun)
}

func (r Report) ComparisonLine() string {
	packageName := DisplayValue(r.Input.Package)
	baseline := DisplayValue(r.Input.CurrentVersion)
	candidate := DisplayValue(r.Input.CandidateVersion)
	ecosystem := DisplayValue(r.Input.Ecosystem)

	return fmt.Sprintf("%s `%s` -> `%s` in `%s`", packageName, baseline, candidate, ecosystem)
}

func (r Report) NextStep() string {
	if r.Diagnostic != nil && r.Diagnostic.SuggestedAction != "" {
		return r.Diagnostic.SuggestedAction
	}

	switch r.OperatorRecommendation {
	case verdict.RecommendationGoodToGo:
		return "The change looks safe enough to approve with the current ruleset."
	case verdict.RecommendationBlock:
		return "Block the upgrade until the suspicious behavior is explained or removed."
	case verdict.RecommendationNeedsReview:
		return "Review the findings and evidence before approving the upgrade."
	default:
		return "Rerun the scenario after fixing the Limier or fixture issue."
	}
}

func WriteAll(reportPath string, summaryPath string, runReport Report) error {
	if err := WriteJSON(reportPath, runReport); err != nil {
		return err
	}

	if err := WriteSummary(summaryPath, runReport); err != nil {
		return err
	}

	return nil
}

func WriteJSON(path string, runReport Report) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create report %q: %w", path, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(runReport); err != nil {
		return fmt.Errorf("encode report %q: %w", path, err)
	}

	return nil
}

func ReadJSON(path string) (Report, error) {
	file, err := os.Open(path)
	if err != nil {
		return Report{}, fmt.Errorf("open report %q: %w", path, err)
	}
	defer file.Close()

	var runReport Report
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&runReport); err != nil {
		return Report{}, fmt.Errorf("decode report %q: %w", path, err)
	}

	return runReport, nil
}

func WriteSummary(path string, runReport Report) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(BuildSummary(runReport)), 0o644); err != nil {
		return fmt.Errorf("write summary %q: %w", path, err)
	}

	return nil
}

func BuildSummary(runReport Report) string {
	var lines []string

	lines = append(lines, "# Limier Summary", "")
	lines = append(lines, "## Comparison", "")
	lines = append(lines, fmt.Sprintf("- Ecosystem: %s", DisplayValue(runReport.Input.Ecosystem)))
	lines = append(lines, fmt.Sprintf("- Package: %s", DisplayValue(runReport.Input.Package)))
	lines = append(lines, fmt.Sprintf("- Baseline version: %s", versionSummary(runReport.Input.CurrentVersion, runReport.Baseline.InstalledVersion)))
	lines = append(lines, fmt.Sprintf("- Candidate version: %s", versionSummary(runReport.Input.CandidateVersion, runReport.Candidate.InstalledVersion)))
	lines = append(lines, fmt.Sprintf("- Fixture: %s", DisplayValue(runReport.Input.FixturePath)))
	lines = append(lines, fmt.Sprintf("- Scenario: %s", DisplayValue(runReport.Scenario.Name)))
	lines = append(lines, fmt.Sprintf("- Scenario path: %s", DisplayValue(runReport.Scenario.Path)))
	lines = append(lines, fmt.Sprintf("- Host signal capture: %s", enabledDisabled(runReport.Scenario.CaptureHostSignals)))
	lines = append(lines, fmt.Sprintf("- Rules path: %s", DisplayValue(runReport.Rules.Path)))
	lines = append(lines, "")
	lines = append(lines, "## Verdict", "")
	lines = append(lines, fmt.Sprintf("- Technical verdict: `%s`", runReport.TechnicalVerdict))
	lines = append(lines, fmt.Sprintf("- Operator recommendation: `%s`", runReport.OperatorRecommendation))
	lines = append(lines, fmt.Sprintf("- Exit code: `%d`", runReport.ExitCode))
	lines = append(lines, fmt.Sprintf("- Inconclusive: %s", yesNo(runReport.Inconclusive())))
	lines = append(lines, "")
	lines = append(lines, "## Run Stability", "")
	lines = append(lines, fmt.Sprintf("- Baseline repeats: %d (stable: %s)", runReport.Baseline.Summary.RunCount, yesNo(runReport.Baseline.Stable)))
	lines = append(lines, fmt.Sprintf("- Candidate repeats: %d (stable: %s)", runReport.Candidate.Summary.RunCount, yesNo(runReport.Candidate.Stable)))
	lines = append(lines, "")
	lines = append(lines, "## What Changed", "")

	if len(runReport.Findings) == 0 {
		lines = append(lines, "- No typed differences detected.")
	} else {
		for _, finding := range runReport.Findings {
			status := ""
			if finding.Suppressed {
				status = " (suppressed)"
			}
			lines = append(lines, fmt.Sprintf("- %s%s", DisplayFinding(finding), status))
		}
	}

	if runReport.Diagnostic != nil {
		lines = append(lines, "")
		lines = append(lines, "## Diagnostic", "")
		lines = append(lines, fmt.Sprintf("- Category: `%s`", runReport.Diagnostic.Category))
		lines = append(lines, fmt.Sprintf("- Code: `%s`", DisplayValue(runReport.Diagnostic.Code)))
		lines = append(lines, fmt.Sprintf("- Summary: %s", DisplayValue(runReport.Diagnostic.Summary)))
		if runReport.Diagnostic.SuggestedAction != "" {
			lines = append(lines, fmt.Sprintf("- Suggested action: %s", runReport.Diagnostic.SuggestedAction))
		}
		for _, path := range runReport.Diagnostic.Evidence {
			lines = append(lines, fmt.Sprintf("- Diagnostic evidence: %s", path))
		}
	}

	lines = append(lines, "")
	lines = append(lines, "## Why This Verdict", "")
	if runReport.Diagnostic != nil {
		lines = append(lines, "- "+runReport.Diagnostic.Summary)
	} else if len(runReport.RuleHits) == 0 {
		lines = append(lines, "- No rules matched. The recommendation comes from the raw diff outcome.")
	} else {
		for _, hit := range runReport.RuleHits {
			line := fmt.Sprintf("- %s matched `%s`", hit.Category, hit.RuleID)
			if strings.TrimSpace(hit.Reason) != "" {
				line += ": " + hit.Reason
			}
			lines = append(lines, line)
		}
	}

	lines = append(lines, "")
	lines = append(lines, "## Next Step", "")
	lines = append(lines, "- "+runReport.NextStep())
	lines = append(lines, "")
	lines = append(lines, "## Evidence", "")
	lines = append(lines, fmt.Sprintf("- Root: %s", DisplayValue(runReport.Evidence.RootPath)))
	lines = append(lines, fmt.Sprintf("- Captured files: %d", len(runReport.Evidence.Files)))
	lines = append(lines, "- Output truncation: "+truncationSummary(runReport))
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." {
		return nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output directory %q: %w", dir, err)
	}

	return nil
}

func DisplayValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unavailable"
	}

	return value
}

func DisplayFinding(finding Finding) string {
	line := finding.Message
	if strings.TrimSpace(finding.Phase) != "" {
		line += fmt.Sprintf(" [%s]", finding.Phase)
	}

	return line
}

func versionSummary(requested string, installed string) string {
	requestedValue := DisplayValue(requested)
	installedValue := DisplayValue(installed)
	if installedValue == "unavailable" {
		return fmt.Sprintf("requested `%s`", requestedValue)
	}

	return fmt.Sprintf("requested `%s`, installed `%s`", requestedValue, installedValue)
}

func enabledDisabled(value bool) string {
	if value {
		return "enabled"
	}

	return "disabled"
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}

	return "no"
}

func truncationSummary(runReport Report) string {
	previewCount, evidenceCount := countTruncatedOutputs(runReport)
	if previewCount == 0 && evidenceCount == 0 {
		return "none"
	}

	parts := []string{}
	if previewCount > 0 {
		parts = append(parts, fmt.Sprintf("%d preview(s)", previewCount))
	}
	if evidenceCount > 0 {
		parts = append(parts, fmt.Sprintf("%d evidence file(s)", evidenceCount))
	}

	return strings.Join(parts, " and ") + " were truncated"
}

func countTruncatedOutputs(runReport Report) (int, int) {
	previewCount := 0
	evidenceCount := 0

	for _, side := range []Side{runReport.Baseline, runReport.Candidate} {
		for _, run := range side.Runs {
			for _, step := range run.Steps {
				if step.Stdout.Truncated {
					previewCount++
				}
				if step.Stderr.Truncated {
					previewCount++
				}
				if step.Evidence.Stdout.Truncated {
					evidenceCount++
				}
				if step.Evidence.Stderr.Truncated {
					evidenceCount++
				}
			}
		}
	}

	return previewCount, evidenceCount
}
