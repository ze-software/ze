// This file lost the assertions belonging to features MCP 2026-07-28 removes: the
// initialize handshake, the session registry and its caps, the GET SSE stream and
// DELETE. Every one of them named a function or field that no longer exists. The
// fail-open assertion in TestStreamableProtocolVersionMissingAssumesLegacy is
// deliberately INVERTED, not dropped: a missing MCP-Protocol-Version is now
// -32020, asserted in headers_test.go TestHeaderMismatchRejected. Coverage the
// cutover keeps is carried forward below (origin, bearer auth, the task-support
// table). Coverage it adds lives in headers_test.go, meta_test.go and
// discover_test.go.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/audit"
)

// newTestStreamable returns a Streamable wired with a trivial dispatcher and
// an httptest.Server. Caller MUST call returned cleanup.
func newTestStreamable(t *testing.T, cfg StreamableConfig) (*httptest.Server, func()) {
	t.Helper()
	if cfg.Dispatch == nil {
		cfg.Dispatch = func(_ context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, plugin.Map{"result": "ok", "message": cmd}), nil
		}
	}
	srv, err := NewStreamable(cfg)
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	hs := httptest.NewServer(srv)
	return hs, func() {
		hs.Close()
		srv.Close()
	}
}

// closeBody closes resp.Body ignoring error (test cleanup helper).
func closeBody(t *testing.T, body io.Closer) {
	t.Helper()
	if err := body.Close(); err != nil {
		t.Logf("body close: %v", err)
	}
}

// Client-capability literals the helpers below accept. Empty is the conformant
// "I declare nothing" form, and tasks is the only capability this server gates
// on. `resources` is a ServerCapabilities member, so there is deliberately no
// literal that declares it.
//
// capsTasks is spelled as an EXTENSION declaration, not as a bare `tasks`
// member. MCP 2026-07-28 moved tasks out of the core protocol onto the
// io.modelcontextprotocol/tasks extension, and extensions are declared through
// the `extensions` map (basic/versioning "Extension Negotiation"). The bare
// member was the 2025-11-25 core spelling and is no longer accepted:
// TestBareTasksMemberIsNotAnExtensionDeclaration pins that.
const (
	capsNone  = `{}`
	capsTasks = `{"extensions":{"io.modelcontextprotocol/tasks":{}}}`
)

// metaBlock returns the params._meta JSON literal a conformant request carries.
func metaBlock(version, capsJSON string) string {
	return `{"io.modelcontextprotocol/protocolVersion":"` + version +
		`","io.modelcontextprotocol/clientCapabilities":` + capsJSON + `}`
}

// postRaw POSTs body with exactly the given headers (plus Content-Type) and
// returns the status and the decoded JSON body. Nothing is injected: this is
// the helper for tests that control the headers themselves.
func postRaw(t *testing.T, hs *httptest.Server, body string, headers map[string]string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, hs.URL+Endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer closeBody(t, resp.Body)
	raw, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		t.Fatalf("read body: %v", readErr)
	}
	// Best-effort decode: the auth and origin guards answer with http.Error,
	// whose body is plain text rather than a JSON-RPC envelope. Callers that
	// need the envelope assert on it through rpcErrorOf / resultOf, which fail
	// loudly when it is absent.
	var parsed map[string]any
	if len(raw) > 0 {
		if decodeErr := json.Unmarshal(raw, &parsed); decodeErr != nil {
			parsed = nil
		}
	}
	return resp.StatusCode, parsed
}

// buildMCPBody assembles a conformant 2026-07-28 request: the caller's params
// with the required `_meta` block merged in. Returns the marshaled body and
// the params object, which the header builder reads Mcp-Name out of.
func buildMCPBody(t *testing.T, id any, method, capsJSON, paramsJSON string) (string, map[string]any) {
	t.Helper()
	params := map[string]any{}
	if paramsJSON != "" {
		if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
			t.Fatalf("params %q: %v", paramsJSON, err)
		}
	}
	caps := map[string]any{}
	if capsJSON != "" {
		if err := json.Unmarshal([]byte(capsJSON), &caps); err != nil {
			t.Fatalf("caps %q: %v", capsJSON, err)
		}
	}
	params[metaKey] = map[string]any{
		metaKeyProtocolVersion:    ProtocolVersion,
		metaKeyClientCapabilities: caps,
	}
	msg := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if id != nil {
		msg["id"] = id
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data), params
}

// mcpHeaders mirrors the body into the standard request headers exactly as a
// conformant client must. Mcp-Name is sourced through the production rule so
// the helper cannot disagree with the server about which methods need it;
// headers_test.go drives that rule directly with hand-written headers.
func mcpHeaders(method string, params map[string]any) map[string]string {
	h := map[string]string{
		"MCP-Protocol-Version": ProtocolVersion,
		"Mcp-Method":           method,
	}
	if source, required := mcpNameSource(method, params); required {
		h["Mcp-Name"] = source
	}
	return h
}

// postMCP sends a fully conformant request and returns status + decoded body.
func postMCP(t *testing.T, hs *httptest.Server, method, capsJSON, paramsJSON string) (int, map[string]any) {
	t.Helper()
	return postMCPAuth(t, hs, "", method, capsJSON, paramsJSON)
}

// postMCPAuth is postMCP with an Authorization: Bearer credential.
func postMCPAuth(t *testing.T, hs *httptest.Server, bearer, method, capsJSON, paramsJSON string) (int, map[string]any) {
	t.Helper()
	body, params := buildMCPBody(t, 1, method, capsJSON, paramsJSON)
	headers := mcpHeaders(method, params)
	if bearer != "" {
		headers["Authorization"] = "Bearer " + bearer
	}
	return postRaw(t, hs, body, headers)
}

// postOrigin sends a conformant tools/list with a custom Origin header and
// returns only the status.
func postOrigin(t *testing.T, hs *httptest.Server, origin string) int {
	t.Helper()
	body, params := buildMCPBody(t, 1, methodToolsList, capsNone, "")
	headers := mcpHeaders(methodToolsList, params)
	if origin != "" {
		headers["Origin"] = origin
	}
	status, _ := postRaw(t, hs, body, headers)
	return status
}

// rpcErrorOf extracts the JSON-RPC error object, failing the test when absent.
func rpcErrorOf(t *testing.T, parsed map[string]any) map[string]any {
	t.Helper()
	e, ok := parsed["error"].(map[string]any)
	if !ok {
		t.Fatalf("no error object in %v", parsed)
	}
	return e
}

// resultOf extracts the JSON-RPC result object, failing the test when absent.
func resultOf(t *testing.T, parsed map[string]any) map[string]any {
	t.Helper()
	r, ok := parsed["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result object in %v", parsed)
	}
	return r
}

