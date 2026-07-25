// Design: docs/architecture/config/syntax.md — config parsing and loading
// Related: tree.go — Tree data structure
// Related: serialize_annotated.go — column-aware annotated serialization
// Related: prune.go — inactive node pruning

package config

import (
	"sort"
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
	return (len(tree.values)+len(tree.multiValues)) == 1 &&
		len(tree.containers) == 0 && len(tree.lists) == 0
}

// serializeContainerInline writes a container with a single leaf child inline:
// "containerName childName value\n" without braces.
func serializeContainerInline(b *textbuf.Buffer, child *Tree, name string, node *ContainerNode, indent int) {
	child.mu.RLock()
	defer child.mu.RUnlock()

	prefix := strings.Repeat("\t", indent)
	b.Str(prefix)
	if child.inactive {
		b.Str("inactive: ")
	}
	b.Str(name)

	// Find the single child in schema order and write it inline.
	for _, childName := range node.Children() {
		childNode := node.Get(childName)
		if writeInlineLeaf(b, child, childName, childNode) {
			b.Str("\n")
			return
		}
	}

	// Fallback: extra values not in schema.
	for k, v := range child.values {
		b.Str(" ")
		b.Str(k)
		b.Str(" ")
		b.Str(quoteIfNeeded(v))
		b.Str("\n")
		return
	}

	b.Str("\n")
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

// Serialize converts a Tree back to config text format.
func Serialize(tree *Tree, schema *Schema) string {
	var b textbuf.Buffer
	serializeTree(&b, tree, schema.root, 0)
	return b.String()
}

// childProvider is any schema node with children that can be iterated.
type childProvider interface {
	Children() []string
	Get(name string) Node
}

// SerializeSubtree serializes a subtree using the given schema node for ordering.
// Works with *ContainerNode, *ListNode, or *FlexNode.
func SerializeSubtree(tree *Tree, node Node) string {
	cp, ok := node.(childProvider)
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
	sort.Strings(valueKeys)
	for _, k := range valueKeys {
		b.Str(prefix)
		if tree.inactiveValues[k] {
			b.Str("inactive: ")
		}
		b.Str(k)
		b.Str(" ")
		b.Str(quoteIfNeeded(tree.values[k]))
		b.Str("\n")
	}
}

// serializeWithChildren serializes tree content using a schema node that provides
// Children() and Get() for ordering.
//
// Holds tree.mu.RLock for the duration so callees can read tree.values /
// tree.containers / tree.lists directly. Recursion into child trees acquires
// the child's own lock independently.
func serializeWithChildren(b *textbuf.Buffer, tree *Tree, node childProvider, indent int) {
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

	switch n := node.(type) {
	case *LeafNode:
		if n.Hidden || n.Ephemeral {
			break
		}
		if v, ok := tree.values[name]; ok {
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
			b.Str("\n")
		}

	case *MultiLeafNode:
		if v, ok := tree.values[name]; ok {
			b.Str(prefix)
			if tree.inactiveValues[name] {
				b.Str("inactive: ")
			}
			b.Str(name)
			b.Str(" ")
			b.Str(v) // Already space-separated
			b.Str("\n")
		}

	case *BracketLeafListNode:
		if v, ok := tree.values[name]; ok {
			b.Str(prefix)
			if tree.inactiveValues[name] {
				b.Str("inactive: ")
			}
			b.Str(name)
			b.Str(" [ ")
			b.Str(v) // Space-separated items
			b.Str(" ]\n")
		}

	case *ValueOrArrayNode:
		// Direct access: caller holds tree.mu.RLock, calling GetSlice
		// would recursively RLock the same mutex (unsafe per Go docs).
		if items := tree.multiValues[name]; len(items) > 0 {
			// Deactivated members render as the bare member in the leaf
			// line plus an `inactive: <leaf> <member>` statement (the
			// hierarchical analog of the set-format `inactive <path>
			// <member>` line): the raw "inactive:" prefix would fail item
			// validation (e.g. ip-address) on reparse.
			bare, inactiveMembers := splitInactiveMembers(items, tree.inactiveMembers[name])
			b.Str(prefix)
			if tree.inactiveValues[name] {
				b.Str("inactive: ")
			}
			b.Str(name)
			if len(bare) == 1 {
				b.Str(" ")
				b.Str(quoteIfNeeded(bare[0]))
				b.Str("\n")
			} else {
				b.Str(" [ ")
				for i, item := range bare {
					if i > 0 {
						b.Str(" ")
					}
					b.Str(quoteIfNeeded(item))
				}
				b.Str(" ]\n")
			}
			for _, member := range inactiveMembers {
				b.Str(prefix)
				b.Str("inactive: ")
				b.Str(name)
				b.Str(" ")
				b.Str(quoteIfNeeded(member))
				b.Str("\n")
			}
		}

	case *ContainerNode:
		if n.Hidden || n.Ephemeral {
			break
		}
		if n.Presence {
			serializePresenceContainer(b, tree, name, n, indent)
		} else if child := tree.containers[name]; child != nil {
			if canInlineContainer(child) {
				serializeContainerInline(b, child, name, n, indent)
			} else {
				b.Str(prefix)
				if child.IsInactive() {
					b.Str("inactive: ")
				}
				b.Str(name)
				b.Str(" {\n")
				serializeTree(b, child, n, indent+1)
				b.Str(prefix)
				b.Str("}\n")
			}
		}

	case *ListNode:
		if n.Hidden || n.Ephemeral {
			break
		}
		if entries := tree.lists[name]; entries != nil {
			if n.KeyName != "" && len(n.Children()) <= 2 && allChildrenAreLeaves(n) {
				// Multi-entry block: name { key1 val1; key2; ... }
				serializeListMultiBlock(b, name, entries, n, tree.listOrder[name], indent)
			} else {
				// Individual blocks: name key { ... }
				keys := make([]string, 0, len(entries))
				for k := range entries {
					keys = append(keys, k)
				}
				sort.Strings(keys)

				for _, key := range keys {
					entry := entries[key]
					b.Str(prefix)
					if entry.IsInactive() {
						b.Str("inactive: ")
					}
					b.Str(name)
					// Skip outputting KeyDefault - it's the implicit default
					if key != KeyDefault {
						b.Str(" ")
						b.Str(quoteIfNeeded(key))
					}
					b.Str(" {\n")
					serializeListEntry(b, entry, n, indent+1)
					b.Str(prefix)
					b.Str("}\n")
				}
			}
		}

	case *FreeformNode:
		if child := tree.containers[name]; child != nil {
			b.Str(prefix)
			b.Str(name)
			b.Str(" {\n")
			serializeFreeform(b, child, indent+1)
			b.Str(prefix)
			b.Str("}\n")
		}

	case *FlexNode:
		// Check if it's a simple value, multiValue, container, or list
		if v, ok := tree.values[name]; ok {
			b.Str(prefix)
			b.Str(name)
			if v != configTrue {
				b.Str(" ")
				b.Str(quoteIfNeeded(v))
			}
			b.Str("\n")
		} else if mv := tree.multiValues[name]; len(mv) > 0 {
			// Inline values (e.g., vpls rd X endpoint Y ...;)
			for _, v := range mv {
				b.Str(prefix)
				b.Str(name)
				b.Str(" ")
				b.Str(v)
				b.Str("\n")
			}
		}
		// Also serialize container form
		if child := tree.containers[name]; child != nil {
			b.Str(prefix)
			b.Str(name)
			b.Str(" {\n")
			serializeFlexContainer(b, child, n, indent+1)
			b.Str(prefix)
			b.Str("}\n")
		}
		// Also serialize list entries (e.g., vpls site5 { ... })
		if entries := tree.lists[name]; entries != nil {
			keys := make([]string, 0, len(entries))
			for k := range entries {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, key := range keys {
				entry := entries[key]
				b.Str(prefix)
				b.Str(name)
				b.Str(" ")
				b.Str(quoteIfNeeded(key))
				b.Str(" {\n")
				serializeFlexContainer(b, entry, n, indent+1)
				b.Str(prefix)
				b.Str("}\n")
			}
		}

	case *InlineListNode:
		if entries := tree.lists[name]; entries != nil {
			keys := make([]string, 0, len(entries))
			for k := range entries {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, key := range keys {
				entry := entries[key]
				// Strip #N suffix from duplicate keys for serialization
				displayKey := StripListKeySuffix(key)

				// entry is a separate Tree; lock it before inspecting
				// entry.containers / entry.lists / entry.values. The
				// block branch releases this lock before recursing
				// via serializeInlineListEntry (which re-locks).
				entry.mu.RLock()
				useInline := len(entry.containers) == 0 && len(entry.lists) == 0
				hasValues := useInline && len(entry.values) > 0

				if hasValues {
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
					b.Str("\n")
				} else {
					entry.mu.RUnlock()
					b.Str(prefix)
					b.Str(name)
					b.Str(" ")
					b.Str(quoteIfNeeded(displayKey))
					b.Str(" {\n")
					serializeInlineListEntry(b, entry, n, indent+1)
					b.Str(prefix)
					b.Str("}\n")
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

// serializeListMultiBlock serializes list entries as a grouped block with positional inline entries.
// Output: name { key1; key2 val1; key3 val1 val2; }.
func serializeListMultiBlock(b *textbuf.Buffer, name string, entries map[string]*Tree, node *ListNode, order []string, indent int) {
	prefix := strings.Repeat("\t", indent)
	innerPrefix := strings.Repeat("\t", indent+1)

	b.Str(prefix)
	b.Str(name)
	b.Str(" {\n")

	// Use insertion order if available, otherwise sort
	keys := order
	if len(keys) == 0 {
		keys = make([]string, 0, len(entries))
		for k := range entries {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}

	for _, key := range keys {
		entry := entries[key]
		if entry == nil {
			continue
		}
		displayKey := StripListKeySuffix(key)
		b.Str(innerPrefix)
		b.Str(quoteIfNeeded(displayKey))

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
	b.Str("}\n")
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
	sort.Strings(keys)

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

	// Check for simple value or flag
	if v, ok := tree.values[name]; ok {
		b.Str(prefix)
		b.Str(name)
		if v != configTrue {
			b.Str(" ")
			b.Str(quoteIfNeeded(v))
		}
		b.Str("\n")
	} else if mv := tree.multiValues[name]; len(mv) > 0 {
		for _, v := range mv {
			b.Str(prefix)
			b.Str(name)
			b.Str(" ")
			b.Str(v)
			b.Str("\n")
		}
	}

	// Block form (container children)
	if child := tree.containers[name]; child != nil {
		b.Str(prefix)
		if child.IsInactive() {
			b.Str("inactive: ")
		}
		b.Str(name)
		b.Str(" {\n")
		serializeTree(b, child, node, indent+1)
		b.Str(prefix)
		b.Str("}\n")
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

// quoteIfNeeded quotes a string if it contains spaces or special characters.
func quoteIfNeeded(s string) string {
	if s == "" {
		return `""`
	}

	needsQuote := false
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '"' || c == '\'' || c == '{' || c == '}' || c == ';' || c == '#' {
			needsQuote = true
			break
		}
	}

	if !needsQuote {
		return s
	}

	// Escape quotes and backslashes
	var b textbuf.Buffer
	b.Byte('"')
	for _, c := range s {
		switch c {
		case '"':
			b.Str(`\"`)
		case '\\':
			b.Str(`\\`)
		case '\n':
			b.Str(`\n`)
		case '\t':
			b.Str(`\t`)
		default:
			b.WriteRune(c)
		}
	}
	b.Byte('"')
	return b.String()
}
