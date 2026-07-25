// Design: docs/guide/mcp/overview.md -- MCP tool auto-generation from command registry
// Related: streamable.go -- Streamable HTTP transport (the only HTTP entry point)

// Package mcp implements the MCP (Model Context Protocol) server surface:
// JSON-RPC tool dispatch wrapping Ze's command dispatcher, served over the
// Streamable HTTP transport (streamable.go). Tools are auto-generated from
// the command registry, plus a small handcrafted set (ze_execute,
// ze_reference); a ToolProvider (ze-chaos) can replace the tool surface
// entirely via StreamableConfig.Provider.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/component/aihelp"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// TaskSupportLevel declares whether a tool supports task-augmented calls.
type TaskSupportLevel uint8

const (
	TaskSupportOptional  TaskSupportLevel = 0 // default: sync or task
	TaskSupportRequired  TaskSupportLevel = 1 // must be called as a task
	TaskSupportForbidden TaskSupportLevel = 2 // must not be called as a task
)

const (
	taskSupportWireOptional  = "optional"
	taskSupportWireRequired  = "required"
	taskSupportWireForbidden = "forbidden"
)

func (t TaskSupportLevel) String() string {
	switch t {
	case TaskSupportRequired:
		return taskSupportWireRequired
	case TaskSupportForbidden:
		return taskSupportWireForbidden
	default:
		return taskSupportWireOptional
	}
}

// UIResourceInfo holds MCP Apps UI resource metadata for a command group.
type UIResourceInfo struct {
	Path        string // relative path under ui/ FS (e.g. "bgp-peer/index.html")
	Permissions string // space-separated capabilities (e.g. "network")
	CSP         string // Content-Security-Policy directive
}

// CommandInfo describes a registered command for MCP tool generation.
type CommandInfo struct {
	Name        string           // Dispatch path, e.g. "show bgp rib status", "show config dump"
	Help        string           // Description from YANG
	ReadOnly    bool             // True if read-only command
	Params      []ParamInfo      // Input parameters from YANG RPC (nil = no typed params)
	TaskSupport TaskSupportLevel // From YANG ze:task-support extension
	UIResource  *UIResourceInfo  // From YANG ze:ui-resource extension (nil = no UI)
}

// ParamInfo describes a single input parameter from YANG RPC metadata.
type ParamInfo struct {
	Name        string // Parameter name (kebab-case from YANG)
	Type        string // YANG type: "string", "uint32", "boolean", etc.
	Description string // From YANG description
	Required    bool   // Mandatory in YANG
}

// CommandLister returns all registered commands. Called at tools/list time
// so the tool list always reflects current registrations.
type CommandLister func() []CommandInfo

// toolGroup is a set of related commands sharing a prefix.
type toolGroup struct {
	prefix      string           // e.g. "show bgp rib", "show config"
	actions     []action         // subcommands within the group
	taskSupport TaskSupportLevel // highest declared across actions
	uiResource  *UIResourceInfo  // from any action with a UI bundle
}

// action is a single subcommand within a group.
type action struct {
	name        string           // action name (suffix after prefix), e.g. "status", "dump"
	help        string           // description
	full        string           // full command path for dispatch
	params      []ParamInfo      // typed parameters from YANG (nil = generic arguments only)
	taskSupport TaskSupportLevel // from YANG ze:task-support
	uiResource  *UIResourceInfo  // from YANG ze:ui-resource
}

