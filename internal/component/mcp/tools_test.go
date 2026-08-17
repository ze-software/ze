package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

// unwindFailureBound is how long TestDispatchRunsUnderTheRequestContext waits
// before it declares that a canceled context never reached the dispatcher. It
// is a failure detector, not a timing assumption. A correct build answers the
// instant cancel() lands, so only a broken build consumes the bound.
const unwindFailureBound = 10 * time.Second

// TestToolProviderInterface and
// TestLegacyHandlerBearerAuthFailureAuditRecord removed with the legacy
// Handler/HandlerWithAudit/ZeProvider deletion (spec-followup-subsystem AC-9).
// ToolProvider coverage now lives in provider_test.go against the Streamable
// transport (TestStreamableProviderServesSessionless and friends); the
// Streamable bearer-auth failure audit record is asserted by
// TestStreamableBearerAuthFailureAuditRecord in streamable_test.go.

func TestGroupCommands(t *testing.T) {
	commands := []CommandInfo{
		{Name: "show bgp rib status", Help: "RIB summary"},
		{Name: "show bgp rib", Help: "Show routes"},
		{Name: "show bgp rib best status", Help: "Best-path status"},
		{Name: "show bgp peer list", Help: "List peers"},
		{Name: "show config dump", Help: "Dump config"},
		{Name: "show config diff", Help: "Diff configs"},
		{Name: "show config validate", Help: "Validate config"},
		{Name: "show schema list", Help: "List schemas"},
		{Name: "show schema methods", Help: "List methods"},
		{Name: "show version", Help: "Show version"},
		{Name: "metrics values", Help: "Metric values"},
		{Name: "metrics list", Help: "List metrics"},
		{Name: "log levels", Help: "Log levels"},
		{Name: "log set", Help: "Set log level"},
		{Name: "cache list", Help: "List cache"},
	}

	groups := groupCommands(commands)

	// Build lookup for easy assertions.
	byPrefix := make(map[string]toolGroup)
	for _, g := range groups {
		byPrefix[g.prefix] = g
	}

	// "show" has multiple depth-2 subgroups -> depth-2 grouping.
	if g, ok := byPrefix["show config"]; !ok {
		t.Fatal("expected 'show config' group")
	} else if len(g.actions) != 3 {
		t.Fatalf("show config: expected 3 actions, got %d", len(g.actions))
	}

	if g, ok := byPrefix["show schema"]; !ok {
		t.Fatal("expected 'show schema' group")
	} else if len(g.actions) != 2 {
		t.Fatalf("show schema: expected 2 actions, got %d", len(g.actions))
	}

	// "show version" is standalone under "show" (depth-1 leftover).
	if _, ok := byPrefix["show version"]; !ok {
		t.Fatal("expected 'show version' group")
	}

	// "show bgp" groups BGP show actions under verb-first grammar.
	if g, ok := byPrefix["show bgp"]; !ok {
		t.Fatal("expected 'show bgp' group")
	} else if len(g.actions) != 4 {
		t.Fatalf("show bgp: expected 4 actions, got %d", len(g.actions))
	}

	// "metrics" at depth 1.
	if g, ok := byPrefix["metrics"]; !ok {
		t.Fatal("expected 'metrics' group")
	} else if len(g.actions) != 2 {
		t.Fatalf("metrics: expected 2 actions, got %d", len(g.actions))
	}

	// "log" at depth 1.
	if g, ok := byPrefix["log"]; !ok {
		t.Fatal("expected 'log' group")
	} else if len(g.actions) != 2 {
		t.Fatalf("log: expected 2 actions, got %d", len(g.actions))
	}

	// "cache" at depth 1 with only 1 action.
	if g, ok := byPrefix["cache"]; !ok {
		t.Fatal("expected 'cache' group")
	} else if len(g.actions) != 1 {
		t.Fatalf("cache: expected 1 action, got %d", len(g.actions))
	}
}

func TestToolName(t *testing.T) {
	tests := []struct {
		prefix string
		want   string
	}{
		{"show bgp", "ze_show_bgp"},
		{"show config", "ze_show_config"},
		{"show schema", "ze_show_schema"},
		{"metrics", "ze_metrics"},
	}
	for _, tt := range tests {
		got := toolName(tt.prefix)
		if got != tt.want {
			t.Errorf("toolName(%q) = %q, want %q", tt.prefix, got, tt.want)
		}
	}
}

func TestGenerateToolsSkipsHandcrafted(t *testing.T) {
	groups := []toolGroup{
		{prefix: "show bgp", actions: []action{{name: "rib status", help: "RIB summary", full: "show bgp rib status"}}},
		{prefix: "metrics", actions: []action{{name: "values", help: "Metric values", full: "metrics values"}}},
	}

	skip := map[string]bool{"ze_show_bgp": true}
	tools := generateTools(groups, skip)

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool (show bgp skipped), got %d", len(tools))
	}
	name, _ := tools[0]["name"].(string)
	if name != "ze_metrics" {
		t.Errorf("expected ze_metrics, got %s", name)
	}
}