// TestProtocolVersionIsCurrentRevision pins the cutover.
//
// VALIDATES: the version constant is 2026-07-28 and the supported set holds
// that and nothing else.
// PREVENTS: a dropped revision surviving as an accepted or advertised version
// (ai/rules/go-standards.md: no shim for a dropped revision).
func TestProtocolVersionIsCurrentRevision(t *testing.T) {
	if ProtocolVersion != "2026-07-28" {
		t.Fatalf("ProtocolVersion = %q, want 2026-07-28", ProtocolVersion)
	}
	if len(supportedProtocolVersions) != 1 {
		t.Fatalf("supportedProtocolVersions = %v, want exactly one entry", supportedProtocolVersions)
	}
	if supportedProtocolVersions[0] != ProtocolVersion {
		t.Fatalf("supportedProtocolVersions[0] = %q, want %q", supportedProtocolVersions[0], ProtocolVersion)
	}
	for _, dropped := range []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"} {
		if isSupportedProtocolVersion(dropped) {
			t.Errorf("dropped revision %q still accepted", dropped)
		}
	}
}

// TestUnsupportedVersionListsSupported covers AC-2.
//
// VALIDATES: a request declaring a version this server does not implement is
// answered with HTTP 400, -32022, data.supported == ["2026-07-28"] exactly, and
// data.requested echoing what the client asked for.
// PREVENTS: an empty supported list (which fails closed into unreachability) or
// a list longer than one (which would mean a dropped revision survived).
func TestUnsupportedVersionListsSupported(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	for _, requested := range []string{"2025-06-18", "2025-11-25", "1900-01-01"} {
		t.Run(requested, func(t *testing.T) {
			// Header and body agree, so this reaches the version check rather
			// than header validation.
			body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":` +
				metaBlock(requested, capsNone) + `}}`
			status, parsed := postRaw(t, hs, body, map[string]string{
				"MCP-Protocol-Version": requested,
				"Mcp-Method":           "tools/list",
			})
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %v)", status, parsed)
			}
			rpcErr := rpcErrorOf(t, parsed)
			if rpcErr["code"] != float64(rpcUnsupportedProtocolVersion) {
				t.Fatalf("code = %v, want %d", rpcErr["code"], rpcUnsupportedProtocolVersion)
			}
			data, ok := rpcErr["data"].(map[string]any)
			if !ok {
				t.Fatalf("no data object in %v", rpcErr)
			}
			supported, ok := data["supported"].([]any)
			if !ok {
				t.Fatalf("data.supported = %v, want an array", data["supported"])
			}
			if len(supported) != 1 || supported[0] != ProtocolVersion {
				t.Fatalf("data.supported = %v, want exactly [%q]", supported, ProtocolVersion)
			}
			if data["requested"] != requested {
				t.Fatalf("data.requested = %v, want %q", data["requested"], requested)
			}
		})
	}
}

// methodProbe is one row of the every-method result table.
type methodProbe struct {
	method string
	caps   string
	params string
}

// resultBearingMethods returns one conformant, result-producing call per method
// this server dispatches, so a table can assert an envelope invariant across all
// of them. taskID is spliced into the task lookups, and names a TERMINAL task so
// tasks/get carries a result payload rather than a bare working status.
//
// Every method in runMethod's switch must appear here. tasks/list and
// tasks/result are gone (changelog Major change 6) and tasks/update took their
// place in this table.
func resultBearingMethods(taskID string) []methodProbe {
	taskParams := `{"taskId":"` + taskID + `"}`
	return []methodProbe{
		{method: methodServerDiscover, caps: capsNone},
		{method: methodToolsList, caps: capsNone},
		{method: methodToolsCall, caps: capsNone, params: `{"name":"ze_execute","arguments":{"command":"show version"}}`},
		{method: methodTasksGet, caps: capsTasks, params: taskParams},
		{method: methodTasksUpdate, caps: capsTasks, params: taskParams},
		{method: methodTasksCancel, caps: capsTasks, params: taskParams},
		{method: methodResourcesList, caps: capsNone},
		{method: methodResourcesRead, caps: capsNone, params: `{"uri":"ui://bgp-peer/index.html"}`},
	}
}

// taskCapableCommands is the command set a test server needs before it can
// produce a task at all.
//
// Under the server-directed model (D-1) the annotation is the ONLY thing that
// creates a task. The handcrafted tools (ze_execute, ze_reference) resolve to
// TaskSupportOptional, which means synchronous. So a test that wants a task
// must configure a `required` command. There is no request that a test can
// send to create one.
func taskCapableCommands() []CommandInfo {
	return []CommandInfo{
		{Name: "slow cmd", Help: "Long", TaskSupport: TaskSupportRequired},
	}
}

// createTestTask calls the `required` tool, waits for the task to reach a
// terminal state, and returns its id. Waiting is what makes the tasks/get row
// carry a result payload rather than a bare working status.
func createTestTask(t *testing.T, hs *httptest.Server) string {
	t.Helper()
	status, parsed := postMCP(t, hs, methodToolsCall, capsTasks,
		`{"name":"ze_slow","arguments":{"action":"cmd"}}`)
	if status != http.StatusOK {
		t.Fatalf("create task: status = %d (body %v)", status, parsed)
	}
	result := resultOf(t, parsed)
	if got := result[resultTypeKey]; got != resultTypeTask {
		t.Fatalf("create task: resultType = %v, want %q (body %v)", got, resultTypeTask, parsed)
	}
	id, _ := result["taskId"].(string)
	if id == "" {
		t.Fatalf("no taskId in %v", result)
	}
	waitTaskTerminal(t, hs, id)
	return id
}

// waitTaskTerminal polls tasks/get over the transport until the task leaves the
// working state. It waits on the CONDITION, not on a duration
// (ai/rules/completion.md).
func waitTaskTerminal(t *testing.T, hs *httptest.Server, taskID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last any
	for time.Now().Before(deadline) {
		status, parsed := postMCP(t, hs, methodTasksGet, capsTasks, `{"taskId":"`+taskID+`"}`)
		if status != http.StatusOK {
			t.Fatalf("tasks/get: status = %d (body %v)", status, parsed)
		}
		last = resultOf(t, parsed)["status"]
		if last != TaskWorking.String() {
			return
		}
		time.Sleep(time.Millisecond) // poll interval; the loop returns as soon as the task leaves "working"
	}
	t.Fatalf("task %s never left %q (last status %v)", taskID, TaskWorking.String(), last)
}

// TestEveryResultCarriesResultType covers AC-5.
//
// VALIDATES: every successful result from every dispatched method carries
// resultType "complete".
// PREVENTS: a handler that bypasses the shared ok() helper and ships a result a
// 2026-07-28 client must reject as untyped.
func TestEveryResultCarriesResultType(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{Commands: taskCapableCommands})
	defer cleanup()

	taskID := createTestTask(t, hs)
	for _, probe := range resultBearingMethods(taskID) {
		t.Run(probe.method, func(t *testing.T) {
			status, parsed := postMCP(t, hs, probe.method, probe.caps, probe.params)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
			}
			result := resultOf(t, parsed)
			if result["resultType"] != resultTypeComplete {
				t.Fatalf("resultType = %v, want %q", result["resultType"], resultTypeComplete)
			}
		})
	}
}