// groupCommands groups commands by their natural prefix.
// Commands like "show bgp rib status", "show bgp rib best" group under "show bgp".
// Commands like "show config dump", "show config diff" group under "show config".
//
// Grouping rule: find the longest shared prefix among at least 2 commands,
// where removing the prefix leaves a non-empty suffix (the action).
// Single commands with no siblings become their own group with no action param.
func groupCommands(commands []CommandInfo) []toolGroup {
	type entry struct {
		full        string
		help        string
		params      []ParamInfo
		taskSupport TaskSupportLevel
		uiResource  *UIResourceInfo
	}

	// Index commands by first-token and first-two-tokens.
	byOne := make(map[string][]entry)
	byTwo := make(map[string][]entry)

	for _, cmd := range commands {
		tokens := strings.Fields(cmd.Name)
		if len(tokens) == 0 {
			continue
		}
		e := entry{full: cmd.Name, help: cmd.Help, params: cmd.Params, taskSupport: cmd.TaskSupport, uiResource: cmd.UIResource}
		one := tokens[0]
		byOne[one] = append(byOne[one], e)
		if len(tokens) >= 2 {
			var tb textbuf.Buffer
			two := tb.Str(tokens[0]).Byte(' ').Str(tokens[1]).String()
			byTwo[two] = append(byTwo[two], e)
		}
	}

	var groups []toolGroup
	used := make(map[string]bool)

	// First pass: depth-2 groups under prefixes that have multiple subgroups.
	// E.g. "show" has "show config", "show schema", "show env" -> depth-2 groups.
	for one, entries := range byOne {
		subgroups := make(map[string]bool)
		for _, e := range entries {
			tokens := strings.Fields(e.full)
			if len(tokens) >= 3 {
				subgroups[tokens[0]+" "+tokens[1]] = true
			}
		}
		if len(subgroups) < 2 {
			continue
		}
		// Generate depth-2 groups.
		for two, twoEntries := range byTwo {
			if !strings.HasPrefix(two, one+" ") {
				continue
			}
			g := toolGroup{prefix: two}
			for _, e := range twoEntries {
				suffix := strings.TrimPrefix(e.full, two+" ")
				if suffix == e.full {
					suffix = ""
				}
				g.actions = append(g.actions, action{
					name:        suffix,
					help:        e.help,
					full:        e.full,
					params:      e.params,
					taskSupport: e.taskSupport,
					uiResource:  e.uiResource,
				})
				used[e.full] = true
			}
			sortActions(g.actions)
			g.taskSupport = groupTaskSupport(g.actions)
			g.uiResource = groupUIResource(g.actions)
			groups = append(groups, g)
		}
		// Depth-1 commands under this prefix not in any depth-2 group.
		for _, e := range entries {
			if used[e.full] {
				continue
			}
			tokens := strings.Fields(e.full)
			if len(tokens) == 2 {
				g := toolGroup{prefix: e.full}
				g.actions = append(g.actions, action{name: "", help: e.help, full: e.full, params: e.params, taskSupport: e.taskSupport, uiResource: e.uiResource})
				g.taskSupport = e.taskSupport
				g.uiResource = e.uiResource
				used[e.full] = true
				groups = append(groups, g)
			}
		}
	}

	// Second pass: depth-1 groups for remaining commands.
	for one, entries := range byOne {
		var remaining []entry
		for _, e := range entries {
			if !used[e.full] {
				remaining = append(remaining, e)
			}
		}
		if len(remaining) == 0 {
			continue
		}

		g := toolGroup{prefix: one}
		for _, e := range remaining {
			suffix := strings.TrimPrefix(e.full, one+" ")
			if suffix == e.full {
				suffix = ""
			}
			g.actions = append(g.actions, action{
				name:        suffix,
				help:        e.help,
				full:        e.full,
				params:      e.params,
				taskSupport: e.taskSupport,
				uiResource:  e.uiResource,
			})
			used[e.full] = true
		}
		sortActions(g.actions)
		g.taskSupport = groupTaskSupport(g.actions)
		g.uiResource = groupUIResource(g.actions)
		groups = append(groups, g)
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].prefix < groups[j].prefix
	})

	return groups
}

func sortActions(actions []action) {
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].name < actions[j].name
	})
}

// toolName converts a command prefix to an MCP tool name.
// "show bgp rib" -> "ze_show_bgp_rib", "show config" -> "ze_show_config".
func toolName(prefix string) string {
	r := strings.NewReplacer(" ", "_", "-", "_")
	var tb textbuf.Buffer
	return tb.Str("ze_").Str(r.Replace(prefix)).String()
}

// generateTools builds MCP tool definitions from command groups.
// skipNames lists tool names already covered by handcrafted tools.
func generateTools(groups []toolGroup, skipNames map[string]bool) []map[string]any {
	var result []map[string]any

	for _, g := range groups {
		if skipNames[toolName(g.prefix)] {
			continue
		}
		tool := buildToolDef(g)
		if tool != nil {
			result = append(result, tool)
		}
	}

	return result
}

