package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGroupCommands(t *testing.T) {
	commands := []CommandInfo{
		{Name: "bgp rib status", Help: "RIB summary"},
		{Name: "bgp rib routes", Help: "Show routes"},
		{Name: "bgp rib best status", Help: "Best-path status"},
		{Name: "bgp peer list", Help: "List peers"},
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

	// "rib" groups at depth 2 (bgp has multiple subgroups: rib, peer).
	if g, ok := byPrefix["bgp rib"]; !ok {
		t.Fatal("expected 'bgp rib' group")
	} else if len(g.actions) != 3 {
		t.Fatalf("rib: expected 3 actions, got %d", len(g.actions))
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
		{"bgp rib", "ze_bgp_rib"},
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
		{prefix: "bgp rib", actions: []action{{name: "status", help: "RIB summary", full: "bgp rib status"}}},
		{prefix: "metrics", actions: []action{{name: "values", help: "Metric values", full: "metrics values"}}},
	}

	skip := map[string]bool{"ze_bgp_rib": true}
	tools := generateTools(groups, skip)

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool (bgp-rib skipped), got %d", len(tools))
	}
	name, _ := tools[0]["name"].(string)
	if name != "ze_metrics" {
		t.Errorf("expected ze_metrics, got %s", name)
	}
}