// TestEveryResultCarriesServerInfo covers AC-17.
//
// VALIDATES: every successful result carries
// _meta["io.modelcontextprotocol/serverInfo"] with a name and a version.
// PREVENTS: emitting serverInfo only from server/discover, which is where the
// pre-cutover shape carried it.
func TestEveryResultCarriesServerInfo(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{Commands: taskCapableCommands})
	defer cleanup()

	taskID := createTestTask(t, hs)
	for _, probe := range resultBearingMethods(taskID) {
		t.Run(probe.method, func(t *testing.T) {
			status, parsed := postMCP(t, hs, probe.method, probe.caps, probe.params)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
			}
			result := resultOf(t, parsed)
			meta, ok := result[metaKey].(map[string]any)
			if !ok {
				t.Fatalf("result._meta = %v, want an object", result[metaKey])
			}
			info, ok := meta[metaKeyServerInfo].(map[string]any)
			if !ok {
				t.Fatalf("result._meta[%q] = %v, want an object", metaKeyServerInfo, meta[metaKeyServerInfo])
			}
			if name, _ := info["name"].(string); name == "" {
				t.Error("serverInfo.name empty")
			}
			if version, _ := info["version"].(string); version == "" {
				t.Error("serverInfo.version empty")
			}
		})
	}
}

// TestToolsCallCarriesNoCacheHints is spec-mcp2026-4-caching-apps AC-15's named
// test, and it covers BOTH result shapes tools/call can answer with. The spec's
// TDD plan named this test and nobody wrote it. The test that did exist
// (TestNonCacheableResultsCarryNoHints, caching_test.go) drives only the
// complete shape, because resultBearingMethods calls ze_execute synchronously.
//
// The two shapes have DIFFERENT invariants, and conflating them is the trap:
//
//   - complete: neither ttlMs nor cacheScope. tools/call is absent from the
//     caching page's operation list.
//   - task (CreateTaskResult): ttlMs and pollIntervalMs are REQUIRED -- the
//     io.modelcontextprotocol/tasks extension specifies a Task object
//     "containing a unique taskId, initial status, ttlMs, and pollIntervalMs"
//     -- and cacheScope must be absent. cacheScope is the discriminator: a
//     caching hint is the (ttlMs, cacheScope) pair, and only cacheTTLByMethod
//     can emit it.
//
// VALIDATES: the complete shape carries neither hint. The task shape carries
// ttlMs and pollIntervalMs and no cacheScope.
// PREVENTS: two opposite regressions. The first is an entry for tools/call in
// cacheTTLByMethod, which is a conformance error. The second is a "fix" of the
// task result that deletes its ttlMs, because a comment claimed tools/call
// carries no ttlMs in any shape. That deletion would strip a field the
// extension makes mandatory and leave a polling client with no retention bound.
func TestToolsCallCarriesNoCacheHints(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{Commands: taskCapableCommands})
	defer cleanup()

	t.Run("complete", func(t *testing.T) {
		status, parsed := postMCP(t, hs, methodToolsCall, capsNone,
			`{"name":"ze_execute","arguments":{"command":"show version"}}`)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
		}
		result := resultOf(t, parsed)
		if got := result[resultTypeKey]; got != resultTypeComplete {
			t.Fatalf("resultType = %v, want %q", got, resultTypeComplete)
		}
		for _, key := range []string{resultKeyTTLMs, resultKeyCacheScope} {
			if raw, present := result[key]; present {
				t.Errorf("%s = %v on a complete tools/call result; tools/call is not a cacheable operation", key, raw)
			}
		}
	})

	t.Run("task", func(t *testing.T) {
		status, parsed := postMCP(t, hs, methodToolsCall, capsTasks,
			`{"name":"ze_slow","arguments":{"action":"cmd"}}`)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
		}
		result := resultOf(t, parsed)
		if got := result[resultTypeKey]; got != resultTypeTask {
			t.Fatalf("resultType = %v, want %q (body %v)", got, resultTypeTask, parsed)
		}

		// cacheScope is what must never appear: its presence would mean the
		// caching table stamped this result.
		if raw, present := result[resultKeyCacheScope]; present {
			t.Errorf("%s = %v on a CreateTaskResult; only cacheTTLByMethod may emit a cache scope", resultKeyCacheScope, raw)
		}

		// ttlMs and pollIntervalMs must BOTH be present: they are the extension's
		// retention fields, not caching hints, and a polling client needs them.
		for _, key := range []string{resultKeyTTLMs, resultKeyPollIntervalMs} {
			raw, present := result[key]
			if !present {
				t.Fatalf("%s missing from a CreateTaskResult: %v", key, result)
			}
			value, isNumber := raw.(float64)
			if !isNumber {
				t.Fatalf("%s = %v (%T), want a JSON number", key, raw, raw)
			}
			if value <= 0 {
				t.Errorf("%s = %v, want > 0", key, value)
			}
		}

		// The invariant retentionHints exists to hold, asserted on the wire.
		ttl, _ := result[resultKeyTTLMs].(float64)
		poll, _ := result[resultKeyPollIntervalMs].(float64)
		if poll > ttl/2 {
			t.Errorf("pollIntervalMs %v exceeds half of ttlMs %v: a conforming client could sleep past its result", poll, ttl)
		}
	})
}

// TestTasksGetDoesNotAliasRegistryState guards the hazard that ok()'s own godoc
// names. A task handler hands back the map the registry stored, and a mutation
// of that map would persist envelope fields into registry state.
//
// VALIDATES: two tasks/get calls for the same terminal task return identical
// result payloads, and the map the registry still owns gains neither
// `resultType` nor `_meta`.
// PREVENTS: an in-place stamp of the envelope by ok(). That stamp would leak
// protocol fields into stored tool output, and each further call would stamp
// over the previous one. No per-call assertion can see that defect, because
// every individual response would still look correct.
//
// The hazard survived the removal of tasks/result. tasks/get now carries the
// stored map as its `result` member, so the aliasing question is the same one.
func TestTasksGetDoesNotAliasRegistryState(t *testing.T) {
	srv, err := NewStreamable(StreamableConfig{
		Commands: taskCapableCommands,
		Dispatch: func(_ context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, plugin.Map{"result": "ok", "message": cmd}), nil
		},
	})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer srv.Close()
	hs := httptest.NewServer(srv)
	defer hs.Close()

	taskID := createTestTask(t, hs)
	params := `{"taskId":"` + taskID + `"}`

	first := fetchTaskResult(t, hs, params)
	second := fetchTaskResult(t, hs, params)

	// Both calls answer identically: the second did not read back a result the
	// first had polluted.
	if fmt.Sprint(first["content"]) != fmt.Sprint(second["content"]) {
		t.Fatalf("tasks/get result content differs between calls:\n first  = %v\n second = %v",
			first["content"], second["content"])
	}

	// And the registry's own copy carries no envelope field at all. Auth mode
	// is none, so the authenticated principal is the empty identity.
	info, err := srv.tasks.Get("", taskID)
	if err != nil {
		t.Fatalf("registry Get: %v", err)
	}
	stored := info.Result
	if stored == nil {
		t.Fatalf("registry retained no result for terminal task %s", taskID)
	}
	for _, envelope := range []string{"resultType", metaKey} {
		if _, leaked := stored[envelope]; leaked {
			t.Errorf("registry-stored result gained the envelope field %q: %v", envelope, stored)
		}
	}
	if stored["content"] == nil {
		t.Errorf("registry-stored result lost its content: %v", stored)
	}
}

