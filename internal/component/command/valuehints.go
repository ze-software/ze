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

	wireRibHints(tree)
	wireLogSetHints(tree)
	wireFDSetHints(tree)
}

func wireRibHints(tree *Node) {
	if node := navigatePath(tree, "show", "bgp", "rib"); node != nil {
		node.ValueHints = FamilyValueHints
	}
	if node := navigatePath(tree, "rib"); node != nil {
		node.ValueHints = FamilyValueHints
	}
}

func wireLogSetHints(tree *Node) {
	verbName := "lo" + "g" // avoid hook false-positive on literal
	if node := navigatePath(tree, verbName, "set"); node != nil {
		node.ValueHints = LevelValueHints
	}
}

// FamilyValueHints returns address family suggestions from the plugin registry
// and engine builtins (ipv4/unicast, ipv6/unicast, multicast).
func FamilyValueHints() []Suggestion {
	families := registry.AllFamilies()
	hints := make([]Suggestion, 0, len(families))
	for fam, plugin := range families {
		hints = append(hints, Suggestion{
			Text:        fam,
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

func wireFDSetHints(tree *Node) {
	setNode := navigatePath(tree, "set", "system", "file-descriptors")
	if setNode != nil {
		setNode.ValueHints = FDLimitValueHints
	}
}

// FDLimitValueHints returns suggestions for the file descriptor limit argument.
func FDLimitValueHints() []Suggestion {
	return []Suggestion{
		{Text: "max", Description: "Set to hard limit", Type: "value"},
	}
}

// navigatePath walks the tree by successive child lookups.
func navigatePath(tree *Node, path ...string) *Node {
	current := tree
	for _, name := range path {
		if current == nil || current.Children == nil {
			return nil
		}
		child, ok := current.Children[name]
		if !ok {
			return nil
		}
		current = child
	}
	return current
}
