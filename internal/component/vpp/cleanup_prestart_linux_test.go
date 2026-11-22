//go:build linux

// Design: docs/research/vpp-deployment-reference.md -- original managed child cleanup
// Related: vpp.go -- actual runOnce result when no child was started
package vpp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Exercise the real command/start producer, not an injected run-loop result.
// A canceled Start owns no child and must preserve the exact context sentinel.
func TestVPPManagerCleanupRunOnceCanceledBeforeStart(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	manager := newCleanupOutcomeManager(t)
	manager.settings.External = false
	manager.vppBinary = executable
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err = manager.runOnce(ctx, filepath.Join(t.TempDir(), "startup.conf"))
	if err != ctx.Err() { //nolint:errorlint // runOnce returns ctx.Err itself for a clean pre-start cancellation (vpp.go); errors.Is would also pass a wrapped startup cause.
		t.Fatalf("no-child cancellation lost its exact cause: %v", err)
	}
	if manager.IsConnected() || manager.ready {
		t.Fatal("canceled command published a managed connection or readiness")
	}
}

// An ordinary refused Start remains a real startup error and has no native
// child cleanup failure to classify. Cancellation handling must not erase it.
func TestVPPManagerCleanupRunOnceRefusedStartKeepsCause(t *testing.T) {
	manager := newCleanupOutcomeManager(t)
	manager.settings.External = false
	manager.vppBinary = filepath.Join(t.TempDir(), "missing-vpp")
	err := manager.runOnce(t.Context(), filepath.Join(t.TempDir(), "startup.conf"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ordinary no-child startup refusal lost its cause: %v", err)
	}
	if _, ok := errors.AsType[nativeCleanupError](err); ok {
		t.Fatalf("ordinary refused Start became a native cleanup failure: %v", err)
	}
}
