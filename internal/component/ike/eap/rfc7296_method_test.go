// VALIDATES: RFC 7296 Section 2.16. Ze admits only EAP methods that generate a shared key, so
// the keyless AUTH mode that the section governs stays unreachable.
// PREVENTS: a wider accepted-method set that lets a keyless EAP method start. Ze has no SK_pi
// or SK_pr AUTH path to serve such a method.
//
// Owner ruling OR-E of 2026-07-30, recorded in plan/spec-rfcgate-1b-rfc7296-pilot.md: the row
// RFC7296-2.16-5 is discharged by proof, never by annotation. The row stays gated, so a wider
// accepted set cannot pass unnoticed.
package eap

import (
	"errors"
	"testing"
)

// eapmKeyDeriving lists the EAP method types Ze offers for IKEv2. EAP-MSCHAPv2 derives an MSK
// at eap_mschapv2.go:139, and EAP-TLS derives one at eap_tls.go:232.
var eapmKeyDeriving = map[uint8]bool{
	TypeMSCHAPv2: true,
	TypeTLS:      true,
}

// RFC requirement: RFC7296-2.16-5 positive -- the rule governs EAP methods that generate no shared key.
// NewSession at eap.go:130-149 dispatches on the method type, and its default arm at
// eap.go:141-142 returns ErrUnsupportedMethod. The test sweeps all 256 type codes and asserts
// that every code outside the two key-deriving methods is refused there. A keyless method never
// starts, which keeps the SK_pi and SK_pr AUTH mode unreachable.
func TestRFC7296EAPKeylessMethodsAreRefused(t *testing.T) {
	for code := range 256 {
		methodType := uint8(code)
		if eapmKeyDeriving[methodType] {
			continue
		}
		if _, err := NewSession(methodType, MethodConfig{}); err == nil {
			t.Fatalf("EAP method type %d was accepted, but it generates no shared key", methodType)
		} else if !errors.Is(err, ErrUnsupportedMethod) {
			t.Fatalf("EAP method type %d failed with %v, want ErrUnsupportedMethod", methodType, err)
		}
	}
}

// RFC requirement: RFC7296-2.16-5 negative -- the refusal is specific, and it is not a blanket
// rejection. NewSession accepts EAP-MSCHAPv2, starts an identity conversation, and the exchange
// yields a real 64-octet MSK for the AUTH payload. NewSession also hands EAP-TLS to
// newTLSMethod at eap.go:136-140, whose empty-certificate error is not ErrUnsupportedMethod.
// Both facts show that the sweep in the positive measures the accepted set, rather than a
// constructor that fails for every input.
func TestRFC7296EAPKeyDerivingMethodsAreAccepted(t *testing.T) {
	session, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatalf("EAP-MSCHAPv2 generates a shared key and must be accepted: %v", err)
	}
	if session.Begin().Type != TypeIdentity {
		t.Fatal("the accepted method did not start an identity conversation")
	}

	completed, final := driveMSCHAPv2(t, "secret", "secret")
	if final.Code != CodeSuccess {
		t.Fatalf("EAP-MSCHAPv2 ended with code %d, want Success", final.Code)
	}
	var zero [64]byte
	if completed.MSK() == zero {
		t.Fatal("the accepted method gave an all-zero MSK, so it generates no shared key")
	}

	// EAP-TLS reaches its own constructor. An empty configuration fails on the certificate,
	// never on the method type.
	if _, err := NewSession(TypeTLS, MethodConfig{}); errors.Is(err, ErrUnsupportedMethod) {
		t.Fatal("EAP-TLS was refused as an unsupported method, but it generates a shared key")
	}
}
