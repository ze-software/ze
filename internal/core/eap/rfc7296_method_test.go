// VALIDATES: eap.NewSession accepts EAP-MSCHAPv2, a method that generates a shared key,
// and the exchange it starts yields a real 64-octet MSK; EAP-TLS reaches its own
// constructor rather than the unsupported-method arm.
// PREVENTS: an accepted-method set narrowed until a key-deriving method an IKEv2 AUTH
// payload needs is refused as unsupported.
//
// RFC7296-2.16-5 was claimed in this file until 2026-09-01, discharged by the argument
// that no keyless EAP method could ever start. eap.NewSession accepts MD5-Challenge now,
// because RFC 3748 Section 5 obliges every EAP implementation to support Types 1-4, so
// that argument is false. Owner ruling OR-E of 2026-07-30, given during the RFC 7296
// extraction pilot (docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md), requires the row
// to be discharged by proof rather than by annotation, and both polarities now sit on
// internal/component/ike/engine/rfc7296_eap_nonkeying_auth_test.go, which drives the
// SK_pi and SK_pr AUTH construction over a real MD5-Challenge exchange.
package eap

import (
	"errors"
	"testing"
)

// TestRFC7296EAPKeyDerivingMethodsAreAccepted checks that the two methods ze offers an
// IKEv2 exchange are both reachable through eap.NewSession.
//
// The method is one call for each: EAP-MSCHAPv2 is constructed, started, driven to
// success by driveMSCHAPv2 and its MSK read, and EAP-TLS is constructed with an empty
// configuration so that the error it returns can be checked not to be
// ErrUnsupportedMethod.
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
