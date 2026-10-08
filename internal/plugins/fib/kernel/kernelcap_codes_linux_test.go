//go:build linux

package fibkernel

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestMPLSCapabilityCodesRegistered proves this plugin's kernel capability is
// enrolled and that both codes it answers with reach `ze explain`.
//
// VALIDATES: an operator meeting doctor-mpls-unavailable or doctor-mpls-unknown
// can look the code up.
// PREVENTS: a capability that names a code the registry does not hold.
func TestMPLSCapabilityCodesRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()

	if !slices.Contains(kernelcap.Enrolled(), "mpls") {
		t.Fatal("the mpls kernel capability is not enrolled, so its codes are unreachable")
	}
	for _, code := range []string{diagnostic.CodeDoctorMPLSUnavailable, diagnostic.CodeDoctorMPLSUnknown} {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("diagnostic code %q is declared by the mpls capability but not registered", code)
		}
	}
}

// VALIDATES: the transit MTU capability is enrolled, reaches `ze explain`
// through both codes, and is a degrading capability: its absence warns and
// never refuses a start.
// PREVENTS: a stock kernel stopping an RSVP-TE router whose only loss is the
// transit MTU bound, and that loss going unreported (owner decision,
// 2026-10-08).
func TestMPLSTransitMTUCapabilityDegrades(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()

	if !slices.Contains(kernelcap.Enrolled(), "mpls-transit-mtu") {
		t.Fatal("the mpls-transit-mtu kernel capability is not enrolled")
	}
	for _, code := range []string{diagnostic.CodeDoctorMPLSTransitMTUUnenforced, diagnostic.CodeDoctorMPLSTransitMTUUnknown} {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("diagnostic code %q is declared by the mpls-transit-mtu capability but not registered", code)
		}
	}
	if transitMTUCapability.Degrades == "" {
		t.Error("the transit MTU capability refuses a start on a stock kernel; it must only degrade")
	}
	if transitMTUCapability.Kernel != "CONFIG_MPLS_IP_MTU" {
		t.Errorf("the capability names %q, want the build symbol in internal/appliance/kernelreq.go", transitMTUCapability.Kernel)
	}
}
