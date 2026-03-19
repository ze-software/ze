// Design: docs/architecture/config/syntax.md — set-format serialization
// Detail: serialize_blame.go — blame view serialization
// Related: serialize.go — hierarchical text serialization
// Related: setparser.go — set-format parsing (inverse of this file)
// Related: meta.go — MetaTree for metadata-aware serialization

package config

import (
	"bufio"
	"sort"
	"strings"
	"time"
)

// ConfigFormat identifies the format of a configuration file.
type ConfigFormat int

const (
	// FormatHierarchical is the traditional hierarchical text format.
	FormatHierarchical ConfigFormat = iota
	// FormatSet is the flat set-command format without metadata.
	FormatSet
	// FormatSetMeta is the flat set-command format with metadata prefixes.
	FormatSetMeta
)

// DetectFormat examines the first non-empty, non-comment line to determine the config format.
//
// Rules:
//   - Any line with "#identifier" (non-space after #), @, %, ^ prefix -> FormatSetMeta
//   - Lines starting with "set " or "delete " -> FormatSet (if no metadata found)
//   - Anything else -> FormatHierarchical
//   - Empty or comments-only -> FormatSet (new files default to set format)
//
// Scans ALL lines because metadata annotations can appear after plain set lines
// (e.g., only some leaves have user/time metadata). Early return on "set" would
// misidentify mixed content as FormatSet, causing metadata lines to be skipped
// as comments and losing data.
func DetectFormat(content string) ConfigFormat {
	scanner := bufio.NewScanner(strings.NewReader(content))
	hasSet := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// "# text" (hash + space) is a comment -- skip it
		if strings.HasPrefix(line, "# ") {
			continue
		}

		// "#identifier" (hash + non-space) is metadata prefix
		if len(line) > 1 && line[0] == '#' && line[1] != ' ' {
			return FormatSetMeta
		}

		// @source, %timestamp, ^previous are also metadata prefixes
		if line[0] == '@' || line[0] == '%' || line[0] == '^' {
			return FormatSetMeta
		}

		// Record set/delete but keep scanning for metadata.
		if strings.HasPrefix(line, "set ") || strings.HasPrefix(line, "delete ") {
			hasSet = true
			continue
		}

		// Anything else is hierarchical
		return FormatHierarchical
	}

	if hasSet {
		return FormatSet
	}

	// Empty or comments-only defaults to set format (new files should not trigger migration)
	return FormatSet
}

// SerializeSet converts a Tree to flat set commands in YANG schema order.
// Each leaf value becomes one "set <path> <value>" line.
func SerializeSet(tree *Tree, schema *Schema) string {
	var b strings.Builder
	serializeSetNode(&b, tree, schema.root, "")
	return b.String()
}

// serializeSetNode walks the schema children in order, emitting set commands.
// prefix accumulates the path segments (e.g., "neighbor 192.0.2.1 ").
func serializeSetNode(b *strings.Builder, tree *Tree, parent childProvider, prefix string) {
	for _, name := range parent.Children() {
		child := parent.Get(name)
		serializeSetChild(b, tree, name, child, prefix)
	}

	// Extra values not in schema
	serializeSetExtraValues(b, tree, parent.Children(), prefix)
}

// serializeSetChild dispatches serialization based on node type.
//
//nolint:cyclop // exhaustive switch over all node types is intentional
func serializeSetChild(b *strings.Builder, tree *Tree, name string, node Node, prefix string) {
	switch n := node.(type) {
	case *LeafNode:
		if v, ok := tree.values[name]; ok {
			b.WriteString("set ")
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(quoteIfNeeded(normalizeBool(v)))
			b.WriteString("\n")
		}

	case *MultiLeafNode:
		if v, ok := tree.values[name]; ok {
			b.WriteString("set ")
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(v)
			b.WriteString("\n")
		}

	case *BracketLeafListNode:
		if v, ok := tree.values[name]; ok {
			b.WriteString("set ")
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" [ ")
			b.WriteString(v)
			b.WriteString(" ]\n")
		}

	case *ValueOrArrayNode:
		if items := tree.GetSlice(name); len(items) > 0 {
			b.WriteString("set ")
			b.WriteString(prefix)
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
			b.WriteString("\n")
		}

	case *ContainerNode:
		serializeSetContainer(b, tree, name, n, prefix)

	case *ListNode:
		serializeSetList(b, tree, name, n, prefix)

	case *FreeformNode:
		serializeSetFreeform(b, tree, name, prefix)

	case *FlexNode:
		serializeSetFlex(b, tree, name, n, prefix)

	case *InlineListNode:
		serializeSetInlineList(b, tree, name, n, prefix)
	}
}

