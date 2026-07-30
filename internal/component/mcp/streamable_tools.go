// Design: docs/architecture/mcp/overview.md -- Tool dispatch and task management
// Related: streamable.go -- HTTP transport, streamable_auth.go -- authentication
// Related: discover.go -- server/discover result assembly and server identity
// Related: caching.go -- the method table runMethod stamps cache hints from
// Related: apps.go -- the MCP Apps gate allTools applies to every descriptor
// Related: mrtr.go -- the input-required guard runMethod applies, and the ze_execute descriptor gate
// Related: tasks.go -- the task registry createTask and the tasks/* handlers drive

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// JSON-RPC method names this server knows. Named once so the dispatch switch,
// the Mcp-Name header rule and the tests cannot drift apart.
// The three tasks/* methods are the io.modelcontextprotocol/tasks extension's
// full client-facing surface for a polling server.
//
// tasks/list and tasks/result are deliberately ABSENT. MCP 2026-07-28 changelog
// Major change 6: the redesigned extension "replaces the blocking tasks/result
// method with polling via tasks/get and a new tasks/update for client-to-server
// input, removes tasks/list". Both now fall through to the default arm and are
// answered 404 with -32601, which is what a client probing an unimplemented
// method must see.
const (
	methodServerDiscover = "server/discover"
	methodToolsList      = "tools/list"
	methodToolsCall      = "tools/call"
	methodTasksGet       = "tasks/get"
	methodTasksUpdate    = "tasks/update"
	methodTasksCancel    = "tasks/cancel"
	methodResourcesList  = "resources/list"
	methodResourcesRead  = "resources/read"
	// methodPromptsGet is not dispatched by this server. It is named because
	// the Mcp-Name header rule covers it, and header validation runs before
	// dispatch. A prompts/get POST is therefore header-checked on its way to
	// a 404.
	methodPromptsGet = "prompts/get"
	// initializeMethod is the handshake MCP 2026-07-28 removed. It is named
	// only so a client that still sends it receives a diagnostic naming the
	// protocol version this server does speak.
	initializeMethod = "initialize"
)

// JSON-RPC error codes.
//
// -32700, -32601, -32602 and -32603 are the standard JSON-RPC 2.0 codes.
// -32020, -32021 and -32022 come from the sub-range MCP 2026-07-28 reserves for
// itself (basic/index Section "Error Codes": "-32020 to -32099 — reserved for
// the MCP specification. ... Implementations MUST NOT emit any code from this
// sub-range that is not defined by this specification and MUST use defined
// codes only with their specified meanings.").
const (
	rpcParseError     = -32700
	rpcMethodNotFound = -32601
	rpcInvalidParams  = -32602
	rpcInternalError  = -32603
	// rpcHeaderMismatch: the HTTP headers do not match the corresponding
	// values in the request body, or required headers are missing/malformed.
	rpcHeaderMismatch = -32020
	// rpcMissingRequiredClientCapability: processing the request requires a
	// capability the client did not declare in clientCapabilities.
	rpcMissingRequiredClientCapability = -32021
	// rpcUnsupportedProtocolVersion: the request's protocol version is unknown
	// to the server or is one it has chosen not to implement.
	rpcUnsupportedProtocolVersion = -32022
)

// headerMismatchLead is the stable leading phrase of every -32020 message, so
// log scanners and clients can match one string rather than a family of them.
const headerMismatchLead = "header mismatch"

// resultTypeComplete is the ResultType discriminator for a finished result.
//
// MCP 2026-07-28 basic/index Section "Result Responses": "The result MUST
// include a resultType field to indicate the type of the result."
//
// It is the default every handler gets. The one other legal value,
// "input_required", is set by the handler itself (mrtr.go) and preserved by
// ok().
const resultTypeComplete = "complete"

