// Design: docs/architecture/api/commands.md -- command tree value hints
// Overview: node.go -- Node struct with ValueHints field
// Related: completer.go -- matchChildren includes ValueHints in output

package command

import (
	"sort"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/envcatalog"
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
	wireEnvHints(tree)
}

// The two command words the env value hints hang from.
const (
	commandEnv        = "env"
	commandGet        = "get"
	commandRegistered = "registered"
)

func wireEnvHints(tree *Node) {
	for _, path := range [][]string{
		{verbShow, commandEnv, commandGet},
		{verbShow, commandEnv, commandRegistered},
		{commandEnv, commandGet},
		{commandEnv, commandRegistered},
		{commandGet},
		{commandRegistered},
	} {
		if node := navigatePath(tree, path...); node != nil {
			node.ValueHints = EnvValueHints
		}
	}
}

// MergeYANGNodes creates command nodes missing from tree but present in the
// YANG tree. Local handlers (registered via MustRegisterLocal) define YANG
// command structures but don't produce RPC-tree nodes, so they'd otherwise
// be absent from completion. This fills those gaps with YANG descriptions.
func MergeYANGNodes(tree, yangTree *Node) {
	if tree == nil || yangTree == nil || yangTree.Children == nil {
		return
	}
	mergeChildren(tree, yangTree)
}

func mergeChildren(dst, src *Node) {
	if src.Children == nil {
		return
	}
	if dst.Children == nil {
		dst.Children = make(map[string]*Node, len(src.Children))
	}
	for name, srcChild := range src.Children {
		dstChild, exists := dst.Children[name]
		if !exists {
			// Both declared help texts come across. A node carrying a summary
			// and no explanation would answer an empty help page while the
			// module it came from declares one.
			dstChild = &Node{Name: name, Description: srcChild.Description, Help: srcChild.Help}
			dst.Children[name] = dstChild
		}
		mergeChildren(dstChild, srcChild)
	}
}

// EnvValueHints returns public env-key suggestions from the shared catalog.
func EnvValueHints() []Suggestion {
	entries := envcatalog.VisibleEntries()
	hints := make([]Suggestion, 0, len(entries))
	for _, e := range entries {
		hints = append(hints, Suggestion{
			Text:        e.Key,
			Description: e.Description,
			Type:        SuggestionValue,
		})
	}
	return hints
}

func wireRibHints(tree *Node) {
	if node := navigatePath(tree, "show", "bgp", "rib"); node != nil {
		node.ValueHints = FamilyValueHints
	}
	if node := navigatePath(tree, "bgp", "rib"); node != nil {
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
			Type:        SuggestionValue,
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