func TestBuildToolDefActionEnum(t *testing.T) {
	g := toolGroup{
		prefix: "show bgp",
		actions: []action{
			{name: "rib", help: "Show routes", full: "show bgp rib"},
			{name: "rib status", help: "RIB summary", full: "show bgp rib status"},
		},
	}

	tool := buildToolDef(g)
	if tool == nil {
		t.Fatal("buildToolDef returned nil")
	}

	name, _ := tool["name"].(string)
	if name != "ze_show_bgp" {
		t.Errorf("name = %q, want ze_show_bgp", name)
	}

	// Parse inputSchema to check action enum.
	schemaRaw, ok := tool["inputSchema"].(json.RawMessage)
	if !ok {
		t.Fatal("inputSchema not json.RawMessage")
	}
	var schema struct {
		Properties struct {
			Action struct {
				Enum []string `json:"enum"`
			} `json:"action"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(schemaRaw, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if len(schema.Properties.Action.Enum) != 2 {
		t.Fatalf("expected 2 action enums, got %d", len(schema.Properties.Action.Enum))
	}
	if schema.Properties.Action.Enum[0] != "rib" || schema.Properties.Action.Enum[1] != "rib status" {
		t.Errorf("action enums = %v, want [rib rib status]", schema.Properties.Action.Enum)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "action" {
		t.Errorf("required = %v, want [action]", schema.Required)
	}
}

func TestDispatchGenerated(t *testing.T) {
	var dispatched string
	s := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
			dispatched = cmd
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON("ok")), nil
		},
	}
	// Keys are the valid actions; the value says whether the command reads an
	// inline peer selector (dispatcher-derived, see Command.TakesInlineSelector).
	valid := map[string]bool{"rib status": false, "rib": false, "peer detail": true}

	// Action only.
	args, _ := json.Marshal(map[string]string{"action": "rib status"})
	result := s.dispatchGenerated("show bgp", valid, args)
	if dispatched != "show bgp rib status" {
		t.Errorf("dispatched = %q, want %q", dispatched, "show bgp rib status")
	}
	content, _ := result["content"].([]map[string]any)
	if len(content) == 0 || content[0]["text"] != `"ok"` {
		t.Errorf("unexpected result: %v", result)
	}

	// Action + arguments.
	args, _ = json.Marshal(map[string]string{"action": "rib", "arguments": "ipv4/unicast"})
	s.dispatchGenerated("show bgp", valid, args)
	if dispatched != "show bgp rib ipv4/unicast" {
		t.Errorf("dispatched = %q, want %q", dispatched, "show bgp rib ipv4/unicast")
	}

	// With peer: the selector goes AFTER the command's own `peer` keyword
	// (ai/rules/cli.md "Peer Commands"), never in front of the command.
	// The old prefix form built `peer 10.0.0.1 show bgp peer detail`, which the
	// dispatcher resolves nowhere.
	args, _ = json.Marshal(map[string]string{"action": "peer detail", "peer": "10.0.0.1"})
	s.dispatchGenerated("show bgp", valid, args)
	if dispatched != "show bgp peer 10.0.0.1 detail" {
		t.Errorf("dispatched = %q, want %q", dispatched, "show bgp peer 10.0.0.1 detail")
	}

	// A peer selector on an action that reads none is refused, not silently
	// spliced somewhere the dispatcher would reject (fail closed).
	dispatched = ""
	args, _ = json.Marshal(map[string]string{"action": "rib status", "peer": "10.0.0.1"})
	result = s.dispatchGenerated("show bgp", valid, args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error when peer is supplied for a non-selector action")
	}
	if dispatched != "" {
		t.Errorf("no command should have been dispatched, got %q", dispatched)
	}

	// Whitespace in peer rejected.
	args, _ = json.Marshal(map[string]string{"action": "rib status", "peer": "10.0 0.1"})
	result = s.dispatchGenerated("show bgp", valid, args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for whitespace in peer")
	}

	// Newline in arguments rejected.
	args, _ = json.Marshal(map[string]string{"action": "rib status", "arguments": "foo\nbar"})
	result = s.dispatchGenerated("show bgp", valid, args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for newline in arguments")
	}

	// Nil validActions rejects all actions.
	args, _ = json.Marshal(map[string]string{"action": "rib status"})
	result = s.dispatchGenerated("show bgp", nil, args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error when validActions is nil")
	}
}

// TestAllToolsWithoutCommandLister exercises Streamable.allTools, the single
// remaining tool-list builder after the legacy handler deletion (AC-9).
func TestAllToolsWithoutCommandLister(t *testing.T) {
	s := &Streamable{cfg: StreamableConfig{}}

	tools := s.allTools(clientCapabilities{})
	if len(tools) != len(handcraftedTools) {
		t.Errorf("without CommandLister: got %d tools, want %d", len(tools), len(handcraftedTools))
	}
}

func TestAllToolsWithCommandLister(t *testing.T) {
	s := &Streamable{cfg: StreamableConfig{
		Commands: func() []CommandInfo {
			return []CommandInfo{
				{Name: "show bgp rib status", Help: "RIB summary"},
				{Name: "show bgp rib", Help: "Show routes"},
				{Name: "show bgp peer list", Help: "List peers"},
				{Name: "metrics values", Help: "Metric values"},
				{Name: "metrics list", Help: "List metrics"},
				{Name: "show config dump", Help: "Dump config"},
			}
		},
	}}

	tools := s.allTools(clientCapabilities{})
	// All handcrafted tools + 3 auto-generated groups (show-bgp, metrics, show-config).
	const autoGenerated = 3
	if want := len(handcraftedTools) + autoGenerated; len(tools) != want {
		t.Errorf("got %d tools, want %d (%d handcrafted + %d auto-generated)",
			len(tools), want, len(handcraftedTools), autoGenerated)
	}

	// Verify tool names appear.
	names := make(map[string]bool)
	for _, tool := range tools {
		if n, ok := tool["name"].(string); ok {
			names[n] = true
		}
	}
	if !names["ze_execute"] {
		t.Error("missing handcrafted ze_execute tool")
	}
	if !names["ze_reference"] {
		t.Error("missing handcrafted ze_reference tool")
	}
	if !names["ze_show_bgp"] {
		t.Error("missing auto-generated ze_show_bgp tool")
	}
	if !names["ze_metrics"] {
		t.Error("missing auto-generated ze_metrics tool")
	}
}

// TestZeReferenceTool verifies the handcrafted ze_reference tool returns the
// machine-readable AI reference as JSON, the same data as `ze help ai --json`,
// so an MCP client can discover this instance's capabilities on connect.
//
// VALIDATES: ze_reference is registered and returns parseable reference JSON
// with the contract keys.
// PREVENTS: the MCP discovery tool silently breaking or losing a top-level
// section relative to the CLI reference.
// TestExecuteWithoutCommandFailsClosed covers AC-15.
//
// It restores the coverage of TestZeExecute_MissingCommandNoCapability, which
// was deleted wholesale with elicit_test.go even though its SUBJECT survived:
// the `input.Command == ""` guard in toolHandlers["ze_execute"] is still there.
// Only the guard's context changed. The pre-cutover code asked the client for a
// command over the server-initiated request frame, and MCP 2026-07-28 removed
// that frame. A failure that names the argument is the only answer left.
//
// VALIDATES: ze_execute with an empty `command` returns isError with "missing
// required argument", and dispatches nothing.
// PREVENTS: the guard being dropped as dead code now that the elicit branch it
// sat beside is gone. Without it an empty command reaches the dispatcher as a
// blank command line.
func TestExecuteWithoutCommandFailsClosed(t *testing.T) {
	dispatched := 0
	runner := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
			dispatched++
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON("unreachable")), nil
		},
	}

	for _, args := range []string{`{"command":""}`, `{}`} {
		t.Run(args, func(t *testing.T) {
			result := toolHandlers["ze_execute"](runner, json.RawMessage(args))
			if result["isError"] != true {
				t.Fatalf("isError = %v, want true (result %v)", result["isError"], result)
			}
			content, _ := result["content"].([]map[string]any)
			if len(content) == 0 {
				t.Fatalf("result.content empty: %v", result)
			}
			text, _ := content[0]["text"].(string)
			if !strings.Contains(text, "missing required argument") {
				t.Fatalf("text = %q, want it to name the missing required argument", text)
			}
		})
	}

	// The guard runs BEFORE dispatch: an empty command never reaches the
	// command line as a blank string.
	if dispatched != 0 {
		t.Fatalf("dispatcher called %d time(s) for an empty command, want 0", dispatched)
	}
}

// TestDispatchRunsUnderTheRequestContext is the regression test for a dead
// field.
//
// server.ctx was written at both construction sites (callTool for a tools/call,
// createTask for a task worker) and read NOWHERE. The two dispatch sites passed
// context.Background(). Two documented mechanisms were inert as a result.
//
// First, a task worker's execution deadline canceled a context that nothing
// observed. So "a well-behaved dispatch that selects on ctx.Done() unwinds on
// its own" (tasks.go, Create) was false, only the registry-side backstop
// worked, and the goroutine leaked unconditionally. Second, a client disconnect
// did not unblock a handler, and MCP 2026-07-28 names that disconnect the
// cancellation signal.
//
// VALIDATES: both dispatch paths -- the handcrafted ze_execute handler and
// (*server).run, which every generated tool and therefore every task takes --
// hand the runner's context to the dispatcher, so canceling it unwinds a
// blocked dispatch. And the nil-ctx fallback still dispatches.
// PREVENTS: either site reverting to context.Background(), which reads as a
// harmless simplification and silently severs the deadline and the disconnect
// again. A dispatcher that never returns is what this test would then hang on.
func TestDispatchRunsUnderTheRequestContext(t *testing.T) {
	// blockingDispatch waits for its context and reports how it ended. A
	// dispatcher that ignored the passed context would never return here. The
	// test therefore fails on a timeout rather than on a proxy assertion.
	blockingDispatch := func(ctx context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	for _, tt := range []struct {
		name string
		call func(s *server) map[string]any
	}{
		{
			name: "ze_execute",
			call: func(s *server) map[string]any {
				return toolHandlers[toolNameExecute](s, json.RawMessage(`{"command":"show version"}`))
			},
		},
		{
			// The generated-tool path: dispatchGenerated funnels into run, which
			// is what a task worker executes.
			name: "generated tool via run",
			call: func(s *server) map[string]any {
				return s.dispatchGenerated("show bgp", map[string]bool{"summary": false},
					json.RawMessage(`{"action":"summary"}`))
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			runner := &server{dispatch: blockingDispatch, ctx: ctx}

			done := make(chan map[string]any, 1)
			go func() { done <- tt.call(runner) }()

			// The dispatch is blocked on the context this runner carries.
			// Canceling it is the only thing that can release it.
			cancel()

			select {
			case result := <-done:
				if result["isError"] != true {
					t.Fatalf("isError = %v, want true: a canceled dispatch must surface as a tool error (%v)",
						result["isError"], result)
				}
			case <-time.After(unwindFailureBound):
				// Not a timing assumption. A correct build returns on the line
				// above the instant cancel() lands. This arm is therefore
				// reached only when the context did not reach the dispatcher. It
				// exists so the mutation fails in seconds instead of a deadlock
				// of the package.
				t.Fatal("dispatch did not unwind after its context was canceled: the runner's ctx never reached the dispatcher")
			}
		})
	}

	// The nil-ctx case a bare *server in a unit test produces still dispatches,
	// under Background rather than panicking on a nil context.
	t.Run("nil ctx falls back to Background", func(t *testing.T) {
		var seen context.Context
		runner := &server{dispatch: func(ctx context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
			seen = ctx
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON("ok: "+cmd)), nil
		}}
		if result := toolHandlers[toolNameExecute](runner, json.RawMessage(`{"command":"show version"}`)); result["isError"] != nil {
			t.Fatalf("nil-ctx dispatch failed: %v", result)
		}
		if seen == nil {
			t.Fatal("dispatcher received a nil context")
		}
		if seen.Err() != nil {
			t.Fatalf("fallback context is already done: %v", seen.Err())
		}
	})
}

func TestZeReferenceTool(t *testing.T) {
	handler, ok := toolHandlers["ze_reference"]
	if !ok {
		t.Fatal("ze_reference handler not registered")
	}

	result := handler(&server{}, json.RawMessage(`{}`))
	if result["isError"] != nil {
		t.Fatalf("ze_reference returned error: %v", result)
	}

	content, ok := result["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("ze_reference returned no text content: %v", result)
	}
	text, _ := content[0]["text"].(string)

	var ref map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &ref); err != nil {
		t.Fatalf("ze_reference output is not valid JSON: %v", err)
	}
	for _, key := range []string{"commands", "rpcs", "dispatch-keys", "plugins", "families", "services"} {
		if _, ok := ref[key]; !ok {
			t.Errorf("ze_reference JSON missing top-level key %q", key)
		}
	}
}

// VALIDATES: MCP completes accepted command actions only after the JSON-RPC
// body is written and flushed.
// PREVENTS: lifecycle teardown racing the MCP response writer.
func TestWriteJSONResponseCompletesCommandAfterFlush(t *testing.T) {
	recorder := httptest.NewRecorder()
	completed := false
	commandResponse := plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"accepted":true}`))
	commandResponse.OnTransportComplete(func() {
		if !recorder.Flushed {
			t.Error("completion ran before the MCP response flush")
		}
		if !strings.Contains(recorder.Body.String(), `"jsonrpc":"2.0"`) {
			t.Error("completion ran before the MCP response body was written")
		}
		completed = true
	})
	rendered := &plugin.RenderedResponse{Output: `{"accepted":true}`, Response: commandResponse}
	resp := &response{
		JSONRPC:    "2.0",
		Result:     TextResult(rendered.Output),
		completion: rendered,
	}

	writeJSONResponse(recorder, resp)

	if !completed {
		t.Fatal("MCP writer did not complete the accepted action")
	}
}

