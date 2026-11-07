// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the signature methods Ze sends
// Related: register_signature_ed448.go -- the Ed448 family under the Identity hash
// Related: auth_eddsa_test.go -- the Ed25519 twin of these tests, and their helpers
//
// RFC: rfc/full/rfc8420.txt -- EdDSA in IKEv2 (not enrolled: no release profile lists
// RFC 8420, so these tests carry no requirement tag).
//
// VALIDATES: an Ed448 certificate key signs AUTH method 14 under the Identity hash for
// a peer that announced Identity, with the RFC 8420 id-Ed448 AlgorithmIdentifier and a
// pure Ed448 signature over the signed octets whole; Ze verifies such an AUTH from an
// Ed448 leaf a P-256 CA issued, whose key crypto/x509 leaves unparsed, and refuses it
// when an octet changed, the signature length is wrong, the AlgorithmIdentifier and
// the certificate key disagree, the AlgorithmIdentifier carries parameters, or the
// leaf comes from a CA Ze does not trust.
// PREVENTS: errUnsupportedKey for every Ed448 certificate, and an Ed448 path that
// prehashes or accepts a signature its certificate cannot have made.
//
// The fixtures under testdata/ed448 are the x509-ed448-ze-responder lab PKI, minted
// with openssl: lab-ca.pem is the P-256 lab CA, leaf.pem an Ed448 leaf it issued, and
// untrusted-leaf.pem an Ed448 leaf from untrusted-ca.example.net.

package engine

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/ed448"
)

// rfc8420AlgIDEd448 is RFC 8420 Appendix A.2 "Binary encoding: 3005 0603 2B65 71".
var rfc8420AlgIDEd448 = []byte{0x30, 0x05, 0x06, 0x03, 0x2B, 0x65, 0x71}

// readPEMFixture answers the DER of the one PEM block in testdata/ed448/name.
func readPEMFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "ed448", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		t.Fatalf("fixture %s holds no PEM block", name)
	}
	return block.Bytes
}

// ed448AuthSA loads the lab CA and the Ed448 leaf (leaf.pem, with its key) into the
// PKI store as name and name-ca, and answers an SA that signs with that leaf and
// takes it as the remote certificate, trusting the lab CA.
func ed448AuthSA(t *testing.T, name string, remoteHashAlgos []uint16) (*SA, ed448.PrivateKey) {
	t.Helper()
	caDER := readPEMFixture(t, "lab-ca.pem")
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse lab CA: %v", err)
	}
	leafDER := readPEMFixture(t, "leaf.pem")
	leafCert, err := x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatalf("parse leaf.pem: %v", err)
	}
	key, err := ed448.ParsePKCS8PrivateKey(readPEMFixture(t, "leaf-key.pem"))
	if err != nil {
		t.Fatalf("parse leaf-key.pem: %v", err)
	}
	if err := pki.Load(&pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			name + "-ca": {Name: name + "-ca", Certificate: caCert, Raw: caDER},
		},
		Certificates: map[string]*pki.CertificateEntry{
			name: {Name: name, Certificate: leafCert, Raw: leafDER, PrivateKey: key},
		},
	}); err != nil {
		t.Fatalf("pki.Load: %v", err)
	}
	t.Cleanup(func() {
		if err := pki.Load(nil); err != nil {
			t.Errorf("pki.Load reset: %v", err)
		}
	})

	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthX509
	sa.PeerCfg.Auth.Certificate = name
	sa.PeerCfg.Auth.CACertificate = name + "-ca"
	sa.RemoteCertRaw = leafDER
	sa.RemoteHashAlgos = remoteHashAlgos
	return sa, key
}

