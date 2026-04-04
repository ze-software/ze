// Design: docs/guide/mcp/overview.md -- MCP tool auto-generation from command registry
// Overview: handler.go -- MCP HTTP handler and handcrafted tools

package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// CommandInfo describes a registered command for MCP tool generation.
type CommandInfo struct {
	Name     string // Dispatch path, e.g. "rib status", "show config dump"
	Help     string // Description from YANG
	ReadOnly bool   // True if read-only command
}

// CommandLister returns all registered commands. Called at tools/list time
// so the tool list always reflects current registrations.
type CommandLister func() []CommandInfo

// toolGroup is a set of related commands sharing a prefix.
type toolGroup struct {
	prefix  string   // e.g. "rib", "show config"
	actions []action // subcommands within the group
}

// action is a single subcommand within a group.
type action struct {
	name string // action name (suffix after prefix), e.g. "status", "dump"
	help string // description
	full string // full command path for dispatch
}

// groupCommands groups commands by their natural prefix.
// Commands like "rib status", "rib routes" group under "rib".
// Commands like "show config dump", "show config diff" group under "show config".
//
// Grouping rule: find the longest shared prefix among at least 2 commands,
// where removing the prefix leaves a non-empty suffix (the action).
// Single commands with no siblings become their own group with no action param.
func groupCommands(commands []CommandInfo) []toolGroup {
	type entry struct {
		full string
		help string
	}

	// Index commands by first-token and first-two-tokens.
	byOne := make(map[string][]entry)
	byTwo := make(map[string][]entry)

	for _, cmd := range commands {
		tokens := strings.Fields(cmd.Name)
		if len(tokens) == 0 {
			continue
		}
		one := tokens[0]
		byOne[one] = append(byOne[one], entry{full: cmd.Name, help: cmd.Help})
		if len(tokens) >= 2 {
			two := tokens[0] + " " + tokens[1]
			byTwo[two] = append(byTwo[two], entry{full: cmd.Name, help: cmd.Help})
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
					name: suffix,
					help: e.help,
					full: e.full,
				})
				used[e.full] = true
			}
			sortActions(g.actions)
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
				g.actions = append(g.actions, action{name: "", help: e.help, full: e.full})
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
				name: suffix,
				help: e.help,
				full: e.full,
			})
			used[e.full] = true
		}
		sortActions(g.actions)
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
// "rib" -> "ze_rib", "show config" -> "ze_show_config".
func toolName(prefix string) string {
	return "ze_" + strings.ReplaceAll(prefix, " ", "_")
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

	properties["arguments"] = map[string]any{
		"type":        "string",
		"description": "Additional arguments to append to the command",
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

	return map[string]any{
		"name":        name,
		"description": desc.String(),
		"inputSchema": json.RawMessage(schemaJSON),
	}
}

// dispatchGenerated handles a tools/call for an auto-generated tool.
// It builds the command string from the tool group prefix + action + arguments.
// validActions contains the server-defined action names; if non-nil, the action
// is validated against this set to prevent injection of arbitrary tokens.
func (s *server) dispatchGenerated(prefix string, validActions map[string]bool, args json.RawMessage) map[string]any {
	var p struct {
		Action    string `json:"action"`
		Arguments string `json:"arguments"`
		Peer      string `json:"peer"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return errResult("invalid arguments: " + err.Error())
	}

	if p.Peer != "" {
		if err := noSpaces("peer", p.Peer); err != nil {
			return errResult(err.Error())
		}
	}
	// Validate action against the server-defined enum to prevent injection.
	if p.Action != "" && validActions != nil && !validActions[p.Action] {
		return errResult(fmt.Sprintf("invalid action %q", p.Action))
	}
	if strings.ContainsAny(p.Action, "\n\r") {
		return errResult("action must not contain newlines")
	}
	if strings.ContainsAny(p.Arguments, "\n\r\t") {
		return errResult("arguments must not contain newlines or tabs")
	}

	var cmd strings.Builder

	if p.Peer != "" {
		fmt.Fprintf(&cmd, "peer %s ", p.Peer)
	}

	cmd.WriteString(prefix)
	if p.Action != "" {
		cmd.WriteString(" ")
		cmd.WriteString(p.Action)
	}
	if p.Arguments != "" {
		cmd.WriteString(" ")
		cmd.WriteString(p.Arguments)
	}

	return s.run(cmd.String())
}
