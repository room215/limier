package adapters_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/room215/limier/internal/adapter"
	"github.com/room215/limier/internal/adapters"
	"github.com/room215/limier/internal/adapters/cargo"
	"github.com/room215/limier/internal/adapters/npm"
	"github.com/room215/limier/internal/adapters/pip"
	"github.com/room215/limier/internal/scenario"
)

type conformanceCase struct {
	name                 string
	adapter              adapter.Adapter
	spec                 adapter.DependencySpec
	expectedMetadata     map[string]string
	missingDependencyErr string
	writeFixture         func(t *testing.T, workspace string)
	verifyPrepared       func(t *testing.T, workspace string)
	writeInstalled       func(t *testing.T, workspace string)
	expectedInstalled    string
}

func TestSupportedIncludesRegisteredAdapters(t *testing.T) {
	t.Parallel()

	got := adapters.Supported()
	want := []string{"cargo", "npm", "pip"}

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Supported() = %v, want %v", got, want)
	}

	for _, name := range want {
		gotAdapter, err := adapters.Lookup(name)
		if err != nil {
			t.Fatalf("Lookup(%q) error = %v", name, err)
		}
		if gotAdapter.Ecosystem() != name {
			t.Fatalf("Lookup(%q).Ecosystem() = %q, want %q", name, gotAdapter.Ecosystem(), name)
		}
	}
}

