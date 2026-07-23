package engine

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"strings"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/ipsec"
	"codeberg.org/thomas-mangin/ze/internal/component/pki"
)

// eapTLSResponderPKI installs a CA plus a server certificate signed by it, under
// the given names, so a test can then ask for a CA name that is deliberately
// absent from the store.
func eapTLSResponderPKI(t *testing.T, caName, certName string) {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: caName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}

	srvKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	srvTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: certName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	srvDER, err := x509.CreateCertificate(rand.Reader, srvTmpl, caCert, &srvKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	srvCert, err := x509.ParseCertificate(srvDER)
	if err != nil {
		t.Fatal(err)
	}

	if err := pki.Load(&pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			caName: {Name: caName, Certificate: caCert, Raw: caDER},
		},
		Certificates: map[string]*pki.CertificateEntry{
			certName: {Name: certName, Certificate: srvCert, Raw: srvDER, PrivateKey: srvKey},
		},
	}); err != nil {
		t.Fatal(err)
	}
	// pki.Load swaps the process-global store (pki/store.go). Restore it so a
	// later test in this package that reads PKI state without loading its own
	// fixture is not silently order-dependent on this one.
	t.Cleanup(func() {
		if err := pki.Load(&pki.PKIConfig{
			CACerts:      make(map[string]*pki.CACertEntry),
			Certificates: make(map[string]*pki.CertificateEntry),
		}); err != nil {
			t.Errorf("restore pki store: %v", err)
		}
	})
}

func eapTLSResponderSA(certName, caName string) *SA {
	sa := &SA{}
	sa.PeerCfg.Auth.Mode = ipsec.AuthEAPTLS
	sa.PeerCfg.Auth.Certificate = certName
	sa.PeerCfg.Auth.CACertificate = caName
	return sa
}

// VALIDATES: a ca-certificate naming a CA absent from the PKI store is an error,
// not a silently empty client trust pool.
// PREVENTS: the responder building a method config that then refuses every client
// at TLS handshake with an opaque "certificate signed by unknown authority",
// while nothing anywhere names the CA that failed to resolve.
func TestEAPTLSServerConfigRefusesMissingCA(t *testing.T) {
	eapTLSResponderPKI(t, "ra-present-ca", "ra-server")

	sa := eapTLSResponderSA("ra-server", "ra-absent-ca")

	cfg, err := eapTLSServerConfig(sa)
	if err == nil {
		t.Fatalf("eapTLSServerConfig accepted a missing CA, returned CACertPEM=%d bytes", len(cfg.CACertPEM))
	}
	if !strings.Contains(err.Error(), "ra-absent-ca") {
		t.Errorf("error does not name the missing CA: %v", err)
	}
}

// VALIDATES: a resolvable ca-certificate still populates the trust pool.
// PREVENTS: the refusal above regressing into a blanket rejection.
func TestEAPTLSServerConfigLoadsResolvableCA(t *testing.T) {
	eapTLSResponderPKI(t, "ra-good-ca", "ra-good-server")

	sa := eapTLSResponderSA("ra-good-server", "ra-good-ca")

	cfg, err := eapTLSServerConfig(sa)
	if err != nil {
		t.Fatalf("eapTLSServerConfig: %v", err)
	}
	if len(cfg.CACertPEM) == 0 {
		t.Error("CACertPEM is empty for a CA that exists in the store")
	}
	if len(cfg.ServerCertPEM) == 0 || len(cfg.ServerKeyPEM) == 0 {
		t.Error("server certificate or key missing")
	}
}

// VALIDATES: an EAP-TLS responder with no ca-certificate configured at all is
// refused at method-config time.
// PREVENTS: a gateway starting with no client trust anchor and rejecting every
// client opaquely. ValidatePKIRefs (ipsec/validate.go) refuses this at config
// verify, so with today's code paths this branch is unreachable from a
// validated config -- it is a deliberate backstop, not the first line of
// defense.
//
// An earlier version of this comment claimed it "also covers the remote-access
// path, which reaches this code through a synthesized PeerCfg". That is false
// and was caught by review: SA.PeerCfg is only ever assigned from cfg.Peers
// (initiator.go, responder.go, rekey.go), and cfg.Peers is populated only from
// `site-to-site peer` entries (ipsec/config.go). Nothing builds a
// SiteToSitePeer from RemoteAccess. The remote-access gap is real but lives in
// ValidateRemoteAccess, and is owned by plan/spec-ipsec-remote-access.md.
func TestEAPTLSServerConfigRefusesEmptyCAName(t *testing.T) {
	eapTLSResponderPKI(t, "ra-any-ca", "ra-any-server")

	sa := eapTLSResponderSA("ra-any-server", "")

	if _, err := eapTLSServerConfig(sa); err == nil {
		t.Fatal("eapTLSServerConfig accepted an EAP-TLS responder with no ca-certificate")
	}
}