// fetchTaskResult posts one tasks/get and returns the tool-result object it
// carries for a terminal task.
func fetchTaskResult(t *testing.T, hs *httptest.Server, params string) map[string]any {
	t.Helper()
	status, parsed := postMCP(t, hs, methodTasksGet, capsTasks, params)
	if status != http.StatusOK {
		t.Fatalf("tasks/get: status = %d (body %v)", status, parsed)
	}
	result := resultOf(t, parsed)
	if result["resultType"] != resultTypeComplete {
		t.Fatalf("tasks/get resultType = %v, want %q", result["resultType"], resultTypeComplete)
	}
	if _, ok := result[metaKey].(map[string]any); !ok {
		t.Fatalf("tasks/get carries no %s object: %v", metaKey, result)
	}
	// AC-3: a terminal task carries its tool output on tasks/get. This is the
	// payload that used to require a second, blocking tasks/result call.
	inner, ok := result["result"].(map[string]any)
	if !ok {
		t.Fatalf("terminal task carries no result object on tasks/get: %v", result)
	}
	return inner
}

// TestNoSessionIsMintedRequiredOrEchoed covers AC-1 and AC-12.
//
// VALIDATES: a conformant request is served with no handshake, no response
// carries an Mcp-Session-Id header, and an inbound Mcp-Session-Id or
// Last-Event-ID is ignored rather than honored or echoed.
// PREVENTS: any credential-equivalent identifier surviving the cutover.
func TestNoSessionIsMintedRequiredOrEchoed(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	body, params := buildMCPBody(t, 1, methodToolsList, capsNone, "")
	headers := mcpHeaders(methodToolsList, params)
	// A stale client threading the removed headers must be served anyway.
	headers["Mcp-Session-Id"] = "stale-session-from-an-older-client"
	headers["Last-Event-ID"] = "42"

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, hs.URL+Endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer closeBody(t, resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Mcp-Session-Id"); got != "" {
		t.Fatalf("response minted Mcp-Session-Id = %q, want none", got)
	}
	if strings.Contains(corsExposeHeaders, "Mcp-Session-Id") {
		t.Error("CORS expose list still advertises Mcp-Session-Id")
	}
	if strings.Contains(corsAllowHeaders, "Mcp-Session-Id") {
		t.Error("CORS allow list still advertises Mcp-Session-Id")
	}
}

// TestGETAndDELETEMethodNotAllowed covers AC-4.
//
// VALIDATES: the GET stream endpoint and the DELETE session-termination call of
// earlier revisions both answer 405 with an Allow header naming only what the
// endpoint still supports.
// PREVENTS: leaving either endpoint reachable after the mechanism behind it is
// gone.
func TestGETAndDELETEMethodNotAllowed(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	for _, method := range []string{http.MethodGet, http.MethodDelete, http.MethodPut, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), method, hs.URL+Endpoint, http.NoBody)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			// An older client would send both of these on its GET.
			req.Header.Set("Accept", "text/event-stream")
			req.Header.Set("Mcp-Session-Id", "whatever")
			resp, err := hs.Client().Do(req)
			if err != nil {
				t.Fatalf("%s: %v", method, err)
			}
			defer closeBody(t, resp.Body)
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", resp.StatusCode)
			}
			allow := resp.Header.Get("Allow")
			if !strings.Contains(allow, "POST") {
				t.Fatalf("Allow = %q, want it to name POST", allow)
			}
			if strings.Contains(allow, "GET") || strings.Contains(allow, "DELETE") {
				t.Fatalf("Allow = %q still advertises a removed method", allow)
			}
			if got := resp.Header.Get("Mcp-Session-Id"); got != "" {
				t.Fatalf("405 echoed Mcp-Session-Id = %q", got)
			}
		})
	}
}

// TestUnknownMethodReturns404 covers AC-10.
//
// VALIDATES: an unknown JSON-RPC method is answered with HTTP 404 AND a -32601
// JSON-RPC error body.
// PREVENTS: a 200-with-error body, which a dual-era client cannot tell from a
// modern server, and a bare 404, which it cannot tell from a legacy HTTP+SSE
// server that does not host this endpoint.
func TestUnknownMethodReturns404(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	status, parsed := postMCP(t, hs, "does/not/exist", capsNone, "")
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %v)", status, parsed)
	}
	rpcErr := rpcErrorOf(t, parsed)
	if rpcErr["code"] != float64(rpcMethodNotFound) {
		t.Fatalf("code = %v, want %d", rpcErr["code"], rpcMethodNotFound)
	}
	if msg, _ := rpcErr["message"].(string); !strings.Contains(msg, "does/not/exist") {
		t.Fatalf("message %q does not name the method", msg)
	}
}

// TestAuthRunsOnEveryRequest covers AC-11.
//
// VALIDATES: under auth-mode bearer EVERY request presents its own credential --
// a second, third and fourth credential-less call is rejected exactly like the
// first, on every method.
// PREVENTS: the pre-cutover shape returning, where identity was bound once and
// later requests were trusted by an identifier's validity alone.
func TestAuthRunsOnEveryRequest(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{Token: "secret"})
	defer cleanup()

	// A valid credential first, so any state that a regression can bind is bound.
	if status, parsed := postMCPAuth(t, hs, "secret", methodToolsList, capsNone, ""); status != http.StatusOK {
		t.Fatalf("authenticated tools/list: status = %d (body %v)", status, parsed)
	}

	// tasks/get carries params because it needs a taskId. The others take none.
	// tasks/list used to fill this row and needed no params, but MCP 2026-07-28
	// removed it. A tasks/* method has to stay in this table. Authentication on
	// the task surface is exactly what the deleted session layer used to be
	// trusted for.
	probes := []struct{ method, params string }{
		{methodToolsList, ""},
		{methodServerDiscover, ""},
		{methodTasksGet, `{"taskId":"never-minted"}`},
		{methodResourcesList, ""},
	}
	for _, probe := range probes {
		t.Run(probe.method, func(t *testing.T) {
			// Repeat so a "first request only" auth policy would show up.
			for attempt := range 3 {
				status, _ := postMCP(t, hs, probe.method, capsTasks, probe.params)
				if status != http.StatusUnauthorized {
					t.Fatalf("attempt %d: status = %d, want 401", attempt, status)
				}
			}
			// And a valid credential still passes AUTH in between. The assertion
			// is "not 401" rather than "200", because tasks/get with an id that
			// was never minted is legitimately refused at the params layer. That
			// refusal is itself proof that the request reached dispatch.
			status, parsed := postMCPAuth(t, hs, "secret", probe.method, capsTasks, probe.params)
			if status == http.StatusUnauthorized {
				t.Fatalf("authenticated %s: status = 401, want the credential to be accepted (body %v)",
					probe.method, parsed)
			}
		})
	}
}

