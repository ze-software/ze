// Design: plan/spec-ipsec-1-pki-store.md -- PKI store tests

package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testPKIConfig(t *testing.T) *PKIConfig {
	t.Helper()
	caKey, caDER := testCACertDER(t)
	devKey, devDER := testDeviceCertDER(t, caKey, caDER)

	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	devCert, err := x509.ParseCertificate(devDER)
	if err != nil {
		t.Fatal(err)
	}

	return &PKIConfig{
		CACerts: map[string]*CACertEntry{
			"test-ca": {Name: "test-ca", Certificate: caCert, Raw: caDER},
		},
		Certificates: map[string]*CertificateEntry{
			"dev-1": {Name: "dev-1", Certificate: devCert, Raw: devDER, PrivateKey: devKey},
		},
	}
}

func TestLoadAndGetCA(t *testing.T) {
	cfg := testPKIConfig(t)
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	ca := GetCA("test-ca")
	if ca == nil {
		t.Fatal("GetCA returned nil for 'test-ca'")
	}
	if ca.Certificate.Subject.CommonName != "Test CA" {
		t.Errorf("expected CN 'Test CA', got %q", ca.Certificate.Subject.CommonName)
	}

	if GetCA("nonexistent") != nil {
		t.Error("expected nil for nonexistent CA")
	}
}

func TestLoadAndGetCertificate(t *testing.T) {
	cfg := testPKIConfig(t)
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	cert := GetCertificate("dev-1")
	if cert == nil {
		t.Fatal("GetCertificate returned nil for 'dev-1'")
	}
	if cert.Certificate.Subject.CommonName != "Test Device" {
		t.Errorf("expected CN 'Test Device', got %q", cert.Certificate.Subject.CommonName)
	}
	if cert.PrivateKey == nil {
		t.Error("expected private key")
	}

	if GetCertificate("nonexistent") != nil {
		t.Error("expected nil for nonexistent certificate")
	}
}

func TestCAPool(t *testing.T) {
	cfg := testPKIConfig(t)
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	pool := CAPool()
	if pool == nil {
		t.Fatal("CAPool returned nil")
	}

	cert := GetCertificate("dev-1")
	_, err := cert.Certificate.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	if err != nil {
		t.Errorf("chain validation failed with CA pool: %v", err)
	}
}

func TestChainValidation(t *testing.T) {
	cfg := testPKIConfig(t)
	err := Load(cfg)
	if err != nil {
		t.Fatalf("Load should succeed for valid chain: %v", err)
	}
}

