package mcp

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
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

// newTrustedTLSServer exercises the production default HTTP client against a
// certificate trusted only for this test. Callers MUST NOT run in parallel.
func newTrustedTLSServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	original := http.DefaultTransport
	base, ok := original.(*http.Transport)
	if !ok {
		t.Fatalf("http.DefaultTransport = %T, want *http.Transport", original)
	}
	transport := base.Clone()
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	if transport.TLSClientConfig.RootCAs == nil {
		transport.TLSClientConfig.RootCAs = x509.NewCertPool()
	} else {
		transport.TLSClientConfig.RootCAs = transport.TLSClientConfig.RootCAs.Clone()
	}
	transport.TLSClientConfig.RootCAs.AddCert(srv.Certificate())
	http.DefaultTransport = transport
	t.Cleanup(func() {
		http.DefaultTransport = original
		transport.CloseIdleConnections()
		srv.Close()
	})
	return srv
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
	srv := newTrustedTLSServer(t, mux)
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
		w.Header().Set("Content-Type", "application/json")
		if _, werr := w.Write(body); werr != nil {
			t.Logf("write: %v", werr)
		}
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		if _, werr := w.Write([]byte(`{"keys":[]}`)); werr != nil {
			t.Logf("write: %v", werr)
		}
	})
	badSrv = newTrustedTLSServer(t, mux)
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
	// RFC requirement: RFC8414-3.3-2 negative -- an AS whose metadata issuer differs from the configured authorization-server is rejected
	if !strings.Contains(err.Error(), "does not match configured") {
		t.Fatalf("error should name issuer mismatch, got %v", err)
	}
}

// -----------------------------------------------------------------------------
// End-to-end OAuth auth through NewStreamable + ServeHTTP
// -----------------------------------------------------------------------------

