package mcp

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// In-process OAuth AS test harness
//
// Provides a single endpoint bundle: RFC 8414 metadata + JWKS endpoint, plus
// helpers to mint RS256 access tokens. Used by the end-to-end tests that
// exercise buildAuthForMode + NewStreamable with a real httptest server.
// -----------------------------------------------------------------------------

type testAS struct {
	srv      *httptest.Server
	priv     *rsa.PrivateKey
	issuer   string
	jwksHits *atomic.Int64
}

func newTestAS(t *testing.T) *testAS {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	hits := &atomic.Int64{}
	mux := http.NewServeMux()
	var issuer string // captured below once server is up

	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		body, err := json.Marshal(map[string]any{
			"issuer":   issuer,
			"jwks_uri": issuer + "/jwks",
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, werr := w.Write(body); werr != nil {
			t.Logf("write metadata: %v", werr)
		}
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		body, err := json.Marshal(map[string]any{
			"keys": []map[string]any{rsaJWK(t, priv, "k1")},
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, werr := w.Write(body); werr != nil {
			t.Logf("write jwks: %v", werr)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	issuer = srv.URL
	return &testAS{srv: srv, priv: priv, issuer: issuer, jwksHits: hits}
}

// Issuer returns the AS URL (matches the value in the metadata document).
func (a *testAS) Issuer() string { return a.issuer }

// MintToken returns an RS256 JWT with the standard claims plus any overrides.
func (a *testAS) MintToken(t *testing.T, overrides map[string]any) string {
	t.Helper()
	now := time.Now()
	claims := map[string]any{
		"iss": a.issuer,
		"sub": "alice",
		"aud": "https://mcp.example/",
		"exp": now.Add(time.Hour).Unix(),
		"iat": now.Unix(),
	}
	maps.Copy(claims, overrides)
	return signRS256(t, a.priv, "k1", claims)
}

// -----------------------------------------------------------------------------
// ISSUE 3 fix -- AS metadata issuer MUST match the configured auth-server URL
// -----------------------------------------------------------------------------

func TestNewStreamable_OAuth_RejectsIssuerMismatch(t *testing.T) {
	// Build a minimal AS that responds with a mismatched issuer value.
	mux := http.NewServeMux()
	var badSrv *httptest.Server // populated below
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		body, _ := json.Marshal(map[string]any{
			"issuer":   "https://impersonator.example/",
			"jwks_uri": badSrv.URL + "/jwks",
		})
		if _, werr := w.Write(body); werr != nil {
			t.Logf("write: %v", werr)
		}
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		if _, werr := w.Write([]byte(`{"keys":[]}`)); werr != nil {
			t.Logf("write: %v", werr)
		}
	})
	badSrv = httptest.NewServer(mux)
	defer badSrv.Close()

	_, err := NewStreamable(StreamableConfig{
		AuthMode: AuthOAuth,
		OAuth: OAuthConfig{
			AuthorizationServer: badSrv.URL,
			Audience:            "https://mcp.example/",
		},
	})
	if err == nil {
		t.Fatal("expected error on issuer mismatch")
	}
	if !strings.Contains(err.Error(), "does not match configured") {
		t.Fatalf("error should name issuer mismatch, got %v", err)
	}
}

// -----------------------------------------------------------------------------
// End-to-end OAuth auth through NewStreamable + ServeHTTP
// -----------------------------------------------------------------------------

func TestNewStreamable_OAuth_AcceptsValidToken(t *testing.T) {
	as := newTestAS(t)
	s, err := NewStreamable(StreamableConfig{
		AuthMode: AuthOAuth,
		OAuth: OAuthConfig{
			AuthorizationServer: as.Issuer(),
			Audience:            "https://mcp.example/",
		},
	})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()

	token := as.MintToken(t, nil)
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`
	req := httptest.NewRequest(http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	sid := w.Header().Get("Mcp-Session-Id")
	if sid == "" {
		t.Fatal("no Mcp-Session-Id on valid-token initialize")
	}
	sess, ok := s.registry.Get(sid)
	if !ok {
		t.Fatalf("session %q not found after initialize", sid)
	}
	if got := sess.Identity().Name; got != "alice" {
		t.Fatalf("identity.Name = %q, want alice", got)
	}
}

func TestNewStreamable_OAuth_RejectsMissingBearer(t *testing.T) {
	as := newTestAS(t)
	s, err := NewStreamable(StreamableConfig{
		AuthMode: AuthOAuth,
		OAuth: OAuthConfig{
			AuthorizationServer: as.Issuer(),
			Audience:            "https://mcp.example/",
		},
	})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`
	req := httptest.NewRequest(http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header.
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	wa := w.Header().Get("WWW-Authenticate")
	if wa == "" {
		t.Fatal("missing WWW-Authenticate header")
	}
	// BLOCKER 2 fix: resource_metadata MUST be present on the challenge.
	if !strings.Contains(wa, `resource_metadata="https://mcp.example/`) {
		t.Fatalf("WWW-Authenticate missing resource_metadata pointing at the well-known URL: %q", wa)
	}
	// ISSUE 12 fix: Cache-Control must be set on 401.
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cc)
	}
}

func TestNewStreamable_OAuth_RejectsWrongAudience(t *testing.T) {
	as := newTestAS(t)
	s, err := NewStreamable(StreamableConfig{
		AuthMode: AuthOAuth,
		OAuth: OAuthConfig{
			AuthorizationServer: as.Issuer(),
			Audience:            "https://mcp.example/",
		},
	})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()

	token := as.MintToken(t, map[string]any{"aud": "https://wrong/"})
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`
	req := httptest.NewRequest(http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if !strings.Contains(w.Header().Get("WWW-Authenticate"), `error_description="invalid audience"`) {
		t.Fatalf("WWW-Authenticate = %q", w.Header().Get("WWW-Authenticate"))
	}
}

// -----------------------------------------------------------------------------
// RFC 9728 metadata endpoint served under AuthMode=OAuth
// -----------------------------------------------------------------------------

func TestNewStreamable_OAuth_MetadataEndpoint(t *testing.T) {
	as := newTestAS(t)
	s, err := NewStreamable(StreamableConfig{
		AuthMode: AuthOAuth,
		OAuth: OAuthConfig{
			AuthorizationServer: as.Issuer(),
			Audience:            "https://mcp.example/",
			RequiredScopes:      []string{"mcp.admin"},
		},
	})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest(http.MethodGet, OAuthMetadataPath, http.NoBody)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["resource"] != "https://mcp.example/" {
		t.Fatalf("resource = %v", got["resource"])
	}
	servers, ok := got["authorization_servers"].([]any)
	if !ok || len(servers) != 1 || servers[0] != as.Issuer() {
		t.Fatalf("authorization_servers = %v, want [%q]", got["authorization_servers"], as.Issuer())
	}
}

// -----------------------------------------------------------------------------
// sameAuthServer canonicalization
// -----------------------------------------------------------------------------

func TestSameAuthServer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"https://as.example/", "https://as.example", true},
		{"https://as.example:443/", "https://as.example/", true},
		{"HTTPS://AS.EXAMPLE/", "https://as.example/", true},
		{"https://as.example/realm/x/", "https://as.example/realm/x", true},
		{"https://as.example/", "https://other.example/", false},
		{"https://as.example/x", "https://as.example/y", false},
		{"http://as.example/", "https://as.example/", false},
		{"", "", false}, // empty should not match anything
		// IPv6 literals must survive canonicalization with brackets intact.
		{"https://[::1]:443/", "https://[::1]/", true},
		{"https://[2001:db8::1]/", "https://[2001:DB8::1]/", true},
	}
	for _, tc := range cases {
		if got := sameAuthServer(tc.a, tc.b); got != tc.want {
			t.Errorf("sameAuthServer(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCanonicalAuthServerURL_RejectsQueryFragment(t *testing.T) {
	cases := []string{
		"https://as.example/?foo=bar",
		"https://as.example/#section",
		"https://as.example/path?x=1",
	}
	for _, tc := range cases {
		if _, err := canonicalAuthServerURL(tc); err == nil {
			t.Errorf("canonicalAuthServerURL(%q) accepted; wanted rejection", tc)
		}
	}
}

func TestAudClaim_MatchesCanonicalVariants(t *testing.T) {
	// RFC 8707 §2: audience comparison MUST happen after URL
	// canonicalization. A spec-compliant AS may emit any of these as
	// `aud` for the same resource and all must match operator
	// configuration of "https://mcp.example/".
	configured := "https://mcp.example/"
	accepted := []string{
		"https://mcp.example/",
		"https://mcp.example",
		"https://mcp.example:443/",
		"https://mcp.example:443",
		"https://MCP.EXAMPLE/",
		"https://mcp.example///",
	}
	for _, a := range accepted {
		t.Run("accept_"+a, func(t *testing.T) {
			claim := audClaim{a}
			if !claim.Matches(configured) {
				t.Fatalf("aud %q did not match configured %q", a, configured)
			}
		})
	}
	rejected := []string{
		"https://other.example/",
		"http://mcp.example/", // scheme downgrade
		"https://mcp.example/path",
	}
	for _, a := range rejected {
		t.Run("reject_"+a, func(t *testing.T) {
			claim := audClaim{a}
			if claim.Matches(configured) {
				t.Fatalf("aud %q unexpectedly matched configured %q", a, configured)
			}
		})
	}
}

func TestNewStreamable_OAuth_AcceptsSlashDivergentAudience(t *testing.T) {
	// Regression test: Streamable audience has trailing slash, token
	// audience does not. Exact-string compare rejected this; canonical
	// compare must accept. Without the canonicalAudience fix this test
	// fails with "invalid audience".
	as := newTestAS(t)
	s, err := NewStreamable(StreamableConfig{
		AuthMode: AuthOAuth,
		OAuth: OAuthConfig{
			AuthorizationServer: as.Issuer(),
			Audience:            "https://mcp.example/", // with trailing slash
		},
	})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()

	token := as.MintToken(t, map[string]any{"aud": "https://mcp.example"}) // no trailing slash
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`
	req := httptest.NewRequest(http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("canonicalised audience mismatch should be accepted: status=%d body=%s",
			w.Code, w.Body.String())
	}
}

func TestVerifyJWT_InvalidUTF8SubjectSanitisedByJSON(t *testing.T) {
	// End-to-end assertion of the defense-in-depth contract: malformed
	// UTF-8 in an AS-issued JWT payload is neutralized by Go's json
	// package. json.Marshal on the AS side and json.Unmarshal on our side
	// both substitute U+FFFD for invalid sequences, so `isSafeSubject`
	// never sees raw bad bytes in practice. Each sub-case asserts the
	// token is accepted AND the raw bad bytes do NOT appear in the
	// resulting Identity.Name. A future regression where json.Unmarshal
	// stops sanitizing would be caught by `utf8.ValidString` before the
	// byte-scan, which is why the check remains as defense in depth.
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Unix(1_700_000_000, 0)
	keys := &stubJWKS{keys: map[string]crypto.PublicKey{"k": &priv.PublicKey}}
	cases := []struct {
		name string
		sub  string
	}{
		{"overlong newline", "alice\xc0\x8aend"},
		{"lone continuation byte", "alice\x80end"},
		{"truncated sequence", "alice\xc3end"},
		{"lone surrogate", "alice\xed\xa0\x80end"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := standardClaims(now, "https://as/", "https://mcp/", time.Hour)
			claims["sub"] = tc.sub
			token := signRS256(t, priv, "k", claims)
			res, err := verifyJWT(token, jwtVerifyOptions{
				ExpectedIssuer:   "https://as/",
				ExpectedAudience: "https://mcp/",
				Keys:             keys,
				Clock:            newFixedClock(now),
			})
			if err != nil {
				t.Fatalf("verifyJWT: %v", err)
			}
			// Raw bad bytes must NOT survive into the identity.
			for _, c := range []string{"\xc0\x8a", "\x80", "\xc3e", "\xed\xa0\x80"} {
				if strings.Contains(res.Subject, c) {
					t.Fatalf("subject leaked raw invalid UTF-8 %q: %q", c, res.Subject)
				}
			}
			// The ASCII envelope "alice...end" must have survived around
			// the bad-byte site.
			if !strings.HasPrefix(res.Subject, "alice") || !strings.HasSuffix(res.Subject, "end") {
				t.Fatalf("subject did not round-trip around the bad bytes: %q", res.Subject)
			}
		})
	}
}

func TestStreamable_EndpointCORSPreflight(t *testing.T) {
	// Browser SPA at https://app.example/ loaded cross-origin to the MCP
	// server. A POST /mcp with Authorization + Content-Type triggers a
	// CORS preflight (OPTIONS). Phase 2 previously returned 405; the fix
	// must accept preflight when Origin is allowlisted.
	cfg := StreamableConfig{
		AuthMode:       AuthBearerList,
		BearerList:     []BearerListEntry{{Name: "alice", Token: "t"}},
		AllowedOrigins: []string{"https://app.example/"},
	}
	s, err := NewStreamable(cfg)
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest(http.MethodOptions, Endpoint, http.NoBody)
	req.Header.Set("Origin", "https://app.example/")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight from allowlisted origin: status=%d, want 204", w.Code)
	}
	if ao := w.Header().Get("Access-Control-Allow-Origin"); ao != "https://app.example/" {
		t.Fatalf("ACAO = %q, want echoed origin", ao)
	}
	if !strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), "POST") {
		t.Fatalf("ACAM missing POST: %q", w.Header().Get("Access-Control-Allow-Methods"))
	}
	if !strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatalf("ACAH missing Authorization: %q", w.Header().Get("Access-Control-Allow-Headers"))
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("ACAC missing: %q", w.Header().Get("Access-Control-Allow-Credentials"))
	}
	if w.Header().Get("Vary") != "Origin" {
		t.Fatalf("Vary = %q, want Origin", w.Header().Get("Vary"))
	}
}

