// Test for the //go:build ignore consistency linter in this directory.
//
// consistency.go uses //go:build ignore so it is excluded from normal
// compilation and from golangci-lint's type-checking pipeline. This file does
// NOT have the ignore tag, so it is the only buildable file in the package and
// gives the linter and verify-changed a real target. It runs the tool as a
// subprocess over a fixture tree.
//
// VALIDATES: a source file the linter cannot read in full is reported, not
// silently treated as clean.
// PREVENTS: the gate reading the head of a file, finding no issue in it, and
// reporting the whole file clean.

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runConsistency runs the linter over root and returns its combined output.
//
// The tool source is read here on purpose. consistency.go carries //go:build
// ignore, so it is not an input to this test package's build, and a subprocess
// read is not an input to the test cache either. Reading it in the test process
// binds the cached result to the tool, so an edit to consistency.go cannot come
// back as a cached pass.
func runConsistency(t *testing.T, root string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	src, err := os.ReadFile("consistency.go")
	if err != nil {
		t.Fatalf("read the tool under test: %v", err)
	}
	if !strings.Contains(string(src), "func scanLines(") {
		t.Fatalf("consistency.go no longer holds scanLines; this test drives the wrong tool")
	}

	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/lint/consistency.go", root)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = repo
	out, _ := cmd.CombinedOutput() // exit 1 means findings, which is a result not a failure
	if ctx.Err() != nil {
		t.Fatalf("consistency linter did not finish: %v", ctx.Err())
	}
	return string(out)
}

// TestConsistencyReportsUnreadableFile drives a .go file holding one line above
// bufio.MaxScanTokenSize. Scan stops on it with bufio.ErrTooLong, so every
// check over the file sees nothing. The tool must say so.
func TestConsistencyReportsUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("x", 70*1024)
	body := "// Design: none\npackage fixture\n\n// c is " + long + "\nvar c = 1\n"
	if err := os.WriteFile(filepath.Join(dir, "toolong.go"), []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	out := runConsistency(t, dir)

	if !strings.Contains(out, "unreadable") {
		t.Fatalf("linter did not report the unreadable file:\n%s", out)
	}
	if !strings.Contains(out, "toolong.go") {
		t.Fatalf("linter did not name the unreadable file:\n%s", out)
	}
	if strings.Contains(out, "All consistency checks passed") {
		t.Fatalf("linter passed a file it could not read:\n%s", out)
	}
}

// TestConsistencyPassesReadableFile pins the other side: a file the linter can
// read in full and that breaks no rule produces no unreadable finding.
func TestConsistencyPassesReadableFile(t *testing.T) {
	dir := t.TempDir()
	body := "// Design: docs/architecture/core-design.md — fixture\npackage fixture\n\nvar c = 1\n"
	if err := os.WriteFile(filepath.Join(dir, "short.go"), []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	out := runConsistency(t, dir)

	if strings.Contains(out, "unreadable") {
		t.Fatalf("linter reported a readable file as unreadable:\n%s", out)
	}
}
