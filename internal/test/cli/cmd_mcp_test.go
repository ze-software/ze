package cli

// TestStreamableGETviaZeTest and TestSSEExpectRequiresListen tested
// the GET /mcp SSE stream (sse-listen / sse-expect), a mechanism MCP revision
// 2026-07-28 removes outright ("Removal of the GET stream endpoint"). They are
// replaced below by tests for the per-request transport that took its place.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// captured is one request the fake MCP server saw.
type captured struct {
	method  string
	headers http.Header
	body    map[string]any
}

// fakeMCP starts an HTTP server that records every request and answers with the
// supplied JSON body. A test can then assert on what the client PUT ON THE WIRE
// rather than on what a real server chose to accept.
func fakeMCP(t *testing.T, status int, response string) (*mcpClient, *[]captured) {
	t.Helper()
	seen := make([]captured, 0, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		entry := captured{method: r.Method, headers: r.Header.Clone()}
		if err := json.Unmarshal(raw, &entry.body); err != nil {
			entry.body = nil
		}
		seen = append(seen, entry)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if _, err := io.WriteString(w, response); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	client := &mcpClient{addr: strings.TrimPrefix(srv.URL, "http://"), http: srv.Client()}
	return client, &seen
}

func params(t *testing.T, req captured) map[string]any {
	t.Helper()
	if req.body == nil {
		t.Fatalf("request body was not a JSON object")
	}
	p, ok := req.body["params"].(map[string]any)
	if !ok {
		t.Fatalf("request params is not an object: %v", req.body["params"])
	}
	return p
}

func meta(t *testing.T, req captured) map[string]any {
	t.Helper()
	m, ok := params(t, req)["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("params carries no _meta object: %v", params(t, req))
	}
	return m
}

// TestConformantRequestShape pins every wire requirement the 2026-07-28
// Streamable HTTP binding puts on a client POST.
// VALIDATES: MCP-Protocol-Version / Mcp-Method / Mcp-Name headers, the Accept
// header listing both content types, and _meta carried INSIDE params.
// PREVENTS: regression to the 2025-06-18 shape (initialize handshake,
// Mcp-Session-Id, _meta at the message top level).
func TestConformantRequestShape(t *testing.T) {
	client, seen := fakeMCP(t, http.StatusOK,
		`{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete","content":[{"type":"text","text":"ok"}]}}`)

	if _, err := client.execute("show version"); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(*seen) != 1 {
		t.Fatalf("want exactly 1 request (no handshake), got %d", len(*seen))
	}
	req := (*seen)[0]

	if req.method != http.MethodPost {
		t.Errorf("HTTP method = %q, want POST", req.method)
	}
	if got := req.headers.Get(headerProtocolVersion); got != mcpProtocolVersion {
		t.Errorf("%s = %q, want %q", headerProtocolVersion, got, mcpProtocolVersion)
	}
	if got := req.headers.Get(headerMethod); got != methodToolsCall {
		t.Errorf("%s = %q, want %q", headerMethod, got, methodToolsCall)
	}
	if got := req.headers.Get(headerName); got != "ze_execute" {
		t.Errorf("%s = %q, want %q", headerName, got, "ze_execute")
	}
	accept := req.headers.Get("Accept")
	if !strings.Contains(accept, "application/json") || !strings.Contains(accept, "text/event-stream") {
		t.Errorf("Accept = %q, must list both application/json and text/event-stream", accept)
	}
	if got := req.headers.Get("Mcp-Session-Id"); got != "" {
		t.Errorf("Mcp-Session-Id = %q, the header is removed in %s", got, mcpProtocolVersion)
	}
	if _, top := req.body["_meta"]; top {
		t.Errorf("_meta sits at the message top level; it MUST be inside params")
	}

	m := meta(t, req)
	if got := m[metaProtocolVersion]; got != mcpProtocolVersion {
		t.Errorf("_meta[%s] = %v, want %q", metaProtocolVersion, got, mcpProtocolVersion)
	}
	if _, ok := m[metaClientCapabilities].(map[string]any); !ok {
		t.Errorf("_meta[%s] missing or not an object: %v", metaClientCapabilities, m[metaClientCapabilities])
	}
	if _, ok := m[metaClientInfo].(map[string]any); !ok {
		t.Errorf("_meta[%s] missing or not an object: %v", metaClientInfo, m[metaClientInfo])
	}
}

// TestClientCapabilitiesPerRequest proves the capability flag reaches every
// request's _meta rather than a one-off handshake.
//
// the two --resources rows are removed with the flag itself, not
// weakened. `resources` is a member of *ServerCapabilities*. The five
// ClientCapabilities members in MCP 2026-07-28 are `experimental`, `roots`,
// `sampling`, `elicitation` and `extensions`. A client that declared
// `resources` declared something no server can key on, and the daemon's gate on
// it (which refused every conformant caller) is gone. A stronger assertion
// below replaces the rows: whatever the flag, NOTHING that is not a real client
// capability is ever emitted.
//
// VALIDATES: --tasks declares the tasks extension, its absence yields the
// conformant empty object, and neither shape ever names a server capability.
func TestClientCapabilitiesPerRequest(t *testing.T) {
	tests := []struct {
		name  string
		tasks bool
		check func(t *testing.T, caps map[string]any)
	}{
		{
			name: "no flags declares an empty capability object",
			check: func(t *testing.T, caps map[string]any) {
				if len(caps) != 0 {
					t.Errorf("clientCapabilities = %v, want {}", caps)
				}
			},
		},
		{
			name:  "tasks declares the tasks extension",
			tasks: true,
			check: func(t *testing.T, caps map[string]any) {
				ext, ok := caps["extensions"].(map[string]any)
				if !ok {
					t.Fatalf("clientCapabilities.extensions missing: %v", caps)
				}
				if _, ok := ext[extensionTasks]; !ok {
					t.Errorf("extensions missing %q: %v", extensionTasks, ext)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, seen := fakeMCP(t, http.StatusOK, `{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete","tools":[]}}`)
			client.declareTasks = tt.tasks

			if _, err := client.send("tools/list", map[string]any{}); err != nil {
				t.Fatalf("send: %v", err)
			}
			caps, ok := meta(t, (*seen)[0])[metaClientCapabilities].(map[string]any)
			if !ok {
				t.Fatalf("clientCapabilities absent from _meta")
			}
			// No shape names a SERVER capability. A client that declared one
			// would invite a server to gate on something it can never send.
			for _, serverCap := range []string{"resources", "tools", "prompts", "logging", "completions"} {
				if _, present := caps[serverCap]; present {
					t.Errorf("clientCapabilities names the server capability %q: %v", serverCap, caps)
				}
			}
			// Every remaining member must be one of the five ClientCapabilities
			// members the specification defines.
			legal := map[string]bool{
				"experimental": true, "roots": true, "sampling": true,
				"elicitation": true, "extensions": true,
			}
			for k := range caps {
				if !legal[k] {
					t.Errorf("clientCapabilities member %q is not a ClientCapabilities member: %v", k, caps)
				}
			}
			tt.check(t, caps)
		})
	}
}

// TestHeaderValueEncoding walks the specification's own Value Encoding table.
// VALIDATES: standard Base64 WITH padding over the UTF-8 bytes, lowercase
// case-sensitive sentinel markers, and the rule that a plain-ASCII value
// matching the sentinel pattern MUST itself be encoded.
func TestHeaderValueEncoding(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"plain ascii rides unencoded", "us-west1", "us-west1"},
		{"tool name rides unencoded", "get_weather", "get_weather"},
		{"resource uri rides unencoded", "file:///projects/myapp/config.json", "file:///projects/myapp/config.json"},
		{"non-ascii is encoded", "Hello, 世界", "=?base64?SGVsbG8sIOS4lueVjA==?="},
		{"leading and trailing space is encoded", " padded ", "=?base64?IHBhZGRlZCA=?="},
		{"newline is encoded", "line1\nline2", "=?base64?bGluZTEKbGluZTI=?="},
		// The "/" in the payload is load-bearing: it proves standard Base64,
		// not base64url, which would have emitted "_".
		{"a value matching the sentinel is encoded", "=?base64?literal?=", "=?base64?PT9iYXNlNjQ/bGl0ZXJhbD89?="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := encodeHeaderValue(tt.value); got != tt.want {
				t.Errorf("encodeHeaderValue(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// TestMcpNameSource pins which methods carry Mcp-Name and where its value comes
// from. The value is params.name for tools/call and prompts/get, params.uri for
// resources/read, and nothing for every other method.
func TestMcpNameSource(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		params  map[string]any
		want    string
		wantSet bool
	}{
		{"tools/call takes params.name", methodToolsCall, map[string]any{keyName: "get_weather"}, "get_weather", true},
		{"prompts/get takes params.name", "prompts/get", map[string]any{keyName: "review"}, "review", true},
		{"resources/read takes params.uri", "resources/read", map[string]any{"uri": "file:///a.json"}, "file:///a.json", true},
		{"tools/list carries no name", "tools/list", map[string]any{}, "", false},
		{"server/discover carries no name", "server/discover", map[string]any{}, "", false},
		{"tools/call without a name yields nothing", methodToolsCall, map[string]any{}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := mcpNameFor(tt.method, tt.params)
			if ok != tt.wantSet || got != tt.want {
				t.Errorf("mcpNameFor(%q) = (%q, %v), want (%q, %v)", tt.method, got, ok, tt.want, tt.wantSet)
			}
		})
	}
}

// TestResultTypeContract pins the ResultType rules. An absent value means
// "complete", and an unrecognized value is invalid. An MRTR interim result that
// the client has queued no answer for fails closed, rather than reporting as a
// completed call.
//
// Driven through send(), the entry point every directive uses, rather than
// through the classifier alone. The multi round-trip loop (cmd_mcp_mrtr.go) now
// takes the input_required verdict. An assertion on the classifier alone would
// therefore no longer prove that the call fails closed.
func TestResultTypeContract(t *testing.T) {
	tests := []struct {
		name    string
		result  string
		wantErr bool
	}{
		{"absent resultType is complete", `{"content":[]}`, false},
		{"complete is accepted", `{"resultType":"complete"}`, false},
		{"input_required fails closed", `{"resultType":"input_required","requestState":"blob"}`, true},
		{"an unrecognized value is invalid", `{"resultType":"partial"}`, true},
		{"a non-string resultType is invalid", `{"resultType":7}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := `{"jsonrpc":"2.0","id":1,"result":` + tt.result + `}`
			client, _ := fakeMCP(t, http.StatusOK, frame)
			_, err := client.send(methodToolsList, map[string]any{})
			if (err != nil) != tt.wantErr {
				t.Errorf("send with result %s: error = %v, wantErr %v", tt.result, err, tt.wantErr)
			}
		})
	}
}

// TestSSEResponseStreamIsPerRequest proves the client handles the SSE answer a
// server MAY return. Keep-alive comments are ignored, and request-scoped
// notifications are skipped. Multi-line data payloads accumulate, and the final
// response ends the read.
func TestSSEResponseStreamIsPerRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		frames := ": keep-alive\n\n" +
			`data: {"jsonrpc":"2.0","method":"notifications/progress","params":{"progress":1}}` + "\n\n" +
			`data: {"jsonrpc":"2.0","id":1,` + "\n" +
			`data: "result":{"resultType":"complete","content":[{"type":"text","text":"streamed"}]}}` + "\n\n"
		if _, err := io.WriteString(w, frames); err != nil {
			t.Errorf("write SSE frames: %v", err)
		}
	}))
	defer srv.Close()

	client := &mcpClient{addr: strings.TrimPrefix(srv.URL, "http://"), http: srv.Client()}
	got, err := client.execute("show version")
	if err != nil {
		t.Fatalf("execute over SSE: %v", err)
	}
	if got != "streamed" {
		t.Errorf("result text = %q, want %q", got, "streamed")
	}
}

// TestProbeMutationsReachTheWire proves the malformed-request directives change
// the bytes sent. A dropped header is absent, an overridden header wins
// verbatim, and a dropped _meta field is gone from params._meta.
func TestProbeMutationsReachTheWire(t *testing.T) {
	client, seen := fakeMCP(t, http.StatusOK, `{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete"}}`)

	var mut probeMutation
	mut.setHeader(headerMethod, probeOmit)
	mut.setHeader(headerProtocolVersion, "2025-06-18")
	mut.setMeta(metaClientCapabilities, nil, true)

	if _, err := client.exchange("tools/list", map[string]any{}, &mut); err != nil {
		t.Fatalf("exchange: %v", err)
	}
	req := (*seen)[0]

	if got := req.headers.Get(headerMethod); got != "" {
		t.Errorf("%s = %q, probe-header %s - must omit it", headerMethod, got, probeOmit)
	}
	if got := req.headers.Get(headerProtocolVersion); got != "2025-06-18" {
		t.Errorf("%s = %q, want the overridden %q", headerProtocolVersion, got, "2025-06-18")
	}
	m := meta(t, req)
	if _, present := m[metaClientCapabilities]; present {
		t.Errorf("_meta still carries %s after probe-meta clientCapabilities %s", metaClientCapabilities, probeOmit)
	}
	if _, present := m[metaProtocolVersion]; !present {
		t.Errorf("_meta lost %s, which was not dropped", metaProtocolVersion)
	}
}

// TestProbeMetaVersionDrivesTheHeader proves the header is derived from the
// _meta value, so a version probe sends a CONSISTENT pair (which exercises
// version rejection) rather than an accidental header/body mismatch.
func TestProbeMetaVersionDrivesTheHeader(t *testing.T) {
	client, seen := fakeMCP(t, http.StatusOK, `{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete"}}`)

	var mut probeMutation
	mut.setMeta(metaProtocolVersion, "2025-06-18", false)

	if _, err := client.exchange("tools/list", map[string]any{}, &mut); err != nil {
		t.Fatalf("exchange: %v", err)
	}
	req := (*seen)[0]
	if got := req.headers.Get(headerProtocolVersion); got != "2025-06-18" {
		t.Errorf("%s = %q, want it derived from _meta (%q)", headerProtocolVersion, got, "2025-06-18")
	}
	if got := meta(t, req)[metaProtocolVersion]; got != "2025-06-18" {
		t.Errorf("_meta[%s] = %v, want %q", metaProtocolVersion, got, "2025-06-18")
	}
}

// TestProbeDroppingEveryMetaFieldOmitsMeta proves a legacy-shaped request is
// expressible: no _meta at all, which is what an initialize POST looks like.
func TestProbeDroppingEveryMetaFieldOmitsMeta(t *testing.T) {
	client, seen := fakeMCP(t, http.StatusBadRequest,
		`{"jsonrpc":"2.0","id":1,"error":{"code":-32020,"message":"missing MCP-Protocol-Version; this server speaks 2026-07-28"}}`)

	var mut probeMutation
	for _, key := range []string{metaProtocolVersion, metaClientCapabilities, metaClientInfo} {
		mut.setMeta(key, nil, true)
	}
	out, err := client.exchange("initialize", map[string]any{}, &mut)
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if _, present := params(t, (*seen)[0])["_meta"]; present {
		t.Errorf("params still carries _meta after every field was dropped")
	}

	line, err := formatProbeResult(out)
	if err != nil {
		t.Fatalf("formatProbeResult: %v", err)
	}
	if !strings.Contains(line, "status=400") || !strings.Contains(line, "code=-32020") {
		t.Errorf("probe line = %q, want both the HTTP status and the JSON-RPC code", line)
	}
}

// TestProbeReportsStatusCodeAndData proves the single probe output line carries
// everything a .ci needs to assert: the HTTP status, the JSON-RPC code, and the
// error data object.
func TestProbeReportsStatusCodeAndData(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		response string
		want     []string
	}{
		{
			name:     "unsupported version carries data.supported",
			status:   http.StatusBadRequest,
			response: `{"jsonrpc":"2.0","id":1,"error":{"code":-32022,"message":"Unsupported protocol version","data":{"supported":["2026-07-28"],"requested":"2025-06-18"}}}`,
			want:     []string{"status=400", "code=-32022", `"supported":["2026-07-28"]`},
		},
		{
			name:     "missing capability carries data.requiredCapabilities",
			status:   http.StatusBadRequest,
			response: `{"jsonrpc":"2.0","id":1,"error":{"code":-32021,"message":"missing capability","data":{"requiredCapabilities":{"resources":{}}}}}`,
			want:     []string{"status=400", "code=-32021", "requiredCapabilities"},
		},
		{
			name:     "malformed meta is -32602, not a header mismatch",
			status:   http.StatusBadRequest,
			response: `{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"_meta missing protocolVersion"}}`,
			want:     []string{"status=400", "code=-32602"},
		},
		{
			name:     "a success prints the result so resultType can be asserted",
			status:   http.StatusOK,
			response: `{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete","supportedVersions":["2026-07-28"]}}`,
			want:     []string{"status=200", "code=ok", `"resultType":"complete"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := fakeMCP(t, tt.status, tt.response)
			out, err := client.exchange("server/discover", map[string]any{}, nil)
			if err != nil {
				t.Fatalf("exchange: %v", err)
			}
			line, err := formatProbeResult(out)
			if err != nil {
				t.Fatalf("formatProbeResult: %v", err)
			}
			for _, want := range tt.want {
				if !strings.Contains(line, want) {
					t.Errorf("probe line %q does not contain %q", line, want)
				}
			}
		})
	}
}

// TestProbeReportsNonJSONBody proves a status-only rejection (a 405 page with
// no JSON-RPC body) still reports its HTTP status, which is how the GET and
// DELETE method-not-allowed cases are asserted.
func TestProbeReportsNonJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete"}}`); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	client := &mcpClient{addr: strings.TrimPrefix(srv.URL, "http://"), http: srv.Client()}
	mut := probeMutation{httpMethod: http.MethodGet}
	out, err := client.exchange("tools/list", map[string]any{}, &mut)
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	line, err := formatProbeResult(out)
	if err != nil {
		t.Fatalf("formatProbeResult: %v", err)
	}
	if !strings.Contains(line, "status=405") || !strings.Contains(line, "code=none") {
		t.Errorf("probe line = %q, want status=405 and code=none", line)
	}
}

// TestRPCErrorCarriesStatusAndCode proves a normal (non-probe) directive still
// surfaces both numbers, so a .ci asserting on stderr can match either.
func TestRPCErrorCarriesStatusAndCode(t *testing.T) {
	client, _ := fakeMCP(t, http.StatusBadRequest,
		`{"jsonrpc":"2.0","id":1,"error":{"code":-32020,"message":"Header mismatch"}}`)

	_, err := client.execute("show version")
	if err == nil {
		t.Fatalf("expected an error for a -32020 response")
	}
	if !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "-32020") {
		t.Errorf("error = %q, want it to name both the HTTP status and the JSON-RPC code", err)
	}
}

// TestWaitReadyFailsOnADeadPort proves the readiness probe reports the endpoint
// it failed to reach, and does not hang.
func TestWaitReadyFailsOnADeadPort(t *testing.T) {
	client := &mcpClient{addr: "127.0.0.1:1", http: &http.Client{}}
	err := client.waitReady(150 * time.Millisecond)
	if err == nil {
		t.Fatalf("expected waitReady to fail against a closed port")
	}
	if !strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Errorf("error = %q, want it to name the unreachable endpoint", err)
	}
}
