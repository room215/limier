package npm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateDependencyVersionRewritesRegistrySpecAndPreservesUnrelatedDeps(t *testing.T) {
	t.Parallel()

	path := writePackageManifest(t, map[string]any{
		"name": "demo",
		"dependencies": map[string]any{
			"left-pad": "^1.1.0",
			"chalk":    "^5.0.0",
		},
	})

	if err := updateDependencyVersion(path, "left-pad", "1.3.0"); err != nil {
		t.Fatalf("updateDependencyVersion() error = %v", err)
	}

	manifest := readPackageManifest(t, path)
	dependencies := manifest["dependencies"].(map[string]any)

	if got := dependencies["left-pad"].(string); got != "1.3.0" {
		t.Fatalf("left-pad version = %q, want %q", got, "1.3.0")
	}
	if got := dependencies["chalk"].(string); got != "^5.0.0" {
		t.Fatalf("chalk version = %q, want %q", got, "^5.0.0")
	}
}

func TestUpdateDependencyVersionRejectsSourceBackedSpecs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		spec string
	}{
		{name: "workspace", spec: "workspace:*"},
		{name: "file", spec: "file:../left-pad"},
		{name: "link", spec: "link:../left-pad"},
		{name: "git", spec: "git://github.com/user/left-pad.git"},
		{name: "git plus", spec: "git+ssh://github.com/user/left-pad.git"},
		{name: "github", spec: "github:user/left-pad"},
		{name: "http", spec: "http://example.com/left-pad.tgz"},
		{name: "https", spec: "https://example.com/left-pad.tgz"},
		{name: "npm alias", spec: "npm:@scope/left-pad@1.0.0"},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			path := writePackageManifest(t, map[string]any{
				"name": "demo",
				"dependencies": map[string]any{
					"left-pad": testCase.spec,
				},
			})

			err := updateDependencyVersion(path, "left-pad", "1.3.0")
			if err == nil {
				t.Fatal("updateDependencyVersion() error = nil, want error")
			}
			if !strings.Contains(err.Error(), "unsupported source-backed npm spec") {
				t.Fatalf("updateDependencyVersion() error = %q, want source-backed npm spec rejection", err)
			}
		})
	}
}

func TestUpdateDependencyVersionRejectsNonStringDependencySyntax(t *testing.T) {
	t.Parallel()

	path := writePackageManifest(t, map[string]any{
		"name": "demo",
		"dependencies": map[string]any{
			"left-pad": map[string]any{
				"version": "^1.1.0",
			},
		},
	})

	err := updateDependencyVersion(path, "left-pad", "1.3.0")
	if err == nil {
		t.Fatal("updateDependencyVersion() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported npm dependency syntax") {
		t.Fatalf("updateDependencyVersion() error = %q, want unsupported syntax error", err)
	}
}

func writePackageManifest(t *testing.T, manifest map[string]any) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), manifestName)
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent() error = %v", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return path
}

func readPackageManifest(t *testing.T, path string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	return manifest
}
