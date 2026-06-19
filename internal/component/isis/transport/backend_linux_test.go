//go:build linux

// Design: plan/spec-isis-3-l2-transport.md -- Linux AF_PACKET backend unit tests
//
// These cover the platform backend's pure helpers (no privileged socket). The
// real raw send/receive on a veth pair is in transport_integration_linux_test.go
// (build tag `integration && linux`), run under QEMU.

package transport

import "testing"

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
	var _ Backend = NewBackend()
}
