//go:build ze_le

package reactor

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"
)

// VALIDATES: the ze_le read-panic seam fires once per trigger file, for the
// named peer only, and removes the file before it panics.
// PREVENTS: an interop scenario (bgp-panic-cease) whose trigger never fires, or
// fires on every UPDATE after the first, which would turn one contained panic
// into a reconnect loop the scenario could not tell from a defect.
func TestInjectReadPanicFiresOncePerTrigger(t *testing.T) {
	directory := t.TempDir()
	saved := readPanicTriggerDir
	readPanicTriggerDir = directory
	t.Cleanup(func() { readPanicTriggerDir = saved })

	target := netip.MustParseAddr("192.0.2.1")
	other := netip.MustParseAddr("192.0.2.2")
	trigger := filepath.Join(directory, readPanicTriggerPrefix+target.String())

	if panicked(func() { injectReadPanic(target) }) {
		t.Fatal("no trigger file exists, yet the seam panicked")
	}
	if err := os.WriteFile(trigger, nil, 0o600); err != nil {
		t.Fatalf("write the trigger file: %v", err)
	}
	if panicked(func() { injectReadPanic(other) }) {
		t.Fatalf("the trigger names %s, yet the seam panicked for %s", target, other)
	}
	if !panicked(func() { injectReadPanic(target) }) {
		t.Fatalf("the trigger file %s exists, yet the seam did not panic", trigger)
	}
	if _, err := os.Stat(trigger); !os.IsNotExist(err) {
		t.Fatalf("the trigger file must be removed before the panic, stat answered %v", err)
	}
	if panicked(func() { injectReadPanic(target) }) {
		t.Fatal("one trigger file raised a second panic")
	}
}

// panicked answers whether fn panicked.
func panicked(fn func()) (raised bool) {
	defer func() {
		if recover() != nil {
			raised = true
		}
	}()
	fn()
	return false
}