// TestComputeX509AuthSignsWithEd448UnderIdentity signs with an Ed448 key for a peer
// that announced Identity. Goal: AC-3. Method: read the AUTH payload's method,
// AlgorithmIdentifier and length, verify the signature over the signed octets whole
// with an empty context, prove it does not cover a SHA-256 digest, and hand Ze's
// verifier a signature of the same shape.
func TestComputeX509AuthSignsWithEd448UnderIdentity(t *testing.T) {
	sa, key := ed448AuthSA(t, "rfc8420-ed448-sign", rfc8420PeerAlgos)

	auth, err := computeX509Auth(sa)
	if err != nil {
		t.Fatalf("computeX509Auth: %v", err)
	}
	if auth.AuthMethod != wire.AuthMethodDigitalSig {
		t.Fatalf("AUTH method %d, want Digital Signature (14)", auth.AuthMethod)
	}
	data := auth.AuthData
	if len(data) != 1+len(rfc8420AlgIDEd448)+ed448.SignatureSize {
		t.Fatalf("AUTH data is %d octets, want 1 + 7 + 114", len(data))
	}
	if int(data[0]) != len(rfc8420AlgIDEd448) {
		t.Fatalf("AlgorithmIdentifier length octet %d, want 7", data[0])
	}
	if !bytes.Equal(data[1:8], rfc8420AlgIDEd448) {
		t.Fatalf("AlgorithmIdentifier % x, want % x", data[1:8], rfc8420AlgIDEd448)
	}
	sig := data[8:]
	octets := signedOctetsOf(t, sa)
	if !ed448.Verify(key.Public(), octets, sig) {
		t.Fatal("ed448.Verify refused the signature over the signed octets whole")
	}
	digest := sha256.Sum256(octets)
	if ed448.Verify(key.Public(), digest[:], sig) {
		t.Fatal("the signature covers a SHA-256 digest, not the signed octets whole")
	}

	remote := remoteOctetsOf(t, sa)
	good, err := ed448.Sign(key, remote)
	if err != nil {
		t.Fatalf("ed448.Sign: %v", err)
	}
	if err := verifyRemoteAuth(sa, digitalSigAuth(rfc8420AlgIDEd448, good)); err != nil {
		t.Fatalf("verifyRemoteAuth refused an Ed448 AUTH Ze's signer shape produced: %v", err)
	}
}

// TestEd448RefusedWithoutIdentity asks an Ed448 key to sign for a peer whose notify
// lacks Identity. Goal: AC-4 for Ed448, RFC 8420 Section 2's MUST NOT. Method: no
// payload, and an error that names the cause, the peer and the fix.
func TestEd448RefusedWithoutIdentity(t *testing.T) {
	sa, _ := ed448AuthSA(t, "rfc8420-ed448-no-identity",
		[]uint16{hashAlgoSHA2256, hashAlgoSHA2384, hashAlgoSHA2512})
	sa.PeerName = "cpe-gateway"

	auth, err := computeX509Auth(sa)
	if !errors.Is(err, errNoMutualHashAlgo) {
		t.Fatalf("computeX509Auth returned (%v, %v), want %v", auth, err, errNoMutualHashAlgo)
	}
	if auth != nil {
		t.Fatalf("computeX509Auth returned a payload (method %d) beside its error", auth.AuthMethod)
	}
	for _, part := range []string{"cpe-gateway", "Ed448", "Identity", "RFC 8420", "ECDSA"} {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("the refusal %q does not name %q", err, part)
		}
	}
}

// TestVerifyX509AuthAcceptsEd448 hands verifyRemoteAuth an Ed448 AUTH from the
// certificate the SA trusts. Goal: AC-6 both polarities for Ed448, and the signature
// length boundary. Method: a good signature verifies; a signature over other octets,
// one altered signature octet, a signature by another key, and signatures of 113 and
// 115 octets are refused with errSignatureFailed.
func TestVerifyX509AuthAcceptsEd448(t *testing.T) {
	sa, key := ed448AuthSA(t, "rfc8420-ed448-verify", rfc8420PeerAlgos)
	octets := remoteOctetsOf(t, sa)
	good, err := ed448.Sign(key, octets)
	if err != nil {
		t.Fatalf("ed448.Sign: %v", err)
	}
	if err := verifyRemoteAuth(sa, digitalSigAuth(rfc8420AlgIDEd448, good)); err != nil {
		t.Fatalf("verifyRemoteAuth refused a good Ed448 AUTH: %v", err)
	}

	altered := bytes.Clone(octets)
	altered[len(altered)-1] ^= 0x01
	otherOctets, err := ed448.Sign(key, altered)
	if err != nil {
		t.Fatalf("ed448.Sign: %v", err)
	}
	badSig := bytes.Clone(good)
	badSig[0] ^= 0x01
	otherKey, err := ed448.ParsePKCS8PrivateKey(readPEMFixture(t, "untrusted-leaf-key.pem"))
	if err != nil {
		t.Fatalf("parse the other key: %v", err)
	}
	byOtherKey, err := ed448.Sign(otherKey, octets)
	if err != nil {
		t.Fatalf("ed448.Sign: %v", err)
	}

	for _, tc := range []struct {
		name      string
		signature []byte
	}{
		{"a signature over other octets", otherOctets},
		{"an altered signature octet", badSig},
		{"a signature by another key", byOtherKey},
		{"a 113-octet signature", good[:113]},
		{"a 115-octet signature", append(bytes.Clone(good), 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyRemoteAuth(sa, digitalSigAuth(rfc8420AlgIDEd448, tc.signature))
			if !errors.Is(err, errSignatureFailed) {
				t.Fatalf("verifyRemoteAuth = %v, want %v", err, errSignatureFailed)
			}
		})
	}
}

