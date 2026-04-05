// Design: docs/guide/mcp/overview.md -- MCP JSON-RPC HTTP handler
// Detail: tools.go -- auto-generated MCP tools from command registry

// Package mcp provides an HTTP handler that speaks MCP (Model Context Protocol)
// JSON-RPC, wrapping Ze's command dispatcher to let AI assistants control BGP.
//
// All tools are auto-generated from the command registry at tools/list time.
// New YANG commands are automatically exposed without modifying this package.
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

// toolHandlers maps handcrafted MCP tool names to their implementations.
// ze_execute is a raw command dispatch escape hatch (equivalent to ze_system dispatch).
var toolHandlers = map[string]func(s *server, args json.RawMessage) map[string]any{
	"ze_execute": func(s *server, args json.RawMessage) map[string]any {
		var input struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(args, &input); err != nil {
			return errResult("invalid arguments: " + err.Error())
		}
		if s.dispatch == nil {
			return errResult("dispatcher not available")
		}
		result, err := s.dispatch(input.Command)
		if err != nil {
			return errResult(err.Error())
		}
		return textResult(result)
	},
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

// handcraftedTools defines tool schemas for handcrafted tools.
var handcraftedTools = []map[string]any{
	{
		"name":        "ze_execute",
		"description": "Execute a ze CLI command and return the result",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The ze command to execute (e.g., 'peer list', 'show bgp summary')",
				},
			},
			"required": []string{"command"},
		},
	},
}
