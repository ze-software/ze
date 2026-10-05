// Design: docs/architecture/storage-backends.md -- unsupported secure storage refuses.

//go:build !linux && !darwin && !freebsd

package zefs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestUnsupportedOpenDirectory proves that both opening and creation refuse
// before a filesystem operation, leaving missing ancestors absent.
func TestUnsupportedOpenDirectory(t *testing.T) {
	base := t.TempDir()
	missing := filepath.Join(base, "missing", "child")
	for _, path := range []string{base, missing} {
		for _, create := range []bool{false, true} {
			file, err := OpenDirectory(path, create)
			if file != nil {
				file.Close() //nolint:errcheck // Cleanup after a contract violation.
				t.Fatalf("unsupported open returned a descriptor for %q", path)
			}
			if !errors.Is(err, errFrameTreeUnsupported) {
				t.Fatalf("OpenDirectory(%q, %v) = %v", path, create, err)
			}
		}
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("unsupported open created paths: %v", entries)
	}
}

// TestUnsupportedRenameNoReplace proves refusal for existing and absent targets,
// preserving both names and bytes; nil parents are not dereferenced either.
func TestUnsupportedRenameNoReplace(t *testing.T) {
	base := t.TempDir()
	for _, name := range []string{"source", "target"} {
		if err := os.WriteFile(filepath.Join(base, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	parent, err := os.Open(base)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close() //nolint:errcheck // Test cleanup.
	for _, target := range []string{"target", "missing"} {
		if err := RenameNoReplace(parent, "source", target); !errors.Is(err, errFrameTreeUnsupported) {
			t.Fatalf("RenameNoReplace(%q) = %v", target, err)
		}
	}
	if err := RenameNoReplace(nil, "source", "target"); !errors.Is(err, errFrameTreeUnsupported) {
		t.Fatalf("nil-parent rename = %v", err)
	}
	for _, name := range []string{"source", "target"} {
		got, err := os.ReadFile(filepath.Join(base, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != name {
			t.Errorf("%s contents changed to %q", name, got)
		}
	}
	if _, err := os.Lstat(filepath.Join(base, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unsupported rename created target: %v", err)
	}
}

// TestUnsupportedFrameTreeOperations retains the same refusal for integrity and
// repair, without invoking the visitor or creating a destination.
func TestUnsupportedFrameTreeOperations(t *testing.T) {
	base := t.TempDir()
	if err := walkFrameTree(base, func(string, []byte) error {
		t.Error("unsupported walk invoked visitor")
		return nil
	}); !errors.Is(err, errFrameTreeUnsupported) {
		t.Fatalf("walk = %v", err)
	}
	destination := filepath.Join(base, "repair")
	report, err := repairFrameTree(base, destination)
	if report != nil || !errors.Is(err, errFrameTreeUnsupported) {
		t.Fatalf("repair = %v, %v", report, err)
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unsupported repair created destination: %v", err)
	}
}