// TestCallToolGeneratedViaHTTP drives a synchronous auto-generated tools/call
// through the Streamable HTTP path: one self-contained POST carrying its own
// headers and _meta -> dispatchGenerated -> dispatcher output framed as MCP
// content.
//
// the three tests below each lost a two-line `initialize` guard
// (`if status != http.StatusOK || sid == ""`). Those guards asserted that the
// handshake SETUP step succeeded, not any behavior under test. MCP 2026-07-28
// removed the handshake, so there is no longer a setup step to guard. Each
// test's real assertion is unchanged: the dispatched command string and the
// framed result. And the single-POST form now exercises header and _meta
// validation on the way in, which the handshake form did not.
func TestCallToolGeneratedViaHTTP(t *testing.T) {
	var dispatched string
	hs, cleanup := newTestStreamable(t, StreamableConfig{
		Dispatch: func(_ context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
			dispatched = cmd
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON("result-ok")), nil
		},
		Commands: func() []CommandInfo {
			return []CommandInfo{
				{Name: "show bgp rib status", Help: "RIB summary"},
				{Name: "show bgp rib", Help: "Show routes"},
				{Name: "show bgp peer list", Help: "List peers"},
				{Name: "show config dump", Help: "Dump config"},
			}
		},
	})
	defer cleanup()

	// Call the auto-generated ze_show_bgp tool with action "rib status".
	_, result := postMCP(t, hs, methodToolsCall, capsNone,
		`{"name":"ze_show_bgp","arguments":{"action":"rib status"}}`)

	if dispatched != "show bgp rib status" {
		t.Errorf("dispatched = %q, want %q", dispatched, "show bgp rib status")
	}
	res, ok := result["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result, got: %v", result)
	}
	content, ok := res["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected content, got: %v", res)
	}
	first, _ := content[0].(map[string]any)
	// Plain-text fake output marshals to a quoted JSON string via the unified
	// typed dispatcher.
	if text, _ := first["text"].(string); text != `"result-ok"` {
		t.Errorf("unexpected response text: %q", text)
	}
}

