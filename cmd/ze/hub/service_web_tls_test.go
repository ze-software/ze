//go:build ze_web

// Design: docs/architecture/pki/tls-listeners.md -- hub web TLS material selection tests

package hub

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	zepki "github.com/ze-software/ze/internal/component/pki"
)

// memCertStore is the self-signed persistence backend, in memory. generated
// records whether the self-signed path was taken.
type memCertStore struct {
	cert      []byte
	key       []byte
	generated bool
}

func (s *memCertStore) ReadCert() ([]byte, error) { return s.cert, nil }
func (s *memCertStore) ReadKey() ([]byte, error)  { return s.key, nil }
func (s *memCertStore) WriteCert(data []byte) error {
	s.cert = append([]byte(nil), data...)
	s.generated = true
	return nil
}
func (s *memCertStore) WriteKey(data []byte) error {
	s.key = append([]byte(nil), data...)
	return nil
}
func (s *memCertStore) Exists() bool { return len(s.cert) > 0 && len(s.key) > 0 }

// loadWebPKIStore installs a store holding a root CA, an intermediate, a
// chain-bearing device certificate named "web-cert", and a keyless entry.
func loadWebPKIStore(t *testing.T) {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "hub root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}

	interKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	interTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "hub intermediate"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	interDER, err := x509.CreateCertificate(rand.Reader, interTmpl, caCert, &interKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	interCert, err := x509.ParseCertificate(interDER)
	if err != nil {
		t.Fatal(err)
	}

	devKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	devTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "hub web leaf"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	devDER, err := x509.CreateCertificate(rand.Reader, devTmpl, interCert, &devKey.PublicKey, interKey)
	if err != nil {
		t.Fatal(err)
	}
	devCert, err := x509.ParseCertificate(devDER)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &zepki.PKIConfig{
		CACerts: map[string]*zepki.CACertEntry{
			"hub-ca": {Name: "hub-ca", Certificate: caCert, Raw: caDER},
		},
		Certificates: map[string]*zepki.CertificateEntry{
			"web-cert": {
				Name: "web-cert", Certificate: devCert, Raw: devDER, PrivateKey: devKey,
				Intermediates: []*x509.Certificate{interCert}, RawIntermediates: [][]byte{interDER},
			},
			"keyless": {
				Name: "keyless", Certificate: devCert, Raw: devDER,
				Intermediates: []*x509.Certificate{interCert}, RawIntermediates: [][]byte{interDER},
			},
		},
	}
	if err := zepki.Load(cfg); err != nil {
		t.Fatalf("pki Load: %v", err)
	}
	t.Cleanup(func() { _ = zepki.Load(nil) })
}

func TestStartWebServerUsesPKIMaterial(t *testing.T) {
	// VALIDATES: AC-1 -- a configured certificate name makes the hub serve the
	// PKI chain instead of a self-signed certificate.
	loadWebPKIStore(t)
	store := &memCertStore{}

	certPEM, keyPEM, err := listenerTLSMaterial("web-cert", store, "127.0.0.1:3443")
	if err != nil {
		t.Fatalf("listenerTLSMaterial: %v", err)
	}
	if store.generated {
		t.Fatal("a configured certificate must not generate a self-signed certificate")
	}

	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("material is not a usable key pair: %v", err)
	}
	if len(pair.Certificate) != 2 {
		t.Fatalf("served chain length = %d, want 2 (leaf + intermediate)", len(pair.Certificate))
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if leaf.Subject.CommonName != "hub web leaf" {
		t.Fatalf("leaf CN = %q, want the PKI store leaf", leaf.Subject.CommonName)
	}
}

func TestStartWebServerFailsClosedOnBrokenReference(t *testing.T) {
	// VALIDATES: AC-3 and R-5 -- a configured-but-unresolvable reference is an
	// ERROR with no material, never a quiet downgrade to self-signed.
	// PREVENTS: the operator trap this spec exists to close -- a production
	// listener serving a self-signed certificate while the config names a real
	// one, visible only as a browser warning.
	loadWebPKIStore(t)

	for _, name := range []string{"typo-cert", "keyless"} {
		t.Run(name, func(t *testing.T) {
			store := &memCertStore{}
			certPEM, keyPEM, err := listenerTLSMaterial(name, store, "127.0.0.1:3443")
			if err == nil {
				t.Fatal("expected an error, got material")
			}
			if certPEM != nil || keyPEM != nil {
				t.Fatal("a failed reference must yield no material")
			}
			if store.generated {
				t.Fatal("a failed reference must NOT fall back to a self-signed certificate")
			}
			if !strings.Contains(err.Error(), name) {
				t.Fatalf("error %q does not name the reference", err)
			}
		})
	}
}

func TestWebServerSelfSignedFallbackUnchanged(t *testing.T) {
	// VALIDATES: AC-2 -- with no certificate configured, the existing
	// self-signed path is taken unchanged and its material is persisted, so an
	// operator who never touches the new leaf sees no behavior change.
	loadWebPKIStore(t)
	store := &memCertStore{}

	certPEM, keyPEM, err := listenerTLSMaterial("", store, "127.0.0.1:3443")
	if err != nil {
		t.Fatalf("listenerTLSMaterial: %v", err)
	}
	if !store.generated {
		t.Fatal("the self-signed path must generate and persist material")
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("self-signed certificate PEM did not decode")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if cert.Issuer.CommonName != cert.Subject.CommonName {
		t.Fatal("expected a self-signed certificate")
	}
	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		t.Fatalf("self-signed material is not a usable key pair: %v", err)
	}

	// A second call reuses the persisted pair rather than regenerating.
	store.generated = false
	if _, _, err := listenerTLSMaterial("", store, "127.0.0.1:3443"); err != nil {
		t.Fatalf("second listenerTLSMaterial: %v", err)
	}
	if store.generated {
		t.Fatal("existing stored material must be reused, not regenerated")
	}
}

func TestListenerMigratorUpdateWebCertificate(t *testing.T) {
	// VALIDATES: AC-9 -- the reload path reaches the running web server's
	// certificate through the always-on seam, so hub code rotates TLS material
	// without importing the compile-out-able web package.
	loadWebPKIStore(t)

	t.Run("rotates the configured certificate", func(t *testing.T) {
		fake := &fakeTLSUpdatable{}
		lm := &listenerMigrator{}
		lm.setWebTLS(fake)

		if err := lm.updateWebCertificate("web-cert"); err != nil {
			t.Fatalf("updateWebCertificate: %v", err)
		}
		if fake.calls != 1 {
			t.Fatalf("seam called %d times, want 1", fake.calls)
		}
		if _, err := tls.X509KeyPair(fake.certPEM, fake.keyPEM); err != nil {
			t.Fatalf("rotated material is not a usable key pair: %v", err)
		}
	})

	t.Run("a broken reference is an error and rotates nothing", func(t *testing.T) {
		fake := &fakeTLSUpdatable{}
		lm := &listenerMigrator{}
		lm.setWebTLS(fake)

		if err := lm.updateWebCertificate("typo-cert"); err == nil {
			t.Fatal("expected an error for an unresolvable reference")
		}
		if fake.calls != 0 {
			t.Fatal("nothing may be installed when the reference does not resolve")
		}
	})

	t.Run("no certificate configured leaves the self-signed cert alone", func(t *testing.T) {
		// The established pair is what a restart with this config serves, so it
		// is what the listener holds after the reload: the same bytes, loaded,
		// never regenerated.
		established := &memCertStore{}
		if _, _, err := listenerTLSMaterial("", established, "127.0.0.1:3443"); err != nil {
			t.Fatalf("establish the self-signed pair: %v", err)
		}
		established.generated = false
		fake := &fakeTLSUpdatable{certStore: established}
		lm := &listenerMigrator{}
		lm.setWebTLS(fake)

		if err := lm.updateWebCertificate(""); err != nil {
			t.Fatalf("updateWebCertificate: %v", err)
		}
		if !bytes.Equal(fake.certPEM, established.cert) {
			t.Fatal("an unset reference must serve the established self-signed certificate")
		}
		if established.generated {
			t.Fatal("an unset reference must not replace the established self-signed pair")
		}
	})

	t.Run("no web server running is not an error", func(t *testing.T) {
		lm := &listenerMigrator{}
		if err := lm.updateWebCertificate("web-cert"); err != nil {
			t.Fatalf("updateWebCertificate with no web server: %v", err)
		}
	})

	t.Run("a refused rotation surfaces", func(t *testing.T) {
		fake := &fakeTLSUpdatable{err: errors.New("refused")}
		lm := &listenerMigrator{}
		lm.setWebTLS(fake)

		if err := lm.updateWebCertificate("web-cert"); err == nil {
			t.Fatal("a refused rotation must surface to the reload")
		}
	})
}

func TestReloadDroppingWebCertificateNameServesSelfSigned(t *testing.T) {
	// VALIDATES: a reload whose config no longer names environment.web.certificate
	// installs the web server's self-signed pair, the certificate a restart with
	// that config would serve (docs/architecture/pki/tls-listeners.md, "a named
	// certificate fails closed" and "rotation does not rebind the hub listeners").
	// PREVENTS: the listener presenting the named chain the operator removed from
	// the config, which is what an empty name used to do: it touched nothing, so
	// a certificate dropped because its key leaked kept serving until a restart.
	loadWebPKIStore(t)

	established := &memCertStore{}
	if _, _, err := listenerTLSMaterial("", established, "127.0.0.1:3443"); err != nil {
		t.Fatalf("establish the self-signed pair: %v", err)
	}
	established.generated = false
	fake := &fakeTLSUpdatable{certStore: established}
	lm := &listenerMigrator{}
	lm.setWebTLS(fake)

	if err := lm.updateWebCertificate("web-cert"); err != nil {
		t.Fatalf("install the named certificate: %v", err)
	}
	if got := servedLeaf(t, fake.certPEM).Subject.CommonName; got != "hub web leaf" {
		t.Fatalf("before the name is dropped the listener holds %q, want the PKI store leaf", got)
	}

	if err := lm.updateWebCertificate(""); err != nil {
		t.Fatalf("dropping the certificate name must be accepted: %v", err)
	}
	if fake.calls != 2 {
		t.Fatalf("seam called %d times, want 2: dropping the name must install a certificate", fake.calls)
	}
	leaf := servedLeaf(t, fake.certPEM)
	if leaf.Subject.CommonName == "hub web leaf" {
		t.Fatal("the listener still holds the named chain the config dropped")
	}
	// The selfcert leaf is not a CA, so CheckSignatureFrom refuses it as a
	// parent. Self-signed is checked as its two facts: it names itself as
	// issuer, and its own key verifies its signature.
	if !bytes.Equal(leaf.RawIssuer, leaf.RawSubject) {
		t.Fatalf("the installed certificate names issuer %q, not itself", leaf.Issuer)
	}
	if err := leaf.CheckSignature(leaf.SignatureAlgorithm, leaf.RawTBSCertificate, leaf.Signature); err != nil {
		t.Fatalf("the installed certificate is not signed by its own key: %v", err)
	}
	if !bytes.Equal(fake.certPEM, established.cert) {
		t.Fatal("the installed certificate is not the established self-signed pair")
	}
	if established.generated {
		t.Fatal("the established self-signed pair must be reused, not regenerated")
	}
	if _, err := tls.X509KeyPair(fake.certPEM, fake.keyPEM); err != nil {
		t.Fatalf("installed material is not a usable key pair: %v", err)
	}
}

// servedLeaf parses the first CERTIFICATE block of the material handed to the
// rotation seam, which is the leaf the next handshake presents.
func servedLeaf(t *testing.T, certPEM []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("installed certificate PEM did not decode")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse installed leaf: %v", err)
	}
	return leaf
}
