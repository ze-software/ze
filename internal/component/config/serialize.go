// Design: docs/architecture/config/syntax.md — config parsing and loading
// Related: tree.go — Tree data structure
// Related: serialize_annotated.go — column-aware annotated serialization
// Related: prune.go — inactive node pruning (uses isInactiveTree)

package config

import (
	"sort"
	"strconv"
	"strings"
)

// maxInlineDepth controls the maximum container inlining depth in serialized output.
// When set to 1, a container with exactly one leaf child is serialized inline
// (e.g., "local ip 1.2.3.4" instead of "local {\n\tip 1.2.3.4\n}").
// Only leaf children (values/multiValues) trigger inlining -- container and list
// children do not, which naturally prevents cascading beyond one level.
const maxInlineDepth = 1

// canInlineContainer reports whether a container's tree data has exactly one
// leaf-like child (value or multiValue), and no containers or lists.
// The inactive leaf is excluded from the count.
func canInlineContainer(tree *Tree) bool {
	if maxInlineDepth < 1 {
		return false
	}
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	valueCount := len(tree.values)
	if _, ok := tree.values[InactiveLeafName]; ok {
		valueCount--
	}
	return (valueCount+len(tree.multiValues)) == 1 &&
		len(tree.containers) == 0 && len(tree.lists) == 0
}

// serializeContainerInline writes a container with a single leaf child inline:
// "containerName childName value\n" without braces.
func serializeContainerInline(b *strings.Builder, child *Tree, name string, node *ContainerNode, indent int) {
	child.mu.RLock()
	defer child.mu.RUnlock()

	prefix := strings.Repeat("\t", indent)
	b.WriteString(prefix)
	// Inline inactive-check: child.mu is already RLocked, cannot call
	// isInactiveTree (which would re-enter Get).
	if v, ok := child.values[InactiveLeafName]; ok && v == configTrue {
		b.WriteString("inactive: ")
	}
	b.WriteString(name)

	// Find the single child in schema order and write it inline.
	for _, childName := range node.Children() {
		if childName == InactiveLeafName {
			continue
		}
		childNode := node.Get(childName)
		if writeInlineLeaf(b, child, childName, childNode) {
			b.WriteString("\n")
			return
		}
	}

	// Fallback: extra values not in schema.
	for k, v := range child.values {
		if k == InactiveLeafName {
			continue
		}
		b.WriteString(" ")
		b.WriteString(k)
		b.WriteString(" ")
		b.WriteString(quoteIfNeeded(v))
		b.WriteString("\n")
		return
	}

	b.WriteString("\n")
}

