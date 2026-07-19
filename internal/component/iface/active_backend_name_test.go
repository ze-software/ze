// VALIDATES: ActiveBackendName reports the name of the loaded backend, is set by
// LoadBackend, and cleared by CloseBackend -- the seam the VPP static backend
// uses to confirm the active iface backend is vpp before trusting a resolved
// sw_if_index (spec-fixit-static-interface-nexthops C-4/R-7).
// PREVENTS: a resolved index being trusted as VPP when a netlink backend is
// actually active (a kernel ifindex mis-programmed as a VPP sw_if_index).

package iface

import "testing"

func TestActiveBackendNameTracksLoadAndClose(t *testing.T) {
	// Restore whatever backend state the surrounding tests expect.
	t.Cleanup(func() { _ = CloseBackend() })

	if err := CloseBackend(); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if got := ActiveBackendName(); got != "" {
		t.Fatalf("with no backend loaded: got %q, want empty", got)
	}

	const name = "test-active-backend-name"
	if err := RegisterBackend(name, func() (Backend, error) { return &fakeBackend{}, nil }); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := LoadBackend(name); err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := ActiveBackendName(); got != name {
		t.Errorf("after load: got %q, want %q", got, name)
	}

	if err := CloseBackend(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if got := ActiveBackendName(); got != "" {
		t.Errorf("after close: got %q, want empty", got)
	}
}
