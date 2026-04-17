package limier

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oneslash/limier/internal/adapter"
	"github.com/oneslash/limier/internal/env/docker"
	"github.com/oneslash/limier/internal/scenario"
)

func TestSideEnvUsesLimierKeys(t *testing.T) {
	t.Parallel()

	env := sideEnv(sideRequest{
		Name:        "baseline",
		PackageName: "requests",
		Version:     "2.32.0",
		Scenario: scenario.Manifest{
			Env: map[string]string{
				"KEEP": "value",
			},
		},
	}, 3)

	if got := env["LIMIER_SIDE"]; got != "baseline" {
		t.Fatalf("LIMIER_SIDE = %q, want %q", got, "baseline")
	}
	if got := env["LIMIER_RUN_INDEX"]; got != "3" {
		t.Fatalf("LIMIER_RUN_INDEX = %q, want %q", got, "3")
	}
	if got := env["LIMIER_PACKAGE"]; got != "requests" {
		t.Fatalf("LIMIER_PACKAGE = %q, want %q", got, "requests")
	}
	if got := env["LIMIER_VERSION_UNDER_TEST"]; got != "2.32.0" {
		t.Fatalf("LIMIER_VERSION_UNDER_TEST = %q, want %q", got, "2.32.0")
	}
	if got := env["KEEP"]; got != "value" {
		t.Fatalf("KEEP = %q, want %q", got, "value")
	}
	legacySideKey := "HAR" + "NESS_SIDE"
	if _, ok := env[legacySideKey]; ok {
		t.Fatalf("env = %#v, want no legacy env keys", env)
	}
}

func TestRunSideRejectsSymlinkedFixtureEntries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outsidePath := filepath.Join(root, "outside-package.json")
	if err := os.WriteFile(outsidePath, []byte("original\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	fixturePath := filepath.Join(root, "fixture")
	if err := os.MkdirAll(fixturePath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.Symlink(outsidePath, filepath.Join(fixturePath, "package.json")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	side, evidenceFiles, diagnostic := runSide(context.Background(), sideRequest{
		Name:         "baseline",
		Version:      "1.0.0",
		FixturePath:  fixturePath,
		PackageName:  "left-pad",
		Scenario:     scenario.Manifest{Repeats: 1},
		Adapter:      &fakeAdapter{},
		EvidenceRoot: filepath.Join(root, "evidence"),
	})

	if diagnostic == nil {
		t.Fatal("diagnostic = nil, want fixture symlink diagnostic")
	}
	if diagnostic.Code != "baseline_fixture_contains_symlink" {
		t.Fatalf("diagnostic.Code = %q, want %q", diagnostic.Code, "baseline_fixture_contains_symlink")
	}
	if diagnostic.Category != "validation_failure" {
		t.Fatalf("diagnostic.Category = %q, want %q", diagnostic.Category, "validation_failure")
	}
	if len(side.Runs) != 0 {
		t.Fatalf("len(side.Runs) = %d, want 0", len(side.Runs))
	}
	if len(evidenceFiles) != 0 {
		t.Fatalf("evidenceFiles = %#v, want none", evidenceFiles)
	}

	data, err := os.ReadFile(outsidePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "original\n" {
		t.Fatalf("outside file = %q, want unchanged content", string(data))
	}
}

func TestRunSideSkipsInstalledVersionReadAfterFailedRun(t *testing.T) {
	t.Parallel()

	fixturePath := t.TempDir()
	fakeDocker := writeFakeDockerBinary(t, `case "$1" in`,
		`  create|start|rm) exit 0 ;;`,
		`  exec) /bin/sh -lc "$5"; exit $? ;;`,
		`  *) exit 1 ;;`,
		`esac`,
	)

	testAdapter := &fakeAdapter{
		prepared: adapter.PreparedWorkspace{
			InstallCommand: "exit 1",
		},
		readInstalledVersionErr: os.ErrNotExist,
	}

	side, _, diagnostic := runSide(context.Background(), sideRequest{
		Name:         "candidate",
		Version:      "2.0.0",
		FixturePath:  fixturePath,
		PackageName:  "left-pad",
		EvidenceRoot: filepath.Join(t.TempDir(), "evidence"),
		Scenario: scenario.Manifest{
			Repeats: 1,
			Workdir: "/workspace",
			Success: scenario.Success{ExitCode: 0},
			Steps: []scenario.Step{
				{Name: "install dependency", Run: "install"},
			},
		},
		Adapter: testAdapter,
		Manager: docker.NewManager(fakeDocker),
	})

	if diagnostic != nil {
		t.Fatalf("diagnostic = %#v, want nil", diagnostic)
	}
	if testAdapter.readInstalledVersionCalls != 0 {
		t.Fatalf("ReadInstalledVersion() calls = %d, want 0", testAdapter.readInstalledVersionCalls)
	}
	if len(side.Runs) != 1 {
		t.Fatalf("len(side.Runs) = %d, want 1", len(side.Runs))
	}
	if side.Runs[0].ExitCode != 1 {
		t.Fatalf("side.Runs[0].ExitCode = %d, want 1", side.Runs[0].ExitCode)
	}
	if side.InstalledVersion != "" {
		t.Fatalf("side.InstalledVersion = %q, want empty", side.InstalledVersion)
	}
}

type fakeAdapter struct {
	prepared                  adapter.PreparedWorkspace
	readInstalledVersionErr   error
	readInstalledVersion      string
	readInstalledVersionCalls int
}

func (a *fakeAdapter) Ecosystem() string {
	return "fake"
}

func (a *fakeAdapter) DefaultImage() string {
	return "alpine:3.20"
}

func (a *fakeAdapter) PrepareWorkspace(context.Context, string, adapter.DependencySpec) (adapter.PreparedWorkspace, error) {
	return a.prepared, nil
}

func (a *fakeAdapter) ResolveStep(step scenario.Step, prepared adapter.PreparedWorkspace) (adapter.StepPlan, error) {
	command := step.Command
	if step.Run == "install" {
		command = prepared.InstallCommand
	}
	return adapter.StepPlan{
		Name:    step.Name,
		Intent:  step.Run,
		Command: command,
	}, nil
}

func (a *fakeAdapter) ReadInstalledVersion(string, adapter.DependencySpec, adapter.PreparedWorkspace) (string, error) {
	a.readInstalledVersionCalls++
	if a.readInstalledVersionErr != nil {
		return "", a.readInstalledVersionErr
	}
	return a.readInstalledVersion, nil
}

func writeFakeDockerBinary(t *testing.T, commands ...string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "docker")
	lines := append([]string{"#!/bin/sh"}, commands...)
	script := strings.Join(append(lines, ""), "\n")

	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return path
}
