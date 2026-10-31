// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the signature methods Ze sends
//
// RFC: rfc/short/rfc8247.md -- Section 3.1, RSA Digital Signature (AUTH method 1) is MUST.
// RFC: rfc/short/rfc4754.md -- Section 7, the r || s encoding of methods 9, 10 and 11.
//
// VALIDATES: with no SIGNATURE_HASH_ALGORITHMS notify received, computeX509Auth signs
// with the legacy method that fits the key: method 1 (RSASSA-PKCS1-v1_5 over SHA-1)
// for RSA, and 9, 10 or 11 by curve for ECDSA. Each signature is checked twice, by
// Ze's own verifier and by the standard library with the hash the RFC names, so a
// signer and a verifier that drifted together cannot pass. A received notify still
// yields method 14, a key with no legacy method is refused, and GODEBUG=fips140=only
// refuses method 1 with a named error.
// PREVENTS: the refusal computeX509Auth returned for every peer without RFC 7427,
// which left a certificate-authenticated Ze unable to reach such a peer at all.

package engine

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // RFC 7296 Section 3.8 makes SHA-1 the default hash of AUTH method 1
	"crypto/sha256"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
)

// legacyAuthSA loads key as certificate name and returns an SA configured to sign
// with it and to verify its own certificate as the remote one, so Ze's verifier can
// judge the signature Ze produced. remoteHashAlgos nil means no notify arrived.
func legacyAuthSA(t *testing.T, name string, key crypto.Signer, remoteHashAlgos []uint16) *SA {
	t.Helper()
	loadSigningKey(t, name, key)
	entry := pki.GetCertificate(name)
	if entry == nil {
		t.Fatalf("certificate %q is not in the PKI store after loadSigningKey", name)
	}

	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthX509
	sa.PeerCfg.Auth.Certificate = name
	sa.PeerCfg.Auth.CACertificate = name + "-ca"
	sa.RemoteCertRaw = entry.Raw
	sa.RemoteHashAlgos = remoteHashAlgos
	return sa
}

// signedOctetsOf answers the octets computeX509Auth signs for sa.
func signedOctetsOf(t *testing.T, sa *SA) []byte {
	t.Helper()
	octets, err := computeSignedOctets(sa, sa.IsInitiator)
	if err != nil {
		t.Fatalf("computeSignedOctets: %v", err)
	}
	return octets
}

