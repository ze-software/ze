// Smoke tests for the //go:build ignore scripts in this directory.
//
// commands.go and doc_drift.go both use //go:build ignore so they are
// excluded from normal compilation and from golangci-lint's type-checking
// pipeline (the linter reports "build constraints exclude all Go files").
// This test file does NOT have the ignore tag, so go test sees it as the
// only file in the package and gives the linter and verify-changed a real
// target. Each test then runs the script as a subprocess via "go run" from
// the package directory and verifies it produces the expected output header.
//
// Purpose: catch regressions where a script's transitive dependencies
// (handler imports, schema imports, plugin registry) break the script
// without anyone noticing until the next manual run.

package main

import (
	"context"
	osexec "os/exec"
	"strings"
	"testing"
	"time"
)

// scriptTimeout bounds how long each smoke test will wait for a script to
// finish. The scripts load YANG modules and walk plugin registries, so a
// few seconds is normal. 60s is generous enough that even a cold run on
// virtualised CI will complete.
const scriptTimeout = 60 * time.Second

// VALIDATES: scripts/docvalid/commands.go compiles and runs end-to-end.
// PREVENTS: a //go:build ignore script silently breaking when its handler
// or schema imports are renamed or refactored. The script may exit non-zero
// (orphans are an expected baseline) but it MUST produce its header line.
func TestValidateCommandsScriptRuns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "go", "run", "commands.go")
	out, _ := cmd.CombinedOutput() //nolint:errcheck // exit code is informational; we assert on stdout
	if !strings.Contains(string(out), "Command Validation") {
		t.Fatalf("commands.go did not produce expected 'Command Validation' header:\n%s", out)
	}
}

// VALIDATES: scripts/docvalid/doc_drift.go compiles and runs end-to-end.
// PREVENTS: silent break of doc_drift via plugin registry refactor.
// The script should normally print "No documentation drift detected"; if a
// future change introduces drift, it must still run to completion and report it.
func TestDocDriftScriptRuns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "go", "run", "doc_drift.go")
	out, _ := cmd.CombinedOutput() //nolint:errcheck // exit code is informational; we assert on stdout/stderr
	s := string(out)
	if !strings.Contains(s, "documentation drift") &&
		!strings.Contains(s, "Documentation drift") &&
		!strings.Contains(s, "No documentation drift") {
		t.Fatalf("doc_drift.go did not produce expected output:\n%s", s)
	}
}