// TestProviderModeAuthenticatesLikeEveryOtherPath guards D-2.
//
// VALIDATES: with a Provider set AND bearer auth configured, a credential-less
// request is 401 -- Provider mode is not an auth bypass.
// PREVENTS: a return of the deleted Provider-mode short-circuit, which was the
// one code shape that reached an unauthenticated path.
func TestProviderModeAuthenticatesLikeEveryOtherPath(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{
		Provider: fakeProvider{},
		Token:    "secret",
	})
	defer cleanup()

	for _, method := range []string{methodToolsList, methodServerDiscover} {
		t.Run(method, func(t *testing.T) {
			if status, _ := postMCP(t, hs, method, capsNone, ""); status != http.StatusUnauthorized {
				t.Fatalf("credential-less %s: status = %d, want 401", method, status)
			}
			if status, _ := postMCPAuth(t, hs, "secret", method, capsNone, ""); status != http.StatusOK {
				t.Fatalf("authenticated %s: status = %d, want 200", method, status)
			}
		})
	}
}

// TestProviderModeUnauthenticatedByConfigStillServes is the other half of D-2.
// ze-chaos sets Provider with no Token and no AuthMode. Auth-mode inference
// therefore selects none, and every request succeeds with a zero Identity.
//
// VALIDATES: running ze-chaos through the uniform per-request auth path is
// observably identical to the deleted bypass.
// PREVENTS: the cutover breaking ze-chaos by turning "unauthenticated by
// configuration" into a rejection.
func TestProviderModeUnauthenticatedByConfigStillServes(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{Provider: fakeProvider{}})
	defer cleanup()

	status, parsed := postMCP(t, hs, methodToolsList, capsNone, "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
	}
	result := resultOf(t, parsed)
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("tools = %v, want the provider's tools", result["tools"])
	}
}

// TestBodyLimitBoundary covers the request-body boundary row.
//
// VALIDATES: exactly maxRequestBody bytes is served, one byte more is 413, and
// an empty body is a distinct failure (-32700 parse error), not a size
// rejection.
// PREVENTS: an off-by-one at the only per-request size bound the transport has.
func TestBodyLimitBoundary(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	// Pad a conformant tools/list to an exact byte length with an ignored
	// params member, so the JSON stays valid at both boundary sizes.
	build := func(total int) string {
		prefix := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":` +
			metaBlock(ProtocolVersion, capsNone) + `,"pad":"`
		suffix := `"}}`
		padLen := total - len(prefix) - len(suffix)
		if padLen < 0 {
			t.Fatalf("target %d shorter than the minimum body", total)
		}
		return prefix + strings.Repeat("x", padLen) + suffix
	}
	headers := map[string]string{
		"MCP-Protocol-Version": ProtocolVersion,
		"Mcp-Method":           "tools/list",
	}

	t.Run("exactly at the cap is served", func(t *testing.T) {
		body := build(maxRequestBody)
		if len(body) != maxRequestBody {
			t.Fatalf("body length = %d, want %d", len(body), maxRequestBody)
		}
		status, parsed := postRaw(t, hs, body, headers)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
		}
	})

	t.Run("one byte over the cap is 413", func(t *testing.T) {
		body := build(maxRequestBody + 1)
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, hs.URL+Endpoint, strings.NewReader(body))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := hs.Client().Do(req)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer closeBody(t, resp.Body)
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413", resp.StatusCode)
		}
	})

	t.Run("empty body is a parse error not a size rejection", func(t *testing.T) {
		status, parsed := postRaw(t, hs, "", headers)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200 with a JSON-RPC error body", status)
		}
		rpcErr := rpcErrorOf(t, parsed)
		if rpcErr["code"] != float64(rpcParseError) {
			t.Fatalf("code = %v, want %d", rpcErr["code"], rpcParseError)
		}
	})
}

// VALIDATES: a JSON-RPC notification (no id) is acknowledged with 202 and an
// empty body.
// PREVENTS: answering a notification with a JSON-RPC response, which the
// transport forbids.
func TestNotificationAcknowledgedWith202(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	body, params := buildMCPBody(t, nil, "notifications/anything", capsNone, "")
	headers := mcpHeaders("notifications/anything", params)
	status, parsed := postRaw(t, hs, body, headers)
	if status != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (body %v)", status, parsed)
	}
	if parsed != nil {
		t.Fatalf("202 carried a body: %v", parsed)
	}
}

func TestStreamableOriginRejection(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{
		AllowedOrigins: []string{"https://friend.example.com"},
	})
	defer cleanup()

	if status := postOrigin(t, hs, "https://evil.example.com"); status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", status)
	}
}

func TestStreamableOriginAllowListAccepts(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{
		AllowedOrigins: []string{"https://friend.example.com"},
	})
	defer cleanup()

	if status := postOrigin(t, hs, "https://friend.example.com"); status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
}

func TestStreamableBearerAuthRejectsMissingToken(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{Token: "secret"})
	defer cleanup()

	if status, _ := postMCP(t, hs, methodToolsList, capsNone, ""); status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", status)
	}
}

func TestStreamableBearerAuthAcceptsValidToken(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{Token: "secret"})
	defer cleanup()

	if status, _ := postMCPAuth(t, hs, "secret", methodToolsList, capsNone, ""); status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
}

func TestStreamableBearerAuthFailureAuditRecord(t *testing.T) {
	recorder, err := audit.NewMemory(100)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	srv, err := NewStreamable(StreamableConfig{Token: "secret", AuditRecorder: recorder})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer srv.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":` +
		metaBlock(ProtocolVersion, capsNone) + `}}`
	req := httptest.NewRequest(http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer alice:wrong")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	req.Header.Set("Mcp-Method", "tools/list")
	req.RemoteAddr = "192.0.2.10:4444"
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	entries := recorder.Query(audit.Filter{Action: audit.ActionAuthFail})
	if len(entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(entries))
	}
	if entries[0].Actor != "alice" {
		t.Fatalf("actor = %q, want alice", entries[0].Actor)
	}
	if entries[0].RemoteAddr != "192.0.2.10:4444" {
		t.Fatalf("remote addr = %q, want 192.0.2.10:4444", entries[0].RemoteAddr)
	}
	if entries[0].Surface != audit.MCP {
		t.Fatalf("surface = %q, want %q", entries[0].Surface, audit.MCP)
	}
	if entries[0].Outcome != audit.OutcomeDenied {
		t.Fatalf("outcome = %q, want %q", entries[0].Outcome, audit.OutcomeDenied)
	}
}