func TestCallToolAutoGeneratedViaHTTPSecondTool(t *testing.T) {
	var dispatched string
	hs, cleanup := newTestStreamable(t, StreamableConfig{
		Dispatch: func(_ context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
			dispatched = cmd
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON("ok")), nil
		},
		Commands: func() []CommandInfo {
			return []CommandInfo{
				{Name: "show bgp rib status", Help: "RIB summary"},
				{Name: "show bgp peer list", Help: "List peers"},
				{Name: "metrics values", Help: "Metric values"},
			}
		},
	})
	defer cleanup()

	// Call auto-generated ze_metrics tool.
	_, result := postMCP(t, hs, methodToolsCall, capsNone,
		`{"name":"ze_metrics","arguments":{"action":"values"}}`)

	if dispatched != "metrics values" {
		t.Errorf("dispatched = %q, want %q", dispatched, "metrics values")
	}
	if _, ok := result["result"].(map[string]any); !ok {
		t.Fatalf("expected result, got: %v", result)
	}
}

// --- Additional tests from deep review findings ---

func TestGroupCommandsEmpty(t *testing.T) {
	// Finding #12: empty input.
	groups := groupCommands(nil)
	if len(groups) != 0 {
		t.Errorf("nil input: got %d groups, want 0", len(groups))
	}
	groups = groupCommands([]CommandInfo{})
	if len(groups) != 0 {
		t.Errorf("empty input: got %d groups, want 0", len(groups))
	}
}

func TestGroupCommandsActionContent(t *testing.T) {
	// Finding #3: assert action content, not just counts.
	commands := []CommandInfo{
		{Name: "show bgp rib status", Help: "RIB summary"},
		{Name: "show bgp rib", Help: "Show routes"},
		{Name: "show bgp rib best status", Help: "Best-path status"},
		{Name: "show bgp peer list", Help: "List peers"},
		{Name: "show config dump", Help: "Dump config"},
	}
	groups := groupCommands(commands)
	byPrefix := make(map[string]toolGroup)
	for _, g := range groups {
		byPrefix[g.prefix] = g
	}
	g, ok := byPrefix["show bgp"]
	if !ok {
		t.Fatal("expected 'show bgp' group")
	}
	want := []struct{ name, full, help string }{
		{"peer list", "show bgp peer list", "List peers"},
		{"rib", "show bgp rib", "Show routes"},
		{"rib best status", "show bgp rib best status", "Best-path status"},
		{"rib status", "show bgp rib status", "RIB summary"},
	}
	if len(g.actions) != len(want) {
		t.Fatalf("actions: got %d, want %d", len(g.actions), len(want))
	}
	for i, w := range want {
		a := g.actions[i]
		if a.name != w.name || a.full != w.full || a.help != w.help {
			t.Errorf("action[%d] = {%q, %q, %q}, want {%q, %q, %q}",
				i, a.name, a.full, a.help, w.name, w.full, w.help)
		}
	}
}

