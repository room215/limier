package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyTreeCopiesRootEvenWhenRootNameWouldNormallyBeSkipped(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	src := filepath.Join(parent, "node_modules")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	fixtureFile := filepath.Join(src, "package.json")
	if err := os.WriteFile(fixtureFile, []byte("{\"name\":\"fixture\"}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	dst := filepath.Join(t.TempDir(), "workspace")
	if err := CopyTree(src, dst); err != nil {
		t.Fatalf("CopyTree() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dst, "package.json"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "{\"name\":\"fixture\"}\n" {
		t.Fatalf("copied file = %q, want fixture contents", string(data))
	}
}
