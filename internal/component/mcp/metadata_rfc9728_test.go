// RFC: rfc/short/rfc9728.md -- Sections 3, 3.1, 3.2 and 3.3, the protected-resource face
// Related: oauth.go -- writeResourceMetadata, the document body
// Related: streamable_auth.go -- resourceMetadataURL, the location Ze advertises

// Goal: prove the RFC 9728 document is published where Section 3 says, in
// the shape Section 3.2 says, from the identifier Section 3.3 says.
// Method: build an OAuth-mode Streamable against the in-process test AS and
// drive ServeHTTP with the request a client would send.
//
// VALIDATES: the well-known suffix is inserted between the host and the
// resource identifier's path, GET is the only method answered, the response
// is a 200 application/json object of Section 2 members with zero-valued
// members omitted, and the resource value is the identifier the URL was
// formed from.
// PREVENTS: the suffix appended after the identifier's path (the shape Ze
// served until 2026-09-21), which a conforming client never fetches.

package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// rfc9728Section2Members is the parameter set RFC 9728 Section 2 registers.
// Section 3.2: the members of a response "are a subset of the metadata
// parameters defined in Section 2".
var rfc9728Section2Members = map[string]struct{}{
	"resource":                                   {},
	"authorization_servers":                      {},
	"jwks_uri":                                   {},
	"scopes_supported":                           {},
	"bearer_methods_supported":                   {},
	"resource_signing_alg_values_supported":      {},
	"resource_name":                              {},
	"resource_documentation":                     {},
	"resource_policy_uri":                        {},
	"resource_tos_uri":                           {},
	"tls_client_certificate_bound_access_tokens": {},
	"authorization_details_types_supported":      {},
	"dpop_signing_alg_values_supported":          {},
	"dpop_bound_access_tokens_required":          {},
	"signed_metadata":                            {},
}

// newMetadataServer builds an OAuth-mode server whose resource identifier
// is cfg.Audience (or cfg.MetadataResource), against a fresh test AS.
func newMetadataServer(t *testing.T, cfg OAuthConfig) *Streamable {
	t.Helper()
	as := newTestAS(t)
	cfg.AuthorizationServer = as.Issuer()
	s, err := NewStreamable(StreamableConfig{AuthMode: AuthOAuth, OAuth: cfg})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

// requestMetadata sends one request for the metadata document and returns
// the recorded response.
func requestMetadata(t *testing.T, s *Streamable, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, http.NoBody)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	return w
}

// decodeMetadata decodes a 200 response body into its raw member map.
func decodeMetadata(t *testing.T, w *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %q", w.Code, w.Body.String())
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatalf("body is not a JSON object: %v; body %q", err, w.Body.String())
	}
	return doc
}

// RFC requirement: RFC9728-3-2 positive — for the resource identifier https://mcp.example/mcp the document is served at /.well-known/oauth-protected-resource/mcp, the well-known string inserted between the host and the path, and resourceMetadataURL advertises that same URL.
// RFC requirement: RFC9728-3-2 negative — the same server answers 404 at /mcp/.well-known/oauth-protected-resource (suffix appended after the path) and at the bare /.well-known/oauth-protected-resource (path dropped), so the document is published at the inserted location and nowhere else.
func TestRFC9728WellKnownInsertedBetweenHostAndPath(t *testing.T) {
	s := newMetadataServer(t, OAuthConfig{Audience: "https://mcp.example/mcp"})

	wantURL := "https://mcp.example/.well-known/oauth-protected-resource/mcp"
	if got := resourceMetadataURL(s.cfg.OAuth); got != wantURL {
		t.Fatalf("resourceMetadataURL = %q, want %q", got, wantURL)
	}
	doc := decodeMetadata(t, requestMetadata(t, s, http.MethodGet, "/.well-known/oauth-protected-resource/mcp"))
	if got := string(doc["resource"]); got != `"https://mcp.example/mcp"` {
		t.Fatalf("resource = %s, want %q", got, "https://mcp.example/mcp")
	}

	for _, wrong := range []string{
		"/mcp/.well-known/oauth-protected-resource",
		"/.well-known/oauth-protected-resource",
	} {
		if w := requestMetadata(t, s, http.MethodGet, wrong); w.Code != http.StatusNotFound {
			t.Fatalf("GET %s: status = %d, want 404 (document must not be published there)", wrong, w.Code)
		}
	}
}

// RFC requirement: RFC9728-3.1-3 positive — the slash terminating the host component is removed before the suffix is inserted: https://mcp.example/ yields https://mcp.example/.well-known/oauth-protected-resource and https://mcp.example/mcp/ yields https://mcp.example/.well-known/oauth-protected-resource/mcp.
// RFC requirement: RFC9728-3.1-3 negative — no advertised URL carries a doubled slash before .well-known or a trailing slash after the inserted suffix.
func TestRFC9728TerminatingSlashRemovedBeforeInsertion(t *testing.T) {
	cases := []struct {
		audience string
		want     string
	}{
		{"https://mcp.example/", "https://mcp.example/.well-known/oauth-protected-resource"},
		{"https://mcp.example/mcp/", "https://mcp.example/.well-known/oauth-protected-resource/mcp"},
	}
	for _, c := range cases {
		got := resourceMetadataURL(OAuthConfig{Audience: c.audience})
		if got != c.want {
			t.Fatalf("resourceMetadataURL(%q) = %q, want %q", c.audience, got, c.want)
		}
		if strings.Contains(got, "//.well-known") {
			t.Fatalf("resourceMetadataURL(%q) = %q keeps the terminating slash", c.audience, got)
		}
		if strings.HasSuffix(got, "/") {
			t.Fatalf("resourceMetadataURL(%q) = %q ends in a slash", c.audience, got)
		}
	}
}

