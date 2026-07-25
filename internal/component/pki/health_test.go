package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
)

func testCertWithExpiry(t *testing.T, notAfter time.Time) (*x509.Certificate, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              notAfter,
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
	return cert, der
}

func TestCheckHealthNoCerts(t *testing.T) {
	current.Store(emptyState)
	status, reason := checkHealth()
	if status != health.StatusHealthy {
		t.Errorf("expected healthy, got %s: %s", status, reason)
	}
}

func TestCheckHealthValidCerts(t *testing.T) {
	cert, der := testCertWithExpiry(t, time.Now().Add(365*24*time.Hour))
	current.Store(&storeState{
		caCerts:      map[string]*CACertEntry{"test-ca": {Name: "test-ca", Certificate: cert, Raw: der}},
		certificates: make(map[string]*CertificateEntry),
	})

	status, _ := checkHealth()
	if status != health.StatusHealthy {
		t.Errorf("expected healthy, got %s", status)
	}
}

func TestCheckHealthExpiringCA(t *testing.T) {
	cert, der := testCertWithExpiry(t, time.Now().Add(15*24*time.Hour))
	current.Store(&storeState{
		caCerts:      map[string]*CACertEntry{"expiring-ca": {Name: "expiring-ca", Certificate: cert, Raw: der}},
		certificates: make(map[string]*CertificateEntry),
	})

	status, reason := checkHealth()
	if status != health.StatusDegraded {
		t.Errorf("expected degraded, got %s: %s", status, reason)
	}
}

func TestCheckHealthExpiredCA(t *testing.T) {
	cert, der := testCertWithExpiry(t, time.Now().Add(-time.Hour))
	current.Store(&storeState{
		caCerts:      map[string]*CACertEntry{"dead-ca": {Name: "dead-ca", Certificate: cert, Raw: der}},
		certificates: make(map[string]*CertificateEntry),
	})

	status, _ := checkHealth()
	if status != health.StatusDown {
		t.Errorf("expected down, got %s", status)
	}
}

func TestRaiseExpiryWarningsClears(t *testing.T) {
	report.ResetForTest()
	cert, der := testCertWithExpiry(t, time.Now().Add(365*24*time.Hour))
	current.Store(&storeState{
		caCerts:      map[string]*CACertEntry{"good-ca": {Name: "good-ca", Certificate: cert, Raw: der}},
		certificates: make(map[string]*CertificateEntry),
	})

	RaiseExpiryWarnings()
	warnings := report.Warnings()
	for _, w := range warnings {
		if w.Source == reportSource {
			t.Errorf("unexpected PKI warning for non-expiring cert: %s", w.Message)
		}
	}
}

func TestRaiseExpiryWarningsRaises(t *testing.T) {
	report.ResetForTest()
	cert, der := testCertWithExpiry(t, time.Now().Add(10*24*time.Hour))
	current.Store(&storeState{
		caCerts:      map[string]*CACertEntry{"soon-ca": {Name: "soon-ca", Certificate: cert, Raw: der}},
		certificates: make(map[string]*CertificateEntry),
	})

	RaiseExpiryWarnings()
	warnings := report.Warnings()
	found := false
	for _, w := range warnings {
		if w.Source == reportSource && w.Subject == "ca/soon-ca" {
			found = true
		}
	}
	if !found {
		t.Error("expected PKI expiry warning for soon-ca")
	}
}

func TestDaysUntil(t *testing.T) {
	now := time.Now()
	if d := daysUntil(now, now.Add(10*24*time.Hour)); d != 10 {
		t.Errorf("expected 10, got %d", d)
	}
	if d := daysUntil(now, now.Add(-time.Hour)); d != 0 {
		t.Errorf("expected 0 for past, got %d", d)
	}
}
