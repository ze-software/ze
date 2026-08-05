// Design: plan/spec-pki-full-chain.md -- DoT/DoH PKI certificate reference tests

package dnsserver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"log/slog"
	"math/big"
	"strings"
	"testing"
	"time"
)

// resolverChain builds the leaf+intermediate PEM and key PEM that a PKI
// resolver hands back, mirroring what pki.ServerTLSMaterial produces.
func resolverChain(t *testing.T, cn string) (chainPEM, keyPEM []byte) {
	t.Helper()

	interKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	interTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn + " intermediate"},
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
		Subject:      pkix.Name{CommonName: cn},
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
	keyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		t.Fatal(err)
	}

	chainPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	chainPEM = append(chainPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: interDER})...)
	return chainPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

func TestParseSecureLeavesCertificate(t *testing.T) {
	// VALIDATES: the `certificate` leaf of a tls container reaches
	// SecureConfig.Certificate, which is what selects the PKI path at apply.
	sc := DefaultSecureConfig()
	node := map[string]any{
		"tls": map[string]any{"enabled": "true", "certificate": "lan"},
	}
	if err := ParseSecureLeaves(node, &sc, "as112"); err != nil {
		t.Fatalf("ParseSecureLeaves: %v", err)
	}
	if sc.Certificate != "lan" {
		t.Fatalf("Certificate = %q, want %q", sc.Certificate, "lan")
	}
	if !sc.DoTEnabled {
		t.Fatal("the existing tls leaves must keep parsing")
	}
}

func TestParseSecureLeavesCertificateConflict(t *testing.T) {
	// VALIDATES: AC-6 -- `certificate` and cert-file/key-file are mutually
	// exclusive, and the conflict is a parse error the plugin verifier surfaces
	// as a rejected commit.
	// PREVENTS: two sources of TLS material in one container, where which one
	// wins is an implementation detail the operator cannot see.
	cases := []struct {
		name string
		tls  map[string]any
	}{
		{"with cert-file", map[string]any{"certificate": "lan", "cert-file": "/etc/ze/tls.pem"}},
		{"with key-file", map[string]any{"certificate": "lan", "key-file": "/etc/ze/tls.key"}},
		{"with both", map[string]any{"certificate": "lan", "cert-file": "/etc/ze/tls.pem", "key-file": "/etc/ze/tls.key"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sc := DefaultSecureConfig()
			err := ParseSecureLeaves(map[string]any{"tls": tc.tls}, &sc, "as112")
			if err == nil {
				t.Fatal("expected a mutual-exclusion error")
			}
			if !strings.Contains(err.Error(), "certificate") {
				t.Fatalf("error %q does not name the conflicting leaves", err)
			}
		})
	}

	t.Run("empty cert-file alongside certificate is fine", func(t *testing.T) {
		// An empty leaf is the same as an absent one; only a real value conflicts.
		sc := DefaultSecureConfig()
		node := map[string]any{"tls": map[string]any{"certificate": "lan", "cert-file": "", "key-file": ""}}
		if err := ParseSecureLeaves(node, &sc, "as112"); err != nil {
			t.Fatalf("ParseSecureLeaves: %v", err)
		}
	})
}