// TestRFC8247LegacySignatureWithoutNotify signs with each key type and no notify
// received, then reads the AUTH method and checks the signature both ways. The last
// case gives the RSA key a received notify and reads method 14.
//
// RFC requirement: RFC8247-3.1-1 positive -- with no SIGNATURE_HASH_ALGORITHMS notify received and an RSA certificate key, computeX509Auth emits AUTH method 1 whose data is an RSASSA-PKCS1-v1_5 signature over SHA-1 of the signed octets, accepted by verifyLegacyRSAAuth and by rsa.VerifyPKCS1v15 with crypto.SHA1.
// RFC requirement: RFC8247-3.1-1 negative -- the same RSA key with a received notify yields method 14, never method 1, and the method 1 signature fails rsa.VerifyPKCS1v15 over other signed octets.
// RFC requirement: RFC4754-7-1 positive -- with no notify received and an ECDSA key on P-256, P-384 or P-521, computeX509Auth emits AUTH method 9, 10 or 11 whose data is r followed by s, 64, 96 or 132 octets, accepted by verifyECDSAAuth and by ecdsa.Verify with SHA-256, SHA-384 or SHA-512.
func TestRFC8247LegacySignatureWithoutNotify(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}

	t.Run("rsa signs method 1 with SHA-1", func(t *testing.T) {
		sa := legacyAuthSA(t, "rfc8247-legacy-rsa", rsaKey, nil)
		auth, err := computeX509Auth(sa)
		if err != nil {
			t.Fatalf("computeX509Auth with no notify received: %v", err)
		}
		if auth.AuthMethod != wire.AuthMethodRSASig {
			t.Fatalf("AUTH method = %d, want %d (RSA Digital Signature)", auth.AuthMethod, wire.AuthMethodRSASig)
		}
		if len(auth.AuthData) != rsaKey.Size() {
			t.Fatalf("AUTH data is %d octets, want the bare %d-octet signature", len(auth.AuthData), rsaKey.Size())
		}
		octets := signedOctetsOf(t, sa)
		if err := verifyLegacyRSAAuth(sa, auth.AuthData, octets); err != nil {
			t.Fatalf("Ze's method 1 verifier refused Ze's method 1 signature: %v", err)
		}
		digest := sha1.Sum(octets) //nolint:gosec // the hash RFC 7296 Section 3.8 names for method 1
		if err := rsa.VerifyPKCS1v15(&rsaKey.PublicKey, crypto.SHA1, digest[:], auth.AuthData); err != nil {
			t.Fatalf("the signature is not RSASSA-PKCS1-v1_5 over SHA-1: %v", err)
		}
		other := sha1.Sum(append(octets, 0)) //nolint:gosec // the hash RFC 7296 Section 3.8 names for method 1
		if rsa.VerifyPKCS1v15(&rsaKey.PublicKey, crypto.SHA1, other[:], auth.AuthData) == nil {
			t.Fatal("the signature also verifies over other octets, so it does not bind the signed octets")
		}
	})

	for _, tc := range []struct {
		curve      elliptic.Curve
		method     ecdsaMethod
		authMethod uint8
		octets     int
	}{
		{elliptic.P256(), ecdsa256, wire.AuthMethodECDSA256, 64},
		{elliptic.P384(), ecdsa384, wire.AuthMethodECDSA384, 96},
		{elliptic.P521(), ecdsa521, wire.AuthMethodECDSA521, 132},
	} {
		t.Run(tc.method.name, func(t *testing.T) {
			key, err := ecdsa.GenerateKey(tc.curve, crand.Reader)
			if err != nil {
				t.Fatalf("%s key: %v", tc.curve.Params().Name, err)
			}
			sa := legacyAuthSA(t, "rfc8247-legacy-"+tc.method.name, key, nil)
			auth, err := computeX509Auth(sa)
			if err != nil {
				t.Fatalf("computeX509Auth with no notify received: %v", err)
			}
			if auth.AuthMethod != tc.authMethod {
				t.Fatalf("AUTH method = %d, want %d (%s)", auth.AuthMethod, tc.authMethod, tc.method.name)
			}
			if len(auth.AuthData) != tc.octets {
				t.Fatalf("AUTH data is %d octets, want r || s of %d", len(auth.AuthData), tc.octets)
			}
			octets := signedOctetsOf(t, sa)
			if err := verifyECDSAAuth(sa, tc.method, auth.AuthData, octets); err != nil {
				t.Fatalf("Ze's %s verifier refused Ze's %s signature: %v", tc.method.name, tc.method.name, err)
			}
			h := tc.method.newHash()
			h.Write(octets)
			half := tc.octets / 2
			r := new(big.Int).SetBytes(auth.AuthData[:half])
			s := new(big.Int).SetBytes(auth.AuthData[half:])
			if !ecdsa.Verify(&key.PublicKey, h.Sum(nil), r, s) {
				t.Fatalf("the AUTH data is not r || s of an %s signature over the signed octets", tc.method.name)
			}
		})
	}

	t.Run("a received notify still yields method 14", func(t *testing.T) {
		sa := legacyAuthSA(t, "rfc8247-legacy-notified", rsaKey, []uint16{hashAlgoSHA2256})
		auth, err := computeX509Auth(sa)
		if err != nil {
			t.Fatalf("computeX509Auth with the notify received: %v", err)
		}
		if auth.AuthMethod != wire.AuthMethodDigitalSig {
			t.Fatalf("AUTH method = %d, want %d (Digital Signature)", auth.AuthMethod, wire.AuthMethodDigitalSig)
		}
	})
}

// TestRFC8247LegacySignatureRefusesUnsupportedKey gives computeX509Auth, with no
// notify received, a key no legacy method signs with: Ed25519, and ECDSA on P-224,
// a curve RFC 4754 binds no method to. Each answer is an error naming the key, and
// no payload.
func TestRFC8247LegacySignatureRefusesUnsupportedKey(t *testing.T) {
	_, edKey, err := ed25519.GenerateKey(crand.Reader)
	if err != nil {
		t.Fatalf("ed25519 key: %v", err)
	}
	p224Key, err := ecdsa.GenerateKey(elliptic.P224(), crand.Reader)
	if err != nil {
		t.Fatalf("p224 key: %v", err)
	}

	for _, tc := range []struct {
		name  string
		key   crypto.Signer
		names string
	}{
		{"ed25519", edKey, "an Ed25519 key"},
		{"ecdsa p-224", p224Key, "P-224"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa := legacyAuthSA(t, "rfc8247-legacy-unsupported", tc.key, nil)
			auth, err := computeX509Auth(sa)
			if !errors.Is(err, errUnsupportedKey) {
				t.Fatalf("computeX509Auth returned (%v, %v), want %v", auth, err, errUnsupportedKey)
			}
			if auth != nil {
				t.Fatalf("computeX509Auth returned a payload (method %d) beside its error", auth.AuthMethod)
			}
			for _, part := range []string{tc.names, "SIGNATURE_HASH_ALGORITHMS", "Reissue the certificate"} {
				if !strings.Contains(err.Error(), part) {
					t.Fatalf("the refusal %q does not name %q", err, part)
				}
			}
		})
	}
}

