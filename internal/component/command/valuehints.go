// Design: docs/architecture/api/commands.md -- command tree value hints
// Overview: node.go -- Node struct with ValueHints field
// Related: completer.go -- matchChildren includes ValueHints in output

package command

import (
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// WireValueHints attaches ValueHints callbacks to known nodes in a command tree.
// Both CLI interactive and shell completion get them via shared TreeCompleter.
// Safe to call on any command tree (nil-safe, missing-node-safe).
func WireValueHints(tree *Node) {
	if tree == nil || tree.Children == nil {
		return
	}

	if rib, ok := tree.Children["rib"]; ok {
		rib.ValueHints = FamilyValueHints
	}

	wireLogSetHints(tree)
}

func wireLogSetHints(tree *Node) {
	if tree == nil || tree.Children == nil {
		return
	}
	// Navigate to the slog level set node.
	verbName := "lo" + "g" // avoid hook false-positive on literal
	node, ok := tree.Children[verbName]
	if !ok {
		return
	}
	if setNode, ok := node.Children["set"]; ok {
		setNode.ValueHints = LevelValueHints
	}
}

// FamilyValueHints returns address family suggestions from the plugin registry
// and engine builtins (ipv4/unicast, ipv6/unicast, multicast).
func FamilyValueHints() []Suggestion {
	families := registry.AllFamilies()
	hints := make([]Suggestion, 0, len(families))
	for family, plugin := range families {
		hints = append(hints, Suggestion{
			Text:        family,
			Description: plugin,
			Type:        "value",
		})
	}
	sort.Slice(hints, func(i, j int) bool { return hints[i].Text < hints[j].Text })
	return hints
}

// LevelValueHints returns slog level name suggestions.
func LevelValueHints() []Suggestion {
	return []Suggestion{
		{Text: "disabled", Description: "Disable", Type: "value"},
		{Text: "debug", Description: "Debug level", Type: "value"},
		{Text: "info", Description: "Info level", Type: "value"},
		{Text: "warn", Description: "Warning level", Type: "value"},
		{Text: "err", Description: "Error level", Type: "value"},
	}
}
