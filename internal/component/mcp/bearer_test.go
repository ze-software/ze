package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// -----------------------------------------------------------------------------
// bearerAuthenticator (legacy single token)
// -----------------------------------------------------------------------------

func TestBearerAuthenticator_ValidToken(t *testing.T) {
	a := bearerAuthenticator{hash: hashToken("sekret")}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, http.NoBody)
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, http.NoBody)

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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, http.NoBody)
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, http.NoBody)
	r.Header.Set("Authorization", "bearer sekret")

	if _, err := a.Authenticate(r); err != nil {
		t.Fatalf("lowercase bearer rejected: %v", err)
	}
}

func TestBearerAuthenticator_WrongScheme(t *testing.T) {
	a := bearerAuthenticator{hash: hashToken("sekret")}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, http.NoBody)
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, http.NoBody)
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, http.NoBody)
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, http.NoBody)

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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, http.NoBody)
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
			{Name: "alice", Token: "alice-token", Scopes: []string{"mcp.read", "mcp.write"}},
			{Name: "bob", Token: "bob-token"},
		},
		// the inline `demo cmd` literal is replaced by the shared
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, http.NoBody)
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, strings.NewReader(body))
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

// TestBearerListAuthenticator_RequiredScopes verifies that when RequiredScopes
// are configured on a bearer-list authenticator, an identity missing any
// required scope is rejected with 403 insufficient_scope.
//
// VALIDATES: S4 -- bearer-list enforces configured RequiredScopes consistently
// with OAuth.
// PREVENTS: an identity lacking required scopes authenticating when
// RequiredScopes are configured.
func TestBearerListAuthenticator_RequiredScopes(t *testing.T) {
	a := bearerListAuthenticator{
		entries: []bearerListEntry{
			{name: "admin", hash: hashToken("admin-token"), scopes: []string{ScopeRead, ScopeAdmin}},
			{name: "reader", hash: hashToken("reader-token"), scopes: []string{ScopeRead}},
			{name: "empty", hash: hashToken("empty-token")},
		},
		requiredScopes: []string{ScopeRead, ScopeAdmin},
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, http.NoBody)
	req.Header.Set("Authorization", "Bearer admin-token")
	id, aerr := a.Authenticate(req)
	if aerr != nil {
		t.Fatalf("admin token rejected: %v", aerr)
	}
	if id.Name != "admin" {
		t.Fatalf("identity = %q, want admin", id.Name)
	}

	for _, tok := range []string{"reader-token", "empty-token"} {
		t.Run(tok, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, http.NoBody)
			r.Header.Set("Authorization", "Bearer "+tok)
			_, err := a.Authenticate(r)
			if err == nil {
				t.Fatalf("token %q missing required scope was accepted", tok)
			}
			if err.Status != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", err.Status)
			}
			if err.ErrorCode != "insufficient_scope" {
				t.Fatalf("error_code = %q, want insufficient_scope", err.ErrorCode)
			}
			if !strings.Contains(err.WWWAuthenticate(), `scope="mcp.read mcp.admin"`) {
				t.Fatalf("WWW-Authenticate = %q, want required scopes", err.WWWAuthenticate())
			}
		})
	}
}