// resultTypeTask is the ResultType discriminator the tasks extension adds for
// a CreateTaskResult. The call was accepted and runs in the background, and
// the result carries a handle rather than the work's output.
//
// MCP 2026-07-28 basic/index Section "ResultType": "Extensions MAY add
// additional ResultType values. The set of supported ResultType values MUST be
// created from the set defined in the core protocol and include any additional
// values of supported extensions that are advertised via capabilities."
//
// This value is therefore only ever emitted to a client that declared the
// extension. The gate in callTool is what makes that true.
const resultTypeTask = "task"

// CreateTaskResult wire keys. MCP is camelCase on the wire, so these are the
// extension's spellings verbatim rather than Ze names.
const (
	resultKeyTaskID         = "taskId"
	resultKeyStatus         = "status"
	resultKeyPollIntervalMs = "pollIntervalMs"
)

// requestScope is the per-request protocol state every handler runs against.
//
// Value type, built once in handlePOST after authentication and copied into
// every handler that needs it. It is deliberately not a pointer, and it is
// deliberately not optional. The compiler forces each handler to receive an
// identity and a capability set. No nil case is therefore left for a handler
// to forget, and no "maybe absent" context is left to dereference. Its zero value
// denies every gated capability (ai/rules/fail-closed-guards.md).
type requestScope struct {
	// Identity is the authenticated principal. A zero Identity means
	// "anonymous under auth-mode none". It never means "not authenticated",
	// because an unauthenticated request is rejected before a scope exists.
	Identity Identity
	// Capabilities is what the client declared for THIS request.
	Capabilities clientCapabilities
	// ProtocolVersion is the version this request declared, already checked
	// against the supported set.
	ProtocolVersion string
	// ClientInfo is the client's self-reported identity. Unverified, so it is
	// carried for display and logging only and never reaches an authorization
	// or ownership decision.
	ClientInfo clientInfo
}

// runMethod runs a JSON-RPC method handler to completion synchronously.
//
// ctx is the originating HTTP request's context. It reaches the command
// dispatcher through server.context (tools.go), so a client disconnect
// unblocks a dispatch that selects on ctx.Done(). MCP 2026-07-28 makes stream
// close the cancellation signal for a request. It also says the server SHOULD
// stop work on a canceled request "as soon as practical", which is the
// obligation that propagation discharges. scope carries the request's
// authenticated identity and declared capabilities by value.
//
// Two things are applied HERE, on the way out, rather than by each handler.
//
// The first is the cache hints. Membership of cacheTTLByMethod (caching.go) is
// the whole decision. A method added to the switch below can therefore neither
// silently miss the stamp nor silently gain it. tools/call must carry none in
// either result shape.
//
// The second is the input-required guard (mrtr.go), which refuses to let an
// interim result leave on a method the specification forbids it on. The guard
// runs FIRST, so an illegal interim result becomes an error before any cache
// hint is considered for it.
func (s *Streamable) runMethod(ctx context.Context, scope requestScope, req *request, remoteAddr string) *response {
	return stampCacheHints(req.Method, s.guardInputRequired(req.Method, s.dispatchMethod(ctx, scope, req, remoteAddr)))
}

// dispatchMethod routes one JSON-RPC method to its handler. Split from
// runMethod so the cache-hint stamp above has exactly one place to sit.
func (s *Streamable) dispatchMethod(ctx context.Context, scope requestScope, req *request, remoteAddr string) *response {
	switch req.Method {
	case methodServerDiscover:
		return s.serverDiscover(req)
	case methodToolsList:
		return s.ok(req.ID, map[string]any{"tools": s.allTools(scope.Capabilities)})
	case methodToolsCall:
		return s.callTool(ctx, req, scope, remoteAddr)
	case methodTasksGet:
		return s.tasksGet(scope, req)
	case methodTasksUpdate:
		return s.tasksUpdate(scope, req)
	case methodTasksCancel:
		return s.tasksCancel(scope, req)
	case methodResourcesList:
		return s.resourcesList(req)
	case methodResourcesRead:
		return s.resourcesRead(req)
	case initializeMethod:
		// An initialize POST that carried conformant headers reaches dispatch
		// rather than header validation. MCP 2026-07-28 basic/versioning asks a
		// modern-only server to "name the protocol versions it supports in any
		// error it returns to an `initialize` request", so this 404 names them
		// too.
		return s.fail(req.ID, rpcMethodNotFound, initializeEraError("method not found").Error())
	default:
		var tb textbuf.Buffer
		return s.fail(req.ID, rpcMethodNotFound, tb.Str("method not found: ").Str(req.Method).String())
	}
}

