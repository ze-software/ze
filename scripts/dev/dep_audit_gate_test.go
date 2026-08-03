package main

// Brings the Python module-tier placement gate (scripts/dev/dep_audit.py) into
// `go test`, so `make ze-unit-test` exercises it. The gate logic and its
// fixture-based unit tests live in dep_audit.py (--check / --selftest); these
// tests run the real script and assert its exit code.
//
// Rule: ai/rules/architecture.md. Spec: plan/spec-tiers-1-rule-and-audit.md.

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

// runGate runs `python3 dep_audit.py <args...>` from the package directory
// (scripts/dev); the script resolves the repo root itself via git.
func runGate(t *testing.T, args ...string) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", append([]string{"dep_audit.py"}, args...)...)
	out, err := cmd.CombinedOutput()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running dep_audit.py %v: %v", args, err)
	}
	return code, string(out)
}

// TestEnginePlacement asserts the module-tier gate is clean on the real tree:
// misplaced engines are accounted for by the migration baseline, and intentional
// non-engine placements are accounted for by the category manifest.
func TestEnginePlacement(t *testing.T) {
	code, out := runGate(t, "--check")
	if code != 0 {
		t.Fatalf("dep_audit.py --check failed (exit %d) -- a misplaced engine, "+
			"stale baseline entry, or non-engine category issue. "+
			"See ai/rules/architecture.md.\n%s", code, out)
	}
}

// TestEnginePlacementSelftest runs the gate's fixture-based unit tests:
// classification, setup-feature registration, category manifest enforcement,
// nested-namespace exclusion, and baseline new/stale/clean behavior.
func TestEnginePlacementSelftest(t *testing.T) {
	code, out := runGate(t, "--selftest")
	if code != 0 {
		t.Fatalf("dep_audit.py --selftest failed (exit %d):\n%s", code, out)
	}
}
