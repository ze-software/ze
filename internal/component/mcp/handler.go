// Design: docs/guide/mcp/overview.md -- MCP JSON-RPC HTTP handler
// Detail: tools.go -- auto-generated MCP tools from command registry

// Package mcp provides an HTTP handler that speaks MCP (Model Context Protocol)
// JSON-RPC, wrapping Ze's command dispatcher to let AI assistants control BGP.
//
// Handcrafted tools: ze_announce, ze_withdraw, ze_peers, ze_peer_control, ze_execute, ze_commands.
// Additional tools are auto-generated from the command registry at tools/list time,
// so new YANG commands are automatically exposed without modifying this package.
//
// Usage: mount Handler() on an HTTP endpoint when --mcp <port> is set.
package mcp

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxRequestBody limits the size of MCP HTTP request bodies (1 MB).
const maxRequestBody = 1 << 20

// CommandDispatcher executes a Ze command and returns its output.
// This matches the signature of reactor.ExecuteCommand.
type CommandDispatcher func(command string) (string, error)

// Handler returns an HTTP handler that speaks MCP JSON-RPC.
// Each POST carries a JSON-RPC request; the response is a JSON-RPC response.
// Validates Content-Type to prevent CSRF from browser origins.
//
// If token is non-empty, requests must include "Authorization: Bearer <token>".
// If commands is non-nil, tools/list dynamically generates tools from registered
// commands. New YANG commands appear as MCP tools without code changes.
// If commands is nil, only the handcrafted tools are exposed.
func Handler(dispatch CommandDispatcher, commands CommandLister, token string) http.Handler {
	s := &server{dispatch: dispatch, commands: commands}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Bearer token authentication (constant-time comparison).
		if token != "" {
			auth := r.Header.Get("Authorization")
			expected := "Bearer " + token
			if subtle.ConstantTimeCompare([]byte(auth), []byte(expected)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}

		// Reject non-JSON content types to prevent CSRF via text/plain forms.
		ct := r.Header.Get("Content-Type")
		if ct != "" && !strings.HasPrefix(ct, "application/json") {
			http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}

		var req request
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, &response{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32700, Message: "parse error"},
			})
			return
		}

		resp := s.handle(&req)
		if resp == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, resp)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, writeErr := w.Write(data); writeErr != nil {
		return
	}
}

// JSON-RPC 2.0 types. All field names are lowercase (no kebab-case conflict).

