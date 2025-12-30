package config

import (
	"sort"
	"strconv"
	"strings"
)

// stripListKeySuffix removes the #N suffix added by AddListEntry for duplicate keys.
// For example, "10.0.0.10#1" becomes "10.0.0.10".
func stripListKeySuffix(key string) string {
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

// Serialize converts a Tree back to ExaBGP config format.
func Serialize(tree *Tree, schema *Schema) string {
	var b strings.Builder
	serializeTree(&b, tree, schema.root, 0)
	return b.String()
}

func serializeTree(b *strings.Builder, tree *Tree, node *ContainerNode, indent int) {
	prefix := strings.Repeat("\t", indent)

	// Serialize in schema order where possible
	for _, name := range node.Children() {
		child := node.Get(name)
		serializeNode(b, tree, name, child, indent)
	}

	// Also serialize any values/containers/lists not in schema (shouldn't happen, but be safe)
	// Values
	schemaNames := make(map[string]bool)
	for _, name := range node.Children() {
		schemaNames[name] = true
	}

	// Sort for deterministic output
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
		b.WriteString(";\n")
	}
}

func serializeNode(b *strings.Builder, tree *Tree, name string, node Node, indent int) {
	prefix := strings.Repeat("\t", indent)

	switch n := node.(type) {
	case *LeafNode:
		if v, ok := tree.values[name]; ok {
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(quoteIfNeeded(normalizeBool(v)))
			b.WriteString(";\n")
		}

	case *MultiLeafNode:
		if v, ok := tree.values[name]; ok {
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(v) // Already space-separated
			b.WriteString(";\n")
		}

	case *ArrayLeafNode:
		if v, ok := tree.values[name]; ok {
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" [ ")
			b.WriteString(v) // Space-separated items
			b.WriteString(" ];\n")
		}

	case *ContainerNode:
		if child := tree.containers[name]; child != nil {
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" {\n")
			serializeTree(b, child, n, indent+1)
			b.WriteString(prefix)
			b.WriteString("}\n")
		}

	case *ListNode:
		if entries := tree.lists[name]; entries != nil {
			// Sort keys for deterministic output
			var keys []string
			for k := range entries {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, key := range keys {
				entry := entries[key]
				b.WriteString(prefix)
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

	case *FreeformNode:
		if child := tree.containers[name]; child != nil {
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" {\n")
			serializeFreeform(b, child, indent+1)
			b.WriteString(prefix)
			b.WriteString("}\n")
		}

	case *FamilyBlockNode:
		// FamilyBlockNode stores families as "AFI SAFI" -> "mode"
		// Serialize using inline syntax for simplicity
		// Add blank line before family for readability
		if child := tree.containers[name]; child != nil {
			b.WriteString("\n")
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" {\n")
			serializeFamilyBlock(b, child, indent+1)
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
			b.WriteString(";\n")
		} else if mv := tree.multiValues[name]; len(mv) > 0 {
			// Inline values (e.g., vpls rd X endpoint Y ...;)
			for _, v := range mv {
				b.WriteString(prefix)
				b.WriteString(name)
				b.WriteString(" ")
				b.WriteString(v)
				b.WriteString(";\n")
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
			var keys []string
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
			var keys []string
			for k := range entries {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, key := range keys {
				entry := entries[key]
				// Strip #N suffix from duplicate keys for serialization
				displayKey := stripListKeySuffix(key)

				// Decide: inline or block?
				// Use inline if all values are simple (no nested containers)
				useInline := len(entry.containers) == 0 && len(entry.lists) == 0

				if useInline && len(entry.values) > 0 {
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
					b.WriteString(";\n")
				} else {
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

func serializeListEntry(b *strings.Builder, tree *Tree, node *ListNode, indent int) {
	prefix := strings.Repeat("\t", indent)

	// Serialize in schema order
	for _, name := range node.Children() {
		child := node.Get(name)
		serializeNode(b, tree, name, child, indent)
	}

	// Values not in schema
	schemaNames := make(map[string]bool)
	for _, name := range node.Children() {
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
		b.WriteString(";\n")
	}
}

func serializeFreeform(b *strings.Builder, tree *Tree, indent int) {
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
			// Has a value - always output as "key [ value ];" to preserve roundtrip
			b.WriteString(" [ ")
			b.WriteString(v)
			b.WriteString(" ]")
		}
		b.WriteString(";\n")
	}
}

// serializeFamilyBlock serializes family entries in inline syntax.
// Output format: "AFI SAFI;" for enable, "AFI SAFI mode;" for other modes.
func serializeFamilyBlock(b *strings.Builder, tree *Tree, indent int) {
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
		// Only output mode if not "true" (default enable)
		if v != configTrue && v != configEnable {
			b.WriteString(" ")
			b.WriteString(v)
		}
		b.WriteString(";\n")
	}
}

func serializeFlexContainer(b *strings.Builder, tree *Tree, node *FlexNode, indent int) {
	prefix := strings.Repeat("\t", indent)

	// Serialize in schema order
	for _, name := range node.Children() {
		child := node.Get(name)
		serializeNode(b, tree, name, child, indent)
	}

	// Values not in schema
	schemaNames := make(map[string]bool)
	for _, name := range node.Children() {
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
		b.WriteString(";\n")
	}
}

func serializeInlineListEntry(b *strings.Builder, tree *Tree, node *InlineListNode, indent int) {
	prefix := strings.Repeat("\t", indent)

	// Serialize in schema order
	for _, name := range node.Children() {
		child := node.Get(name)
		serializeNode(b, tree, name, child, indent)
	}

	// Values not in schema
	schemaNames := make(map[string]bool)
	for _, name := range node.Children() {
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
		b.WriteString(";\n")
	}
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
