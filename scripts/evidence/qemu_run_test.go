package main

// Brings the QEMU harness (scripts/evidence/qemu-run.py) into `go test`, so
// `make ze-unit-test` exercises its fixture-based selftest. The tool's logic and
// unit tests live in qemu-run.py (--selftest); this runs the real script and
// asserts a clean exit. Same shape as scripts/dev/migrate_module_test.go.
//
// The selftest needs neither QEMU nor a download, so it runs on any host.
//
// Rule: ai/rules/qemu-testing.md. Spec: plan/learned/1173-relocate-scratch-and-cache.md.

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

// TestQemuRunSelftest runs the ISO-cache fixture tests.
//
// VALIDATES: an extracted Alpine initramfs is keyed to the ISO it came from, so a
// cached extract is reused for the same ISO and never for a different one.
// PREVENTS: the stale hit where bumping ALPINE_VERSION downloads a new ISO and
// boots it with the previous version's initramfs, because the extract directory
// was a fixed name and the cache hit tested only that initramfs-virt existed.
func TestQemuRunSelftest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", "qemu-run.py", "--selftest")
	out, err := cmd.CombinedOutput()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running qemu-run.py --selftest: %v", err)
	}
	if code != 0 {
		t.Fatalf("qemu-run.py --selftest failed (exit %d):\n%s", code, string(out))
	}
}
