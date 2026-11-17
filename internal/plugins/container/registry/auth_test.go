// Design: docs/architecture/container/image-store.md -- the challenge and the token, proven against the test registry
// Related: auth.go -- the exchange under test

package registry

import (
	"crypto/x509"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/ociregistry"
)

// TestPullBearerTokenExchangeSendsCredentialsToRealmOnly proves the bearer
// flow: the credentials go to the realm, the token goes to the registry,
// and no registry request carries the credentials. Wrong credentials are
// refused by the realm, and the fetch fails as an authentication refusal.
func TestPullBearerTokenExchangeSendsCredentialsToRealmOnly(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthBearer, Username: "operator", Password: "s3cret-pass"})
	layer := []byte("private layer")
	_, layerDescriptor := putImage(t, r, "team/app", "1", layer)
	ref := mustReference(t, r.Host()+"/team/app:1")
	c := newClient(t, Config{Registry: r.Host(), Credentials: NewCredentials("operator", "s3cret-pass")})

	if _, err := c.Resolve(t.Context(), ref); err != nil {
		t.Fatal(err)
	}
	if _, err := fetchToFile(t, t.Context(), c, ref, blobOf(t, layerDescriptor)); err != nil {
		t.Fatal(err)
	}
	basic := "Basic " + base64.StdEncoding.EncodeToString([]byte("operator:s3cret-pass"))
	realmAsked, tokenSent := 0, 0
	for _, request := range r.Requests() {
		if request.Path == "/token" {
			realmAsked++
			if request.Authorization != basic {
				t.Errorf("the realm received %q, want the credentials", request.Authorization)
			}
			continue
		}
		if request.Authorization == basic {
			t.Errorf("the registry request %s carried the credentials", request.Path)
		}
		if request.Authorization == "Bearer "+r.Token() {
			tokenSent++
		}
	}
	if realmAsked == 0 {
		t.Error("the realm was never asked for a token")
	}
	if tokenSent == 0 {
		t.Error("no registry request carried the token")
	}

	wrong := newClient(t, Config{Registry: r.Host(), Credentials: NewCredentials("operator", "wrong")})
	_, err := wrong.Resolve(t.Context(), ref)
	checkFailure(t, err, FailureAuthRefused)
}

// TestPullBasicChallenge proves a basic challenge is answered with the
// credentials to the registry itself, and that a client without credentials
// is refused naming that the configuration holds none.
func TestPullBasicChallenge(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthBasic, Username: "operator", Password: "basic-pass"})
	_, layerDescriptor := putImage(t, r, "app", "1", []byte("basic layer"))
	ref := mustReference(t, r.Host()+"/app:1")
	c := newClient(t, Config{Registry: r.Host(), Credentials: NewCredentials("operator", "basic-pass")})

	if _, err := c.Resolve(t.Context(), ref); err != nil {
		t.Fatal(err)
	}
	if _, err := fetchToFile(t, t.Context(), c, ref, blobOf(t, layerDescriptor)); err != nil {
		t.Fatal(err)
	}

	anonymous := newClient(t, Config{Registry: r.Host()})
	_, err := anonymous.Resolve(t.Context(), ref)
	failure := requireFailure(t, err, FailureAuthRefused)
	if !strings.Contains(failure.Why(), "holds none") {
		t.Errorf("why = %q, want that the configuration holds no credentials", failure.Why())
	}
}

// TestTokenRealmOverPlainHTTPRefused proves a registry reached over HTTPS
// that names a plain HTTP realm is refused before the credentials leave,
// while the same plain realm is asked for a registry whose transport is HTTP.
func TestTokenRealmOverPlainHTTPRefused(t *testing.T) {
	secure := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthBearer, TLS: true, Realm: "http://127.0.0.1:1/token"})
	putImage(t, secure, "app", "1", []byte("tls layer"))
	anchor := x509.NewCertPool()
	anchor.AddCert(secure.Certificate())
	c := newClient(t, Config{Registry: secure.Host(), Transport: TransportHTTPS, TrustAnchor: anchor, Credentials: NewCredentials("operator", "pass")})
	_, err := c.Resolve(t.Context(), mustReference(t, secure.Host()+"/app:1"))
	failure := requireFailure(t, err, FailureAuthRefused)
	if !strings.Contains(failure.Why(), "without TLS") {
		t.Errorf("why = %q, want the plain realm named", failure.Why())
	}

	plain := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthBearer})
	putImage(t, plain, "app", "1", []byte("plain layer"))
	overHTTP := newClient(t, Config{Registry: plain.Host(), Transport: TransportHTTP})
	if _, err := overHTTP.Resolve(t.Context(), mustReference(t, plain.Host()+"/app:1")); err != nil {
		t.Errorf("a plain realm of an HTTP registry was refused: %v", err)
	}
}

// TestTokenAnswerSizeBound proves the bound of the realm's answer: 64 KiB is
// read and one byte more is refused.
func TestTokenAnswerSizeBound(t *testing.T) {
	for _, tc := range []struct {
		size int
		ok   bool
	}{{size: tokenAnswerSizeMax, ok: true}, {size: tokenAnswerSizeMax + 1, ok: false}} {
		r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthBearer, TokenAnswerSize: tc.size})
		putImage(t, r, "app", "1", []byte("layer"))
		c := newClient(t, Config{Registry: r.Host()})
		_, err := c.Resolve(t.Context(), mustReference(t, r.Host()+"/app:1"))
		if tc.ok {
			if err != nil {
				t.Errorf("a token answer of %d bytes was refused: %v", tc.size, err)
			}
			continue
		}
		checkFailure(t, err, FailureRefused)
	}
}