func TestAdaptersConformToSharedContract(t *testing.T) {
	t.Parallel()

	cases := []conformanceCase{
		{
			name:    "npm",
			adapter: npm.New(),
			spec: adapter.DependencySpec{
				Package: "left-pad",
				Version: "1.3.0",
			},
			expectedMetadata: map[string]string{
				"manifest_path":          "package.json",
				"installed_version_kind": "package_manifest",
			},
			missingDependencyErr: `dependency "left-pad" is not declared`,
			writeFixture: func(t *testing.T, workspace string) {
				t.Helper()

				contents := map[string]any{
					"name": "demo",
					"dependencies": map[string]string{
						"left-pad": "1.1.0",
						"chalk":    "5.0.0",
					},
				}
				writeJSONFile(t, filepath.Join(workspace, "package.json"), contents)
			},
			verifyPrepared: func(t *testing.T, workspace string) {
				t.Helper()

				var manifest struct {
					Dependencies map[string]string `json:"dependencies"`
				}
				readJSONFile(t, filepath.Join(workspace, "package.json"), &manifest)

				if got := manifest.Dependencies["left-pad"]; got != "1.3.0" {
					t.Fatalf("left-pad version = %q, want %q", got, "1.3.0")
				}
				if got := manifest.Dependencies["chalk"]; got != "5.0.0" {
					t.Fatalf("chalk version = %q, want %q", got, "5.0.0")
				}
			},
			writeInstalled: func(t *testing.T, workspace string) {
				t.Helper()

				writeJSONFile(t, filepath.Join(workspace, "node_modules", "left-pad", "package.json"), map[string]string{
					"version": "1.3.0",
				})
			},
			expectedInstalled: "1.3.0",
		},
		{
			name:    "pip",
			adapter: pip.New(),
			spec: adapter.DependencySpec{
				Package: "requests",
				Version: "2.32.0",
			},
			expectedMetadata: map[string]string{
				"manifest_path":          "requirements.txt",
				"installed_version_kind": "python_dist_info",
				"virtual_env":            ".limier-venv",
			},
			missingDependencyErr: `dependency "requests" is not declared`,
			writeFixture: func(t *testing.T, workspace string) {
				t.Helper()

				contents := strings.Join([]string{
					"requests>=2.31.0",
					"urllib3==2.2.1",
					"",
				}, "\n")
				writeTextFile(t, filepath.Join(workspace, "requirements.txt"), contents)
			},
			verifyPrepared: func(t *testing.T, workspace string) {
				t.Helper()

				data, err := os.ReadFile(filepath.Join(workspace, "requirements.txt"))
				if err != nil {
					t.Fatalf("ReadFile() error = %v", err)
				}

				got := string(data)
				if !strings.Contains(got, "requests==2.32.0") {
					t.Fatalf("requirements.txt = %q, want requests pinned to 2.32.0", got)
				}
				if !strings.Contains(got, "urllib3==2.2.1") {
					t.Fatalf("requirements.txt = %q, want unrelated dependency preserved", got)
				}
			},
			writeInstalled: func(t *testing.T, workspace string) {
				t.Helper()

				metadataPath := filepath.Join(workspace, ".limier-venv", "lib", "python3.12", "site-packages", "requests-2.32.0.dist-info", "METADATA")
				writeTextFile(t, metadataPath, "Name: requests\nVersion: 2.32.0\n")
			},
			expectedInstalled: "2.32.0",
		},
		{
			name:    "cargo",
			adapter: cargo.New(),
			spec: adapter.DependencySpec{
				Package: "serde",
				Version: "1.0.219",
			},
			expectedMetadata: map[string]string{
				"manifest_path":          "Cargo.toml",
				"lockfile_path":          "Cargo.lock",
				"installed_version_path": ".limier-cargo-metadata.json",
				"installed_version_kind": "cargo_metadata",
			},
			missingDependencyErr: `dependency "serde" is not declared`,
			writeFixture: func(t *testing.T, workspace string) {
				t.Helper()

				writeTextFile(t, filepath.Join(workspace, "Cargo.toml"), strings.TrimSpace(`
[package]
name = "demo"
version = "0.1.0"
edition = "2021"

[dependencies]
serde = { version = "1.0.217", features = ["derive"] }

[dev-dependencies]
rand = "0.8.5"
`)+"\n")
			},
			verifyPrepared: func(t *testing.T, workspace string) {
				t.Helper()

				data, err := os.ReadFile(filepath.Join(workspace, "Cargo.toml"))
				if err != nil {
					t.Fatalf("ReadFile() error = %v", err)
				}

				var manifest map[string]any
				if err := toml.Unmarshal(data, &manifest); err != nil {
					t.Fatalf("toml.Unmarshal() error = %v", err)
				}

				dependencies := manifest["dependencies"].(map[string]any)
				serde := dependencies["serde"].(map[string]any)
				if got := serde["version"].(string); got != "=1.0.219" {
					t.Fatalf("serde version = %q, want %q", got, "=1.0.219")
				}

				features := serde["features"].([]any)
				if len(features) != 1 || features[0].(string) != "derive" {
					t.Fatalf("serde features = %#v, want derive preserved", features)
				}

				devDependencies := manifest["dev-dependencies"].(map[string]any)
				if got := devDependencies["rand"].(string); got != "0.8.5" {
					t.Fatalf("rand version = %q, want %q", got, "0.8.5")
				}
			},
			writeInstalled: func(t *testing.T, workspace string) {
				t.Helper()

				document := map[string]any{
					"packages": []map[string]any{
						{
							"id":      "path+file:///workspace#demo@0.1.0",
							"name":    "demo",
							"version": "0.1.0",
						},
						{
							"id":      "registry+https://github.com/rust-lang/crates.io-index#serde@1.0.219",
							"name":    "serde",
							"version": "1.0.219",
						},
					},
					"workspace_members": []string{"path+file:///workspace#demo@0.1.0"},
					"resolve": map[string]any{
						"root": "path+file:///workspace#demo@0.1.0",
						"nodes": []map[string]any{
							{
								"id": "path+file:///workspace#demo@0.1.0",
								"deps": []map[string]any{
									{
										"name": "serde",
										"pkg":  "registry+https://github.com/rust-lang/crates.io-index#serde@1.0.219",
									},
								},
							},
						},
					},
				}
				writeJSONFile(t, filepath.Join(workspace, ".limier-cargo-metadata.json"), document)
			},
			expectedInstalled: "1.0.219",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			workspace := t.TempDir()
			tc.writeFixture(t, workspace)

			prepared, err := tc.adapter.PrepareWorkspace(context.Background(), workspace, tc.spec)
			if err != nil {
				t.Fatalf("PrepareWorkspace() error = %v", err)
			}

			if strings.TrimSpace(prepared.InstallCommand) == "" {
				t.Fatal("PrepareWorkspace() returned empty install command")
			}

			metadata := prepared.ReportMetadata()
			if got := metadata["install_command"]; got != prepared.InstallCommand {
				t.Fatalf("install_command metadata = %q, want %q", got, prepared.InstallCommand)
			}
			for key, want := range tc.expectedMetadata {
				if got := metadata[key]; got != want {
					t.Fatalf("report metadata %q = %q, want %q", key, got, want)
				}
			}

			tc.verifyPrepared(t, workspace)

			installStep, err := tc.adapter.ResolveStep(scenario.Step{
				Name: "install",
				Run:  "install",
			}, prepared)
			if err != nil {
				t.Fatalf("ResolveStep(install) error = %v", err)
			}
			if installStep.Command != prepared.InstallCommand {
				t.Fatalf("install command = %q, want %q", installStep.Command, prepared.InstallCommand)
			}

			execStep, err := tc.adapter.ResolveStep(scenario.Step{
				Name:    "exercise",
				Run:     "exercise",
				Command: "echo ok",
			}, prepared)
			if err != nil {
				t.Fatalf("ResolveStep(exercise) error = %v", err)
			}
			if !strings.Contains(execStep.Command, "echo ok") {
				t.Fatalf("exercise command = %q, want original command", execStep.Command)
			}

			tc.writeInstalled(t, workspace)

			gotVersion, err := tc.adapter.ReadInstalledVersion(workspace, tc.spec, prepared)
			if err != nil {
				t.Fatalf("ReadInstalledVersion() error = %v", err)
			}
			if gotVersion != tc.expectedInstalled {
				t.Fatalf("installed version = %q, want %q", gotVersion, tc.expectedInstalled)
			}

			missingWorkspace := t.TempDir()
			tc.writeFixture(t, missingWorkspace)
			if err := removeDependencyReference(tc.name, missingWorkspace, tc.spec.Package); err != nil {
				t.Fatalf("removeDependencyReference() error = %v", err)
			}

			_, err = tc.adapter.PrepareWorkspace(context.Background(), missingWorkspace, tc.spec)
			if err == nil {
				t.Fatal("PrepareWorkspace() error = nil, want missing dependency error")
			}
			if !strings.Contains(err.Error(), tc.missingDependencyErr) {
				t.Fatalf("PrepareWorkspace() error = %q, want %q", err, tc.missingDependencyErr)
			}
		})
	}
}

func removeDependencyReference(ecosystem string, workspace string, dependency string) error {
	switch ecosystem {
	case "npm":
		var manifest map[string]any
		path := filepath.Join(workspace, "package.json")
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			return err
		}
		dependencies := manifest["dependencies"].(map[string]any)
		delete(dependencies, dependency)
		writeJSONFileNoHelper(path, manifest)
		return nil
	case "pip":
		return os.WriteFile(filepath.Join(workspace, "requirements.txt"), []byte("urllib3==2.2.1\n"), 0o644)
	case "cargo":
		return os.WriteFile(filepath.Join(workspace, "Cargo.toml"), []byte(strings.TrimSpace(`
[package]
name = "demo"
version = "0.1.0"
edition = "2021"

[dependencies]
rand = "0.8.5"
`)+"\n"), 0o644)
	default:
		return nil
	}
}

func readJSONFile(t *testing.T, path string, target any) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", path, err)
	}
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent() error = %v", err)
	}
	data = append(data, '\n')

	writeTextFile(t, path, string(data))
}

func writeJSONFileNoHelper(path string, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		panic(err)
	}
}

func writeTextFile(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