// allTools returns the provider's tools (Provider mode), or the combined
// handcrafted + auto-generated tool list (command-registry mode).
//
// caps is the requesting client's declared capability set. It is a parameter
// rather than a field, so the compiler forces every caller to decide what the
// client supports. Two gates read it, and both are applied here rather than at
// descriptor construction, so one call site covers every origin:
//
//   - gateUIMeta (apps.go) strips `_meta.ui` from every descriptor when the
//     client did not declare the MCP Apps extension. It is applied to the
//     ASSEMBLED list, so it covers provider descriptors as well as generated
//     ones.
//   - gateExecuteCommandRequired (mrtr.go) marks ze_execute's `command`
//     argument required for a client that did not declare form-mode
//     elicitation. That is precisely the client the server answers with the
//     missing-argument error rather than an input request. It is applied to the
//     handcrafted descriptors only. A ToolProvider owns its own handlers, so a
//     same-named tool there is not the handler this schema describes.
//
// The order is part of the contract: handcrafted tools first, then generated
// tools sorted by command prefix. MCP 2026-07-28 server/tools says servers
// SHOULD "return tools in a deterministic order (i.e., the same ordering across
// requests when the underlying set of tools has not changed)".
func (s *Streamable) allTools(caps clientCapabilities) []map[string]any {
	if s.cfg.Provider != nil {
		return gateUIMeta(s.cfg.Provider.Tools(), caps.UIApps)
	}
	handcrafted := gateExecuteCommandRequired(handcraftedTools, caps.ElicitForm)
	if s.cfg.Commands == nil {
		result := make([]map[string]any, len(handcrafted))
		copy(result, handcrafted)
		return gateUIMeta(result, caps.UIApps)
	}
	groups := groupCommands(s.cfg.Commands())
	generated := generateTools(groups, handcraftedNames())
	result := make([]map[string]any, len(handcrafted), len(handcrafted)+len(generated))
	copy(result, handcrafted)
	result = append(result, generated...)
	return gateUIMeta(result, caps.UIApps)
}

