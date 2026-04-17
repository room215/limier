package render

import (
	"fmt"
	"strings"

	"github.com/room215/limier/internal/report"
	"github.com/room215/limier/internal/verdict"
)

const (
	FormatGitHubComment = "github-comment"
	FormatGitLabNote    = "gitlab-note"
	FormatBuildSummary  = "build-summary"
)

func Render(runReport report.Report, format string) (string, error) {
	switch format {
	case FormatGitHubComment:
		return renderSurface(runReport, "## Limier Dependency Review", 5), nil
	case FormatGitLabNote:
		return renderSurface(runReport, "### Limier Dependency Review", 5), nil
	case FormatBuildSummary:
		return renderSurface(runReport, "# Limier Build Summary", 8), nil
	default:
		return "", fmt.Errorf("unsupported render format %q", format)
	}
}

func Inspect(runReport report.Report) string {
	var lines []string

	lines = append(lines, "# Limier Inspect", "")
	lines = append(lines, fmt.Sprintf("- Package: %s", runReport.ComparisonLine()))
	lines = append(lines, fmt.Sprintf("- Technical verdict: `%s`", runReport.TechnicalVerdict))
	lines = append(lines, fmt.Sprintf("- Operator recommendation: `%s`", runReport.OperatorRecommendation))
	lines = append(lines, fmt.Sprintf("- Exit code: `%d`", runReport.ExitCode))
	lines = append(lines, fmt.Sprintf("- Baseline stability: %d repeat(s), stable `%t`", runReport.Baseline.Summary.RunCount, runReport.Baseline.Stable))
	lines = append(lines, fmt.Sprintf("- Candidate stability: %d repeat(s), stable `%t`", runReport.Candidate.Summary.RunCount, runReport.Candidate.Stable))

	if runReport.Diagnostic == nil {
		lines = append(lines, "- Diagnostic status: no structured diagnostic; the report is conclusive.")
		lines = append(lines, "")
		lines = append(lines, "## What To Inspect", "")
		lines = append(lines, fmt.Sprintf("- Review the %d finding(s) and supporting evidence under `%s`.", len(runReport.Findings), report.DisplayValue(runReport.Evidence.RootPath)))
		lines = append(lines, "")
		return strings.Join(lines, "\n")
	}

	lines = append(lines, "- Diagnostic status: structured inconclusive diagnostic available.")
	lines = append(lines, "")
	lines = append(lines, "## Diagnostic", "")
	lines = append(lines, fmt.Sprintf("- Category: `%s`", runReport.Diagnostic.Category))
	lines = append(lines, fmt.Sprintf("- Code: `%s`", report.DisplayValue(runReport.Diagnostic.Code)))
	lines = append(lines, fmt.Sprintf("- Summary: %s", report.DisplayValue(runReport.Diagnostic.Summary)))
	if runReport.Diagnostic.SuggestedAction != "" {
		lines = append(lines, fmt.Sprintf("- Suggested action: %s", runReport.Diagnostic.SuggestedAction))
	}
	if len(runReport.Diagnostic.Evidence) > 0 {
		for _, path := range runReport.Diagnostic.Evidence {
			lines = append(lines, fmt.Sprintf("- Diagnostic evidence: %s", path))
		}
	} else {
		lines = append(lines, fmt.Sprintf("- Evidence root: %s", report.DisplayValue(runReport.Evidence.RootPath)))
	}
	lines = append(lines, "")
	lines = append(lines, "## What To Inspect", "")
	lines = append(lines, fmt.Sprintf("- Open the evidence under `%s` and start with the diagnostic summary above.", report.DisplayValue(runReport.Evidence.RootPath)))
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

func renderSurface(runReport report.Report, title string, findingLimit int) string {
	var lines []string

	lines = append(lines, title, "")
	lines = append(lines, fmt.Sprintf("- Comparison: %s", runReport.ComparisonLine()))
	lines = append(lines, fmt.Sprintf("- Technical verdict: `%s`", runReport.TechnicalVerdict))
	lines = append(lines, fmt.Sprintf("- Operator recommendation: `%s`", runReport.OperatorRecommendation))
	lines = append(lines, fmt.Sprintf("- Exit code: `%d`", runReport.ExitCode))
	lines = append(lines, fmt.Sprintf("- Baseline stability: %d repeat(s), stable `%t`", runReport.Baseline.Summary.RunCount, runReport.Baseline.Stable))
	lines = append(lines, fmt.Sprintf("- Candidate stability: %d repeat(s), stable `%t`", runReport.Candidate.Summary.RunCount, runReport.Candidate.Stable))
	lines = append(lines, fmt.Sprintf("- Evidence root: `%s`", report.DisplayValue(runReport.Evidence.RootPath)))
	lines = append(lines, "")
	lines = append(lines, "### What Changed", "")

	if len(runReport.Findings) == 0 {
		lines = append(lines, "- No typed differences detected.")
	} else {
		for _, finding := range limitedFindings(runReport.Findings, findingLimit) {
			lines = append(lines, "- "+findingLine(finding))
		}
		if len(runReport.Findings) > findingLimit {
			lines = append(lines, fmt.Sprintf("- %d more finding(s) are available in `report.json`.", len(runReport.Findings)-findingLimit))
		}
	}

	lines = append(lines, "")
	lines = append(lines, "### Why", "")
	if runReport.Diagnostic != nil {
		lines = append(lines, fmt.Sprintf("- Inconclusive diagnostic: %s", runReport.Diagnostic.Summary))
		lines = append(lines, fmt.Sprintf("- Diagnostic category: `%s`", runReport.Diagnostic.Category))
		if runReport.Diagnostic.SuggestedAction != "" {
			lines = append(lines, fmt.Sprintf("- Suggested action: %s", runReport.Diagnostic.SuggestedAction))
		}
	} else if len(runReport.RuleHits) == 0 {
		lines = append(lines, "- No rules matched. This output mirrors the report-level verdict and recommendation.")
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
	lines = append(lines, "### Next Step", "")
	lines = append(lines, fmt.Sprintf("- %s", nextStep(runReport)))
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

func findingLine(finding report.Finding) string {
	line := report.DisplayFinding(finding)
	if finding.Suppressed {
		line += " (suppressed)"
	}
	return line
}

func limitedFindings(findings []report.Finding, limit int) []report.Finding {
	if len(findings) <= limit {
		return findings
	}
	return findings[:limit]
}

func nextStep(runReport report.Report) string {
	if runReport.Diagnostic != nil && runReport.Diagnostic.SuggestedAction != "" {
		return runReport.Diagnostic.SuggestedAction
	}

	switch runReport.OperatorRecommendation {
	case verdict.RecommendationGoodToGo:
		return "Approve the upgrade if the current ruleset matches your policy."
	case verdict.RecommendationBlock:
		return "Keep the upgrade blocked until the suspicious behavior is explained or removed."
	case verdict.RecommendationNeedsReview:
		return "Review the findings and evidence before approving the upgrade."
	default:
		return "Rerun after fixing the underlying Limier, runner, or fixture issue."
	}
}
