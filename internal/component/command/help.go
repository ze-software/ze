// Design: docs/architecture/api/commands.md -- dynamic help generation
// Related: node.go -- command tree types

package command

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// readOnlyVerbs are verbs that do not modify state.
var readOnlyVerbs = map[string]bool{
	"show":     true,
	"validate": true,
	"monitor":  true,
}

// IsReadOnlyVerb returns true if the verb does not modify state.
// show, validate, and monitor are read-only.
// set, del, and update are mutating.
func IsReadOnlyVerb(verb string) bool {
	return readOnlyVerbs[verb]
}

// FindNode navigates the tree by the given path and returns the node,
// or nil if any segment is not found. Returns nil if root is nil.
func FindNode(root *Node, path []string) *Node {
	if root == nil {
		return nil
	}
	current := root
	for _, segment := range path {
		if current.Children == nil {
			return nil
		}
		child, ok := current.Children[segment]
		if !ok {
			return nil
		}
		current = child
	}
	return current
}

// WriteHelp writes formatted help for the node at the given path.
// If path is nil/empty, writes top-level help (lists children of root).
// Returns true if the path was found, false if the path does not exist or root is nil.
func WriteHelp(w io.Writer, root *Node, path []string) bool {
	if root == nil {
		return false
	}
	node := root
	if len(path) > 0 {
		node = FindNode(root, path)
		if node == nil {
			return false
		}
	}

	if len(node.Children) == 0 {
		if node.Description != "" {
			writeHelpLine(w, node.Description)
		}
		return true
	}

	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		child := node.Children[name]
		desc := child.Description
		if desc == "" && len(child.Children) > 0 {
			desc = describeChildren(child)
		}
		writeHelpEntry(w, name, desc)
	}

	return true
}

// writeHelpLine writes a single indented line to w.
// Help output goes to stderr; write errors are not actionable.
func writeHelpLine(w io.Writer, text string) {
	fmt.Fprintf(w, "  %s\n", text) //nolint:errcheck // help output to stderr
}

// writeHelpEntry writes a formatted name + description line to w.
// Help output goes to stderr; write errors are not actionable.
func writeHelpEntry(w io.Writer, name, desc string) {
	fmt.Fprintf(w, "  %-16s %s\n", name, desc) //nolint:errcheck // help output to stderr
}

// describeChildren returns a summary of a node's children for grouping nodes
// that have no description of their own.
func describeChildren(node *Node) string {
	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	sort.Strings(names)

	if len(names) > 4 {
		return fmt.Sprintf("subcommands: %s, %s, %s, ... (%d total)",
			names[0], names[1], names[2], len(names))
	}

	return "subcommands: " + strings.Join(names, ", ")
}