func TestBuildSecureTLSFromResolver(t *testing.T) {
	// VALIDATES: AC-5 -- a DoT/DoH listener with `tls { certificate <name> }`
	// serves the leaf AND its intermediate, resolved through the injected
	// resolver (the tier seam: core dnsserver never imports component pki).
	chainPEM, keyPEM := resolverChain(t, "dot leaf")

	var asked string
	m := New(slog.Default(), nil, Options{
		TLSMaterialResolver: func(name string) ([]byte, []byte, error) {
			asked = name
			return chainPEM, keyPEM, nil
		},
	})

	sc := DefaultSecureConfig()
	sc.DoTEnabled = true
	sc.Certificate = "lan"

	cfg, err := m.buildSecureTLS(sc, []string{"127.0.0.1"}, slog.Default())
	if err != nil {
		t.Fatalf("buildSecureTLS: %v", err)
	}
	if asked != "lan" {
		t.Fatalf("resolver asked for %q, want %q", asked, "lan")
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("Certificates = %d, want 1", len(cfg.Certificates))
	}
	if got := len(cfg.Certificates[0].Certificate); got != 2 {
		t.Fatalf("served chain length = %d, want 2 (leaf + intermediate)", got)
	}
	leaf, err := x509.ParseCertificate(cfg.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if leaf.Subject.CommonName != "dot leaf" {
		t.Fatalf("leaf CN = %q, want the resolved certificate", leaf.Subject.CommonName)
	}
}

func TestBuildSecureTLSResolverFailureIsLoud(t *testing.T) {
	// VALIDATES: AC-7 for DoT/DoH -- a reference that does not resolve produces
	// an ERROR, so ApplyWithSecure leaves the secure listeners unstarted and
	// logs it. The cleartext listeners are untouched, which is the contract this
	// surface already had for a broken cert-file.
	// PREVENTS: falling back to the ephemeral self-signed certificate, which
	// would leave DoT clients trusting nothing while the config named a real
	// certificate.
	m := New(slog.Default(), nil, Options{
		TLSMaterialResolver: func(string) ([]byte, []byte, error) {
			return nil, nil, errors.New("certificate lan not found")
		},
	})

	sc := DefaultSecureConfig()
	sc.DoTEnabled = true
	sc.Certificate = "lan"

	cfg, err := m.buildSecureTLS(sc, []string{"127.0.0.1"}, slog.Default())
	if err == nil {
		t.Fatal("expected an error when the reference does not resolve")
	}
	if cfg != nil {
		t.Fatal("no TLS config may be returned alongside an error")
	}
	if m.selfSigned != nil {
		t.Fatal("a failed reference must NOT fall back to the self-signed certificate")
	}
}

func TestBuildSecureTLSResolverMissing(t *testing.T) {
	// VALIDATES: a consumer that sets Certificate but injects no resolver fails
	// loudly. This guards a future DoT/DoH consumer that copies the config
	// plumbing and forgets the injection: without it, the reference would be
	// silently ignored and an ephemeral self-signed certificate served instead.
	m := New(slog.Default(), nil, Options{})

	sc := DefaultSecureConfig()
	sc.DoTEnabled = true
	sc.Certificate = "lan"

	cfg, err := m.buildSecureTLS(sc, []string{"127.0.0.1"}, slog.Default())
	if err == nil {
		t.Fatal("expected an error when no resolver is injected")
	}
	if cfg != nil {
		t.Fatal("no TLS config may be returned alongside an error")
	}
	if !strings.Contains(err.Error(), "lan") {
		t.Fatalf("error %q does not name the unresolvable reference", err)
	}
}

func TestBuildSecureTLSFileAndSelfSignedUnchanged(t *testing.T) {
	// VALIDATES: the existing DoT/DoH paths are untouched by the new branch.
	// With no certificate reference, an injected resolver must never be
	// consulted, and the self-signed fallback must still be cached.
	resolverCalled := false
	m := New(slog.Default(), nil, Options{
		TLSMaterialResolver: func(string) ([]byte, []byte, error) {
			resolverCalled = true
			return nil, nil, errors.New("must not be called")
		},
	})

	sc := DefaultSecureConfig()
	sc.DoTEnabled = true

	cfg, err := m.buildSecureTLS(sc, []string{"127.0.0.1"}, slog.Default())
	if err != nil {
		t.Fatalf("buildSecureTLS: %v", err)
	}
	if resolverCalled {
		t.Fatal("the resolver must not be consulted when no certificate is referenced")
	}
	if cfg == nil || m.selfSigned == nil {
		t.Fatal("the self-signed fallback must still be generated and cached")
	}

	again, err := m.buildSecureTLS(sc, []string{"127.0.0.1"}, slog.Default())
	if err != nil {
		t.Fatalf("second buildSecureTLS: %v", err)
	}
	if again != cfg {
		t.Fatal("the cached self-signed certificate must be reused, so a reload does not churn a rebind")
	}
}
