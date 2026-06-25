package selfcert

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

type memoryCertStore struct {
	cert []byte
	key  []byte
}

func (s *memoryCertStore) ReadCert() ([]byte, error) { return s.cert, nil }
func (s *memoryCertStore) ReadKey() ([]byte, error)  { return s.key, nil }
func (s *memoryCertStore) WriteCert(data []byte) error {
	s.cert = append([]byte(nil), data...)
	return nil
}
func (s *memoryCertStore) WriteKey(data []byte) error {
	s.key = append([]byte(nil), data...)
	return nil
}
func (s *memoryCertStore) Exists() bool { return len(s.cert) > 0 && len(s.key) > 0 }

func parseCertForTest(t *testing.T, certPEM []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("certificate PEM did not decode")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return cert
}

func TestSelfCert_GenerateAndLoad(t *testing.T) {
	store := &memoryCertStore{}

	// VALIDATES: selfcert generates PEM material, stores it, and loads the same material on the next call.
	// PREVENTS: web compile-out leaving install/lg cert bootstrap pinned to internal/component/web.
	certPEM, keyPEM, err := LoadOrGenerateCert(store, "127.0.0.1:8443")
	if err != nil {
		t.Fatalf("LoadOrGenerateCert generate: %v", err)
	}
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		t.Fatalf("generated certificate or key is empty")
	}
	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		t.Fatalf("generated pair is not usable by TLS: %v", err)
	}

	loadedCert, loadedKey, err := LoadOrGenerateCert(store, "127.0.0.1:9443")
	if err != nil {
		t.Fatalf("LoadOrGenerateCert load: %v", err)
	}
	if !bytes.Equal(loadedCert, certPEM) || !bytes.Equal(loadedKey, keyPEM) {
		t.Fatal("stored certificate material changed on load")
	}
}

func TestSelfCert_SANsForListenAddr(t *testing.T) {
	// VALIDATES: SANs include localhost, loopback, listen address, and explicit DNS/IP names.
	// PREVENTS: first-boot HTTPS certs losing browser-valid SANs during the package move.
	certPEM, _, err := GenerateWebCertWithNames("192.0.2.10:8443", []string{"router.local", "198.51.100.7"}, 0)
	if err != nil {
		t.Fatalf("GenerateWebCertWithNames: %v", err)
	}
	cert := parseCertForTest(t, certPEM)

	wantDNS := map[string]bool{"localhost": false, "router.local": false}
	for _, name := range cert.DNSNames {
		if _, ok := wantDNS[name]; ok {
			wantDNS[name] = true
		}
	}
	for name, found := range wantDNS {
		if !found {
			t.Fatalf("missing DNS SAN %s in %v", name, cert.DNSNames)
		}
	}

	wantIP := map[string]bool{"127.0.0.1": false, "::1": false, "192.0.2.10": false, "198.51.100.7": false}
	for _, ip := range cert.IPAddresses {
		if _, ok := wantIP[ip.String()]; ok {
			wantIP[ip.String()] = true
		}
	}
	for ip, found := range wantIP {
		if !found {
			t.Fatalf("missing IP SAN %s in %v", ip, cert.IPAddresses)
		}
	}
}

func TestSelfCert_NewTLSConfig(t *testing.T) {
	certPEM, keyPEM, err := GenerateWebCertWithAddr("")
	if err != nil {
		t.Fatalf("GenerateWebCert: %v", err)
	}

	// VALIDATES: selfcert TLS config keeps TLS 1.2 minimum and rejects invalid PEM.
	// PREVENTS: extracting cert helpers weakening the HTTPS transport policy.
	cfg, err := NewTLSConfig(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("NewTLSConfig valid material: %v", err)
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %x, want TLS 1.2", cfg.MinVersion)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("certificates = %d, want 1", len(cfg.Certificates))
	}
	if _, err := NewTLSConfig([]byte("bad cert"), []byte("bad key")); err == nil {
		t.Fatal("NewTLSConfig accepted invalid PEM")
	}
}
