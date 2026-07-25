package rest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/audit"
)

// testEngine creates an APIEngine with fake implementations for testing.
func testEngine() *api.APIEngine {
	exec := func(_ context.Context, _ api.CallerIdentity, command string) (*plugin.Response, error) {
		switch command {
		case "show bgp summary":
			return plugin.NewResponse(api.StatusDone, plugin.RawJSON(`{"peer-count":3}`)), nil
		case "show version":
			return plugin.NewResponse(api.StatusDone, plugin.RawJSON(`{"version":"1.0"}`)), nil
		default:
			return plugin.NewResponse(api.StatusDone, plugin.RawJSON("ok: "+command)), nil
		}
	}
	cmds := func() []api.CommandMeta {
		return []api.CommandMeta{
			{Name: "show bgp summary", Description: "Show BGP summary", ReadOnly: true},
			{Name: "show status", Description: "Show process status", ReadOnly: true},
			{Name: "bgp monitor", Description: "Monitor BGP events", ReadOnly: true},
			{Name: "show bgp rib", Description: "Show routes", ReadOnly: true, Params: []api.ParamMeta{
				{Name: "family", Type: "string", Description: "Address family"},
			}},
			{Name: "request reload", Description: "Reload config", ReadOnly: false},
		}
	}
	auth := func(_, _ string) bool { return true }
	stream := func(_ context.Context, _ api.CallerIdentity, _ string) (<-chan string, func(), error) {
		ch := make(chan string, 2) //nolint:mnd // test events
		ch <- `{"event":"update"}`
		ch <- `{"event":"withdraw"}`
		close(ch)
		return ch, func() {}, nil
	}
	return api.NewAPIEngine(exec, cmds, auth, stream)
}

// testServer creates a RESTServer backed by httptest for testing.
func testServer(t *testing.T) *RESTServer {
	t.Helper()
	engine := testEngine()
	openAPI, err := api.OpenAPISchema(engine.ListCommands(&api.ListCommandsRequest{}))
	require.NoError(t, err)

	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		return &fakeEditor{values: make(map[string]string)}, nil
	})

	srv, err := NewRESTServer(RESTConfig{ListenAddrs: []string{"127.0.0.1:0"}}, engine, sessions, func() []byte { return openAPI })
	require.NoError(t, err)
	return srv
}

// doResult holds the result of an HTTP request.
type doResult struct {
	Status int
	Header http.Header
	Body   string
}

// do sends an HTTP request to the test server and returns status, headers, and body.
func do(t *testing.T, srv *RESTServer, method, path, body string) doResult {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	srv.srv.Handler.ServeHTTP(w, req)
	resp := w.Result()
	data, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.NoError(t, err)
	return doResult{Status: resp.StatusCode, Header: resp.Header, Body: string(data)}
}

// doWithHeader sends an HTTP request with custom headers.
func doWithHeader(t *testing.T, srv *RESTServer, method, path, body string, headers map[string]string) doResult { //nolint:unparam // method is parameterized even if tests use POST today
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	srv.srv.Handler.ServeHTTP(w, req)
	resp := w.Result()
	data, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.NoError(t, err)
	return doResult{Status: resp.StatusCode, Header: resp.Header, Body: string(data)}
}

// fakeEditor implements api.ConfigEditor for testing.
type fakeEditor struct {
	values           map[string]string
	committedContent string
}

func (e *fakeEditor) SetValue(path []string, key, value string) error {
	e.values[strings.Join(path, ".")+"."+key] = value
	return nil
}

func (e *fakeEditor) DeleteByPath(fullPath []string) error {
	delete(e.values, strings.Join(fullPath, "."))
	return nil
}

func (e *fakeEditor) Diff() string {
	if len(e.values) == 0 {
		return ""
	}
	var b strings.Builder
	for k, v := range e.values {
		b.WriteString("+" + k + " = " + v + "\n")
	}
	return b.String()
}
func (e *fakeEditor) Save() error {
	e.committedContent = e.WorkingContent()
	return nil
}
func (e *fakeEditor) StageCandidate(time.Time) (string, string, error) {
	return e.WorkingContent(), "test-version", nil
}
func (e *fakeEditor) MarkCommittedContent(content string) { e.committedContent = content }
func (e *fakeEditor) RestoreOriginalContent(string) error { return nil }
func (e *fakeEditor) Discard() error                      { e.values = make(map[string]string); return nil }
func (e *fakeEditor) OriginalContent() string             { return "# original\n" }
func (e *fakeEditor) WorkingContent() string {
	if len(e.values) == 0 {
		return "# config\n"
	}
	keys := make([]string, 0, len(e.values))
	for key := range e.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("# config\n")
	for _, key := range keys {
		b.WriteString(key)
		b.WriteString(" = ")
		b.WriteString(e.values[key])
		b.WriteByte('\n')
	}
	return b.String()
}

