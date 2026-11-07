// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the signature methods Ze sends
// Related: register_signature_eddsa.go -- the Ed25519 family under the Identity hash
// Related: auth_signature_scheme.go -- the table computeX509Auth and verifyX509Auth read
//
// RFC: rfc/full/rfc8420.txt -- EdDSA in IKEv2 (not enrolled: no release profile lists
// RFC 8420, so these tests carry no requirement tag).
//
// VALIDATES: an Ed25519 certificate key signs AUTH method 14 under the Identity hash
// for a peer that announced Identity, with the RFC 8420 AlgorithmIdentifier and a pure
// Ed25519 signature over the signed octets whole; Ze announces Identity; Ze verifies
// such an AUTH and refuses it when an octet changed, the key differs, the
// AlgorithmIdentifier and the certificate key disagree, or the AlgorithmIdentifier
// carries parameters.
// PREVENTS: errUnsupportedKey for every Ed25519 certificate (the home CPE case), and an
// EdDSA path that prehashes or accepts a signature its certificate cannot have made.

package engine

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// rfc8420AlgIDEd25519 is RFC 8420 Appendix A.1 "Binary encoding: 3005 0603 2B65 70".
var rfc8420AlgIDEd25519 = []byte{0x30, 0x05, 0x06, 0x03, 0x2B, 0x65, 0x70}

// rfc8420PeerAlgos is what strongSwan announces with its openssl plugin loaded:
// SHA2-256, SHA2-384, SHA2-512 and Identity.
var rfc8420PeerAlgos = []uint16{hashAlgoSHA2256, hashAlgoSHA2384, hashAlgoSHA2512, hashAlgoIdentity}

// newEd25519Key answers a fresh Ed25519 key.
func newEd25519Key(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, key, err := ed25519.GenerateKey(crand.Reader)
	if err != nil {
		t.Fatalf("ed25519 key: %v", err)
	}
	return key
}

// digitalSigAuth frames a method 14 AUTH payload: length octet, algID, signature.
func digitalSigAuth(algID, signature []byte) *wire.PayloadAUTH {
	data := make([]byte, 0, 1+len(algID)+len(signature))
	data = append(data, byte(len(algID)))
	data = append(data, algID...)
	data = append(data, signature...)
	return &wire.PayloadAUTH{AuthMethod: wire.AuthMethodDigitalSig, AuthData: data}
}

// remoteOctetsOf answers the octets verifyRemoteAuth expects the remote party signed.
func remoteOctetsOf(t *testing.T, sa *SA) []byte {
	t.Helper()
	octets, err := computeSignedOctets(sa, !sa.IsInitiator)
	if err != nil {
		t.Fatalf("compute the remote party's signed octets: %v", err)
	}
	return octets
}

// TestSignatureHashNotifyAnnouncesIdentity reads the notify Ze sends, as both roles
// share one builder. Goal: AC-1, RFC 8420 Section 3's MUST to announce Identity when
// EdDSA is supported. Method: decode the notification data and compare the list.
func TestSignatureHashNotifyAnnouncesIdentity(t *testing.T) {
	notify := buildSignatureHashAlgosNotify()
	if notify.NotifyMsgType != wire.NotifySignatureHashAlgorithms {
		t.Fatalf("notify type %d, want SIGNATURE_HASH_ALGORITHMS", notify.NotifyMsgType)
	}
	want := []byte{0, 2, 0, 3, 0, 4, 0, 5}
	if !bytes.Equal(notify.NotificationData, want) {
		t.Fatalf("the notify lists % x, want % x (SHA2-256, SHA2-384, SHA2-512, Identity)",
			notify.NotificationData, want)
	}
}

// TestEd25519SignsPureRFC8032Vector signs RFC 8032 Section 7.1 TEST 1 through the
// family's signer. Goal: the Identity path is pure Ed25519, never Ed25519ph (RFC 8420
// Section 2). Method: the fixed key and the empty message give the vector's exact
// signature, and a request to prehash with SHA-512 is refused.
func TestEd25519SignsPureRFC8032Vector(t *testing.T) {
	seed, err := hex.DecodeString("9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	want, err := hex.DecodeString("e5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e06522490155" +
		"5fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b")
	if err != nil {
		t.Fatalf("signature: %v", err)
	}
	key := ed25519.NewKeyFromSeed(seed)

	sig, err := signDigest(key, []byte{}, hashIdentity)
	if err != nil {
		t.Fatalf("signDigest under Identity: %v", err)
	}
	if !bytes.Equal(sig, want) {
		t.Fatalf("signature\n% x\nwant RFC 8032 TEST 1\n% x", sig, want)
	}

	if sig, err := signDigest(key, make([]byte, 64), crypto.SHA512); err == nil {
		t.Fatalf("signDigest signed Ed25519ph (% x), which RFC 8420 Section 2 forbids", sig)
	}
}