type request struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type response struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  any              `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// server handles MCP requests.
type server struct {
	dispatch CommandDispatcher
	commands CommandLister // nil = handcrafted tools only
}

// methods maps MCP method names to their handlers.
var methods = map[string]func(s *server, req *request) *response{
	"initialize": func(s *server, req *request) *response {
		// MCP protocol uses camelCase (external spec). Build as maps.
		return s.ok(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "ze-mcp", "version": "1.0.0"},
		})
	},
	"notifications/initialized": func(_ *server, _ *request) *response {
		return nil // notification -- no response
	},
	"tools/list": func(s *server, req *request) *response {
		return s.ok(req.ID, map[string]any{"tools": s.allTools()})
	},
	"tools/call": func(s *server, req *request) *response {
		return s.callTool(req)
	},
}

func (s *server) handle(req *request) *response {
	handler, ok := methods[req.Method]
	if !ok {
		return s.fail(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
	return handler(s, req)
}

// toolHandlers maps MCP tool names to their implementations.
var toolHandlers = map[string]func(s *server, args json.RawMessage) map[string]any{
	"ze_announce":     (*server).toolAnnounce,
	"ze_withdraw":     (*server).toolWithdraw,
	"ze_peers":        (*server).toolPeers,
	"ze_peer_control": (*server).toolPeerControl,
	"ze_execute":      (*server).toolExecute,
	"ze_commands":     (*server).toolCommands,
}

// handcraftedNames returns the set of tool names from handcrafted tools.
// Used to filter auto-generated tools and prevent duplicate names.
func handcraftedNames() map[string]bool {
	names := make(map[string]bool, len(toolHandlers))
	for name := range toolHandlers {
		names[name] = true
	}
	return names
}

// allTools returns handcrafted tools plus auto-generated tools from the command registry.
func (s *server) allTools() []map[string]any {
	if s.commands == nil {
		result := make([]map[string]any, len(handcraftedTools))
		copy(result, handcraftedTools)
		return result
	}

	groups := groupCommands(s.commands())
	generated := generateTools(groups, handcraftedNames())

	result := make([]map[string]any, len(handcraftedTools), len(handcraftedTools)+len(generated))
	copy(result, handcraftedTools)
	result = append(result, generated...)
	return result
}

func (s *server) callTool(req *request) *response {
	var params callParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.fail(req.ID, -32602, "invalid params: "+err.Error())
	}

	// Handcrafted tools take priority.
	if handler, ok := toolHandlers[params.Name]; ok {
		return s.ok(req.ID, handler(s, params.Arguments))
	}

	// Try auto-generated tools: look up the command prefix and valid actions.
	if s.commands != nil {
		if prefix, validActions, ok := s.findGeneratedTool(params.Name); ok {
			return s.ok(req.ID, s.dispatchGenerated(prefix, validActions, params.Arguments))
		}
	}

	return s.fail(req.ID, -32602, fmt.Sprintf("unknown tool: %s", params.Name))
}

// findGeneratedTool maps an auto-generated tool name back to its command prefix
// and valid action names. Returns ("", nil, false) if not found.
func (s *server) findGeneratedTool(name string) (string, map[string]bool, bool) {
	skip := handcraftedNames()
	groups := groupCommands(s.commands())
	for _, g := range groups {
		if skip[toolName(g.prefix)] {
			continue
		}
		if toolName(g.prefix) == name {
			valid := make(map[string]bool, len(g.actions))
			for _, a := range g.actions {
				valid[a.name] = true
			}
			return g.prefix, valid, true
		}
	}
	return "", nil, false
}

// noSpaces rejects values containing whitespace or newlines.
// The dispatcher tokenizes by spaces, so embedded spaces would
// split a single value into multiple tokens and corrupt the command.
// Semantic validation (valid IP, valid prefix, etc.) is done by the dispatcher.
func noSpaces(field, value string) error {
	if strings.ContainsAny(value, " \t\n\r") {
		return fmt.Errorf("%s must not contain whitespace: %q", field, value)
	}
	return nil
}

// toolAnnounce announces routes to peers.
func (s *server) toolAnnounce(args json.RawMessage) map[string]any {
	var p struct {
		Peer      string   `json:"peer"`
		Origin    string   `json:"origin"`
		NextHop   string   `json:"next-hop"`
		LocalPref *int     `json:"local-preference"`
		ASPath    []int    `json:"as-path"`
		Community []string `json:"community"`
		Family    string   `json:"family"`
		Prefixes  []string `json:"prefixes"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return errResult("invalid arguments: " + err.Error())
	}
	if p.Family == "" || len(p.Prefixes) == 0 {
		return errResult("family and prefixes are required")
	}
	if p.Peer == "" {
		p.Peer = "*"
	}

	// Validate inputs.
	if err := noSpaces("peer", p.Peer); err != nil {
		return errResult(err.Error())
	}
	if err := noSpaces("family", p.Family); err != nil {
		return errResult(err.Error())
	}
	if p.Origin != "" {
		if err := noSpaces("origin", p.Origin); err != nil {
			return errResult(err.Error())
		}
	}
	if p.NextHop != "" {
		if err := noSpaces("next-hop", p.NextHop); err != nil {
			return errResult(err.Error())
		}
	}
	for _, pfx := range p.Prefixes {
		if err := noSpaces("prefix", pfx); err != nil {
			return errResult(err.Error())
		}
	}
	for _, c := range p.Community {
		if err := noSpaces("community", c); err != nil {
			return errResult(err.Error())
		}
	}

	// Build the text command.
	var cmd strings.Builder
	fmt.Fprintf(&cmd, "peer %s update text", p.Peer)
	if p.Origin != "" {
		fmt.Fprintf(&cmd, " origin %s", p.Origin)
	}
	if p.LocalPref != nil {
		fmt.Fprintf(&cmd, " local-preference %d", *p.LocalPref)
	}
	if len(p.ASPath) > 0 {
		cmd.WriteString(" as-path [")
		for i, asn := range p.ASPath {
			if i > 0 {
				cmd.WriteString(" ")
			}
			fmt.Fprintf(&cmd, "%d", asn)
		}
		cmd.WriteString("]")
	}
	if p.NextHop != "" {
		fmt.Fprintf(&cmd, " next-hop %s", p.NextHop)
	}
	// Use bracketed syntax for multiple communities to avoid parser replacement.
	if len(p.Community) > 0 {
		cmd.WriteString(" community [")
		cmd.WriteString(strings.Join(p.Community, " "))
		cmd.WriteString("]")
	}
	fmt.Fprintf(&cmd, " nlri %s", p.Family)
	for _, pfx := range p.Prefixes {
		fmt.Fprintf(&cmd, " add %s", pfx)
	}

	return s.run(cmd.String())
}

