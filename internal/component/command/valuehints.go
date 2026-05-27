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
// Only runtime-dynamic hints (plugin families) remain; static hints (log levels,
// FD limit) are now YANG-driven via ArgDefs.
func WireValueHints(tree *Node) {
	if tree == nil || tree.Children == nil {
		return
	}

	wireRibHints(tree)
}

func wireRibHints(tree *Node) {
	if node := navigatePath(tree, "show", "bgp", "rib"); node != nil {
		node.ValueHints = FamilyValueHints
	}
	if node := navigatePath(tree, "rib"); node != nil {
		node.ValueHints = FamilyValueHints
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
