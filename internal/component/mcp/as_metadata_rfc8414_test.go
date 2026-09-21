// Design: docs/architecture/mcp/overview.md -- MCP OAuth resource server
//
// RFC 8414 Sections 3.1 and 6.1 as they bind the party that QUERIES the
// authorization server metadata document: the request is an HTTP GET at the
// well-known path inserted between host and path of the issuer identifier,
// the fetch runs over TLS, and the server certificate is checked.

package mcp

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordingAS is a fake authorization server that serves one RFC 8414
// document at exactly one path and records every request it receives.
type recordingAS struct {
	mu       sync.Mutex
	requests []string // "METHOD path", in arrival order
	docPath  string   // the only path that answers 200
	issuer   string
}

func (r *recordingAS) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	r.requests = append(r.requests, req.Method+" "+req.URL.Path)
	r.mu.Unlock()
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if req.URL.Path != r.docPath {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	body, err := json.Marshal(map[string]any{
		"issuer":   r.issuer,
		"jwks_uri": r.issuer + "/jwks",
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, werr := w.Write(body); werr != nil {
		return
	}
}

func (r *recordingAS) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.requests...)
}

func fetchCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// VALIDATES: the fetch is one GET at the well-known path.
// RFC requirement: RFC8414-3.1-1 positive -- the metadata fetch for an issuer with no path
// component is one HTTP GET at /.well-known/oauth-authorization-server, and the document it
// returns is decoded.
func TestRFC8414MetadataRequestIsGETAtWellKnownPath(t *testing.T) {
	as := &recordingAS{docPath: asMetadataWellKnownPath}
	srv := httptest.NewServer(as)
	defer srv.Close()
	as.issuer = srv.URL

	md, err := fetchASMetadata(fetchCtx(t), nil, srv.URL)
	if err != nil {
		t.Fatalf("fetchASMetadata: %v", err)
	}
	if md.Issuer != srv.URL {
		t.Fatalf("issuer = %q, want %q", md.Issuer, srv.URL)
	}
	seen := as.seen()
	if len(seen) != 1 {
		t.Fatalf("requests = %v, want exactly one", seen)
	}
	if seen[0] != "GET "+asMetadataWellKnownPath {
		t.Fatalf("request = %q, want %q", seen[0], "GET "+asMetadataWellKnownPath)
	}
}

// VALIDATES: no other verb and no other path reaches the AS.
// RFC requirement: RFC8414-3.1-1 negative -- an AS that answers 405 to every method but GET and
// 404 off the well-known path still serves the fetch, so no other verb and no other path was
// ever tried.
func TestRFC8414MetadataRequestUsesNoOtherVerbOrPath(t *testing.T) {
	as := &recordingAS{docPath: asMetadataWellKnownPath}
	srv := httptest.NewServer(as)
	defer srv.Close()
	as.issuer = srv.URL

	if _, err := fetchASMetadata(fetchCtx(t), nil, srv.URL); err != nil {
		t.Fatalf("fetchASMetadata: %v", err)
	}
	for _, r := range as.seen() {
		if strings.HasPrefix(r, "GET ") {
			continue
		}
		t.Fatalf("non-GET request reached the AS: %q", r)
	}
	for _, r := range as.seen() {
		if r != "GET "+asMetadataWellKnownPath {
			t.Fatalf("request off the well-known path reached the AS: %q", r)
		}
	}
}

// VALIDATES: the suffix is inserted between host and path, slash stripped.
// RFC requirement: RFC8414-3.1-2 positive -- for the issuer <origin>/issuer1/ the terminating
// slash is removed and the request path is /.well-known/oauth-authorization-server/issuer1.
func TestRFC8414MetadataPathInsertedBeforeIssuerPath(t *testing.T) {
	as := &recordingAS{docPath: asMetadataWellKnownPath + "/issuer1"}
	srv := httptest.NewServer(as)
	defer srv.Close()
	as.issuer = srv.URL + "/issuer1"

	md, err := fetchASMetadata(fetchCtx(t), nil, srv.URL+"/issuer1/")
	if err != nil {
		t.Fatalf("fetchASMetadata: %v", err)
	}
	if md.Issuer != srv.URL+"/issuer1" {
		t.Fatalf("issuer = %q, want %q", md.Issuer, srv.URL+"/issuer1")
	}
	seen := as.seen()
	want := "GET " + asMetadataWellKnownPath + "/issuer1"
	if len(seen) != 1 || seen[0] != want {
		t.Fatalf("requests = %v, want [%q]", seen, want)
	}
}