// serializeSetContainer handles container nodes, including presence containers.
func serializeSetContainer(b *strings.Builder, tree *Tree, name string, node *ContainerNode, prefix string) {
	if node.Presence {
		// Presence container as flag or value
		if v, ok := tree.values[name]; ok {
			b.WriteString("set ")
			b.WriteString(prefix)
			b.WriteString(name)
			if v != configTrue {
				b.WriteString(" ")
				b.WriteString(quoteIfNeeded(v))
			}
			b.WriteString("\n")
		}
		// Presence container with children
		if child := tree.containers[name]; child != nil {
			childPrefix := prefix + name + " "
			serializeSetNode(b, child, node, childPrefix)
		}
		return
	}

	// Regular container
	if child := tree.containers[name]; child != nil {
		childPrefix := prefix + name + " "
		serializeSetNode(b, child, node, childPrefix)
	}
}

// serializeSetList handles list nodes with keyed entries.
func serializeSetList(b *strings.Builder, tree *Tree, name string, node *ListNode, prefix string) {
	entries := tree.lists[name]
	if entries == nil {
		return
	}

	// Use insertion order if available, otherwise sort
	keys := tree.listOrder[name]
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
		entryPrefix := prefix + name + " " + quoteIfNeeded(displayKey) + " "
		serializeSetNode(b, entry, node, entryPrefix)
	}
}

// serializeSetFreeform handles freeform nodes (set of key-value pairs).
func serializeSetFreeform(b *strings.Builder, tree *Tree, name, prefix string) {
	child := tree.containers[name]
	if child == nil {
		return
	}

	keys := make([]string, 0, len(child.values))
	for k := range child.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := child.values[k]
		b.WriteString("set ")
		b.WriteString(prefix)
		b.WriteString(name)
		b.WriteString(" ")
		b.WriteString(k)
		if v != configTrue {
			b.WriteString(" ")
			b.WriteString(quoteIfNeeded(v))
		}
		b.WriteString("\n")
	}
}

// serializeSetFlex handles flex nodes (flag, value, container, or list forms).
func serializeSetFlex(b *strings.Builder, tree *Tree, name string, node *FlexNode, prefix string) {
	// Simple value or flag
	if v, ok := tree.values[name]; ok {
		b.WriteString("set ")
		b.WriteString(prefix)
		b.WriteString(name)
		if v != configTrue {
			b.WriteString(" ")
			b.WriteString(quoteIfNeeded(v))
		}
		b.WriteString("\n")
	}

	// Multi-values
	if mv := tree.multiValues[name]; len(mv) > 0 {
		for _, v := range mv {
			b.WriteString("set ")
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(v)
			b.WriteString("\n")
		}
	}

	// Container form
	if child := tree.containers[name]; child != nil {
		childPrefix := prefix + name + " "
		serializeSetNode(b, child, node, childPrefix)
	}

	// List entries
	if entries := tree.lists[name]; entries != nil {
		keys := make([]string, 0, len(entries))
		for k := range entries {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			entry := entries[key]
			entryPrefix := prefix + name + " " + quoteIfNeeded(key) + " "
			serializeSetNode(b, entry, node, entryPrefix)
		}
	}
}

// serializeSetInlineList handles inline list entries.
func serializeSetInlineList(b *strings.Builder, tree *Tree, name string, node *InlineListNode, prefix string) {
	entries := tree.lists[name]
	if entries == nil {
		return
	}

	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		entry := entries[key]
		displayKey := StripListKeySuffix(key)
		entryPrefix := prefix + name + " " + quoteIfNeeded(displayKey) + " "

		// Emit each child value
		for _, childName := range node.Children() {
			v, ok := entry.values[childName]
			if !ok {
				continue
			}
			b.WriteString("set ")
			b.WriteString(entryPrefix)
			b.WriteString(childName)
			b.WriteString(" ")
			b.WriteString(quoteIfNeeded(v))
			b.WriteString("\n")
		}
	}
}

// serializeSetExtraValues writes tree values not in the schema.
func serializeSetExtraValues(b *strings.Builder, tree *Tree, children []string, prefix string) {
	schemaNames := make(map[string]bool, len(children))
	for _, name := range children {
		schemaNames[name] = true
	}

	var extraKeys []string
	for k := range tree.values {
		if !schemaNames[k] {
			extraKeys = append(extraKeys, k)
		}
	}
	sort.Strings(extraKeys)

	for _, k := range extraKeys {
		b.WriteString("set ")
		b.WriteString(prefix)
		b.WriteString(k)
		b.WriteString(" ")
		b.WriteString(quoteIfNeeded(tree.values[k]))
		b.WriteString("\n")
	}
}

