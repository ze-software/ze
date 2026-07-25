// Design: plan/learned/733-pki-store.md -- PKI config parser tests

package pki

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
)

func testCACertDER(t *testing.T) (any, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return key, der
}

func testDeviceCertDER(t *testing.T, caKey any, caDER []byte) (*ecdsa.PrivateKey, []byte) {
	t.Helper()
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "Test Device"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		DNSNames:     []string{"device.example.com"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	return key, der
}

func marshalKeyB64(t *testing.T, key any) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func makePKITree(t *testing.T, caB64, certB64, keyB64 string) *config.Tree {
	t.Helper()
	tree := config.NewTree()
	pkiContainer := tree.GetOrCreateContainer("pki")

	if caB64 != "" {
		caEntry := config.NewTree()
		caEntry.Set("certificate", caB64)
		pkiContainer.AddListEntry("ca", "test-ca", caEntry)
	}

	if certB64 != "" {
		certEntry := config.NewTree()
		certEntry.Set("certificate", certB64)
		if keyB64 != "" {
			privContainer := certEntry.GetOrCreateContainer("private")
			privContainer.Set("key", keyB64)
		}
		pkiContainer.AddListEntry("certificate", "dev-1", certEntry)
	}

	return tree
}

func TestParseCACertificate(t *testing.T) {
	_, caDER := testCACertDER(t)
	caB64 := base64.StdEncoding.EncodeToString(caDER)

	tree := makePKITree(t, caB64, "", "")

	cfg, err := ParseConfig(tree)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if len(cfg.CACerts) != 1 {
		t.Fatalf("expected 1 CA cert, got %d", len(cfg.CACerts))
	}

	ca, ok := cfg.CACerts["test-ca"]
	if !ok {
		t.Fatal("CA cert 'test-ca' not found")
	}
	if ca.Certificate.Subject.CommonName != "Test CA" {
		t.Errorf("expected subject CN 'Test CA', got %q", ca.Certificate.Subject.CommonName)
	}
	if !ca.Certificate.IsCA {
		t.Error("expected IsCA=true")
	}
}

func TestParseDeviceCertificate(t *testing.T) {
	caKey, caDER := testCACertDER(t)
	devKey, devDER := testDeviceCertDER(t, caKey, caDER)
	keyB64 := marshalKeyB64(t, devKey)
	caB64 := base64.StdEncoding.EncodeToString(caDER)
	devB64 := base64.StdEncoding.EncodeToString(devDER)

	tree := makePKITree(t, caB64, devB64, keyB64)

	cfg, err := ParseConfig(tree)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if len(cfg.Certificates) != 1 {
		t.Fatalf("expected 1 certificate, got %d", len(cfg.Certificates))
	}

	cert, ok := cfg.Certificates["dev-1"]
	if !ok {
		t.Fatal("certificate 'dev-1' not found")
	}
	if cert.Certificate.Subject.CommonName != "Test Device" {
		t.Errorf("expected subject CN 'Test Device', got %q", cert.Certificate.Subject.CommonName)
	}
	if cert.PrivateKey == nil {
		t.Error("expected private key to be parsed")
	}
}

func TestParsePrivateKeyECDSA(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	b64 := marshalKeyB64(t, key)
	parsed, pErr := parsePrivateKey(b64)
	if pErr != nil {
		t.Fatalf("parsePrivateKey ECDSA: %v", pErr)
	}
	if _, ok := parsed.(*ecdsa.PrivateKey); !ok {
		t.Errorf("expected *ecdsa.PrivateKey, got %T", parsed)
	}
}

func TestParsePrivateKeyRSA(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	b64 := marshalKeyB64(t, key)
	parsed, pErr := parsePrivateKey(b64)
	if pErr != nil {
		t.Fatalf("parsePrivateKey RSA: %v", pErr)
	}
	if _, ok := parsed.(*rsa.PrivateKey); !ok {
		t.Errorf("expected *rsa.PrivateKey, got %T", parsed)
	}
}

func TestParsePrivateKeyEd25519(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	b64 := marshalKeyB64(t, priv)
	parsed, pErr := parsePrivateKey(b64)
	if pErr != nil {
		t.Fatalf("parsePrivateKey Ed25519: %v", pErr)
	}
	if _, ok := parsed.(ed25519.PrivateKey); !ok {
		t.Errorf("expected ed25519.PrivateKey, got %T", parsed)
	}
}

func TestKeyMismatch(t *testing.T) {
	caKey, caDER := testCACertDER(t)
	_, devDER := testDeviceCertDER(t, caKey, caDER)

	wrongKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	wrongKeyB64 := marshalKeyB64(t, wrongKey)
	devB64 := base64.StdEncoding.EncodeToString(devDER)

	tree := config.NewTree()
	pkiContainer := tree.GetOrCreateContainer("pki")
	certEntry := config.NewTree()
	certEntry.Set("certificate", devB64)
	privContainer := certEntry.GetOrCreateContainer("private")
	privContainer.Set("key", wrongKeyB64)
	pkiContainer.AddListEntry("certificate", "dev-1", certEntry)

	_, err = ParseConfig(tree)
	if err == nil {
		t.Fatal("expected key mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Errorf("expected key mismatch error, got: %v", err)
	}
}

func TestParseCACertEmpty(t *testing.T) {
	tree := config.NewTree()
	pkiContainer := tree.GetOrCreateContainer("pki")
	caEntry := config.NewTree()
	caEntry.Set("certificate", "")
	pkiContainer.AddListEntry("ca", "empty-ca", caEntry)

	_, err := ParseConfig(tree)
	if err == nil {
		t.Fatal("expected error for empty certificate")
	}
}

func TestParseCACertInvalidBase64(t *testing.T) {
	tree := config.NewTree()
	pkiContainer := tree.GetOrCreateContainer("pki")
	caEntry := config.NewTree()
	caEntry.Set("certificate", "not-valid-base64!!!")
	pkiContainer.AddListEntry("ca", "bad-ca", caEntry)

	_, err := ParseConfig(tree)
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestNameValidationPathTraversal(t *testing.T) {
	_, caDER := testCACertDER(t)
	caB64 := base64.StdEncoding.EncodeToString(caDER)

	tree := config.NewTree()
	pkiContainer := tree.GetOrCreateContainer("pki")
	caEntry := config.NewTree()
	caEntry.Set("certificate", caB64)
	pkiContainer.AddListEntry("ca", "../../etc/shadow", caEntry)

	_, err := ParseConfig(tree)
	if err == nil {
		t.Fatal("expected error for path-traversal name")
	}
	if !strings.Contains(err.Error(), "invalid characters") {
		t.Errorf("expected invalid characters error, got: %v", err)
	}
}

func TestNameValidationTooLong(t *testing.T) {
	_, caDER := testCACertDER(t)
	caB64 := base64.StdEncoding.EncodeToString(caDER)

	longName := strings.Repeat("a", 256)
	tree := config.NewTree()
	pkiContainer := tree.GetOrCreateContainer("pki")
	caEntry := config.NewTree()
	caEntry.Set("certificate", caB64)
	pkiContainer.AddListEntry("ca", longName, caEntry)

	_, err := ParseConfig(tree)
	if err == nil {
		t.Fatal("expected error for too-long name")
	}
	if !strings.Contains(err.Error(), "255") {
		t.Errorf("expected length error, got: %v", err)
	}
}

func TestNameValidationAcceptsValid(t *testing.T) {
	_, caDER := testCACertDER(t)
	caB64 := base64.StdEncoding.EncodeToString(caDER)

	for _, name := range []string{"exa-cpe-ca", "EXAFO000000400", "my.cert.v2", "under_score"} {
		tree := makePKITree(t, caB64, "", "")
		pkiContainer := tree.GetContainer("pki")
		if pkiContainer == nil {
			tree = config.NewTree()
			pkiContainer = tree.GetOrCreateContainer("pki")
		}
		caEntry := config.NewTree()
		caEntry.Set("certificate", caB64)
		pkiContainer.AddListEntry("ca", name, caEntry)

		cfg, err := ParseConfig(tree)
		if err != nil {
			t.Errorf("name %q should be valid, got: %v", name, err)
			continue
		}
		if _, ok := cfg.CACerts[name]; !ok {
			t.Errorf("name %q not in parsed CACerts", name)
		}
	}
}

func TestParseNoPKIBlock(t *testing.T) {
	tree := config.NewTree()
	cfg, err := ParseConfig(tree)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.CACerts) != 0 || len(cfg.Certificates) != 0 {
		t.Error("expected empty config for tree without pki block")
	}
}

func TestParseNilTree(t *testing.T) {
	cfg, err := ParseConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.CACerts) != 0 || len(cfg.Certificates) != 0 {
		t.Error("expected empty config for nil tree")
	}
}
