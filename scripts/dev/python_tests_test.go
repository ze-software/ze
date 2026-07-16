package main

// Brings the Python unit tests under scripts/dev into `go test`, so
// `make ze-unit-test` actually runs them.
//
// Before this, eight *_test.py files sat here and NOTHING executed them: the repo
// has no pytest and no `unittest discover`, no make target references *_test.py,
// and the Go tests in this package cover the Go tools only. They all passed when
// run by hand, so this wires up working tests rather than repairing broken ones.
// A test nothing runs is the test-side twin of unwired code: it reads as coverage
// and provides none (ai/rules/wiring-completeness.md).
//
// Discovery is by glob rather than a hardcoded list, deliberately: a new
// <tool>_test.py is picked up with no further wiring, so this cannot rot the way
// the originals did. The empty-glob guard below is what makes that safe.
//
// Two conventions coexist here on purpose. A Python tool with an in-script
// --selftest (dep_audit.py, migrate_module.py, qemu-run.py) gets a small Go test
// that shells out to it. A tool with a standalone <tool>_test.py is covered by
// this file. Both end up inside `go test`; neither needs a make target.

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestPythonUnitTests runs every scripts/dev/*_test.py under `go test`.
//
// VALIDATES: the Python dev tooling's own unit tests execute in CI-equivalent runs
// (`make ze-unit-test` covers ./scripts/dev via `go list ./...`).
// PREVENTS: a *_test.py silently never running, which is how all eight existing
// ones ended up as dead weight that looked like coverage.
func TestPythonUnitTests(t *testing.T) {
	// go test runs with the package directory as the working directory.
	matches, err := filepath.Glob("*_test.py")
	if err != nil {
		t.Fatalf("globbing *_test.py: %v", err)
	}
	// An empty glob means the layout moved and this wiring stopped covering
	// anything. Fail loudly instead of passing vacuously, which is the exact
	// failure mode this test exists to prevent.
	if len(matches) == 0 {
		t.Fatal("no *_test.py found in scripts/dev: did the layout change? " +
			"This test must not pass vacuously.")
	}

	for _, script := range matches {
		t.Run(script, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "python3", script)
			out, err := cmd.CombinedOutput()
			code := 0
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				code = ee.ExitCode()
			} else if err != nil {
				t.Fatalf("running %s: %v", script, err)
			}
			if code != 0 {
				t.Fatalf("%s failed (exit %d):\n%s", script, code, string(out))
			}
		})
	}
}
