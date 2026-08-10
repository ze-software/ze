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
	if !id.isAnonymous() {
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
// End-to-end: Streamable with bearer-list honors identity on EVERY request
// -----------------------------------------------------------------------------

// VALIDATES: the per-request authenticator's Identity -- name and scopes --
// is what reaches the handlers, proven through the task registry, which is
// keyed by principal: alice's task is invisible to bob.
// PREVENTS: the identity coming from anywhere but the authenticator (notably a
// client-supplied _meta.clientInfo field), and cross-principal task visibility.
func TestStreamable_BearerListIdentityOnEveryRequest(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{
		AuthMode: AuthBearerList,
		BearerList: []BearerListEntry{
			{Name: "alice", Token: "alice-token", Scopes: []string{"mcp.read"}},
			{Name: "bob", Token: "bob-token"},
		},
		// test-relax: the inline `demo cmd` literal is replaced by the shared
		// taskCapableCommands fixture, not deleted. Its TaskSupport was
		// optional, and under the server-directed model (D-1) no request shape
		// can turn an optional command into a task. `task:{}` is gone. The
		// fixture supplies a `required` command so this test can still mint the
		// task its ownership assertions are about.
		Commands: taskCapableCommands,
	})
	defer cleanup()

	// alice creates a task. Its ownership comes from the credential on THIS
	// request, not from any prior handshake. Note the params carry NO task
	// member: the server decides from the annotation.
	status, parsed := postMCPAuth(t, hs, "alice-token", methodToolsCall, capsTasks,
		`{"name":"ze_slow","arguments":{"action":"cmd"}}`)
	if status != http.StatusOK {
		t.Fatalf("alice tools/call: status = %d (body %v)", status, parsed)
	}
	aliceTask, _ := resultOf(t, parsed)["taskId"].(string)
	if aliceTask == "" {
		t.Fatalf("no taskId in %v", parsed)
	}

	// alice sees it.
	status, parsed = postMCPAuth(t, hs, "alice-token", methodTasksGet, capsTasks,
		`{"taskId":"`+aliceTask+`"}`)
	if status != http.StatusOK {
		t.Fatalf("alice tasks/get: status = %d (body %v)", status, parsed)
	}
	if got, _ := resultOf(t, parsed)["taskId"].(string); got != aliceTask {
		t.Fatalf("alice tasks/get returned %q, want %q", got, aliceTask)
	}

	// bob does not, even naming the id directly.
	_, parsed = postMCPAuth(t, hs, "bob-token", methodTasksGet, capsTasks,
		`{"taskId":"`+aliceTask+`"}`)
	rpcErr := rpcErrorOf(t, parsed)
	if msg, _ := rpcErr["message"].(string); !strings.Contains(msg, "not found") {
		t.Fatalf("bob tasks/get on alice's task: message = %q, want a not-found denial", msg)
	}

	// And there is no enumeration surface to leak through at all. MCP 2026-07-28
	// removed tasks/list, so bob cannot ask "what tasks exist" in the first
	// place. That is a stronger guarantee than the empty list this assertion
	// used to check. An empty list proves the filter worked, and an absent
	// method proves there is no filter to get wrong.
	status, parsed = postMCPAuth(t, hs, "bob-token", "tasks/list", capsTasks, "")
	if status != http.StatusNotFound {
		t.Fatalf("bob tasks/list: status = %d, want 404 (method removed) (body %v)", status, parsed)
	}
	if code, _ := rpcErrorOf(t, parsed)["code"].(float64); int(code) != rpcMethodNotFound {
		t.Fatalf("bob tasks/list: code = %v, want %d (method not found)", code, rpcMethodNotFound)
	}

	// The scope rides the same Identity value the name does.
	req := httptest.NewRequest(http.MethodPost, Endpoint, http.NoBody)
	req.Header.Set("Authorization", "Bearer alice-token")
	srv, err := NewStreamable(StreamableConfig{
		AuthMode:   AuthBearerList,
		BearerList: []BearerListEntry{{Name: "alice", Token: "alice-token", Scopes: []string{"mcp.read"}}},
	})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer srv.Close()
	identity, aerr := srv.authenticate(req)
	if aerr != nil {
		t.Fatalf("authenticate: %v", aerr)
	}
	if identity.Name != "alice" {
		t.Fatalf("identity name = %q, want alice", identity.Name)
	}
	if !identity.HasScope("mcp.read") {
		t.Fatalf("identity missing mcp.read scope: %v", identity.Scopes)
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

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":` +
		metaBlock(ProtocolVersion, capsNone) + `}}`
	req := httptest.NewRequest(http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	req.Header.Set("Mcp-Method", "tools/list")
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

// TestStreamable_BearerListReAuthsOnEverySubsequentRequest is the INVERSION of
// the pre-cutover TestStreamable_BearerListNoReAuthOnSubsequentRequests, which
// asserted that a follow-up carrying only a session id was served without a
// credential.
//
// VALIDATES: there is no identifier a client can present in place of its
// credential -- a follow-up without Authorization is 401 even when it threads
// the header the old shape would have accepted.
// PREVENTS: the session id remaining a bearer credential in its own right, and
// a revoked token staying usable until a session expires.
func TestStreamable_BearerListReAuthsOnEverySubsequentRequest(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{
		AuthMode: AuthBearerList,
		BearerList: []BearerListEntry{
			{Name: "alice", Token: "alice-token"},
		},
	})
	defer cleanup()

	// A credentialled request first.
	if status, parsed := postMCPAuth(t, hs, "alice-token", methodToolsList, capsNone, ""); status != http.StatusOK {
		t.Fatalf("authenticated tools/list: status = %d (body %v)", status, parsed)
	}

	// The follow-up carries no Authorization, and threads the removed session
	// header in case anything still honors it. It must be refused.
	body, params := buildMCPBody(t, 2, methodToolsList, capsNone, "")
	headers := mcpHeaders(methodToolsList, params)
	headers["Mcp-Session-Id"] = "any-value-a-stale-client-might-send"
	status, _ := postRaw(t, hs, body, headers)
	if status != http.StatusUnauthorized {
		t.Fatalf("follow-up without Authorization = %d, want 401", status)
	}
}