func TestStreamable_EndpointCORSPreflight_RejectsNonAllowlistedOrigin(t *testing.T) {
	// Preflight from an origin NOT in the allowlist must fail the Origin
	// check BEFORE reaching the OPTIONS dispatch (so the Origin guard
	// protects preflight symmetry with the real request).
	cfg := StreamableConfig{
		AuthMode:       AuthBearerList,
		BearerList:     []BearerListEntry{{Name: "alice", Token: "t"}},
		AllowedOrigins: []string{"https://app.example/"},
	}
	s, err := NewStreamable(cfg)
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest(http.MethodOptions, Endpoint, http.NoBody)
	req.Header.Set("Origin", "https://evil.example/")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("preflight from non-allowlisted origin: status=%d, want 403", w.Code)
	}
}

func TestStreamable_MainPathResponseCORSEchoesOrigin(t *testing.T) {
	// Browser SPA's fetch() to /mcp passes preflight and then sends the
	// real POST. The response MUST carry Access-Control-Allow-Origin,
	// Access-Control-Allow-Credentials, and Access-Control-Expose-Headers
	// so the browser surfaces the response and the JS caller can read
	// Mcp-Session-Id / WWW-Authenticate.
	cfg := StreamableConfig{
		AuthMode:       AuthBearerList,
		BearerList:     []BearerListEntry{{Name: "alice", Token: "t"}},
		AllowedOrigins: []string{"https://app.example/"},
	}
	s, err := NewStreamable(cfg)
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`
	req := httptest.NewRequest(http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Origin", "https://app.example/")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("initialize status = %d, body = %s", w.Code, w.Body.String())
	}
	if ao := w.Header().Get("Access-Control-Allow-Origin"); ao != "https://app.example/" {
		t.Fatalf("ACAO on success = %q, want echoed origin", ao)
	}
	if ac := w.Header().Get("Access-Control-Allow-Credentials"); ac != "true" {
		t.Fatalf("ACAC on success = %q, want true", ac)
	}
	if expose := w.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(expose, "Mcp-Session-Id") {
		t.Fatalf("ACEH missing Mcp-Session-Id: %q", expose)
	}
	// Vary: Origin defends shared caches.
	if vary := w.Header().Get("Vary"); !strings.Contains(vary, "Origin") {
		t.Fatalf("Vary missing Origin: %q", vary)
	}
}

func TestStreamable_MainPath401CORSEchoesOrigin(t *testing.T) {
	// A 401 response on /mcp (wrong bearer) must also carry CORS +
	// expose WWW-Authenticate so the browser client can read the 401
	// challenge URL and discover the authorization server.
	cfg := StreamableConfig{
		AuthMode:       AuthBearerList,
		BearerList:     []BearerListEntry{{Name: "alice", Token: "t"}},
		AllowedOrigins: []string{"https://app.example/"},
	}
	s, err := NewStreamable(cfg)
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`
	req := httptest.NewRequest(http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong")
	req.Header.Set("Origin", "https://app.example/")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if ao := w.Header().Get("Access-Control-Allow-Origin"); ao != "https://app.example/" {
		t.Fatalf("ACAO on 401 = %q, want echoed origin", ao)
	}
	if expose := w.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(expose, "WWW-Authenticate") {
		t.Fatalf("ACEH missing WWW-Authenticate: %q", expose)
	}
}

func TestStreamable_NotFoundCarriesCORS(t *testing.T) {
	// A browser-allowlisted client hitting a wrong sub-path (/mcp/extra)
	// must receive the 404 with CORS headers so the JS caller sees the
	// descriptive error rather than a CORS rejection.
	cfg := StreamableConfig{
		AuthMode:       AuthNone,
		AllowedOrigins: []string{"https://app.example/"},
	}
	s, err := NewStreamable(cfg)
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest(http.MethodGet, "/mcp/extra", http.NoBody)
	req.Header.Set("Origin", "https://app.example/")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if ao := w.Header().Get("Access-Control-Allow-Origin"); ao != "https://app.example/" {
		t.Fatalf("ACAO on 404 = %q, want echoed origin", ao)
	}
}

func TestStreamable_MethodNotAllowedCarriesCORS(t *testing.T) {
	// PATCH / other non-supported methods on /mcp return 405; the response
	// must carry CORS so the JS caller sees the Allow header and reason.
	cfg := StreamableConfig{
		AuthMode:       AuthNone,
		AllowedOrigins: []string{"https://app.example/"},
	}
	s, err := NewStreamable(cfg)
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest(http.MethodPatch, Endpoint, http.NoBody)
	req.Header.Set("Origin", "https://app.example/")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", w.Code)
	}
	if ao := w.Header().Get("Access-Control-Allow-Origin"); ao != "https://app.example/" {
		t.Fatalf("ACAO on 405 = %q, want echoed origin", ao)
	}
	if allow := w.Header().Get("Allow"); !strings.Contains(allow, "POST") {
		t.Fatalf("Allow header missing: %q", allow)
	}
}