type denyAllAuthorizer struct {
	username string
	command  string
	readOnly bool
}

func (a *denyAllAuthorizer) Authorize(username, _, command string, isReadOnly bool) bool {
	a.username = username
	a.command = command
	a.readOnly = isReadOnly
	return false
}

// VALIDATES: AC-1 -- GET /api/v1/commands returns command list.
// PREVENTS: missing commands in REST response.
func TestRESTListCommands(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "GET", "/api/v1/commands", "")
	assert.Equal(t, http.StatusOK, r.Status)

	var cmds []api.CommandMeta
	require.NoError(t, json.Unmarshal([]byte(r.Body), &cmds))
	assert.Len(t, cmds, 5)
}

// VALIDATES: AC-2 -- POST /api/v1/execute returns command output.
// PREVENTS: execute endpoint broken.
func TestRESTExecute(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "POST", "/api/v1/execute", `{"command":"show bgp summary"}`)
	assert.Equal(t, http.StatusOK, r.Status)

	// test-relax: the envelope's Data is now the marker interface ResponseData
	// (marshal-only); json.Unmarshal cannot target it, so decode only the
	// scalar status field the assertion checks.
	var result struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(r.Body), &result))
	assert.Equal(t, "done", result.Status)
}

// VALIDATES: request context and HTTP remote address reach the API executor.
// PREVENTS: execute requests losing cancellation or accounting metadata at the REST boundary.
func TestExecutePropagatesRequestContextAndRemoteAddr(t *testing.T) {
	type ctxKey struct{}

	var (
		gotCtx  context.Context
		gotAuth api.CallerIdentity
	)

	engine := api.NewAPIEngine(
		func(ctx context.Context, auth api.CallerIdentity, command string) (*plugin.Response, error) {
			gotCtx = ctx
			gotAuth = auth
			return plugin.NewResponse(api.StatusDone, plugin.RawJSON("ok: "+command)), nil
		},
		func() []api.CommandMeta {
			return []api.CommandMeta{{Name: "show bgp summary", ReadOnly: true}}
		},
		func(_, _ string) bool { return true },
		nil,
	)

	openAPI, err := api.OpenAPISchema(engine.ListCommands(&api.ListCommandsRequest{}))
	require.NoError(t, err)

	srv, err := NewRESTServer(RESTConfig{ListenAddrs: []string{"127.0.0.1:0"}}, engine, nil, func() []byte { return openAPI })
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/execute", strings.NewReader(`{"command":"show bgp summary"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.10:4444"
	req = req.WithContext(context.WithValue(req.Context(), ctxKey{}, "trace-id"))

	rec := httptest.NewRecorder()
	srv.srv.Handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, gotCtx)
	assert.Equal(t, "trace-id", gotCtx.Value(ctxKey{}))
	assert.Equal(t, "api", gotAuth.Username)
	assert.Equal(t, "198.51.100.10:4444", gotAuth.RemoteAddr)
}

// VALIDATES: AC-3 -- POST /api/v1/execute without auth returns 401.
// PREVENTS: unauthenticated access.
func TestRESTExecuteUnauthorized(t *testing.T) {
	engine := testEngine()
	openAPI, _ := api.OpenAPISchema(nil)
	srv, err := NewRESTServer(RESTConfig{ListenAddrs: []string{"127.0.0.1:0"}, Token: "secret"}, engine, nil, func() []byte { return openAPI })
	require.NoError(t, err)

	// No Authorization header.
	r := do(t, srv, "POST", "/api/v1/execute", `{"command":"show bgp summary"}`)
	assert.Equal(t, http.StatusUnauthorized, r.Status)

	// Wrong token.
	r = doWithHeader(t, srv, "POST", "/api/v1/execute", `{"command":"show bgp summary"}`, map[string]string{
		"Authorization": "Bearer wrong",
		"Content-Type":  "application/json",
	})
	assert.Equal(t, http.StatusUnauthorized, r.Status)

	// Correct token.
	r = doWithHeader(t, srv, "POST", "/api/v1/execute", `{"command":"show bgp summary"}`, map[string]string{
		"Authorization": "Bearer secret",
		"Content-Type":  "application/json",
	})
	assert.Equal(t, http.StatusOK, r.Status)
}

