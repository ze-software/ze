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