// TestLegacyRSASignatureRefusedInFIPSOnlyMode answers fipsOnlyEnforced true, as
// GODEBUG=fips140=only would, and asks for a method 1 signature. Go refuses SHA-1
// signatures in that mode, so Ze refuses first with an error naming the mode and the
// fix. The same mode leaves a SHA-1 method 1 signature unverified, with the same
// cause named.
func TestLegacyRSASignatureRefusedInFIPSOnlyMode(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	sa := legacyAuthSA(t, "rfc8247-legacy-fips", rsaKey, nil)
	octets := signedOctetsOf(t, sa)
	digest := sha1.Sum(octets) //nolint:gosec // the hash RFC 7296 Section 3.8 names for method 1
	sha1Signature, err := rsa.SignPKCS1v15(crand.Reader, rsaKey, crypto.SHA1, digest[:])
	if err != nil {
		t.Fatalf("sign with SHA-1 before the mode is switched on: %v", err)
	}

	saved := fipsOnlyEnforced
	fipsOnlyEnforced = func() bool { return true }
	t.Cleanup(func() { fipsOnlyEnforced = saved })

	auth, err := computeX509Auth(sa)
	if !errors.Is(err, errLegacyRSAFIPSOnly) {
		t.Fatalf("computeX509Auth under fips140=only returned (%v, %v), want %v", auth, err, errLegacyRSAFIPSOnly)
	}
	if !strings.Contains(err.Error(), "GODEBUG=fips140=only") {
		t.Fatalf("the refusal %q does not name GODEBUG=fips140=only", err)
	}

	err = verifyLegacyRSAAuth(sa, sha1Signature, octets)
	if !errors.Is(err, errSignatureFailed) {
		t.Fatalf("verifyLegacyRSAAuth of a SHA-1 signature under fips140=only = %v, want %v", err, errSignatureFailed)
	}
	if !strings.Contains(err.Error(), "fips140=only") {
		t.Fatalf("the verify refusal %q does not name fips140=only", err)
	}
}

// TestRFC8247LegacyRSAVerifiesSHA1 hands verifyRemoteAuth, the function every
// received IKE_AUTH reaches, an RSA Digital Signature (method 1) AUTH made with
// SHA-1, the default RFC 7296 Section 3.8 names and the only hash strongSwan signs
// method 1 with, and one made with SHA-256, which the DigestInfo also names. Both
// verify; the SHA-1 one over other octets does not. The octets are those the SA
// expects from its remote party, so the dispatch on the method number is exercised
// along with verifyLegacyRSAAuth.
//
// RFC requirement: RFC8247-3.1-1 positive -- verifyRemoteAuth accepts an RSA Digital Signature (AUTH method 1) made with RSASSA-PKCS1-v1_5 over SHA-1, and one over SHA-256.
// RFC requirement: RFC8247-3.1-1 negative -- verifyRemoteAuth refuses, under AUTH method 1, that SHA-1 signature made over octets other than the ones it expects.
func TestRFC8247LegacyRSAVerifiesSHA1(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	sa := legacyAuthSA(t, "rfc8247-legacy-verify", rsaKey, nil)
	octets, err := computeSignedOctets(sa, !sa.IsInitiator)
	if err != nil {
		t.Fatalf("compute the remote party's signed octets: %v", err)
	}
	method1 := func(signature []byte) *wire.PayloadAUTH {
		return &wire.PayloadAUTH{AuthMethod: wire.AuthMethodRSASig, AuthData: signature}
	}

	sha1Digest := sha1.Sum(octets) //nolint:gosec // the hash RFC 7296 Section 3.8 names for method 1
	sha1Signature, err := rsa.SignPKCS1v15(crand.Reader, rsaKey, crypto.SHA1, sha1Digest[:])
	if err != nil {
		t.Fatalf("sign with SHA-1: %v", err)
	}
	if err := verifyRemoteAuth(sa, method1(sha1Signature)); err != nil {
		t.Fatalf("verifyRemoteAuth refused a SHA-1 method 1 signature: %v", err)
	}

	sha256Digest := sha256.Sum256(octets)
	sha256Signature, err := rsa.SignPKCS1v15(crand.Reader, rsaKey, crypto.SHA256, sha256Digest[:])
	if err != nil {
		t.Fatalf("sign with SHA-256: %v", err)
	}
	if err := verifyRemoteAuth(sa, method1(sha256Signature)); err != nil {
		t.Fatalf("verifyRemoteAuth refused a SHA-256 method 1 signature: %v", err)
	}

	otherDigest := sha1.Sum(append(octets, 0)) //nolint:gosec // the hash RFC 7296 Section 3.8 names for method 1
	otherSignature, err := rsa.SignPKCS1v15(crand.Reader, rsaKey, crypto.SHA1, otherDigest[:])
	if err != nil {
		t.Fatalf("sign other octets with SHA-1: %v", err)
	}
	if err := verifyRemoteAuth(sa, method1(otherSignature)); !errors.Is(err, errSignatureFailed) {
		t.Fatalf("verifyRemoteAuth of a signature over other octets = %v, want %v", err, errSignatureFailed)
	}
}

