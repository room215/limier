package preset

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const prefix = "preset:"

//go:embed assets/**
var assets embed.FS

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
	case "npm-ci":
		return materializeFile("scenarios/npm-ci.yml", "scenario-npm-ci-*.yml")
	default:
		return "", nil, fmt.Errorf("unsupported scenario preset %q", ref)
	}
}

func ResolveFixture(path string, ecosystem string) (string, Cleanup, error) {
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
