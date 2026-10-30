// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- RFC 4754 conformance coverage
//
// Proves RFC 4754 Section 7: an AUTH payload of method 9, 10 or 11 carries the
// signature as r || s, each integer left-padded with zeros to its fixed length,
// and Ze verifies exactly that encoding through verifyRemoteAuth, the function
// every received IKE_AUTH reaches.

package engine

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
)

// rfc4754Leaf builds an end-entity certificate on the given curve, issued by the CA.
func rfc4754Leaf(t *testing.T, curve elliptic.Curve, ca *x509.Certificate, caKey *ecdsa.PrivateKey) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatalf("generate the %s leaf key: %v", curve.Params().Name, err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "rfc4754-peer"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create the %s leaf certificate: %v", curve.Params().Name, err)
	}
	return der, key
}

// rfc4754Sign signs the octets the SA expects from its remote party and returns r
// and s, each left-padded to n octets. It retries until r has a zero leading octet
// (at most 10000 tries; each succeeds with probability 1/256 or better), so the
// padding Section 7 requires is always exercised rather than left to chance.
func rfc4754Sign(t *testing.T, sa *SA, method ecdsaMethod, key *ecdsa.PrivateKey) (r, s []byte) {
	t.Helper()
	octets, err := computeSignedOctets(sa, !sa.IsInitiator)
	if err != nil {
		t.Fatalf("compute the signed octets: %v", err)
	}
	h := method.newHash()
	h.Write(octets)
	digest := h.Sum(nil)

	n := method.scalarOctets
	for range 10000 {
		ri, si, err := ecdsa.Sign(rand.Reader, key, digest)
		if err != nil {
			t.Fatalf("sign with %s: %v", method.name, err)
		}
		if ri.BitLen() > 8*(n-1) {
			continue
		}
		return ri.FillBytes(make([]byte, n)), si.FillBytes(make([]byte, n))
	}
	t.Fatalf("no %s signature with a short r in 10000 tries", method.name)
	return nil, nil
}

// TestRFC4754SignatureIsRConcatS builds, for each of ECDSA-256, ECDSA-384 and
// ECDSA-521, a peer certificate on the method's curve and an AUTH payload over the
// octets Ze expects, then hands each encoding to verifyRemoteAuth and reads the
// answer. The positive case is r || s with r zero-padded; the negative cases are
// the same signature as s || r and as an RFC 7427 DER signature under the method
// number, and an ECDSA-384 AUTH from a P-256 key.
//
// RFC requirement: RFC4754-7-1 positive -- for each of AUTH methods 9, 10 and 11, verifyRemoteAuth accepts a signature payload that is r followed by s, each a big-endian integer zero-padded to 32, 48 or 66 octets.
// RFC requirement: RFC4754-7-1 negative -- verifyRemoteAuth refuses, under each of AUTH methods 9, 10 and 11, a payload that is not that concatenation: the same signature as an ASN.1 DER SEQUENCE is refused for its length, and as s followed by r it fails verification.
func TestRFC4754SignatureIsRConcatS(t *testing.T) {
	caCert, caDER, caKey := rctCA(t, "rfc4754-ca")
	if err := pki.Load(&pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			"rfc4754-ca": {Name: "rfc4754-ca", Certificate: caCert, Raw: caDER},
		},
	}); err != nil {
		t.Fatalf("load the PKI store: %v", err)
	}
	t.Cleanup(func() {
		if err := pki.Load(nil); err != nil {
			t.Errorf("clear the PKI store: %v", err)
		}
	})

	for _, tc := range []struct {
		authMethod uint8
		method     ecdsaMethod
	}{
		{wire.AuthMethodECDSA256, ecdsa256},
		{wire.AuthMethodECDSA384, ecdsa384},
		{wire.AuthMethodECDSA521, ecdsa521},
	} {
		t.Run(tc.method.name, func(t *testing.T) {
			sa := testSAWithKeys(t)
			sa.PeerCfg.Auth.Mode = ipsec.AuthX509
			sa.PeerCfg.Auth.CACertificate = "rfc4754-ca"
			der, key := rfc4754Leaf(t, tc.method.curve, caCert, caKey)
			sa.RemoteCertRaw = der

			r, s := rfc4754Sign(t, sa, tc.method, key)
			if r[0] != 0 {
				t.Fatalf("r = %x does not start with a zero octet; the padding case is not exercised", r)
			}

			// Positive: r || s, each n octets.
			concat := append(append([]byte{}, r...), s...)
			auth := &wire.PayloadAUTH{AuthMethod: tc.authMethod, AuthData: concat}
			if err := verifyRemoteAuth(sa, auth); err != nil {
				t.Fatalf("%s AUTH of r || s (%d octets) was refused: %v", tc.method.name, len(concat), err)
			}

			// Negative: the same integers in the wrong order are not the encoding.
			swapped := append(append([]byte{}, s...), r...)
			auth = &wire.PayloadAUTH{AuthMethod: tc.authMethod, AuthData: swapped}
			if err := verifyRemoteAuth(sa, auth); !errors.Is(err, errSignatureFailed) {
				t.Fatalf("%s AUTH of s || r: verifyRemoteAuth = %v, want %v", tc.method.name, err, errSignatureFailed)
			}

			// Negative: the same signature DER-encoded, as RFC 7427 would carry it.
			derSig := rfc4754DER(t, r, s)
			auth = &wire.PayloadAUTH{AuthMethod: tc.authMethod, AuthData: derSig}
			if err := verifyRemoteAuth(sa, auth); !errors.Is(err, errECDSASignatureLength) {
				t.Fatalf("%s AUTH of a DER signature (%d octets): verifyRemoteAuth = %v, want %v",
					tc.method.name, len(derSig), err, errECDSASignatureLength)
			}
		})
	}

	// Negative: ECDSA-384 names the 384-bit curve, so a P-256 key cannot sign it.
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthX509
	sa.PeerCfg.Auth.CACertificate = "rfc4754-ca"
	der, _ := rfc4754Leaf(t, elliptic.P256(), caCert, caKey)
	sa.RemoteCertRaw = der
	auth := &wire.PayloadAUTH{AuthMethod: wire.AuthMethodECDSA384, AuthData: make([]byte, 96)}
	if err := verifyRemoteAuth(sa, auth); !errors.Is(err, errUnsupportedKey) {
		t.Fatalf("ECDSA-384 AUTH from a P-256 certificate: verifyRemoteAuth = %v, want %v", err, errUnsupportedKey)
	}
}

// rfc4754DER encodes r and s as the ASN.1 SEQUENCE { INTEGER r, INTEGER s } that
// ecdsa.SignASN1 writes.
func rfc4754DER(t *testing.T, r, s []byte) []byte {
	t.Helper()
	sig := struct{ R, S *big.Int }{new(big.Int).SetBytes(r), new(big.Int).SetBytes(s)}
	der, err := asn1.Marshal(sig)
	if err != nil {
		t.Fatalf("DER-encode the signature: %v", err)
	}
	return der
}