func TestBuildToolDefActionEnum(t *testing.T) {
	g := toolGroup{
		prefix: "bgp rib",
		actions: []action{
			{name: "routes", help: "Show routes", full: "bgp rib routes"},
			{name: "status", help: "RIB summary", full: "bgp rib status"},
		},
	}

	tool := buildToolDef(g)
	if tool == nil {
		t.Fatal("buildToolDef returned nil")
	}

	name, _ := tool["name"].(string)
	if name != "ze_bgp_rib" {
		t.Errorf("name = %q, want ze_bgp_rib", name)
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
	if schema.Properties.Action.Enum[0] != "routes" || schema.Properties.Action.Enum[1] != "status" {
		t.Errorf("action enums = %v, want [routes status]", schema.Properties.Action.Enum)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "action" {
		t.Errorf("required = %v, want [action]", schema.Required)
	}
}

func TestDispatchGenerated(t *testing.T) {
	var dispatched string
	s := &server{
		dispatch: func(cmd string) (string, error) {
			dispatched = cmd
			return "ok", nil
		},
	}
	valid := map[string]bool{"status": true, "routes": true}

	// Action only.
	args, _ := json.Marshal(map[string]string{"action": "status"})
	result := s.dispatchGenerated("bgp rib", valid, args)
	if dispatched != "bgp rib status" {
		t.Errorf("dispatched = %q, want %q", dispatched, "bgp rib status")
	}
	content, _ := result["content"].([]map[string]any)
	if len(content) == 0 || content[0]["text"] != "ok" {
		t.Errorf("unexpected result: %v", result)
	}

	// Action + arguments.
	args, _ = json.Marshal(map[string]string{"action": "routes", "arguments": "ipv4/unicast"})
	s.dispatchGenerated("bgp rib", valid, args)
	if dispatched != "bgp rib routes ipv4/unicast" {
		t.Errorf("dispatched = %q, want %q", dispatched, "bgp rib routes ipv4/unicast")
	}

	// With peer.
	args, _ = json.Marshal(map[string]string{"action": "status", "peer": "10.0.0.1"})
	s.dispatchGenerated("bgp rib", valid, args)
	if dispatched != "peer 10.0.0.1 bgp rib status" {
		t.Errorf("dispatched = %q, want %q", dispatched, "peer 10.0.0.1 bgp rib status")
	}

	// Whitespace in peer rejected.
	args, _ = json.Marshal(map[string]string{"action": "status", "peer": "10.0 0.1"})
	result = s.dispatchGenerated("bgp rib", valid, args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for whitespace in peer")
	}

	// Newline in arguments rejected.
	args, _ = json.Marshal(map[string]string{"action": "status", "arguments": "foo\nbar"})
	result = s.dispatchGenerated("bgp rib", valid, args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for newline in arguments")
	}

	// Nil validActions rejects all actions.
	args, _ = json.Marshal(map[string]string{"action": "status"})
	result = s.dispatchGenerated("bgp rib", nil, args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error when validActions is nil")
	}
}

func TestAllToolsWithoutCommandLister(t *testing.T) {
	s := &server{dispatch: func(string) (string, error) { return "", nil }}

	tools := s.allTools()
	if len(tools) != len(handcraftedTools) {
		t.Errorf("without CommandLister: got %d tools, want %d", len(tools), len(handcraftedTools))
	}
}

func TestAllToolsWithCommandLister(t *testing.T) {
	s := &server{
		dispatch: func(string) (string, error) { return "", nil },
		commands: func() []CommandInfo {
			return []CommandInfo{
				{Name: "bgp rib status", Help: "RIB summary"},
				{Name: "bgp rib routes", Help: "Show routes"},
				{Name: "bgp peer list", Help: "List peers"},
				{Name: "metrics values", Help: "Metric values"},
				{Name: "metrics list", Help: "List metrics"},
			}
		},
	}

	tools := s.allTools()
	// Handcrafted (ze_execute) + auto-generated: 2 groups (rib, metrics) = 4 total.
	if len(tools) != 4 {
		t.Errorf("got %d tools, want 4 (ze_execute + bgp-rib + bgp-peer + metrics)", len(tools))
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
	if !names["ze_bgp_rib"] {
		t.Error("missing auto-generated ze_bgp_rib tool")
	}
	if !names["ze_metrics"] {
		t.Error("missing auto-generated ze_metrics tool")
	}
}

func TestCallToolGeneratedViaHTTP(t *testing.T) {
	var dispatched string
	handler := Handler(
		func(cmd string) (string, error) {
			dispatched = cmd
			return "result-ok", nil
		},
		func() []CommandInfo {
			return []CommandInfo{
				{Name: "bgp rib status", Help: "RIB summary"},
				{Name: "bgp rib routes", Help: "Show routes"},
				{Name: "bgp peer list", Help: "List peers"},
			}
		},
		"",
	)

	// Call the auto-generated ze_bgp_rib tool with action "status".
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ze_bgp_rib","arguments":{"action":"status"}}}`
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if dispatched != "bgp rib status" {
		t.Errorf("dispatched = %q, want %q", dispatched, "bgp rib status")
	}

	var resp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Result.Content) == 0 || resp.Result.Content[0].Text != "result-ok" {
		t.Errorf("unexpected response: %s", rr.Body.String())
	}
}

func TestCallToolAutoGeneratedViaHTTPSecondTool(t *testing.T) {
	var dispatched string
	handler := Handler(
		func(cmd string) (string, error) {
			dispatched = cmd
			return "ok", nil
		},
		func() []CommandInfo {
			return []CommandInfo{
				{Name: "bgp rib status", Help: "RIB summary"},
				{Name: "bgp peer list", Help: "List peers"},
				{Name: "metrics values", Help: "Metric values"},
			}
		},
		"",
	)

	// Call auto-generated ze_metrics tool.
	body := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ze_metrics","arguments":{"action":"values"}}}`
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if dispatched != "metrics values" {
		t.Errorf("dispatched = %q, want %q", dispatched, "metrics values")
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
		{Name: "bgp rib status", Help: "RIB summary"},
		{Name: "bgp rib routes", Help: "Show routes"},
		{Name: "bgp rib best status", Help: "Best-path status"},
		{Name: "bgp peer list", Help: "List peers"},
	}
	groups := groupCommands(commands)
	byPrefix := make(map[string]toolGroup)
	for _, g := range groups {
		byPrefix[g.prefix] = g
	}
	g, ok := byPrefix["bgp rib"]
	if !ok {
		t.Fatal("expected 'bgp rib' group")
	}
	want := []struct{ name, full, help string }{
		{"best status", "bgp rib best status", "Best-path status"},
		{"routes", "bgp rib routes", "Show routes"},
		{"status", "bgp rib status", "RIB summary"},
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
		dispatch: func(cmd string) (string, error) {
			dispatched = cmd
			return "ok", nil
		},
	}

	valid := map[string]bool{"best status": true, "routes": true, "status": true}
	args, _ := json.Marshal(map[string]string{"action": "best status"})
	result := s.dispatchGenerated("bgp rib", valid, args)
	if _, isErr := result["isError"]; isErr {
		t.Errorf("multi-word action should not be rejected: %v", result)
	}
	if dispatched != "bgp rib best status" {
		t.Errorf("dispatched = %q, want %q", dispatched, "bgp rib best status")
	}
}

func TestDispatchGeneratedInvalidAction(t *testing.T) {
	// Security finding #2: action not in enum is rejected.
	s := &server{dispatch: func(string) (string, error) { return "", nil }}

	valid := map[string]bool{"status": true, "routes": true}
	args, _ := json.Marshal(map[string]string{"action": "routes ipv4/unicast peer * teardown"})
	result := s.dispatchGenerated("bgp rib", valid, args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for action not in enum")
	}
}

func TestDispatchGeneratedInvalidJSON(t *testing.T) {
	// Finding #5: invalid JSON args.
	s := &server{dispatch: func(string) (string, error) { return "", nil }}
	result := s.dispatchGenerated("bgp rib", nil, []byte("not-json"))
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for invalid JSON")
	}
}

func TestDispatchGeneratedEmptyArgs(t *testing.T) {
	// Finding: prefix-only dispatch (no action, no arguments).
	var dispatched string
	s := &server{
		dispatch: func(cmd string) (string, error) {
			dispatched = cmd
			return "ok", nil
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
	s := &server{dispatch: func(string) (string, error) { return "", nil }}
	args, _ := json.Marshal(map[string]string{"arguments": "foo\tbar"})
	result := s.dispatchGenerated("bgp rib", nil, args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for tab in arguments")
	}
}

func TestDispatchGeneratedDispatchError(t *testing.T) {
	// Finding: dispatch error propagated.
	s := &server{
		dispatch: func(string) (string, error) {
			return "", fmt.Errorf("connection refused")
		},
	}
	valid := map[string]bool{"status": true}
	args, _ := json.Marshal(map[string]string{"action": "status"})
	result := s.dispatchGenerated("bgp rib", valid, args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error result when dispatch fails")
	}
}

func TestCallToolUnknownViaHTTP(t *testing.T) {
	// Finding #6: unknown tool name returns error.
	handler := Handler(
		func(string) (string, error) { return "", nil },
		func() []CommandInfo { return nil },
		"",
	)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ze_nonexistent","arguments":{}}}`
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var resp struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected JSON-RPC error")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("error code = %d, want -32602", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "unknown tool") {
		t.Errorf("error message = %q, want to contain 'unknown tool'", resp.Error.Message)
	}
}

func TestHandcraftedSkipPreventsDuplicates(t *testing.T) {
	// Finding #1: handcrafted tool names prevent duplicate auto-generated tools.
	s := &server{
		dispatch: func(string) (string, error) { return "", nil },
		commands: func() []CommandInfo {
			return []CommandInfo{
				// "commands" prefix would generate ze_commands, colliding with handcrafted.
				{Name: "commands list", Help: "List commands"},
				{Name: "commands help", Help: "Help on commands"},
				{Name: "bgp rib status", Help: "RIB summary"},
				{Name: "bgp peer list", Help: "List peers"},
			}
		},
	}

	tools := s.allTools()
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

func TestBearerTokenAuth(t *testing.T) {
	handler := Handler(
		func(string) (string, error) { return "ok", nil },
		nil,
		"secret-token-123",
	)

	// No token: rejected.
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("no token: status = %d, want 401", rr.Code)
	}

	// Wrong token: rejected.
	req, _ = http.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-token")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("wrong token: status = %d, want 401", rr.Code)
	}

	// Correct token: accepted.
	req, _ = http.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret-token-123")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("correct token: status = %d, want 200", rr.Code)
	}
}

