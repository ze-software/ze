package mcp

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// oauthAuthenticator unit tests (AC-7, AC-9 variants)
// -----------------------------------------------------------------------------

// Every oauth test pins the expected issuer to https://as/ and audience to
// https://mcp/; token claims are varied case-by-case.
func newOAuthAuth(t *testing.T, priv *rsa.PrivateKey, scopes []string) oauthAuthenticator {
	t.Helper()
	keys := &stubJWKS{keys: map[string]crypto.PublicKey{"k": &priv.PublicKey}}
	a, err := buildOAuthAuthenticator(OAuthConfig{
		AuthorizationServer: "https://as/",
		Audience:            "https://mcp/",
		RequiredScopes:      scopes,
	}, keys, "https://mcp.example/.well-known/oauth-protected-resource")
	if err != nil {
		t.Fatalf("buildOAuthAuthenticator: %v", err)
	}
	a.clock = func() time.Time { return time.Unix(1_700_000_000, 0) }
	return a
}

func TestOAuth_Authenticate_MissingHeader(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	a := newOAuthAuth(t, priv, nil)
	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)

	_, aerr := a.Authenticate(r)
	if aerr == nil {
		t.Fatal("expected challenge error on missing Authorization")
	}
	if aerr.ErrorCode != "invalid_request" {
		t.Fatalf("error_code = %q, want invalid_request", aerr.ErrorCode)
	}
	if !strings.Contains(aerr.WWWAuthenticate(), `resource_metadata="https://mcp.example/`) {
		t.Fatalf("WWW-Authenticate missing resource_metadata: %q", aerr.WWWAuthenticate())
	}
}

func TestOAuth_Authenticate_ValidToken(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	a := newOAuthAuth(t, priv, nil)

	now := time.Unix(1_700_000_000, 0)
	token := signRS256(t, priv, "k", standardClaims(now, "https://as/", "https://mcp/", time.Hour))

	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)
	r.Header.Set("Authorization", "Bearer "+token)

	id, aerr := a.Authenticate(r)
	if aerr != nil {
		t.Fatalf("valid token rejected: %v", aerr)
	}
	if id.Name != "alice" {
		t.Fatalf("subject = %q, want alice", id.Name)
	}
	if !id.HasScope("mcp.read") {
		t.Fatalf("scopes = %v", id.Scopes)
	}
}

func TestOAuth_Authenticate_WrongAudience(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	a := newOAuthAuth(t, priv, nil)
	now := time.Unix(1_700_000_000, 0)
	token := signRS256(t, priv, "k", standardClaims(now, "https://as/", "https://other/", time.Hour))

	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)
	r.Header.Set("Authorization", "Bearer "+token)

	_, aerr := a.Authenticate(r)
	if aerr == nil {
		t.Fatal("expected challenge")
	}
	if aerr.ErrorDescription != "invalid audience" {
		t.Fatalf("desc = %q", aerr.ErrorDescription)
	}
}

func TestOAuth_Authenticate_WrongIssuer(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	a := newOAuthAuth(t, priv, nil)
	now := time.Unix(1_700_000_000, 0)
	token := signRS256(t, priv, "k", standardClaims(now, "https://other/", "https://mcp/", time.Hour))

	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)
	r.Header.Set("Authorization", "Bearer "+token)

	_, aerr := a.Authenticate(r)
	if aerr == nil || aerr.ErrorDescription != "invalid issuer" {
		t.Fatalf("aerr = %+v", aerr)
	}
}

func TestOAuth_Authenticate_Expired(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	a := newOAuthAuth(t, priv, nil)
	now := time.Unix(1_700_000_000, 0)
	token := signRS256(t, priv, "k", standardClaims(now, "https://as/", "https://mcp/", -2*time.Minute))

	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)
	r.Header.Set("Authorization", "Bearer "+token)

	_, aerr := a.Authenticate(r)
	if aerr == nil || aerr.ErrorDescription != "token expired" {
		t.Fatalf("aerr = %+v", aerr)
	}
}

func TestOAuth_Authenticate_InsufficientScope(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	a := newOAuthAuth(t, priv, []string{"mcp.admin"})
	now := time.Unix(1_700_000_000, 0)
	// Token only has mcp.read + mcp.write (from standardClaims).
	token := signRS256(t, priv, "k", standardClaims(now, "https://as/", "https://mcp/", time.Hour))

	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)
	r.Header.Set("Authorization", "Bearer "+token)

	_, aerr := a.Authenticate(r)
	if aerr == nil {
		t.Fatal("expected insufficient_scope")
	}
	if aerr.ErrorCode != "insufficient_scope" {
		t.Fatalf("code = %q", aerr.ErrorCode)
	}
	if aerr.Scope != "mcp.admin" {
		t.Fatalf("scope param = %q, want mcp.admin", aerr.Scope)
	}
	// WWW-Authenticate must contain the scope parameter.
	if !strings.Contains(aerr.WWWAuthenticate(), `scope="mcp.admin"`) {
		t.Fatalf("WWW-Authenticate = %q", aerr.WWWAuthenticate())
	}
}