// TestVerifyX509AuthAcceptsEd448LeafUnderP256CA proves assumption A-2 and AC-14 at
// the engine: crypto/x509 parses the Ed448 leaf with its key unread, the chain to the
// P-256 lab CA verifies, and the key the Ed448 family reads from the raw
// SubjectPublicKeyInfo is the leaf key's own. Method: read the parsed certificate,
// then verify an Ed448 AUTH through verifyRemoteAuth, which runs getRemoteCert.
func TestVerifyX509AuthAcceptsEd448LeafUnderP256CA(t *testing.T) {
	sa, key := ed448AuthSA(t, "rfc8420-ed448-chain", rfc8420PeerAlgos)
	cert, err := x509.ParseCertificate(sa.RemoteCertRaw)
	if err != nil {
		t.Fatalf("parse the Ed448 leaf: %v", err)
	}
	if cert.PublicKey != nil {
		t.Fatalf("crypto/x509 parsed the Ed448 key as %T; certificatePublicKey's fallback is then dead", cert.PublicKey)
	}
	pub, err := certificatePublicKey(cert)
	if err != nil {
		t.Fatalf("certificatePublicKey: %v", err)
	}
	if !key.Public().Equal(pub) {
		t.Fatal("the key read from the SubjectPublicKeyInfo is not the leaf key's")
	}
	sig, err := ed448.Sign(key, remoteOctetsOf(t, sa))
	if err != nil {
		t.Fatalf("ed448.Sign: %v", err)
	}
	if err := verifyRemoteAuth(sa, digitalSigAuth(rfc8420AlgIDEd448, sig)); err != nil {
		t.Fatalf("verifyRemoteAuth refused an Ed448 leaf under the P-256 lab CA: %v", err)
	}
}

// TestVerifyX509AuthRefusesEd448LeafFromUntrustedCA hands verifyRemoteAuth an Ed448
// leaf from a CA the store does not hold. Goal: AC-15. Method: the AUTH signature is
// valid under the leaf's key, so only the chain check can refuse it, and the error
// names the peer and the unknown authority.
func TestVerifyX509AuthRefusesEd448LeafFromUntrustedCA(t *testing.T) {
	sa, _ := ed448AuthSA(t, "rfc8420-ed448-untrusted", rfc8420PeerAlgos)
	sa.PeerName = "swan"
	sa.RemoteCertRaw = readPEMFixture(t, "untrusted-leaf.pem")
	key, err := ed448.ParsePKCS8PrivateKey(readPEMFixture(t, "untrusted-leaf-key.pem"))
	if err != nil {
		t.Fatalf("parse the untrusted leaf key: %v", err)
	}
	sig, err := ed448.Sign(key, remoteOctetsOf(t, sa))
	if err != nil {
		t.Fatalf("ed448.Sign: %v", err)
	}
	err = verifyRemoteAuth(sa, digitalSigAuth(rfc8420AlgIDEd448, sig))
	if err == nil {
		t.Fatal("verifyRemoteAuth accepted an Ed448 leaf from an untrusted CA")
	}
	if _, ok := errors.AsType[x509.UnknownAuthorityError](err); !ok {
		t.Fatalf("verifyRemoteAuth = %v, want an x509.UnknownAuthorityError", err)
	}
	if !strings.Contains(err.Error(), `"swan"`) {
		t.Fatalf("the refusal %q does not name the peer", err)
	}
}