// VALIDATES: AC-16 -- REST failed authentication emits an audit record with source IP and attempted user.
// PREVENTS: REST auth failures being visible only as HTTP 401 responses.
func TestRESTAuthFailureAuditRecord(t *testing.T) {
	engine := testEngine()
	openAPI, _ := api.OpenAPISchema(nil)
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	srv, err := NewRESTServer(RESTConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		Authenticator: func(header string) (string, bool) {
			return "", false
		},
		AuditRecorder: recorder,
	}, engine, nil, func() []byte { return openAPI })
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/execute", strings.NewReader(`{"command":"show bgp summary"}`))
	req.Header.Set("Authorization", "Bearer alice:wrong")
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.10:4444"
	rec := httptest.NewRecorder()

	srv.srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	entries := recorder.Query(audit.Filter{Action: audit.ActionAuthFail})
	require.Len(t, entries, 1)
	assert.Equal(t, "alice", entries[0].Actor)
	assert.Equal(t, "192.0.2.10:4444", entries[0].RemoteAddr)
	assert.Equal(t, audit.REST, entries[0].Surface)
	assert.Equal(t, audit.OutcomeDenied, entries[0].Outcome)
}

// VALIDATES: AC-8 -- REST no-auth mode gives the default api identity read-only access, not admin access.
// PREVENTS: REST without authenticator or token accepting write commands and config sessions.
func TestRESTNoAuthReadOnly(t *testing.T) {
	srv := testServer(t)

	read := do(t, srv, "POST", "/api/v1/execute", `{"command":"show bgp summary"}`)
	assert.Equal(t, http.StatusOK, read.Status)

	write := do(t, srv, "POST", "/api/v1/execute", `{"command":"request reload"}`)
	assert.Equal(t, http.StatusForbidden, write.Status)

	session := do(t, srv, "POST", "/api/v1/config/sessions", "")
	assert.Equal(t, http.StatusForbidden, session.Status)
}

// VALIDATES: AC-4 -- GET /api/v1/peers returns peer summary.
// PREVENTS: convenience route broken.
func TestRESTPeersConvenience(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "GET", "/api/v1/peers", "")
	assert.Equal(t, http.StatusOK, r.Status)

	// test-relax: the envelope's Data is now the marker interface ResponseData
	// (marshal-only); json.Unmarshal cannot target it, so decode only the
	// scalar status field the assertion checks.
	var result struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(r.Body), &result))
	assert.Equal(t, "done", result.Status)
}

// VALIDATES: AC-5 -- Config session create + set + commit.
// PREVENTS: config lifecycle broken over REST.
func TestRESTConfigSession(t *testing.T) {
	engine := testEngine()
	openAPI, err := api.OpenAPISchema(engine.ListCommands(&api.ListCommandsRequest{}))
	require.NoError(t, err)
	var editor *fakeEditor
	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		editor = &fakeEditor{values: make(map[string]string)}
		return editor, nil
	})
	srv, err := NewRESTServer(RESTConfig{ListenAddrs: []string{"127.0.0.1:0"}, Token: "secret"}, engine, sessions, func() []byte { return openAPI })
	require.NoError(t, err)
	headers := map[string]string{"Authorization": "Bearer secret", "Content-Type": "application/json"}

	// Create session.
	r := doWithHeader(t, srv, "POST", "/api/v1/config/sessions", "", headers)
	assert.Equal(t, http.StatusCreated, r.Status)
	var created map[string]string
	require.NoError(t, json.Unmarshal([]byte(r.Body), &created))
	id := created["session-id"]
	assert.NotEmpty(t, id)

	// Set value.
	r = doWithHeader(t, srv, "PUT", "/api/v1/config/sessions/"+id,
		`{"path":"bgp.router-id","value":"10.0.0.1"}`, headers)
	assert.Equal(t, http.StatusOK, r.Status)

	// Diff.
	r = doWithHeader(t, srv, "GET", "/api/v1/config/sessions/"+id+"/diff", "", headers)
	assert.Equal(t, http.StatusOK, r.Status)
	assert.Contains(t, r.Body, "diff")

	// Commit.
	r = doWithHeader(t, srv, "POST", "/api/v1/config/sessions/"+id+"/commit", "", headers)
	assert.Equal(t, http.StatusOK, r.Status)
	assert.Contains(t, r.Body, "committed")
	require.NotNil(t, editor)
	assert.Contains(t, editor.committedContent, "bgp.router-id = 10.0.0.1")
}

