package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/api"
)

// testEngine creates an APIEngine with fake implementations for testing.
func testEngine() *api.APIEngine {
	exec := func(_, command string) (string, error) {
		switch command {
		case "bgp summary":
			return `{"peer-count":3}`, nil
		case "show version":
			return `{"version":"1.0"}`, nil
		default:
			return "ok: " + command, nil
		}
	}
	cmds := func() []api.CommandMeta {
		return []api.CommandMeta{
			{Name: "bgp summary", Description: "Show BGP summary", ReadOnly: true},
			{Name: "bgp rib routes", Description: "Show routes", ReadOnly: true, Params: []api.ParamMeta{
				{Name: "family", Type: "string", Description: "Address family"},
			}},
			{Name: "daemon reload", Description: "Reload config", ReadOnly: false},
		}
	}
	auth := func(_, _ string) bool { return true }
	stream := func(_ context.Context, _, _ string) (<-chan string, func(), error) {
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
	openAPI, err := api.OpenAPISchema(engine.ListCommands(""))
	require.NoError(t, err)

	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		return &fakeEditor{values: make(map[string]string)}, nil
	})

	srv, err := NewRESTServer(RESTConfig{ListenAddr: "127.0.0.1:0"}, engine, sessions, func() []byte { return openAPI })
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
	values map[string]string
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

func (e *fakeEditor) Save() error            { return nil }
func (e *fakeEditor) Discard() error         { e.values = make(map[string]string); return nil }
func (e *fakeEditor) WorkingContent() string { return "# config\n" }

// VALIDATES: AC-1 -- GET /api/v1/commands returns command list.
// PREVENTS: missing commands in REST response.
func TestRESTListCommands(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "GET", "/api/v1/commands", "")
	assert.Equal(t, http.StatusOK, r.Status)

	var cmds []api.CommandMeta
	require.NoError(t, json.Unmarshal([]byte(r.Body), &cmds))
	assert.Len(t, cmds, 3)
}

// VALIDATES: AC-2 -- POST /api/v1/execute returns command output.
// PREVENTS: execute endpoint broken.
func TestRESTExecute(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "POST", "/api/v1/execute", `{"command":"bgp summary"}`)
	assert.Equal(t, http.StatusOK, r.Status)

	var result api.ExecResult
	require.NoError(t, json.Unmarshal([]byte(r.Body), &result))
	assert.Equal(t, "done", result.Status)
}

// VALIDATES: AC-3 -- POST /api/v1/execute without auth returns 401.
// PREVENTS: unauthenticated access.
func TestRESTExecuteUnauthorized(t *testing.T) {
	engine := testEngine()
	openAPI, _ := api.OpenAPISchema(nil)
	srv, err := NewRESTServer(RESTConfig{ListenAddr: "127.0.0.1:0", Token: "secret"}, engine, nil, func() []byte { return openAPI })
	require.NoError(t, err)

	// No Authorization header.
	r := do(t, srv, "POST", "/api/v1/execute", `{"command":"bgp summary"}`)
	assert.Equal(t, http.StatusUnauthorized, r.Status)

	// Wrong token.
	r = doWithHeader(t, srv, "POST", "/api/v1/execute", `{"command":"bgp summary"}`, map[string]string{
		"Authorization": "Bearer wrong",
		"Content-Type":  "application/json",
	})
	assert.Equal(t, http.StatusUnauthorized, r.Status)

	// Correct token.
	r = doWithHeader(t, srv, "POST", "/api/v1/execute", `{"command":"bgp summary"}`, map[string]string{
		"Authorization": "Bearer secret",
		"Content-Type":  "application/json",
	})
	assert.Equal(t, http.StatusOK, r.Status)
}

// VALIDATES: AC-4 -- GET /api/v1/peers returns peer summary.
// PREVENTS: convenience route broken.
func TestRESTPeersConvenience(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "GET", "/api/v1/peers", "")
	assert.Equal(t, http.StatusOK, r.Status)

	var result api.ExecResult
	require.NoError(t, json.Unmarshal([]byte(r.Body), &result))
	assert.Equal(t, "done", result.Status)
}

// VALIDATES: AC-5 -- Config session create + set + commit.
// PREVENTS: config lifecycle broken over REST.
func TestRESTConfigSession(t *testing.T) {
	srv := testServer(t)

	// Create session.
	r := do(t, srv, "POST", "/api/v1/config/sessions", "")
	assert.Equal(t, http.StatusCreated, r.Status)
	var created map[string]string
	require.NoError(t, json.Unmarshal([]byte(r.Body), &created))
	id := created["session-id"]
	assert.NotEmpty(t, id)

	// Set value.
	r = do(t, srv, "PUT", "/api/v1/config/sessions/"+id,
		`{"path":"bgp.router-id","value":"10.0.0.1"}`)
	assert.Equal(t, http.StatusOK, r.Status)

	// Diff.
	r = do(t, srv, "GET", "/api/v1/config/sessions/"+id+"/diff", "")
	assert.Equal(t, http.StatusOK, r.Status)
	assert.Contains(t, r.Body, "diff")

	// Commit.
	r = do(t, srv, "POST", "/api/v1/config/sessions/"+id+"/commit", "")
	assert.Equal(t, http.StatusOK, r.Status)
	assert.Contains(t, r.Body, "committed")
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
		ListenAddr: "127.0.0.1:0",
		CORSOrigin: "https://dashboard.example.com",
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
	r := do(t, srv, "GET", "/api/v1/commands/bgp/rib/routes", "")
	assert.Equal(t, http.StatusOK, r.Status)

	var cmd api.CommandMeta
	require.NoError(t, json.Unmarshal([]byte(r.Body), &cmd))
	assert.Equal(t, "bgp rib routes", cmd.Name)
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
	r := do(t, srv, "POST", "/api/v1/execute", `{"command":"bgp rib routes","params":{"family":"ipv4/unicast"}}`)
	assert.Equal(t, http.StatusOK, r.Status)
	// The fake executor returns "ok: <command>" for unknown commands.
	assert.Contains(t, r.Body, "bgp rib routes family ipv4/unicast")
}

// VALIDATES: Execute rejects param keys with whitespace.
// PREVENTS: command injection via param keys.
func TestRESTExecuteParamKeyInjection(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "POST", "/api/v1/execute", `{"command":"bgp summary","params":{"bad key":"value"}}`)
	assert.Equal(t, http.StatusBadRequest, r.Status)
	assert.Contains(t, r.Body, "whitespace")
}

// VALIDATES: Execute rejects param values with whitespace.
// PREVENTS: command injection via param values.
func TestRESTExecuteParamValueInjection(t *testing.T) {
	srv := testServer(t)
	r := do(t, srv, "POST", "/api/v1/execute", `{"command":"bgp summary","params":{"family":"ipv4 unicast"}}`)
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
	exec := func(username, _ string) (string, error) {
		seenUser = username
		return `"ok"`, nil
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
		ListenAddr:    "127.0.0.1:0",
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
