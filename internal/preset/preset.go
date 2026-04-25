package preset

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const prefix = "preset:"

//go:embed assets
var assets embed.FS

var pipPackagePattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?$`)

type Cleanup func()

func ResolveRules(path string) (string, Cleanup, error) {
	ref := strings.TrimSpace(path)
	if ref == "" {
		ref = "preset:default"
	}

	if !isPreset(ref) {
		return ref, nil, nil
	}

	switch presetName(ref) {
	case "default":
		return materializeFile("rules/default.yml", "rules-default-*.yml")
	default:
		return "", nil, fmt.Errorf("unsupported rules preset %q", ref)
	}
}

func ResolveScenario(path string, ecosystem string) (string, Cleanup, error) {
	ref := strings.TrimSpace(path)
	if ref == "" {
		ref = "preset:" + strings.ToLower(strings.TrimSpace(ecosystem)) + "-ci"
	}

	if !isPreset(ref) {
		return ref, nil, nil
	}

	switch presetName(ref) {
	case "cargo-ci":
		return materializeFile("scenarios/cargo-ci.yml", "scenario-cargo-ci-*.yml")
	case "npm-ci":
		return materializeFile("scenarios/npm-ci.yml", "scenario-npm-ci-*.yml")
	case "pip-ci":
		return materializeFile("scenarios/pip-ci.yml", "scenario-pip-ci-*.yml")
	default:
		return "", nil, fmt.Errorf("unsupported scenario preset %q", ref)
	}
}

func ResolveFixture(path string, ecosystem string, packageName string) (string, Cleanup, error) {
	ref := strings.TrimSpace(path)
	if ref == "" {
		ref = "preset:" + strings.ToLower(strings.TrimSpace(ecosystem)) + "-require"
	}

	if !isPreset(ref) {
		return ref, nil, nil
	}

	switch presetName(ref) {
	case "npm-require":
		return materializeDir("fixtures/npm-require")
	case "pip-require":
		return materializePipFixture(packageName)
	case "cargo-require":
		return materializeCargoFixture(packageName)
	default:
		return "", nil, fmt.Errorf("unsupported fixture preset %q; pass --fixture for custom %s projects", ref, strings.TrimSpace(ecosystem))
	}
}

func isPreset(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), prefix)
}

func presetName(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= len(prefix) {
		trimmed = trimmed[len(prefix):]
	}
	return strings.ToLower(strings.TrimSpace(trimmed))
}

func materializeFile(assetPath string, pattern string) (string, Cleanup, error) {
	data, err := assets.ReadFile(filepath.ToSlash(filepath.Join("assets", assetPath)))
	if err != nil {
		return "", nil, fmt.Errorf("read embedded preset %q: %w", assetPath, err)
	}

	file, err := os.CreateTemp("", "limier-"+pattern)
	if err != nil {
		return "", nil, fmt.Errorf("create preset temp file: %w", err)
	}
	path := file.Name()

	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", nil, fmt.Errorf("write preset temp file %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", nil, fmt.Errorf("close preset temp file %q: %w", path, err)
	}

	return path, func() { _ = os.Remove(path) }, nil
}

func materializeDir(assetDir string) (string, Cleanup, error) {
	root, err := os.MkdirTemp("", "limier-preset-*")
	if err != nil {
		return "", nil, fmt.Errorf("create preset temp directory: %w", err)
	}

	prefix := filepath.ToSlash(filepath.Join("assets", assetDir))
	if err := fs.WalkDir(assets, prefix, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == prefix {
			return nil
		}

		relative, err := filepath.Rel(prefix, path)
		if err != nil {
			return err
		}
		target := filepath.Join(root, filepath.FromSlash(relative))

		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := assets.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}); err != nil {
		_ = os.RemoveAll(root)
		return "", nil, fmt.Errorf("materialize preset directory %q: %w", assetDir, err)
	}

	return root, func() { _ = os.RemoveAll(root) }, nil
}

func materializePipFixture(packageName string) (string, Cleanup, error) {
	normalized, err := validatePackageName(packageName, "pip")
	if err != nil {
		return "", nil, err
	}

	return materializeGeneratedDir(map[string]string{
		"requirements.txt": normalized + "==0.0.0\n",
	})
}

func materializeCargoFixture(packageName string) (string, Cleanup, error) {
	normalized, err := validatePackageName(packageName, "cargo")
	if err != nil {
		return "", nil, err
	}

	manifest := strings.Join([]string{
		`[package]`,
		`name = "limier-cargo-generic-fixture"`,
		`version = "0.0.0"`,
		`edition = "2021"`,
		``,
		`[dependencies]`,
		strconv.Quote(normalized) + ` = "0.0.0"`,
		``,
	}, "\n")

	return materializeGeneratedDir(map[string]string{
		"Cargo.toml": manifest,
		filepath.Join("src", "lib.rs"): strings.Join([]string{
			"pub fn limier_fixture_ready() -> bool {",
			"    true",
			"}",
			"",
		}, "\n"),
	})
}

func materializeGeneratedDir(files map[string]string) (string, Cleanup, error) {
	root, err := os.MkdirTemp("", "limier-preset-*")
	if err != nil {
		return "", nil, fmt.Errorf("create preset temp directory: %w", err)
	}

	for relativePath, contents := range files {
		target := filepath.Join(root, relativePath)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			_ = os.RemoveAll(root)
			return "", nil, fmt.Errorf("create generated preset directory %q: %w", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, []byte(contents), 0o644); err != nil {
			_ = os.RemoveAll(root)
			return "", nil, fmt.Errorf("write generated preset file %q: %w", target, err)
		}
	}

	return root, func() { _ = os.RemoveAll(root) }, nil
}

func validatePackageName(packageName string, ecosystem string) (string, error) {
	normalized := strings.TrimSpace(packageName)
	if normalized == "" {
		return "", fmt.Errorf("package is required for %s fixture preset", ecosystem)
	}

	switch ecosystem {
	case "pip":
		if !pipPackagePattern.MatchString(normalized) {
			return "", fmt.Errorf("package %q is not supported by the pip fixture preset", packageName)
		}
	case "cargo":
		for i, r := range normalized {
			switch {
			case i == 0 && !isASCIILetter(r):
				return "", fmt.Errorf("package %q is not supported by the cargo fixture preset", packageName)
			case r >= 'a' && r <= 'z':
			case r >= 'A' && r <= 'Z':
			case r >= '0' && r <= '9':
			case r == '-' || r == '_':
			default:
				return "", fmt.Errorf("package %q is not supported by the cargo fixture preset", packageName)
			}
		}
	default:
		return "", fmt.Errorf("unsupported generated fixture ecosystem %q", ecosystem)
	}

	return normalized, nil
}

func isASCIILetter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}