// toolWithdraw withdraws routes from peers.
func (s *server) toolWithdraw(args json.RawMessage) map[string]any {
	var p struct {
		Peer     string   `json:"peer"`
		Family   string   `json:"family"`
		Prefixes []string `json:"prefixes"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return errResult("invalid arguments: " + err.Error())
	}
	if p.Family == "" || len(p.Prefixes) == 0 {
		return errResult("family and prefixes are required")
	}
	if p.Peer == "" {
		p.Peer = "*"
	}
	if err := noSpaces("peer", p.Peer); err != nil {
		return errResult(err.Error())
	}
	if err := noSpaces("family", p.Family); err != nil {
		return errResult(err.Error())
	}
	for _, pfx := range p.Prefixes {
		if err := noSpaces("prefix", pfx); err != nil {
			return errResult(err.Error())
		}
	}

	var cmd strings.Builder
	fmt.Fprintf(&cmd, "peer %s update text nlri %s", p.Peer, p.Family)
	for _, pfx := range p.Prefixes {
		fmt.Fprintf(&cmd, " del %s", pfx)
	}

	return s.run(cmd.String())
}

// toolPeers lists BGP peers and their status.
func (s *server) toolPeers(args json.RawMessage) map[string]any {
	var p struct {
		Peer string `json:"peer"`
	}
	// Peer is optional -- malformed args just list all peers.
	_ = json.Unmarshal(args, &p)

	if p.Peer != "" {
		if err := noSpaces("peer", p.Peer); err != nil {
			return errResult(err.Error())
		}
		return s.run(fmt.Sprintf("peer %s show bgp peer", p.Peer))
	}
	return s.run("peer list")
}

// toolPeerControl manages peer lifecycle (teardown, pause, resume, flush).
func (s *server) toolPeerControl(args json.RawMessage) map[string]any {
	var p struct {
		Peer   string `json:"peer"`
		Action string `json:"action"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return errResult("invalid arguments: " + err.Error())
	}
	if p.Peer == "" || p.Action == "" {
		return errResult("peer and action are required")
	}
	if err := noSpaces("peer", p.Peer); err != nil {
		return errResult(err.Error())
	}

	validActions := map[string]bool{"teardown": true, "pause": true, "resume": true, "flush": true}
	if !validActions[p.Action] {
		return errResult(fmt.Sprintf("invalid action %q (use: teardown, pause, resume, flush)", p.Action))
	}

	return s.run(fmt.Sprintf("peer %s %s", p.Peer, p.Action))
}

// toolExecute runs a raw Ze command (escape hatch for anything not covered by specific tools).
func (s *server) toolExecute(args json.RawMessage) map[string]any {
	var p struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return errResult("invalid arguments: " + err.Error())
	}
	if p.Command == "" {
		return errResult("command is required")
	}
	return s.run(p.Command)
}

