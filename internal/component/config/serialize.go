// Design: docs/architecture/config/syntax.md — config parsing and loading
// Related: tree.go — Tree data structure
// Related: serialize_annotated.go — column-aware annotated serialization
// Related: prune.go — inactive node pruning

package config

import (
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// maxInlineDepth controls the maximum container inlining depth in serialized output.
// When set to 1, a container with exactly one leaf child is serialized inline
// (e.g., "local ip 1.2.3.4" instead of "local {\n\tip 1.2.3.4\n}").
// Only leaf children (values/multiValues) trigger inlining -- container and list
// children do not, which naturally prevents cascading beyond one level.
const maxInlineDepth = 1

// canInlineContainer reports whether a container's tree data has exactly one
// leaf-like child (value or multiValue), and no containers or lists.
// The inactive leaf is excluded from the count. A container with any
// deactivated leaf cannot inline -- the "inactive: " prefix only renders
// correctly on a multi-line statement, not in inline form.
// CanInlineContainer reports whether a container tree would be serialized inline.
func CanInlineContainer(tree *Tree) bool {
	return canInlineContainer(tree)
}

func canInlineContainer(tree *Tree) bool {
	if maxInlineDepth < 1 {
		return false
	}
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	if len(tree.inactiveValues) > 0 {
		return false
	}
	// A deactivated leaf-list member needs its own `inactive: <leaf>
	// <member>` statement line, which inline form cannot carry.
	if len(tree.inactiveMembers) > 0 {
		return false
	}
	// A comment inside the container, the one on its leaf included, has no
	// line of its own in the inline form. The container's own comments do:
	// above the line and after it.
	if tree.hasInnerCommentsLocked() {
		return false
	}
	return (len(tree.values)+len(tree.multiValues)) == 1 &&
		len(tree.containers) == 0 && len(tree.lists) == 0
}

// serializeContainerInline writes a container with a single leaf child inline:
// "containerName childName value\n" without braces, with the container's own
// comments above the line and after it.
func serializeContainerInline(b *textbuf.Buffer, child *Tree, name string, node *ContainerNode, indent int) {
	child.mu.RLock()
	defer child.mu.RUnlock()

	prefix := strings.Repeat("\t", indent)
	comments := child.comments.Statement
	writeCommentsAbove(b, prefix, comments)
	b.Str(prefix)
	if child.inactive {
		b.Str("inactive: ")
	}
	b.Str(name)
	writeInlineContainerChild(b, child, node)
	writeCommentAfter(b, comments.Trailing)
	b.Str("\n")
}

// writeInlineContainerChild writes the one leaf of an inline container, " name
// value". Caller holds child.mu.RLock.
func writeInlineContainerChild(b *textbuf.Buffer, child *Tree, node *ContainerNode) {
	// Find the single child in schema order and write it inline.
	for _, childName := range node.Children() {
		childNode := node.Get(childName)
		if writeInlineLeaf(b, child, childName, childNode) {
			return
		}
	}

	// Fallback: extra values not in schema.
	for k, v := range child.values {
		b.Str(" ")
		b.Str(k)
		b.Str(" ")
		b.Str(quoteIfNeeded(v))
		return
	}
}

// writeInlineLeaf writes a leaf value inline (without prefix or newline).
// Returns true if the child had data and was written.
//
// Caller MUST hold tree.mu.RLock() -- this helper reads tree.values and
// tree.multiValues directly rather than going through Get/GetSlice (which
// would attempt to re-acquire the lock).
func writeInlineLeaf(b *textbuf.Buffer, tree *Tree, name string, node Node) bool {
	switch n := node.(type) {
	case *LeafNode:
		if v, ok := tree.values[name]; ok {
			b.Str(" ")
			b.Str(name)
			// "type empty" leaves inline as a bare flag with no value.
			if n.Type != TypeEmpty {
				b.Str(" ")
				b.Str(quoteIfNeeded(normalizeBool(v)))
			}
			return true
		}
	case *MultiLeafNode:
		if v, ok := tree.values[name]; ok {
			b.Str(" ")
			b.Str(name)
			b.Str(" ")
			b.Str(v)
			return true
		}
	case *BracketLeafListNode:
		if v, ok := tree.values[name]; ok {
			b.Str(" ")
			b.Str(name)
			b.Str(" [ ")
			b.Str(v)
			b.Str(" ]")
			return true
		}
	case *ValueOrArrayNode:
		if items := tree.multiValues[name]; len(items) > 0 {
			b.Str(" ")
			b.Str(name)
			if len(items) == 1 {
				b.Str(" ")
				b.Str(quoteIfNeeded(items[0]))
			} else {
				b.Str(" [ ")
				for i, item := range items {
					if i > 0 {
						b.Str(" ")
					}
					b.Str(quoteIfNeeded(item))
				}
				b.Str(" ]")
			}
			return true
		}
	}
	return false
}

// StripListKeySuffix removes the #N suffix added by AddListEntry for duplicate keys.
// For example, "10.0.0.10#1" becomes "10.0.0.10".
func StripListKeySuffix(key string) string {
	if idx := strings.LastIndex(key, "#"); idx > 0 {
		suffix := key[idx+1:]
		if _, err := strconv.Atoi(suffix); err == nil {
			return key[:idx]
		}
	}
	return key
}

// normalizeBool converts internal boolean values to config format.
// Converts true → enable, false → disable.
func normalizeBool(v string) string {
	switch v {
	case configTrue:
		return configEnable
	case configFalse:
		return configDisable
	default:
		return v
	}
}

// Serialize converts a Tree back to config text format. A ze:sensitive leaf
// prints in its $9$ form (encodeSecrets), never in clear: every writer of
// config text (fmt, migrate, the editor) prints through here or SerializeSet.
//
// Every comment the tree carries is printed beside its statement
// (serialize_comments.go). The file's head comes first, so a schema stamp the
// parser read from line 1 stays on line 1 for ScanStampRelease.
func Serialize(tree *Tree, schema *Schema) string {
	var b textbuf.Buffer
	encoded := EncodeSecrets(tree, schema)
	writeBlockBody(&b, "", encoded.commentsOfBlock(), func(body *textbuf.Buffer) {
		serializeTree(body, encoded, schema.root, 0)
	})
	return b.String()
}

// ChildProvider is any schema node with children that can be iterated.
type ChildProvider interface {
	Children() []string
	Get(name string) Node
}

// SerializeSubtree serializes a subtree using the given schema node for ordering.
// Works with *ContainerNode, *ListNode, or *FlexNode.
func SerializeSubtree(tree *Tree, node Node) string {
	cp, ok := node.(ChildProvider)
	if !ok {
		return ""
	}
	var b textbuf.Buffer
	serializeWithChildren(&b, tree, cp, 0)
	return b.String()
}

// serializeExtraValues writes tree values that are not in the schema's children list.
// This handles unknown/extra keys that appear in the config but aren't defined in schema.
func serializeExtraValues(b *textbuf.Buffer, tree *Tree, children []string, indent int) {
	prefix := strings.Repeat("\t", indent)

	schemaNames := make(map[string]bool, len(children))
	for _, name := range children {
		schemaNames[name] = true
	}

	var valueKeys []string
	for k := range tree.values {
		if !schemaNames[k] {
			valueKeys = append(valueKeys, k)
		}
	}
	slices.Sort(valueKeys)
	for _, k := range valueKeys {
		comments := tree.leafCommentsLocked(k, "")
		writeCommentsAbove(b, prefix, comments)
		b.Str(prefix)
		if tree.inactiveValues[k] {
			b.Str("inactive: ")
		}
		b.Str(k)
		b.Str(" ")
		b.Str(quoteIfNeeded(tree.values[k]))
		writeCommentAfter(b, comments.Trailing)
		b.Str("\n")
	}
}

// serializeWithChildren serializes tree content using a schema node that provides
// Children() and Get() for ordering.
//
// Holds tree.mu.RLock for the duration so callees can read tree.values /
// tree.containers / tree.lists directly. Recursion into child trees acquires
// the child's own lock independently.
func serializeWithChildren(b *textbuf.Buffer, tree *Tree, node ChildProvider, indent int) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	for _, name := range node.Children() {
		child := node.Get(name)
		serializeNode(b, tree, name, child, indent)
	}

	serializeExtraValues(b, tree, node.Children(), indent)
}

