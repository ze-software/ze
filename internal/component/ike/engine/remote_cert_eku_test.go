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

// ekuIPsecIKE is id-kp-ipsecIKE (RFC 4945 Section 5.1.3.12), the key purpose an IKE
// certificate names when it carries an extended key usage. Go has no constant for it, so
// it goes in UnknownExtKeyUsage. OpenSSL writes it as "ipsecIKE" in extendedKeyUsage;
// strongSwan's pki has no flag that writes it.
var ekuIPsecIKE = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 17}

// This file carries no RFC requirement tag: rfc5280_ike_purpose_test.go carries the tagged
// cases of the same rule.

// VALIDATES: an IKE peer certificate's extended key usage is judged by RFC 4945 Section
// 5.1.3.12 and never by Go's TLS default. A certificate with no extended key usage, or
// one naming id-kp-ipsecIKE, authenticates. One naming only a TLS purpose (clientAuth or
// serverAuth) is refused by the IKE rule, "Otherwise, reject cert.", and the refusal
// says so rather than Go's "incompatible key usage".
// PREVENTS: the mandatory trust anchor importing Go's TLS default. An empty KeyUsages in
// x509.VerifyOptions means ExtKeyUsageServerAuth. A certificate with no extension passes
// that, so the repository fixtures hid it, while a certificate naming id-kp-ipsecIKE
// failed with IncompatibleUsage and a serverAuth-only one passed.
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

	// refused marks a certificate RFC 4945 Section 5.1.3.12 rejects: its extended key
	// usage is present and names neither id-kp-ipsecIKE nor anyExtendedKeyUsage.
	cases := []struct {
		name    string
		usages  []x509.ExtKeyUsage
		unknown []asn1.ObjectIdentifier
		refused bool
	}{
		{"no-extension", nil, nil, false},
		{"client-auth-only", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, nil, true},
		{"server-auth-only", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, nil, true},
		{"ipsec-ike-only", nil, []asn1.ObjectIdentifier{ekuIPsecIKE}, false},
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

			err := verifyRemoteAuth(sa, rctDigitalSigAuth(t, sa, key))
			if err != nil && strings.Contains(err.Error(), "incompatible key usage") {
				t.Fatalf("the peer certificate was judged by a TLS extended key usage: %v", err)
			}
			if tc.refused {
				if err == nil {
					t.Fatal("a certificate whose extended key usage excludes IKE authenticated")
				}
				if !strings.Contains(err.Error(), "extended key usage") {
					t.Fatalf("the refusal does not name the extended key usage: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("a certificate the configured anchor signed was refused: %v", err)
			}
		})
	}
}
