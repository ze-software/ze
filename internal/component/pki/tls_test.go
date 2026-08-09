// Design: docs/architecture/pki/tls-listeners.md -- server TLS material tests

package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

// chainPKIConfig builds a store config holding a root CA, an intermediate it
// signed, and a device certificate the intermediate signed. notAfter bounds the
// leaf so expiry cases can be built from the same helper.
func chainPKIConfig(t *testing.T, notAfter time.Time) *PKIConfig {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(100),
		Subject:               pkix.Name{CommonName: "chain root"},
		NotBefore:             time.Now().Add(-2 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		SubjectKeyId:          []byte{1, 1, 1, 1},
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
		SerialNumber:          big.NewInt(200),
		Subject:               pkix.Name{CommonName: "chain intermediate"},
		NotBefore:             time.Now().Add(-2 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		SubjectKeyId:          []byte{2, 2, 2, 2},
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
		SerialNumber: big.NewInt(300),
		Subject:      pkix.Name{CommonName: "chain leaf"},
		NotBefore:    time.Now().Add(-2 * time.Hour),
		NotAfter:     notAfter,
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

	// A second leaf, issued directly by the root, with no intermediate (AC-11).
	directKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	directTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(400),
		Subject:      pkix.Name{CommonName: "direct leaf"},
		NotBefore:    time.Now().Add(-2 * time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	directDER, err := x509.CreateCertificate(rand.Reader, directTmpl, caCert, &directKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	directCert, err := x509.ParseCertificate(directDER)
	if err != nil {
		t.Fatal(err)
	}

	return &PKIConfig{
		CACerts: map[string]*CACertEntry{
			"chain-ca": {Name: "chain-ca", Certificate: caCert, Raw: caDER},
		},
		Certificates: map[string]*CertificateEntry{
			"web-cert": {
				Name:             "web-cert",
				Certificate:      devCert,
				Raw:              devDER,
				PrivateKey:       devKey,
				Intermediates:    []*x509.Certificate{interCert},
				RawIntermediates: [][]byte{interDER},
			},
			"direct-cert": {
				Name:        "direct-cert",
				Certificate: directCert,
				Raw:         directDER,
				PrivateKey:  directKey,
			},
			"keyless": {
				Name:             "keyless",
				Certificate:      devCert,
				Raw:              devDER,
				Intermediates:    []*x509.Certificate{interCert},
				RawIntermediates: [][]byte{interDER},
			},
		},
	}
}

func TestServerTLSMaterialAssemblesChain(t *testing.T) {
	// VALIDATES: AC-1/AC-4 -- the material a TLS listener serves carries the leaf
	// AND every intermediate, so a client can build the path to the trust anchor.
	// PREVENTS: serving a bare leaf, which browsers reject with
	// "unable to get local issuer certificate" whenever the intermediate is not
	// already cached.
	cfg := chainPKIConfig(t, time.Now().Add(365*24*time.Hour))
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(func() { _ = Load(nil) })

	certPEM, keyPEM, err := ServerTLSMaterial("web-cert")
	if err != nil {
		t.Fatalf("ServerTLSMaterial: %v", err)
	}

	if got := pemBlockCount(certPEM, "CERTIFICATE"); got != 2 {
		t.Fatalf("certificate PEM holds %d CERTIFICATE blocks, want 2 (leaf + intermediate)", got)
	}

	// The material must be directly loadable by the TLS stack, leaf first.
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("tls.X509KeyPair on the produced material: %v", err)
	}
	if len(pair.Certificate) != 2 {
		t.Fatalf("served chain length = %d, want 2", len(pair.Certificate))
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if leaf.Subject.CommonName != "chain leaf" {
		t.Fatalf("first served certificate CN = %q, want the leaf", leaf.Subject.CommonName)
	}
	inter, err := x509.ParseCertificate(pair.Certificate[1])
	if err != nil {
		t.Fatal(err)
	}
	if inter.Subject.CommonName != "chain intermediate" {
		t.Fatalf("second served certificate CN = %q, want the intermediate", inter.Subject.CommonName)
	}
}

func TestServerTLSMaterialLeafOnly(t *testing.T) {
	// VALIDATES: AC-11 -- an entry with no intermediate yields a single-block
	// certificate PEM and no error. A leaf issued directly by a stored root has
	// nothing to add.
	cfg := chainPKIConfig(t, time.Now().Add(365*24*time.Hour))
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(func() { _ = Load(nil) })

	certPEM, keyPEM, err := ServerTLSMaterial("direct-cert")
	if err != nil {
		t.Fatalf("ServerTLSMaterial: %v", err)
	}
	if got := pemBlockCount(certPEM, "CERTIFICATE"); got != 1 {
		t.Fatalf("certificate PEM holds %d CERTIFICATE blocks, want 1", got)
	}
	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		t.Fatalf("tls.X509KeyPair: %v", err)
	}
}

func TestServerTLSMaterialNotFound(t *testing.T) {
	// VALIDATES: AC-3/R-5 -- an unresolvable reference is an ERROR. The caller
	// must have nothing to serve, so it cannot quietly reach for a self-signed
	// certificate while the operator believes their chain is on the wire.
	cfg := chainPKIConfig(t, time.Now().Add(365*24*time.Hour))
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(func() { _ = Load(nil) })

	certPEM, keyPEM, err := ServerTLSMaterial("no-such-cert")
	if err == nil {
		t.Fatal("expected an error for an unknown certificate name")
	}
	if certPEM != nil || keyPEM != nil {
		t.Fatal("no material may be returned alongside an error")
	}
	if !strings.Contains(err.Error(), "no-such-cert") {
		t.Fatalf("error %q does not name the unresolved reference", err)
	}
	// The available names help the operator spot a typo.
	if !strings.Contains(err.Error(), "web-cert") {
		t.Fatalf("error %q does not list the available certificates", err)
	}
}

func TestServerTLSMaterialNoPrivateKey(t *testing.T) {
	// VALIDATES: AC-7 -- a store entry with no `private { key }` cannot serve
	// TLS. The pki block makes the key optional (a peer's certificate needs
	// none), so this is a reachable operator mistake, not a defensive branch.
	cfg := chainPKIConfig(t, time.Now().Add(365*24*time.Hour))
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(func() { _ = Load(nil) })

	_, _, err := ServerTLSMaterial("keyless")
	if err == nil {
		t.Fatal("expected an error for a certificate with no private key")
	}
	if !strings.Contains(err.Error(), "private key") {
		t.Fatalf("error %q does not explain the missing private key", err)
	}
}

func TestServerTLSMaterialEmptyName(t *testing.T) {
	// An empty name means "no reference configured" and must never reach this
	// loader; if it does, it is a wiring defect. Fail closed rather than return
	// empty material a caller could hand to a TLS listener.
	_, _, err := ServerTLSMaterial("")
	if err == nil {
		t.Fatal("expected an error for an empty certificate name")
	}
}

func TestCheckCertReferenceDiagnostics(t *testing.T) {
	// VALIDATES: AC-8/AC-11 -- the doctor helper classifies every way a
	// configured reference can be wrong, using distinct codes so `ze explain`
	// gives the operator the right fix.
	now := time.Now()
	healthy := chainPKIConfig(t, now.Add(365*24*time.Hour))

	t.Run("no reference configured is not a problem", func(t *testing.T) {
		if got := CheckCertReference(healthy, "", now); got != nil {
			t.Fatalf("unset reference produced %v, want no diagnostics", got)
		}
	})

	t.Run("healthy reference is clean", func(t *testing.T) {
		if got := CheckCertReference(healthy, "web-cert", now); len(got) != 0 {
			t.Fatalf("healthy reference produced %v, want none", got)
		}
	})

	t.Run("leaf-only reference issued by a stored root is clean", func(t *testing.T) {
		// AC-11: no intermediate, no incomplete-chain diagnostic.
		if got := CheckCertReference(healthy, "direct-cert", now); len(got) != 0 {
			t.Fatalf("direct-issued leaf produced %v, want none", got)
		}
	})

	t.Run("missing entry", func(t *testing.T) {
		got := CheckCertReference(healthy, "typo", now)
		requireProblem(t, got, CodeCertReference, "error", "typo")
	})

	t.Run("nil config with a reference set", func(t *testing.T) {
		// An operator who wrote `certificate lan` and no pki block at all.
		got := CheckCertReference(nil, "lan", now)
		requireProblem(t, got, CodeCertReference, "error", "lan")
	})

	t.Run("keyless entry", func(t *testing.T) {
		got := CheckCertReference(healthy, "keyless", now)
		requireProblem(t, got, CodeCertReference, "error", "private key")
	})

	t.Run("expired certificate", func(t *testing.T) {
		expired := chainPKIConfig(t, now.Add(-time.Hour))
		got := CheckCertReference(expired, "web-cert", now)
		requireProblem(t, got, CodeCertExpired, "error", "web-cert")
	})

	t.Run("certificate expiring inside the warning window", func(t *testing.T) {
		soon := chainPKIConfig(t, now.Add(10*24*time.Hour))
		got := CheckCertReference(soon, "web-cert", now)
		requireProblem(t, got, CodeCertExpired, "warning", "day")
	})

	t.Run("intermediate that does not issue the leaf", func(t *testing.T) {
		// AKI/SKI mismatch: the operator pasted the wrong intermediate, so the
		// served chain does not reach the trust anchor.
		broken := chainPKIConfig(t, now.Add(365*24*time.Hour))
		other := chainPKIConfig(t, now.Add(365*24*time.Hour))
		entry := broken.Certificates["web-cert"]
		otherInter := other.Certificates["web-cert"]
		entry.Intermediates = otherInter.Intermediates
		entry.RawIntermediates = otherInter.RawIntermediates

		got := CheckCertReference(broken, "web-cert", now)
		requireProblem(t, got, CodeCertReference, "error", "chain")
	})
}

func requireProblem(t *testing.T, got []CertProblem, code, severity, substr string) {
	t.Helper()
	for _, p := range got {
		if p.Code == code && p.Severity == severity && strings.Contains(p.Message, substr) {
			return
		}
	}
	t.Fatalf("no %s/%s diagnostic containing %q in %+v", code, severity, substr, got)
}

func pemBlockCount(data []byte, blockType string) int {
	count := 0
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return count
		}
		if block.Type == blockType {
			count++
		}
	}
}
