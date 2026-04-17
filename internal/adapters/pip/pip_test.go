package pip

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/room215/limier/internal/adapter"
	"github.com/room215/limier/internal/scenario"
)

func TestUpdateRequirementVersionPreservesMarkersAndComments(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), requirementsName)
	contents := strings.Join([]string{
		`requests[socks] >= 2.31.0 ; python_version < "3.13"  # keep this marker`,
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := updateRequirementVersion(path, "requests", "2.32.0"); err != nil {
		t.Fatalf("updateRequirementVersion() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	got := string(data)
	want := "requests[socks]==2.32.0 ; python_version < \"3.13\"  # keep this marker\n"
	if got != want {
		t.Fatalf("updated requirements = %q, want %q", got, want)
	}
}

func TestUpdateRequirementVersionUpdatesAllMatchingLines(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), requirementsName)
	contents := strings.Join([]string{
		"requests>=2.31.0",
		"urllib3==2.2.1",
		`requests[socks] <= 2.31.0 ; python_version < "3.13"  # keep this marker`,
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := updateRequirementVersion(path, "requests", "2.32.0"); err != nil {
		t.Fatalf("updateRequirementVersion() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	want := strings.Join([]string{
		"requests==2.32.0",
		"urllib3==2.2.1",
		`requests[socks]==2.32.0 ; python_version < "3.13"  # keep this marker`,
		"",
	}, "\n")
	if got := string(data); got != want {
		t.Fatalf("updated requirements = %q, want %q", got, want)
	}
}

func TestUpdateRequirementVersionMatchesDottedEquivalentPackageName(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), requirementsName)
	if err := os.WriteFile(path, []byte("zope.interface==5.0.0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := updateRequirementVersion(path, "zope-interface", "5.1.0"); err != nil {
		t.Fatalf("updateRequirementVersion() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if got := string(data); got != "zope.interface==5.1.0\n" {
		t.Fatalf("updated requirements = %q, want %q", got, "zope.interface==5.1.0\n")
	}
}

func TestUpdateRequirementVersionRejectsDirectReference(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), requirementsName)
	if err := os.WriteFile(path, []byte("demo @ https://example.com/demo-1.0.0.tar.gz\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err := updateRequirementVersion(path, "demo", "2.0.0")
	if err == nil {
		t.Fatal("updateRequirementVersion() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "unsupported direct reference") {
		t.Fatalf("updateRequirementVersion() error = %q, want unsupported direct reference", err)
	}
}

func TestPrepareWorkspaceRejectsUnsafeVersion(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	requirementsPath := filepath.Join(workspace, requirementsName)
	original := "requests==2.31.0\n"
	if err := os.WriteFile(requirementsPath, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := pipAdapter{}.PrepareWorkspace(context.Background(), workspace, adapter.DependencySpec{
		Package: "requests",
		Version: "2.32.0\nurllib3==9.9.9",
	})
	if err == nil {
		t.Fatal("PrepareWorkspace() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported control character") {
		t.Fatalf("PrepareWorkspace() error = %q, want unsupported control character", err)
	}

	data, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got := string(data); got != original {
		t.Fatalf("requirements.txt = %q, want %q", got, original)
	}
}

func TestPrepareWorkspaceReportsLimierVirtualEnv(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	requirementsPath := filepath.Join(workspace, requirementsName)
	if err := os.WriteFile(requirementsPath, []byte("requests==2.31.0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	prepared, err := pipAdapter{}.PrepareWorkspace(context.Background(), workspace, adapter.DependencySpec{
		Package: "requests",
		Version: "2.32.0",
	})
	if err != nil {
		t.Fatalf("PrepareWorkspace() error = %v", err)
	}

	if got := prepared.ReportMetadata()["virtual_env"]; got != ".limier-venv" {
		t.Fatalf("virtual_env = %q, want %q", got, ".limier-venv")
	}
	if installCommand := prepared.InstallCommand; !strings.Contains(installCommand, ".limier-venv") {
		t.Fatalf("install_command = %q, want Limier virtual env", installCommand)
	}
}

func TestResolveStepUsesLimierVirtualEnv(t *testing.T) {
	t.Parallel()

	installStep, err := pipAdapter{}.ResolveStep(scenario.Step{
		Name: "install",
		Run:  "install",
	}, adapter.PreparedWorkspace{})
	if err != nil {
		t.Fatalf("ResolveStep() install error = %v", err)
	}
	if !strings.Contains(installStep.Command, ".limier-venv") {
		t.Fatalf("install command = %q, want Limier virtual env", installStep.Command)
	}
	legacyVirtualEnv := ".har" + "ness-venv"
	if strings.Contains(installStep.Command, legacyVirtualEnv) {
		t.Fatalf("install command = %q, want no legacy virtual env", installStep.Command)
	}

	execStep, err := pipAdapter{}.ResolveStep(scenario.Step{
		Name:    "test",
		Run:     "exec",
		Command: "pytest",
	}, adapter.PreparedWorkspace{})
	if err != nil {
		t.Fatalf("ResolveStep() exec error = %v", err)
	}
	if !strings.Contains(execStep.Command, ". .limier-venv/bin/activate && pytest") {
		t.Fatalf("exec command = %q, want Limier activation", execStep.Command)
	}
	if strings.Contains(execStep.Command, legacyVirtualEnv) {
		t.Fatalf("exec command = %q, want no legacy virtual env", execStep.Command)
	}
}

func TestReadInstalledVersionMatchesDottedEquivalentPackageName(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	metadataPath := filepath.Join(
		workspace,
		virtualEnvDir,
		"lib",
		"python3.12",
		"site-packages",
		"zope_interface-5.1.0.dist-info",
		"METADATA",
	)
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	metadata := strings.Join([]string{
		"Name: zope-interface",
		"Version: 5.1.0",
		"",
	}, "\n")
	if err := os.WriteFile(metadataPath, []byte(metadata), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	version, err := pipAdapter{}.ReadInstalledVersion(workspace, adapter.DependencySpec{
		Package: "zope.interface",
	}, adapter.PreparedWorkspace{})
	if err != nil {
		t.Fatalf("ReadInstalledVersion() error = %v", err)
	}

	if version != "5.1.0" {
		t.Fatalf("installed version = %q, want %q", version, "5.1.0")
	}
}
