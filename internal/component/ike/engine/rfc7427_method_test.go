// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- RFC 7427 conformance coverage
//
// Proves RFC 7427 Section 3: once SIGNATURE_HASH_ALGORITHMS has been sent and
// received by each peer and signature authentication is to be used, the AUTH
// payload ze builds carries the Digital Signature method (14) and never a legacy
// signature method number, whatever legacy method the key would otherwise use.

package engine

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/rsa"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// TestRFC7427DigitalSignatureMethodIsUsed builds the certificate AUTH for SAs whose
// peer sent SIGNATURE_HASH_ALGORITHMS (ze always sends its own), one for each key
// type that has a legacy method of its own (RSA, and ECDSA on P-256, P-384 and
// P-521), and reads the method each yields. RFC 7427 Section 3 conditions the
// requirement on both ends having sent the notify, so the case with none received
// is not this requirement's: TestRFC7427DigitalSignatureNeedsTheNotify and the RFC
// 8247 legacy tests hold it.
//
// RFC requirement: RFC7427-3-5 positive — with SIGNATURE_HASH_ALGORITHMS received and certificate authentication configured, computeX509Auth emits an AUTH payload whose Auth Method is 14 (Digital Signature).
// RFC requirement: RFC7427-3-5 negative — with the notify received computeX509Auth never emits a legacy signature method: for an RSA key and for ECDSA keys on P-256, P-384 and P-521 the method is not RSA Digital Signature (1), DSS Digital Signature (3) or ECDSA (9, 10, 11).
func TestRFC7427DigitalSignatureMethodIsUsed(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	keys := []struct {
		name string
		key  crypto.Signer
		sent []uint16
	}{
		{"rsa", rsaKey, []uint16{hashAlgoSHA2256}},
	}
	for _, tc := range []struct {
		curve elliptic.Curve
		sent  uint16
	}{
		{elliptic.P256(), hashAlgoSHA2256},
		{elliptic.P384(), hashAlgoSHA2384},
		{elliptic.P521(), hashAlgoSHA2256},
	} {
		key, err := ecdsa.GenerateKey(tc.curve, crand.Reader)
		if err != nil {
			t.Fatalf("%s key: %v", tc.curve.Params().Name, err)
		}
		keys = append(keys, struct {
			name string
			key  crypto.Signer
			sent []uint16
		}{tc.curve.Params().Name, key, []uint16{tc.sent}})
	}

	legacy := []uint8{wire.AuthMethodRSASig, wire.AuthMethodDSSSig,
		wire.AuthMethodECDSA256, wire.AuthMethodECDSA384, wire.AuthMethodECDSA521}

	for _, tc := range keys {
		t.Run(tc.name, func(t *testing.T) {
			loadSigningKey(t, "rfc7427-method-14", tc.key)

			sa := testSAWithKeys(t)
			sa.PeerCfg.Auth.Mode = ipsec.AuthX509
			sa.PeerCfg.Auth.Certificate = "rfc7427-method-14"
			sa.RemoteHashAlgos = tc.sent

			auth, err := computeX509Auth(sa)
			if err != nil {
				t.Fatalf("computeX509Auth with the notify received: %v", err)
			}
			if slices.Contains(legacy, auth.AuthMethod) {
				t.Fatalf("AUTH method = %d is a legacy signature method; RFC 7427 Section 3 requires method 14", auth.AuthMethod)
			}
			if auth.AuthMethod != wire.AuthMethodDigitalSig {
				t.Fatalf("AUTH method = %d, want %d (Digital Signature)", auth.AuthMethod, wire.AuthMethodDigitalSig)
			}
			if len(auth.AuthData) == 0 {
				t.Fatal("AUTH payload carries no signature data")
			}
		})
	}
}