func TestNewStreamable_OAuth_AcceptsValidToken(t *testing.T) {
	// RFC requirement: RFC8414-3.3-2 positive -- when the AS-reported issuer equals the configured authorization-server, NewStreamable builds and the minted token verifies
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
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":` +
		metaBlock(ProtocolVersion, capsNone) + `}}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	req.Header.Set("Mcp-Method", "tools/list")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Mcp-Session-Id"); got != "" {
		t.Fatalf("response minted Mcp-Session-Id = %q; this revision has no sessions", got)
	}
	// The verified token's subject is the per-request Identity every handler
	// runs as. Asserted at the producer rather than through a session, which
	// no longer exists.
	identity, aerr := s.authenticate(req)
	if aerr != nil {
		t.Fatalf("authenticate: %v", aerr)
	}
	if identity.Name != "alice" {
		t.Fatalf("identity.Name = %q, want alice", identity.Name)
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

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":` +
		metaBlock(ProtocolVersion, capsNone) + `}}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	req.Header.Set("Mcp-Method", "tools/list")
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
	// RFC requirement: RFC9728-5.1-1 positive -- the 401 WWW-Authenticate includes resource_metadata (auth.go:177 appends it; oauth.go:122 sets it on the challenge)
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
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":` +
		metaBlock(ProtocolVersion, capsNone) + `}}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	req.Header.Set("Mcp-Method", "tools/list")
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, OAuthMetadataPath, http.NoBody)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	// The unauthenticated metadata GET is part of Ze's discovery profile.
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// RFC requirement: RFC9728-2-1 positive -- the served metadata document contains the resource field (writeResourceMetadata, oauth.go:170-171,184-192)
	if got["resource"] != "https://mcp.example/" {
		t.Fatalf("resource = %v", got["resource"])
	}
	// RFC requirement: RFC9728-2-2 positive -- the served metadata document contains authorization_servers as a JSON array (writeResourceMetadata, oauth.go:172,190)
	servers, ok := got["authorization_servers"].([]any)
	if !ok || len(servers) != 1 || servers[0] != as.Issuer() {
		t.Fatalf("authorization_servers = %v, want [%q]", got["authorization_servers"], as.Issuer())
	}
}

// TestAudClaimMatchesExactIdentifier checks that URL spellings cannot broaden
// the audience bound into an access token.
func TestAudClaimMatchesExactIdentifier(t *testing.T) {
	configured := "https://mcp.example/"
	if !(audClaim{configured}).Matches(configured) {
		t.Fatal("identical audience rejected")
	}
	for _, audience := range []string{
		"https://mcp.example",
		"https://mcp.example:443/",
		"https://MCP.EXAMPLE/",
		"https://mcp.example///",
		"https://mcp.example/?tenant=other",
		"https://user@mcp.example/",
		"https://mcp.example/#other",
		"https://other.example/",
		"http://mcp.example/",
		"https://mcp.example/path",
	} {
		if (audClaim{audience}).Matches(configured) {
			t.Fatalf("audience %q matched %q", audience, configured)
		}
	}
}

// TestOAuthAudienceIdentity sends tokens through the HTTP authentication entry.
// It distinguishes exact matches from slash, query, and Unicode aliases.
// RFC requirement: RFC8707-5-1 positive -- historical ID sourced to RFC 7519 Sections 2 and 4.1.3: a token with the exact configured audience reaches the MCP tools/list HTTP handler.
// RFC requirement: RFC8707-5-1 negative -- historical ID sourced to RFC 7519 Sections 2 and 4.1.3: tokens differing from the configured audience by slash, query or Unicode spelling receive HTTP 401 invalid audience.
func TestOAuthAudienceIdentity(t *testing.T) {
	as := newTestAS(t)
	s, err := NewStreamable(StreamableConfig{
		AuthMode: AuthOAuth,
		OAuth: OAuthConfig{
			AuthorizationServer: as.Issuer(),
			Audience:            "https://mcp.example/\u00e9",
		},
	})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()
	for _, audience := range []string{
		"https://mcp.example/\u00e9",
		"https://mcp.example/\u00e9/",
		"https://mcp.example/e\u0301",
		"https://mcp.example/\u00e9?tenant=other",
	} {
		token := as.MintToken(t, map[string]any{"aud": audience})
		body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":` +
			metaBlock(ProtocolVersion, capsNone) + `}}`
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
		req.Header.Set("Mcp-Method", "tools/list")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		if audience == s.cfg.OAuth.Audience {
			if w.Code != http.StatusOK {
				t.Fatalf("exact audience: status=%d body=%s", w.Code, w.Body.String())
			}
			continue
		}
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("different audience %q: status=%d", audience, w.Code)
		}
		if !strings.Contains(w.Header().Get("WWW-Authenticate"), `error_description="invalid audience"`) {
			t.Fatalf("different audience %q: challenge=%q", audience, w.Header().Get("WWW-Authenticate"))
		}
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, Endpoint, http.NoBody)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, Endpoint, http.NoBody)
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
	// WWW-Authenticate.
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

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":` +
		metaBlock(ProtocolVersion, capsNone) + `}}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	req.Header.Set("Mcp-Method", "tools/list")
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Origin", "https://app.example/")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("tools/list status = %d, body = %s", w.Code, w.Body.String())
	}
	if ao := w.Header().Get("Access-Control-Allow-Origin"); ao != "https://app.example/" {
		t.Fatalf("ACAO on success = %q, want echoed origin", ao)
	}
	if ac := w.Header().Get("Access-Control-Allow-Credentials"); ac != "true" {
		t.Fatalf("ACAC on success = %q, want true", ac)
	}
	if expose := w.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(expose, "WWW-Authenticate") {
		t.Fatalf("ACEH missing WWW-Authenticate: %q", expose)
	}
	if expose := w.Header().Get("Access-Control-Expose-Headers"); strings.Contains(expose, "Mcp-Session-Id") {
		t.Fatalf("ACEH still advertises the removed Mcp-Session-Id: %q", expose)
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

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":` +
		metaBlock(ProtocolVersion, capsNone) + `}}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	req.Header.Set("Mcp-Method", "tools/list")
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/mcp/extra", http.NoBody)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, Endpoint, http.NoBody)
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

// TestAudClaimDistinguishesIDN proves audience identity is independent of DNS
// equivalence between Unicode and punycode spellings.
func TestAudClaimDistinguishesIDN(t *testing.T) {
	configured := "https://müllerei.example/"
	claim := audClaim{"https://xn--mllerei-n2a.example/"}
	if claim.Matches(configured) {
		t.Fatal("punycode audience matched a different Unicode identifier")
	}
	if !(audClaim{configured}).Matches(configured) {
		t.Fatal("identical Unicode audience rejected")
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

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":` +
		metaBlock(ProtocolVersion, capsNone) + `}}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	req.Header.Set("Mcp-Method", "tools/list")
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, Endpoint, http.NoBody)
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
	// A fragment, userinfo, or non-HTTPS identifier cannot locate metadata.
	cases := []OAuthConfig{
		{Audience: "https://mcp.example/#section"},
		{Audience: "https://user@mcp.example/"},
		{Audience: "not-a-url"},
		{Audience: "http://mcp.example/"},
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, OAuthMetadataPath, http.NoBody)
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
	req2 := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, OAuthMetadataPath, http.NoBody)
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

func TestOAuthHTTPSURLRejectsEmptyHost(t *testing.T) {
	if _, err := oauthHTTPSURL("https:///jwks"); err == nil {
		t.Fatal("accepted an HTTPS URL without a host")
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, http.NoBody)
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

func TestOAuthHTTPSURL(t *testing.T) {
	for _, raw := range []string{
		"https://as/jwks",
		"https://as/jwks?version=2",
		"HTTPS://as/jwks",
	} {
		if _, err := oauthHTTPSURL(raw); err != nil {
			t.Fatalf("valid URL %q: %v", raw, err)
		}
	}
	for _, raw := range []string{
		"", "http://as/jwks", "file:///etc/keys.json", "htps://as/jwks",
		"https://user@as/jwks", "https://as/jwks#key", "https://as/jwks#",
	} {
		if _, err := oauthHTTPSURL(raw); err == nil {
			t.Fatalf("invalid URL %q accepted", raw)
		}
	}
}

// TestOAuthRejectsIssuerAliases proves discovery never trusts metadata whose
// issuer differs only in spelling from the configured trust anchor.
// RFC requirement: RFC8414-3.3-2 negative -- issuer slash, path, and query aliases fail startup before their JWKS URL is requested.
func TestOAuthRejectsIssuerAliases(t *testing.T) {
	for _, suffix := range []string{"/", "/realm/../", "?tenant=other"} {
		t.Run(suffix, func(t *testing.T) {
			var issuer string
			var keyRequests atomic.Int64
			srv := newTrustedTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/jwks" {
					keyRequests.Add(1)
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(map[string]any{
					"issuer": issuer + suffix, "jwks_uri": issuer + "/jwks",
				}); err != nil {
					t.Errorf("write metadata: %v", err)
				}
			}))
			issuer = srv.URL
			s, err := NewStreamable(StreamableConfig{
				AuthMode: AuthOAuth,
				OAuth:    OAuthConfig{AuthorizationServer: issuer, Audience: "https://mcp.example/"},
			})
			if s != nil {
				s.Close()
				t.Fatal("mismatched issuer returned a server")
			}
			if err == nil || !strings.Contains(err.Error(), "does not match configured") {
				t.Fatalf("expected issuer mismatch, got %v", err)
			}
			if keyRequests.Load() != 0 {
				t.Fatal("used the mismatched document's JWKS")
			}
		})
	}
}

// TestRFC8414JWKSMetadataSecurity drives metadata and JWKS retrieval through
// NewStreamable, then submits an access token to the HTTP endpoint.
// RFC requirement: RFC8414-2-7 positive -- an HTTPS jwks_uri supplies the key that authenticates a signed request.
// RFC requirement: RFC8414-2-7 negative -- HTTP jwks_uri and HTTPS redirects to HTTP both fail startup before a plaintext key request.
// RFC requirement: RFC8414-2-8 positive -- a mixed signing/encryption JWKS with explicit use values verifies a token with its signing key.
// RFC requirement: RFC8414-2-8 negative -- a mixed JWKS containing an unlabelled key is rejected before token verification.
func TestRFC8414JWKSMetadataSecurity(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	for _, scenario := range []string{"labeled keys", "missing use", "HTTP keys", "redirect to HTTP"} {
		t.Run(scenario, func(t *testing.T) {
			var plainHits atomic.Int64
			plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				plainHits.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer plain.Close()
			var issuer string
			srv := newTrustedTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == asMetadataWellKnownPath {
					keys := issuer + "/jwks"
					if scenario == "HTTP keys" {
						keys = plain.URL + "/jwks"
					}
					if err := json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "jwks_uri": keys}); err != nil {
						t.Errorf("write metadata: %v", err)
					}
					return
				}
				if scenario == "redirect to HTTP" {
					http.Redirect(w, r, plain.URL+"/jwks", http.StatusFound)
					return
				}
				signing := rsaJWK(t, priv, "signing")
				encryption := rsaJWK(t, priv, "encryption")
				encryption["use"] = "enc"
				if scenario == "missing use" {
					delete(signing, "use")
				}
				if err := json.NewEncoder(w).Encode(map[string]any{
					"keys": []map[string]any{signing, encryption},
				}); err != nil {
					t.Errorf("write keys: %v", err)
				}
			}))
			issuer = srv.URL
			s, err := NewStreamable(StreamableConfig{
				AuthMode: AuthOAuth,
				OAuth:    OAuthConfig{AuthorizationServer: issuer, Audience: "https://mcp.example/mcp"},
			})
			if scenario != "labeled keys" {
				if s != nil {
					s.Close()
					t.Fatal("unsafe metadata returned a server")
				}
				if err == nil {
					t.Fatal("unsafe metadata returned no startup error")
				}
				if plainHits.Load() != 0 {
					t.Fatal("JWKS retrieval reached the plaintext destination")
				}
				return
			}
			if err != nil {
				t.Fatalf("valid metadata: %v", err)
			}
			defer s.Close()
			claims := standardClaims(time.Now(), issuer, "https://mcp.example/mcp", time.Hour)
			token := signRS256(t, priv, "signing", claims)
			body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":` +
				metaBlock(ProtocolVersion, capsNone) + `}}`
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
			req.Header.Set("Mcp-Method", "tools/list")
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			s.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("valid signing key did not authenticate: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

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

// An invalid typed mode must fail construction instead of selecting anonymous
// authentication through the dispatcher's default arm.
func TestNewStreamableRejectsUnknownAuthenticationMode(t *testing.T) {
	s, err := NewStreamable(StreamableConfig{AuthMode: AuthMode(255)})
	if s != nil {
		s.Close()
		t.Fatal("unknown authentication mode constructed a server")
	}
	if err == nil {
		t.Fatal("unknown authentication mode returned no error")
	}
}

// URL validation errors are printed when the listener cannot start. Userinfo
// credentials must not reach that operator-visible error text.
func TestOAuthURLValidationDoesNotExposeCredentials(t *testing.T) {
	for _, raw := range []string{
		"https://secret-token@as.example/jwks",
		"http://secret-token:password@as.example/jwks",
		"https://secret-token:password@as.example/%invalid",
	} {
		_, err := oauthHTTPSURL(raw)
		if err == nil {
			t.Fatalf("credential-bearing URL accepted")
		}
		for _, secret := range []string{"secret-token", "password"} {
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("URL rejection exposed a credential")
			}
		}
	}
}

// Assert at compile time that stubJWKS satisfies jwksLookup (guards against
// test-helper drift).
var _ jwksLookup = (*stubJWKS)(nil)

// Keep crypto import tethered for potential future use.
var _ = crypto.SHA256
