package engine

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
)

// ekuLeaf builds an end-entity certificate that carries exactly the given extended key
// usages. An empty list leaves the extension out, which is what gen-pki.sh produces.
func ekuLeaf(t *testing.T, cn string, usages []x509.ExtKeyUsage, unknown []asn1.ObjectIdentifier, issuer *x509.Certificate, issuerKey *ecdsa.PrivateKey) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate the leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:       big.NewInt(3),
		Subject:            pkix.Name{CommonName: cn},
		NotBefore:          time.Now().Add(-time.Hour),
		NotAfter:           time.Now().Add(time.Hour),
		ExtKeyUsage:        usages,
		UnknownExtKeyUsage: unknown,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, issuer, &key.PublicKey, issuerKey)
	if err != nil {
		t.Fatalf("create the leaf certificate: %v", err)
	}
	return der, key
}

// ekuIPsecIKE is id-kp-ipsecIKE, the extended key usage strongSwan puts on a certificate
// issued with "pki --issue --flag ike" (RFC 4945 Section 5.1.3.12). Go has no constant for
// it, so it goes in UnknownExtKeyUsage.
var ekuIPsecIKE = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 17}

// This file carries no RFC requirement tag. RFC 7296 puts no extended key usage rule on an
// IKE peer certificate, and that absence is what the test holds.

// VALIDATES: an IKE peer certificate authenticates whatever extended key usage it
// carries, as long as it chains to the configured anchor.
// PREVENTS: the mandatory trust anchor importing Go's TLS default. An empty KeyUsages in
// x509.VerifyOptions means ExtKeyUsageServerAuth. A certificate with no extension passes
// that, so the repository fixtures hid it. A certificate that names clientAuth alone, or
// strongSwan's ipsecIKE from "pki --issue --flag ike", failed with IncompatibleUsage.
// An operator avoided the path entirely until the trust anchor became mandatory.
func TestEkuPeerCertificateIsNotJudgedByTLSExtendedKeyUsage(t *testing.T) {
	caCert, caDER, caKey := rctCA(t, "eku-ca")
	if err := pki.Load(&pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			"eku-ca": {Name: "eku-ca", Certificate: caCert, Raw: caDER},
		},
	}); err != nil {
		t.Fatalf("load the PKI store: %v", err)
	}
	t.Cleanup(func() {
		if err := pki.Load(nil); err != nil {
			t.Errorf("clear the PKI store: %v", err)
		}
	})

	cases := []struct {
		name    string
		usages  []x509.ExtKeyUsage
		unknown []asn1.ObjectIdentifier
	}{
		{"no-extension", nil, nil},
		{"client-auth-only", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, nil},
		{"server-auth-only", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, nil},
		{"ipsec-ike-only", nil, []asn1.ObjectIdentifier{ekuIPsecIKE}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sa := testSAWithKeys(t)
			sa.PeerCfg.Auth.Mode = ipsec.AuthX509
			sa.PeerCfg.Auth.CACertificate = "eku-ca"
			sa.PeerCfg.Auth.RemoteID = "eku-peer"
			sa.RemoteIDPayload = &wire.PayloadID{
				IDPayloadType: wire.PayloadTypeIDr,
				IDType:        wire.IDTypeFQDN,
				IDData:        []byte("eku-peer"),
			}

			der, key := ekuLeaf(t, "eku-peer", tc.usages, tc.unknown, caCert, caKey)
			sa.RemoteCertRaw = der

			if err := verifyRemoteAuth(sa, rctDigitalSigAuth(t, sa, key)); err != nil {
				if strings.Contains(err.Error(), "incompatible key usage") {
					t.Fatalf("the peer certificate was judged by a TLS extended key usage: %v", err)
				}
				t.Fatalf("a certificate the configured anchor signed was refused: %v", err)
			}
		})
	}
}