func TestGroupCommandsSingleToken(t *testing.T) {
	// Finding: single-token command.
	groups := groupCommands([]CommandInfo{{Name: "summary", Help: "BGP summary"}})
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	if groups[0].prefix != "summary" {
		t.Errorf("prefix = %q, want summary", groups[0].prefix)
	}
	if len(groups[0].actions) != 1 || groups[0].actions[0].name != "" {
		t.Errorf("expected single action with empty name, got %v", groups[0].actions)
	}
}

func TestBuildToolDefNoAction(t *testing.T) {
	// Finding #4: no-named-actions branch (prefix IS the command).
	g := toolGroup{
		prefix:  "version",
		actions: []action{{name: "", help: "Show version", full: "version"}},
	}
	tool := buildToolDef(g)
	if tool == nil {
		t.Fatal("buildToolDef returned nil")
	}

	schemaRaw, ok := tool["inputSchema"].(json.RawMessage)
	if !ok {
		t.Fatal("inputSchema not json.RawMessage")
	}
	var schema struct {
		Properties map[string]any `json:"properties"`
		Required   []string       `json:"required"`
	}
	if err := json.Unmarshal(schemaRaw, &schema); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, hasAction := schema.Properties["action"]; hasAction {
		t.Error("no-action tool should not have 'action' property")
	}
	if len(schema.Required) != 0 {
		t.Errorf("required = %v, want empty", schema.Required)
	}
	desc, ok := tool["description"].(string)
	if !ok {
		t.Fatal("description not string")
	}
	if desc != "Show version" {
		t.Errorf("description = %q, want %q", desc, "Show version")
	}
}

func TestBuildToolDefSingleAction(t *testing.T) {
	// Finding #10: single named action uses help text as description.
	g := toolGroup{
		prefix:  "cache",
		actions: []action{{name: "list", help: "List cache entries", full: "cache list"}},
	}
	tool := buildToolDef(g)
	desc, ok := tool["description"].(string)
	if !ok {
		t.Fatal("description not string")
	}
	if desc != "List cache entries" {
		t.Errorf("description = %q, want %q", desc, "List cache entries")
	}
}

func TestBuildToolDefSingleActionEmptyHelp(t *testing.T) {
	// Finding: single action with empty help.
	g := toolGroup{
		prefix:  "cache",
		actions: []action{{name: "list", help: "", full: "cache list"}},
	}
	tool := buildToolDef(g)
	desc, ok := tool["description"].(string)
	if !ok {
		t.Fatal("description not string")
	}
	if desc != "Run 'cache list'." {
		t.Errorf("description = %q, want %q", desc, "Run 'cache list'.")
	}
}

func TestDispatchGeneratedMultiWordAction(t *testing.T) {
	// Finding #2: multi-word actions from server-controlled enum must work.
	var dispatched string
	s := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
			dispatched = cmd
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON("ok")), nil
		},
	}

	valid := map[string]bool{"rib best status": true, "rib": true, "rib status": true}
	args, _ := json.Marshal(map[string]string{"action": "rib best status"})
	result := s.dispatchGenerated("show bgp", valid, args)
	if _, isErr := result["isError"]; isErr {
		t.Errorf("multi-word action should not be rejected: %v", result)
	}
	if dispatched != "show bgp rib best status" {
		t.Errorf("dispatched = %q, want %q", dispatched, "show bgp rib best status")
	}
}

func TestDispatchGeneratedInvalidAction(t *testing.T) {
	// Security finding #2: action not in enum is rejected.
	s := &server{dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}}

	valid := map[string]bool{"rib status": true, "rib": true}
	args, _ := json.Marshal(map[string]string{"action": "rib ipv4/unicast peer * teardown"})
	result := s.dispatchGenerated("show bgp", valid, args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for action not in enum")
	}
}

func TestDispatchGeneratedInvalidJSON(t *testing.T) {
	// Finding #5: invalid JSON args.
	s := &server{dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}}
	result := s.dispatchGenerated("show bgp", nil, []byte("not-json"))
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for invalid JSON")
	}
}

func TestDispatchGeneratedEmptyArgs(t *testing.T) {
	// Finding: prefix-only dispatch (no action, no arguments).
	var dispatched string
	s := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
			dispatched = cmd
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON("ok")), nil
		},
	}
	args, _ := json.Marshal(map[string]string{})
	s.dispatchGenerated("version", nil, args)
	if dispatched != "version" {
		t.Errorf("dispatched = %q, want %q", dispatched, "version")
	}
}

func TestDispatchGeneratedTabInArguments(t *testing.T) {
	// Finding #15: tab characters rejected.
	s := &server{dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}}
	args, _ := json.Marshal(map[string]string{"arguments": "foo\tbar"})
	result := s.dispatchGenerated("show bgp", nil, args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for tab in arguments")
	}
}

func TestDispatchGeneratedDispatchError(t *testing.T) {
	// Finding: dispatch error propagated.
	s := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	valid := map[string]bool{"rib status": true}
	args, _ := json.Marshal(map[string]string{"action": "rib status"})
	result := s.dispatchGenerated("show bgp", valid, args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error result when dispatch fails")
	}
}

// TestCallToolUnknownViaHTTP verifies an unknown tool name returns the
// JSON-RPC invalid-params error through the Streamable HTTP path
// (registry mode; the provider-mode twin lives in provider_test.go).
func TestCallToolUnknownViaHTTP(t *testing.T) {
	// Finding #6: unknown tool name returns error.
	hs, cleanup := newTestStreamable(t, StreamableConfig{
		Commands: func() []CommandInfo { return nil },
	})
	defer cleanup()

	_, result := postMCP(t, hs, methodToolsCall, capsNone,
		`{"name":"ze_nonexistent","arguments":{}}`)

	errObj, ok := result["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected JSON-RPC error, got: %v", result)
	}
	if code, _ := errObj["code"].(float64); int(code) != -32602 {
		t.Errorf("error code = %v, want -32602", errObj["code"])
	}
	if msg, _ := errObj["message"].(string); !strings.Contains(msg, "unknown tool") {
		t.Errorf("error message = %q, want to contain 'unknown tool'", msg)
	}
}