// VALIDATES: config session writes consult profile RBAC for authenticated users.
// PREVENTS: read-only config users mutating config through REST sessions.
func TestRESTConfigSessionAuthorizerDeny(t *testing.T) {
	engine := testEngine()
	openAPI, err := api.OpenAPISchema(engine.ListCommands(&api.ListCommandsRequest{}))
	require.NoError(t, err)
	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		return &fakeEditor{values: make(map[string]string)}, nil
	})
	authorizer := &denyAllAuthorizer{}
	srv, err := NewRESTServer(RESTConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		Authenticator: func(header string) (string, bool) {
			return "alice", header == "Bearer alice-token"
		},
		Authorizer: authorizer,
	}, engine, sessions, func() []byte { return openAPI })
	require.NoError(t, err)

	r := doWithHeader(t, srv, "POST", "/api/v1/config/sessions", "", map[string]string{
		"Authorization": "Bearer alice-token",
	})
	assert.Equal(t, http.StatusForbidden, r.Status)
	assert.Equal(t, "alice", authorizer.username)
	assert.Equal(t, "config edit", authorizer.command)
	assert.False(t, authorizer.readOnly)
}

// VALIDATES: AC-9 -- Config commit via REST emits an audit record with actor, surface, action, and summary.
// PREVENTS: REST config session commits bypassing the unified audit trail.
func TestRESTConfigCommitAuditRecord(t *testing.T) {
	engine := testEngine()
	openAPI, err := api.OpenAPISchema(engine.ListCommands(&api.ListCommandsRequest{}))
	require.NoError(t, err)
	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		return &fakeEditor{values: make(map[string]string)}, nil
	})
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	srv, err := NewRESTServer(RESTConfig{ListenAddrs: []string{"127.0.0.1:0"}, Token: "secret", AuditRecorder: recorder}, engine, sessions, func() []byte { return openAPI })
	require.NoError(t, err)
	headers := map[string]string{"Authorization": "Bearer secret", "Content-Type": "application/json"}

	created := doWithHeader(t, srv, "POST", "/api/v1/config/sessions", "", headers)
	require.Equal(t, http.StatusCreated, created.Status)
	var createdBody map[string]string
	require.NoError(t, json.Unmarshal([]byte(created.Body), &createdBody))
	id := createdBody["session-id"]

	set := doWithHeader(t, srv, "PUT", "/api/v1/config/sessions/"+id, `{"path":"bgp.router-id","value":"10.0.0.1"}`, headers)
	require.Equal(t, http.StatusOK, set.Status)
	commit := doWithHeader(t, srv, "POST", "/api/v1/config/sessions/"+id+"/commit", "", headers)
	require.Equal(t, http.StatusOK, commit.Status)

	entries := recorder.Query(audit.Filter{Action: audit.ActionConfigCommit})
	require.Len(t, entries, 1)
	assert.Equal(t, "api", entries[0].Actor)
	assert.Equal(t, audit.REST, entries[0].Surface)
	assert.Equal(t, audit.OutcomeSuccess, entries[0].Outcome)
	assert.Contains(t, entries[0].Detail, "bgp.router-id")
}

