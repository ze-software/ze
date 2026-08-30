// RFC: rfc/short/rfc7427.md -- Section 3 states the Digital Signature method as a
// conditional permission; Section 4 states which algorithm the signer may pick.
//
// VALIDATES: Ze emits AUTH method 14 only after a SIGNATURE_HASH_ALGORITHMS notify has
// arrived, and it signs only with an algorithm that notify listed.
// PREVENTS: the two wire-visible defects these tests were written for. computeX509Auth
// emitted method 14 whether or not the notify had arrived, and selectSignatureAlgorithm
// read the ECDSA curve alone while the RSA branch fell through to SHA-256 without
// testing that SHA2-256 was offered. A peer that advertised one hash was therefore sent
// a signature under another, and a peer that advertised nothing was sent a method RFC
// 7427 Section 3 does not permit it to be sent.

package engine

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
)

// loadSigningKey puts one device certificate for key, and the CA that signed it, into
// the process PKI store under name. pki.Load validates the chain, so the CA is built
// here rather than self-signing the device certificate.
func loadSigningKey(t *testing.T, name string, key crypto.Signer) {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		t.Fatalf("ca key: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name + "-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(crand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("ca certificate: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse ca certificate: %v", err)
	}

	devTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	devDER, err := x509.CreateCertificate(crand.Reader, devTmpl, caCert, key.Public(), caKey)
	if err != nil {
		t.Fatalf("device certificate: %v", err)
	}
	devCert, err := x509.ParseCertificate(devDER)
	if err != nil {
		t.Fatalf("parse device certificate: %v", err)
	}

	if err := pki.Load(&pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			name + "-ca": {Name: name + "-ca", Certificate: caCert, Raw: caDER},
		},
		Certificates: map[string]*pki.CertificateEntry{
			name: {Name: name, Certificate: devCert, Raw: devDER, PrivateKey: key},
		},
	}); err != nil {
		t.Fatalf("pki.Load: %v", err)
	}
}

// RFC requirement: RFC7427-3-4 positive -- "the peer is only allowed to use this
// authentication method if the Notify payload of type SIGNATURE_HASH_ALGORITHMS has
// been sent and received by each peer" (RFC 7427 Section 3). Ze sends the notify in
// every IKE_SA_INIT, so the permission turns on reception, and a received list makes
// method 14 the method computeX509Auth emits.
//
// RFC requirement: RFC7427-3-4 negative -- the discriminator, and the case the
// permission exists for. With no notify received the condition is not met, so the
// method is refused rather than emitted. Without this case the requirement would be
// proven by a test that never enters the branch the RFC is about.
func TestRFC7427DigitalSignatureNeedsTheNotify(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		t.Fatalf("p256 key: %v", err)
	}
	loadSigningKey(t, "rfc7427-notify-gate", key)

	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthX509
	sa.PeerCfg.Auth.Certificate = "rfc7427-notify-gate"

	// No SIGNATURE_HASH_ALGORITHMS notify arrived from this peer.
	sa.RemoteHashAlgos = nil
	if _, err := computeX509Auth(sa); !errors.Is(err, errNoSignatureHashAlgos) {
		t.Fatalf("with no notify received, computeX509Auth returned %v, want %v",
			err, errNoSignatureHashAlgos)
	}

	// The same SA once the notify has arrived.
	sa.RemoteHashAlgos = []uint16{hashAlgoSHA2256}
	auth, err := computeX509Auth(sa)
	if err != nil {
		t.Fatalf("with the notify received, computeX509Auth: %v", err)
	}
	if auth.AuthMethod != wire.AuthMethodDigitalSig {
		t.Fatalf("AUTH method is %d, want %d (Digital Signature)",
			auth.AuthMethod, wire.AuthMethodDigitalSig)
	}
}

