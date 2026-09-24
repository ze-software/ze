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
	"sync/atomic"
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
	// RFC requirement: RFC8414-3-2 positive -- the application requests metadata at the registered oauth-authorization-server suffix.
	// RFC requirement: RFC8414-3-3 positive -- the application uses its fixed RFC8414 suffix for discovery.
	as := &recordingAS{docPath: asMetadataWellKnownPath}
	srv := newTrustedTLSServer(t, as)
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
	srv := newTrustedTLSServer(t, as)
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
	srv := newTrustedTLSServer(t, as)
	defer srv.Close()
	as.issuer = srv.URL + "/issuer1/"

	md, err := fetchASMetadata(fetchCtx(t), nil, srv.URL+"/issuer1/")
	if err != nil {
		t.Fatalf("fetchASMetadata: %v", err)
	}
	if md.Issuer != srv.URL+"/issuer1/" {
		t.Fatalf("issuer = %q, want %q", md.Issuer, srv.URL+"/issuer1/")
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
	// RFC requirement: RFC8414-3-2 negative -- discovery never requests the appended, unregistered metadata location.
	// RFC requirement: RFC8414-3-3 negative -- an AS offering only an alternate location cannot change the application's discovery suffix.
	as := &recordingAS{docPath: "/issuer1" + asMetadataWellKnownPath}
	srv := newTrustedTLSServer(t, as)
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
	if _, isUnknownCA := errors.AsType[x509.UnknownAuthorityError](err); !isUnknownCA {
		t.Fatalf("error = %v, want x509.UnknownAuthorityError", err)
	}
	if md.Issuer != "" {
		t.Fatalf("issuer %q leaked from a refused fetch", md.Issuer)
	}
	if len(as.seen()) != 0 {
		t.Fatalf("handler served a request over an untrusted session: %v", as.seen())
	}
}

// TestRFC8414MetadataRequiresHTTPS checks the configured URL before a network
// request and proves that redirect following cannot cross into plaintext.
// RFC requirement: RFC8414-3.3-1 positive -- HTTPS metadata from a trusted server is decoded.
// RFC requirement: RFC8414-3.3-1 negative -- an HTTP issuer and an HTTPS-to-HTTP redirect are refused without contacting the HTTP destination.
func TestRFC8414MetadataRequiresHTTPS(t *testing.T) {
	var plainHits atomic.Int64
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		plainHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer plain.Close()
	if md, err := fetchASMetadata(fetchCtx(t), nil, plain.URL); err == nil || md != (asMetadata{}) {
		t.Fatalf("HTTP issuer accepted: %+v, %v", md, err)
	}
	redirect := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL, http.StatusFound)
	}))
	defer redirect.Close()
	if md, err := fetchASMetadata(fetchCtx(t), redirect.Client(), redirect.URL); err == nil || md != (asMetadata{}) {
		t.Fatalf("plaintext redirect accepted: %+v, %v", md, err)
	}
	if plainHits.Load() != 0 {
		t.Fatal("metadata request reached the plaintext destination")
	}

	as := &recordingAS{docPath: asMetadataWellKnownPath}
	secure := httptest.NewTLSServer(as)
	defer secure.Close()
	as.issuer = secure.URL
	md, err := fetchASMetadata(fetchCtx(t), secure.Client(), secure.URL)
	if err != nil || md.Issuer != secure.URL {
		t.Fatalf("trusted HTTPS metadata rejected: %+v, %v", md, err)
	}
}

func TestConfiguredIssuerQueryRejectedBeforeMetadataFetch(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	for _, suffix := range []string{"?tenant=x", "?"} {
		t.Run(suffix, func(t *testing.T) {
			if _, err := fetchASMetadata(t.Context(), srv.Client(), srv.URL+suffix); err == nil {
				t.Fatal("configured issuer query was accepted")
			}
			if hits.Load() != 0 {
				t.Fatal("invalid issuer triggered a metadata request")
			}
		})
	}
}

// TestRFC8414MetadataHTTPResponse checks the response envelope before accepting
// issuer and key metadata, including a JSON array in place of an object.
// RFC requirement: RFC8414-3.2-1 positive -- 200 application/json containing an object supplies the issuer and key URL.
// RFC requirement: RFC8414-3.2-1 negative -- a non-200 status, wrong or absent Content-Type, and a non-object JSON body each return no metadata.
func TestRFC8414MetadataHTTPResponse(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		contentType string
		array       bool
		valid       bool
	}{
		{"JSON object", http.StatusOK, "application/json", false, true},
		{"JSON with charset", http.StatusOK, "application/json; charset=utf-8", false, true},
		{"created", http.StatusCreated, "application/json", false, false},
		{"HTML", http.StatusOK, "text/html", false, false},
		{"absent type", http.StatusOK, "", false, false},
		{"array", http.StatusOK, "application/json", true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var issuer string
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header()["Content-Type"] = []string{c.contentType}
				w.WriteHeader(c.status)
				var body any = map[string]any{"issuer": issuer, "jwks_uri": issuer + "/jwks"}
				if c.array {
					body = []any{body}
				}
				if err := json.NewEncoder(w).Encode(body); err != nil {
					t.Errorf("write metadata: %v", err)
				}
			}))
			defer srv.Close()
			issuer = srv.URL
			md, err := fetchASMetadata(fetchCtx(t), srv.Client(), issuer)
			if c.valid {
				if err != nil || md.Issuer != issuer || md.JWKSURI != issuer+"/jwks" {
					t.Fatalf("valid metadata: %+v, %v", md, err)
				}
				return
			}
			if err == nil || md != (asMetadata{}) {
				t.Fatalf("invalid response supplied metadata: %+v, %v", md, err)
			}
		})
	}
}

// TestRFC8414MetadataIssuerIdentity proves metadata discovery compares JSON
// strings after unescaping but without URL or Unicode normalization.
// RFC requirement: RFC8414-3.3-2 positive -- a JSON-escaped issuer equal to the configured identifier is accepted.
// RFC requirement: RFC8414-3.3-2 negative -- slash, path, and Unicode aliases of the configured issuer are rejected.
// RFC requirement: RFC8414-4-2 positive -- an identical decomposed Unicode issuer is accepted without normalization.
// RFC requirement: RFC8414-4-2 negative -- composed and decomposed Unicode issuer spellings are not interchangeable.
// RFC requirement: RFC8414-4-3 positive -- JSON escapes decode before an exact issuer comparison.
// RFC requirement: RFC8414-4-3 negative -- a different issuer code point returns no metadata.
func TestRFC8414MetadataIssuerIdentity(t *testing.T) {
	var advertised string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, err := json.Marshal(map[string]any{"issuer": advertised, "jwks_uri": "https://as.example/jwks"})
		if err != nil {
			t.Errorf("encode: %v", err)
			return
		}
		escaped := strings.ReplaceAll(string(body), "/", `\/`)
		if _, err := w.Write([]byte(escaped)); err != nil {
			t.Errorf("write metadata: %v", err)
		}
	}))
	defer srv.Close()
	expected := srv.URL + "/e\u0301"
	for _, issuer := range []string{
		expected, srv.URL + "/\u00e9", expected + "/", srv.URL + "/other/../e\u0301",
	} {
		advertised = issuer
		md, err := fetchASMetadata(fetchCtx(t), srv.Client(), expected)
		if issuer == expected {
			if err != nil || md.Issuer != expected {
				t.Fatalf("identical issuer rejected: %+v, %v", md, err)
			}
			continue
		}
		if err == nil || md != (asMetadata{}) {
			t.Fatalf("issuer alias %q accepted: %+v, %v", issuer, md, err)
		}
	}
}