// TestStreamable_BearerListScopeEnforcement verifies per-identity mcp.read,
// mcp.write, and mcp.admin scope enforcement on tools/call across generated
// tools, task-required tools, ze_reference, and ze_execute.
//
// VALIDATES: S4 -- bearer-list per-identity scopes gate read vs mutating tool
// calls (including NotReadOnly show commands and task-required mutating tools)
// and return HTTP 403 with RFC 6750 insufficient_scope on denial before any
// command is dispatched.
// PREVENTS: a read-only bearer-list identity invoking mutating commands or
// minting mutating background tasks.
func TestStreamable_BearerListScopeEnforcement(t *testing.T) {
	dispatched := 0
	commands := func() ([]CommandInfo, error) {
		return []CommandInfo{
			{Name: "show bgp rib", ShortHelp: "Show RIB", ReadOnly: true},
			{Name: "show bgp peer detail", ShortHelp: "Show peer detail", ReadOnly: true, TakesSelector: true},
			{Name: "show config artifact source", ShortHelp: "Show raw source", ReadOnly: false},
			{Name: "send bgp update", ShortHelp: "Send update", ReadOnly: false, TaskSupport: TaskSupportRequired},
		}, nil
	}
	srv, err := NewStreamable(StreamableConfig{
		AuthMode: AuthBearerList,
		BearerList: []BearerListEntry{
			{Name: "reader", Token: "reader-token", Scopes: []string{ScopeRead}},
			{Name: "writer", Token: "writer-token", Scopes: []string{ScopeWrite}},
			{Name: "both", Token: "both-token", Scopes: []string{ScopeRead, ScopeWrite}},
			{Name: "admin", Token: "admin-token", Scopes: []string{ScopeAdmin}},
			{Name: "other", Token: "other-token", Scopes: []string{"custom.other"}},
			{Name: "unscoped", Token: "unscoped-token"},
		},
		Commands: commands,
		Dispatch: func(_ context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
			dispatched++
			return plugin.NewResponse(plugin.StatusDone, plugin.Map{"cmd": cmd}), nil
		},
	})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer srv.Close()

	call := func(t *testing.T, token, caps, toolName, argsJSON string) *httptest.ResponseRecorder {
		t.Helper()
		paramsJSON := `{"name":"` + toolName + `","arguments":` + argsJSON + `}`
		body, params := buildMCPBody(t, 1, methodToolsCall, caps, paramsJSON)
		headers := mcpHeaders(methodToolsCall, params)
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, strings.NewReader(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		return w
	}

	cases := []struct {
		name       string
		token      string
		caps       string
		tool       string
		args       string
		wantStatus int
		wantScope  string
	}{
		// reader (mcp.read only)
		{"reader allowed read-only generated tool", "reader-token", capsNone, "ze_show_bgp", `{"action":"rib"}`, http.StatusOK, ""},
		{"reader allowed ze_reference", "reader-token", capsNone, "ze_reference", `{}`, http.StatusOK, ""},
		{"reader allowed ze_execute read-only", "reader-token", capsNone, "ze_execute", `{"command":"show bgp rib"}`, http.StatusOK, ""},
		{"reader allowed ze_execute read-only with selector", "reader-token", capsNone, "ze_execute", `{"command":"show bgp peer 192.0.2.1 detail"}`, http.StatusOK, ""},
		{"reader allowed ze_execute read-only with pipe", "reader-token", capsNone, "ze_execute", `{"command":"show bgp rib | count"}`, http.StatusOK, ""},
		{"reader denied mutating generated tool", "reader-token", capsNone, "ze_send", `{"action":"bgp update"}`, http.StatusForbidden, ScopeWrite},
		{"reader denied mutating task-required tool", "reader-token", capsTasks, "ze_send", `{"action":"bgp update"}`, http.StatusForbidden, ScopeWrite},
		{"reader denied NotReadOnly show generated tool", "reader-token", capsNone, "ze_show_config", `{"action":"artifact source"}`, http.StatusForbidden, ScopeWrite},
		{"reader denied ze_execute mutating", "reader-token", capsNone, "ze_execute", `{"command":"send bgp update"}`, http.StatusForbidden, ScopeWrite},
		{"reader denied ze_execute NotReadOnly show", "reader-token", capsNone, "ze_execute", `{"command":"show config artifact source"}`, http.StatusForbidden, ScopeWrite},
		{"reader denied ze_execute unrecognized command", "reader-token", capsNone, "ze_execute", `{"command":"show mystery"}`, http.StatusForbidden, ScopeWrite},
		{"reader denied ze_execute pipe save", "reader-token", capsNone, "ze_execute", `{"command":"show bgp rib | save /tmp/x"}`, http.StatusForbidden, ScopeWrite},

		// writer (mcp.write only)
		{"writer allowed mutating generated tool", "writer-token", capsNone, "ze_send", `{"action":"bgp update"}`, http.StatusOK, ""},
		{"writer allowed ze_execute mutating", "writer-token", capsNone, "ze_execute", `{"command":"send bgp update"}`, http.StatusOK, ""},
		{"writer denied read-only generated tool", "writer-token", capsNone, "ze_show_bgp", `{"action":"rib"}`, http.StatusForbidden, ScopeRead},
		{"writer denied ze_reference", "writer-token", capsNone, "ze_reference", `{}`, http.StatusForbidden, ScopeRead},

		// both (mcp.read + mcp.write)
		{"both allowed read-only", "both-token", capsNone, "ze_show_bgp", `{"action":"rib"}`, http.StatusOK, ""},
		{"both allowed mutating", "both-token", capsNone, "ze_send", `{"action":"bgp update"}`, http.StatusOK, ""},

		// admin (mcp.admin satisfies both read and write)
		{"admin allowed read-only", "admin-token", capsNone, "ze_show_bgp", `{"action":"rib"}`, http.StatusOK, ""},
		{"admin allowed mutating", "admin-token", capsNone, "ze_send", `{"action":"bgp update"}`, http.StatusOK, ""},
		{"admin allowed ze_reference", "admin-token", capsNone, "ze_reference", `{}`, http.StatusOK, ""},

		// unrelated custom scope denies both read and write
		{"other denied read-only", "other-token", capsNone, "ze_show_bgp", `{"action":"rib"}`, http.StatusForbidden, ScopeRead},
		{"other denied mutating", "other-token", capsNone, "ze_send", `{"action":"bgp update"}`, http.StatusForbidden, ScopeWrite},

		// empty scope list imposes no scope restriction
		{"unscoped allowed read-only", "unscoped-token", capsNone, "ze_show_bgp", `{"action":"rib"}`, http.StatusOK, ""},
		{"unscoped allowed mutating", "unscoped-token", capsNone, "ze_send", `{"action":"bgp update"}`, http.StatusOK, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := dispatched
			w := call(t, tc.token, tc.caps, tc.tool, tc.args)
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body=%s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if tc.wantStatus == http.StatusForbidden {
				if dispatched != before {
					t.Fatalf("command was dispatched despite 403 scope denial")
				}
				wa := w.Header().Get("WWW-Authenticate")
				if !strings.Contains(wa, `error="insufficient_scope"`) {
					t.Fatalf("WWW-Authenticate = %q, want error=\"insufficient_scope\"", wa)
				}
				if !strings.Contains(wa, `scope="`+tc.wantScope+`"`) {
					t.Fatalf("WWW-Authenticate = %q, want scope=%q", wa, tc.wantScope)
				}
				if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
					t.Fatalf("Cache-Control = %q, want no-store", cc)
				}
			}
		})
	}
}