// callTool executes a tools/call request.
//
// ctx is the originating HTTP request's context. It flows onto runner.ctx, and
// from there into every dispatch through server.context (tools.go). A client
// disconnect therefore reaches the command dispatcher. scope supplies the
// authenticated identity the dispatched command runs as.
func (s *Streamable) callTool(ctx context.Context, req *request, scope requestScope, remoteAddr string) *response {
	var tb textbuf.Buffer
	var params callParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.fail(req.ID, rpcInvalidParams, tb.Str("invalid params: ").Err(err).String())
	}
	// Provider mode: delegate directly. A provider supplies its own tool set
	// with no YANG behind it. No descriptor can therefore carry
	// `ze:task-support`, and the server-directed rule below has nothing to
	// read.
	if s.cfg.Provider != nil {
		result := s.cfg.Provider.CallTool(params.Name, params.Arguments)
		if result == nil {
			return s.fail(req.ID, rpcInvalidParams, tb.Reset().Str("unknown tool: ").Str(params.Name).String())
		}
		return s.ok(req.ID, result)
	}

	// The server-directed eligibility decision (D-1). The client no longer opts
	// a call into task execution. This server decides from the command's
	// `ze:task-support` annotation. And the client only says once, per request,
	// whether it can understand a task handle at all.
	//
	// MCP 2026-07-28 changelog Major change 6: the tasks extension "allows
	// servers to return task handles unsolicited without per-request opt-in".
	//
	// MCP 2026-07-28 basic/versioning Section "Extension Negotiation": "If one
	// party supports an extension but the other does not, the supporting party
	// MUST either revert to core protocol behavior or reject the request with an
	// appropriate error."
	//
	// Ze reverts. A client that did not declare the tasks extension gets its
	// answer synchronously rather than an error (D-2). The extension is an
	// optimization over a synchronous call, never a precondition for the work.
	// A refusal would therefore make the 9 annotated commands unreachable to
	// every client that has not adopted an optional extension.
	//
	// Both halves of the guard matter, and they fail closed in opposite
	// directions. `forbidden` and `optional` never produce a task, whatever the
	// client declared. And `required` produces one ONLY for a client that
	// declared the extension. There is no path to a task handle that skips
	// either check.
	if s.lookupTaskSupport(params.Name) == TaskSupportRequired && scope.Capabilities.Tasks {
		return s.createTask(req, scope, remoteAddr, params)
	}

	runner := &server{
		dispatch:   s.cfg.Dispatch,
		commands:   s.cfg.Commands,
		ctx:        ctx,
		username:   scope.Identity.Name,
		remoteAddr: remoteAddr,
		// The capability gate is read ONCE, here, from this request's `_meta`,
		// and handed to the handler. No handler re-parses it. inputResponses
		// carries the client's answers to a previous InputRequiredResult, and
		// it is the only thing that connects the two attempts. Nothing is
		// retained server-side between them.
		caps:           scope.Capabilities,
		inputResponses: decodeInputResponses(req.Params),
	}
	if handler, ok := toolHandlers[params.Name]; ok {
		return s.ok(req.ID, handler(runner, params.Arguments))
	}
	if s.cfg.Commands != nil {
		if prefix, validActions, ok := s.findGeneratedTool(params.Name); ok {
			return s.ok(req.ID, runner.dispatchGenerated(prefix, validActions, params.Arguments))
		}
	}
	return s.fail(req.ID, rpcInvalidParams, tb.Reset().Str("unknown tool: ").Str(params.Name).String())
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