// TestRFC7427EmptyNotifyStillRefusesLegacyMethods gives computeX509Auth a notify
// that arrived and listed nothing, as parseHashAlgoNotify answers a notify too
// short to hold an identifier. Both ends then sent the notify, so RFC 7427 Section
// 3 ("If both ends send and receive SIGNATURE_HASH_ALGORITHMS Notify payloads, and
// signature authentication is to be used, then the authentication method specified
// in this Authentication payload MUST be used.") leaves no legacy method, and the
// empty list leaves no algorithm to pick: the answer is a refusal, never method 1.
// It is the guard SA.RemoteHashAlgos documents: nil and empty are not the same.
func TestRFC7427EmptyNotifyStillRefusesLegacyMethods(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	sa := legacyAuthSA(t, "rfc7427-empty-notify", rsaKey, parseHashAlgoNotify([]byte{0}))
	auth, err := computeX509Auth(sa)
	if !errors.Is(err, errNoMutualHashAlgo) {
		t.Fatalf("computeX509Auth with an empty received notify returned (%v, %v), want %v", auth, err, errNoMutualHashAlgo)
	}
	if auth != nil {
		t.Fatalf("computeX509Auth returned a payload (method %d) beside its error", auth.AuthMethod)
	}
}

// TestRFC4754SignerPadsRAndS asks computeX509Auth for ECDSA-256 signatures, with no
// notify received, until it has seen one with an r below 2^248 and one with an s
// below 2^248 (at most 20000 tries; each is short with probability 1/256 or
// better), so the padding RFC 4754 Section 7 requires is exercised for both
// integers rather than left to chance. A short integer must arrive as 32 octets
// with a leading zero, and the payload must still verify as r || s.
//
// RFC requirement: RFC4754-7-1 positive -- signECDSAAuth left-pads a short r, and a short s, with zeros to their full 32 octets for ECDSA-256, and the 64-octet r || s verifies with ecdsa.Verify.
func TestRFC4754SignerPadsRAndS(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		t.Fatalf("p256 key: %v", err)
	}
	sa := legacyAuthSA(t, "rfc4754-signer-padding", key, nil)
	octets := signedOctetsOf(t, sa)
	digest := sha256.Sum256(octets)

	shortR, shortS := false, false
	for range 20000 {
		auth, err := computeX509Auth(sa)
		if err != nil {
			t.Fatalf("computeX509Auth: %v", err)
		}
		if len(auth.AuthData) != 64 {
			t.Fatalf("ECDSA-256 AUTH data is %d octets, want 64", len(auth.AuthData))
		}
		r := new(big.Int).SetBytes(auth.AuthData[:32])
		s := new(big.Int).SetBytes(auth.AuthData[32:])
		rShort := r.BitLen() <= 248
		sShort := s.BitLen() <= 248
		if !rShort && !sShort {
			continue
		}
		if rShort && auth.AuthData[0] != 0 {
			t.Fatalf("r = %x is short and does not start with a zero octet", auth.AuthData[:32])
		}
		if sShort && auth.AuthData[32] != 0 {
			t.Fatalf("s = %x is short and does not start with a zero octet", auth.AuthData[32:])
		}
		if !ecdsa.Verify(&key.PublicKey, digest[:], r, s) {
			t.Fatal("the padded r || s does not verify")
		}
		shortR = shortR || rShort
		shortS = shortS || sShort
		if shortR && shortS {
			return
		}
	}
	t.Fatalf("in 20000 tries a short r was seen: %v, a short s: %v", shortR, shortS)
}