func TestOAuth_Authenticate_AlgNoneRejected(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	a := newOAuthAuth(t, priv, nil)

	// Build an alg=none token by hand.
	header, _ := json.Marshal(map[string]string{"alg": "none"})
	payload, _ := json.Marshal(map[string]any{"sub": "x"})
	token := b64url(header) + "." + b64url(payload) + "."

	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)
	r.Header.Set("Authorization", "Bearer "+token)

	_, aerr := a.Authenticate(r)
	if aerr == nil || aerr.ErrorDescription != "alg=none is not accepted" {
		t.Fatalf("aerr = %+v", aerr)
	}
}

func TestOAuth_BuildAuth_RequiresAudience(t *testing.T) {
	_, err := buildOAuthAuthenticator(OAuthConfig{AuthorizationServer: "x"}, &stubJWKS{}, "")
	if err == nil {
		t.Fatal("expected error when audience missing")
	}
}

func TestOAuth_BuildAuth_RequiresAS(t *testing.T) {
	_, err := buildOAuthAuthenticator(OAuthConfig{Audience: "x"}, &stubJWKS{}, "")
	if err == nil {
		t.Fatal("expected error when authorization-server missing")
	}
}

func TestOAuth_BuildAuth_RequiresKeys(t *testing.T) {
	_, err := buildOAuthAuthenticator(OAuthConfig{Audience: "a", AuthorizationServer: "x"}, nil, "")
	if err == nil {
		t.Fatal("expected error when keys nil")
	}
}

// -----------------------------------------------------------------------------
// RFC 9728 protected-resource metadata handler (AC-8)
// -----------------------------------------------------------------------------

func TestResourceMetadata_Document(t *testing.T) {
	cfg := OAuthConfig{
		AuthorizationServer: "https://as.example/",
		Audience:            "https://mcp.example/",
		RequiredScopes:      []string{"mcp.read", "mcp.admin"},
	}
	w := httptest.NewRecorder()
	writeResourceMetadata(w, cfg)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q", ct)
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["resource"] != "https://mcp.example/" {
		t.Fatalf("resource = %v", got["resource"])
	}
	as, ok := got["authorization_servers"].([]any)
	if !ok || len(as) != 1 || as[0] != "https://as.example/" {
		t.Fatalf("authorization_servers = %v", got["authorization_servers"])
	}
	bm, ok := got["bearer_methods_supported"].([]any)
	if !ok || len(bm) != 1 || bm[0] != "header" {
		t.Fatalf("bearer_methods_supported = %v", got["bearer_methods_supported"])
	}
	scopes, ok := got["scopes_supported"].([]any)
	if !ok || len(scopes) != 2 {
		t.Fatalf("scopes_supported = %v", got["scopes_supported"])
	}
}

func TestResourceMetadata_ResourceFallsBackToAudience(t *testing.T) {
	cfg := OAuthConfig{
		AuthorizationServer: "https://as/",
		Audience:            "https://aud/",
	}
	w := httptest.NewRecorder()
	writeResourceMetadata(w, cfg)
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["resource"] != "https://aud/" {
		t.Fatalf("resource = %v, want audience fallback", got["resource"])
	}
}

func TestResourceMetadata_ExplicitMetadataResourceWins(t *testing.T) {
	cfg := OAuthConfig{
		AuthorizationServer: "https://as/",
		Audience:            "https://aud/",
		MetadataResource:    "https://explicit/",
	}
	w := httptest.NewRecorder()
	writeResourceMetadata(w, cfg)
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["resource"] != "https://explicit/" {
		t.Fatalf("resource = %v, want explicit override", got["resource"])
	}
}

// -----------------------------------------------------------------------------
// Streamable HTTP integration: metadata endpoint gated by AuthMode (AC-8a)
// -----------------------------------------------------------------------------

func TestStreamable_MetadataEndpoint_Gated(t *testing.T) {
	// AuthMode=Bearer: metadata URL 404s.
	s, err := NewStreamable(StreamableConfig{AuthMode: AuthBearer, Token: "x"})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, OAuthMetadataPath, http.NoBody)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("metadata status = %d for AuthMode=Bearer, want 404", w.Code)
	}
}

// -----------------------------------------------------------------------------
// b64url helper
// -----------------------------------------------------------------------------

func b64url(b []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	// Encode manually (avoid re-importing encoding/base64 in a helper).
	var sb strings.Builder
	buf := b
	for len(buf) >= 3 {
		v := uint32(buf[0])<<16 | uint32(buf[1])<<8 | uint32(buf[2])
		sb.WriteByte(alphabet[(v>>18)&0x3f])
		sb.WriteByte(alphabet[(v>>12)&0x3f])
		sb.WriteByte(alphabet[(v>>6)&0x3f])
		sb.WriteByte(alphabet[v&0x3f])
		buf = buf[3:]
	}
	switch len(buf) {
	case 1:
		v := uint32(buf[0]) << 16
		sb.WriteByte(alphabet[(v>>18)&0x3f])
		sb.WriteByte(alphabet[(v>>12)&0x3f])
	case 2:
		v := uint32(buf[0])<<16 | uint32(buf[1])<<8
		sb.WriteByte(alphabet[(v>>18)&0x3f])
		sb.WriteByte(alphabet[(v>>12)&0x3f])
		sb.WriteByte(alphabet[(v>>6)&0x3f])
	}
	return sb.String()
}
