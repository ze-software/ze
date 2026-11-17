// Design: docs/architecture/container/image-store.md -- the transport and the trust anchor, proven against the test registry
// Related: client.go -- New and transportFailure under test

package registry

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

	"github.com/ze-software/ze/internal/test/ociregistry"
)

// TestHTTPTransportFetchesOverPlainHTTP proves a registry the operator gave
// the HTTP transport is fetched from over plain HTTP.
func TestHTTPTransportFetchesOverPlainHTTP(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	putImage(t, r, "app", "1", []byte("plain layer"))
	c := newClient(t, Config{Registry: r.Host(), Transport: TransportHTTP})
	if _, err := c.Resolve(t.Context(), mustReference(t, r.Host()+"/app:1")); err != nil {
		t.Fatal(err)
	}
}

// TestHTTPSTransportRefusesPlainHTTP proves the same plain server under the
// HTTPS transport is refused, naming that it answered without TLS.
func TestHTTPSTransportRefusesPlainHTTP(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	putImage(t, r, "app", "1", []byte("plain layer"))
	c := newClient(t, Config{Registry: r.Host(), Transport: TransportHTTPS})
	_, err := c.Resolve(t.Context(), mustReference(t, r.Host()+"/app:1"))
	failure := requireFailure(t, err, FailureUnreachable)
	if !strings.Contains(failure.Why(), "without TLS") {
		t.Errorf("why = %q, want the missing TLS named", failure.Why())
	}
	if n := len(r.Requests()); n != 0 {
		t.Errorf("the plain server answered %d requests under the HTTPS transport", n)
	}
}

// TestTrustAnchorReplacesSystemRoots proves a registry whose certificate the
// named anchor signed is trusted, that an anchor which did not sign it is
// refused even though the certificate is otherwise valid, and that without
// an anchor the system roots decide, which do not hold the test authority.
func TestTrustAnchorReplacesSystemRoots(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone, TLS: true})
	putImage(t, r, "app", "1", []byte("tls layer"))
	ref := mustReference(t, r.Host()+"/app:1")

	signer := x509.NewCertPool()
	signer.AddCert(r.Certificate())
	trusted := newClient(t, Config{Registry: r.Host(), Transport: TransportHTTPS, TrustAnchor: signer})
	if _, err := trusted.Resolve(t.Context(), ref); err != nil {
		t.Fatalf("the registry signed by the anchor was refused: %v", err)
	}

	another := x509.NewCertPool()
	another.AddCert(otherAuthority(t))
	for name, anchor := range map[string]*x509.CertPool{"another anchor": another, "system roots": nil} {
		c := newClient(t, Config{Registry: r.Host(), Transport: TransportHTTPS, TrustAnchor: anchor})
		_, err := c.Resolve(t.Context(), ref)
		failure := requireFailure(t, err, FailureUnreachable)
		if !strings.Contains(failure.Why(), "trusted authority") {
			t.Errorf("%s: why = %q, want the untrusted certificate named", name, failure.Why())
		}
	}
}

// otherAuthority makes a self-signed authority that signed nothing the test
// registry serves.
func otherAuthority(t *testing.T) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "another authority"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return authority
}
