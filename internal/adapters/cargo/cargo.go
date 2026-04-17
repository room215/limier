package cargo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oneslash/limier/internal/adapter"
	"github.com/oneslash/limier/internal/scenario"
	toml "github.com/pelletier/go-toml/v2"
)

const (
	manifestName          = "Cargo.toml"
	lockfileName          = "Cargo.lock"
	installedVersionFile  = ".limier-cargo-metadata.json"
	defaultContainerImage = "rust:1"
)

var dependencySections = []string{
	"dependencies",
	"dev-dependencies",
	"build-dependencies",
}

type cargoAdapter struct{}

type cargoMetadataDocument struct {
	Packages         []cargoMetadataPackage `json:"packages"`
	WorkspaceMembers []string               `json:"workspace_members"`
	Resolve          cargoMetadataResolve   `json:"resolve"`
}

type cargoMetadataResolve struct {
	Root  string              `json:"root"`
	Nodes []cargoMetadataNode `json:"nodes"`
}

type cargoMetadataNode struct {
	ID   string             `json:"id"`
	Deps []cargoMetadataDep `json:"deps"`
}

type cargoMetadataDep struct {
	Name string `json:"name"`
	Pkg  string `json:"pkg"`
}

type cargoMetadataPackage struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

type updateStatus struct {
	updated           bool
	deferredWorkspace bool
}

type rewriteResult struct {
	value             any
	updated           bool
	deferredWorkspace bool
}

func New() adapter.Adapter {
	return cargoAdapter{}
}

func (cargoAdapter) Ecosystem() string {
	return "cargo"
}

func (cargoAdapter) DefaultImage() string {
	return defaultContainerImage
}

func (cargoAdapter) PrepareWorkspace(_ context.Context, workspace string, spec adapter.DependencySpec) (adapter.PreparedWorkspace, error) {
	if strings.TrimSpace(spec.Package) == "" {
		return adapter.PreparedWorkspace{}, fmt.Errorf("package is required")
	}

	if strings.TrimSpace(spec.Version) == "" {
		return adapter.PreparedWorkspace{}, fmt.Errorf("version is required")
	}

	manifestPath := filepath.Join(workspace, manifestName)
	if err := updateDependencyRequirement(manifestPath, spec.Package, spec.Version); err != nil {
		return adapter.PreparedWorkspace{}, err
	}

	installCommand := defaultInstallCommand()

	return adapter.PreparedWorkspace{
		InstallCommand: installCommand,
		Metadata: adapter.Metadata{
			ManifestPath:         manifestName,
			LockfilePath:         lockfileName,
			InstalledVersionPath: installedVersionFile,
			InstalledVersionKind: "cargo_metadata",
		},
	}, nil
}

func (cargoAdapter) ResolveStep(step scenario.Step, prepared adapter.PreparedWorkspace) (adapter.StepPlan, error) {
	command := step.Command
	if step.Run == "install" {
		command = prepared.InstallCommand
		if strings.TrimSpace(command) == "" {
			command = defaultInstallCommand()
		}
	}

	return adapter.StepPlan{
		Name:    step.Name,
		Intent:  step.Run,
		Command: command,
	}, nil
}

func (cargoAdapter) ReadInstalledVersion(workspace string, spec adapter.DependencySpec, prepared adapter.PreparedWorkspace) (string, error) {
	metadataPath := prepared.Metadata.InstalledVersionPath
	if strings.TrimSpace(metadataPath) == "" {
		metadataPath = installedVersionFile
	}

	data, err := os.ReadFile(filepath.Join(workspace, metadataPath))
	if err != nil {
		return "", fmt.Errorf("read Cargo metadata %q: %w", filepath.Join(workspace, metadataPath), err)
	}

	var document cargoMetadataDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return "", fmt.Errorf("parse Cargo metadata %q: %w", filepath.Join(workspace, metadataPath), err)
	}

	version, err := resolveInstalledVersion(document, spec.Package)
	if err != nil {
		return "", err
	}

	return version, nil
}

func updateDependencyRequirement(manifestPath string, dependency string, version string) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest %q: %w", manifestPath, err)
	}

	var manifest map[string]any
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parse manifest %q: %w", manifestPath, err)
	}

	status, err := updateDependencyTables(manifest, dependency, exactVersion(version))
	if err != nil {
		return err
	}
	if status.updated {
		output, err := toml.Marshal(manifest)
		if err != nil {
			return fmt.Errorf("encode manifest %q: %w", manifestPath, err)
		}

		if err := os.WriteFile(manifestPath, output, 0o644); err != nil {
			return fmt.Errorf("write manifest %q: %w", manifestPath, err)
		}

		return nil
	}

	if status.deferredWorkspace {
		return fmt.Errorf("dependency %q is inherited from the workspace, but no matching updatable entry was found in [workspace.dependencies]", dependency)
	}

	return fmt.Errorf("dependency %q is not declared in Cargo dependency tables", dependency)
}