// writeInlineLeaf writes a leaf value inline (without prefix or newline).
// Returns true if the child had data and was written.
//
// Caller MUST hold tree.mu.RLock() -- this helper reads tree.values and
// tree.multiValues directly rather than going through Get/GetSlice (which
// would attempt to re-acquire the lock).
func writeInlineLeaf(b *strings.Builder, tree *Tree, name string, node Node) bool {
	switch node.(type) {
	case *LeafNode:
		if v, ok := tree.values[name]; ok {
			b.WriteString(" ")
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(quoteIfNeeded(normalizeBool(v)))
			return true
		}
	case *MultiLeafNode:
		if v, ok := tree.values[name]; ok {
			b.WriteString(" ")
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(v)
			return true
		}
	case *BracketLeafListNode:
		if v, ok := tree.values[name]; ok {
			b.WriteString(" ")
			b.WriteString(name)
			b.WriteString(" [ ")
			b.WriteString(v)
			b.WriteString(" ]")
			return true
		}
	case *ValueOrArrayNode:
		if items := tree.multiValues[name]; len(items) > 0 {
			b.WriteString(" ")
			b.WriteString(name)
			if len(items) == 1 {
				b.WriteString(" ")
				b.WriteString(quoteIfNeeded(items[0]))
			} else {
				b.WriteString(" [ ")
				for i, item := range items {
					if i > 0 {
						b.WriteString(" ")
					}
					b.WriteString(quoteIfNeeded(item))
				}
				b.WriteString(" ]")
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

// isInactiveTree checks if a tree node has the inactive leaf set to true.
func isInactiveTree(tree *Tree) bool {
	v, ok := tree.Get(InactiveLeafName)
	return ok && v == configTrue
}

// Serialize converts a Tree back to config text format.
func Serialize(tree *Tree, schema *Schema) string {
	var b strings.Builder
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
	var b strings.Builder
	serializeWithChildren(&b, tree, cp, 0)
	return b.String()
}

// serializeExtraValues writes tree values that are not in the schema's children list.
// This handles unknown/extra keys that appear in the config but aren't defined in schema.
func serializeExtraValues(b *strings.Builder, tree *Tree, children []string, indent int) {
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
		b.WriteString(prefix)
		b.WriteString(k)
		b.WriteString(" ")
		b.WriteString(quoteIfNeeded(tree.values[k]))
		b.WriteString("\n")
	}
}

// serializeWithChildren serializes tree content using a schema node that provides
// Children() and Get() for ordering.
//
// Holds tree.mu.RLock for the duration so callees can read tree.values /
// tree.containers / tree.lists directly. Recursion into child trees acquires
// the child's own lock independently.
func serializeWithChildren(b *strings.Builder, tree *Tree, node childProvider, indent int) {
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
func serializeTree(b *strings.Builder, tree *Tree, node *ContainerNode, indent int) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	for _, name := range node.Children() {
		child := node.Get(name)
		serializeNode(b, tree, name, child, indent)
	}

	serializeExtraValues(b, tree, node.Children(), indent)
}

func serializeNode(b *strings.Builder, tree *Tree, name string, node Node, indent int) {
	prefix := strings.Repeat("\t", indent)

	switch n := node.(type) {
	case *LeafNode:
		if n.Hidden || n.Ephemeral {
			break
		}
		if name == InactiveLeafName {
			break // Rendered as "inactive: " prefix on the parent, not as a leaf
		}
		if v, ok := tree.values[name]; ok {
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(quoteIfNeeded(normalizeBool(v)))
			b.WriteString("\n")
		}

	case *MultiLeafNode:
		if v, ok := tree.values[name]; ok {
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(v) // Already space-separated
			b.WriteString("\n")
		}

	case *BracketLeafListNode:
		if v, ok := tree.values[name]; ok {
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" [ ")
			b.WriteString(v) // Space-separated items
			b.WriteString(" ]\n")
		}

	case *ValueOrArrayNode:
		// Direct access: caller holds tree.mu.RLock, calling GetSlice
		// would recursively RLock the same mutex (unsafe per Go docs).
		if items := tree.multiValues[name]; len(items) > 0 {
			b.WriteString(prefix)
			b.WriteString(name)
			if len(items) == 1 {
				b.WriteString(" ")
				b.WriteString(quoteIfNeeded(items[0]))
				b.WriteString("\n")
			} else {
				b.WriteString(" [ ")
				for i, item := range items {
					if i > 0 {
						b.WriteString(" ")
					}
					b.WriteString(quoteIfNeeded(item))
				}
				b.WriteString(" ]\n")
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
				b.WriteString(prefix)
				if isInactiveTree(child) {
					b.WriteString("inactive: ")
				}
				b.WriteString(name)
				b.WriteString(" {\n")
				serializeTree(b, child, n, indent+1)
				b.WriteString(prefix)
				b.WriteString("}\n")
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
					b.WriteString(prefix)
					if isInactiveTree(entry) {
						b.WriteString("inactive: ")
					}
					b.WriteString(name)
					// Skip outputting KeyDefault - it's the implicit default
					if key != KeyDefault {
						b.WriteString(" ")
						b.WriteString(quoteIfNeeded(key))
					}
					b.WriteString(" {\n")
					serializeListEntry(b, entry, n, indent+1)
					b.WriteString(prefix)
					b.WriteString("}\n")
				}
			}
		}

	case *FreeformNode:
		if child := tree.containers[name]; child != nil {
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" {\n")
			serializeFreeform(b, child, indent+1)
			b.WriteString(prefix)
			b.WriteString("}\n")
		}

	case *FlexNode:
		// Check if it's a simple value, multiValue, container, or list
		if v, ok := tree.values[name]; ok {
			b.WriteString(prefix)
			b.WriteString(name)
			if v != configTrue {
				b.WriteString(" ")
				b.WriteString(quoteIfNeeded(v))
			}
			b.WriteString("\n")
		} else if mv := tree.multiValues[name]; len(mv) > 0 {
			// Inline values (e.g., vpls rd X endpoint Y ...;)
			for _, v := range mv {
				b.WriteString(prefix)
				b.WriteString(name)
				b.WriteString(" ")
				b.WriteString(v)
				b.WriteString("\n")
			}
		}
		// Also serialize container form
		if child := tree.containers[name]; child != nil {
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" {\n")
			serializeFlexContainer(b, child, n, indent+1)
			b.WriteString(prefix)
			b.WriteString("}\n")
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
				b.WriteString(prefix)
				b.WriteString(name)
				b.WriteString(" ")
				b.WriteString(quoteIfNeeded(key))
				b.WriteString(" {\n")
				serializeFlexContainer(b, entry, n, indent+1)
				b.WriteString(prefix)
				b.WriteString("}\n")
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
					b.WriteString(prefix)
					b.WriteString(name)
					b.WriteString(" ")
					b.WriteString(quoteIfNeeded(displayKey))
					for _, attrName := range n.Children() {
						if v, ok := entry.values[attrName]; ok {
							b.WriteString(" ")
							b.WriteString(attrName)
							b.WriteString(" ")
							b.WriteString(quoteIfNeeded(v))
						}
					}
					// Also add any values not in schema order
					for k, v := range entry.values {
						if !n.Has(k) {
							b.WriteString(" ")
							b.WriteString(k)
							b.WriteString(" ")
							b.WriteString(quoteIfNeeded(v))
						}
					}
					entry.mu.RUnlock()
					b.WriteString("\n")
				} else {
					entry.mu.RUnlock()
					b.WriteString(prefix)
					b.WriteString(name)
					b.WriteString(" ")
					b.WriteString(quoteIfNeeded(displayKey))
					b.WriteString(" {\n")
					serializeInlineListEntry(b, entry, n, indent+1)
					b.WriteString(prefix)
					b.WriteString("}\n")
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
func serializeListMultiBlock(b *strings.Builder, name string, entries map[string]*Tree, node *ListNode, order []string, indent int) {
	prefix := strings.Repeat("\t", indent)
	innerPrefix := strings.Repeat("\t", indent+1)

	b.WriteString(prefix)
	b.WriteString(name)
	b.WriteString(" {\n")

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
		b.WriteString(innerPrefix)
		b.WriteString(quoteIfNeeded(displayKey))

		// Positional children in definition order
		for _, childName := range node.Children() {
			if v, ok := entry.Get(childName); ok {
				b.WriteString(" ")
				b.WriteString(quoteIfNeeded(v))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString(prefix)
	b.WriteString("}\n")
}

func serializeListEntry(b *strings.Builder, tree *Tree, node *ListNode, indent int) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	for _, name := range node.Children() {
		child := node.Get(name)
		serializeNode(b, tree, name, child, indent)
	}

	serializeExtraValues(b, tree, node.Children(), indent)
}

func serializeFreeform(b *strings.Builder, tree *Tree, indent int) {
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
		b.WriteString(prefix)
		b.WriteString(k)
		if v != configTrue {
			if strings.HasPrefix(v, "[ ") && strings.HasSuffix(v, " ]") {
				// Already bracketed — output as-is
				b.WriteString(" ")
				b.WriteString(v)
			} else {
				// Wrap in brackets to preserve roundtrip
				b.WriteString(" [ ")
				b.WriteString(v)
				b.WriteString(" ]")
			}
		}
		b.WriteString("\n")
	}
}

// serializePresenceContainer serializes a presence container in flag, value, or block form.
// Mirrors FlexNode serialization: checks values, multiValues, containers, and lists.
func serializePresenceContainer(b *strings.Builder, tree *Tree, name string, node *ContainerNode, indent int) {
	prefix := strings.Repeat("\t", indent)

	// Check for simple value or flag
	if v, ok := tree.values[name]; ok {
		b.WriteString(prefix)
		b.WriteString(name)
		if v != configTrue {
			b.WriteString(" ")
			b.WriteString(quoteIfNeeded(v))
		}
		b.WriteString("\n")
	} else if mv := tree.multiValues[name]; len(mv) > 0 {
		for _, v := range mv {
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(v)
			b.WriteString("\n")
		}
	}

	// Block form (container children)
	if child := tree.containers[name]; child != nil {
		b.WriteString(prefix)
		if isInactiveTree(child) {
			b.WriteString("inactive: ")
		}
		b.WriteString(name)
		b.WriteString(" {\n")
		serializeTree(b, child, node, indent+1)
		b.WriteString(prefix)
		b.WriteString("}\n")
	}
}

func serializeFlexContainer(b *strings.Builder, tree *Tree, node *FlexNode, indent int) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	for _, name := range node.Children() {
		child := node.Get(name)
		serializeNode(b, tree, name, child, indent)
	}

	serializeExtraValues(b, tree, node.Children(), indent)
}

func serializeInlineListEntry(b *strings.Builder, tree *Tree, node *InlineListNode, indent int) {
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
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range s {
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}
