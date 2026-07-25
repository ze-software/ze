// Design: docs/architecture/mcp/overview.md -- Tool dispatch and task management
// Related: streamable.go -- HTTP transport, streamable_auth.go -- authentication

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// runMethod runs a JSON-RPC method handler to completion synchronously. ctx
// is the originating HTTP request's context; tool handlers propagate it into
// blocking calls (notably session.Elicit) so a client disconnect unblocks
// the handler via ctx.Done().
func (s *Streamable) runMethod(ctx context.Context, sess *session, req *request, remoteAddr string) *response {
	switch req.Method {
	case "notifications/initialized":
		return nil
	case "tools/list":
		return s.ok(req.ID, map[string]any{"tools": s.allTools()})
	case "tools/call":
		return s.callTool(ctx, req, sess, remoteAddr)
	case "tasks/list":
		return s.tasksList(sess, req)
	case "tasks/get":
		return s.tasksGet(sess, req)
	case "tasks/result":
		return s.tasksResult(sess, req)
	case "tasks/cancel":
		return s.tasksCancel(sess, req)
	case "resources/list":
		return s.resourcesList(sess, req)
	case "resources/read":
		return s.resourcesRead(sess, req)
	default:
		return s.fail(req.ID, -32601, "method not found: "+req.Method)
	}
}

// allTools returns the provider's tools (Provider mode), or the combined
// handcrafted + auto-generated tool list (command-registry mode).
func (s *Streamable) allTools() []map[string]any {
	if s.cfg.Provider != nil {
		return s.cfg.Provider.Tools()
	}
	if s.cfg.Commands == nil {
		result := make([]map[string]any, len(handcraftedTools))
		copy(result, handcraftedTools)
		return result
	}
	groups := groupCommands(s.cfg.Commands())
	generated := generateTools(groups, handcraftedNames())
	result := make([]map[string]any, len(handcraftedTools), len(handcraftedTools)+len(generated))
	copy(result, handcraftedTools)
	result = append(result, generated...)
	return result
}

// callTool executes a tools/call request. ctx is the originating HTTP
// request's context; it flows onto runner.ctx so tool handlers that call
// blocking ops like session.Elicit see client disconnect. sess is the
// Streamable session bound to the active POST; nil means the POST has no
// session context and nil-aware handlers degrade gracefully.
func (s *Streamable) callTool(ctx context.Context, req *request, sess *session, remoteAddr string) *response {
	var params callParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.fail(req.ID, -32602, "invalid params: "+err.Error())
	}
	// Provider mode: delegate directly; provider tools never support
	// task-augmented calls (the legacy handler had no tasks either).
	if s.cfg.Provider != nil {
		var tb textbuf.Buffer
		if params.Task != nil {
			return s.fail(req.ID, -32602, tb.Str("tool ").Str(params.Name).Str(" does not support task-augmented calls").String())
		}
		result := s.cfg.Provider.CallTool(params.Name, params.Arguments)
		if result == nil {
			return s.fail(req.ID, -32602, tb.Str("unknown tool: ").Str(params.Name).String())
		}
		return s.ok(req.ID, result)
	}
	ts := s.lookupTaskSupport(params.Name)
	if params.Task != nil {
		if ts == TaskSupportForbidden {
			return s.fail(req.ID, -32602, "tool "+params.Name+" does not support task-augmented calls")
		}
		return s.createTask(req, sess, remoteAddr, params)
	}
	if ts == TaskSupportRequired {
		return s.fail(req.ID, -32602, "tool "+params.Name+" requires task-augmented call (pass task: {})")
	}
	var username string
	if sess != nil {
		username = sess.Identity().Name
	}
	runner := &server{dispatch: s.cfg.Dispatch, commands: s.cfg.Commands, session: sess, ctx: ctx, username: username, remoteAddr: remoteAddr}
	if handler, ok := toolHandlers[params.Name]; ok {
		return s.ok(req.ID, handler(runner, params.Arguments))
	}
	if s.cfg.Commands != nil {
		if prefix, validActions, ok := s.findGeneratedTool(params.Name); ok {
			return s.ok(req.ID, runner.dispatchGenerated(prefix, validActions, params.Arguments))
		}
	}
	return s.fail(req.ID, -32602, "unknown tool: "+params.Name)
}