// VALIDATES: the OIDC-style appended path is never requested.
// RFC requirement: RFC8414-3.1-2 negative -- an AS that serves the document only at the
// appended location /issuer1/.well-known/oauth-authorization-server is never found: the fetch
// fails and the AS saw neither the appended path nor a path ending in the terminating slash.
func TestRFC8414MetadataPathNeverAppendedToIssuerPath(t *testing.T) {
	as := &recordingAS{docPath: "/issuer1" + asMetadataWellKnownPath}
	srv := httptest.NewServer(as)
	defer srv.Close()
	as.issuer = srv.URL + "/issuer1"

	_, err := fetchASMetadata(fetchCtx(t), nil, srv.URL+"/issuer1/")
	if err == nil {
		t.Fatal("fetchASMetadata found a document at the appended path")
	}
	if !strings.Contains(err.Error(), "status 404") {
		t.Fatalf("error = %v, want the 404 from the RFC 8414 path", err)
	}
	for _, r := range as.seen() {
		if strings.Contains(r, "/issuer1"+asMetadataWellKnownPath) {
			t.Fatalf("appended path was requested: %q", r)
		}
		if strings.HasSuffix(r, "/") {
			t.Fatalf("terminating slash was kept: %q", r)
		}
	}
}

// VALIDATES: the fetch works over TLS.
// RFC requirement: RFC8414-6.1-1 positive -- the metadata document is fetched over TLS and
// decoded when the client trusts the server certificate.
func TestRFC8414MetadataFetchOverTLS(t *testing.T) {
	as := &recordingAS{docPath: asMetadataWellKnownPath}
	srv := httptest.NewTLSServer(as)
	defer srv.Close()
	as.issuer = srv.URL
	if !strings.HasPrefix(srv.URL, "https://") {
		t.Fatalf("test server URL %q is not https", srv.URL)
	}

	md, err := fetchASMetadata(fetchCtx(t), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fetchASMetadata over TLS: %v", err)
	}
	if md.Issuer != srv.URL {
		t.Fatalf("issuer = %q, want %q", md.Issuer, srv.URL)
	}
	if len(as.seen()) != 1 {
		t.Fatalf("requests = %v, want exactly one", as.seen())
	}
}

// VALIDATES: an https issuer is never fetched in cleartext.
// RFC requirement: RFC8414-6.1-1 negative -- an https issuer whose server answers in cleartext
// is refused and its handler is never reached: the fetch does not fall back to plain HTTP.
func TestRFC8414MetadataFetchNeverDowngradesToCleartext(t *testing.T) {
	as := &recordingAS{docPath: asMetadataWellKnownPath}
	srv := httptest.NewServer(as)
	defer srv.Close()
	httpsURL := "https://" + strings.TrimPrefix(srv.URL, "http://")
	as.issuer = httpsURL

	md, err := fetchASMetadata(fetchCtx(t), nil, httpsURL)
	if err == nil {
		t.Fatalf("fetchASMetadata accepted a cleartext answer for %q: %+v", httpsURL, md)
	}
	if md.Issuer != "" {
		t.Fatalf("issuer %q leaked from a refused fetch", md.Issuer)
	}
	if len(as.seen()) != 0 {
		t.Fatalf("cleartext handler served the request: %v", as.seen())
	}
}

// VALIDATES: a trusted certificate passes the check.
// RFC requirement: RFC8414-6.1-3 positive -- a server certificate the client trusts passes the
// certificate check and the document is decoded.
func TestRFC8414MetadataFetchAcceptsTrustedCertificate(t *testing.T) {
	as := &recordingAS{docPath: asMetadataWellKnownPath}
	srv := httptest.NewTLSServer(as)
	defer srv.Close()
	as.issuer = srv.URL

	md, err := fetchASMetadata(fetchCtx(t), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fetchASMetadata with a trusted certificate: %v", err)
	}
	if md.JWKSURI != srv.URL+"/jwks" {
		t.Fatalf("jwks_uri = %q, want %q", md.JWKSURI, srv.URL+"/jwks")
	}
}

// VALIDATES: an untrusted certificate is refused.
// RFC requirement: RFC8414-6.1-3 negative -- a server certificate from an authority the client
// does not trust is refused with x509.UnknownAuthorityError and the document is never read.
func TestRFC8414MetadataFetchRefusesUntrustedCertificate(t *testing.T) {
	as := &recordingAS{docPath: asMetadataWellKnownPath}
	srv := httptest.NewTLSServer(as)
	defer srv.Close()
	as.issuer = srv.URL

	// A nil client is what buildAuthForMode passes: the default transport
	// with the system trust store, which does not hold httptest's CA.
	md, err := fetchASMetadata(fetchCtx(t), nil, srv.URL)
	if err == nil {
		t.Fatalf("fetchASMetadata accepted an untrusted certificate: %+v", md)
	}
	var unknownCA x509.UnknownAuthorityError
	if !errors.As(err, &unknownCA) {
		t.Fatalf("error = %v, want x509.UnknownAuthorityError", err)
	}
	if md.Issuer != "" {
		t.Fatalf("issuer %q leaked from a refused fetch", md.Issuer)
	}
	if len(as.seen()) != 0 {
		t.Fatalf("handler served a request over an untrusted session: %v", as.seen())
	}
}