// buildToolDef creates an MCP tool definition from a command group.
func buildToolDef(g toolGroup) map[string]any {
	name := toolName(g.prefix)

	var desc textbuf.Buffer
	desc.Str("Commands under '").Str(g.prefix).Str("'.")

	properties := map[string]any{}
	var required []string

	// Separate empty-name actions (command IS the prefix) from named ones.
	var namedActions []action
	for _, a := range g.actions {
		if a.name != "" {
			namedActions = append(namedActions, a)
		}
	}

	if len(namedActions) > 0 {
		actionEnums := make([]string, len(namedActions))
		actionDescs := make([]string, 0, len(namedActions))
		for i, a := range namedActions {
			actionEnums[i] = a.name
			if a.help != "" {
				var tb textbuf.Buffer
				actionDescs = append(actionDescs, tb.Str(a.name).Str(": ").Str(a.help).String())
			}
		}

		actionProp := map[string]any{
			"type": "string",
			"enum": actionEnums,
		}
		if len(actionDescs) > 0 {
			actionProp["description"] = textbuf.Join(actionDescs, ". ")
		} else {
			actionProp["description"] = "Action to perform"
		}
		properties["action"] = actionProp
		required = append(required, "action")

		desc.Reset()
		if len(namedActions) == 1 {
			if namedActions[0].help != "" {
				desc.Str(namedActions[0].help)
			} else {
				desc.Str("Run '").Str(g.prefix).Byte(' ').Str(namedActions[0].name).Str("'.")
			}
		} else {
			desc.Str("Actions: ").Join(actionEnums, ", ").Byte('.')
		}
	} else if len(g.actions) == 1 && g.actions[0].help != "" {
		desc.Reset()
		desc.Str(g.actions[0].help)
	}

	// Add typed parameters from YANG RPC metadata.
	// Parameters are collected across all actions; mandatory YANG inputs
	// are added to the required list so AI clients see them as required.
	addedParams, yangRequired := addYANGParams(g.actions, properties)
	required = append(required, yangRequired...)

	// Only add generic "arguments" if no typed params were found.
	if !addedParams {
		properties["arguments"] = map[string]any{
			"type":        "string",
			"description": "Additional arguments to append to the command",
		}
	}

	properties["peer"] = map[string]any{
		"type":        "string",
		"description": "Peer selector: address, name, or * for all",
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return nil
	}

	tool := map[string]any{
		"name":        name,
		"description": desc.String(),
		"inputSchema": json.RawMessage(schemaJSON),
	}
	tool["execution"] = map[string]any{
		"taskSupport": g.taskSupport.String(),
	}
	if g.uiResource != nil {
		uiMap := map[string]any{
			"resourceUri": uiScheme + g.uiResource.Path,
		}
		if g.uiResource.Permissions != "" {
			uiMap["permissions"] = strings.Fields(g.uiResource.Permissions)
		}
		if g.uiResource.CSP != "" {
			uiMap["csp"] = g.uiResource.CSP
		}
		tool["_meta"] = map[string]any{"ui": uiMap}
	}
	return tool
}

// groupUIResource returns the first non-nil UIResourceInfo from the actions.
func groupUIResource(actions []action) *UIResourceInfo {
	for _, a := range actions {
		if a.uiResource != nil {
			return a.uiResource
		}
	}
	return nil
}

// groupTaskSupport derives the group-level taskSupport from its actions.
// If any action is required, the group is required. If all are forbidden,
// the group is forbidden. Otherwise optional.
func groupTaskSupport(actions []action) TaskSupportLevel {
	hasRequired := false
	allForbidden := true
	for _, a := range actions {
		if a.taskSupport == TaskSupportRequired {
			hasRequired = true
		}
		if a.taskSupport != TaskSupportForbidden {
			allForbidden = false
		}
	}
	if hasRequired {
		return TaskSupportRequired
	}
	if allForbidden && len(actions) > 0 {
		return TaskSupportForbidden
	}
	return TaskSupportOptional
}

// addYANGParams collects typed parameters from YANG RPC metadata across all
// actions and adds them as named JSON Schema properties. Returns whether any
// params were added and the names of mandatory params.
func addYANGParams(actions []action, properties map[string]any) (bool, []string) {
	seen := make(map[string]bool)
	var added bool
	var requiredParams []string
	for _, a := range actions {
		for _, p := range a.params {
			if seen[p.Name] {
				continue
			}
			seen[p.Name] = true
			prop := map[string]any{
				"type": yangTypeToJSON(p.Type),
			}
			if p.Description != "" {
				prop["description"] = p.Description
			}
			properties[p.Name] = prop
			added = true
			if p.Required {
				requiredParams = append(requiredParams, p.Name)
			}
		}
	}
	return added, requiredParams
}

// yangTypeToJSON maps YANG type names to JSON Schema types.
// Unknown types map to "string" which is the safest JSON Schema fallback.
var yangTypeToJSONMap = map[string]string{
	"uint8":   "integer",
	"uint16":  "integer",
	"uint32":  "integer",
	"uint64":  "integer",
	"int8":    "integer",
	"int16":   "integer",
	"int32":   "integer",
	"int64":   "integer",
	"boolean": "boolean",
}