// VALIDATES: AC-10 -- Config discard via REST emits an audit record with actor, surface, action, and summary.
// PREVENTS: REST config discards losing audit attribution.
func TestRESTConfigDiscardAuditRecord(t *testing.T) {
	engine := testEngine()
	openAPI, err := api.OpenAPISchema(engine.ListCommands(&api.ListCommandsRequest{}))
	require.NoError(t, err)
	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		return &fakeEditor{values: make(map[string]string)}, nil
	})
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	srv, err := NewRESTServer(RESTConfig{ListenAddrs: []string{"127.0.0.1:0"}, Token: "secret", AuditRecorder: recorder}, engine, sessions, func() []byte { return openAPI })
	require.NoError(t, err)
	headers := map[string]string{"Authorization": "Bearer secret", "Content-Type": "application/json"}

	created := doWithHeader(t, srv, "POST", "/api/v1/config/sessions", "", headers)
	require.Equal(t, http.StatusCreated, created.Status)
	var createdBody map[string]string
	require.NoError(t, json.Unmarshal([]byte(created.Body), &createdBody))
	id := createdBody["session-id"]

	set := doWithHeader(t, srv, "PUT", "/api/v1/config/sessions/"+id, `{"path":"bgp.router-id","value":"10.0.0.1"}`, headers)
	require.Equal(t, http.StatusOK, set.Status)
	discard := doWithHeader(t, srv, "DELETE", "/api/v1/config/sessions/"+id, "", headers)
	require.Equal(t, http.StatusOK, discard.Status)

	entries := recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard})
	require.Len(t, entries, 1)
	assert.Equal(t, "api", entries[0].Actor)
	assert.Equal(t, audit.REST, entries[0].Surface)
	assert.Equal(t, audit.OutcomeSuccess, entries[0].Outcome)
	assert.Contains(t, entries[0].Detail, "bgp.router-id")
}

// VALIDATES: AC-6 -- GET /api/v1/openapi.json returns valid spec.
// PREVENTS: OpenAPI endpoint broken.
func TestRESTOpenAPISchema(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "GET", "/api/v1/openapi.json", "")
	assert.Equal(t, http.StatusOK, r.Status)
	assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

	var spec map[string]any
	require.NoError(t, json.Unmarshal([]byte(r.Body), &spec))
	assert.Equal(t, "3.1.0", spec["openapi"])
}

// VALIDATES: AC-7 -- GET /api/v1/docs returns HTML page referencing vendored assets.
// PREVENTS: docs endpoint broken or still using external CDN.
func TestRESTDocs(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "GET", "/api/v1/docs", "")
	assert.Equal(t, http.StatusOK, r.Status)
	assert.Contains(t, r.Header.Get("Content-Type"), "text/html")
	assert.Contains(t, r.Body, "swagger-ui")
	// Verify no CDN references remain.
	assert.NotContains(t, r.Body, "unpkg.com")
	assert.Contains(t, r.Body, "/api/v1/docs/swagger-ui.css")
	assert.Contains(t, r.Body, "/api/v1/docs/swagger-ui-bundle.js")
}

// VALIDATES: vendored Swagger CSS served locally.
// PREVENTS: docs page broken when CDN unreachable.
func TestRESTSwaggerCSS(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "GET", "/api/v1/docs/swagger-ui.css", "")
	assert.Equal(t, http.StatusOK, r.Status)
	assert.Contains(t, r.Header.Get("Content-Type"), "text/css")
	assert.NotEmpty(t, r.Body)
}

// VALIDATES: vendored Swagger JS served locally.
// PREVENTS: docs page broken when CDN unreachable.
func TestRESTSwaggerJS(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "GET", "/api/v1/docs/swagger-ui-bundle.js", "")
	assert.Equal(t, http.StatusOK, r.Status)
	assert.Contains(t, r.Header.Get("Content-Type"), "javascript")
	assert.NotEmpty(t, r.Body)
}

// TestRESTDocsRequireAuthWhenConfigured verifies documentation endpoints follow
// the same auth policy as command and config APIs.
//
// VALIDATES: OpenAPI and Swagger UI are not public when REST auth is configured.
// PREVENTS: Authenticated API surface being enumerated without credentials.
func TestRESTDocsRequireAuthWhenConfigured(t *testing.T) {
	engine := testEngine()
	openAPI, _ := api.OpenAPISchema(nil)
	srv, err := NewRESTServer(RESTConfig{ListenAddrs: []string{"127.0.0.1:0"}, Token: "secret"}, engine, nil, func() []byte { return openAPI })
	require.NoError(t, err)

	paths := []string{
		"/api/v1/openapi.json",
		"/api/v1/docs",
		"/api/v1/docs/swagger-ui.css",
		"/api/v1/docs/swagger-ui-bundle.js",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			unauthorized := do(t, srv, "GET", path, "")
			assert.Equal(t, http.StatusUnauthorized, unauthorized.Status)

			authorized := doWithHeader(t, srv, "GET", path, "", map[string]string{"Authorization": "Bearer secret"})
			assert.Equal(t, http.StatusOK, authorized.Status)
		})
	}
}

