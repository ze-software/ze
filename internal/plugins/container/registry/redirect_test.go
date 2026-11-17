// Design: docs/architecture/container/image-store.md -- the redirect rule, proven against the test registry
// Related: client.go -- follow, the rule under test

package registry

import (
	"bytes"
	"crypto/x509"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/ociregistry"
)

// TestRedirectToOtherHostDropsAuthorization proves a blob redirect to
// another host and port is followed and carries no authorization header,
// while the registry itself received the token.
func TestRedirectToOtherHostDropsAuthorization(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthBearer})
	other := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	layer := []byte("redirected layer")
	_, layerDescriptor := putImage(t, r, "app", "1", layer)
	other.PutBlob(ociregistry.MediaOCILayerGzip, layer)
	r.SetFault(layerDescriptor.Digest, ociregistry.Fault{RedirectTo: other.BlobURL("app", layerDescriptor.Digest)})
	c := newClient(t, Config{Registry: r.Host()})

	content, err := fetchToFile(t, t.Context(), c, mustReference(t, r.Host()+"/app:1"), blobOf(t, layerDescriptor))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, layer) {
		t.Errorf("blob = %q, want %q", content, layer)
	}
	if len(other.Requests()) == 0 {
		t.Fatal("the redirect target was never asked")
	}
	for _, request := range other.Requests() {
		if request.Authorization != "" {
			t.Errorf("the other host received the authorization %q", request.Authorization)
		}
	}
	tokenSeen := false
	for _, request := range r.Requests() {
		tokenSeen = tokenSeen || request.Authorization == "Bearer "+r.Token()
	}
	if !tokenSeen {
		t.Error("the registry itself never received the token, so the test proves nothing")
	}
}

// TestRedirectSameHostKeepsAuthorization proves a redirect to the same host
// and port keeps the header: the registry demands the token at every hop,
// so the fetch succeeds only if each hop carried it.
func TestRedirectSameHostKeepsAuthorization(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthBearer})
	layer := []byte("hopping layer")
	_, layerDescriptor := putImage(t, r, "app", "1", layer)
	r.SetFault(layerDescriptor.Digest, ociregistry.Fault{RedirectHops: 2})
	c := newClient(t, Config{Registry: r.Host()})

	if _, err := fetchToFile(t, t.Context(), c, mustReference(t, r.Host()+"/app:1"), blobOf(t, layerDescriptor)); err != nil {
		t.Fatal(err)
	}
	withToken := 0
	for _, request := range r.Requests() {
		if request.Path == "/v2/app/blobs/"+layerDescriptor.Digest && request.Authorization == "Bearer "+r.Token() {
			withToken++
		}
	}
	if withToken != 3 {
		t.Errorf("blob requests with the token = %d, want 3: the request and its two hops", withToken)
	}
}

// TestRedirectDowngradeRefused proves a redirect from HTTPS to HTTP is
// refused and the plain target is never asked.
func TestRedirectDowngradeRefused(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone, TLS: true})
	plain := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	layer := []byte("downgraded layer")
	_, layerDescriptor := putImage(t, r, "app", "1", layer)
	plain.PutBlob(ociregistry.MediaOCILayerGzip, layer)
	r.SetFault(layerDescriptor.Digest, ociregistry.Fault{RedirectTo: plain.BlobURL("app", layerDescriptor.Digest)})
	anchor := x509.NewCertPool()
	anchor.AddCert(r.Certificate())
	c := newClient(t, Config{Registry: r.Host(), Transport: TransportHTTPS, TrustAnchor: anchor})

	_, err := fetchToFile(t, t.Context(), c, mustReference(t, r.Host()+"/app:1"), blobOf(t, layerDescriptor))
	failure := requireFailure(t, err, FailureUnreachable)
	if !strings.Contains(failure.Why(), "HTTPS") {
		t.Errorf("why = %q, want the downgrade named", failure.Why())
	}
	if n := len(plain.Requests()); n != 0 {
		t.Errorf("the plain target was asked %d times", n)
	}
}

// TestRedirectLimit proves ten redirects are followed and the eleventh is
// refused.
func TestRedirectLimit(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	_, layerDescriptor := putImage(t, r, "app", "1", []byte("far layer"))
	c := newClient(t, Config{Registry: r.Host()})
	ref := mustReference(t, r.Host()+"/app:1")

	r.SetFault(layerDescriptor.Digest, ociregistry.Fault{RedirectHops: redirectMax})
	if _, err := fetchToFile(t, t.Context(), c, ref, blobOf(t, layerDescriptor)); err != nil {
		t.Fatalf("ten redirects were not followed: %v", err)
	}
	r.SetFault(layerDescriptor.Digest, ociregistry.Fault{RedirectHops: redirectMax + 1})
	_, err := fetchToFile(t, t.Context(), c, ref, blobOf(t, layerDescriptor))
	failure := requireFailure(t, err, FailureUnreachable)
	if !strings.Contains(failure.Problem(), "too many redirects") {
		t.Errorf("problem = %q, want the redirect bound", failure.Problem())
	}
}