// lookupTaskSupport returns the taskSupport level for a tool by name.
// Handcrafted tools default to optional.
func (s *Streamable) lookupTaskSupport(name string) TaskSupportLevel {
	if s.cfg.Commands == nil {
		return TaskSupportOptional
	}
	skip := handcraftedNames()
	if skip[name] {
		return TaskSupportOptional
	}
	groups := groupCommands(s.cfg.Commands())
	for _, g := range groups {
		if skip[toolName(g.prefix)] {
			continue
		}
		if toolName(g.prefix) == name {
			return g.taskSupport
		}
	}
	return TaskSupportOptional
}

// findGeneratedTool maps an auto-generated tool name back to its command prefix.
func (s *Streamable) findGeneratedTool(name string) (string, map[string]bool, bool) {
	skip := handcraftedNames()
	groups := groupCommands(s.cfg.Commands())
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

// parseTaskID extracts the taskId field from JSON-RPC params (MCP camelCase).
func parseTaskID(raw json.RawMessage) (string, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", err
	}
	id, _ := m["taskId"].(string)
	if id == "" {
		return "", errors.New("missing or empty taskId")
	}
	return id, nil
}

func (s *Streamable) tasksList(sess *session, req *request) *response {
	if sess == nil || !sess.ClientSupportsTasks() {
		return s.fail(req.ID, -32601, "method not found: tasks/list")
	}
	identity := sess.Identity().Name
	tasks := s.tasks.List(identity)
	items := make([]map[string]any, len(tasks))
	for i, t := range tasks {
		items[i] = t.ToWire()
	}
	return s.ok(req.ID, map[string]any{"tasks": items})
}

func (s *Streamable) tasksGet(sess *session, req *request) *response {
	if sess == nil || !sess.ClientSupportsTasks() {
		return s.fail(req.ID, -32601, "method not found: tasks/get")
	}
	taskID, err := parseTaskID(req.Params)
	if err != nil {
		return s.fail(req.ID, -32602, "invalid params: "+err.Error())
	}
	info, err := s.tasks.Get(sess.Identity().Name, taskID)
	if err != nil {
		return s.fail(req.ID, -32602, err.Error())
	}
	return s.ok(req.ID, info.ToWire())
}

func (s *Streamable) tasksResult(sess *session, req *request) *response {
	if sess == nil || !sess.ClientSupportsTasks() {
		return s.fail(req.ID, -32601, "method not found: tasks/result")
	}
	taskID, err := parseTaskID(req.Params)
	if err != nil {
		return s.fail(req.ID, -32602, "invalid params: "+err.Error())
	}
	result, err := s.tasks.Result(sess.Identity().Name, taskID)
	if err != nil {
		return s.fail(req.ID, -32602, err.Error())
	}
	return s.ok(req.ID, result)
}

func (s *Streamable) tasksCancel(sess *session, req *request) *response {
	if sess == nil || !sess.ClientSupportsTasks() {
		return s.fail(req.ID, -32601, "method not found: tasks/cancel")
	}
	taskID, err := parseTaskID(req.Params)
	if err != nil {
		return s.fail(req.ID, -32602, "invalid params: "+err.Error())
	}
	state, err := s.tasks.Cancel(sess.Identity().Name, taskID)
	if err != nil {
		return s.fail(req.ID, -32602, err.Error())
	}
	return s.ok(req.ID, map[string]any{"taskId": taskID, "status": state.String()})
}

