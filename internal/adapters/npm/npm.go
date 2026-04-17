package npm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/room215/limier/internal/adapter"
	"github.com/room215/limier/internal/scenario"
)

const (
	manifestName = "package.json"
	installCmd   = "npm install"
)

type npmAdapter struct{}

type installedPackage struct {
	Version string `json:"version"`
}

func New() adapter.Adapter {
	return npmAdapter{}
}

func (npmAdapter) Ecosystem() string {
	return "npm"
}

func (npmAdapter) DefaultImage() string {
	return "node:22"
}

func (npmAdapter) PrepareWorkspace(_ context.Context, workspace string, spec adapter.DependencySpec) (adapter.PreparedWorkspace, error) {
	if strings.TrimSpace(spec.Package) == "" {
		return adapter.PreparedWorkspace{}, fmt.Errorf("package is required")
	}

	if strings.TrimSpace(spec.Version) == "" {
		return adapter.PreparedWorkspace{}, fmt.Errorf("version is required")
	}

	manifestPath := filepath.Join(workspace, manifestName)
	if err := updateDependencyVersion(manifestPath, spec.Package, spec.Version); err != nil {
		return adapter.PreparedWorkspace{}, err
	}

	return adapter.PreparedWorkspace{
		InstallCommand: installCmd,
		Metadata: adapter.Metadata{
			ManifestPath:         manifestName,
			InstalledVersionPath: filepath.Join("node_modules", filepath.FromSlash(spec.Package), manifestName),
			InstalledVersionKind: "package_manifest",
		},
	}, nil
}

func (npmAdapter) ResolveStep(step scenario.Step, prepared adapter.PreparedWorkspace) (adapter.StepPlan, error) {
	command := step.Command
	if step.Run == "install" {
		command = prepared.InstallCommand
		if strings.TrimSpace(command) == "" {
			command = installCmd
		}
	}

	return adapter.StepPlan{
		Name:    step.Name,
		Intent:  step.Run,
		Command: command,
	}, nil
}

func (npmAdapter) ReadInstalledVersion(workspace string, spec adapter.DependencySpec, _ adapter.PreparedWorkspace) (string, error) {
	packagePath := filepath.Join(workspace, "node_modules", filepath.FromSlash(spec.Package), manifestName)
	data, err := os.ReadFile(packagePath)
	if err != nil {
		return "", fmt.Errorf("read installed dependency manifest %q: %w", packagePath, err)
	}

	var pkg installedPackage
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("parse installed dependency manifest %q: %w", packagePath, err)
	}

	if strings.TrimSpace(pkg.Version) == "" {
		return "", fmt.Errorf("installed dependency manifest %q is missing version", packagePath)
	}

	return pkg.Version, nil
}

func readManifest(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("fixture must contain %q", manifestName)
		}

		return nil, fmt.Errorf("read manifest %q: %w", path, err)
	}

	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest %q: %w", path, err)
	}

	return manifest, nil
}

func dependencySections(manifest map[string]any, dependency string) []string {
	var sections []string
	for _, section := range []string{"dependencies", "devDependencies"} {
		rawDeps, ok := manifest[section]
		if !ok {
			continue
		}

		deps, ok := rawDeps.(map[string]any)
		if !ok {
			continue
		}

		if _, ok := deps[dependency]; ok {
			sections = append(sections, section)
		}
	}

	return sections
}

func updateDependencyVersion(manifestPath string, dependency string, version string) error {
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return err
	}

	sections := dependencySections(manifest, dependency)
	if len(sections) == 0 {
		return fmt.Errorf("dependency %q is not declared in dependencies or devDependencies", dependency)
	}

	for _, section := range sections {
		deps := manifest[section].(map[string]any)
		if err := validateSupportedDependencySpec(deps[dependency], dependency); err != nil {
			return err
		}
		deps[dependency] = version
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest %q: %w", manifestPath, err)
	}

	data = append(data, '\n')
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("write manifest %q: %w", manifestPath, err)
	}

	return nil
}

func validateSupportedDependencySpec(rawValue any, dependency string) error {
	spec, ok := rawValue.(string)
	if !ok {
		return fmt.Errorf("dependency %q in %q uses unsupported npm dependency syntax", dependency, manifestName)
	}

	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return fmt.Errorf("dependency %q in %q uses unsupported npm dependency syntax", dependency, manifestName)
	}

	if usesUnsupportedSourceSpec(trimmed) {
		return fmt.Errorf("dependency %q in %q uses unsupported source-backed npm spec %q", dependency, manifestName, trimmed)
	}

	return nil
}

func usesUnsupportedSourceSpec(spec string) bool {
	lower := strings.ToLower(strings.TrimSpace(spec))
	for _, prefix := range []string{
		"workspace:",
		"file:",
		"link:",
		"git:",
		"git+",
		"github:",
		"http:",
		"https:",
		"npm:",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}

	return false
}
