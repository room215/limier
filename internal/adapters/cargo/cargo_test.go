package cargo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

func TestUpdateDependencyRequirementUpdatesWorkspaceInheritedDependency(t *testing.T) {
	t.Parallel()

	path := writeCargoManifest(t, strings.TrimSpace(`
[package]
name = "demo"
version = "0.1.0"
edition = "2021"

[dependencies]
serde = { workspace = true, features = ["derive"] }

[workspace.dependencies]
serde = "1.0.217"
`)+"\n")

	if err := updateDependencyRequirement(path, "serde", "1.0.219"); err != nil {
		t.Fatalf("updateDependencyRequirement() error = %v", err)
	}

	manifest := readCargoManifest(t, path)
	dependencies := manifest["dependencies"].(map[string]any)
	serde := dependencies["serde"].(map[string]any)

	if got := serde["workspace"].(bool); !got {
		t.Fatalf("serde.workspace = %v, want true", got)
	}
	if _, ok := serde["version"]; ok {
		t.Fatalf("serde.version = %#v, want workspace-inherited dependency unchanged", serde["version"])
	}
	features := serde["features"].([]any)
	if len(features) != 1 || features[0].(string) != "derive" {
		t.Fatalf("serde.features = %#v, want derive preserved", features)
	}

	workspace := manifest["workspace"].(map[string]any)
	workspaceDependencies := workspace["dependencies"].(map[string]any)
	if got := workspaceDependencies["serde"].(string); got != "=1.0.219" {
		t.Fatalf("workspace serde version = %q, want %q", got, "=1.0.219")
	}
}

func TestUpdateDependencyRequirementRejectsDirectSourceBackedDependencies(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		specLine string
	}{
		{
			name:     "path dependency",
			specLine: `serde = { path = "../serde" }`,
		},
		{
			name:     "git dependency",
			specLine: `serde = { git = "https://github.com/serde-rs/serde", branch = "main" }`,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			path := writeCargoManifest(t, strings.TrimSpace(`
[package]
name = "demo"
version = "0.1.0"
edition = "2021"

[dependencies]
`+testCase.specLine)+"\n")

			err := updateDependencyRequirement(path, "serde", "1.0.219")
			if err == nil {
				t.Fatal("updateDependencyRequirement() error = nil, want error")
			}
			if !strings.Contains(err.Error(), "unsupported Cargo source-backed requirement") {
				t.Fatalf("updateDependencyRequirement() error = %q, want source-backed requirement rejection", err)
			}
		})
	}
}

func TestUpdateDependencyRequirementRejectsWorkspaceInheritedDependencyWithoutWorkspaceDefinition(t *testing.T) {
	t.Parallel()

	path := writeCargoManifest(t, strings.TrimSpace(`
[package]
name = "demo"
version = "0.1.0"
edition = "2021"

[dependencies]
serde = { workspace = true }
`)+"\n")

	err := updateDependencyRequirement(path, "serde", "1.0.219")
	if err == nil {
		t.Fatal("updateDependencyRequirement() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "inherited from the workspace") {
		t.Fatalf("updateDependencyRequirement() error = %q, want workspace inheritance message", err)
	}
}

func TestUpdateDependencyRequirementRejectsSourceBackedWorkspaceDependency(t *testing.T) {
	t.Parallel()

	path := writeCargoManifest(t, strings.TrimSpace(`
[package]
name = "demo"
version = "0.1.0"
edition = "2021"

[dependencies]
serde = { workspace = true }

[workspace.dependencies]
serde = { git = "https://github.com/serde-rs/serde", rev = "abc123" }
`)+"\n")

	err := updateDependencyRequirement(path, "serde", "1.0.219")
	if err == nil {
		t.Fatal("updateDependencyRequirement() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported Cargo source-backed requirement") {
		t.Fatalf("updateDependencyRequirement() error = %q, want source-backed requirement rejection", err)
	}
}

func TestDefaultInstallCommandExecutesCargoCheckBeforeMetadata(t *testing.T) {
	t.Parallel()

	command := defaultInstallCommand()

	if !strings.Contains(command, "cargo check --manifest-path Cargo.toml") {
		t.Fatalf("defaultInstallCommand() = %q, want cargo check", command)
	}
	if !strings.Contains(command, "&& cargo metadata --format-version=1 --manifest-path Cargo.toml > .limier-cargo-metadata.json") {
		t.Fatalf("defaultInstallCommand() = %q, want metadata export after cargo check", command)
	}
}

func writeCargoManifest(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), manifestName)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return path
}

func readCargoManifest(t *testing.T, path string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var manifest map[string]any
	if err := toml.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("toml.Unmarshal() error = %v", err)
	}

	return manifest
}
