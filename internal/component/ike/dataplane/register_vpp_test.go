// VALIDATES: in a ze_vpp build the "vpp" dataplane backend is registered:
// Load("vpp") gets past the registry lookup and fails only on the missing VPP
// connector (ErrNotSupported), never with "not registered".
// PREVENTS: the gated register_vpp.go dropping out of the build while the tag
// is set, which would silently turn every `dataplane vpp` config into an
// unknown-backend error.
//
//go:build ze_vpp

package dataplane

import (
	"errors"
	"strings"
	"testing"
)

func TestVPPBackendRegistered(t *testing.T) {
	err := Load("vpp")
	if err == nil {
		// A live VPP connector in the test environment would make Load
		// succeed; that still proves registration.
		return
	}
	if strings.Contains(err.Error(), "not registered") {
		t.Fatalf("ze_vpp build: vpp backend not registered: %v", err)
	}
	if !errors.Is(err, ErrNotSupported) && !strings.Contains(err.Error(), "vpp") {
		t.Fatalf("Load(vpp) failed for an unexpected reason: %v", err)
	}
}