// VALIDATES: AC-8 -- SSE stream delivers events.
// PREVENTS: streaming broken.
func TestRESTStreamSSE(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "GET", "/api/v1/execute/stream?command=bgp+monitor", "")
	assert.Equal(t, http.StatusOK, r.Status)
	assert.Equal(t, "text/event-stream", r.Header.Get("Content-Type"))
	assert.Contains(t, r.Body, "data: {\"event\":\"update\"}")
	assert.Contains(t, r.Body, "data: {\"event\":\"withdraw\"}")
}

// VALIDATES: AC-9 -- CORS preflight returns headers.
// PREVENTS: CORS broken for browser clients.
func TestRESTCORS(t *testing.T) {
	engine := testEngine()
	openAPI, _ := api.OpenAPISchema(nil)
	srv, err := NewRESTServer(RESTConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		CORSOrigin:  "https://dashboard.example.com",
	}, engine, nil, func() []byte { return openAPI })
	require.NoError(t, err)

	r := do(t, srv, "OPTIONS", "/api/v1/execute", "")
	assert.Equal(t, http.StatusNoContent, r.Status)
	assert.Equal(t, "https://dashboard.example.com", r.Header.Get("Access-Control-Allow-Origin"))
	assert.Contains(t, r.Header.Get("Access-Control-Allow-Methods"), "POST")
}

// VALIDATES: AC-10 -- POST /api/v1/execute with missing command returns 400.
// PREVENTS: empty command accepted.
func TestRESTExecuteMissingCommand(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "POST", "/api/v1/execute", `{"command":""}`)
	assert.Equal(t, http.StatusBadRequest, r.Status)
}

// VALIDATES: POST /api/v1/execute with invalid JSON returns 400.
// PREVENTS: malformed request accepted.
func TestRESTExecuteInvalidJSON(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "POST", "/api/v1/execute", `not json`)
	assert.Equal(t, http.StatusBadRequest, r.Status)
}

// VALIDATES: GET /api/v1/commands/{path} returns command metadata.
// PREVENTS: describe endpoint broken.
func TestRESTDescribeCommand(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "GET", "/api/v1/commands/show/bgp/rib", "")
	assert.Equal(t, http.StatusOK, r.Status)

	var cmd api.CommandMeta
	require.NoError(t, json.Unmarshal([]byte(r.Body), &cmd))
	assert.Equal(t, "show bgp rib", cmd.Name)
	assert.Len(t, cmd.Params, 1)
}

// VALIDATES: GET /api/v1/commands/{unknown} returns 404.
// PREVENTS: unknown command returns 200.
func TestRESTDescribeCommandNotFound(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "GET", "/api/v1/commands/nonexistent/cmd", "")
	assert.Equal(t, http.StatusNotFound, r.Status)
}

// VALIDATES: Execute with params appends key-value pairs to command.
// PREVENTS: params silently ignored.
func TestRESTExecuteWithParams(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "POST", "/api/v1/execute", `{"command":"show bgp rib","params":{"family":"ipv4/unicast"}}`)
	assert.Equal(t, http.StatusOK, r.Status)
	// The fake executor returns "ok: <command>" for unknown commands.
	assert.Contains(t, r.Body, "show bgp rib family ipv4/unicast")
}

// VALIDATES: Execute rejects param keys with whitespace.
// PREVENTS: command injection via param keys.
func TestRESTExecuteParamKeyInjection(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "POST", "/api/v1/execute", `{"command":"show bgp summary","params":{"bad key":"value"}}`)
	assert.Equal(t, http.StatusBadRequest, r.Status)
	assert.Contains(t, r.Body, "whitespace")
}

// VALIDATES: Execute rejects param values with whitespace.
// PREVENTS: command injection via param values.
func TestRESTExecuteParamValueInjection(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "POST", "/api/v1/execute", `{"command":"show bgp summary","params":{"family":"ipv4 unicast"}}`)
	assert.Equal(t, http.StatusBadRequest, r.Status)
	assert.Contains(t, r.Body, "whitespace")
}

