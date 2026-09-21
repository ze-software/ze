// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- RFC 7427 conformance coverage
//
// Proves RFC 7427 Section 3: once SIGNATURE_HASH_ALGORITHMS has been sent and
// received by each peer and signature authentication is to be used, the AUTH
// payload ze builds carries the Digital Signature method (14) and never a legacy
// signature method number.

package engine

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// TestRFC7427DigitalSignatureMethodIsUsed builds the certificate AUTH for an SA
// whose peer sent SIGNATURE_HASH_ALGORITHMS (ze always sends its own), and for
// the same SA with nothing received, and reads the method each yields.
//
// RFC requirement: RFC7427-3-5 positive — with SIGNATURE_HASH_ALGORITHMS received and certificate authentication configured, computeX509Auth emits an AUTH payload whose Auth Method is 14 (Digital Signature).
// RFC requirement: RFC7427-3-5 negative — computeX509Auth never emits a legacy signature method: with the notify received the method is not RSA Digital Signature (1) or DSS Digital Signature (3), and with no notify received it returns no payload at all rather than falling back to one.
func TestRFC7427DigitalSignatureMethodIsUsed(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		t.Fatalf("p256 key: %v", err)
	}
	loadSigningKey(t, "rfc7427-method-14", key)

	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthX509
	sa.PeerCfg.Auth.Certificate = "rfc7427-method-14"
	sa.RemoteHashAlgos = []uint16{hashAlgoSHA2256}

	auth, err := computeX509Auth(sa)
	if err != nil {
		t.Fatalf("computeX509Auth with the notify received: %v", err)
	}
	if auth.AuthMethod != wire.AuthMethodDigitalSig {
		t.Fatalf("AUTH method = %d, want %d (Digital Signature)", auth.AuthMethod, wire.AuthMethodDigitalSig)
	}
	if auth.AuthMethod == wire.AuthMethodRSASig || auth.AuthMethod == 3 {
		t.Fatalf("AUTH method = %d is a legacy signature method; RFC 7427 Section 3 requires method 14", auth.AuthMethod)
	}
	if len(auth.AuthData) == 0 {
		t.Fatal("AUTH payload carries no signature data")
	}

	sa.RemoteHashAlgos = nil
	legacy, err := computeX509Auth(sa)
	if err == nil {
		t.Fatalf("with no notify received, computeX509Auth returned a payload with method %d; want no payload", legacy.AuthMethod)
	}
	if legacy != nil {
		t.Fatalf("with no notify received, computeX509Auth returned a payload (method %d) beside its error", legacy.AuthMethod)
	}
}
