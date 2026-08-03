package main

// Brings the Python module-tier migration tool (scripts/dev/migrate_module.py)
// into `go test`, so `make ze-unit-test` exercises its fixture-based selftest.
// The tool's logic and unit tests live in migrate_module.py (--selftest); this
// runs the real script and asserts a clean exit.
//
// Rule: ai/rules/architecture.md. Umbrella: plan/spec-tiers-0-umbrella.md.

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

// TestMigrateModuleSelftest runs the migration tool's isolated fixture tests:
// boundary-safe import rewrite, pluginDirs edit, the RPC-drop guard, the residual
// scan over .ci/.yang/docs/.go-comments, and the all.go set-diff helpers.
func TestMigrateModuleSelftest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", "migrate_module.py", "--selftest")
	out, err := cmd.CombinedOutput()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running migrate_module.py --selftest: %v", err)
	}
	if code != 0 {
		t.Fatalf("migrate_module.py --selftest failed (exit %d):\n%s", code, string(out))
	}
}