// TestVerifyRefusesEd448AlgIDWithMismatchedKey crosses the AlgorithmIdentifier and
// the certificate key. Goal: AC-7 for Ed448. Method: an id-Ed448 AUTH from an Ed25519
// certificate, and id-Ed25519 or ecdsa-with-SHA256 AUTHs from an Ed448 certificate,
// are refused although each signature is valid under its signer's key.
func TestVerifyRefusesEd448AlgIDWithMismatchedKey(t *testing.T) {
	t.Run("id-Ed448 from an Ed25519 certificate", func(t *testing.T) {
		sa := legacyAuthSA(t, "rfc8420-ed448-confused-25519", newEd25519Key(t), rfc8420PeerAlgos)
		other, err := ed448.ParsePKCS8PrivateKey(readPEMFixture(t, "leaf-key.pem"))
		if err != nil {
			t.Fatalf("parse Ed448 key: %v", err)
		}
		sig, err := ed448.Sign(other, remoteOctetsOf(t, sa))
		if err != nil {
			t.Fatalf("ed448.Sign: %v", err)
		}
		if err := verifyRemoteAuth(sa, digitalSigAuth(rfc8420AlgIDEd448, sig)); !errors.Is(err, errSignatureFailed) {
			t.Fatalf("verifyRemoteAuth = %v, want %v", err, errSignatureFailed)
		}
	})
	t.Run("id-Ed25519 from an Ed448 certificate", func(t *testing.T) {
		sa, key := ed448AuthSA(t, "rfc8420-ed448-confused-448", rfc8420PeerAlgos)
		sig, err := ed448.Sign(key, remoteOctetsOf(t, sa))
		if err != nil {
			t.Fatalf("ed448.Sign: %v", err)
		}
		if err := verifyRemoteAuth(sa, digitalSigAuth(rfc8420AlgIDEd25519, sig)); !errors.Is(err, errSignatureFailed) {
			t.Fatalf("verifyRemoteAuth = %v, want %v", err, errSignatureFailed)
		}
	})
	t.Run("ecdsa-with-SHA256 from an Ed448 certificate", func(t *testing.T) {
		sa, key := ed448AuthSA(t, "rfc8420-ed448-confused-ec", rfc8420PeerAlgos)
		digest := sha256.Sum256(remoteOctetsOf(t, sa))
		sig, err := ed448.Sign(key, digest[:])
		if err != nil {
			t.Fatalf("ed448.Sign: %v", err)
		}
		if err := verifyRemoteAuth(sa, digitalSigAuth(algIDECDSASHA256, sig)); !errors.Is(err, errSignatureFailed) {
			t.Fatalf("verifyRemoteAuth = %v, want %v", err, errSignatureFailed)
		}
	})
}

// TestVerifyRefusesEd448AlgIDWithParameters sends the Ed448 OID in shapes RFC 8420
// Appendix A.2 does not allow. Goal: AC-8 for Ed448. Method: with NULL parameters, as
// a bare OID, and truncated, each with a valid signature, the identifier is refused.
func TestVerifyRefusesEd448AlgIDWithParameters(t *testing.T) {
	sa, key := ed448AuthSA(t, "rfc8420-ed448-params", rfc8420PeerAlgos)
	sig, err := ed448.Sign(key, remoteOctetsOf(t, sa))
	if err != nil {
		t.Fatalf("ed448.Sign: %v", err)
	}
	for _, tc := range []struct {
		name  string
		algID []byte
	}{
		{"NULL parameters", []byte{0x30, 0x07, 0x06, 0x03, 0x2B, 0x65, 0x71, 0x05, 0x00}},
		{"a bare OID", []byte{0x06, 0x03, 0x2B, 0x65, 0x71}},
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

// TestGetRemoteCertNamesEd448SignedChain hands getRemoteCert an Ed448 leaf that an
// Ed448 CA signed. Goal: the Known Limitations refusal of spec home-cpe-5: crypto/x509
// cannot check an Ed448 signature, and the refusal names the peer, the issuer and
// the fix instead of "signed by unknown authority". Method: load the Ed448 CA as the
// peer's ca-certificate, so only the signature algorithm can refuse the chain.
func TestGetRemoteCertNamesEd448SignedChain(t *testing.T) {
	sa, _ := ed448AuthSA(t, "rfc8420-ed448-signed-chain", rfc8420PeerAlgos)
	caDER := readPEMFixture(t, "ed448-ca.pem")
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse the Ed448 CA: %v", err)
	}
	if err := pki.Load(&pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			"ed448-ca": {Name: "ed448-ca", Certificate: caCert, Raw: caDER},
		},
	}); err != nil {
		t.Fatalf("pki.Load: %v", err)
	}
	sa.PeerName = "swan"
	sa.PeerCfg.Auth.CACertificate = "ed448-ca"
	sa.RemoteCertRaw = readPEMFixture(t, "ed448-signed-leaf.pem")

	_, err = getRemoteCert(sa)
	if err == nil {
		t.Fatal("getRemoteCert accepted a certificate an Ed448 CA signed")
	}
	for _, part := range []string{`"swan"`, "ed448-ca.example.net", "Ed448", "ECDSA"} {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("the refusal %q does not name %q", err, part)
		}
	}
}
