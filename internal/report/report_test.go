package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/room215/limier/internal/verdict"
)

func TestWriteJSONUsesLimierVersionField(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "report.json")
	runReport := Report{
		LimierVersion: "1.2.3",
		Telemetry: Telemetry{
			Mode:    "required",
			Status:  TelemetryStatusActive,
			Sensors: []string{"process.exec"},
		},
		Diagnostic: NewDiagnostic(
			DiagnosticCategoryDocker,
			"candidate_docker_run_failed",
			"docker failed",
			"Check Docker and rerun.",
		),
	}

	if err := WriteJSON(path, runReport); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(data)
	if !strings.Contains(contents, `"limier_version": "1.2.3"`) {
		t.Fatalf("report JSON = %q, want limier_version field", contents)
	}
	if !strings.Contains(contents, `"diagnostic": {`) {
		t.Fatalf("report JSON = %q, want diagnostic field", contents)
	}
	if !strings.Contains(contents, `"telemetry": {`) || !strings.Contains(contents, `"status": "active"`) {
		t.Fatalf("report JSON = %q, want active telemetry contract", contents)
	}
	legacyVersionField := "har" + "ness_version"
	if strings.Contains(contents, legacyVersionField) {
		t.Fatalf("report JSON = %q, want no legacy version field", contents)
	}
	if strings.Contains(contents, `"error":`) {
		t.Fatalf("report JSON = %q, want no legacy error field", contents)
	}
}

func TestBuildSummaryUsesLimierBranding(t *testing.T) {
	t.Parallel()

	summary := BuildSummary(Report{
		OperatorRecommendation: verdict.RecommendationRerun,
		Telemetry: Telemetry{
			Mode:   "required",
			Status: TelemetryStatusFailed,
		},
		Diagnostic: NewDiagnostic(
			DiagnosticCategoryDocker,
			"candidate_docker_run_failed",
			"docker failed",
			"Confirm Docker is available and healthy on the runner, then rerun Limier.",
		),
	})

	if !strings.Contains(summary, "# Limier Summary") {
		t.Fatalf("summary = %q, want Limier heading", summary)
	}
	if !strings.Contains(summary, "Confirm Docker is available and healthy on the runner, then rerun Limier.") {
		t.Fatalf("summary = %q, want Limier rerun guidance", summary)
	}
	if !strings.Contains(summary, "## Diagnostic") {
		t.Fatalf("summary = %q, want diagnostic section", summary)
	}
	if !strings.Contains(summary, "Telemetry status: `failed`") {
		t.Fatalf("summary = %q, want telemetry status", summary)
	}
	legacyHeading := "Har" + "ness Summary"
	if strings.Contains(summary, legacyHeading) {
		t.Fatalf("summary = %q, want no legacy heading", summary)
	}
}
