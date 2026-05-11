// Design: docs/guide/mcp/overview.md -- MCP tool auto-generation from command registry
// Overview: handler.go -- MCP HTTP handler and handcrafted tools

package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
	Name        string           // Dispatch path, e.g. "bgp rib status", "show config dump"
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
	prefix      string           // e.g. "bgp rib", "show config"
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
// Commands like "bgp rib status", "bgp rib routes" group under "bgp rib".
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
			two := tokens[0] + " " + tokens[1]
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
// "bgp rib" -> "ze_bgp_rib", "show config" -> "ze_show_config".
func toolName(prefix string) string {
	r := strings.NewReplacer(" ", "_", "-", "_")
	return "ze_" + r.Replace(prefix)
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

	var desc strings.Builder
	fmt.Fprintf(&desc, "Commands under '%s'.", g.prefix)

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
				actionDescs = append(actionDescs, fmt.Sprintf("%s: %s", a.name, a.help))
			}
		}

		actionProp := map[string]any{
			"type": "string",
			"enum": actionEnums,
		}
		if len(actionDescs) > 0 {
			actionProp["description"] = strings.Join(actionDescs, ". ")
		} else {
			actionProp["description"] = "Action to perform"
		}
		properties["action"] = actionProp
		required = append(required, "action")

		desc.Reset()
		if len(namedActions) == 1 {
			if namedActions[0].help != "" {
				desc.WriteString(namedActions[0].help)
			} else {
				fmt.Fprintf(&desc, "Run '%s %s'.", g.prefix, namedActions[0].name)
			}
		} else {
			fmt.Fprintf(&desc, "Actions: %s.", strings.Join(actionEnums, ", "))
		}
	} else if len(g.actions) == 1 && g.actions[0].help != "" {
		desc.Reset()
		desc.WriteString(g.actions[0].help)
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
		return ErrResult("invalid arguments: " + err.Error())
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

	var cmd strings.Builder

	if peer != "" {
		fmt.Fprintf(&cmd, "peer %s ", peer)
	}

	cmd.WriteString(prefix)
	if action != "" {
		cmd.WriteString(" ")
		cmd.WriteString(action)
	}

	// Append typed YANG params as "key value" pairs.
	for key, val := range all {
		if reservedParams[key] || val == nil {
			continue
		}
		sval := fmt.Sprint(val)
		if sval == "" {
			continue
		}
		if strings.ContainsAny(sval, "\n\r\t") {
			return ErrResult(fmt.Sprintf("parameter %q must not contain newlines or tabs", key))
		}
		cmd.WriteString(" ")
		cmd.WriteString(key)
		cmd.WriteString(" ")
		cmd.WriteString(sval)
	}

	if arguments != "" {
		cmd.WriteString(" ")
		cmd.WriteString(arguments)
	}

	return s.run(cmd.String())
}
