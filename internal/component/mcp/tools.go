// Design: docs/guide/mcp/overview.md -- MCP tool auto-generation from command registry
// Related: streamable.go -- Streamable HTTP transport (the only HTTP entry point)
// Related: apps.go -- gates the _meta.ui object buildToolDef emits below

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
	"slices"
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
	// TakesSelector reports that the dispatcher consumes an INLINE peer
	// selector for this command -- the `show bgp peer <selector> detail` shape.
	// Supplied by the hub from the dispatcher's own predicate
	// (pluginserver.Command.TakesInlineSelector); MCP must never infer it from
	// the command name.
	TakesSelector bool
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
	name          string           // action name (suffix after prefix), e.g. "status", "dump"
	help          string           // description
	full          string           // full command path for dispatch
	params        []ParamInfo      // typed parameters from YANG (nil = generic arguments only)
	taskSupport   TaskSupportLevel // from YANG ze:task-support
	uiResource    *UIResourceInfo  // from YANG ze:ui-resource
	takesSelector bool             // dispatcher consumes an inline peer selector for `full`
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
		full          string
		help          string
		params        []ParamInfo
		taskSupport   TaskSupportLevel
		uiResource    *UIResourceInfo
		takesSelector bool
	}

	// Index commands by first-token and first-two-tokens.
	byOne := make(map[string][]entry)
	byTwo := make(map[string][]entry)

	for _, cmd := range commands {
		tokens := strings.Fields(cmd.Name)
		if len(tokens) == 0 {
			continue
		}
		e := entry{full: cmd.Name, help: cmd.Help, params: cmd.Params, taskSupport: cmd.TaskSupport, uiResource: cmd.UIResource, takesSelector: cmd.TakesSelector}
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
					name:          suffix,
					help:          e.help,
					full:          e.full,
					params:        e.params,
					taskSupport:   e.taskSupport,
					uiResource:    e.uiResource,
					takesSelector: e.takesSelector,
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
				g.actions = append(g.actions, action{name: "", help: e.help, full: e.full, params: e.params, taskSupport: e.taskSupport, uiResource: e.uiResource, takesSelector: e.takesSelector})
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

	// Advertise `peer` ONLY on groups that contain at least one command the
	// dispatcher actually reads a selector for. It used to be added to EVERY
	// generated tool, so a model was invited to set it on `show config dump`
	// and got `peer <sel> show config dump`, which resolves nowhere.
	if groupTakesSelector(g.actions) {
		properties["peer"] = map[string]any{
			"type":        "string",
			"description": "Peer selector: address, name, as<N>, glob, or * for all. Only valid for actions that address a peer; the value is placed after the 'peer' keyword of the command.",
		}
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
	// MCP Apps (the io.modelcontextprotocol/ui extension). The three keys are
	// the extension's own, verbatim: its overview names "_meta.ui.resourceUri
	// field pointing to a ui:// resource", says the "_meta.ui object can include
	// permissions to request additional capabilities (e.g., microphone,
	// camera)", and names "_meta.ui.csp" for "what external origins the app can
	// load resources from".
	//
	// Emitted unconditionally here and removed downstream by gateUIMeta when the
	// requesting client did not declare the extension (apps.go). The gate lives
	// there rather than here so that ONE site covers provider descriptors too.
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
		tool[metaKey] = map[string]any{metaKeyUI: uiMap}
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
//
// Precedence is FORBIDDEN-WINS, and that is the whole point of this function.
//
// # Why the precedence had to be inverted (D-1b)
//
// The eligibility decision is made per TOOL, not per command. lookupTaskSupport
// (streamable_tools.go) resolves a tool name to a command group, and a group
// folds several actions into one level. Under the old client-directed model the
// client still had to ask for a task, and a per-action check ran on the way in.
// A `required` win here was therefore harmless.
//
// Under the server-directed model this value IS the promotion rule. `required`
// means the server returns a task handle unasked. Required-wins would therefore
// auto-task an action its YANG explicitly annotated `forbidden`. The four
// mutating rib commands (`clear bgp rib in/out`, `request bgp rib
// inject/withdraw`) are exactly the set that annotation exists to protect.
//
// So a single `forbidden` action poisons the whole group. The cost of an error
// in this direction is one genuinely long action that runs synchronously. The
// cost in the other direction is an auto-tasked route injection
// (ai/rules/evidence.md).
//
// No group mixes levels today. The forbidden rib actions sit under the `clear`
// and `request` roots, and the required one sits under `show`, so they land in
// different tools. The fix is free now and expensive later. If a mixed group is
// ever authored, the right repair is to split the tool, not to relax this
// guard.
func groupTaskSupport(actions []action) TaskSupportLevel {
	hasRequired := false
	for _, a := range actions {
		if a.taskSupport == TaskSupportForbidden {
			return TaskSupportForbidden
		}
		if a.taskSupport == TaskSupportRequired {
			hasRequired = true
		}
	}
	if hasRequired {
		return TaskSupportRequired
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

// peerKeyword is the command token a peer selector value attaches to. The
// grammar's peer exception spells the selector immediately after it
// (`show bgp peer <selector> detail`, `request peer <selector> flush`) --
// ai/rules/cli.md, "Peer Commands".
const peerKeyword = "peer"

// actionAcceptsPeer reports whether a `peer` argument is BOTH read by the
// dispatcher for this command and placeable in it. Both halves are required:
//
//   - takesSelector says the dispatcher consumes an inline selector, but it is
//     true for non-peer inline selectors too (`clear dns cache record <name>`,
//     `create interface bridge name <n>`);
//   - a `peer` token is what anchors the value's position (spliceSelector).
//
// Advertising the argument on exactly the set that can use it keeps the tool
// schema honest: every tool that offers `peer` accepts one, and no tool offers
// an argument whose only possible outcome is an error.
func actionAcceptsPeer(a action) bool {
	if !a.takesSelector {
		return false
	}
	for tok := range strings.FieldsSeq(a.full) {
		if strings.EqualFold(tok, peerKeyword) {
			return true
		}
	}
	return false
}

// groupTakesSelector reports whether any command in the group accepts a peer
// selector, i.e. whether the `peer` tool argument is meaningful for this tool
// at all. Derived from the dispatcher-supplied flag, never from the name.
func groupTakesSelector(actions []action) bool {
	return slices.ContainsFunc(actions, actionAcceptsPeer)
}

// spliceSelector inserts sel into full at the grammar's selector position:
// immediately after the command's own `peer` keyword. It returns ok=false when
// full has no `peer` token, so the caller can fail closed rather than guess a
// position.
//
// The position matters, not just the presence: the dispatcher extracts an
// inline selector at the FIRST token where the input diverges from the
// registered key (matchCommandTokens/implicitSelectorDef in
// internal/component/plugin/server/command.go). Right after `peer` is both that
// divergence point and the spelling the YANG descriptions document. When `peer`
// is the final token (`delete bgp peer`) the value lands trailing, which the
// same dispatcher resolves through positional ArgDef matching.
func spliceSelector(full, sel string) (string, bool) {
	tokens := strings.Fields(full)
	for i, tok := range tokens {
		if !strings.EqualFold(tok, peerKeyword) {
			continue
		}
		var tb textbuf.Buffer
		tb.Join(tokens[:i+1], " ").Byte(' ').Str(sel)
		if i+1 < len(tokens) {
			tb.Byte(' ').Join(tokens[i+1:], " ")
		}
		return tb.String(), true
	}
	return "", false
}

// server runs one tool dispatch.
//
// Lifetime: one *server per HTTP request. `Streamable.callTool` creates it
// per tools/call; the task worker builds one per task run. ctx carries
// request-scoped state; storing it on the struct keeps the `toolHandlers` map
// signature compact. DO NOT hoist the construction out of the request scope --
// sharing a *server across concurrent requests would race on the ctx field.
type server struct {
	dispatch CommandDispatcher
	commands CommandLister
	// ctx is the active HTTP request's context, or a task worker's
	// deadline-bearing context. Every dispatch MUST run under it, so that a
	// client disconnect and the task execution deadline both reach the
	// dispatcher. Read it through the context accessor below rather than
	// directly. The accessor is what gives the nil case (a bare *server in a
	// unit test) one defined answer.
	ctx context.Context //nolint:containedctx // per-request state; see godoc above
	// username is the authenticated principal the dispatched command runs as.
	// It comes from the per-request authenticator, never from a request body.
	username   string
	remoteAddr string
	// caps is what THIS request's client declared. A handler that wants to ask
	// the user for something reads caps.ElicitForm before it emits an
	// inputRequests entry. MCP 2026-07-28 basic/patterns/mrtr Section "Server
	// Requirements": "7. Servers MUST NOT send an inputRequests that the client
	// has not declared support for in its capabilities." The zero value denies,
	// so a runner built without capabilities never prompts.
	caps clientCapabilities
	// inputResponses is params.inputResponses from a Multi Round-Trip retry. It
	// holds the answers to the inputRequests a previous InputRequiredResult
	// asked for, and it is nil on a first attempt. It is the ONLY thing that
	// crosses between the two requests. Nothing is held server-side between
	// them.
	inputResponses map[string]any
}

// context returns the context every dispatch this runner makes MUST run under.
//
// It is the ONE reader of the ctx field, so one place exists where the
// request's deadline and its cancellation enter the command dispatcher.
//
// Both call sites used to hard-code context.Background(). That silently severed
// two mechanisms documented elsewhere as working: a task worker's execution
// deadline (tasks.go, Create), and a client disconnect that unblocks a handler
// (streamable_tools.go, runMethod). A canceled context that nothing observes
// cancels nothing.
//
// The nil fallback is for unit tests that build a bare *server. Falling back to
// Background is safe in exactly that case and nowhere else: every production
// path (callTool, createTask) sets the field.
func (s *server) context() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

// dispatchGenerated handles a tools/call for an auto-generated tool.
// It builds the command string from the tool group prefix + action + typed params + arguments.
// actionSelector maps each server-defined action name to whether that command
// takes a peer selector; membership in the map is what validates the action, so
// an arbitrary token can never be injected as one.
//
// Typed YANG params (any JSON field not in reservedParams) are appended as
// "key value" pairs after the action. This lets handlers receive structured
// params through the standard text command interface.
func (s *server) dispatchGenerated(prefix string, actionSelector map[string]bool, args json.RawMessage) map[string]any {
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
	if _, known := actionSelector[action]; action != "" && !known {
		var tb textbuf.Buffer
		return ErrResult(tb.Str("invalid action ").Quoted(action).String())
	}
	if strings.ContainsAny(action, "\n\r") {
		return ErrResult("action must not contain newlines")
	}
	if strings.ContainsAny(arguments, "\n\r\t") {
		return ErrResult("arguments must not contain newlines or tabs")
	}

	// Resolve the full dispatch path, then place the peer selector where the
	// grammar puts it: after the command's own `peer` keyword
	// (`show bgp peer <selector> detail`), NOT in front of the whole command.
	// Prefixing produced `peer <selector> show bgp peer detail`, which resolves
	// nowhere -- every peer-scoped MCP tool call failed.
	full := prefix
	if action != "" {
		var tb textbuf.Buffer
		full = tb.Str(prefix).Byte(' ').Str(action).String()
	}
	if peer != "" {
		if !actionSelector[action] {
			// Note for the reader: a few commands declare a YANG input leaf
			// that is literally named "peer" but is NOT a peer selector -- the
			// veth peer interface name of `create interface veth name`
			// (internal/component/iface/yang/ze-iface-api.yang, rpc
			// interface-create-veth). Because "peer" is a reservedParams key it
			// is consumed here rather than forwarded as a typed parameter, so
			// that value cannot be supplied over MCP at all. That name
			// collision predates this guard and is not fixed by it; the guard
			// only makes the refusal explicit instead of emitting a command
			// string the dispatcher cannot resolve.
			var tb textbuf.Buffer
			return ErrResult(tb.Str("command ").Quoted(full).Str(" does not take a peer selector; the \"peer\" argument is not accepted here").String())
		}
		spliced, ok := spliceSelector(full, peer)
		if !ok {
			// Fail closed: the command was declared selector-taking but has no
			// `peer` keyword to anchor the value to, so any placement would be a
			// guess. Say so instead of emitting a command the daemon rejects.
			var tb textbuf.Buffer
			return ErrResult(tb.Str("cannot place a peer selector in ").Quoted(full).Str(": the command has no \"peer\" keyword").String())
		}
		full = spliced
	}

	var cmd textbuf.Buffer
	cmd.Str(full)

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
	// Data carries the structured payload a few MCP-defined error codes
	// require: `supported`/`requested` on -32022, `requiredCapabilities` on
	// -32021. Omitted when absent -- notably on -32020, whose schema defines
	// no data object at all and whose detail rides in Message.
	Data any `json:"data,omitempty"`
}

// callParams is the `params` object of a tools/call request.
//
// There is deliberately no `task` member. MCP 2026-07-28 changelog Major change
// 6 moved tasks onto the io.modelcontextprotocol/tasks extension, which "allows
// servers to return task handles unsolicited without per-request opt-in".
//
// The SERVER decides a call runs as a task, from the command's
// `ze:task-support` annotation. No client-supplied field is therefore left to
// read (D-1).
type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// toolNameExecute is the handcrafted raw-dispatch tool. It is named once
// because three places must agree on it: the handler map below, the descriptor
// in handcraftedTools, and gateExecuteCommandRequired (mrtr.go). That last one
// reshapes the descriptor for a client this server cannot prompt. A literal in
// each would let the descriptor and the handler drift, which is exactly the
// defect that made the elicitation path unreachable.
const toolNameExecute = "ze_execute"

// toolHandlers maps handcrafted MCP tool names to their implementations.
// ze_execute is a raw command dispatch escape hatch (equivalent to ze_system dispatch).
var toolHandlers = map[string]func(s *server, args json.RawMessage) map[string]any{
	toolNameExecute: func(s *server, args json.RawMessage) map[string]any {
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
		command, outcome := s.resolveExecuteCommand(input.Command)
		switch outcome {
		case inputAccepted:
			// fall through to dispatch
		case inputMissing:
			return s.askForCommand()
		case inputDeclined:
			return ErrResult(tb.Reset().Str("no command was supplied: ").Err(ErrElicitDeclined).String())
		case inputCanceled:
			return ErrResult(tb.Reset().Str("no command was supplied: ").Err(ErrElicitCanceled).String())
		case inputMalformed:
			return ErrResult(tb.Reset().Str("could not read the supplied command: ").Err(ErrElicitMalformed).String())
		}
		result, err := s.dispatch.JSON(s.context(), plugin.CallerIdentity{Username: s.username, RemoteAddr: s.remoteAddr}, command)
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
//
// This is the path EVERY auto-generated tool takes, and therefore the path
// every task takes. The request-scoped context (s.context) is what makes the
// task execution deadline and a client disconnect reach the dispatcher at
// all.
func (s *server) run(command string) map[string]any {
	output, err := s.dispatch.JSON(s.context(), plugin.CallerIdentity{Username: s.username, RemoteAddr: s.remoteAddr}, command)
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
//
// The ze_execute descriptor carries NO `required` array here. That is the
// capability-dependent half of the contract, and not an omission.
// gateExecuteCommandRequired (mrtr.go) adds the array back for a client that
// did not declare form-mode elicitation, because that client must supply the
// argument.
//
// A client that DID declare it can omit `command` and receive the Multi
// Round-Trip input request instead. An unconditional `required` here would
// therefore tell a schema-validating host that the one call which reaches Ze's
// only elicitation is malformed. And the host would never make that call.
var handcraftedTools = []map[string]any{
	{
		"name": toolNameExecute,
		"description": "Execute a ze CLI command and return the result. Supply 'command'. " +
			"A client that declared form-mode elicitation may omit it: the call then returns " +
			"resultType \"input_required\" asking for the command, which the client answers by " +
			"retrying the call with 'inputResponses'. A client that did not declare form-mode " +
			"elicitation must supply it; omitting it returns an error naming the missing argument.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				elicitFieldCommand: map[string]any{
					"type":        "string",
					"description": "The ze command to execute (e.g., 'show bgp peer list', 'show bgp summary').",
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