func TestSameAuthServer_IDNCanonical(t *testing.T) {
	// Operator-typed Unicode hostname vs AS-reported punycode (or
	// vice-versa) must canonicalize equal. Mirrors the canonicalOrigin
	// IDN handling that was missing in normaliseURL.
	cases := []struct {
		a, b string
		want bool
	}{
		{"https://müllerei.example/", "https://xn--mllerei-n2a.example/", true},
		{"https://xn--mllerei-n2a.example/", "https://müllerei.example/", true},
		{"https://MÜLLEREI.example/", "https://xn--mllerei-n2a.example/", true},
		{"https://münchen.example/", "https://xn--mnchen-3ya.example/", true},
		{"https://other.example/", "https://xn--mllerei-n2a.example/", false},
	}
	for _, tc := range cases {
		t.Run(tc.a+"_vs_"+tc.b, func(t *testing.T) {
			if got := sameAuthServer(tc.a, tc.b); got != tc.want {
				t.Errorf("sameAuthServer(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestAudClaim_MatchesIDNCanonical(t *testing.T) {
	// Same symmetry for audience compare: a token with `aud` in punycode
	// must match a Unicode-typed configured audience.
	configured := "https://müllerei.example/"
	claim := audClaim{"https://xn--mllerei-n2a.example/"}
	if !claim.Matches(configured) {
		t.Fatalf("punycode aud did not match Unicode configured audience")
	}
}

func TestStreamable_MainPathNoOriginNoCORS(t *testing.T) {
	// Non-browser client (no Origin header) gets no CORS headers on the
	// response. CORS headers are irrelevant for non-browser clients and
	// emitting them is harmless; omitting them is cleaner.
	cfg := StreamableConfig{AuthMode: AuthNone}
	s, err := NewStreamable(cfg)
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`
	req := httptest.NewRequest(http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if ao := w.Header().Get("Access-Control-Allow-Origin"); ao != "" {
		t.Fatalf("ACAO should be empty without Origin header, got %q", ao)
	}
}

func TestStreamable_EndpointPreflight_RequiresOrigin(t *testing.T) {
	// A bare OPTIONS without Origin header is not a CORS preflight; treat
	// it as a misconfigured client rather than silently returning 204.
	cfg := StreamableConfig{AuthMode: AuthNone}
	s, err := NewStreamable(cfg)
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest(http.MethodOptions, Endpoint, http.NoBody)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("OPTIONS without Origin: status=%d, want 400", w.Code)
	}
}

func TestVerifyJWT_RejectsControlCharSubject(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Unix(1_700_000_000, 0)
	cases := []struct {
		name string
		sub  string
	}{
		{"newline", "alice\ninjected"},
		{"carriage return", "alice\rx"},
		{"null byte", "alice\x00x"},
		{"escape", "alice\x1b[31m"},
		{"del", "alice\x7f"},
	}
	keys := &stubJWKS{keys: map[string]crypto.PublicKey{"k": &priv.PublicKey}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := standardClaims(now, "https://as/", "https://mcp/", time.Hour)
			claims["sub"] = tc.sub
			token := signRS256(t, priv, "k", claims)
			_, err := verifyJWT(token, jwtVerifyOptions{
				ExpectedIssuer:   "https://as/",
				ExpectedAudience: "https://mcp/",
				Keys:             keys,
				Clock:            newFixedClock(now),
			})
			if !errors.Is(err, errJWTUnsafeSub) {
				t.Fatalf("expected errJWTUnsafeSub for sub=%q, got %v", tc.sub, err)
			}
		})
	}
	t.Run("non-ascii accepted", func(t *testing.T) {
		claims := standardClaims(now, "https://as/", "https://mcp/", time.Hour)
		claims["sub"] = "alice@école.fr"
		token := signRS256(t, priv, "k", claims)
		res, err := verifyJWT(token, jwtVerifyOptions{
			ExpectedIssuer:   "https://as/",
			ExpectedAudience: "https://mcp/",
			Keys:             keys,
			Clock:            newFixedClock(now),
		})
		if err != nil {
			t.Fatalf("valid unicode sub rejected: %v", err)
		}
		if res.Subject != "alice@école.fr" {
			t.Fatalf("subject round-trip: %q", res.Subject)
		}
	})
}

func TestResourceMetadataURL_RejectsMalformedBase(t *testing.T) {
	// Operator-supplied audience with query/fragment/userinfo must not
	// produce a malformed URL in the 401 challenge.
	cases := []OAuthConfig{
		{Audience: "https://mcp.example/?x=1"},
		{Audience: "https://mcp.example/#section"},
		{Audience: "https://user@mcp.example/"},
		{Audience: "not-a-url"},
	}
	for _, cfg := range cases {
		t.Run(cfg.Audience, func(t *testing.T) {
			if got := resourceMetadataURL(cfg); got != "" {
				t.Fatalf("resourceMetadataURL(%q) = %q, want empty for malformed base", cfg.Audience, got)
			}
		})
	}
	// Well-formed audience produces the expected URL.
	cfg := OAuthConfig{Audience: "https://mcp.example/"}
	want := "https://mcp.example/.well-known/oauth-protected-resource"
	if got := resourceMetadataURL(cfg); got != want {
		t.Fatalf("resourceMetadataURL = %q, want %q", got, want)
	}
}

func TestNewStreamable_OAuth_MetadataCORS(t *testing.T) {
	// Browser SPA loaded from a foreign origin must be able to discover
	// the AS via /.well-known/oauth-protected-resource. The origin check
	// MUST NOT gate this endpoint, and CORS wildcard + preflight headers
	// MUST be emitted.
	as := newTestAS(t)
	s, err := NewStreamable(StreamableConfig{
		AuthMode: AuthOAuth,
		OAuth: OAuthConfig{
			AuthorizationServer: as.Issuer(),
			Audience:            "https://mcp.example/",
		},
	})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()

	// Cross-origin GET succeeds despite non-loopback Origin header.
	req := httptest.NewRequest(http.MethodGet, OAuthMetadataPath, http.NoBody)
	req.Header.Set("Origin", "https://app.example/")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("cross-origin GET status = %d, want 200", w.Code)
	}
	if ao := w.Header().Get("Access-Control-Allow-Origin"); ao != "*" {
		t.Fatalf("ACAO = %q, want *", ao)
	}

	// Preflight OPTIONS returns 204 + allows GET.
	req2 := httptest.NewRequest(http.MethodOptions, OAuthMetadataPath, http.NoBody)
	req2.Header.Set("Origin", "https://app.example/")
	req2.Header.Set("Access-Control-Request-Method", "GET")
	w2 := httptest.NewRecorder()
	s.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d, want 204", w2.Code)
	}
	if m := w2.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(m, "GET") {
		t.Fatalf("ACAM = %q, want to contain GET", m)
	}
}

func TestValidateJWKSURI_RejectsEmptyHost(t *testing.T) {
	// jwks_uri with scheme but no authority should fail at startup, not at
	// fetch time with an obscure "no Host in request URL" error.
	err := validateJWKSURI("https://as/", "https:///jwks")
	if err == nil {
		t.Fatal("expected rejection for empty-authority jwks_uri")
	}
	if !strings.Contains(err.Error(), "missing host") {
		t.Fatalf("error should mention missing host, got: %v", err)
	}
}

func TestBearerList_DuplicateTokensCollapseToFirstMatch(t *testing.T) {
	// Companion to the Validate-time rejection in
	// internal/component/config/loader_extract_test.go. Asserts the
	// runtime behavior the Validate check is designed to prevent: two
	// identities with the same token collapse to the first match, the
	// second identity's scopes are unreachable. A regression where the
	// runtime silently prefers a later identity (or randomizes) would be
	// caught here.
	a := bearerListAuthenticator{entries: []bearerListEntry{
		{name: "alice", hash: hashToken("shared"), scopes: []string{"first"}},
		{name: "bob", hash: hashToken("shared"), scopes: []string{"second-unreachable"}},
	}}
	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)
	r.Header.Set("Authorization", "Bearer shared")
	id, err := a.Authenticate(r)
	if err != nil {
		t.Fatalf("shared-token auth: %v", err)
	}
	if id.Name != "alice" || !id.HasScope("first") {
		t.Fatalf("first match wins: got name=%q scopes=%v, want alice/[first]", id.Name, id.Scopes)
	}
	if id.HasScope("second-unreachable") {
		t.Fatalf("second identity's scope leaked into first match: %v", id.Scopes)
	}
}

func TestCanonicalAuthServerURL_RejectsUserInfo(t *testing.T) {
	// Per RFC 8414 §2, issuer identifiers must not carry userinfo.
	// Silently stripping it would collapse URLs-with-credentials and
	// URLs-without into the same canonical form.
	cases := []string{
		"https://user@as.example/",
		"https://user:pass@as.example/",
	}
	for _, tc := range cases {
		if _, err := canonicalAuthServerURL(tc); err == nil {
			t.Errorf("canonicalAuthServerURL(%q) accepted; wanted rejection", tc)
		}
	}
}

func TestValidateJWKSURI_RejectsMalformedAS(t *testing.T) {
	// A typo'd AS scheme (e.g. `htps://`) must fail-closed. Previously the
	// mirror-scheme check only fired on `https`, so a malformed AS would
	// silently admit any jwks scheme (including HTTP on an HTTPS-intended
	// deployment).
	cases := []struct {
		name string
		as   string
		jwks string
	}{
		{"typo scheme", "htps://as/", "http://as/jwks"},
		{"empty scheme", "//as/", "https://as/jwks"},
		{"ftp scheme", "ftp://as/", "https://as/jwks"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateJWKSURI(tc.as, tc.jwks); err == nil {
				t.Fatalf("validateJWKSURI(%q, %q) accepted; wanted rejection", tc.as, tc.jwks)
			}
		})
	}
}

func TestCanonicalAuthServerURL_IPv6(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://[::1]:443/", "https://[::1]"},
		{"https://[::1]/", "https://[::1]"},
		{"https://[::1]:8443/realm/x/", "https://[::1]:8443/realm/x"},
	}
	for _, tc := range cases {
		got, err := canonicalAuthServerURL(tc.in)
		if err != nil {
			t.Fatalf("canonicalAuthServerURL(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("canonicalAuthServerURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestValidateJWKSURI(t *testing.T) {
	cases := []struct {
		name    string
		as      string
		jwks    string
		wantErr bool
	}{
		{"empty jwks rejected", "https://as/", "", true},
		{"https as + https jwks ok", "https://as/", "https://as/jwks", false},
		{"https as + http jwks rejected (downgrade)", "https://as/", "http://cdn/jwks", true},
		{"http as + http jwks ok", "http://as/", "http://as/jwks", false},
		{"https as + file:// jwks rejected", "https://as/", "file:///etc/keys.json", true},
		{"case-insensitive scheme match", "HTTPS://as/", "HTTPS://as/jwks", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJWKSURI(tc.as, tc.jwks)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateJWKSURI(%q, %q) err = %v, wantErr = %v",
					tc.as, tc.jwks, err, tc.wantErr)
			}
		})
	}
}

func TestNewStreamable_OAuth_MetadataUsesCanonicalIssuer(t *testing.T) {
	// The RFC 9728 document MUST advertise md.Issuer (AS-reported form) so
	// the string clients receive is byte-identical to what tokens carry in
	// the `iss` claim. Operator-configured URLs that differ only by
	// canonical-trivial elements (trailing slash, case-folding, default
	// port) MUST resolve to the AS-reported form in the published doc.
	//
	// Helper spins up an AS that reports a specific issuer string, builds
	// a Streamable with a given operator-configured URL, and asserts the
	// metadata handler advertises the AS-reported issuer.
	runCase := func(t *testing.T, operatorURL string, asIssuer func(srvURL string) string) {
		t.Helper()
		mux := http.NewServeMux()
		var srv *httptest.Server
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("rsa: %v", err)
		}
		mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
			body, _ := json.Marshal(map[string]any{
				"issuer":   asIssuer(srv.URL),
				"jwks_uri": srv.URL + "/jwks",
			})
			if _, werr := w.Write(body); werr != nil {
				t.Logf("write metadata: %v", werr)
			}
		})
		mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
			body, _ := json.Marshal(map[string]any{
				"keys": []map[string]any{rsaJWK(t, priv, "k1")},
			})
			if _, werr := w.Write(body); werr != nil {
				t.Logf("write jwks: %v", werr)
			}
		})
		srv = httptest.NewServer(mux)
		defer srv.Close()

		// operatorURL may embed {srv} (full URL) or {host} (host:port) so
		// we can express case-folded or slash-variant flavors.
		cfgURL := strings.Replace(operatorURL, "{srv}", srv.URL, 1)
		hostPort := strings.TrimPrefix(srv.URL, "http://")
		cfgURL = strings.Replace(cfgURL, "{host}", hostPort, 1)
		s, err := NewStreamable(StreamableConfig{
			AuthMode: AuthOAuth,
			OAuth: OAuthConfig{
				AuthorizationServer: cfgURL,
				Audience:            "https://mcp.example/",
			},
		})
		if err != nil {
			t.Fatalf("NewStreamable (operator=%q): %v", cfgURL, err)
		}
		defer s.Close()

		req := httptest.NewRequest(http.MethodGet, OAuthMetadataPath, http.NoBody)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		var got map[string]any
		if uerr := json.Unmarshal(w.Body.Bytes(), &got); uerr != nil {
			t.Fatalf("decode: %v", uerr)
		}
		wantIssuer := asIssuer(srv.URL)
		servers, _ := got["authorization_servers"].([]any)
		if len(servers) != 1 || servers[0] != wantIssuer {
			t.Fatalf("authorization_servers[0] = %v, want %q (AS-reported)",
				servers, wantIssuer)
		}
	}

	t.Run("trailing slash divergence", func(t *testing.T) {
		// Operator typed a trailing slash; AS reports without.
		runCase(t, "{srv}/", func(srvURL string) string { return srvURL })
	})

	t.Run("case-folding divergence", func(t *testing.T) {
		// Operator typed uppercase scheme; AS reports the lowercase form.
		// Build the operator URL by uppercasing srv.URL's scheme portion.
		runCase(t, "HTTP://{host}", func(srvURL string) string { return srvURL })
	})
}

