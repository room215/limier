package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oneslash/limier/internal/collector"
	"github.com/oneslash/limier/internal/report"
	"github.com/oneslash/limier/internal/rules"
	"github.com/oneslash/limier/internal/verdict"
	"gopkg.in/yaml.v3"
)

type corpusCase struct {
	Name             string     `yaml:"name"`
	Description      string     `yaml:"description"`
	Rules            string     `yaml:"rules"`
	ExpectedExitCode int        `yaml:"expected_exit_code"`
	Want             corpusWant `yaml:"want"`
	Baseline         corpusSide `yaml:"baseline"`
	Candidate        corpusSide `yaml:"candidate"`
}

type corpusWant struct {
	TechnicalVerdict       verdict.Technical      `yaml:"technical_verdict"`
	OperatorRecommendation verdict.Recommendation `yaml:"operator_recommendation"`
	FindingKinds           []string               `yaml:"finding_kinds"`
	RuleCategories         []string               `yaml:"rule_categories"`
	DiagnosticCode         string                 `yaml:"diagnostic_code"`
}

type corpusSide struct {
	Stable bool        `yaml:"stable"`
	Runs   []corpusRun `yaml:"runs"`
}

type corpusRun struct {
	ExitCode int           `yaml:"exit_code"`
	Steps    []corpusStep  `yaml:"steps"`
	Events   []corpusEvent `yaml:"events"`
}

type corpusStep struct {
	Name    string `yaml:"name"`
	Intent  string `yaml:"intent"`
	Command string `yaml:"command"`
	Stdout  string `yaml:"stdout"`
	Stderr  string `yaml:"stderr"`
}

type corpusEvent struct {
	Kind    string `yaml:"kind"`
	Step    string `yaml:"step"`
	Command string `yaml:"command"`
}

func TestPhaseSevenValidationCorpus(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	paths, err := filepath.Glob(filepath.Join(repoRoot, "validation", "corpus", "*.yml"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("validation corpus = empty, want repository-owned cases")
	}

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			testCase := loadCorpusCase(t, path)
			ruleSet, err := rules.Load(filepath.Join(repoRoot, testCase.Rules))
			if err != nil {
				t.Fatalf("rules.Load(%q) error = %v", testCase.Rules, err)
			}

			result := Evaluate(
				buildCorpusSide(filepath.Base(path), "baseline", testCase.Baseline),
				buildCorpusSide(filepath.Base(path), "candidate", testCase.Candidate),
				testCase.ExpectedExitCode,
				ruleSet,
			)

			if result.TechnicalVerdict != testCase.Want.TechnicalVerdict {
				t.Fatalf("technical verdict = %q, want %q", result.TechnicalVerdict, testCase.Want.TechnicalVerdict)
			}
			if result.OperatorRecommendation != testCase.Want.OperatorRecommendation {
				t.Fatalf("operator recommendation = %q, want %q", result.OperatorRecommendation, testCase.Want.OperatorRecommendation)
			}

			requireFindingKinds(t, result.Findings, testCase.Want.FindingKinds)
			requireRuleCategories(t, result.RuleHits, testCase.Want.RuleCategories)

			if testCase.Want.DiagnosticCode != "" {
				if result.Diagnostic == nil {
					t.Fatalf("diagnostic = nil, want code %q", testCase.Want.DiagnosticCode)
				}
				if result.Diagnostic.Code != testCase.Want.DiagnosticCode {
					t.Fatalf("diagnostic code = %q, want %q", result.Diagnostic.Code, testCase.Want.DiagnosticCode)
				}
			}
		})
	}
}

func loadCorpusCase(t *testing.T, path string) corpusCase {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var testCase corpusCase
	if err := yaml.Unmarshal(data, &testCase); err != nil {
		t.Fatalf("yaml.Unmarshal(%q) error = %v", path, err)
	}

	return testCase
}

func buildCorpusSide(caseName string, sideName string, side corpusSide) report.Side {
	runs := make([]report.Run, 0, len(side.Runs))
	for runIndex, run := range side.Runs {
		steps := make([]report.Step, 0, len(run.Steps))
		for _, step := range run.Steps {
			steps = append(steps, report.Step{
				Name:     step.Name,
				Intent:   step.Intent,
				Command:  step.Command,
				ExitCode: run.ExitCode,
				Stdout:   output(step.Stdout),
				Stderr:   output(step.Stderr),
				Evidence: report.StepEvidence{
					Stdout: report.OutputFile{Path: filepath.Join("/tmp", caseName, sideName, "stdout.txt")},
					Stderr: report.OutputFile{Path: filepath.Join("/tmp", caseName, sideName, "stderr.txt")},
				},
			})
		}

		events := make([]collector.Event, 0, len(run.Events))
		for _, event := range run.Events {
			events = append(events, collector.Event{
				Kind:    event.Kind,
				Step:    event.Step,
				Command: event.Command,
			})
		}

		runs = append(runs, report.Run{
			RunIndex:   runIndex + 1,
			ExitCode:   run.ExitCode,
			Steps:      steps,
			Events:     events,
			EventsPath: filepath.Join("/tmp", caseName, sideName, "events.json"),
		})
	}

	summary, _ := AssessSide(runs)

	return report.Side{
		Stable:  side.Stable,
		Summary: summary,
		Runs:    runs,
	}
}

func output(preview string) report.Output {
	return report.Output{
		Preview:    preview,
		TotalBytes: int64(len(preview)),
		SHA256:     preview,
	}
}

func requireFindingKinds(t *testing.T, findings []report.Finding, want []string) {
	t.Helper()

	if len(want) != len(findings) {
		t.Fatalf("finding count = %d, want %d (%v)", len(findings), len(want), want)
	}

	for index, kind := range want {
		if findings[index].Kind != kind {
			t.Fatalf("finding[%d].Kind = %q, want %q", index, findings[index].Kind, kind)
		}
	}
}

func requireRuleCategories(t *testing.T, hits []report.RuleHit, want []string) {
	t.Helper()

	if len(want) != len(hits) {
		t.Fatalf("rule hit count = %d, want %d (%v)", len(hits), len(want), want)
	}

	for index, category := range want {
		if hits[index].Category != category {
			t.Fatalf("ruleHits[%d].Category = %q, want %q", index, hits[index].Category, category)
		}
	}
}