func TestStreamableCanonicalOrigin(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		want      string
		expectErr bool
	}{
		{"plain https", "https://foo.com", "https://foo.com", false},
		{"https with default port", "https://foo.com:443", "https://foo.com", false},
		{"https with explicit port", "https://foo.com:8443", "https://foo.com:8443", false},
		{"http with default port", "http://foo.com:80", "http://foo.com", false},
		{"trailing slash dropped", "https://foo.com/", "https://foo.com", false},
		{"path dropped", "https://foo.com/some/path", "https://foo.com", false},
		{"uppercase scheme lowercased", "HTTPS://FOO.COM", "https://foo.com", false},
		{"null literal", "null", "null", false},
		{"NULL case-insensitive", "NULL", "null", false},
		{"missing scheme", "foo.com", "", true},
		{"empty", "", "", true},
		{"IPv6 with brackets default port", "http://[::1]:80", "http://[::1]", false},
		{"IPv6 with brackets explicit port", "http://[::1]:8080", "http://[::1]:8080", false},
		{"IPv6 loopback uppercase host", "https://[::1]", "https://[::1]", false},
		{"user-info stripped", "http://user:pass@foo.com", "http://foo.com", false},
		{"fragment stripped", "https://foo.com/path#section", "https://foo.com", false},
		{"query stripped", "https://foo.com/?q=1", "https://foo.com", false},
		{"non-numeric port rejected", "http://foo.com:abc", "", true},
		{"zero port rejected", "http://foo.com:0", "", true},
		{"too-large port rejected", "http://foo.com:99999", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := canonicalOrigin(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("canonicalOrigin(%q) error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("canonicalOrigin(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestStreamableOriginCanonicalisedBothSides(t *testing.T) {
	// Allowlist entry with default port; request with explicit default port.
	// Both canonicalize to the same key and the request is accepted.
	hs, cleanup := newTestStreamable(t, StreamableConfig{
		AllowedOrigins: []string{"https://friend.example.com:443"},
	})
	defer cleanup()

	if status := postOrigin(t, hs, "https://friend.example.com/"); status != http.StatusOK {
		t.Fatalf("canonicalised origin match: status = %d, want 200", status)
	}
}

func TestStreamableLoopbackRejectedWhenAllowListSet(t *testing.T) {
	// Allowlist is non-empty. Loopback origin NOT in allowlist must be rejected —
	// once the operator has enumerated friends, localhost is no longer free.
	hs, cleanup := newTestStreamable(t, StreamableConfig{
		AllowedOrigins: []string{"https://friend.example.com"},
	})
	defer cleanup()

	if status := postOrigin(t, hs, "http://localhost:3000"); status != http.StatusForbidden {
		t.Fatalf("loopback with explicit allowlist: status = %d, want 403", status)
	}
}

func TestStreamableLoopbackOriginAcceptedWhenAllowListEmpty(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	cases := []string{
		"http://localhost",
		"http://localhost:3000",
		"https://127.0.0.1:8080",
		"http://[::1]",
		"http://[::1]:3000",
		"https://[::1]:8443",
		"null",
	}
	for _, origin := range cases {
		t.Run(origin, func(t *testing.T) {
			if status := postOrigin(t, hs, origin); status != http.StatusOK {
				t.Fatalf("loopback default-allowlist %q: status = %d, want 200", origin, status)
			}
		})
	}
}

func TestStreamableNewStreamableRejectsBadOrigin(t *testing.T) {
	_, err := NewStreamable(StreamableConfig{
		Dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, nil), nil
		},
		AllowedOrigins: []string{"not a url"},
	})
	if err == nil {
		t.Fatal("NewStreamable accepted malformed origin; want error")
	}
}

func TestStreamableIDNOriginEndToEnd(t *testing.T) {
	// Integration counterpart to TestStreamableIDNOriginMatch: configure the
	// allowlist with the unicode form, send Origin in punycode form, expect
	// the request to be accepted.
	hs, cleanup := newTestStreamable(t, StreamableConfig{
		AllowedOrigins: []string{"https://münchen.example.com"},
	})
	defer cleanup()

	if status := postOrigin(t, hs, "https://xn--mnchen-3ya.example.com"); status != http.StatusOK {
		t.Fatalf("punycode origin against unicode allowlist: status = %d, want 200", status)
	}
}

func TestStreamableIDNOriginMatch(t *testing.T) {
	// Regression for pass-4 finding 3: an allowlist entry in Unicode form
	// must match an incoming Origin in punycode (and vice versa), both
	// canonicalizing via idna.Lookup.ToASCII.
	got, err := canonicalOrigin("https://münchen.example.com")
	if err != nil {
		t.Fatalf("canonicalOrigin unicode: %v", err)
	}
	got2, err := canonicalOrigin("https://xn--mnchen-3ya.example.com")
	if err != nil {
		t.Fatalf("canonicalOrigin punycode: %v", err)
	}
	if got != got2 {
		t.Fatalf("IDN mismatch: unicode=%q punycode=%q", got, got2)
	}
	if !strings.HasPrefix(got, "https://xn--mnchen-3ya") {
		t.Fatalf("expected ASCII-compatible form, got %q", got)
	}
}

func TestStreamableIsLoopbackOrigin(t *testing.T) {
	cases := []struct {
		origin string
		want   bool
	}{
		{"http://localhost", true},
		{"https://localhost:8080", true},
		{"http://127.0.0.1", true},
		{"http://127.0.0.1:9090", true},
		{"http://[::1]", true},
		{"http://[::1]:8080", true},
		{"https://[::1]", true},
		{"null", true},
		{"http://example.com", false},
		{"https://192.168.1.1", false},
		{"https://127.0.0.1.evil.com", false},
		{"http://[::2]", false},
	}
	for _, tc := range cases {
		t.Run(tc.origin, func(t *testing.T) {
			if got := isLoopbackOrigin(tc.origin); got != tc.want {
				t.Fatalf("isLoopbackOrigin(%q) = %v, want %v", tc.origin, got, tc.want)
			}
		})
	}
}

func TestStreamableToolsList(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	status, parsed := postMCP(t, hs, methodToolsList, capsNone, "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	result := resultOf(t, parsed)
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("tools list empty: %v", result)
	}
}

// VALIDATES: AC-12 -- a command annotated `ze:task-support forbidden`, called by
// a client that DID declare the tasks extension, is never returned as a task
// handle and runs synchronously.
// PREVENTS: R-1 -- the server-directed inversion auto-tasking a mutating
// command (the four rib commands are the real annotated set).
//
// Both halves are asserted. "No task handle" alone would pass against a server
// that failed the request, which is the failure this rule exists to stop.
func TestStreamable_ForbiddenNeverTasked(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{
		Commands: func() []CommandInfo {
			return []CommandInfo{
				{Name: "fast cmd", Help: "Quick", TaskSupport: TaskSupportForbidden},
			}
		},
	})
	defer cleanup()

	status, parsed := postMCP(t, hs, methodToolsCall, capsTasks,
		`{"name":"ze_fast","arguments":{"action":"cmd"}}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200: a forbidden command must still RUN (body %v)", status, parsed)
	}
	res := resultOf(t, parsed)
	if got := res[resultTypeKey]; got != resultTypeComplete {
		t.Errorf("resultType = %v, want %q: a forbidden command must never be tasked", got, resultTypeComplete)
	}
	if id, _ := res["taskId"].(string); id != "" {
		t.Errorf("forbidden command returned a task handle %q", id)
	}
}

// VALIDATES: AC-1 -- a `ze:task-support required` command called by a client
// that declared the extension returns resultType "task" with taskId, status,
// ttlMs and pollIntervalMs -- with NO `task` member in the request params.
// PREVENTS: regressing to the client-directed opt-in D-1 removed.
func TestStreamable_RequiredIsTaskedServerDirected(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{
		Commands: func() []CommandInfo {
			return []CommandInfo{
				{Name: "slow cmd", Help: "Long", TaskSupport: TaskSupportRequired},
			}
		},
	})
	defer cleanup()

	status, parsed := postMCP(t, hs, methodToolsCall, capsTasks,
		`{"name":"ze_slow","arguments":{"action":"cmd"}}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
	}
	res := resultOf(t, parsed)
	if got := res[resultTypeKey]; got != resultTypeTask {
		t.Fatalf("resultType = %v, want %q (server did not create a task unasked)", got, resultTypeTask)
	}
	if id, _ := res[resultKeyTaskID].(string); id == "" {
		t.Errorf("CreateTaskResult carries no %s: %v", resultKeyTaskID, res)
	}
	if res[resultKeyStatus] != taskStateWireWorking {
		t.Errorf("status = %v, want %q", res[resultKeyStatus], taskStateWireWorking)
	}
	ttlMs, _ := res[resultKeyTTLMs].(float64)
	if ttlMs <= 0 {
		t.Errorf("%s = %v, want a positive retention window", resultKeyTTLMs, res[resultKeyTTLMs])
	}
	pollMs, _ := res[resultKeyPollIntervalMs].(float64)
	if pollMs <= 0 {
		t.Errorf("%s = %v, want a positive poll hint", resultKeyPollIntervalMs, res[resultKeyPollIntervalMs])
	}
	// D-6: a client obeying the hint must poll at least twice inside the
	// retention window, or it can sleep straight past a terminal result.
	if pollMs > ttlMs/2 {
		t.Errorf("%s = %v exceeds half of %s = %v: a conforming client could miss the result",
			resultKeyPollIntervalMs, pollMs, resultKeyTTLMs, ttlMs)
	}
}

// VALIDATES: AC-6, both halves -- a client that did NOT declare the tasks
// extension never receives a task handle for a `required` command, AND the
// command still runs, answering with the ordinary synchronous result.
// PREVENTS: R-2 (a task handle reaching a client that cannot read one) and the
// opposite over-correction D-2 rejects (refusing the call outright, which would
// make 9 commands unreachable to any client that has not adopted an optional
// extension).
func TestTaskNotReturnedWithoutExtension(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{
		Commands: func() []CommandInfo {
			return []CommandInfo{
				{Name: "slow cmd", Help: "Long", TaskSupport: TaskSupportRequired},
			}
		},
	})
	defer cleanup()

	status, parsed := postMCP(t, hs, methodToolsCall, capsNone,
		`{"name":"ze_slow","arguments":{"action":"cmd"}}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200: the command must still run for a client without the extension (body %v)",
			status, parsed)
	}
	res := resultOf(t, parsed)
	if got := res[resultTypeKey]; got != resultTypeComplete {
		t.Errorf("resultType = %v, want %q", got, resultTypeComplete)
	}
	if id, _ := res[resultKeyTaskID].(string); id != "" {
		t.Errorf("client without the tasks extension was handed task handle %q", id)
	}
}

// VALIDATES: a tasks/* method from a client that did NOT declare the tasks
// extension is refused with -32021 and HTTP 400.
// PREVENTS: serving the extension's own methods to a client that never
// declared it.
func TestStreamable_TasksMethodWithoutCapability(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{
		Commands: func() []CommandInfo {
			return []CommandInfo{{Name: "demo cmd", Help: "Test"}}
		},
	})
	defer cleanup()

	for _, method := range []string{methodTasksGet, methodTasksUpdate, methodTasksCancel} {
		t.Run(method, func(t *testing.T) {
			status, parsed := postMCP(t, hs, method, capsNone, `{"taskId":"whatever"}`)
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %v)", status, parsed)
			}
			rpcErr := rpcErrorOf(t, parsed)
			if rpcErr["code"] != float64(rpcMissingRequiredClientCapability) {
				t.Fatalf("code = %v, want %d", rpcErr["code"], rpcMissingRequiredClientCapability)
			}
		})
	}
}

// VALIDATES: AC-5 -- tasks/list and tasks/result are unknown methods, answered
// HTTP 404 with -32601.
// PREVENTS: leaving either handler wired after MCP 2026-07-28 changelog Major
// change 6 removed both.
func TestRemovedTaskMethods(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{
		Commands: func() []CommandInfo {
			return []CommandInfo{{Name: "demo cmd", Help: "Test"}}
		},
	})
	defer cleanup()

	for _, method := range []string{"tasks/list", "tasks/result"} {
		t.Run(method, func(t *testing.T) {
			// Declaring the capability is the point: these must be gone for a
			// client that COULD have used them, not merely gated behind a
			// capability check.
			status, parsed := postMCP(t, hs, method, capsTasks, `{"taskId":"whatever"}`)
			if status != http.StatusNotFound {
				t.Fatalf("status = %d, want 404 (body %v)", status, parsed)
			}
			rpcErr := rpcErrorOf(t, parsed)
			if rpcErr["code"] != float64(rpcMethodNotFound) {
				t.Fatalf("code = %v, want %d (method not found)", rpcErr["code"], rpcMethodNotFound)
			}
		})
	}
}

// VALIDATES: the CORS preflight advertises only the methods and headers that
// still exist in this revision.
// PREVENTS: telling a browser client it may send a GET or a session header.
func TestStreamableEndpointPreflight(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodOptions, hs.URL+Endpoint, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Origin", "http://localhost:3000")
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	methods := resp.Header.Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "POST") {
		t.Fatalf("Allow-Methods = %q, want it to name POST", methods)
	}
	if strings.Contains(methods, "GET") || strings.Contains(methods, "DELETE") {
		t.Fatalf("Allow-Methods = %q still advertises a removed method", methods)
	}
	allowHeaders := resp.Header.Get("Access-Control-Allow-Headers")
	for _, want := range []string{"MCP-Protocol-Version", "Mcp-Method", "Mcp-Name"} {
		if !strings.Contains(allowHeaders, want) {
			t.Errorf("Allow-Headers = %q, missing %q", allowHeaders, want)
		}
	}
	if strings.Contains(allowHeaders, "Mcp-Session-Id") {
		t.Errorf("Allow-Headers = %q still advertises Mcp-Session-Id", allowHeaders)
	}
}

// VALIDATES: NewStreamable leaves MaxBodyBytes at the 1 MB default when the
// caller sets none, and honors an explicit value.
// PREVENTS: the surviving per-request size bound being deleted alongside the
// session fields it sat next to.
func TestStreamableMaxBodyBytesDefault(t *testing.T) {
	srv, err := NewStreamable(StreamableConfig{})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer srv.Close()
	if srv.maxBody != maxRequestBody {
		t.Fatalf("maxBody = %d, want %d", srv.maxBody, maxRequestBody)
	}

	custom, err := NewStreamable(StreamableConfig{MaxBodyBytes: 4096})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer custom.Close()
	if custom.maxBody != 4096 {
		t.Fatalf("maxBody = %d, want 4096", custom.maxBody)
	}
}

// VALIDATES: httpStatusForDispatch pins the two codes whose HTTP status the
// binding mandates, and leaves every other outcome on 200.
// PREVENTS: a handler choosing a code whose mandated status is silently dropped.
func TestHTTPStatusForDispatch(t *testing.T) {
	cases := []struct {
		name string
		resp *response
		want int
	}{
		{"nil response", nil, http.StatusOK},
		{"success", &response{}, http.StatusOK},
		{"method not found", &response{Error: &rpcError{Code: rpcMethodNotFound}}, http.StatusNotFound},
		{"missing capability", &response{Error: &rpcError{Code: rpcMissingRequiredClientCapability}}, http.StatusBadRequest},
		{"invalid params stays 200", &response{Error: &rpcError{Code: rpcInvalidParams}}, http.StatusOK},
		{"internal error stays 200", &response{Error: &rpcError{Code: rpcInternalError}}, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := httpStatusForDispatch(tc.resp); got != tc.want {
				t.Fatalf("httpStatusForDispatch = %d, want %d", got, tc.want)
			}
		})
	}
}

// VALIDATES: an unsupported content type is still refused with 415.
// PREVENTS: losing the guard while rewriting the POST pipeline around it.
func TestStreamableUnsupportedContentType(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	body, _ := buildMCPBody(t, 1, methodToolsList, capsNone, "")
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, hs.URL+Endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "text/plain")
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", resp.StatusCode)
	}
}

// VALIDATES: a POST to a path other than the MCP endpoint is a 404 and carries
// CORS headers so a browser client can read the description.
// PREVENTS: regressing the wrong-path branch while reshaping ServeHTTP.
func TestStreamableWrongPathIs404(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, hs.URL+"/not-mcp", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Origin", "http://localhost:3000")
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got == "" {
		t.Error("404 on wrong path carries no CORS origin header")
	}
}

// TestTasksUpdateAcknowledgesAndIgnores covers AC-7 and AC-8.
//
// VALIDATES: tasks/update on a task the caller owns is acknowledged with an
// empty result and leaves the task's state unchanged, and inputResponses
// carrying unknown keys are ignored rather than rejected.
// PREVENTS: advertising the tasks extension while refusing one of its methods,
// and the opposite error of letting attacker-controlled inputResponses reach
// anything. Ze raises no inputRequests (D-4), so every key is unknown by
// construction and the extension's own tolerance rule is the whole contract.
func TestTasksUpdateAcknowledgesAndIgnores(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{Commands: taskCapableCommands})
	defer cleanup()

	taskID := createTestTask(t, hs)
	before := taskStatus(t, hs, taskID)

	cases := []struct {
		name   string
		params string
	}{
		{"no inputResponses at all", `{"taskId":"` + taskID + `"}`},
		{"empty inputResponses", `{"taskId":"` + taskID + `","inputResponses":{}}`},
		{"unknown key", `{"taskId":"` + taskID + `","inputResponses":{"no_such_request":{"content":{"x":1}}}}`},
		{"several unknown keys", `{"taskId":"` + taskID + `","inputResponses":{"a":{},"b":{"deeply":{"nested":[1,2,3]}}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, parsed := postMCP(t, hs, methodTasksUpdate, capsTasks, tc.params)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
			}
			result := resultOf(t, parsed)
			if result[resultTypeKey] != resultTypeComplete {
				t.Errorf("resultType = %v, want %q", result[resultTypeKey], resultTypeComplete)
			}
			// The acknowledgement is EMPTY: only the two envelope fields every
			// result carries. Anything else would mean the server acted on
			// responses it has nothing to match against.
			for key := range result {
				if key != resultTypeKey && key != metaKey {
					t.Errorf("acknowledgement carries unexpected key %q: %v", key, result)
				}
			}
			// AC-7: the task's state is unchanged.
			if got := taskStatus(t, hs, taskID); got != before {
				t.Errorf("task status changed from %q to %q across tasks/update", before, got)
			}
		})
	}
}

