// VALIDATES: without the ze_vpp build tag the "vpp" dataplane backend is NOT
// registered: Load("vpp") fails closed at the registry lookup with a clear
// "not registered" error (dataplane.go Load), and the xfrm backend is still
// present.
// PREVENTS: the GoVPP-backed IPsec dataplane leaking into a vpp-less build, or
// the fallback path degrading to a silent no-op instead of a rejection.
//
//go:build !ze_vpp

package dataplane

import (
	"strings"
	"testing"
)

func TestVPPBackendAbsent(t *testing.T) {
	err := Load("vpp")
	if err == nil {
		t.Fatal("non-ze_vpp build: Load(vpp) unexpectedly succeeded (backend not compiled out)")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("Load(vpp) = %v, want fail-closed 'not registered' rejection", err)
	}
}
