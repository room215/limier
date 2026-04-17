package pip

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/oneslash/limier/internal/adapter"
	"github.com/oneslash/limier/internal/scenario"
)

const (
	requirementsName = "requirements.txt"
	virtualEnvDir    = ".limier-venv"
	installCommand   = "python -m venv .limier-venv && . .limier-venv/bin/activate && python -m pip install --disable-pip-version-check -r requirements.txt"
)

var requirementLinePattern = regexp.MustCompile(`^(\s*)([A-Za-z0-9._-]+)(\[[^]]+\])?(.*)$`)

type pipAdapter struct{}

func New() adapter.Adapter {
	return pipAdapter{}
}

func (pipAdapter) Ecosystem() string {
	return "pip"
}

func (pipAdapter) DefaultImage() string {
	return "python:3.12"
}

func (pipAdapter) PrepareWorkspace(_ context.Context, workspace string, spec adapter.DependencySpec) (adapter.PreparedWorkspace, error) {
	if strings.TrimSpace(spec.Package) == "" {
		return adapter.PreparedWorkspace{}, fmt.Errorf("package is required")
	}

	if strings.TrimSpace(spec.Version) == "" {
		return adapter.PreparedWorkspace{}, fmt.Errorf("version is required")
	}

	requirementsPath := filepath.Join(workspace, requirementsName)
	if err := updateRequirementVersion(requirementsPath, spec.Package, spec.Version); err != nil {
		return adapter.PreparedWorkspace{}, err
	}

	return adapter.PreparedWorkspace{
		InstallCommand: installCommand,
		Metadata: adapter.Metadata{
			ManifestPath:         requirementsName,
			InstalledVersionPath: filepath.Join(virtualEnvDir, "lib", "*", "site-packages", "*.dist-info", "METADATA"),
			InstalledVersionKind: "python_dist_info",
			AdditionalReportPairs: map[string]string{
				"virtual_env": virtualEnvDir,
			},
		},
	}, nil
}

func (pipAdapter) ResolveStep(step scenario.Step, prepared adapter.PreparedWorkspace) (adapter.StepPlan, error) {
	command := step.Command
	if step.Run == "install" {
		command = prepared.InstallCommand
		if strings.TrimSpace(command) == "" {
			command = installCommand
		}
	} else {
		command = ". .limier-venv/bin/activate && " + step.Command
	}

	return adapter.StepPlan{
		Name:    step.Name,
		Intent:  step.Run,
		Command: command,
	}, nil
}

func (pipAdapter) ReadInstalledVersion(workspace string, spec adapter.DependencySpec, _ adapter.PreparedWorkspace) (string, error) {
	sitePackagesPattern := filepath.Join(workspace, virtualEnvDir, "lib", "*", "site-packages")
	sitePackagesRoots, err := filepath.Glob(sitePackagesPattern)
	if err != nil {
		return "", fmt.Errorf("scan virtual environment %q: %w", sitePackagesPattern, err)
	}

	if len(sitePackagesRoots) == 0 {
		return "", fmt.Errorf("virtual environment metadata not found under %q", filepath.Join(workspace, virtualEnvDir))
	}

	normalizedPackage := normalizePackageName(spec.Package)
	for _, root := range sitePackagesRoots {
		distInfoPattern := filepath.Join(root, "*.dist-info", "METADATA")
		matches, err := filepath.Glob(distInfoPattern)
		if err != nil {
			return "", fmt.Errorf("scan metadata under %q: %w", root, err)
		}

		for _, match := range matches {
			name, version, err := readMetadata(match)
			if err != nil {
				return "", err
			}

			if normalizePackageName(name) == normalizedPackage {
				return version, nil
			}
		}
	}

	return "", fmt.Errorf("installed package %q not found in virtual environment", spec.Package)
}

