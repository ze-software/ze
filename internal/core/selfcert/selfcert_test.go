package selfcert

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
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

// TestNewTLSConfigServesChain validates the stdlib behavior the served chain
// rests on (docs/architecture/pki/tls-listeners.md, "the leaf comes first, then
// the intermediates"): tls.X509KeyPair parses EVERY CERTIFICATE block in the
// certificate PEM into tls.Certificate.Certificate, so a leaf+intermediate
// concatenation already serves the full chain and selfcert needs no change.
//
// PREVENTS: building the PKI chain feature on an unverified stdlib belief. If
// this ever regressed, pki.ServerTLSMaterial would hand over a correct
// two-block PEM and the listener would still send only the leaf.
func TestNewTLSConfigServesChain(t *testing.T) {
	leafPEM, keyPEM, interPEM := chainMaterial(t)

	chainPEM := append(append([]byte(nil), leafPEM...), interPEM...)
	cfg, err := NewTLSConfig(chainPEM, keyPEM)
	if err != nil {
		t.Fatalf("NewTLSConfig: %v", err)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("Certificates = %d, want 1", len(cfg.Certificates))
	}
	if got := len(cfg.Certificates[0].Certificate); got != 2 {
		t.Fatalf("served chain length = %d, want 2 (leaf + intermediate)", got)
	}

	// The leaf must be first: TLS requires the sender's certificate at index 0.
	leafBlock, _ := pem.Decode(leafPEM)
	if !bytes.Equal(cfg.Certificates[0].Certificate[0], leafBlock.Bytes) {
		t.Fatal("first served certificate is not the leaf")
	}
	interBlock, _ := pem.Decode(interPEM)
	if !bytes.Equal(cfg.Certificates[0].Certificate[1], interBlock.Bytes) {
		t.Fatal("second served certificate is not the intermediate")
	}

	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %d, want TLS 1.2", cfg.MinVersion)
	}
}

// chainMaterial builds a self-signed intermediate CA and a leaf certificate it
// issued, returning the leaf PEM, the leaf key PEM, and the intermediate PEM.
func chainMaterial(t *testing.T) (leafPEM, keyPEM, interPEM []byte) {
	t.Helper()

	interKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	interTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "chain-test intermediate"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	interDER, err := x509.CreateCertificate(rand.Reader, interTmpl, interTmpl, &interKey.PublicKey, interKey)
	if err != nil {
		t.Fatal(err)
	}
	interCert, err := x509.ParseCertificate(interDER)
	if err != nil {
		t.Fatal(err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "chain-test leaf"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, interCert, &leafKey.PublicKey, interKey)
	if err != nil {
		t.Fatal(err)
	}

	leafKeyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		t.Fatal(err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: leafKeyDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: interDER})
}
