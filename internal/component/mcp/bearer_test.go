package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------------
// bearerAuthenticator (legacy single token)
// -----------------------------------------------------------------------------

func TestBearerAuthenticator_ValidToken(t *testing.T) {
	a := bearerAuthenticator{hash: hashToken("sekret")}
	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)
	r.Header.Set("Authorization", "Bearer sekret")

	id, err := a.Authenticate(r)
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if !id.IsAnonymous() {
		t.Fatalf("bearer mode identity should be anonymous, got %+v", id)
	}
}

func TestBearerAuthenticator_MissingHeader(t *testing.T) {
	a := bearerAuthenticator{hash: hashToken("sekret")}
	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)

	_, err := a.Authenticate(r)
	if err == nil {
		t.Fatal("missing Authorization should be rejected")
	}
	if err.Status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", err.Status)
	}
	if err.ErrorCode != "invalid_request" {
		t.Fatalf("error_code = %q, want invalid_request", err.ErrorCode)
	}
}

func TestBearerAuthenticator_WrongToken(t *testing.T) {
	a := bearerAuthenticator{hash: hashToken("sekret")}
	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)
	r.Header.Set("Authorization", "Bearer nope")

	_, err := a.Authenticate(r)
	if err == nil {
		t.Fatal("wrong token should be rejected")
	}
	if err.ErrorCode != "invalid_token" {
		t.Fatalf("error_code = %q, want invalid_token", err.ErrorCode)
	}
}

func TestBearerAuthenticator_LowercaseBearerSchemeAccepted(t *testing.T) {
	// RFC 7235: auth-scheme is case-insensitive.
	a := bearerAuthenticator{hash: hashToken("sekret")}
	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)
	r.Header.Set("Authorization", "bearer sekret")

	if _, err := a.Authenticate(r); err != nil {
		t.Fatalf("lowercase bearer rejected: %v", err)
	}
}

func TestBearerAuthenticator_WrongScheme(t *testing.T) {
	a := bearerAuthenticator{hash: hashToken("sekret")}
	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)
	r.Header.Set("Authorization", "Basic sekret")

	_, err := a.Authenticate(r)
	if err == nil {
		t.Fatal("Basic scheme should be rejected")
	}
}

// -----------------------------------------------------------------------------
// bearerListAuthenticator (AC-10 / AC-11)
// -----------------------------------------------------------------------------

func TestBearerListAuthenticator_ValidIdentity(t *testing.T) {
	a := bearerListAuthenticator{entries: []bearerListEntry{
		{name: "alice", hash: hashToken("alice-token"), scopes: []string{"mcp.read", "mcp.write"}},
		{name: "bob", hash: hashToken("bob-token")},
	}}
	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)
	r.Header.Set("Authorization", "Bearer alice-token")

	id, err := a.Authenticate(r)
	if err != nil {
		t.Fatalf("valid alice token rejected: %v", err)
	}
	if id.Name != "alice" {
		t.Fatalf("identity.Name = %q, want alice", id.Name)
	}
	if !id.HasScope("mcp.read") || !id.HasScope("mcp.write") {
		t.Fatalf("scopes not attached: %v", id.Scopes)
	}
}

func TestBearerListAuthenticator_InvalidToken(t *testing.T) {
	a := bearerListAuthenticator{entries: []bearerListEntry{
		{name: "alice", hash: hashToken("alice-token")},
	}}
	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)
	r.Header.Set("Authorization", "Bearer unknown")

	_, err := a.Authenticate(r)
	if err == nil {
		t.Fatal("unknown token should be rejected")
	}
	if err.ErrorCode != "invalid_token" {
		t.Fatalf("error_code = %q, want invalid_token", err.ErrorCode)
	}
}

func TestBearerListAuthenticator_MissingHeader(t *testing.T) {
	a := bearerListAuthenticator{entries: []bearerListEntry{
		{name: "alice", hash: hashToken("alice-token")},
	}}
	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)

	_, err := a.Authenticate(r)
	if err == nil {
		t.Fatal("missing header should be rejected")
	}
	if err.ErrorCode != "invalid_request" {
		t.Fatalf("error_code = %q, want invalid_request", err.ErrorCode)
	}
}

func TestBearerListAuthenticator_SecondIdentityMatches(t *testing.T) {
	a := bearerListAuthenticator{entries: []bearerListEntry{
		{name: "alice", hash: hashToken("alice-token")},
		{name: "bob", hash: hashToken("bob-token")},
	}}
	r := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)
	r.Header.Set("Authorization", "Bearer bob-token")

	id, err := a.Authenticate(r)
	if err != nil {
		t.Fatalf("valid bob token rejected: %v", err)
	}
	if id.Name != "bob" {
		t.Fatalf("identity.Name = %q, want bob", id.Name)
	}
}

