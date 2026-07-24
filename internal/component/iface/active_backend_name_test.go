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

// TestEnsureBackend covers the helper OSPF (and other config-less consumers) use
// to get an interface backend when no interface{} block loaded one: it fails
// closed with no OS default, and is a strict no-op when a backend is already
// loaded (so an explicit interface{} backend always wins).
func TestEnsureBackend(t *testing.T) {
	t.Cleanup(func() { _ = CloseBackend() })
	if err := CloseBackend(); err != nil {
		t.Fatalf("reset: %v", err)
	}

	// No backend loaded and no OS default (e.g. darwin): fail closed, do not load.
	if defaultBackendName == "" {
		if err := EnsureBackend(); err == nil {
			t.Error("EnsureBackend must error when no backend is loaded and no default exists")
		}
		if got := ActiveBackendName(); got != "" {
			t.Errorf("EnsureBackend must not load a backend on the no-default path: got %q", got)
		}
	}

	// With a backend already loaded, EnsureBackend must NOT swap it.
	const name = "test-ensure-backend"
	if err := RegisterBackend(name, func() (Backend, error) { return &fakeBackend{}, nil }); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := LoadBackend(name); err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := EnsureBackend(); err != nil {
		t.Fatalf("EnsureBackend with a backend loaded: %v", err)
	}
	if got := ActiveBackendName(); got != name {
		t.Errorf("EnsureBackend must not swap the loaded backend: got %q, want %q", got, name)
	}
}