func updateRequirementVersion(path string, dependency string, version string) error {
	if err := validateRequirementVersion(version); err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read requirements %q: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")
	replaced := false
	for index, line := range lines {
		rewritten, matched, err := rewriteRequirementLine(line, dependency, version)
		if err != nil {
			return err
		}
		if !matched {
			continue
		}

		lines[index] = rewritten
		replaced = true
	}

	if !replaced {
		return fmt.Errorf("dependency %q is not declared in %q", dependency, requirementsName)
	}

	output := strings.Join(lines, "\n")
	if !strings.HasSuffix(output, "\n") {
		output += "\n"
	}

	if err := os.WriteFile(path, []byte(output), 0o644); err != nil {
		return fmt.Errorf("write requirements %q: %w", path, err)
	}

	return nil
}

func validateRequirementVersion(version string) error {
	if strings.TrimSpace(version) == "" {
		return fmt.Errorf("version is required")
	}

	for _, r := range version {
		switch {
		case unicode.IsControl(r):
			return fmt.Errorf("version contains unsupported control character %q", r)
		case unicode.IsSpace(r):
			return fmt.Errorf("version contains unsupported whitespace character %q", r)
		}

		switch r {
		case '#', ';', '@', '\\':
			return fmt.Errorf("version contains unsupported requirements syntax character %q", r)
		}
	}

	return nil
}

func readMetadata(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read package metadata %q: %w", path, err)
	}

	var name string
	var version string
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "Name: "):
			name = strings.TrimSpace(strings.TrimPrefix(line, "Name: "))
		case strings.HasPrefix(line, "Version: "):
			version = strings.TrimSpace(strings.TrimPrefix(line, "Version: "))
		}
	}

	if name == "" || version == "" {
		return "", "", fmt.Errorf("package metadata %q is missing Name or Version", path)
	}

	return name, version, nil
}

func normalizePackageName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, ".", "-")
	value = strings.ReplaceAll(value, "_", "-")

	return value
}

func rewriteRequirementLine(line string, dependency string, version string) (string, bool, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", false, nil
	}

	matches := requirementLinePattern.FindStringSubmatch(line)
	if len(matches) == 0 {
		return "", false, nil
	}

	if normalizePackageName(matches[2]) != normalizePackageName(dependency) {
		return "", false, nil
	}

	suffix, err := requirementSuffix(matches[4], dependency)
	if err != nil {
		return "", true, err
	}

	return matches[1] + matches[2] + matches[3] + "==" + version + suffix, true, nil
}

func requirementSuffix(remainder string, dependency string) (string, error) {
	markerStart := len(remainder)
	if index := strings.Index(remainder, ";"); index >= 0 && index < markerStart {
		markerStart = index
	}
	if index := inlineCommentIndex(remainder); index >= 0 && index < markerStart {
		markerStart = index
	}

	spec := remainder[:markerStart]
	suffix := remainder[markerStart:]
	if markerStart < len(remainder) {
		for markerStart > 0 {
			if remainder[markerStart-1] != ' ' && remainder[markerStart-1] != '\t' {
				break
			}
			markerStart--
		}
		suffix = remainder[markerStart:]
	}

	trimmedSpec := strings.TrimSpace(spec)
	switch {
	case trimmedSpec == "":
		return suffix, nil
	case strings.HasPrefix(trimmedSpec, "@"):
		return "", fmt.Errorf("dependency %q in %q uses an unsupported direct reference", dependency, requirementsName)
	case hasConstraintPrefix(trimmedSpec):
		return suffix, nil
	default:
		return "", fmt.Errorf("dependency %q in %q uses unsupported requirement syntax %q", dependency, requirementsName, strings.TrimSpace(remainder))
	}
}

func hasConstraintPrefix(value string) bool {
	for _, prefix := range []string{"==", "!=", "~=", ">=", "<=", ">", "<"} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}

	return false
}

func inlineCommentIndex(value string) int {
	for index := 0; index < len(value); index++ {
		if value[index] != '#' {
			continue
		}
		if index == 0 || value[index-1] == ' ' || value[index-1] == '\t' {
			return index
		}
	}

	return -1
}