// --- Metadata-aware serialization ---

// SerializeSetWithMeta converts a Tree to flat set commands with metadata prefixes.
// Each leaf value becomes one line: [#user @time %session] set <path> <value>.
// Lines without metadata in the MetaTree are emitted as bare set commands.
func SerializeSetWithMeta(tree *Tree, meta *MetaTree, schema *Schema) string {
	var b strings.Builder
	serializeSetMetaNode(&b, tree, meta, schema.root, "")
	return b.String()
}

// writeMetaPrefix writes the metadata prefix for a leaf entry.
// Format: #user @source %ISO8601 ^previous (each present only if non-empty).
func writeMetaPrefix(b *strings.Builder, e MetaEntry) {
	if e.User != "" {
		b.WriteString("#")
		b.WriteString(e.User)
		b.WriteString(" ")
	}
	if e.Source != "" {
		b.WriteString("@")
		b.WriteString(e.Source)
		b.WriteString(" ")
	}
	if !e.Time.IsZero() {
		b.WriteString("%")
		b.WriteString(e.Time.UTC().Format(time.RFC3339))
		b.WriteString(" ")
	}
	if e.Previous != "" {
		b.WriteString("^")
		if strings.ContainsAny(e.Previous, " \"\\") {
			// Quote and escape backslashes then double quotes.
			// Order matters: escape \ first to avoid double-escaping \".
			escaped := strings.ReplaceAll(e.Previous, "\\", "\\\\")
			escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
			b.WriteString("\"")
			b.WriteString(escaped)
			b.WriteString("\"")
		} else {
			b.WriteString(e.Previous)
		}
		b.WriteString(" ")
	}
}

// metaContainerChild returns the child MetaTree for a container, or nil.
func metaContainerChild(meta *MetaTree, name string) *MetaTree {
	if meta == nil {
		return nil
	}
	return meta.containers[name]
}

// metaListEntry returns the MetaTree for a list entry key, or nil.
// List navigation: meta.containers[listName] -> .lists[key].
func metaListEntry(meta *MetaTree, listName, key string) *MetaTree {
	if meta == nil {
		return nil
	}
	listMeta := meta.containers[listName]
	if listMeta == nil {
		return nil
	}
	return listMeta.lists[key]
}

// serializeSetMetaNode walks schema children, emitting set commands with metadata.
func serializeSetMetaNode(b *strings.Builder, tree *Tree, meta *MetaTree, parent childProvider, prefix string) {
	for _, name := range parent.Children() {
		child := parent.Get(name)
		serializeSetMetaChild(b, tree, meta, name, child, prefix)
	}

	serializeSetMetaExtraValues(b, tree, meta, parent.Children(), prefix)
	writeDeleteMetaLines(b, tree, meta, prefix)
}

// serializeSetMetaChild dispatches metadata-aware serialization by node type.
//
//nolint:cyclop // exhaustive switch over all node types is intentional
func serializeSetMetaChild(b *strings.Builder, tree *Tree, meta *MetaTree, name string, node Node, prefix string) {
	switch n := node.(type) {
	case *LeafNode:
		if v, ok := tree.values[name]; ok {
			writeMetaLeafLine(b, meta, name, prefix+name+" ", quoteIfNeeded(normalizeBool(v)))
		}

	case *MultiLeafNode:
		if v, ok := tree.values[name]; ok {
			writeMetaLeafLine(b, meta, name, prefix+name+" ", v)
		}

	case *BracketLeafListNode:
		if v, ok := tree.values[name]; ok {
			writeMetaLeafLine(b, meta, name, prefix+name+" ", "[ "+v+" ]")
		}

	case *ValueOrArrayNode:
		if items := tree.GetSlice(name); len(items) > 0 {
			pathPfx := prefix + name + " "
			if len(items) == 1 {
				writeMetaLeafLine(b, meta, name, pathPfx, quoteIfNeeded(items[0]))
			} else {
				parts := make([]string, len(items))
				for i, item := range items {
					parts[i] = quoteIfNeeded(item)
				}
				writeMetaLeafLine(b, meta, name, pathPfx, "[ "+strings.Join(parts, " ")+" ]")
			}
		}

	case *ContainerNode:
		serializeSetMetaContainer(b, tree, meta, name, n, prefix)

	case *ListNode:
		serializeSetMetaList(b, tree, meta, name, n, prefix)

	case *FreeformNode:
		serializeSetMetaFreeform(b, tree, meta, name, prefix)

	case *FlexNode:
		serializeSetMetaFlex(b, tree, meta, name, n, prefix)

	case *InlineListNode:
		serializeSetMetaInlineList(b, tree, meta, name, n, prefix)
	}
}