// findGeneratedTool maps an auto-generated tool name back to its command prefix
// and its action set. The returned map's KEYS are the valid action names (an
// action absent from the map is rejected); each VALUE says whether that
// command takes a peer selector, which dispatchGenerated needs to decide
// whether a `peer` argument is accepted and where its value is spliced in.
func (s *Streamable) findGeneratedTool(name string) (string, map[string]bool, bool) {
	skip := handcraftedNames()
	groups := groupCommands(s.cfg.Commands())
	for _, g := range groups {
		if skip[toolName(g.prefix)] {
			continue
		}
		if toolName(g.prefix) == name {
			actionSelector := make(map[string]bool, len(g.actions))
			for _, a := range g.actions {
				// Same predicate buildToolDef advertises with, so a tool never
				// offers a `peer` argument its dispatch path would refuse.
				actionSelector[a.name] = actionAcceptsPeer(a)
			}
			return g.prefix, actionSelector, true
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

// tasksUpdate answers tasks/update: the client-to-server direction the tasks
// extension adds for a task that asked for more input.
//
// MCP 2026-07-28 ext-tasks: a server "MUST ignore unknown or already-satisfied
// inputResponses keys" and acknowledge with an empty result.
//
// For Ze every key is unknown BY CONSTRUCTION: no Ze task can raise
// `inputRequests` (see the comment on TaskState in task_state.go), so there is
// nothing outstanding for a response to satisfy. The handler therefore acts on
// exactly one thing, the `taskId`, and it ownership-checks that id first. The
// `inputResponses` payload is attacker-controlled and is discarded unread. That
// is safe precisely because nothing consumes it. If a Ze task ever gains the
// ability to elicit, this becomes a real validation requirement and must be
// written then rather than assumed now.
//
// This method is implemented rather than stubbed on purpose. A server that
// advertises the extension in server/discover and refuses one of its methods
// claims a shape it does not speak (ai/rules/no-parking.md).
func (s *Streamable) tasksUpdate(scope requestScope, req *request) *response {
	if !scope.Capabilities.Tasks {
		return s.failMissingTasksCapability(req.ID)
	}
	taskID, err := parseTaskID(req.Params)
	if err != nil {
		var tb textbuf.Buffer
		return s.fail(req.ID, rpcInvalidParams, tb.Str("invalid params: ").Err(err).String())
	}
	// Ownership first. A foreign or unknown taskId is refused identically, so
	// the reply cannot be used to probe for another principal's task ids.
	if _, err := s.tasks.Get(scope.Identity.Name, taskID); err != nil {
		return s.fail(req.ID, rpcInvalidParams, err.Error())
	}
	return s.ok(req.ID, map[string]any{})
}

func (s *Streamable) tasksGet(scope requestScope, req *request) *response {
	if !scope.Capabilities.Tasks {
		return s.failMissingTasksCapability(req.ID)
	}
	taskID, err := parseTaskID(req.Params)
	if err != nil {
		var tb textbuf.Buffer
		return s.fail(req.ID, rpcInvalidParams, tb.Str("invalid params: ").Err(err).String())
	}
	info, err := s.tasks.Get(scope.Identity.Name, taskID)
	if err != nil {
		return s.fail(req.ID, rpcInvalidParams, err.Error())
	}
	return s.ok(req.ID, info.toWire())
}

// tasksCancel answers tasks/cancel: cooperative cancellation of a task the
// caller owns.
//
// MCP 2026-07-28 ext-tasks, server guidance: "Handle tasks/cancel --
// Acknowledge cancellation requests with an empty result." The acknowledgement
// is therefore EMPTY apart from the two envelope fields ok() stamps on every
// result, exactly like tasksUpdate.
//
// It deliberately does not report the resulting state, even though
// taskRegistry.Cancel returns one. Cancellation is cooperative. The worker's
// context is canceled and the entry is marked, but a worker already past its
// last cancellation check still runs to completion. A status read at
// acknowledgement time is therefore a snapshot, and it can be stale before the
// client parses it.
//
// A reported state would invite a client to treat the ack as the final word.
// tasks/get is the method that answers "what state is it in now", and it is
// the one whose answer is fresh when it is read.
func (s *Streamable) tasksCancel(scope requestScope, req *request) *response {
	if !scope.Capabilities.Tasks {
		return s.failMissingTasksCapability(req.ID)
	}
	taskID, err := parseTaskID(req.Params)
	if err != nil {
		var tb textbuf.Buffer
		return s.fail(req.ID, rpcInvalidParams, tb.Str("invalid params: ").Err(err).String())
	}
	// Ownership first, and the state Cancel reports is deliberately discarded.
	// A foreign or unknown taskId is refused identically to an owned one that
	// does not exist. The reply therefore cannot be used to probe for another
	// principal's task ids.
	if _, err := s.tasks.Cancel(scope.Identity.Name, taskID); err != nil {
		return s.fail(req.ID, rpcInvalidParams, err.Error())
	}
	return s.ok(req.ID, map[string]any{})
}

// createTask registers a task, launches its worker and returns the extension's
// CreateTaskResult. Reached only from callTool, and only for a `required` tool
// called by a client that declared the tasks extension.
//
// The capability check is repeated here rather than trusted from the caller.
// This is the function that mints a task handle, so it is the last place the
// guard can sit and still be the guard. A future second call site that forgot
// the check would otherwise give a task to a client that cannot read one
// (R-2, ai/rules/fail-closed-guards.md).
func (s *Streamable) createTask(req *request, scope requestScope, remoteAddr string, params callParams) *response {
	if !scope.Capabilities.Tasks {
		return s.failMissingTasksCapability(req.ID)
	}

	// Resolve the tool BEFORE the task entry is allocated (finding #1: do not
	// waste concurrency slots on unknown tools).
	handler, isHandcrafted := toolHandlers[params.Name]
	var prefix string
	var validActions map[string]bool
	if !isHandcrafted && s.cfg.Commands != nil {
		var found bool
		prefix, validActions, found = s.findGeneratedTool(params.Name)
		if !found {
			var tb textbuf.Buffer
			return s.fail(req.ID, rpcInvalidParams, tb.Str("unknown tool: ").Str(params.Name).String())
		}
	} else if !isHandcrafted {
		var tb textbuf.Buffer
		return s.fail(req.ID, rpcInvalidParams, tb.Str("unknown tool: ").Str(params.Name).String())
	}

	identity := scope.Identity.Name

	// The TTL is the server's, not the client's. The client-requested TTL
	// branch died with `params.task` (D-1). With no per-call opt-in there is no
	// per-call field to carry it, and retention is a server-side resource
	// decision in any case. The one TTL input is now TaskRegistryConfig.TTL,
	// clamped once at construction (newTaskRegistry, tasks.go).
	taskID, taskCtx, _, err := s.tasks.Create(identity)
	if err != nil {
		return s.fail(req.ID, rpcInvalidParams, err.Error())
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
			ctx:        wCtx,
			username:   identity,
			remoteAddr: remoteAddr,
			// caps and inputResponses are deliberately left zero, and this is
			// what makes the `input_required` task state unreachable (D-4).
			//
			// A task worker finishes long after its tools/call returned. An
			// InputRequiredResult produced here would therefore be stored as
			// the task's RESULT and returned on a later tasks/get. A zero
			// capability set makes any handler that would elicit take the
			// missing-argument path instead. No task can therefore raise
			// `inputRequests`, and no task can enter `input_required`. Read the
			// TaskState comment in task_state.go for the trigger that would
			// reopen this.
		}
		if isHandcrafted {
			return capturedHandler(runner, capturedArgs), nil
		}
		return runner.dispatchGenerated(capturedPrefix, capturedActions, capturedArgs), nil
	}

	runTaskWorker(taskCtx, s.tasks, taskID, work)

	// The entry is registered and the worker launched BEFORE this response is
	// built. A client that polls the instant it reads the handle therefore
	// always finds the task. A handle that arrived before the registry knew
	// about it would make a legitimate immediate poll indistinguishable from a
	// forged id.
	//
	// pollIntervalMs is derived from the TTL rather than fixed (D-6). A
	// constant larger than half the retention window is too long. A client that
	// obeys such a hint sleeps past a terminal result and finds it already
	// swept. The derivation keeps `pollIntervalMs <= ttlMs/2` true for every
	// legal TTL.
	ttlMs, pollMs := s.tasks.retentionHints()
	return s.ok(req.ID, map[string]any{
		resultTypeKey:           resultTypeTask,
		resultKeyTaskID:         taskID,
		resultKeyStatus:         TaskWorking.String(),
		resultKeyTTLMs:          ttlMs,
		resultKeyPollIntervalMs: pollMs,
	})
}

// ok wraps a successful method result in a JSON-RPC response. It stamps the
// two envelope fields this revision requires on EVERY result: the resultType
// discriminator and the server's identity under `_meta`. Every successful path
// in this server returns through here, so both are emitted from one site
// rather than per method.
//
// The caller's map is copied rather than stamped in place. tasksResult returns
// the map the registry stored, and a mutation would persist envelope fields
// into registry state.
func (s *Streamable) ok(id *json.RawMessage, result map[string]any) *response {
	out := make(map[string]any, len(result)+2)
	maps.Copy(out, result)
	// A handler that already chose a discriminator keeps it. Everything else is
	// complete. Two handlers choose: mrtr.go sets "input_required" on an interim
	// result, and createTask sets "task" on a CreateTaskResult. An
	// unconditional stamp here would relabel both as finished. A client would
	// then read a prompt, or a bare task handle, as the answer it asked for.
	//
	// The test is "did the handler set one", not "is it one of these two", so a
	// future extension result type cannot be silently overwritten by forgetting
	// to extend a predicate here.
	if existing, ok := out[resultTypeKey].(string); !ok || existing == "" {
		out[resultTypeKey] = resultTypeComplete
	}
	out[metaKey] = s.resultMeta(result[metaKey])
	return &response{JSONRPC: "2.0", ID: id, Result: out}
}

// resultMeta merges the server identity into whatever `_meta` a handler already
// supplied.
//
// MCP 2026-07-28 changelog: "servers SHOULD identify themselves in each
// result's `_meta` (io.modelcontextprotocol/serverInfo)".
func (s *Streamable) resultMeta(existing any) map[string]any {
	prior, _ := existing.(map[string]any)
	out := make(map[string]any, len(prior)+1)
	maps.Copy(out, prior)
	out[metaKeyServerInfo] = s.serverInfo()
	return out
}

func (s *Streamable) fail(id *json.RawMessage, code int, msg string) *response {
	return &response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
}

// failUnsupportedVersion builds the UnsupportedProtocolVersionError for a
// request declaring a version this server does not implement.
//
// MCP 2026-07-28 schema UnsupportedProtocolVersionError: `data.supported` lists
// the "protocol versions the server supports. The client should choose a
// mutually supported version from this list and retry", and `data.requested`
// echoes "the protocol version that was requested by the client".
//
// The supported list is cloned from supportedProtocolVersions, so it cannot
// drift from what the version check accepts. And `requested` is the value
// already parsed out of the body rather than a raw header.
func (s *Streamable) failUnsupportedVersion(id *json.RawMessage, requested string) *response {
	return &response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    rpcUnsupportedProtocolVersion,
			Message: "unsupported protocol version",
			Data: map[string]any{
				"supported": slices.Clone(supportedProtocolVersions),
				"requested": requested,
			},
		},
	}
}

