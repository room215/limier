package scenario

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRequiresInstallAndCommands(t *testing.T) {
	t.Parallel()

	manifest := Manifest{
		Version: 1,
		Name:    "bad",
		Repeats: 1,
		Workdir: "/workspace",
		Steps: []Step{
			{Name: "exercise", Run: "exercise"},
		},
	}

	err := manifest.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "at least one install step is required") {
		t.Fatalf("Validate() error = %q, want install step message", err)
	}

	if !strings.Contains(err.Error(), "steps[0].command is required") {
		t.Fatalf("Validate() error = %q, want command message", err)
	}
}

func TestLoadDefaultsTelemetryToRequired(t *testing.T) {
	t.Parallel()

	manifest, err := Load(writeScenario(t, `
version: 1
name: demo
steps:
  - name: install
    run: install
  - name: exercise
    run: exercise
    command: echo ok
`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if manifest.Telemetry.Mode != TelemetryModeRequired {
		t.Fatalf("Telemetry.Mode = %q, want %q", manifest.Telemetry.Mode, TelemetryModeRequired)
	}
}

func TestLoadHonorsDisabledTelemetry(t *testing.T) {
	t.Parallel()

	manifest, err := Load(writeScenario(t, `
version: 1
name: demo
telemetry:
  mode: off
steps:
  - name: install
    run: install
  - name: exercise
    run: exercise
    command: echo ok
`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if manifest.Telemetry.Mode != TelemetryModeOff {
		t.Fatalf("Telemetry.Mode = %q, want %q", manifest.Telemetry.Mode, TelemetryModeOff)
	}
}

func TestLoadRejectsUnknownTelemetryMode(t *testing.T) {
	t.Parallel()

	_, err := Load(writeScenario(t, `
version: 1
name: demo
telemetry:
  mode: sometimes
steps:
  - name: install
    run: install
`))
	if err == nil {
		t.Fatal("Load() error = nil, want telemetry validation error")
	}
	if !strings.Contains(err.Error(), "telemetry.mode must be required or off") {
		t.Fatalf("Load() error = %q, want telemetry mode message", err)
	}
}

func TestLoadRejectsDeletedEvidenceConfiguration(t *testing.T) {
	t.Parallel()

	_, err := Load(writeScenario(t, `
version: 1
name: demo
evidence:
  capture_host_signals: false
steps:
  - name: install
    run: install
`))
	if err == nil {
		t.Fatal("Load() error = nil, want deleted evidence field error")
	}
	if !strings.Contains(err.Error(), "field evidence not found") {
		t.Fatalf("Load() error = %q, want unknown evidence field message", err)
	}
}

func TestParseTelemetryModeNormalizesInput(t *testing.T) {
	t.Parallel()

	mode, err := ParseTelemetryMode(" REQUIRED ")
	if err != nil {
		t.Fatalf("ParseTelemetryMode() error = %v", err)
	}
	if mode != TelemetryModeRequired {
		t.Fatalf("ParseTelemetryMode() = %q, want %q", mode, TelemetryModeRequired)
	}
}

func TestRepositorySampleScenarioLoads(t *testing.T) {
	t.Parallel()

	manifest, err := Load("../../scenarios/npm.yml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if manifest.Name == "" {
		t.Fatal("manifest.Name = empty, want sample scenario name")
	}
	if len(manifest.Steps) < 2 {
		t.Fatalf("len(manifest.Steps) = %d, want at least install and exercise", len(manifest.Steps))
	}
}

func writeScenario(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "scenario.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(contents)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return path
}