// -----------------------------------------------------------------------------
// buildAuthenticator dispatch
// -----------------------------------------------------------------------------

func TestBuildAuthenticator_DispatchesPerMode(t *testing.T) {
	cases := []struct {
		mode     AuthMode
		cfg      StreamableConfig
		wantType string
	}{
		{AuthNone, StreamableConfig{}, "noneAuthenticator"},
		{AuthBearer, StreamableConfig{Token: "x"}, "bearerAuthenticator"},
		{AuthBearerList, StreamableConfig{BearerList: []BearerListEntry{{Name: "a", Token: "t"}}}, "bearerListAuthenticator"},
		{AuthUnspecified, StreamableConfig{}, "noneAuthenticator"}, // zero mode falls to None
	}
	for _, tc := range cases {
		t.Run(tc.mode.String(), func(t *testing.T) {
			got := buildAuthenticator(tc.mode, tc.cfg)
			var typeName string
			switch got.(type) {
			case noneAuthenticator:
				typeName = "noneAuthenticator"
			case bearerAuthenticator:
				typeName = "bearerAuthenticator"
			case bearerListAuthenticator:
				typeName = "bearerListAuthenticator"
			default:
				typeName = "unknown"
			}
			if typeName != tc.wantType {
				t.Fatalf("buildAuthenticator(%v) returned %s, want %s", tc.mode, typeName, tc.wantType)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// End-to-end: Streamable with bearer-list honors identity on initialize
// -----------------------------------------------------------------------------

func TestStreamable_BearerListIdentityOnSession(t *testing.T) {
	cfg := StreamableConfig{
		AuthMode: AuthBearerList,
		BearerList: []BearerListEntry{
			{Name: "alice", Token: "alice-token", Scopes: []string{"mcp.read"}},
		},
	}
	s, err := NewStreamable(cfg)
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`
	req := httptest.NewRequest(http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer alice-token")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("initialize status = %d, body = %s", w.Code, w.Body.String())
	}
	sid := w.Header().Get("Mcp-Session-Id")
	if sid == "" {
		t.Fatal("no Mcp-Session-Id header on successful initialize")
	}
	sess, ok := s.registry.Get(sid)
	if !ok {
		t.Fatalf("session %q not in registry after initialize", sid)
	}
	if got := sess.Identity().Name; got != "alice" {
		t.Fatalf("session identity name = %q, want alice", got)
	}
	if !sess.Identity().HasScope("mcp.read") {
		t.Fatalf("session identity missing mcp.read scope: %v", sess.Identity().Scopes)
	}
}

func TestStreamable_BearerListRejectsInvalidToken(t *testing.T) {
	cfg := StreamableConfig{
		AuthMode: AuthBearerList,
		BearerList: []BearerListEntry{
			{Name: "alice", Token: "alice-token"},
		},
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
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401. body = %s", w.Code, w.Body.String())
	}
	wa := w.Header().Get("WWW-Authenticate")
	if wa == "" {
		t.Fatal("missing WWW-Authenticate header")
	}
	if wantPrefix := "Bearer "; wa[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("WWW-Authenticate = %q, want Bearer prefix", wa)
	}
}

func TestStreamable_BearerListNoReAuthOnSubsequentRequests(t *testing.T) {
	// AC-11a: After initialize with identity, subsequent POST with same
	// session-id is accepted; identity still present on session (no re-auth).
	cfg := StreamableConfig{
		AuthMode: AuthBearerList,
		BearerList: []BearerListEntry{
			{Name: "alice", Token: "alice-token"},
		},
	}
	s, err := NewStreamable(cfg)
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer s.Close()

	// Step 1: initialize with valid token.
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`
	req := httptest.NewRequest(http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer alice-token")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("initialize failed: %d %s", w.Code, w.Body.String())
	}
	sid := w.Header().Get("Mcp-Session-Id")

	// Step 2: subsequent tools/list carrying only Mcp-Session-Id. No Authorization.
	body2 := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	req2 := httptest.NewRequest(http.MethodPost, Endpoint, strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Mcp-Session-Id", sid)
	req2.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	w2 := httptest.NewRecorder()
	s.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("follow-up without Authorization = %d, want 200. body = %s", w2.Code, w2.Body.String())
	}
}