func updateDependencyTables(node map[string]any, dependency string, version string) (updateStatus, error) {
	status := updateStatus{}

	for _, section := range dependencySections {
		rawTable, ok := node[section]
		if !ok {
			continue
		}

		sectionStatus, err := updateDependencyTable(rawTable, dependency, version)
		if err != nil {
			return updateStatus{}, err
		}
		status.updated = status.updated || sectionStatus.updated
		status.deferredWorkspace = status.deferredWorkspace || sectionStatus.deferredWorkspace
	}

	rawWorkspace, ok := node["workspace"]
	if ok {
		workspace, ok := rawWorkspace.(map[string]any)
		if !ok {
			return updateStatus{}, fmt.Errorf("workspace table in %q uses unsupported TOML shape", manifestName)
		}

		workspaceStatus, err := updateDependencyTables(workspace, dependency, version)
		if err != nil {
			return updateStatus{}, err
		}
		status.updated = status.updated || workspaceStatus.updated
		status.deferredWorkspace = status.deferredWorkspace || workspaceStatus.deferredWorkspace
	}

	rawTarget, ok := node["target"]
	if ok {
		targets, ok := rawTarget.(map[string]any)
		if !ok {
			return updateStatus{}, fmt.Errorf("target table in %q uses unsupported TOML shape", manifestName)
		}

		for targetName, rawTargetTable := range targets {
			targetTable, ok := rawTargetTable.(map[string]any)
			if !ok {
				return updateStatus{}, fmt.Errorf("target %q in %q uses unsupported TOML shape", targetName, manifestName)
			}

			targetStatus, err := updateDependencyTables(targetTable, dependency, version)
			if err != nil {
				return updateStatus{}, err
			}
			status.updated = status.updated || targetStatus.updated
			status.deferredWorkspace = status.deferredWorkspace || targetStatus.deferredWorkspace
		}
	}

	return status, nil
}

func updateDependencyTable(rawTable any, dependency string, version string) (updateStatus, error) {
	table, ok := rawTable.(map[string]any)
	if !ok {
		return updateStatus{}, fmt.Errorf("dependency table in %q uses unsupported TOML shape", manifestName)
	}

	status := updateStatus{}
	for key, rawValue := range table {
		if !matchesDependency(key, rawValue, dependency) {
			continue
		}

		result, err := rewriteDependencyValue(rawValue, dependency, version)
		if err != nil {
			return updateStatus{}, err
		}
		table[key] = result.value
		status.updated = status.updated || result.updated
		status.deferredWorkspace = status.deferredWorkspace || result.deferredWorkspace
	}

	return status, nil
}

func matchesDependency(key string, rawValue any, dependency string) bool {
	if normalizeDependencyName(key) == normalizeDependencyName(dependency) {
		return true
	}

	value, ok := rawValue.(map[string]any)
	if !ok {
		return false
	}

	pkgName, ok := value["package"].(string)
	return ok && normalizeDependencyName(pkgName) == normalizeDependencyName(dependency)
}

func rewriteDependencyValue(rawValue any, dependency string, version string) (rewriteResult, error) {
	switch value := rawValue.(type) {
	case string:
		return rewriteResult{
			value:   version,
			updated: true,
		}, nil
	case map[string]any:
		if sourceKey, ok := unsupportedSourceKey(value); ok {
			return rewriteResult{}, fmt.Errorf("dependency %q in %q uses unsupported Cargo source-backed requirement via %q", dependency, manifestName, sourceKey)
		}
		if usesWorkspaceVersion(value) {
			return rewriteResult{
				value:             value,
				deferredWorkspace: true,
			}, nil
		}
		value["version"] = version
		return rewriteResult{
			value:   value,
			updated: true,
		}, nil
	default:
		return rewriteResult{}, fmt.Errorf("dependency %q in %q uses unsupported Cargo requirement syntax", dependency, manifestName)
	}
}

func unsupportedSourceKey(value map[string]any) (string, bool) {
	for _, key := range []string{"path", "git", "branch", "tag", "rev"} {
		rawValue, ok := value[key]
		if !ok {
			continue
		}
		switch typed := rawValue.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return key, true
			}
		default:
			return key, true
		}
	}

	return "", false
}

func usesWorkspaceVersion(value map[string]any) bool {
	rawWorkspace, ok := value["workspace"]
	if !ok {
		return false
	}

	workspace, ok := rawWorkspace.(bool)
	return ok && workspace
}

func resolveInstalledVersion(document cargoMetadataDocument, dependency string) (string, error) {
	packageByID := make(map[string]cargoMetadataPackage, len(document.Packages))
	for _, pkg := range document.Packages {
		packageByID[pkg.ID] = pkg
	}

	rootID := strings.TrimSpace(document.Resolve.Root)
	if rootID == "" && len(document.WorkspaceMembers) == 1 {
		rootID = document.WorkspaceMembers[0]
	}

	if rootID != "" {
		nodeByID := make(map[string]cargoMetadataNode, len(document.Resolve.Nodes))
		for _, node := range document.Resolve.Nodes {
			nodeByID[node.ID] = node
		}

		rootNode, ok := nodeByID[rootID]
		if ok {
			for _, dep := range rootNode.Deps {
				if normalizeDependencyName(dep.Name) != normalizeDependencyName(dependency) {
					continue
				}

				pkg, ok := packageByID[dep.Pkg]
				if !ok || strings.TrimSpace(pkg.Version) == "" {
					break
				}

				return pkg.Version, nil
			}
		}
	}

	var matchedVersions []string
	for _, pkg := range document.Packages {
		if normalizeDependencyName(pkg.Name) != normalizeDependencyName(dependency) {
			continue
		}
		matchedVersions = append(matchedVersions, pkg.Version)
	}

	switch len(matchedVersions) {
	case 1:
		return matchedVersions[0], nil
	case 0:
		return "", fmt.Errorf("installed Cargo package %q not found in metadata", dependency)
	default:
		return "", fmt.Errorf("installed Cargo package %q resolved to multiple versions in metadata; root dependency lookup was ambiguous", dependency)
	}
}

func exactVersion(version string) string {
	version = strings.TrimSpace(version)
	if strings.HasPrefix(version, "=") {
		return version
	}

	return "=" + version
}

func normalizeDependencyName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func defaultInstallCommand() string {
	return fmt.Sprintf("cargo check --manifest-path %s && cargo metadata --format-version=1 --manifest-path %s > %s", manifestName, manifestName, installedVersionFile)
}
