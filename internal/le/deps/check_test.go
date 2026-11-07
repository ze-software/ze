// VALIDATES: deps check reports an import the ledger lacks for its directory
// and lists unused rows as stale.
// PREVENTS: a seeded ledger that misses a use going unnoticed.
package deps

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// writeCheckFile writes content at the checkout-relative rel under root.
func writeCheckFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestCheckReportsUncoveredImport builds a git checkout whose ledger approves
// one of two uses, and holds one row no file uses.
func TestCheckReportsUncoveredImport(t *testing.T) {
	root := t.TempDir()
	command := exec.CommandContext(t.Context(), "git", "init", "-q")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	writeCheckFile(t, root, "go.mod", "module github.com/ze-software/ze\n")
	writeCheckFile(t, root, LedgerRel, "example.org/a\tinternal/x\tin use\nexample.org/gone\tinternal/x\tno longer used\n")
	writeCheckFile(t, root, "internal/x/x.go", "package x\n\nimport _ \"example.org/a\"\n")
	writeCheckFile(t, root, "internal/y/y_test.go", "package y\n\nimport _ \"example.org/a\"\n")
	writeCheckFile(t, root, "vendor/example.org/a/a.go", "package a\n\nimport _ \"example.org/b\"\n")
	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	want := []Use{{Package: "example.org/a", Importer: "internal/y", File: "internal/y/y_test.go"}}
	if len(report.Unapproved) != 1 || report.Unapproved[0] != want[0] {
		t.Fatalf("unapproved: got %+v, want %+v", report.Unapproved, want)
	}
	if len(report.Stale) != 1 || report.Stale[0].Package != "example.org/gone" {
		t.Fatalf("stale: got %+v", report.Stale)
	}
	if report.Files != 2 {
		t.Fatalf("files judged: got %d, want 2 (vendor/ is not judged)", report.Files)
	}
}