func TestHandcraftedSkipPreventsDuplicates(t *testing.T) {
	// Finding #1: handcrafted tool names prevent duplicate auto-generated tools.
	s := &Streamable{cfg: StreamableConfig{
		Commands: func() []CommandInfo {
			return []CommandInfo{
				// "commands" prefix would generate ze_commands, colliding with handcrafted.
				{Name: "commands list", Help: "List commands"},
				{Name: "commands help", Help: "Help on commands"},
				{Name: "show bgp rib status", Help: "RIB summary"},
				{Name: "show bgp peer list", Help: "List peers"},
			}
		},
	}}

	tools := s.allTools(clientCapabilities{})
	// Count how many times ze_commands appears.
	count := 0
	for _, tool := range tools {
		if tool["name"] == "ze_commands" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("ze_commands appears %d times, want exactly 1 (handcrafted only)", count)
	}
}

// TestBearerTokenAuth and TestBearerTokenEmptyAllowsAll removed
// with the legacy Handler deletion (spec-followup-subsystem AC-9). The bearer
// authenticator is covered by bearer_test.go
// (TestBearerAuthenticator_ValidToken/MissingHeader/WrongToken) and the
// Streamable's initialize-time auth failure by
// TestStreamableBearerAuthFailureAuditRecord in streamable_test.go.

func TestTypedParamsInToolSchema(t *testing.T) {
	// Verify YANG RPC params flow through to tool JSON schema as typed properties.
	s := &Streamable{cfg: StreamableConfig{
		Commands: func() []CommandInfo {
			return []CommandInfo{
				{
					Name: "show bgp rib",
					Help: "Show routes",
					Params: []ParamInfo{
						{Name: "family", Type: "string", Description: "Address family", Required: false},
						{Name: "count", Type: "uint32", Description: "Max results", Required: false},
					},
				},
				{
					Name: "show bgp rib status",
					Help: "RIB summary",
				},
				{
					Name: "show bgp peer list",
					Help: "List peers",
				},
				{
					// The one selector-taking command in this group: the
					// dispatcher reads an inline peer selector for it, so the
					// group legitimately advertises a `peer` argument.
					Name:          "show bgp peer detail",
					Help:          "Peer detail",
					TakesSelector: true,
				},
				{
					Name: "show config dump",
					Help: "Dump config",
				},
			}
		},
	}}

	tools := s.allTools(clientCapabilities{})
	// Find ze_show_bgp in the tool list.
	var ribTool map[string]any
	var configTool map[string]any
	for _, tool := range tools {
		if tool["name"] == "ze_show_bgp" {
			ribTool = tool
		}
		if tool["name"] == "ze_show_config" {
			configTool = tool
		}
	}
	if ribTool == nil {
		t.Fatal("ze_show_bgp tool not found")
	}

	schemaRaw, ok := ribTool["inputSchema"].(json.RawMessage)
	if !ok {
		t.Fatal("inputSchema not json.RawMessage")
	}
	var schema struct {
		Properties map[string]struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schemaRaw, &schema); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// "family" should be string type.
	if fam, ok := schema.Properties["family"]; !ok {
		t.Error("missing 'family' property from YANG params")
	} else {
		if fam.Type != "string" {
			t.Errorf("family type = %q, want string", fam.Type)
		}
		if fam.Description != "Address family" {
			t.Errorf("family description = %q, want 'Address family'", fam.Description)
		}
	}

	// "count" should be integer (mapped from uint32).
	if cnt, ok := schema.Properties["count"]; !ok {
		t.Error("missing 'count' property from YANG params")
	} else if cnt.Type != "integer" {
		t.Errorf("count type = %q, want integer", cnt.Type)
	}

	// "arguments" should NOT be present (typed params replace it).
	if _, ok := schema.Properties["arguments"]; ok {
		t.Error("'arguments' should not be present when typed params exist")
	}

	// "peer" should still be present: this group contains a selector-taking
	// command ("show bgp peer detail").
	if _, ok := schema.Properties["peer"]; !ok {
		t.Error("missing 'peer' property")
	}

	// ...and must be ABSENT from a group with no selector-taking command.
	// It used to be added to every generated tool, which invited a model to set
	// it on `show config dump` and produced a command resolving nowhere.
	if configTool == nil {
		t.Fatal("ze_show_config tool not found")
	}
	configSchemaRaw, ok := configTool["inputSchema"].(json.RawMessage)
	if !ok {
		t.Fatal("ze_show_config inputSchema not json.RawMessage")
	}
	var configSchema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(configSchemaRaw, &configSchema); err != nil {
		t.Fatalf("unmarshal ze_show_config schema: %v", err)
	}
	if _, ok := configSchema.Properties["peer"]; ok {
		t.Error("ze_show_config advertises a 'peer' property but none of its commands take a selector")
	}

	// "action" should still have enum.
	if _, ok := schema.Properties["action"]; !ok {
		t.Error("missing 'action' property")
	}
}