// VALIDATES: Peer name with whitespace in URL returns 400.
// PREVENTS: command injection via URL path.
func TestRESTPeerNameWhitespace(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "GET", "/api/v1/peers/10.0.0.1%20teardown", "")
	assert.Equal(t, http.StatusBadRequest, r.Status)
	assert.Contains(t, r.Body, "whitespace")
}

// VALIDATES: RIB family with whitespace in URL returns 400.
// PREVENTS: command injection via family path.
func TestRESTRIBFamilyWhitespace(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "GET", "/api/v1/rib/ipv4%20unicast", "")
	assert.Equal(t, http.StatusBadRequest, r.Status)
	assert.Contains(t, r.Body, "whitespace")
}

// VALIDATES: per-user authenticator passes username to engine.
// PREVENTS: all requests authenticated as "api" default.
func TestRESTAuthenticator(t *testing.T) {
	var seenUser string
	exec := func(_ context.Context, auth api.CallerIdentity, _ string) (*plugin.Response, error) {
		seenUser = auth.Username
		return plugin.NewResponse(api.StatusDone, plugin.RawJSON(`"ok"`)), nil
	}
	cmds := func() []api.CommandMeta { return nil }
	auth := func(_, _ string) bool { return true }
	engine := api.NewAPIEngine(exec, cmds, auth, nil)

	authenticator := func(header string) (string, bool) {
		switch header {
		case "Bearer alice-token":
			return "alice", true
		case "Bearer bob-token":
			return "bob", true
		default:
			return "", false
		}
	}

	openAPI, _ := api.OpenAPISchema(nil)
	srv, err := NewRESTServer(RESTConfig{
		ListenAddrs:   []string{"127.0.0.1:0"},
		Authenticator: authenticator,
	}, engine, nil, func() []byte { return openAPI })
	require.NoError(t, err)

	// Missing header rejected.
	r := do(t, srv, "POST", "/api/v1/execute", `{"command":"test"}`)
	assert.Equal(t, http.StatusUnauthorized, r.Status)

	// Alice.
	r = doWithHeader(t, srv, "POST", "/api/v1/execute", `{"command":"test"}`, map[string]string{
		"Authorization": "Bearer alice-token",
		"Content-Type":  "application/json",
	})
	assert.Equal(t, http.StatusOK, r.Status)
	assert.Equal(t, "alice", seenUser)

	// Bob.
	r = doWithHeader(t, srv, "POST", "/api/v1/execute", `{"command":"test"}`, map[string]string{
		"Authorization": "Bearer bob-token",
		"Content-Type":  "application/json",
	})
	assert.Equal(t, http.StatusOK, r.Status)
	assert.Equal(t, "bob", seenUser)
}

// TestNewRESTServer_RequiresListenAddrs verifies empty-slice / empty-entry
// rejection.
func TestNewRESTServer_RequiresListenAddrs(t *testing.T) {
	engine := testEngine()
	openAPI, _ := api.OpenAPISchema(nil)

	_, err := NewRESTServer(RESTConfig{}, engine, nil, func() []byte { return openAPI })
	assert.Error(t, err, "empty ListenAddrs must be rejected")
	assert.Contains(t, err.Error(), "at least one listen address")

	_, err = NewRESTServer(RESTConfig{ListenAddrs: []string{""}}, engine, nil, func() []byte { return openAPI })
	assert.Error(t, err, "empty string entry must be rejected")
	assert.Contains(t, err.Error(), "must not be empty")
}

// TestNewRESTServer_RejectsNonLoopback verifies REST rejects all non-loopback
// listen addresses regardless of auth config, since REST has no TLS transport.
//
// VALIDATES: Non-loopback REST listeners are unconditionally rejected.
// PREVENTS: Management traffic crossing the network in cleartext.
func TestNewRESTServer_RejectsNonLoopback(t *testing.T) {
	engine := testEngine()
	openAPI, _ := api.OpenAPISchema(nil)

	tests := []struct {
		name string
		cfg  RESTConfig
	}{
		{
			name: "no_auth",
			cfg:  RESTConfig{ListenAddrs: []string{"0.0.0.0:8081"}},
		},
		{
			name: "token",
			cfg:  RESTConfig{ListenAddrs: []string{"0.0.0.0:8081"}, Token: "secret"},
		},
		{
			name: "per_user_auth",
			cfg: RESTConfig{
				ListenAddrs:   []string{"0.0.0.0:8081"},
				Authenticator: func(string) (string, bool) { return "alice", true },
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRESTServer(tt.cfg, engine, nil, func() []byte { return openAPI })
			require.Error(t, err)
			assert.Contains(t, err.Error(), "must be loopback")
		})
	}
}