func yangTypeToJSON(yangType string) string {
	if jsonType, ok := yangTypeToJSONMap[yangType]; ok {
		return jsonType
	}
	return "string"
}

// reservedParams are the built-in dispatch parameters, not forwarded as typed args.
var reservedParams = map[string]bool{"action": true, "arguments": true, "peer": true}

// server runs one tool dispatch.
//
// Lifetime: one *server per HTTP request. `Streamable.callTool` creates it
// per tools/call; the task worker builds one per task run. ctx and session
// carry request-scoped state; storing them on the struct keeps the
// `toolHandlers` map signature compact. DO NOT hoist the construction out of
// the request scope -- sharing a *server across concurrent requests would
// race on the ctx/session fields. When a handler needs ctx or session it
// reads the field directly and degrades on nil (unit tests construct
// *server directly without either).
type server struct {
	dispatch CommandDispatcher
	commands CommandLister
	// session carries the active POST's session so tool handlers (notably
	// ze_execute's missing-command branch) can call session.Elicit. Nil
	// when dispatch runs outside a session context (isolated handler
	// tests). Nil-aware handlers must degrade gracefully.
	session *session
	// ctx is the active HTTP request's context. Tool handlers that call
	// into session.Elicit (or any other blocking op) MUST pass this ctx
	// through so a client disconnect unblocks the suspended handler via
	// ctx.Done() -- otherwise the correlation lingers until the session
	// TTL sweeps it. Nil in unit tests; use context.Background() as the
	// fallback in that case.
	ctx        context.Context //nolint:containedctx // per-request state; see godoc above
	username   string
	remoteAddr string
}

// dispatchGenerated handles a tools/call for an auto-generated tool.
// It builds the command string from the tool group prefix + action + typed params + arguments.
// validActions contains the server-defined action names; if non-nil, the action
// is validated against this set to prevent injection of arbitrary tokens.
//
// Typed YANG params (any JSON field not in reservedParams) are appended as
// "key value" pairs after the action. This lets handlers receive structured
// params through the standard text command interface.
func (s *server) dispatchGenerated(prefix string, validActions map[string]bool, args json.RawMessage) map[string]any {
	// Unmarshal into a generic map to capture typed params alongside standard ones.
	var all map[string]any
	if err := json.Unmarshal(args, &all); err != nil {
		var tb textbuf.Buffer
		return ErrResult(tb.Str("invalid arguments: ").Err(err).String())
	}

	action, _ := all["action"].(string)
	arguments, _ := all["arguments"].(string)
	peer, _ := all["peer"].(string)

	if peer != "" {
		if err := noSpaces("peer", peer); err != nil {
			return ErrResult(err.Error())
		}
	}
	if action != "" && !validActions[action] {
		return ErrResult(fmt.Sprintf("invalid action %q", action))
	}
	if strings.ContainsAny(action, "\n\r") {
		return ErrResult("action must not contain newlines")
	}
	if strings.ContainsAny(arguments, "\n\r\t") {
		return ErrResult("arguments must not contain newlines or tabs")
	}

	var cmd textbuf.Buffer

	if peer != "" {
		cmd.Str("peer ").Str(peer).Byte(' ')
	}

	cmd.Str(prefix)
	if action != "" {
		cmd.Byte(' ').Str(action)
	}

	for key, val := range all {
		if reservedParams[key] || val == nil {
			continue
		}
		sval := fmt.Sprint(val)
		if sval == "" {
			continue
		}
		if strings.ContainsAny(sval, "\n\r\t") {
			var tb textbuf.Buffer
			return ErrResult(tb.Str("parameter ").Quoted(key).Str(" must not contain newlines or tabs").String())
		}
		cmd.Byte(' ').Str(key).Byte(' ').Str(sval)
	}

	if arguments != "" {
		cmd.Byte(' ').Str(arguments)
	}

	return s.run(cmd.String())
}

// --- Shared tool-dispatch primitives (moved from the deleted legacy
// handler.go; spec-followup-subsystem AC-9). The Streamable transport and the
// ze-chaos ToolProvider both build on these. ---

// maxRequestBody limits the size of MCP HTTP request bodies (1 MB).
const maxRequestBody = 1 << 20