// toolCommands lists available Ze commands.
func (s *server) toolCommands(_ json.RawMessage) map[string]any {
	return s.run("command-list --json")
}

// run dispatches a command and returns the result as MCP content.
func (s *server) run(command string) map[string]any {
	output, err := s.dispatch(command)
	if err != nil {
		return errResult(err.Error())
	}
	return textResult(output)
}

func (s *server) ok(id *json.RawMessage, result any) *response {
	return &response{JSONRPC: "2.0", ID: id, Result: result}
}

func (s *server) fail(id *json.RawMessage, code int, msg string) *response {
	return &response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
}

func textResult(s string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": s}},
	}
}

func errResult(msg string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": "Error: " + msg}},
		"isError": true,
	}
}

// Tool definitions. MCP camelCase fields built as maps to avoid kebab-case hook conflicts.
//
//nolint:lll // JSON schemas are long by nature
var handcraftedTools = []map[string]any{
	{
		"name":        "ze_announce",
		"description": "Announce BGP routes to peers. Builds and sends UPDATE messages with the specified attributes and prefixes.",
		"inputSchema": json.RawMessage(`{"type":"object","properties":{"peer":{"type":"string","description":"Peer selector: address, name, or * for all (default: *)"},"origin":{"type":"string","enum":["igp","egp","incomplete"],"description":"ORIGIN attribute"},"next-hop":{"type":"string","description":"Next-hop IP address"},"local-preference":{"type":"integer","description":"LOCAL_PREF value"},"as-path":{"type":"array","items":{"type":"integer"},"description":"AS_PATH as list of ASNs"},"community":{"type":"array","items":{"type":"string"},"description":"Communities (e.g. 65000:100)"},"family":{"type":"string","description":"Address family (e.g. ipv4/unicast, ipv6/unicast)"},"prefixes":{"type":"array","items":{"type":"string"},"description":"Prefixes to announce (e.g. 10.0.0.0/24)"}},"required":["family","prefixes"]}`),
	},
	{
		"name":        "ze_withdraw",
		"description": "Withdraw BGP routes from peers. Sends UPDATE messages removing the specified prefixes.",
		"inputSchema": json.RawMessage(`{"type":"object","properties":{"peer":{"type":"string","description":"Peer selector: address, name, or * for all (default: *)"},"family":{"type":"string","description":"Address family (e.g. ipv4/unicast)"},"prefixes":{"type":"array","items":{"type":"string"},"description":"Prefixes to withdraw (e.g. 10.0.0.0/24)"}},"required":["family","prefixes"]}`),
	},
	{
		"name":        "ze_peers",
		"description": "List BGP peers with their current state (Idle, Connect, OpenSent, Established, etc.), ASN, uptime, and counters.",
		"inputSchema": json.RawMessage(`{"type":"object","properties":{"peer":{"type":"string","description":"Optional peer selector for detailed view (address or name). Omit for summary list."}}}`),
	},
	{
		"name":        "ze_peer_control",
		"description": "Control BGP peer lifecycle: tear down session, pause/resume updates, or flush routes.",
		"inputSchema": json.RawMessage(`{"type":"object","properties":{"peer":{"type":"string","description":"Peer selector: address, name, or * for all"},"action":{"type":"string","enum":["teardown","pause","resume","flush"],"description":"Action to perform"}},"required":["peer","action"]}`),
	},
	{
		"name":        "ze_execute",
		"description": "Execute any Ze command (escape hatch). Use ze_commands to discover available commands. Prefer the specific tools (ze_announce, ze_withdraw, ze_peers, ze_peer_control) when possible.",
		"inputSchema": json.RawMessage(`{"type":"object","properties":{"command":{"type":"string","description":"Ze command string"}},"required":["command"]}`),
	},
	{
		"name":        "ze_commands",
		"description": "List all available Ze commands with descriptions.",
		"inputSchema": json.RawMessage(`{"type":"object","properties":{}}`),
	},
}
