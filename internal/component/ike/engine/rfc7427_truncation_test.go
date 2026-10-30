// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- IKE_AUTH authentication methods
// Related: auth.go -- verifyRemoteAuth, verifyX509Auth, hashFromAlgID, verifySignature
// Related: rfc7427_sighash_test.go -- loadSigningKey
//
// VALIDATES: a Digital Signature AUTH payload whose hash is longer than the signer's
// curve order is verified over the ANSI X9.62:2005 truncation of the digest, the
// leftmost bits up to the bit length of the order, and over no other cut.
// METHOD: a P-256 peer names ecdsa-with-SHA384, a 384-bit hash over a 256-bit order.
// The test builds the X9.62 truncation itself, signs it with the peer's key, and hands
// the AUTH payload to verifyRemoteAuth, the function Ze runs on a received IKE_AUTH.
// PREVENTS: a verifier that cuts the digest from the other end, or that hashes the
// signed octets with a function other than the one the AlgorithmIdentifier names, and
// so refuses every peer whose key is shorter than its hash.
package engine

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/sha512"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
)

// x962Truncate answers the leftmost orderBits bits of digest, the hash truncation ANSI
// X9.62:2005 specifies for a hash longer than the curve order. It is written here
// rather than borrowed from the code under test, so the test does not grade Ze's
// truncation with Ze's own.
func x962Truncate(digest []byte, orderBits int) []byte {
	if len(digest)*8 <= orderBits {
		return digest
	}
	octets := (orderBits + 7) / 8
	out := make([]byte, octets)
	copy(out, digest[:octets])
	// A bit length that is not a whole number of octets keeps the leftmost bits, so the
	// value moves right by the spare bits rather than losing its low ones.
	if spare := octets*8 - orderBits; spare > 0 {
		var carry byte
		for i := range out {
			next := out[i] << (8 - spare)
			out[i] = out[i]>>spare | carry
			carry = next
		}
	}
	return out
}

// RFC requirement: RFC7427-3-1 positive -- "For hash truncation, the method specified in
// ANSI X9.62:2005 [X9.62] MUST be used." (RFC 7427 Section 3). A P-256 peer signs the
// leftmost 256 bits of the SHA-384 digest of its signed octets and names
// ecdsa-with-SHA384, and verifyRemoteAuth accepts that AUTH payload.
//
// RFC requirement: RFC7427-3-1 negative -- the same key signs the rightmost 256 bits of
// the same digest, a cut X9.62 does not make, and verifyRemoteAuth refuses that AUTH
// payload.
func TestRFC7427ECDSAHashTruncationIsX962(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		t.Fatalf("p256 key: %v", err)
	}
	const name = "rfc7427-truncation"
	loadSigningKey(t, name, key)
	entry := pki.GetCertificate(name)
	if entry == nil {
		t.Fatalf("certificate %q not in the PKI store", name)
	}

	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthX509
	sa.PeerCfg.Auth.CACertificate = name + "-ca"
	sa.RemoteCertRaw = entry.Raw

	// The octets the responder signs, as verifyRemoteAuth computes them for the peer.
	signedOctets, err := computeSignedOctets(sa, !sa.IsInitiator)
	if err != nil {
		t.Fatalf("computeSignedOctets: %v", err)
	}
	full := sha512.Sum384(signedOctets)
	orderBits := key.Curve.Params().N.BitLen()
	if len(full)*8 <= orderBits {
		t.Fatalf("SHA-384 is %d bits and the P-256 order %d: no truncation to test",
			len(full)*8, orderBits)
	}

	authWith := func(t *testing.T, digest []byte) *wire.PayloadAUTH {
		t.Helper()
		sig, err := ecdsa.SignASN1(crand.Reader, key, digest)
		if err != nil {
			t.Fatalf("ecdsa.SignASN1: %v", err)
		}
		data := make([]byte, 0, 1+len(algIDECDSASHA384)+len(sig))
		data = append(data, byte(len(algIDECDSASHA384)))
		data = append(data, algIDECDSASHA384...)
		data = append(data, sig...)
		return &wire.PayloadAUTH{AuthMethod: wire.AuthMethodDigitalSig, AuthData: data}
	}

	t.Run("leftmost bits, the X9.62 truncation, verify", func(t *testing.T) {
		if err := verifyRemoteAuth(sa, authWith(t, x962Truncate(full[:], orderBits))); err != nil {
			t.Fatalf("verifyRemoteAuth refused a signature over the X9.62 truncation: %v", err)
		}
	})

	t.Run("rightmost bits, another truncation, are refused", func(t *testing.T) {
		rightmost := full[len(full)-orderBits/8:]
		if err := verifyRemoteAuth(sa, authWith(t, rightmost)); err == nil {
			t.Fatal("verifyRemoteAuth accepted a signature over the rightmost bits of the digest")
		}
	})
}