func TestToolDescriptor_TaskSupportField(t *testing.T) {
	commands := []CommandInfo{
		{Name: "show bgp rib dump", Help: "Dump RIB", TaskSupport: TaskSupportRequired},
		{Name: "show bgp rib status", Help: "RIB summary", TaskSupport: TaskSupportOptional},
		{Name: "show config dump", Help: "Dump config", TaskSupport: TaskSupportOptional},
	}
	groups := groupCommands(commands)
	tools := generateTools(groups, handcraftedNames())
	if len(tools) == 0 {
		t.Fatal("no tools generated")
	}
	for _, tool := range tools {
		exec, ok := tool["execution"].(map[string]any)
		if !ok {
			t.Fatalf("tool %v missing execution field", tool["name"])
		}
		ts, ok := exec["taskSupport"].(string)
		if !ok {
			t.Fatalf("tool %v missing taskSupport", tool["name"])
		}
		if ts != "required" && ts != "optional" && ts != "forbidden" {
			t.Errorf("tool %v: unexpected taskSupport %q", tool["name"], ts)
		}
	}

	commands2 := []CommandInfo{
		{Name: "ping host", Help: "Ping", TaskSupport: TaskSupportForbidden},
	}
	groups2 := groupCommands(commands2)
	tools2 := generateTools(groups2, handcraftedNames())
	if len(tools2) == 0 {
		t.Fatal("no tools generated for forbidden case")
	}
	exec2, ok := tools2[0]["execution"].(map[string]any)
	if !ok {
		t.Fatal("missing execution field on forbidden tool")
	}
	if exec2["taskSupport"] != "forbidden" {
		t.Errorf("expected forbidden, got %v", exec2["taskSupport"])
	}
}

func TestYANGTypeToJSON(t *testing.T) {
	tests := []struct {
		yang string
		want string
	}{
		{"string", "string"},
		{"uint32", "integer"},
		{"int64", "integer"},
		{"boolean", "boolean"},
		{"enumeration", "string"},
		{"ip-address", "string"},
		{"unknown-type", "string"},
	}
	for _, tt := range tests {
		got := yangTypeToJSON(tt.yang)
		if got != tt.want {
			t.Errorf("yangTypeToJSON(%q) = %q, want %q", tt.yang, got, tt.want)
		}
	}
}

func TestToolDescriptor_UIMetaFromYANG(t *testing.T) {
	commands := []CommandInfo{
		{
			Name: "show bgp peer list",
			Help: "List peers",
			UIResource: &UIResourceInfo{
				Path:        "bgp-peer/index.html",
				Permissions: "network",
				CSP:         "default-src 'self'",
			},
		},
		{Name: "show bgp peer detail", Help: "Peer details"},
		{Name: "show bgp rib status", Help: "RIB summary"},
		{Name: "show bgp rib", Help: "Show routes"},
		{Name: "show config dump", Help: "Dump config"},
	}
	groups := groupCommands(commands)
	var peerGroup *toolGroup
	for i := range groups {
		if groups[i].prefix == "show bgp" {
			peerGroup = &groups[i]
			break
		}
	}
	if peerGroup == nil {
		var names []string
		for _, g := range groups {
			names = append(names, g.prefix)
		}
		t.Fatalf("show bgp group not found in %v", names)
	}
	tool := buildToolDef(*peerGroup)
	if tool == nil {
		t.Fatal("buildToolDef returned nil")
	}
	meta, ok := tool["_meta"].(map[string]any)
	if !ok {
		t.Fatal("tool missing _meta")
	}
	ui, ok := meta["ui"].(map[string]any)
	if !ok {
		t.Fatal("_meta missing ui")
	}
	if uri, _ := ui["resourceUri"].(string); uri != "ui://bgp-peer/index.html" {
		t.Errorf("resourceUri = %q, want %q", uri, "ui://bgp-peer/index.html")
	}
}

func TestToolDescriptor_UIMetaPermissionsAndCSP(t *testing.T) {
	commands := []CommandInfo{
		{
			Name: "show bgp peer list",
			Help: "List peers",
			UIResource: &UIResourceInfo{
				Path:        "bgp-peer/index.html",
				Permissions: "network clipboard",
				CSP:         "default-src 'self'; script-src 'self'",
			},
		},
	}
	groups := groupCommands(commands)
	if len(groups) == 0 {
		t.Fatal("no groups")
	}
	tool := buildToolDef(groups[0])
	if tool == nil {
		t.Fatal("buildToolDef returned nil")
	}
	meta, _ := tool["_meta"].(map[string]any)
	ui, _ := meta["ui"].(map[string]any)

	perms, ok := ui["permissions"].([]string)
	if !ok {
		t.Fatal("permissions not []string")
	}
	if len(perms) != 2 || perms[0] != "network" || perms[1] != "clipboard" {
		t.Errorf("permissions = %v, want [network clipboard]", perms)
	}
	csp, _ := ui["csp"].(string)
	if csp != "default-src 'self'; script-src 'self'" {
		t.Errorf("csp = %q, want %q", csp, "default-src 'self'; script-src 'self'")
	}
}

func TestToolDescriptor_NoUIMetaWithoutResource(t *testing.T) {
	commands := []CommandInfo{
		{Name: "show bgp rib status", Help: "RIB summary"},
		{Name: "show bgp rib", Help: "Show routes"},
		{Name: "show config dump", Help: "Dump config"},
	}
	groups := groupCommands(commands)
	for _, g := range groups {
		tool := buildToolDef(g)
		if tool == nil {
			continue
		}
		if tool["_meta"] != nil {
			t.Errorf("tool %q has _meta but no UIResource", tool["name"])
		}
	}
}

// TestPeerPropertyOnlyWhereUsable pins the advertise-equals-accept invariant:
// a generated tool offers the `peer` argument exactly when its dispatch path
// can place a value in the command.
//
// VALIDATES: `peer` is advertised for a command with an inline selector AND a
// `peer` keyword, and withheld both when there is no inline selector and when
// the inline selector is not a peer (`clear dns cache record <name>`).
// PREVENTS: re-advertising `peer` on every generated tool (the original bug,
// which produced `peer <sel> show bgp peer detail`), and the subtler variant of
// offering it on a selector-taking command with no `peer` keyword to anchor the
// value to, where every call could only ever return an error.
func TestPeerPropertyOnlyWhereUsable(t *testing.T) {
	tests := []struct {
		name   string
		action action
		want   bool
	}{
		{"peer keyword and inline selector", action{name: "peer detail", full: "show bgp peer detail", takesSelector: true}, true},
		{"peer keyword, selector last", action{name: "peer", full: "delete bgp peer", takesSelector: true}, true},
		{"inline selector but no peer keyword", action{name: "cache record", full: "clear dns cache record", takesSelector: true}, false},
		{"peer keyword but no inline selector", action{name: "peer list", full: "show bgp peer list", takesSelector: false}, false},
		{"neither", action{name: "dump", full: "show config dump", takesSelector: false}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := actionAcceptsPeer(tt.action); got != tt.want {
				t.Errorf("actionAcceptsPeer(%q) = %v, want %v", tt.action.full, got, tt.want)
			}
			if got := groupTakesSelector([]action{tt.action}); got != tt.want {
				t.Errorf("groupTakesSelector(%q) = %v, want %v", tt.action.full, got, tt.want)
			}
		})
	}

	// Whatever the group advertises, the dispatch path must agree: splicing
	// succeeds for exactly the accepted set, and puts the value after `peer`.
	for _, tt := range tests {
		spliced, ok := spliceSelector(tt.action.full, "10.0.0.1")
		if !tt.want {
			continue
		}
		if !ok {
			t.Errorf("%s: advertised as accepting a peer but spliceSelector refused %q", tt.name, tt.action.full)
			continue
		}
		if !strings.Contains(spliced, "peer 10.0.0.1") {
			t.Errorf("%s: selector not placed after the peer keyword: %q", tt.name, spliced)
		}
	}
}