// TestComputeX509AuthSignsWithEd25519UnderIdentity is the wiring test: a PKI
// certificate with an Ed25519 key, named by the peer's authentication, signs the real
// AUTH payload. Goal: AC-2. Method: computeX509Auth for a peer that announced
// Identity, then read the method, the AlgorithmIdentifier bytes, the signature length,
// and check the signature with the standard library over the signed octets whole;
// Ze's own verifier must accept the same payload.
func TestComputeX509AuthSignsWithEd25519UnderIdentity(t *testing.T) {
	key := newEd25519Key(t)
	sa := legacyAuthSA(t, "rfc8420-sign", key, rfc8420PeerAlgos)

	auth, err := computeX509Auth(sa)
	if err != nil {
		t.Fatalf("computeX509Auth: %v", err)
	}
	if auth.AuthMethod != wire.AuthMethodDigitalSig {
		t.Fatalf("AUTH method %d, want Digital Signature (14)", auth.AuthMethod)
	}
	data := auth.AuthData
	if len(data) != 1+len(rfc8420AlgIDEd25519)+ed25519.SignatureSize {
		t.Fatalf("AUTH data is %d octets, want 1 + 7 + 64", len(data))
	}
	if int(data[0]) != len(rfc8420AlgIDEd25519) {
		t.Fatalf("AlgorithmIdentifier length octet %d, want 7", data[0])
	}
	if !bytes.Equal(data[1:8], rfc8420AlgIDEd25519) {
		t.Fatalf("AlgorithmIdentifier % x, want % x", data[1:8], rfc8420AlgIDEd25519)
	}
	sig := data[8:]
	pub, ok := key.Public().(ed25519.PublicKey)
	if !ok {
		t.Fatalf("the key's public half is %T", key.Public())
	}
	octets := signedOctetsOf(t, sa)
	if !ed25519.Verify(pub, octets, sig) {
		t.Fatal("ed25519.Verify refused the signature over the signed octets whole")
	}
	digest := sha256.Sum256(octets)
	if ed25519.Verify(pub, digest[:], sig) {
		t.Fatal("the signature covers a SHA-256 digest, not the signed octets whole")
	}

	// The same SA verifies its own certificate as the remote one; the remote party's
	// octets differ from the local ones, so sign those for Ze's verifier.
	remote := remoteOctetsOf(t, sa)
	if err := verifyRemoteAuth(sa, digitalSigAuth(rfc8420AlgIDEd25519, ed25519.Sign(key, remote))); err != nil {
		t.Fatalf("verifyRemoteAuth refused an Ed25519 AUTH Ze's signer shape produced: %v", err)
	}
}

// TestEdDSARefusedWithoutIdentity asks an Ed25519 key to sign for a peer whose notify
// lacks Identity. Goal: AC-4, RFC 8420 Section 2's MUST NOT. Method: no payload, and
// an error that names the cause, the peer and the fix.
func TestEdDSARefusedWithoutIdentity(t *testing.T) {
	sa := legacyAuthSA(t, "rfc8420-no-identity", newEd25519Key(t),
		[]uint16{hashAlgoSHA2256, hashAlgoSHA2384, hashAlgoSHA2512})
	sa.PeerName = "cpe-gateway"

	auth, err := computeX509Auth(sa)
	if !errors.Is(err, errNoMutualHashAlgo) {
		t.Fatalf("computeX509Auth returned (%v, %v), want %v", auth, err, errNoMutualHashAlgo)
	}
	if auth != nil {
		t.Fatalf("computeX509Auth returned a payload (method %d) beside its error", auth.AuthMethod)
	}
	for _, part := range []string{"cpe-gateway", "Identity", "RFC 8420", "ECDSA"} {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("the refusal %q does not name %q", err, part)
		}
	}
}

// TestVerifyX509AuthAcceptsEd25519 hands verifyRemoteAuth an Ed25519 AUTH from the
// certificate the SA trusts. Goal: AC-6 both polarities. Method: a good signature
// verifies; one altered octet of the signed octets, one altered signature octet, a
// signature by another key, and signatures one octet short or long are refused.
func TestVerifyX509AuthAcceptsEd25519(t *testing.T) {
	key := newEd25519Key(t)
	sa := legacyAuthSA(t, "rfc8420-verify", key, rfc8420PeerAlgos)
	octets := remoteOctetsOf(t, sa)
	good := ed25519.Sign(key, octets)

	if err := verifyRemoteAuth(sa, digitalSigAuth(rfc8420AlgIDEd25519, good)); err != nil {
		t.Fatalf("verifyRemoteAuth refused a good Ed25519 AUTH: %v", err)
	}

	altered := bytes.Clone(octets)
	altered[len(altered)-1] ^= 0x01
	otherOctets := ed25519.Sign(key, altered)
	badSig := bytes.Clone(good)
	badSig[0] ^= 0x01
	otherKey := ed25519.Sign(newEd25519Key(t), octets)

	for _, tc := range []struct {
		name      string
		signature []byte
	}{
		{"a signature over other octets", otherOctets},
		{"an altered signature octet", badSig},
		{"a signature by another key", otherKey},
		{"a 63-octet signature", good[:63]},
		{"a 65-octet signature", append(bytes.Clone(good), 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyRemoteAuth(sa, digitalSigAuth(rfc8420AlgIDEd25519, tc.signature))
			if !errors.Is(err, errSignatureFailed) {
				t.Fatalf("verifyRemoteAuth = %v, want %v", err, errSignatureFailed)
			}
		})
	}
}

