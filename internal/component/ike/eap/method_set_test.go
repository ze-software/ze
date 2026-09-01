// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP method dispatch
//
// NewSession's accepted-method set is closed: three method Types are built and
// every other value of the octet is refused with ErrUnsupportedMethod. The
// refusal is load-bearing beyond its own message, because TypeDerivesKey answers
// false for an unknown Type and says in its own doc comment that "NewSession
// refuses an unknown Type before a session exists, so no exchange reaches this
// with one". A default arm that let one through would turn that sentence into an
// unproven safety claim and route a keyless exchange at an MSK no method filled.
//
// The sweep is the only thing that can hold a CLOSED set. A test naming three
// accepted Types proves nothing about the 253 it does not name, and the set was
// swept until 2026-09-01, when the two RFC-tagged sweeps that did it moved their
// tags to internal/component/ike/engine and were deleted here with them. This
// file carries no tag on purpose: the two requirement ids live in the engine
// package now, and a second copy of either would be a second claim on one proof.
//
// VALIDATES: NewSession accepts exactly Types 4, 13 and 26, and refuses each of
// the other 253 values of the Type octet with an error wrapping
// ErrUnsupportedMethod.
// PREVENTS: a method Type added to the switch without a decision about the key it
// derives; a default arm that returns a session, a nil error, or an error a
// caller cannot match; and the silent unpinning of Types 5 (OTP) and 6 (GTC),
// which sit next to the accepted values and derive no key.

package eap

import (
	"errors"
	"testing"
)

// TestNewSessionAcceptsExactlyTheThreeConfiguredMethods sweeps all 256 values of
// the EAP Type octet.
//
// The accepted set is derived from the exported constants rather than written
// out, so adding a method to NewSession without adding it here fails on the new
// Type and adding it to both is one edit that states the decision once.
func TestNewSessionAcceptsExactlyTheThreeConfiguredMethods(t *testing.T) {
	accepted := map[uint8]struct{}{
		TypeMD5Challenge: {},
		TypeTLS:          {},
		TypeMSCHAPv2:     {},
	}

	for code := range 256 {
		methodType := uint8(code)

		session, err := NewSession(methodType, MethodConfig{Password: "secret"})
		if session != nil {
			session.Close()
		}

		if _, ok := accepted[methodType]; ok {
			// EAP-TLS fails on the absent certificate material rather than on its
			// Type, so the accepted arm asserts only that the refusal is not the
			// unsupported-method one.
			if errors.Is(err, ErrUnsupportedMethod) {
				t.Fatalf("type %d is a method ze runs, and NewSession refused it as unsupported", methodType)
			}
			continue
		}

		if err == nil {
			t.Fatalf("type %d is outside the accepted set and NewSession built a session for it; TypeDerivesKey answers false for it, so the exchange would sign with an MSK no method filled", methodType)
		}
		if !errors.Is(err, ErrUnsupportedMethod) {
			t.Fatalf("type %d was refused with %v, which does not wrap ErrUnsupportedMethod, so a caller cannot tell an unsupported method from a construction failure", methodType, err)
		}
	}
}