// serializeTree is the primary walker entry; holds tree.mu.RLock across the
// schema-ordered walk. Recursion crosses to child trees via serializeNode,
// which re-enters serializeTree / serializeListEntry / etc. on a different
// tree that locks its own mutex.
func serializeTree(b *textbuf.Buffer, tree *Tree, node *ContainerNode, indent int) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	for _, name := range node.Children() {
		child := node.Get(name)
		serializeNode(b, tree, name, child, indent)
	}

	serializeExtraValues(b, tree, node.Children(), indent)
}

func serializeNode(b *textbuf.Buffer, tree *Tree, name string, node Node, indent int) {
	prefix := strings.Repeat("\t", indent)

	// Serialize is the file form `ze config fmt` and `ze config migrate`
	// write, so a ze:hidden node the tree holds is printed: hidden keeps a
	// node out of completion and the web editor, and dropping it here would
	// delete operator config on write. Only ze:ephemeral is never persisted.
	switch n := node.(type) {
	case *LeafNode:
		if n.Ephemeral {
			break
		}
		if v, ok := tree.values[name]; ok {
			comments := tree.leafCommentsLocked(name, "")
			writeCommentsAbove(b, prefix, comments)
			b.Str(prefix)
			if tree.inactiveValues[name] {
				b.Str("inactive: ")
			}
			b.Str(name)
			// "type empty" leaves are bare presence flags ("no-default-route");
			// the tokenizer's ASI supplies the terminating ';' at the newline.
			if n.Type != TypeEmpty {
				b.Str(" ")
				b.Str(quoteIfNeeded(normalizeBool(v)))
			}
			writeCommentAfter(b, comments.Trailing)
			b.Str("\n")
		}

	case *MultiLeafNode:
		if v, ok := tree.values[name]; ok {
			comments := tree.leafCommentsLocked(name, "")
			writeCommentsAbove(b, prefix, comments)
			b.Str(prefix)
			if tree.inactiveValues[name] {
				b.Str("inactive: ")
			}
			b.Str(name)
			b.Str(" ")
			b.Str(v) // Already space-separated
			writeCommentAfter(b, comments.Trailing)
			b.Str("\n")
		}

	case *BracketLeafListNode:
		serializeBracketLeafList(b, tree, name, prefix)

	case *ValueOrArrayNode:
		// Direct access: caller holds tree.mu.RLock, calling GetSlice
		// would recursively RLock the same mutex (unsafe per Go docs).
		if items := tree.multiValues[name]; len(items) > 0 {
			// Deactivated members render as the bare member in the leaf
			// line plus an `inactive: <leaf> <member>` statement (the
			// hierarchical analog of the set-format `nop <path> <member>`
			// line): the raw "inactive:" prefix would fail item validation
			// (e.g. ip-address) on reparse.
			bare, inactiveMembers := splitInactiveMembers(items, tree.inactiveMembers[name])
			comments := tree.leafCommentsLocked(name, "")
			writeCommentsAbove(b, prefix, comments)
			b.Str(prefix)
			if tree.inactiveValues[name] {
				b.Str("inactive: ")
			}
			b.Str(name)
			// A whole inactive leaf keeps its brackets even for one member:
			// `inactive: <leaf> <member>` is a member statement, like `nop`.
			if len(bare) == 1 && !tree.inactiveValues[name] {
				b.Str(" ")
				b.Str(quoteIfNeeded(bare[0]))
			} else {
				b.Str(" [ ")
				for i, item := range bare {
					if i > 0 {
						b.Str(" ")
					}
					b.Str(quoteIfNeeded(item))
				}
				b.Str(" ]")
			}
			writeCommentAfter(b, comments.Trailing)
			b.Str("\n")
			for _, member := range inactiveMembers {
				writeInactiveMemberLine(b, tree, name, member, prefix)
			}
		}

	case *ContainerNode:
		if n.Ephemeral {
			break
		}
		if n.Presence {
			serializePresenceContainer(b, tree, name, n, indent)
		} else if child := tree.containers[name]; child != nil {
			switch {
			case n.Flatten && canFlattenContainer(child, n):
				serializeFlattenedContainer(b, child, name, n, indent)
			case canInlineContainer(child):
				serializeContainerInline(b, child, name, n, indent)
			default:
				writeBlock(b, prefix, child.commentsOfBlock(), func(open *textbuf.Buffer) {
					if child.IsInactive() {
						open.Str("inactive: ")
					}
					open.Str(name)
				}, func(body *textbuf.Buffer) {
					serializeTree(body, child, n, indent+1)
				})
			}
		}

	case *ListNode:
		if n.Ephemeral {
			break
		}
		if entries := tree.lists[name]; entries != nil {
			if usesMultiBlockForm(n) && entriesArePositional(entries, n) {
				// Multi-entry block: name { key1 val1; key2; ... }
				serializeListMultiBlock(b, name, entries, n, tree.listOrder[name], indent)
			} else {
				// Individual blocks: name key { ... }
				serializeListBlocks(b, entries, tree.listOrder[name], name, n, indent, "")
			}
		}

	case *FreeformNode:
		if child := tree.containers[name]; child != nil {
			writeBlock(b, prefix, child.commentsOfBlock(), func(open *textbuf.Buffer) {
				open.Str(name)
			}, func(body *textbuf.Buffer) {
				serializeFreeform(body, child, indent+1)
			})
		}

	case *FlexNode:
		// Check if it's a simple value, multiValue, container, or list.
		// The value lines are one leaf statement for the comments: its
		// comments go around the first line.
		comments := tree.leafCommentsLocked(name, "")
		if v, ok := tree.values[name]; ok {
			writeCommentsAbove(b, prefix, comments)
			b.Str(prefix)
			b.Str(name)
			if v != configTrue {
				b.Str(" ")
				b.Str(quoteIfNeeded(v))
			}
			writeCommentAfter(b, comments.Trailing)
			b.Str("\n")
		} else if mv := tree.multiValues[name]; len(mv) > 0 {
			// Inline values (e.g., vpls rd X endpoint Y ...;)
			writeCommentsAbove(b, prefix, comments)
			for i, v := range mv {
				b.Str(prefix)
				b.Str(name)
				b.Str(" ")
				b.Str(v)
				if i == 0 {
					writeCommentAfter(b, comments.Trailing)
				}
				b.Str("\n")
			}
		}
		// Also serialize container form
		if child := tree.containers[name]; child != nil {
			writeBlock(b, prefix, child.commentsOfBlock(), func(open *textbuf.Buffer) {
				open.Str(name)
			}, func(body *textbuf.Buffer) {
				serializeFlexContainer(body, child, n, indent+1)
			})
		}
		// Also serialize list entries (e.g., vpls site5 { ... })
		if entries := tree.lists[name]; entries != nil {
			keys := make([]string, 0, len(entries))
			for k := range entries {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			for _, key := range keys {
				entry := entries[key]
				writeBlock(b, prefix, entry.commentsOfBlock(), func(open *textbuf.Buffer) {
					open.Str(name).Str(" ").Str(quoteIfNeeded(key))
				}, func(body *textbuf.Buffer) {
					serializeFlexContainer(body, entry, n, indent+1)
				})
			}
		}

	case *InlineListNode:
		if entries := tree.lists[name]; entries != nil {
			keys := make([]string, 0, len(entries))
			for k := range entries {
				keys = append(keys, k)
			}
			slices.Sort(keys)

			for _, key := range keys {
				entry := entries[key]
				// Strip #N suffix from duplicate keys for serialization
				displayKey := StripListKeySuffix(key)

				// entry is a separate Tree; lock it before inspecting
				// entry.containers / entry.lists / entry.values. The
				// block branch releases this lock before recursing
				// via serializeInlineListEntry (which re-locks).
				// A comment inside the entry has no line in the one-line
				// form, so such an entry is written as a block.
				entry.mu.RLock()
				useInline := len(entry.containers) == 0 && len(entry.lists) == 0 && !entry.hasInnerCommentsLocked()
				hasValues := useInline && len(entry.values) > 0

				if hasValues {
					statement := entry.comments.Statement
					writeCommentsAbove(b, prefix, statement)
					b.Str(prefix)
					b.Str(name)
					b.Str(" ")
					b.Str(quoteIfNeeded(displayKey))
					for _, attrName := range n.Children() {
						if v, ok := entry.values[attrName]; ok {
							b.Str(" ")
							b.Str(attrName)
							b.Str(" ")
							b.Str(quoteIfNeeded(v))
						}
					}
					// Also add any values not in schema order
					for k, v := range entry.values {
						if !n.Has(k) {
							b.Str(" ")
							b.Str(k)
							b.Str(" ")
							b.Str(quoteIfNeeded(v))
						}
					}
					entry.mu.RUnlock()
					writeCommentAfter(b, statement.Trailing)
					b.Str("\n")
				} else {
					entry.mu.RUnlock()
					writeBlock(b, prefix, entry.commentsOfBlock(), func(open *textbuf.Buffer) {
						open.Str(name).Str(" ").Str(quoteIfNeeded(displayKey))
					}, func(body *textbuf.Buffer) {
						serializeInlineListEntry(body, entry, n, indent+1)
					})
				}
			}
		}
	}
}

// allChildrenAreLeaves reports whether all children of a ListNode are simple leaves.
// Used to decide between multi-entry block (positional inline) and individual block serialization.
func allChildrenAreLeaves(n *ListNode) bool {
	for _, name := range n.Children() {
		if _, ok := n.Get(name).(*LeafNode); !ok {
			return false
		}
	}
	return true
}

// entriesArePositional reports whether every entry can be written in the
// positional multi-entry form. A value is read back by its position, so an
// entry that sets a later child while an earlier one is absent would print
// `key 1` for priority 1 and read back as link-local 1. Such a list is
// written one block per entry instead, where every value carries its name.
// So is a list with a comment inside an entry, which a positional line has no
// room for, and a list with an entry whose key is one of its child names: the
// parser reads `name { use 000 }` as an anonymous entry setting `use`
// (parseList), while `name use { ... }` keeps `use` as the key.
func entriesArePositional(entries map[string]*Tree, n *ListNode) bool {
	for key := range entries {
		if n.Get(key) != nil {
			return false
		}
	}
	for _, entry := range entries {
		entry.mu.RLock()
		inner := entry.hasInnerCommentsLocked()
		entry.mu.RUnlock()
		if inner {
			return false
		}
		gap := false
		for _, name := range n.Children() {
			if _, ok := entry.Get(name); !ok {
				gap = true
				continue
			}
			if gap {
				return false
			}
		}
	}
	return true
}

// serializeListMultiBlock serializes list entries as a grouped block with positional inline entries.
// Output: name { key1; key2 val1; key3 val1 val2; }.
//
// The parser gives every comment of this statement to its last entry
// (parseListInlineBlock), so the printer writes the comments of all entries
// around the whole statement: above `name {` and after its `}`.
func serializeListMultiBlock(b *textbuf.Buffer, name string, entries map[string]*Tree, node *ListNode, order []string, indent int) {
	prefix := strings.Repeat("\t", indent)
	innerPrefix := strings.Repeat("\t", indent+1)

	// Use insertion order if available, otherwise sort
	keys := order
	if len(keys) == 0 {
		keys = make([]string, 0, len(entries))
		for k := range entries {
			keys = append(keys, k)
		}
		slices.Sort(keys)
	}

	var comments statementComments
	for _, key := range keys {
		if entry := entries[key]; entry != nil {
			entryComments := entry.commentsOfBlock().Statement
			comments.Leading = append(comments.Leading, entryComments.Leading...)
			comments.Trailing = append(comments.Trailing, entryComments.Trailing...)
		}
	}
	writeCommentsAbove(b, prefix, comments)
	b.Str(prefix)
	b.Str(name)
	b.Str(" {\n")

	for _, key := range keys {
		entry := entries[key]
		if entry == nil {
			continue
		}
		displayKey := StripListKeySuffix(key)
		b.Str(innerPrefix)
		b.Str(quoteIfNeeded(ListKeyOpeningValue(node, displayKey)))

		// Positional children in definition order
		for _, childName := range node.Children() {
			if v, ok := entry.Get(childName); ok {
				b.Str(" ")
				b.Str(quoteIfNeeded(v))
			}
		}
		b.Str("\n")
	}

	b.Str(prefix)
	b.Str("}")
	writeCommentAfter(b, comments.Trailing)
	b.Str("\n")
}

// serializeListBlocks writes one block per list entry: "name key { ... }".
// keyword is empty for a plain list, and holds the parent container's name for
// a child of a ze:flatten container, which writes "attach process key { ... }"
// (flatten.go).
// order is the entry order the tree recorded for the list; listBlockKeys
// decides whether it is printed.
func serializeListBlocks(b *textbuf.Buffer, entries map[string]*Tree, order []string, name string, node *ListNode, indent int, keyword string) {
	prefix := strings.Repeat("\t", indent)

	for _, key := range listBlockKeys(entries, node, order) {
		entry := entries[key]
		comments := entry.commentsOfBlock()
		open := func(line *textbuf.Buffer) {
			if entry.IsInactive() {
				line.Str("inactive: ")
			}
			if keyword != "" {
				line.Str(keyword)
				line.Str(" ")
			}
			line.Str(name)
			// Skip outputting KeyDefault - it's the implicit default
			if key != KeyDefault {
				line.Str(" ")
				line.Str(quoteIfNeeded(ListKeyOpeningValue(node, key)))
			}
		}
		if members, ok := inlineLeafListEntry(entry, node, key); ok {
			writeCommentsAbove(b, prefix, comments.Statement)
			b.Str(prefix)
			open(b)
			writeInlineLeafListMembers(b, members)
			writeCommentAfter(b, comments.Statement.Trailing)
			b.Str("\n")
			continue
		}
		writeBlock(b, prefix, comments, open, func(body *textbuf.Buffer) {
			serializeListEntry(body, entry, node, indent+1)
		})
	}
}

// listBlockKeys returns the keys of a list's entries in the order the file form
// prints them.
//
// YANG `ordered-by user` makes the entry order part of the value: a firewall
// chain's terms, a prefix filter's entries and an AAA server list are tried
// first to last, and their consumers read that order through
// configorder.Entries or GetListOrdered. Such a list prints in the order the
// tree recorded, so a rewrite (fmt, migrate) never changes which entry matches
// first. An entry the record misses is printed after the recorded ones, sorted,
// because dropping it would lose configuration.
//
// Every other list prints sorted by key, which is the canonical form: its
// consumers do not read the order (the IKE and ESP proposals, for one, are
// sorted by their number in ike/ipsec config.go).
func listBlockKeys(entries map[string]*Tree, node *ListNode, order []string) []string {
	keys := make([]string, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	if node.UserOrdered {
		for _, key := range order {
			if _, ok := entries[key]; !ok {
				continue // The entry this key recorded was removed.
			}
			if seen[key] {
				continue // The record names this key twice.
			}
			seen[key] = true
			keys = append(keys, key)
		}
	}
	rest := make([]string, 0, len(entries)-len(keys))
	for key := range entries {
		if !seen[key] {
			rest = append(rest, key)
		}
	}
	slices.Sort(rest)
	return append(keys, rest...)
}

// inlineLeafListEntry reports whether a list entry renders as one line,
// "name key [ member member ]", and returns the members when it does.
//
// This is the writer half of parseListInlineEntry: that reader gives the bracket
// members to the LAST child of the entry, so the form is only unambiguous when
// the list has exactly ONE child. A list with more children keeps the block form,
// because the one-line form would drop the child NAME and leave a reader unable
// to tell which leaf the members landed in.
//
// The other four refusals each name something the one line cannot carry: the
// implicit KeyDefault entry has no key to write the members after, a value
// outside the schema, a deactivated leaf and a deactivated member each own a
// line of their own.
func inlineLeafListEntry(entry *Tree, node *ListNode, key string) ([]string, bool) {
	if key == KeyDefault {
		return nil, false
	}
	children := node.Children()
	if len(children) != 1 {
		return nil, false
	}
	name := children[0]
	if !isInlineLeafListChild(node.Get(name)) {
		return nil, false
	}

	entry.mu.RLock()
	defer entry.mu.RUnlock()

	if len(entry.containers) != 0 || len(entry.lists) != 0 {
		return nil, false
	}
	if len(entry.inactiveValues) != 0 || len(entry.inactiveMembers) != 0 {
		return nil, false
	}
	// A comment inside the entry has no line in the one-line form either.
	if entry.hasInnerCommentsLocked() {
		return nil, false
	}
	for valueName := range entry.values {
		if valueName != name {
			return nil, false
		}
	}
	members := entry.multiValues[name]
	if len(members) == 0 {
		return nil, false
	}
	return members, true
}

// writeInlineLeafListMembers writes the " [ a b ]" tail of an inline list entry.
// The caller ends the line, after the entry's trailing comment if it has one:
// the newline terminates the statement, the tokenizer inserting the ';' there
// the same way every other leaf line in this file ends.
func writeInlineLeafListMembers(b *textbuf.Buffer, members []string) {
	b.Str(" [ ")
	for i, member := range members {
		if i > 0 {
			b.Str(" ")
		}
		b.Str(quoteIfNeeded(member))
	}
	b.Str(" ]")
}

func serializeListEntry(b *textbuf.Buffer, tree *Tree, node *ListNode, indent int) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	for _, name := range node.Children() {
		child := node.Get(name)
		serializeNode(b, tree, name, child, indent)
	}

	serializeExtraValues(b, tree, node.Children(), indent)
}

func serializeFreeform(b *textbuf.Buffer, tree *Tree, indent int) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	prefix := strings.Repeat("\t", indent)

	// Sort keys for deterministic output
	keys := make([]string, 0, len(tree.values))
	for k := range tree.values {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, k := range keys {
		v := tree.values[k]
		b.Str(prefix)
		b.Str(k)
		if v != configTrue {
			if strings.HasPrefix(v, "[ ") && strings.HasSuffix(v, " ]") {
				// Already bracketed — output as-is
				b.Str(" ")
				b.Str(v)
			} else {
				// Wrap in brackets to preserve roundtrip
				b.Str(" [ ")
				b.Str(v)
				b.Str(" ]")
			}
		}
		b.Str("\n")
	}
}

// serializePresenceContainer serializes a presence container in flag, value, or block form.
// Mirrors FlexNode serialization: checks values, multiValues, containers, and lists.
func serializePresenceContainer(b *textbuf.Buffer, tree *Tree, name string, node *ContainerNode, indent int) {
	prefix := strings.Repeat("\t", indent)

	// Check for simple value or flag. The value lines are one leaf statement
	// for the comments, as in the FlexNode branch of serializeNode.
	comments := tree.leafCommentsLocked(name, "")
	if v, ok := tree.values[name]; ok {
		writeCommentsAbove(b, prefix, comments)
		b.Str(prefix)
		b.Str(name)
		if v != configTrue {
			b.Str(" ")
			b.Str(quoteIfNeeded(v))
		}
		writeCommentAfter(b, comments.Trailing)
		b.Str("\n")
	} else if mv := tree.multiValues[name]; len(mv) > 0 {
		writeCommentsAbove(b, prefix, comments)
		for i, v := range mv {
			b.Str(prefix)
			b.Str(name)
			b.Str(" ")
			b.Str(v)
			if i == 0 {
				writeCommentAfter(b, comments.Trailing)
			}
			b.Str("\n")
		}
	}

	// Block form (container children)
	if child := tree.containers[name]; child != nil {
		writeBlock(b, prefix, child.commentsOfBlock(), func(open *textbuf.Buffer) {
			if child.IsInactive() {
				open.Str("inactive: ")
			}
			open.Str(name)
		}, func(body *textbuf.Buffer) {
			serializeTree(body, child, node, indent+1)
		})
	}
}

func serializeFlexContainer(b *textbuf.Buffer, tree *Tree, node *FlexNode, indent int) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	for _, name := range node.Children() {
		child := node.Get(name)
		serializeNode(b, tree, name, child, indent)
	}

	serializeExtraValues(b, tree, node.Children(), indent)
}

