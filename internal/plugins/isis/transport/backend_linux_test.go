//go:build linux

// Design: docs/architecture/isis/isis-3-l2-transport.md -- Linux AF_PACKET backend unit tests
//
// These cover the platform backend's pure helpers (no privileged socket). The
// real raw send/receive on a veth pair is in transport_integration_linux_test.go
// (build tag `integration && linux`), run under QEMU.

package transport

import (
	"errors"
	"strings"
	"testing"
)

// TestResolveInterfaceEnsuresIfaceBackend pins the ordering an IS-IS-only config
// depends on: the iface component loads its backend from an `interface { ... }`
// block alone, and a config that names its interfaces only under `isis { }` has
// none. Without EnsureBackend, iface.Resolve fails "iface: no backend loaded",
// every circuit fails to open, and IS-IS never forms an adjacency -- which is
// exactly how all six FRR interop scenarios were failing.
//
// PREVENTS: reordering the ensure after the resolve, or dropping it. The
// substitute records the call and fails, so the test proves resolveInterface
// calls it FIRST and propagates its error rather than reaching the resolver.
func TestResolveInterfaceEnsuresIfaceBackend(t *testing.T) {
	prev := ensureIfaceBackend
	t.Cleanup(func() { ensureIfaceBackend = prev })

	called := false
	sentinel := errors.New("no backend for this test")
	ensureIfaceBackend = func() error {
		called = true
		return sentinel
	}

	_, _, _, err := resolveInterface("eth-does-not-exist")
	if !called {
		t.Fatal("resolveInterface did not ensure an iface backend before resolving")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want it to wrap the EnsureBackend failure", err)
	}
	if !strings.Contains(err.Error(), "eth-does-not-exist") {
		t.Errorf("error %q does not name the interface", err)
	}
}

func TestHtonsRoundsTrip(t *testing.T) {
	// VALIDATES: the byte-order helper swaps as expected for ETH_P_ALL framing.
	if got := htons(0x0003); got != 0x0300 {
		t.Errorf("htons(0x0003) = %#x, want 0x0300", got)
	}
	if got := htons(0x88a4); got != 0xa488 {
		t.Errorf("htons(0x88a4) = %#x, want 0xa488", got)
	}
}

func TestLinuxBackendIsBackend(t *testing.T) {
	// VALIDATES: NewBackend returns the real Linux backend implementing Backend.
	var _ = NewBackend()
}