// TestGroupTaskSupportForbiddenWins covers AC-14 and closes R-1b.
//
// VALIDATES: a command group mixing `required` and `forbidden` actions resolves
// to forbidden, so the group is never auto-tasked.
// PREVENTS: the precedence that was harmless under the old client-directed
// model becoming a promotion rule under the server-directed one. Required-wins
// would auto-task an action whose YANG explicitly annotated it forbidden.
//
// The mixed case is the whole point. No group mixes levels today. The forbidden
// rib actions sit under `clear`/`request` while the required one sits under
// `show`, so they land in different tools. That is exactly why the guard has to
// be tested rather than observed.
func TestGroupTaskSupportForbiddenWins(t *testing.T) {
	cases := []struct {
		name    string
		actions []action
		want    TaskSupportLevel
	}{
		{
			name:    "no actions is optional",
			actions: nil,
			want:    TaskSupportOptional,
		},
		{
			name:    "all optional",
			actions: []action{{name: "a"}, {name: "b"}},
			want:    TaskSupportOptional,
		},
		{
			name:    "a required action promotes the group",
			actions: []action{{name: "a"}, {name: "b", taskSupport: TaskSupportRequired}},
			want:    TaskSupportRequired,
		},
		{
			name:    "all forbidden",
			actions: []action{{name: "a", taskSupport: TaskSupportForbidden}, {name: "b", taskSupport: TaskSupportForbidden}},
			want:    TaskSupportForbidden,
		},
		{
			// The R-1b case: one forbidden action poisons the group.
			name: "mixed required and forbidden resolves to forbidden",
			actions: []action{
				{name: "show", taskSupport: TaskSupportRequired},
				{name: "inject", taskSupport: TaskSupportForbidden},
			},
			want: TaskSupportForbidden,
		},
		{
			// Order must not decide it.
			name: "mixed in the other order still resolves to forbidden",
			actions: []action{
				{name: "inject", taskSupport: TaskSupportForbidden},
				{name: "show", taskSupport: TaskSupportRequired},
			},
			want: TaskSupportForbidden,
		},
		{
			name: "a single forbidden action among optionals still forbids",
			actions: []action{
				{name: "a"},
				{name: "inject", taskSupport: TaskSupportForbidden},
				{name: "b"},
			},
			want: TaskSupportForbidden,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := groupTaskSupport(tc.actions); got != tc.want {
				t.Fatalf("groupTaskSupport(%v) = %v, want %v", tc.actions, got, tc.want)
			}
		})
	}
}

// TestTaskEligibilityFromAnnotation covers the D-1 mapping.
//
// VALIDATES: the annotation alone decides, per tool -- `required` yields a task
// handle, `forbidden` and `optional` run synchronously -- for a client that HAS
// declared the extension.
// PREVENTS: reintroducing a second per-command policy surface, or a wall-clock
// promotion rule, beside the one YANG annotation.
func TestTaskEligibilityFromAnnotation(t *testing.T) {
	cases := []struct {
		name        string
		level       TaskSupportLevel
		wantTasked  bool
		wantSummary string
	}{
		{name: "required is always tasked", level: TaskSupportRequired, wantTasked: true, wantSummary: "task"},
		{name: "forbidden is never tasked", level: TaskSupportForbidden, wantTasked: false, wantSummary: "complete"},
		{name: "optional is synchronous", level: TaskSupportOptional, wantTasked: false, wantSummary: "complete"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, err := NewStreamable(StreamableConfig{
				Commands: func() []CommandInfo {
					return []CommandInfo{{Name: "probe cmd", Help: "Probe", TaskSupport: tc.level}}
				},
				// A dispatcher is required, not optional scaffolding. The
				// `required` case launches a real worker goroutine, and a nil
				// dispatch would panic there rather than inside the request.
				Dispatch: func(_ context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
					return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON("ok: "+cmd)), nil
				},
			})
			if err != nil {
				t.Fatalf("NewStreamable: %v", err)
			}
			defer srv.Close()

			if got := srv.lookupTaskSupport("ze_probe"); got != tc.level {
				t.Fatalf("lookupTaskSupport = %v, want %v", got, tc.level)
			}

			hs := httptest.NewServer(srv)
			defer hs.Close()

			status, parsed := postMCP(t, hs, methodToolsCall, capsTasks,
				`{"name":"ze_probe","arguments":{"action":"cmd"}}`)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200: the command must run either way (body %v)", status, parsed)
			}
			result := resultOf(t, parsed)
			if got := result[resultTypeKey]; got != tc.wantSummary {
				t.Fatalf("resultType = %v, want %q", got, tc.wantSummary)
			}
			id, _ := result[resultKeyTaskID].(string)
			if tasked := id != ""; tasked != tc.wantTasked {
				t.Fatalf("tasked = %v (taskId %q), want %v", tasked, id, tc.wantTasked)
			}
		})
	}
}