func serializeInlineListEntry(b *textbuf.Buffer, tree *Tree, node *InlineListNode, indent int) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	for _, name := range node.Children() {
		child := node.Get(name)
		serializeNode(b, tree, name, child, indent)
	}

	serializeExtraValues(b, tree, node.Children(), indent)
}

// serializeBracketLeafList writes a bracket leaf-list from its member store:
// every member, active or not, then one `inactive: <leaf> <member>` statement
// for each deactivated member, the form parseNodeInactive reads back. The
// mirror names the active members only, so writing it would delete every
// deactivated member on the next save. Caller holds tree.mu.RLock.
func serializeBracketLeafList(b *textbuf.Buffer, tree *Tree, name, prefix string) {
	body, ok := bracketBody(tree, name)
	if !ok {
		return
	}
	comments := tree.leafCommentsLocked(name, "")
	writeCommentsAbove(b, prefix, comments)
	b.Str(prefix)
	if tree.inactiveValues[name] {
		b.Str("inactive: ")
	}
	b.Str(name).Str(" [ ").Str(body).Str(" ]")
	writeCommentAfter(b, comments.Trailing)
	b.Byte('\n')
	for _, member := range tree.multiValues[name] {
		if tree.inactiveMembers[name][member] {
			writeInactiveMemberLine(b, tree, name, member, prefix)
		}
	}
}

// writeInactiveMemberLine writes the statement `inactive: <name> <member>`
// with the comments the parser attached to that line. Caller holds
// tree.mu.RLock.
func writeInactiveMemberLine(b *textbuf.Buffer, tree *Tree, name, member, prefix string) {
	comments := tree.leafCommentsLocked(name, member)
	writeCommentsAbove(b, prefix, comments)
	b.Str(prefix).Str("inactive: ").Str(name).Byte(' ').Str(quoteIfNeeded(member))
	writeCommentAfter(b, comments.Trailing)
	b.Byte('\n')
}

// bracketBody answers the text between a bracket leaf-list's brackets: every
// member, quoted when it needs it, so both parsers read the same members back.
// A value with no members (Tree.Set on a name no parser or editor stored as a
// leaf-list) is answered as it stands. Caller holds tree.mu.RLock.
func bracketBody(tree *Tree, name string) (string, bool) {
	if items := tree.multiValues[name]; len(items) > 0 {
		return JoinLeafListMembers(items), true
	}
	v, ok := tree.values[name]
	return v, ok
}