// writeMetaLeafLine writes set line(s) with optional metadata prefix.
// pathPrefix is everything before the value (including trailing space).
// value is the formatted value (empty for flag-style entries).
// For contested leaves (multiple session entries), it emits one line per entry,
// substituting the entry's Value for the tree value.
func writeMetaLeafLine(b *strings.Builder, meta *MetaTree, name, pathPrefix, value string) {
	if meta != nil {
		entries := meta.entries[name]
		if len(entries) > 1 {
			// Contested leaf: emit one line per session entry with its own value.
			for _, e := range entries {
				writeMetaPrefix(b, e)
				if e.Value == "" && e.Source != "" {
					// Active session deleted the value (committed entries have no Source).
					b.WriteString("delete ")
					b.WriteString(strings.TrimRight(pathPrefix, " "))
					b.WriteString("\n")
					continue
				}
				b.WriteString("set ")
				b.WriteString(pathPrefix)
				if e.Value != "" {
					if !strings.HasSuffix(pathPrefix, " ") {
						b.WriteString(" ")
					}
					b.WriteString(quoteIfNeeded(e.Value))
				} else {
					// Sessionless (committed) entry: Value is always "" because
					// committed metadata doesn't store the value separately --
					// the tree value IS the committed value. Use the tree value.
					b.WriteString(value)
				}
				b.WriteString("\n")
			}
			return
		}
		if len(entries) == 1 {
			writeMetaPrefix(b, entries[0])
		}
	}
	b.WriteString("set ")
	b.WriteString(pathPrefix)
	b.WriteString(value)
	b.WriteString("\n")
}

// serializeSetMetaContainer handles container nodes with metadata.
func serializeSetMetaContainer(b *strings.Builder, tree *Tree, meta *MetaTree, name string, node *ContainerNode, prefix string) {
	if node.Presence {
		if v, ok := tree.values[name]; ok {
			if v != configTrue {
				writeMetaLeafLine(b, meta, name, prefix+name+" ", quoteIfNeeded(v))
			} else {
				writeMetaLeafLine(b, meta, name, prefix+name, "")
			}
		}
		if child := tree.containers[name]; child != nil {
			childPrefix := prefix + name + " "
			serializeSetMetaNode(b, child, metaContainerChild(meta, name), node, childPrefix)
		}
		return
	}

	if child := tree.containers[name]; child != nil {
		childPrefix := prefix + name + " "
		serializeSetMetaNode(b, child, metaContainerChild(meta, name), node, childPrefix)
	}
}

// serializeSetMetaList handles list nodes with metadata.
func serializeSetMetaList(b *strings.Builder, tree *Tree, meta *MetaTree, name string, node *ListNode, prefix string) {
	entries := tree.lists[name]
	if entries == nil {
		return
	}

	keys := tree.listOrder[name]
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
		entryPrefix := prefix + name + " " + quoteIfNeeded(displayKey) + " "
		entryMeta := metaListEntry(meta, name, key)
		serializeSetMetaNode(b, entry, entryMeta, node, entryPrefix)
	}
}

// leafLineWriter is a function that writes a single leaf line with metadata.
// pathPrefix is the path up to and including the trailing space before the value.
// value is the formatted value (may be empty for flag-style entries).
// writeMetaLeafLine satisfies this signature for metadata-aware set serialization.
type leafLineWriter func(b *strings.Builder, meta *MetaTree, name, pathPrefix, value string)

// serializeSetMetaFreeform handles freeform nodes with metadata.
func serializeSetMetaFreeform(b *strings.Builder, tree *Tree, meta *MetaTree, name, prefix string) {
	writeFreeformLines(b, tree, meta, name, prefix, writeMetaLeafLine)
}

// serializeSetMetaFlex handles flex nodes with metadata.
func serializeSetMetaFlex(b *strings.Builder, tree *Tree, meta *MetaTree, name string, node *FlexNode, prefix string) {
	writeFlexLines(b, tree, meta, name, node, prefix, writeMetaLeafLine)
}

// serializeSetMetaInlineList handles inline list entries with metadata.
func serializeSetMetaInlineList(b *strings.Builder, tree *Tree, meta *MetaTree, name string, node *InlineListNode, prefix string) {
	writeInlineListLines(b, tree, meta, name, node, prefix, writeMetaLeafLine)
}

