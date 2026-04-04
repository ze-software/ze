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
		{Name: "rib status", Help: "RIB summary"},
		{Name: "rib routes", Help: "Show routes"},
		{Name: "rib best status", Help: "Best-path status"},
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

	// "rib" groups at depth 1 (no depth-2 sibling groups).
	if g, ok := byPrefix["rib"]; !ok {
		t.Fatal("expected 'rib' group")
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
		{"rib", "ze_rib"},
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
		{prefix: "rib", actions: []action{{name: "status", help: "RIB summary", full: "rib status"}}},
		{prefix: "metrics", actions: []action{{name: "values", help: "Metric values", full: "metrics values"}}},
	}

	skip := map[string]bool{"ze_rib": true}
	tools := generateTools(groups, skip)

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool (rib skipped), got %d", len(tools))
	}
	name, _ := tools[0]["name"].(string)
	if name != "ze_metrics" {
		t.Errorf("expected ze_metrics, got %s", name)
	}
}

func TestBuildToolDefActionEnum(t *testing.T) {
	g := toolGroup{
		prefix: "rib",
		actions: []action{
			{name: "routes", help: "Show routes", full: "rib routes"},
			{name: "status", help: "RIB summary", full: "rib status"},
		},
	}

	tool := buildToolDef(g)
	if tool == nil {
		t.Fatal("buildToolDef returned nil")
	}

	name, _ := tool["name"].(string)
	if name != "ze_rib" {
		t.Errorf("name = %q, want ze_rib", name)
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
	result := s.dispatchGenerated("rib", valid, args)
	if dispatched != "rib status" {
		t.Errorf("dispatched = %q, want %q", dispatched, "rib status")
	}
	content, _ := result["content"].([]map[string]any)
	if len(content) == 0 || content[0]["text"] != "ok" {
		t.Errorf("unexpected result: %v", result)
	}

	// Action + arguments.
	args, _ = json.Marshal(map[string]string{"action": "routes", "arguments": "ipv4/unicast"})
	s.dispatchGenerated("rib", valid, args)
	if dispatched != "rib routes ipv4/unicast" {
		t.Errorf("dispatched = %q, want %q", dispatched, "rib routes ipv4/unicast")
	}

	// With peer.
	args, _ = json.Marshal(map[string]string{"action": "status", "peer": "10.0.0.1"})
	s.dispatchGenerated("rib", valid, args)
	if dispatched != "peer 10.0.0.1 rib status" {
		t.Errorf("dispatched = %q, want %q", dispatched, "peer 10.0.0.1 rib status")
	}

	// Whitespace in peer rejected.
	args, _ = json.Marshal(map[string]string{"action": "status", "peer": "10.0 0.1"})
	result = s.dispatchGenerated("rib", valid, args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for whitespace in peer")
	}

	// Newline in arguments rejected.
	args, _ = json.Marshal(map[string]string{"action": "status", "arguments": "foo\nbar"})
	result = s.dispatchGenerated("rib", valid, args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for newline in arguments")
	}

	// Nil validActions rejects all actions.
	args, _ = json.Marshal(map[string]string{"action": "status"})
	result = s.dispatchGenerated("rib", nil, args)
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
				{Name: "rib status", Help: "RIB summary"},
				{Name: "rib routes", Help: "Show routes"},
				{Name: "metrics values", Help: "Metric values"},
				{Name: "metrics list", Help: "List metrics"},
			}
		},
	}

	tools := s.allTools()
	// Handcrafted + 2 auto-generated groups (rib, metrics).
	want := len(handcraftedTools) + 2
	if len(tools) != want {
		t.Errorf("got %d tools, want %d (handcrafted=%d + generated=2)", len(tools), want, len(handcraftedTools))
	}

	// Verify auto-generated tool names appear.
	names := make(map[string]bool)
	for _, tool := range tools {
		if n, ok := tool["name"].(string); ok {
			names[n] = true
		}
	}
	if !names["ze_rib"] {
		t.Error("missing auto-generated ze_rib tool")
	}
	if !names["ze_metrics"] {
		t.Error("missing auto-generated ze_metrics tool")
	}
	// Handcrafted still present.
	if !names["ze_announce"] {
		t.Error("missing handcrafted ze_announce tool")
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
				{Name: "rib status", Help: "RIB summary"},
				{Name: "rib routes", Help: "Show routes"},
			}
		},
		"",
	)

	// Call the auto-generated ze_rib tool with action "status".
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ze_rib","arguments":{"action":"status"}}}`
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if dispatched != "rib status" {
		t.Errorf("dispatched = %q, want %q", dispatched, "rib status")
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

func TestCallToolHandcraftedStillWorks(t *testing.T) {
	var dispatched string
	handler := Handler(
		func(cmd string) (string, error) {
			dispatched = cmd
			return "ok", nil
		},
		func() []CommandInfo {
			return []CommandInfo{{Name: "rib status", Help: "RIB summary"}}
		},
		"",
	)

	// Call handcrafted ze_execute tool.
	body := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ze_execute","arguments":{"command":"peer list"}}}`
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if dispatched != "peer list" {
		t.Errorf("dispatched = %q, want %q", dispatched, "peer list")
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
		{Name: "rib status", Help: "RIB summary"},
		{Name: "rib routes", Help: "Show routes"},
		{Name: "rib best status", Help: "Best-path status"},
	}
	groups := groupCommands(commands)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g := groups[0]
	if g.prefix != "rib" {
		t.Fatalf("prefix = %q, want rib", g.prefix)
	}
	// Actions are sorted alphabetically.
	want := []struct{ name, full, help string }{
		{"best status", "rib best status", "Best-path status"},
		{"routes", "rib routes", "Show routes"},
		{"status", "rib status", "RIB summary"},
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
	result := s.dispatchGenerated("rib", valid, args)
	if _, isErr := result["isError"]; isErr {
		t.Errorf("multi-word action should not be rejected: %v", result)
	}
	if dispatched != "rib best status" {
		t.Errorf("dispatched = %q, want %q", dispatched, "rib best status")
	}
}

func TestDispatchGeneratedInvalidAction(t *testing.T) {
	// Security finding #2: action not in enum is rejected.
	s := &server{dispatch: func(string) (string, error) { return "", nil }}

	valid := map[string]bool{"status": true, "routes": true}
	args, _ := json.Marshal(map[string]string{"action": "routes ipv4/unicast peer * teardown"})
	result := s.dispatchGenerated("rib", valid, args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for action not in enum")
	}
}

func TestDispatchGeneratedInvalidJSON(t *testing.T) {
	// Finding #5: invalid JSON args.
	s := &server{dispatch: func(string) (string, error) { return "", nil }}
	result := s.dispatchGenerated("rib", nil, []byte("not-json"))
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
	result := s.dispatchGenerated("rib", nil, args)
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
	result := s.dispatchGenerated("rib", valid, args)
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
				{Name: "rib status", Help: "RIB summary"},
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

// --- Handcrafted tool tests ---

func TestToolAnnounce(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		want    string // expected dispatch command prefix (empty = error expected)
		wantErr bool
	}{
		{
			name: "all fields",
			args: map[string]any{
				"peer": "10.0.0.1", "origin": "igp", "next-hop": "1.1.1.1",
				"local-preference": 100, "as-path": []int{65000, 65001},
				"community": []string{"65000:100", "65000:200"},
				"family":    "ipv4/unicast", "prefixes": []string{"10.0.0.0/24"},
			},
			want: "peer 10.0.0.1 update text origin igp local-preference 100 as-path [65000 65001] next-hop 1.1.1.1 community [65000:100 65000:200] nlri ipv4/unicast add 10.0.0.0/24",
		},
		{
			name: "minimal",
			args: map[string]any{"family": "ipv4/unicast", "prefixes": []string{"10.0.0.0/24"}},
			want: "peer * update text nlri ipv4/unicast add 10.0.0.0/24",
		},
		{
			name:    "missing family",
			args:    map[string]any{"prefixes": []string{"10.0.0.0/24"}},
			wantErr: true,
		},
		{
			name:    "missing prefixes",
			args:    map[string]any{"family": "ipv4/unicast"},
			wantErr: true,
		},
		{
			name:    "whitespace in community",
			args:    map[string]any{"family": "ipv4/unicast", "prefixes": []string{"10.0.0.0/24"}, "community": []string{"65000:100 evil"}},
			wantErr: true,
		},
		{
			name:    "whitespace in prefix",
			args:    map[string]any{"family": "ipv4/unicast", "prefixes": []string{"10.0 .0.0/24"}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dispatched string
			s := &server{dispatch: func(cmd string) (string, error) {
				dispatched = cmd
				return "ok", nil
			}}
			args, _ := json.Marshal(tt.args)
			result := s.toolAnnounce(args)
			if tt.wantErr {
				if _, isErr := result["isError"]; !isErr {
					t.Error("expected error result")
				}
				return
			}
			if _, isErr := result["isError"]; isErr {
				t.Errorf("unexpected error: %v", result)
				return
			}
			if dispatched != tt.want {
				t.Errorf("dispatched:\n  got  %q\n  want %q", dispatched, tt.want)
			}
		})
	}
}

func TestToolWithdraw(t *testing.T) {
	var dispatched string
	s := &server{dispatch: func(cmd string) (string, error) {
		dispatched = cmd
		return "ok", nil
	}}

	// Valid withdraw.
	args, _ := json.Marshal(map[string]any{
		"peer": "10.0.0.1", "family": "ipv4/unicast", "prefixes": []string{"10.0.0.0/24", "10.0.1.0/24"},
	})
	result := s.toolWithdraw(args)
	if _, isErr := result["isError"]; isErr {
		t.Fatalf("unexpected error: %v", result)
	}
	want := "peer 10.0.0.1 update text nlri ipv4/unicast del 10.0.0.0/24 del 10.0.1.0/24"
	if dispatched != want {
		t.Errorf("dispatched:\n  got  %q\n  want %q", dispatched, want)
	}

	// Missing family.
	args, _ = json.Marshal(map[string]any{"prefixes": []string{"10.0.0.0/24"}})
	result = s.toolWithdraw(args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for missing family")
	}
}

func TestToolPeers(t *testing.T) {
	var dispatched string
	s := &server{dispatch: func(cmd string) (string, error) {
		dispatched = cmd
		return "ok", nil
	}}

	// No peer: dispatches "peer list".
	args, _ := json.Marshal(map[string]any{})
	s.toolPeers(args)
	if dispatched != "peer list" {
		t.Errorf("no peer: dispatched %q, want %q", dispatched, "peer list")
	}

	// With peer: dispatches "peer X show bgp peer".
	args, _ = json.Marshal(map[string]any{"peer": "10.0.0.1"})
	s.toolPeers(args)
	if dispatched != "peer 10.0.0.1 show bgp peer" {
		t.Errorf("with peer: dispatched %q, want %q", dispatched, "peer 10.0.0.1 show bgp peer")
	}

	// Whitespace in peer rejected.
	args, _ = json.Marshal(map[string]any{"peer": "10.0 0.1"})
	result := s.toolPeers(args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for whitespace in peer")
	}
}

func TestToolPeerControl(t *testing.T) {
	var dispatched string
	s := &server{dispatch: func(cmd string) (string, error) {
		dispatched = cmd
		return "ok", nil
	}}

	for _, action := range []string{"teardown", "pause", "resume", "flush"} {
		args, _ := json.Marshal(map[string]any{"peer": "10.0.0.1", "action": action})
		result := s.toolPeerControl(args)
		if _, isErr := result["isError"]; isErr {
			t.Errorf("action %s: unexpected error: %v", action, result)
			continue
		}
		want := "peer 10.0.0.1 " + action
		if dispatched != want {
			t.Errorf("action %s: dispatched %q, want %q", action, dispatched, want)
		}
	}

	// Invalid action.
	args, _ := json.Marshal(map[string]any{"peer": "10.0.0.1", "action": "destroy"})
	result := s.toolPeerControl(args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for invalid action")
	}

	// Missing peer.
	args, _ = json.Marshal(map[string]any{"action": "teardown"})
	result = s.toolPeerControl(args)
	if _, isErr := result["isError"]; !isErr {
		t.Error("expected error for missing peer")
	}
}

func TestToolCommands(t *testing.T) {
	var dispatched string
	s := &server{dispatch: func(cmd string) (string, error) {
		dispatched = cmd
		return "[]", nil
	}}
	s.toolCommands(nil)
	if dispatched != "command-list --json" {
		t.Errorf("dispatched %q, want %q", dispatched, "command-list --json")
	}
}