// ToolProvider supplies tool definitions and handles tool calls for an MCP
// server. ze-chaos implements this with its own tools (chaos_status, ...);
// set StreamableConfig.Provider to serve a provider's tools instead of the
// command-registry surface.
type ToolProvider interface {
	ServerName() string
	Tools() []map[string]any
	CallTool(name string, args json.RawMessage) map[string]any
}

// CommandDispatcher executes a Ze command and returns the typed response. It
// is an alias for the unified plugin.CommandDispatcher every surface shares;
// the MCP handlers render the JSON string at their edge via
// CommandDispatcher.JSON, threading the authenticated caller's identity so
// authorization and accounting apply to MCP surfaces, not only SSH.
type CommandDispatcher = plugin.CommandDispatcher

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
	Task      json.RawMessage `json:"task,omitempty"`
}

// toolHandlers maps handcrafted MCP tool names to their implementations.
// ze_execute is a raw command dispatch escape hatch (equivalent to ze_system dispatch).
var toolHandlers = map[string]func(s *server, args json.RawMessage) map[string]any{
	"ze_execute": func(s *server, args json.RawMessage) map[string]any {
		var input struct {
			Command string `json:"command"`
		}
		var tb textbuf.Buffer
		if err := json.Unmarshal(args, &input); err != nil {
			return ErrResult(tb.Str("invalid arguments: ").Err(err).String())
		}
		if s.dispatch == nil {
			return ErrResult("dispatcher not available")
		}
		// Missing command: if the client declared the elicitation capability,
		// prompt for one. Otherwise fail fast so the caller re-invokes with a
		// command instead of blocking on an Elicit that will never be answered.
		if input.Command == "" {
			if s.session == nil || !s.session.ClientSupportsElicit() {
				return ErrResult("missing required argument: command")
			}
			// Prefer the POST's context so a client disconnect unblocks the
			// suspended handler; fall back to Background when the server was
			// constructed without one (unit tests).
			ctx := s.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			content, err := s.session.Elicit(ctx,
				"Which ze command would you like to run?",
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type":        "string",
							"description": "A ze CLI command, e.g. 'peer list' or 'show bgp summary'",
						},
					},
					"required": []any{"command"},
				})
			if err != nil {
				return ErrResult(tb.Str("elicit: ").Err(err).String())
			}
			cmd, _ := content["command"].(string)
			if cmd == "" {
				return ErrResult("elicit returned empty command")
			}
			input.Command = cmd
		}
		result, err := s.dispatch.JSON(context.Background(), plugin.CallerIdentity{Username: s.username, RemoteAddr: s.remoteAddr}, input.Command)
		if err != nil {
			return ErrResult(err.Error())
		}
		return TextResult(result)
	},
	// ze_reference returns the full machine-readable AI reference, assembled
	// from the same source as `ze help ai --json` (internal/component/aihelp),
	// so an MCP client can discover this instance's capabilities on connect.
	"ze_reference": func(_ *server, _ json.RawMessage) map[string]any {
		data, err := json.MarshalIndent(aihelp.Build(), "", "  ")
		if err != nil {
			return ErrResult("could not marshal AI reference")
		}
		return TextResult(string(data))
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
	output, err := s.dispatch.JSON(context.Background(), plugin.CallerIdentity{Username: s.username, RemoteAddr: s.remoteAddr}, command)
	if err != nil {
		return ErrResult(err.Error())
	}
	return TextResult(output)
}

// TextResult returns an MCP text content result.
func TextResult(s string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": s}},
	}
}

// ErrResult returns an MCP error content result.
func ErrResult(msg string) map[string]any {
	var tb textbuf.Buffer
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": tb.Str("Error: ").Str(msg).String()}},
		"isError": true,
	}
}

// handcraftedTools defines tool schemas for handcrafted tools.
var handcraftedTools = []map[string]any{
	{
		"name":        "ze_execute",
		"description": "Execute a ze CLI command and return the result. When invoked with a client that declared capabilities.elicitation, omitting 'command' causes the server to prompt for one via elicitation/create.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The ze command to execute (e.g., 'peer list', 'show bgp summary'). Optional only when the client supports elicitation.",
				},
			},
		},
	},
	{
		"name":        "ze_reference",
		"description": "Full machine-readable reference for this ze daemon: CLI commands, daemon API endpoints (ze-show:*, ze-set:*, ...) with dispatch keys, loaded plugins, address families, and config services. Call this first to discover what this instance can do. Returns the same JSON as 'ze help ai --json'.",
		"inputSchema": map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
}