// failMissingTasksCapability builds the MissingRequiredClientCapabilityError
// for a task request whose `_meta` did not declare task support.
//
// MCP 2026-07-28 schema MissingRequiredClientCapabilityError: "Returned when
// processing a request requires a capability the client did not declare in
// clientCapabilities", carrying `data.requiredCapabilities` in the
// ClientCapabilities shape. This is NOT -32601: an undeclared capability is a
// method the client cannot be served, not a method the server does not have.
//
// Tasks is the only capability this server can demand, so the capability is
// fixed here rather than passed in. Every OTHER capability Ze offers -- notably
// resources -- is a *ServerCapabilities* member. A conformant client cannot
// declare one, so a demand for it would refuse every conformant caller.
//
// The requiredCapabilities payload is spelled in the EXTENSION shape. That is
// the shape a client must send to be served: an identifier under
// `extensions`, per MCP 2026-07-28 basic/versioning Section "Extension
// Negotiation" ("Extensions are advertised in the `extensions` field of
// capabilities, which is a map of extension identifiers to per-extension
// settings objects"). A bare `tasks` member would tell the client to send
// something this server no longer accepts.
func (s *Streamable) failMissingTasksCapability(id *json.RawMessage) *response {
	var tb textbuf.Buffer
	msg := tb.Str("client did not declare the ").Str(extensionTasks).
		Str(` extension in params._meta["io.modelcontextprotocol/clientCapabilities"].extensions`).String()
	return &response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    rpcMissingRequiredClientCapability,
			Message: msg,
			Data: map[string]any{
				"requiredCapabilities": map[string]any{
					capabilityExtensionsKey: map[string]any{extensionTasks: map[string]any{}},
				},
			},
		},
	}
}

// writeJSONResponse writes a single JSON-RPC response with HTTP 200.
func writeJSONResponse(w http.ResponseWriter, v any) {
	writeJSONResponseStatus(w, http.StatusOK, v)
}

// writeJSONResponseStatus writes a single JSON-RPC response with an explicit
// HTTP status. MCP 2026-07-28 pins a status to several JSON-RPC error codes, so
// the status is chosen with the body rather than defaulted to 200.
func writeJSONResponseStatus(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, writeErr := w.Write(data); writeErr != nil {
		return
	}
}
