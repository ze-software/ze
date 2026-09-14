package pki

import (
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestPKICodesRegistered looks up every code this package puts in a finding:
// the CA root check (doctor.go) and the configured certificate reference
// (tls.go).
//
// VALIDATES: `ze explain <code>` resolves every code the PKI surfaces report.
// PREVENTS: a code this package spells its own way, which the registry and the
// DoT/DoH check would then disagree with.
func TestPKICodesRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()

	codes := []string{
		diagnostic.CodeDoctorPKICARootMissing,
		diagnostic.CodeDoctorPKICARootExpiry,
		diagnostic.CodeDoctorTLSInvalid,
		diagnostic.CodeDoctorTLSReference,
		diagnostic.CodeDoctorTLSExpired,
	}
	for _, code := range codes {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("diagnostic code %q is emitted but not registered", code)
		}
	}
}