// writeFreeformLines is the shared implementation for freeform serialization.
func writeFreeformLines(b *strings.Builder, tree *Tree, meta *MetaTree, name, prefix string, writeLine leafLineWriter) {
	child := tree.containers[name]
	if child == nil {
		return
	}

	childMeta := metaContainerChild(meta, name)

	keys := make([]string, 0, len(child.values))
	for k := range child.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := child.values[k]
		keyPrefix := prefix + name + " " + k
		if v != configTrue {
			writeLine(b, childMeta, k, keyPrefix+" ", quoteIfNeeded(v))
		} else {
			writeLine(b, childMeta, k, keyPrefix, "")
		}
	}
}

// writeFlexLines is the shared implementation for flex node serialization.
// Container/list children always use serializeSetMetaNode (which recurses
// through the standard child dispatch, reaching leaves that call writeLine).
func writeFlexLines(b *strings.Builder, tree *Tree, meta *MetaTree, name string, node *FlexNode, prefix string, writeLine leafLineWriter) {
	if v, ok := tree.values[name]; ok {
		if v != configTrue {
			writeLine(b, meta, name, prefix+name+" ", quoteIfNeeded(v))
		} else {
			writeLine(b, meta, name, prefix+name, "")
		}
	}

	if mv := tree.multiValues[name]; len(mv) > 0 {
		for _, v := range mv {
			writeLine(b, meta, name, prefix+name+" ", v)
		}
	}

	if child := tree.containers[name]; child != nil {
		childPrefix := prefix + name + " "
		serializeSetMetaNode(b, child, metaContainerChild(meta, name), node, childPrefix)
	}

	if entries := tree.lists[name]; entries != nil {
		keys := make([]string, 0, len(entries))
		for k := range entries {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			entry := entries[key]
			entryPrefix := prefix + name + " " + quoteIfNeeded(key) + " "
			entryMeta := metaListEntry(meta, name, key)
			serializeSetMetaNode(b, entry, entryMeta, node, entryPrefix)
		}
	}
}

// writeInlineListLines is the shared implementation for inline list serialization.
func writeInlineListLines(b *strings.Builder, tree *Tree, meta *MetaTree, name string, node *InlineListNode, prefix string, writeLine leafLineWriter) {
	entries := tree.lists[name]
	if entries == nil {
		return
	}

	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		entry := entries[key]
		displayKey := StripListKeySuffix(key)
		entryPrefix := prefix + name + " " + quoteIfNeeded(displayKey) + " "
		entryMeta := metaListEntry(meta, name, key)

		for _, childName := range node.Children() {
			v, ok := entry.values[childName]
			if !ok {
				continue
			}
			writeLine(b, entryMeta, childName, entryPrefix+childName+" ", quoteIfNeeded(v))
		}
	}
}

// serializeSetMetaExtraValues writes extra values with metadata.
func serializeSetMetaExtraValues(b *strings.Builder, tree *Tree, meta *MetaTree, children []string, prefix string) {
	schemaNames := make(map[string]bool, len(children))
	for _, name := range children {
		schemaNames[name] = true
	}

	var extraKeys []string
	for k := range tree.values {
		if !schemaNames[k] {
			extraKeys = append(extraKeys, k)
		}
	}
	sort.Strings(extraKeys)

	for _, k := range extraKeys {
		writeMetaLeafLine(b, meta, k, prefix+k+" ", quoteIfNeeded(tree.values[k]))
	}
}

// writeDeleteMetaLines emits delete lines for meta entries without corresponding tree values.
// When a session deletes a leaf, metadata survives in the meta tree but the tree value is gone.
// This function serializes those entries so they round-trip through parse.
//
// Limitation: only handles leaf-level orphans at the current tree node. If an entire
// container is deleted from the tree while metadata exists deeper in the meta structure,
// those deeper entries are not emitted. This is acceptable because only leaf-level deletes
// go through writeThroughDelete; container-level deletes don't record metadata.
func writeDeleteMetaLines(b *strings.Builder, tree *Tree, meta *MetaTree, prefix string) {
	if meta == nil {
		return
	}

	var names []string
	for name := range meta.entries {
		if _, hasValue := tree.values[name]; !hasValue {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return
	}
	sort.Strings(names)

	for _, name := range names {
		for _, e := range meta.entries[name] {
			writeMetaPrefix(b, e)
			if e.Value != "" {
				// Session set a value, but tree lacks it (another session deleted).
				b.WriteString("set ")
				b.WriteString(prefix)
				b.WriteString(name)
				b.WriteString(" ")
				b.WriteString(quoteIfNeeded(e.Value))
			} else {
				// Session deleted the value.
				b.WriteString("delete ")
				b.WriteString(prefix)
				b.WriteString(name)
			}
			b.WriteString("\n")
		}
	}
}