// RFC requirement: RFC9728-3-3 positive — the suffix Ze serves under is the registered default oauth-protected-resource (Section 8.3), and a GET there returns the document.
// RFC requirement: RFC9728-3-3 negative — an unregistered suffix, /.well-known/example-protected-resource, is answered 404.
// RFC requirement: RFC9728-3-4 positive — Ze specifies one fixed suffix, OAuthMetadataPath, and serves the document at it.
// RFC requirement: RFC9728-3-4 negative — a suffix the request chooses is not honored: /.well-known/example-protected-resource is answered 404.
func TestRFC9728RegisteredSuffix(t *testing.T) {
	if OAuthMetadataPath != "/.well-known/oauth-protected-resource" {
		t.Fatalf("OAuthMetadataPath = %q, want the registered default", OAuthMetadataPath)
	}
	s := newMetadataServer(t, OAuthConfig{Audience: "https://mcp.example/"})
	decodeMetadata(t, requestMetadata(t, s, http.MethodGet, OAuthMetadataPath))
	if w := requestMetadata(t, s, http.MethodGet, "/.well-known/example-protected-resource"); w.Code != http.StatusNotFound {
		t.Fatalf("unregistered suffix: status = %d, want 404", w.Code)
	}
}

// RFC requirement: RFC9728-3.1-2 positive — an HTTP GET at the metadata URL is answered 200 with the document.
// RFC requirement: RFC9728-3.1-2 negative — POST and PUT at the metadata URL are refused with 405 and Allow: GET, OPTIONS, so the document is obtainable through GET only.
func TestRFC9728MetadataQueriedWithGET(t *testing.T) {
	s := newMetadataServer(t, OAuthConfig{Audience: "https://mcp.example/"})
	decodeMetadata(t, requestMetadata(t, s, http.MethodGet, OAuthMetadataPath))
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		w := requestMetadata(t, s, method, OAuthMetadataPath)
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s: status = %d, want 405", method, w.Code)
		}
		if allow := w.Header().Get("Allow"); allow != "GET, OPTIONS" {
			t.Fatalf("%s: Allow = %q, want %q", method, allow, "GET, OPTIONS")
		}
	}
}

// RFC requirement: RFC9728-3.2-3 positive — the successful response is 200 OK, Content-Type application/json, and a JSON object.
// RFC requirement: RFC9728-3.2-3 negative — no member outside the Section 2 parameter set appears in the object.
func TestRFC9728ResponseShape(t *testing.T) {
	s := newMetadataServer(t, OAuthConfig{Audience: "https://mcp.example/", RequiredScopes: []string{"mcp.admin"}})
	w := requestMetadata(t, s, http.MethodGet, OAuthMetadataPath)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	doc := decodeMetadata(t, w)
	if len(doc) == 0 {
		t.Fatal("document has no members")
	}
	for name := range doc {
		if _, known := rfc9728Section2Members[name]; !known {
			t.Fatalf("member %q is not a Section 2 parameter", name)
		}
	}
}

// RFC requirement: RFC9728-3.2-4 positive — a non-empty required-scopes list is emitted as scopes_supported.
// RFC requirement: RFC9728-3.2-4 negative — with zero required scopes the scopes_supported member is absent rather than emitted empty.
func TestRFC9728ZeroValuedParametersOmitted(t *testing.T) {
	withScopes := newMetadataServer(t, OAuthConfig{Audience: "https://mcp.example/", RequiredScopes: []string{"mcp.admin"}})
	doc := decodeMetadata(t, requestMetadata(t, withScopes, http.MethodGet, OAuthMetadataPath))
	if got := string(doc["scopes_supported"]); got != `["mcp.admin"]` {
		t.Fatalf("scopes_supported = %s, want [\"mcp.admin\"]", got)
	}

	noScopes := newMetadataServer(t, OAuthConfig{Audience: "https://mcp.example/"})
	doc = decodeMetadata(t, requestMetadata(t, noScopes, http.MethodGet, OAuthMetadataPath))
	if raw, present := doc["scopes_supported"]; present {
		t.Fatalf("scopes_supported = %s, want the member omitted", raw)
	}
}

// RFC requirement: RFC9728-3.3-1 positive — the resource value returned is the identifier the suffix was inserted into: with metadata-resource https://mcp.example/api the document at /.well-known/oauth-protected-resource/api carries resource https://mcp.example/api and resourceMetadataURL is that identifier with the suffix inserted.
// RFC requirement: RFC9728-3.3-1 negative — when metadata-resource overrides audience, the returned resource is never the audience, so the value and the location derive from one identifier.
func TestRFC9728ResourceMatchesInsertedIdentifier(t *testing.T) {
	s := newMetadataServer(t, OAuthConfig{
		Audience:         "https://mcp.example/",
		MetadataResource: "https://mcp.example/api",
	})
	wantURL := "https://mcp.example/.well-known/oauth-protected-resource/api"
	if got := resourceMetadataURL(s.cfg.OAuth); got != wantURL {
		t.Fatalf("resourceMetadataURL = %q, want %q", got, wantURL)
	}
	doc := decodeMetadata(t, requestMetadata(t, s, http.MethodGet, "/.well-known/oauth-protected-resource/api"))
	got := string(doc["resource"])
	if got == `"https://mcp.example/"` {
		t.Fatal("resource carries the audience, not the identifier the URL was formed from")
	}
	if got != `"https://mcp.example/api"` {
		t.Fatalf("resource = %s, want %q", got, "https://mcp.example/api")
	}
}