func TestBearerTokenEmptyAllowsAll(t *testing.T) {
	handler := Handler(
		func(string) (string, error) { return "ok", nil },
		nil,
		"",
	)

	// No token, empty config: accepted.
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("empty token config: status = %d, want 200", rr.Code)
	}
}

func TestTypedParamsInToolSchema(t *testing.T) {
	// Verify YANG RPC params flow through to tool JSON schema as typed properties.
	s := &server{
		dispatch: func(string) (string, error) { return "", nil },
		commands: func() []CommandInfo {
			return []CommandInfo{
				{
					Name: "bgp rib routes",
					Help: "Show routes",
					Params: []ParamInfo{
						{Name: "family", Type: "string", Description: "Address family", Required: false},
						{Name: "count", Type: "uint32", Description: "Max results", Required: false},
					},
				},
				{
					Name: "bgp rib status",
					Help: "RIB summary",
				},
				{
					Name: "bgp peer list",
					Help: "List peers",
				},
			}
		},
	}

	tools := s.allTools()
	// Find ze_bgp_rib in the tool list.
	var ribTool map[string]any
	for _, tool := range tools {
		if tool["name"] == "ze_bgp_rib" {
			ribTool = tool
			break
		}
	}
	if ribTool == nil {
		t.Fatal("ze_bgp_rib tool not found")
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

	// "peer" should still be present.
	if _, ok := schema.Properties["peer"]; !ok {
		t.Error("missing 'peer' property")
	}

	// "action" should still have enum.
	if _, ok := schema.Properties["action"]; !ok {
		t.Error("missing 'action' property")
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