// TestRESTServer_MultiListener verifies ListenAndServe binds every entry in
// RESTConfig.ListenAddrs and that both endpoints serve the same engine.
//
// VALIDATES: AC-5 (REST config with two server entries binds both endpoints).
// VALIDATES: AC-14 (Shutdown closes every listener).
// PREVENTS: Regression where only the first REST listener is bound.
func TestRESTServer_MultiListener(t *testing.T) {
	engine := testEngine()
	openAPI, _ := api.OpenAPISchema(nil)

	srv, err := NewRESTServer(RESTConfig{
		ListenAddrs: []string{"127.0.0.1:0", "127.0.0.1:0"},
	}, engine, nil, func() []byte { return openAPI })
	require.NoError(t, err)

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- srv.ListenAndServe(context.Background())
	}()

	// Wait for both listeners to be bound.
	deadline := time.Now().Add(3 * time.Second)
	for !srv.Ready() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, srv.Ready(), "server must become ready within 3s")

	addrs := srv.Addresses()
	require.Len(t, addrs, 2)
	assert.NotEqual(t, addrs[0], addrs[1], "two listeners must bind distinct ports")

	client := &http.Client{Timeout: 3 * time.Second}
	for i, addr := range addrs {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+"/api/v1/commands", http.NoBody)
		require.NoError(t, reqErr, "listener %d", i)
		resp, doErr := client.Do(req)
		require.NoError(t, doErr, "listener %d (%s)", i, addr)
		body, readErr := io.ReadAll(resp.Body)
		require.NoError(t, resp.Body.Close())
		require.NoError(t, readErr)
		assert.Equal(t, http.StatusOK, resp.StatusCode, "listener %d (%s)", i, addr)
		assert.Contains(t, string(body), "show bgp summary", "listener %d (%s)", i, addr)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, srv.Shutdown(shutdownCtx))

	select {
	case err := <-serveErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("ListenAndServe returned unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ListenAndServe did not return after Shutdown")
	}
}

// TestRESTServer_BindFailureClosesPartialListeners verifies that when the
// second entry fails to bind, the first listener is closed and
// ListenAndServe returns the bind error.
//
// VALIDATES: AC-15 (fail-fast on partial bind).
func TestRESTServer_BindFailureClosesPartialListeners(t *testing.T) {
	// Squat on a port to force the second bind to fail.
	squatter, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer squatter.Close() //nolint:errcheck // test cleanup
	squattedAddr := squatter.Addr().String()

	engine := testEngine()
	openAPI, _ := api.OpenAPISchema(nil)

	srv, err := NewRESTServer(RESTConfig{
		ListenAddrs: []string{"127.0.0.1:0", squattedAddr},
	}, engine, nil, func() []byte { return openAPI })
	require.NoError(t, err)

	err = srv.ListenAndServe(context.Background())
	require.Error(t, err, "ListenAndServe must fail when any bind fails")
	assert.Contains(t, err.Error(), squattedAddr)
}

func TestRESTServerStartReturnsBindFailure(t *testing.T) {
	squatter, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer squatter.Close() //nolint:errcheck // test cleanup
	squattedAddr := squatter.Addr().String()

	engine := testEngine()
	openAPI, _ := api.OpenAPISchema(nil)

	srv, err := NewRESTServer(RESTConfig{
		ListenAddrs: []string{"127.0.0.1:0", squattedAddr},
	}, engine, nil, func() []byte { return openAPI })
	require.NoError(t, err)

	errCh, err := srv.Start(context.Background())
	require.Error(t, err, "Start must fail before returning when any bind fails")
	assert.Nil(t, errCh)
	assert.Contains(t, err.Error(), squattedAddr)
}