// TestVerifyRefusesEdDSAAlgIDWithMismatchedKey crosses the AlgorithmIdentifier and the
// certificate key. Goal: AC-7, no algorithm confusion. Method: an id-Ed25519 AUTH from
// an ECDSA certificate, and an ecdsa-with-SHA256 AUTH carrying an Ed25519 signature
// from an Ed25519 certificate, are both refused although each signature is valid
// under its signer's key.
func TestVerifyRefusesEdDSAAlgIDWithMismatchedKey(t *testing.T) {
	t.Run("id-Ed25519 from an ECDSA certificate", func(t *testing.T) {
		ecKey, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
		if err != nil {
			t.Fatalf("ecdsa key: %v", err)
		}
		sa := legacyAuthSA(t, "rfc8420-confused-ec", ecKey, rfc8420PeerAlgos)
		octets := remoteOctetsOf(t, sa)
		auth := digitalSigAuth(rfc8420AlgIDEd25519, ed25519.Sign(newEd25519Key(t), octets))
		if err := verifyRemoteAuth(sa, auth); !errors.Is(err, errSignatureFailed) {
			t.Fatalf("verifyRemoteAuth = %v, want %v", err, errSignatureFailed)
		}
		// The certificate's own key signing the octets unhashed: ecdsa.VerifyASN1
		// would accept it, because it keeps the leftmost bits of whatever it is
		// handed, so only the family's refusal of the Identity hash stops it.
		raw, err := ecdsa.SignASN1(crand.Reader, ecKey, octets)
		if err != nil {
			t.Fatalf("sign the octets unhashed: %v", err)
		}
		if err := verifyRemoteAuth(sa, digitalSigAuth(rfc8420AlgIDEd25519, raw)); !errors.Is(err, errSignatureFailed) {
			t.Fatalf("verifyRemoteAuth of an unhashed ECDSA signature under id-Ed25519 = %v, want %v",
				err, errSignatureFailed)
		}
	})
	t.Run("ecdsa-with-SHA256 from an Ed25519 certificate", func(t *testing.T) {
		key := newEd25519Key(t)
		sa := legacyAuthSA(t, "rfc8420-confused-ed", key, rfc8420PeerAlgos)
		digest := sha256.Sum256(remoteOctetsOf(t, sa))
		auth := digitalSigAuth(algIDECDSASHA256, ed25519.Sign(key, digest[:]))
		if err := verifyRemoteAuth(sa, auth); !errors.Is(err, errSignatureFailed) {
			t.Fatalf("verifyRemoteAuth = %v, want %v", err, errSignatureFailed)
		}
	})
}

// TestVerifyRefusesEdDSAAlgIDWithParameters sends the Ed25519 OID in shapes RFC 8420
// Appendix A does not allow. Goal: AC-8 and the AlgorithmIdentifier length boundary.
// Method: with NULL parameters (9 octets), as a bare OID (5), and truncated (6), each
// with a valid signature, verifyRemoteAuth refuses the identifier.
func TestVerifyRefusesEdDSAAlgIDWithParameters(t *testing.T) {
	key := newEd25519Key(t)
	sa := legacyAuthSA(t, "rfc8420-params", key, rfc8420PeerAlgos)
	sig := ed25519.Sign(key, remoteOctetsOf(t, sa))

	for _, tc := range []struct {
		name  string
		algID []byte
	}{
		{"NULL parameters", []byte{0x30, 0x07, 0x06, 0x03, 0x2B, 0x65, 0x70, 0x05, 0x00}},
		{"a bare OID", []byte{0x06, 0x03, 0x2B, 0x65, 0x70}},
		{"a truncated OID", []byte{0x30, 0x05, 0x06, 0x03, 0x2B, 0x65}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyRemoteAuth(sa, digitalSigAuth(tc.algID, sig))
			if err == nil {
				t.Fatalf("verifyRemoteAuth accepted the AlgorithmIdentifier % x", tc.algID)
			}
			if !strings.Contains(err.Error(), "unsupported algorithm identifier") {
				t.Fatalf("verifyRemoteAuth = %v, want the unsupported algorithm identifier refusal", err)
			}
		})
	}
}