// createTask handles tools/call with params.task present. It validates the
// tool exists, registers a task, launches a worker, and returns CreateTaskResult.
func (s *Streamable) createTask(req *request, sess *session, remoteAddr string, params callParams) *response {
	if sess == nil || !sess.ClientSupportsTasks() {
		return s.fail(req.ID, -32602, "client did not declare tasks capability")
	}

	// Resolve tool BEFORE allocating task entry (finding #1: avoid wasting
	// concurrency slots on unknown tools).
	handler, isHandcrafted := toolHandlers[params.Name]
	var prefix string
	var validActions map[string]bool
	if !isHandcrafted && s.cfg.Commands != nil {
		var found bool
		prefix, validActions, found = s.findGeneratedTool(params.Name)
		if !found {
			return s.fail(req.ID, -32602, "unknown tool: "+params.Name)
		}
	} else if !isHandcrafted {
		return s.fail(req.ID, -32602, "unknown tool: "+params.Name)
	}

	identity := sess.Identity().Name

	var ttl time.Duration
	if params.Task != nil {
		var m map[string]any
		if err := json.Unmarshal(params.Task, &m); err == nil {
			if ms, ok := m["ttl"].(float64); ok && ms > 0 {
				ttl = time.Duration(ms) * time.Millisecond
			}
		}
	}

	taskID, taskCtx, _, err := s.tasks.Create(identity, sess.ID(), ttl)
	if err != nil {
		return s.fail(req.ID, -32602, err.Error())
	}

	// Capture resolved dispatch path so the worker never calls
	// groupCommands again (finding #3: avoid O(N) per-request).
	capturedHandler := handler
	capturedPrefix := prefix
	capturedActions := validActions
	capturedArgs := params.Arguments

	work := func(wCtx context.Context) (map[string]any, error) {
		runner := &server{
			dispatch:   s.cfg.Dispatch,
			commands:   s.cfg.Commands,
			session:    sess,
			ctx:        wCtx,
			username:   identity,
			remoteAddr: remoteAddr,
		}
		if isHandcrafted {
			return capturedHandler(runner, capturedArgs), nil
		}
		return runner.dispatchGenerated(capturedPrefix, capturedActions, capturedArgs), nil
	}

	runTaskWorker(taskCtx, s.tasks, sess, taskID, work)

	return s.ok(req.ID, map[string]any{
		"taskId": taskID,
		"status": TaskWorking.String(),
	})
}

func (s *Streamable) ok(id *json.RawMessage, result any) *response {
	return &response{JSONRPC: "2.0", ID: id, Result: result}
}

func (s *Streamable) fail(id *json.RawMessage, code int, msg string) *response {
	return &response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
}

// writeJSONResponse writes a single JSON-RPC response with Content-Type JSON.
func writeJSONResponse(w http.ResponseWriter, v any) {
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

// acceptsEventStream reports whether Accept permits text/event-stream.
func acceptsEventStream(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return false
	}
	for part := range strings.SplitSeq(accept, ",") {
		mediaType := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if mediaType == mimeEventStream || mediaType == "*/*" {
			return true
		}
	}
	return false
}

// parseInitializeProtocolVersion extracts protocolVersion from an initialize
// request.
//
//   - Missing / empty / unparseable params -> server's preferred ProtocolVersion.
//   - Known value -> echo it back (the client asked, the server honors).
//   - Unknown value -> errUnsupportedProtocolVersion.
//
// MCP uses camelCase externally; parse via generic map to avoid struct tags.
func parseInitializeProtocolVersion(req *request) (string, error) {
	if len(req.Params) == 0 {
		return ProtocolVersion, nil
	}
	var p map[string]any
	if err := json.Unmarshal(req.Params, &p); err != nil {
		// Malformed params is not a version mismatch; let the client initialize
		// at the server's preferred version instead of 400-ing them out.
		return ProtocolVersion, nil //nolint:nilerr // intentional permissive fallback
	}
	raw, present := p["protocolVersion"]
	if !present {
		return ProtocolVersion, nil
	}
	v, ok := raw.(string)
	if !ok || v == "" {
		return ProtocolVersion, nil
	}
	if _, known := supportedProtocolVersions[v]; !known {
		return "", fmt.Errorf("%w: %q", errUnsupportedProtocolVersion, v)
	}
	return v, nil
}