// RFC requirement: RFC7427-4-1 positive -- "When calculating the digital signature, a
// peer MUST pick one algorithm sent by the other peer" (RFC 7427 Section 4). Each
// positive case sends one algorithm and asserts that the identifier Ze selects is that
// algorithm, so the selection follows the peer's list rather than the local key alone.
//
// RFC requirement: RFC7427-4-1 negative -- a list holding no algorithm this key can
// sign with leaves nothing to pick, so the selection fails instead of returning an
// identifier the peer never sent. The empty list is the same case: a peer that sent no
// notify sent no algorithm.
func TestRFC7427SignatureAlgorithmIsOneThePeerSent(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	ecKey256, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		t.Fatalf("p256 key: %v", err)
	}
	ecKey384, err := ecdsa.GenerateKey(elliptic.P384(), crand.Reader)
	if err != nil {
		t.Fatalf("p384 key: %v", err)
	}

	// 1 is SHA-1, 2 SHA2-256, 3 SHA2-384, 4 SHA2-512 (RFC 7427 Section 4).
	for _, tc := range []struct {
		name    string
		key     crypto.PrivateKey
		sent    []uint16
		want    []byte
		wantErr bool
	}{
		{"rsa, the peer sent SHA2-256 alone", rsaKey, []uint16{2}, algIDSHA256WithRSA, false},
		{"rsa, the peer sent SHA2-384 alone", rsaKey, []uint16{3}, algIDSHA384WithRSA, false},
		{"rsa, the peer sent SHA2-512 alone", rsaKey, []uint16{4}, algIDSHA512WithRSA, false},
		{"rsa, the peer sent all three and the strongest wins", rsaKey, []uint16{2, 3, 4}, algIDSHA512WithRSA, false},
		{"rsa, the peer sent SHA-1 alone", rsaKey, []uint16{1}, nil, true},
		{"rsa, the peer sent nothing", rsaKey, nil, nil, true},
		{"p-256, the peer sent SHA2-256", ecKey256, []uint16{2}, algIDECDSASHA256, false},
		{"p-256, the peer sent SHA2-384 alone", ecKey256, []uint16{3}, nil, true},
		{"p-256, the peer sent nothing", ecKey256, nil, nil, true},
		{"p-384, the peer sent SHA2-384", ecKey384, []uint16{3}, algIDECDSASHA384, false},
		{"p-384, the peer sent SHA2-256 alone", ecKey384, []uint16{2}, nil, true},
		{"p-384, the peer sent nothing", ecKey384, nil, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			algID, _, err := selectSignatureAlgorithm(tc.key, tc.sent)
			if tc.wantErr {
				if !errors.Is(err, errNoMutualHashAlgo) {
					t.Fatalf("selectSignatureAlgorithm returned (% x, %v), want %v",
						algID, err, errNoMutualHashAlgo)
				}
				return
			}
			if err != nil {
				t.Fatalf("selectSignatureAlgorithm: %v", err)
			}
			if !bytes.Equal(algID, tc.want) {
				t.Fatalf("algorithm identifier is % x, want % x", algID, tc.want)
			}
		})
	}
}

// RFC requirement: RFC7427-4-1 negative -- the same requirement at the producer that
// puts the signature on the wire. computeX509Auth signs with the private key the PKI
// store holds, so this is where "one algorithm sent by the other peer" either holds or
// does not for a real key. Both key types are covered, because the defect had one shape
// for each: the ECDSA branch read the curve and never the peer's list, and the RSA
// branch fell through to SHA-256 without testing that SHA2-256 was sent.
func TestRFC7427AuthRefusesAnAlgorithmThePeerDidNotSend(t *testing.T) {
	ecKey384, err := ecdsa.GenerateKey(elliptic.P384(), crand.Reader)
	if err != nil {
		t.Fatalf("p384 key: %v", err)
	}
	rsaKey, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}

	for _, tc := range []struct {
		name string
		key  crypto.Signer
		sent []uint16
	}{
		{"a P-384 key against a peer that sent SHA2-256 alone", ecKey384, []uint16{2}},
		{"an RSA key against a peer that sent SHA-1 alone", rsaKey, []uint16{1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loadSigningKey(t, "rfc7427-unoffered", tc.key)

			sa := testSAWithKeys(t)
			sa.PeerCfg.Auth.Mode = ipsec.AuthX509
			sa.PeerCfg.Auth.Certificate = "rfc7427-unoffered"
			sa.RemoteHashAlgos = tc.sent

			auth, err := computeX509Auth(sa)
			if !errors.Is(err, errNoMutualHashAlgo) {
				t.Fatalf("computeX509Auth returned (%v, %v), want %v",
					auth, err, errNoMutualHashAlgo)
			}
		})
	}
}
