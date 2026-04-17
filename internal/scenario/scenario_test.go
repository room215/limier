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

func TestLoadDefaultsHostSignalCaptureToTrue(t *testing.T) {
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

	if !manifest.Evidence.HostSignalsEnabled() {
		t.Fatal("HostSignalsEnabled() = false, want true")
	}
}

func TestLoadHonorsDisabledHostSignalCapture(t *testing.T) {
	t.Parallel()

	manifest, err := Load(writeScenario(t, `
version: 1
name: demo
evidence:
  capture_host_signals: false
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

	if manifest.Evidence.HostSignalsEnabled() {
		t.Fatal("HostSignalsEnabled() = true, want false")
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
