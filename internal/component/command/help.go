// Design: docs/architecture/api/commands.md -- dynamic help generation
// Related: node.go -- command tree types

package command

import (
	"sort"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// readOnlyVerbs are verbs that do not modify state.
var readOnlyVerbs = map[string]bool{
	"show":     true,
	"validate": true,
	"monitor":  true,
}

// IsReadOnlyVerb returns true if the verb does not modify state.
// show, validate, and monitor are read-only.
// set, clear, request, del, and update are mutating.
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

// listedChildren names the children a help page lists, with the description
// each one shows, sorted by name.
//
// A `ze:modifier "choice"` child is left out: its name is never typed, so it is
// not a subcommand and not a token. The words its leaf declares reach the
// operator through the generated usage line and through completion
// (usage.go, completer.go). Every other child is listed, including a modifier
// group whose keyword the operator does type.
//
// A `ze:modifier "one-of"` child is left out for the same reason, and its
// MEMBERS take its place: each member's keyword IS a token the operator types.
// Listing the group would name a word the handler rejects and hide the words it
// accepts.
func listedChildren(node *Node) []HelpEntry {
	entries := make([]HelpEntry, 0, len(node.Children))
	for name, child := range node.Children {
		if child == nil {
			continue
		}
		if child.Modifier == ModifierChoice {
			continue
		}
		if child.Modifier == ModifierOneOf {
			for memberName, member := range child.Children {
				entries = append(entries, helpEntryFor(memberName, member))
			}
			continue
		}
		entries = append(entries, helpEntryFor(name, child))
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

// helpEntryFor answers the line one child shows: its own description, or a
// summary of its children when it states none.
func helpEntryFor(name string, node *Node) HelpEntry {
	if node == nil {
		return HelpEntry{Name: name}
	}
	desc := node.Description
	if desc == "" && len(node.Children) > 0 {
		desc = describeChildren(node)
	}
	return HelpEntry{Name: name, Desc: desc}
}

// HelpEntry is a name + summary pair for a help section row.
type HelpEntry struct {
	Name string
	Desc string
}

// HelpEntries returns the children of the node at path as name/description pairs.
// Returns nil if the path is not found or root is nil.
func HelpEntries(root *Node, path []string) []HelpEntry {
	if root == nil {
		return nil
	}
	node := root
	if len(path) > 0 {
		node = FindNode(root, path)
		if node == nil {
			return nil
		}
	}
	entries := listedChildren(node)
	if len(entries) == 0 {
		return nil
	}
	return entries
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
		var b textbuf.Buffer
		return b.Reset().Str("subcommands: ").Str(names[0]).Str(", ").Str(names[1]).Str(", ").Str(names[2]).Str(", ... (").Int(int64(len(names))).Str(" total)").String()
	}

	var tb textbuf.Buffer
	return tb.Str("subcommands: ").Join(names, ", ").String()
}
