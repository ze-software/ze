// Design: docs/functional-tests.md -- readiness markers shared by fixture processes.

package fixture

import (
	"context"
	"errors"
	"os"
	"testing"
)

// TestWaitFileUsesItsNamedMarker drives the registered entry point with a
// run-relative marker, then proves a different fixture's marker cannot release it.
func TestWaitFileUsesItsNamedMarker(t *testing.T) {
	t.Chdir(t.TempDir())
	driversMu.RLock()
	driver := drivers["plugin/wait-file"]
	driversMu.RUnlock()
	if driver == nil {
		t.Fatal("wait-file is not registered")
	}
	const marker = "export.ready"
	if err := os.WriteFile(marker, []byte("ready"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := driver(t.Context(), []string{marker}); err != nil {
		t.Fatalf("named marker was not accepted: %v", err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dynamicGroupMarker02, []byte("ready"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := driver(ctx, []string{marker}); !errors.Is(err, context.Canceled) {
		t.Fatalf("another fixture's marker released the wait: %v", err)
	}
	if err := driver(t.Context(), nil); err == nil {
		t.Fatal("wait-file accepted a missing marker argument")
	}
	if err := driver(t.Context(), []string{marker, "extra"}); err == nil {
		t.Fatal("wait-file accepted an extra marker argument")
	}
}

// TestDynamicGroupWaitKeepsItsFixedMarker pins the separate no-argument
// entry point while the generic wait-file entry point accepts a named marker.
func TestDynamicGroupWaitKeepsItsFixedMarker(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(dynamicGroupMarker02, []byte("ready"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := dynamicGroupWait02(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	if err := dynamicGroupWait02(t.Context(), []string{"other.ready"}); err == nil {
		t.Fatal("dynamic-group wait accepted a marker override")
	}
}