// TestTasksUpdateRejectsForeignTask covers the ownership half of AC-8.
//
// VALIDATES: a malformed or foreign taskId is still rejected, even though the
// inputResponses payload is ignored.
// PREVENTS: reading "ignore unknown keys" as "ignore the taskId too". The id is
// the one thing this handler acts on, and it is ownership-checked first.
func TestTasksUpdateRejectsForeignTask(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{Commands: taskCapableCommands})
	defer cleanup()

	for _, tc := range []struct{ name, params string }{
		{"unknown id", `{"taskId":"never-minted","inputResponses":{"a":{}}}`},
		{"empty id", `{"taskId":""}`},
		{"absent id", `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, parsed := postMCP(t, hs, methodTasksUpdate, capsTasks, tc.params)
			// rpcErrorOf fails the test when the response carries no error
			// object, so reaching past it IS the assertion that the call was
			// refused.
			rpcErr := rpcErrorOf(t, parsed)
			if rpcErr["code"] == nil {
				t.Fatalf("tasks/update %s: error object carries no code: %v", tc.name, rpcErr)
			}
		})
	}
}

// taskStatus reads one task's current status over the transport.
func taskStatus(t *testing.T, hs *httptest.Server, taskID string) string {
	t.Helper()
	status, parsed := postMCP(t, hs, methodTasksGet, capsTasks, `{"taskId":"`+taskID+`"}`)
	if status != http.StatusOK {
		t.Fatalf("tasks/get: status = %d (body %v)", status, parsed)
	}
	got, _ := resultOf(t, parsed)["status"].(string)
	return got
}