// resolveOperatorURLVariant substitutes {srv} / {host} placeholders for the
// httptest server's URL. Kept here so runCase's template logic stays tiny.

// -----------------------------------------------------------------------------
// resourceMetadataURL derivation
// -----------------------------------------------------------------------------

func TestResourceMetadataURL(t *testing.T) {
	cases := []struct {
		name string
		cfg  OAuthConfig
		want string
	}{
		{"audience only", OAuthConfig{Audience: "https://mcp.example/"}, "https://mcp.example/.well-known/oauth-protected-resource"},
		{"audience trailing-slash stripped", OAuthConfig{Audience: "https://mcp.example///"}, "https://mcp.example/.well-known/oauth-protected-resource"},
		{"explicit metadata-resource wins", OAuthConfig{Audience: "https://aud/", MetadataResource: "https://meta/"}, "https://meta/.well-known/oauth-protected-resource"},
		{"neither set", OAuthConfig{}, ""},
	}
	for _, tc := range cases {
		if got := resourceMetadataURL(tc.cfg); got != tc.want {
			t.Errorf("%s: resourceMetadataURL = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// Assert at compile time that stubJWKS satisfies jwksLookup (guards against
// test-helper drift).
var _ jwksLookup = (*stubJWKS)(nil)

// Keep crypto import tethered for potential future use.
var _ = crypto.SHA256
