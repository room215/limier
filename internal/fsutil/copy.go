package fsutil

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

var ErrSymlinkUnsupported = errors.New("symlinked fixture entries are unsupported")

type SymlinkError struct {
	Path string
}

func (e *SymlinkError) Error() string {
	if e == nil {
		return ErrSymlinkUnsupported.Error()
	}
	if e.Path == "" {
		return ErrSymlinkUnsupported.Error()
	}
	return fmt.Sprintf("%s: %s", ErrSymlinkUnsupported, e.Path)
}

func (e *SymlinkError) Unwrap() error {
	return ErrSymlinkUnsupported
}

func CopyTree(src string, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("compute relative path for %q: %w", path, err)
		}

		if rel != "." && entry.IsDir() && shouldSkip(entry.Name()) {
			return filepath.SkipDir
		}

		if rel == "." {
			info, err := entry.Info()
			if err != nil {
				return err
			}

			return os.MkdirAll(dst, info.Mode())
		}

		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}

		if entry.Type()&os.ModeSymlink != 0 {
			return &SymlinkError{Path: path}
		}

		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src string, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create directory for %q: %w", dst, err)
	}

	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file %q: %w", src, err)
	}
	defer source.Close()

	target, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create target file %q: %w", dst, err)
	}
	defer target.Close()

	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("copy %q to %q: %w", src, dst, err)
	}

	return nil
}

func shouldSkip(name string) bool {
	switch name {
	case ".git", "node_modules", ".venv", ".limier-venv", "__pycache__":
		return true
	default:
		return false
	}
}