func TestChainValidationFails(t *testing.T) {
	otherKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherCA := &x509.Certificate{
		SerialNumber:          big.NewInt(99),
		Subject:               pkix.Name{CommonName: "Other CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	otherCaDER, err := x509.CreateCertificate(rand.Reader, otherCA, otherCA, &otherKey.PublicKey, otherKey)
	if err != nil {
		t.Fatal(err)
	}
	otherCaCert, err := x509.ParseCertificate(otherCaDER)
	if err != nil {
		t.Fatal(err)
	}

	caKey, caDER := testCACertDER(t)
	_, devDER := testDeviceCertDER(t, caKey, caDER)
	devCert, err := x509.ParseCertificate(devDER)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &PKIConfig{
		CACerts: map[string]*CACertEntry{
			"other-ca": {Name: "other-ca", Certificate: otherCaCert, Raw: otherCaDER},
		},
		Certificates: map[string]*CertificateEntry{
			"dev-1": {Name: "dev-1", Certificate: devCert, Raw: devDER},
		},
	}

	err = Load(cfg)
	if err == nil {
		t.Fatal("expected chain validation failure")
	}
	if !strings.Contains(err.Error(), "chain validation") {
		t.Errorf("expected chain validation error, got: %v", err)
	}
}

func TestExpiredCertRejected(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Expired CA"},
		NotBefore:             time.Now().Add(-48 * time.Hour),
		NotAfter:              time.Now().Add(-time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &PKIConfig{
		CACerts: map[string]*CACertEntry{
			"expired-ca": {Name: "expired-ca", Certificate: cert, Raw: der},
		},
		Certificates: make(map[string]*CertificateEntry),
	}

	err = Load(cfg)
	if err == nil {
		t.Fatal("expected expired cert error")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected expired error, got: %v", err)
	}
}

func TestStoreAtomicSwap(t *testing.T) {
	cfg1 := testPKIConfig(t)
	if err := Load(cfg1); err != nil {
		t.Fatalf("Load cfg1: %v", err)
	}

	ca1 := GetCA("test-ca")
	if ca1 == nil {
		t.Fatal("expected test-ca after first load")
	}

	caKey2, caDER2 := testCACertDER(t)
	_ = caKey2
	caCert2, err := x509.ParseCertificate(caDER2)
	if err != nil {
		t.Fatal(err)
	}

	cfg2 := &PKIConfig{
		CACerts: map[string]*CACertEntry{
			"new-ca": {Name: "new-ca", Certificate: caCert2, Raw: caDER2},
		},
		Certificates: make(map[string]*CertificateEntry),
	}
	if err := Load(cfg2); err != nil {
		t.Fatalf("Load cfg2: %v", err)
	}

	if GetCA("test-ca") != nil {
		t.Error("test-ca should be gone after swap")
	}
	if GetCA("new-ca") == nil {
		t.Error("new-ca should exist after swap")
	}
}

func TestLoadNil(t *testing.T) {
	if err := Load(nil); err != nil {
		t.Fatalf("Load(nil): %v", err)
	}
	if GetCA("anything") != nil {
		t.Error("expected nil after loading nil config")
	}
}

func TestListCACerts(t *testing.T) {
	cfg := testPKIConfig(t)
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	cas := ListCACerts()
	if len(cas) != 1 {
		t.Fatalf("expected 1 CA, got %d", len(cas))
	}
	if cas[0].Name != "test-ca" {
		t.Errorf("expected 'test-ca', got %q", cas[0].Name)
	}
}

func TestListCertificates(t *testing.T) {
	cfg := testPKIConfig(t)
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	certs := ListCertificates()
	if len(certs) != 1 {
		t.Fatalf("expected 1 certificate, got %d", len(certs))
	}
	if certs[0].Name != "dev-1" {
		t.Errorf("expected 'dev-1', got %q", certs[0].Name)
	}
}

func TestExportPEM(t *testing.T) {
	cfg := testPKIConfig(t)
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	certPath, keyPath, caPath, err := ExportPEM("dev-1")
	if err != nil {
		t.Fatalf("ExportPEM: %v", err)
	}
	t.Cleanup(func() {
		if cErr := CleanupPEM("dev-1"); cErr != nil {
			t.Logf("CleanupPEM: %v", cErr)
		}
	})

	if _, sErr := os.Stat(certPath); sErr != nil {
		t.Errorf("cert file not created: %v", sErr)
	}
	if _, sErr := os.Stat(keyPath); sErr != nil {
		t.Errorf("key file not created: %v", sErr)
	}

	keyInfo, sErr := os.Stat(keyPath)
	if sErr == nil && keyInfo.Mode().Perm() != 0o600 {
		t.Errorf("key file permissions: want 0600, got %o", keyInfo.Mode().Perm())
	}

	if caPath != "" {
		if _, sErr := os.Stat(caPath); sErr != nil {
			t.Errorf("ca file not created: %v", sErr)
		}
	}

	certData, rErr := os.ReadFile(certPath)
	if rErr != nil {
		t.Fatalf("read cert: %v", rErr)
	}
	if !strings.Contains(string(certData), "BEGIN CERTIFICATE") {
		t.Error("cert file missing PEM header")
	}
}

func TestExportPEMNotFound(t *testing.T) {
	if err := Load(nil); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := ExportPEM("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent cert")
	}
}

func TestExportPEMCleanup(t *testing.T) {
	cfg := testPKIConfig(t)
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	certPath, _, _, err := ExportPEM("dev-1")
	if err != nil {
		t.Fatalf("ExportPEM: %v", err)
	}

	if cErr := CleanupPEM("dev-1"); cErr != nil {
		t.Fatalf("CleanupPEM: %v", cErr)
	}

	if _, sErr := os.Stat(certPath); sErr == nil {
		t.Error("cert file should be removed after cleanup")
	}

	caFiles, _ := filepath.Glob(filepath.Join(exportDir, "ca-*.pem"))
	if len(caFiles) > 0 {
		t.Errorf("CA files should be removed after cleanup, found: %v", caFiles)
	}
}

func TestIntermediatePool(t *testing.T) {
	cfg := testPKIConfig(t)
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}
	pool := IntermediatePool()
	if pool == nil {
		t.Fatal("IntermediatePool returned nil")
	}
}

func TestExportPEMWithIntermediate(t *testing.T) {
	caKey, caDER := testCACertDER(t)
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}

	interKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	interTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(10),
		Subject:               pkix.Name{CommonName: "Intermediate CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
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
		SerialNumber: big.NewInt(20),
		Subject:      pkix.Name{CommonName: "Device With Intermediate"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	devDER, err := x509.CreateCertificate(rand.Reader, devTmpl, interCert, &devKey.PublicKey, interKey)
	if err != nil {
		t.Fatal(err)
	}
	devCert, err := x509.ParseCertificate(devDER)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &PKIConfig{
		CACerts: map[string]*CACertEntry{
			"test-ca": {Name: "test-ca", Certificate: caCert, Raw: caDER},
		},
		Certificates: map[string]*CertificateEntry{
			"dev-inter": {
				Name:         "dev-inter",
				Certificate:  devCert,
				Raw:          devDER,
				PrivateKey:   devKey,
				Intermediate: interCert,
				RawInter:     interDER,
			},
		},
	}

	if lErr := Load(cfg); lErr != nil {
		t.Fatalf("Load: %v", lErr)
	}

	certPath, _, _, eErr := ExportPEM("dev-inter")
	if eErr != nil {
		t.Fatalf("ExportPEM: %v", eErr)
	}
	t.Cleanup(func() {
		if cErr := CleanupPEM("dev-inter"); cErr != nil {
			t.Logf("CleanupPEM: %v", cErr)
		}
	})

	certData, rErr := os.ReadFile(certPath)
	if rErr != nil {
		t.Fatalf("read cert: %v", rErr)
	}
	count := strings.Count(string(certData), "BEGIN CERTIFICATE")
	if count != 2 {
		t.Errorf("expected 2 PEM blocks (cert + intermediate), got %d", count)
	}
}

func TestShowPKICertificates(t *testing.T) {
	cfg := testPKIConfig(t)
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	resp, err := handleShowPKICertificates(nil, nil)
	if err != nil {
		t.Fatalf("handleShowPKICertificates: %v", err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", resp.Data)
	}
	count, ok := data["count"].(int)
	if !ok || count != 2 {
		t.Errorf("expected count=2, got %v", data["count"])
	}
}

func TestShowPKICertificateByName(t *testing.T) {
	cfg := testPKIConfig(t)
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	resp, err := handleShowPKICertificate(nil, []string{"dev-1"})
	if err != nil {
		t.Fatalf("handleShowPKICertificate: %v", err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", resp.Data)
	}
	if data["name"] != "dev-1" {
		t.Errorf("expected name=dev-1, got %v", data["name"])
	}
	if data["has-private-key"] != true {
		t.Error("expected has-private-key=true")
	}
}

func TestShowPKICertificateNotFound(t *testing.T) {
	cfg := testPKIConfig(t)
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	resp, err := handleShowPKICertificate(nil, []string{"nonexistent"})
	if err != nil {
		t.Fatalf("handleShowPKICertificate: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("expected error status, got %q", resp.Status)
	}
}

func TestShowPKICertificateNoArgs(t *testing.T) {
	resp, err := handleShowPKICertificate(nil, nil)
	if err != nil {
		t.Fatalf("handleShowPKICertificate: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("expected error status, got %q", resp.Status)
	}
}

func TestCertCN(t *testing.T) {
	cfg := testPKIConfig(t)
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	cn := CertCN("dev-1")
	if cn != "Test Device" {
		t.Errorf("expected 'Test Device', got %q", cn)
	}
	if CertCN("nonexistent") != "" {
		t.Error("expected empty string for nonexistent cert")
	}
}
