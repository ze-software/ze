// Design: docs/architecture/config/syntax.md — set-format serialization
// Detail: serialize_blame.go — blame view serialization
// Related: serialize.go — hierarchical text serialization
// Related: setparser.go — set-format parsing (inverse of this file)
// Related: meta.go — MetaTree for metadata-aware serialization
// Related: serialize_annotated.go — column-aware annotated serialization

package config

import (
	"bufio"
	"sort"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
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
	var b textbuf.Buffer
	serializeSetNode(&b, tree, schema.root, "")
	return b.String()
}

// emitSetInactive emits an `inactive <path>` line if the leaf at name
// is marked inactive. Caller must hold tree.mu.RLock and have already
// emitted the matching `set` line so the inactive declaration refers
// to a known path.
func emitSetInactive(b *textbuf.Buffer, tree *Tree, name, prefix string) {
	if !tree.inactiveValues[name] {
		return
	}
	b.Str("inactive ")
	b.Str(prefix)
	b.Str(name)
	b.Str("\n")
}

// serializeSetNode walks the schema children in order, emitting set commands.
// prefix accumulates the path segments (e.g., "neighbor 192.0.2.1 ").
//
// Holds tree.mu.RLock for the duration of the walk so callees can read
// tree.values / tree.containers / tree.lists directly. Recursion into child
// trees acquires the child's own lock independently.
func serializeSetNode(b *textbuf.Buffer, tree *Tree, parent childProvider, prefix string) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
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
func serializeSetChild(b *textbuf.Buffer, tree *Tree, name string, node Node, prefix string) {

	switch n := node.(type) {
	case *LeafNode:
		if v, ok := tree.values[name]; ok {
			b.Str("set ")
			b.Str(prefix)
			b.Str(name)
			b.Str(" ")
			b.Str(quoteIfNeeded(normalizeBool(v)))
			b.Str("\n")
			emitSetInactive(b, tree, name, prefix)
		}

	case *MultiLeafNode:
		if v, ok := tree.values[name]; ok {
			b.Str("set ")
			b.Str(prefix)
			b.Str(name)
			b.Str(" ")
			b.Str(v)
			b.Str("\n")
			emitSetInactive(b, tree, name, prefix)
		}

	case *BracketLeafListNode:
		if v, ok := tree.values[name]; ok {
			b.Str("set ")
			b.Str(prefix)
			b.Str(name)
			b.Str(" [ ")
			b.Str(v)
			b.Str(" ]\n")
			emitSetInactive(b, tree, name, prefix)
		}

	case *ValueOrArrayNode:
		// Direct access: caller holds tree.mu.RLock (see serializeSetNode).
		if items := tree.multiValues[name]; len(items) > 0 {
			b.Str("set ")
			b.Str(prefix)
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
			b.Str("\n")
			emitSetInactive(b, tree, name, prefix)
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
func serializeSetContainer(b *textbuf.Buffer, tree *Tree, name string, node *ContainerNode, prefix string) {
	var tb textbuf.Buffer
	if node.Presence {
		// Presence container as flag or value
		if v, ok := tree.values[name]; ok {
			b.Str("set ")
			b.Str(prefix)
			b.Str(name)
			if v != configTrue {
				b.Str(" ")
				b.Str(quoteIfNeeded(v))
			}
			b.Str("\n")
		}
		// Presence container with children
		if child := tree.containers[name]; child != nil {
			childPrefix := tb.Reset().Str(prefix).Str(name).Byte(' ').String()
			serializeSetNode(b, child, node, childPrefix)
		}
		return
	}

	// Regular container
	if child := tree.containers[name]; child != nil {
		childPrefix := tb.Reset().Str(prefix).Str(name).Byte(' ').String()
		serializeSetNode(b, child, node, childPrefix)
		emitSetInactiveStructural(b, child, tb.Reset().Str(prefix).Str(name).String())
	}
}

// emitSetInactiveStructural emits `inactive <path>` for a container
// or list entry that is deactivated.
func emitSetInactiveStructural(b *textbuf.Buffer, sub *Tree, path string) {
	if sub == nil {
		return
	}
	if !sub.IsInactive() {
		return
	}
	b.Str("inactive ")
	b.Str(path)
	b.Str("\n")
}

// serializeSetList handles list nodes with keyed entries.
func serializeSetList(b *textbuf.Buffer, tree *Tree, name string, node *ListNode, prefix string) {
	var tb textbuf.Buffer
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
		entryPrefix := tb.Reset().Str(prefix).Str(name).Byte(' ').Str(quoteIfNeeded(displayKey)).Byte(' ').String()
		serializeSetNode(b, entry, node, entryPrefix)
		emitSetInactiveStructural(b, entry, prefix+name+" "+quoteIfNeeded(displayKey))
	}
}

// serializeSetFreeform handles freeform nodes (set of key-value pairs).
func serializeSetFreeform(b *textbuf.Buffer, tree *Tree, name, prefix string) {
	child := tree.containers[name]
	if child == nil {
		return
	}

	// child is a separate Tree from the outer RLock holder; lock it
	// before reading its values map.
	child.mu.RLock()
	defer child.mu.RUnlock()

	keys := make([]string, 0, len(child.values))
	for k := range child.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := child.values[k]
		b.Str("set ")
		b.Str(prefix)
		b.Str(name)
		b.Str(" ")
		b.Str(k)
		if v != configTrue {
			b.Str(" ")
			b.Str(quoteIfNeeded(v))
		}
		b.Str("\n")
	}
}

// serializeSetFlex handles flex nodes (flag, value, container, or list forms).
func serializeSetFlex(b *textbuf.Buffer, tree *Tree, name string, node *FlexNode, prefix string) {
	var tb textbuf.Buffer
	// Simple value or flag
	if v, ok := tree.values[name]; ok {
		b.Str("set ")
		b.Str(prefix)
		b.Str(name)
		if v != configTrue {
			b.Str(" ")
			b.Str(quoteIfNeeded(v))
		}
		b.Str("\n")
	}

	// Multi-values
	if mv := tree.multiValues[name]; len(mv) > 0 {
		for _, v := range mv {
			b.Str("set ")
			b.Str(prefix)
			b.Str(name)
			b.Str(" ")
			b.Str(v)
			b.Str("\n")
		}
	}

	// Container form
	if child := tree.containers[name]; child != nil {
		childPrefix := tb.Reset().Str(prefix).Str(name).Byte(' ').String()
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
			entryPrefix := tb.Reset().Str(prefix).Str(name).Byte(' ').Str(quoteIfNeeded(key)).Byte(' ').String()
			serializeSetNode(b, entry, node, entryPrefix)
		}
	}
}

// serializeSetInlineList handles inline list entries.
func serializeSetInlineList(b *textbuf.Buffer, tree *Tree, name string, node *InlineListNode, prefix string) {
	var tb textbuf.Buffer
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
		entryPrefix := tb.Reset().Str(prefix).Str(name).Byte(' ').Str(quoteIfNeeded(displayKey)).Byte(' ').String()

		// entry is a separate Tree: lock it before reading entry.values
		// directly (the caller's lock covers tree, not entry).
		entry.mu.RLock()
		for _, childName := range node.Children() {
			v, ok := entry.values[childName]
			if !ok {
				continue
			}
			b.Str("set ")
			b.Str(entryPrefix)
			b.Str(childName)
			b.Str(" ")
			b.Str(quoteIfNeeded(v))
			b.Str("\n")
		}
		entry.mu.RUnlock()
	}
}

// serializeSetExtraValues writes tree values not in the schema.
func serializeSetExtraValues(b *textbuf.Buffer, tree *Tree, children []string, prefix string) {
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
		b.Str("set ")
		b.Str(prefix)
		b.Str(k)
		b.Str(" ")
		b.Str(quoteIfNeeded(tree.values[k]))
		b.Str("\n")
	}
}

// --- Metadata-aware serialization ---

// SerializeSetWithMeta converts a Tree to flat set commands with metadata prefixes.
// Each leaf value becomes one line: [#user @time %session] set <path> <value>.
// Lines without metadata in the MetaTree are emitted as bare set commands.
func SerializeSetWithMeta(tree *Tree, meta *MetaTree, schema *Schema) string {
	var b textbuf.Buffer
	serializeSetMetaNode(&b, tree, meta, schema.root, "")
	return b.String()
}

// writeMetaPrefix writes the metadata prefix for a leaf entry.
// Format: #user @source %ISO8601 ^previous (each present only if non-empty).
func writeMetaPrefix(b *textbuf.Buffer, e MetaEntry) {
	if e.User != "" {
		b.Str("#")
		b.Str(e.User)
		b.Str(" ")
	}
	if e.Source != "" {
		b.Str("@")
		b.Str(e.Source)
		b.Str(" ")
	}
	if !e.Time.IsZero() {
		b.Str("%")
		b.Str(e.Time.UTC().Format(time.RFC3339))
		b.Str(" ")
	}
	if e.Previous != "" {
		b.Str("^")
		if strings.ContainsAny(e.Previous, " \"\\") {
			// Quote and escape backslashes then double quotes.
			// Order matters: escape \ first to avoid double-escaping \".
			escaped := strings.ReplaceAll(e.Previous, "\\", "\\\\")
			escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
			b.Str("\"")
			b.Str(escaped)
			b.Str("\"")
		} else {
			b.Str(e.Previous)
		}
		b.Str(" ")
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
//
// Holds tree.mu.RLock (and meta.mu.RLock when meta is non-nil) for the
// duration of the walk so callees can read tree / meta internals directly.
// Recursion into child trees and sub-metas acquires their own locks
// independently.
func serializeSetMetaNode(b *textbuf.Buffer, tree *Tree, meta *MetaTree, parent childProvider, prefix string) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	if meta != nil {
		meta.mu.RLock()
		defer meta.mu.RUnlock()
	}
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
func serializeSetMetaChild(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node Node, prefix string) {
	var tb textbuf.Buffer
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
		// Direct access: caller holds tree.mu.RLock (see serializeSetMetaNode).
		if items := tree.multiValues[name]; len(items) > 0 {
			pathPfx := tb.Reset().Str(prefix).Str(name).Byte(' ').String()
			if len(items) == 1 {
				writeMetaLeafLine(b, meta, name, pathPfx, quoteIfNeeded(items[0]))
			} else {
				parts := make([]string, len(items))
				for i, item := range items {
					parts[i] = quoteIfNeeded(item)
				}
				writeMetaLeafLine(b, meta, name, pathPfx, tb.Reset().Str("[ ").Join(parts, " ").Str(" ]").String())
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
func writeMetaLeafLine(b *textbuf.Buffer, meta *MetaTree, name, pathPrefix, value string) {
	if meta != nil {
		entries := meta.entries[name]
		if len(entries) > 1 {
			// Contested leaf: emit one line per session entry with its own value.
			for _, e := range entries {
				writeMetaPrefix(b, e)
				if e.Value == "" && e.Source != "" {
					// Active session deleted the value (committed entries have no Source).
					b.Str("delete ")
					b.Str(strings.TrimRight(pathPrefix, " "))
					b.Str("\n")
					continue
				}
				b.Str("set ")
				b.Str(pathPrefix)
				if e.Value != "" {
					if !strings.HasSuffix(pathPrefix, " ") {
						b.Str(" ")
					}
					b.Str(quoteIfNeeded(e.Value))
				} else {
					// Sessionless (committed) entry: Value is always "" because
					// committed metadata doesn't store the value separately --
					// the tree value IS the committed value. Use the tree value.
					b.Str(value)
				}
				b.Str("\n")
			}
			return
		}
		if len(entries) == 1 {
			writeMetaPrefix(b, entries[0])
		}
	}
	b.Str("set ")
	b.Str(pathPrefix)
	b.Str(value)
	b.Str("\n")
}

// serializeSetMetaContainer handles container nodes with metadata.
func serializeSetMetaContainer(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *ContainerNode, prefix string) {
	var tb textbuf.Buffer
	if node.Presence {
		if v, ok := tree.values[name]; ok {
			if v != configTrue {
				writeMetaLeafLine(b, meta, name, tb.Reset().Str(prefix).Str(name).Byte(' ').String(), quoteIfNeeded(v))
			} else {
				writeMetaLeafLine(b, meta, name, tb.Reset().Str(prefix).Str(name).String(), "")
			}
		}
		if child := tree.containers[name]; child != nil {
			childPrefix := tb.Reset().Str(prefix).Str(name).Byte(' ').String()
			serializeSetMetaNode(b, child, metaContainerChild(meta, name), node, childPrefix)
		}
		return
	}

	if child := tree.containers[name]; child != nil {
		childPrefix := tb.Reset().Str(prefix).Str(name).Byte(' ').String()
		serializeSetMetaNode(b, child, metaContainerChild(meta, name), node, childPrefix)
	}
}

// serializeSetMetaList handles list nodes with metadata.
func serializeSetMetaList(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *ListNode, prefix string) {
	var tb textbuf.Buffer
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
		entryPrefix := tb.Reset().Str(prefix).Str(name).Byte(' ').Str(quoteIfNeeded(displayKey)).Byte(' ').String()
		entryMeta := metaListEntry(meta, name, key)
		serializeSetMetaNode(b, entry, entryMeta, node, entryPrefix)
	}
}

// leafLineWriter is a function that writes a single leaf line with metadata.
// pathPrefix is the path up to and including the trailing space before the value.
// value is the formatted value (may be empty for flag-style entries).
// writeMetaLeafLine satisfies this signature for metadata-aware set serialization.
type leafLineWriter func(b *textbuf.Buffer, meta *MetaTree, name, pathPrefix, value string)

// serializeSetMetaFreeform handles freeform nodes with metadata.
func serializeSetMetaFreeform(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name, prefix string) {
	writeFreeformLines(b, tree, meta, name, prefix, writeMetaLeafLine)
}

// serializeSetMetaFlex handles flex nodes with metadata.
func serializeSetMetaFlex(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *FlexNode, prefix string) {
	writeFlexLines(b, tree, meta, name, node, prefix, writeMetaLeafLine)
}

// serializeSetMetaInlineList handles inline list entries with metadata.
func serializeSetMetaInlineList(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *InlineListNode, prefix string) {
	writeInlineListLines(b, tree, meta, name, node, prefix, writeMetaLeafLine)
}

// writeFreeformLines is the shared implementation for freeform serialization.
//
// The non-meta writer ignores the childMeta argument (the nil path of
// writeLine). The meta writer (writeMetaLeafLine) reads childMeta.entries,
// which lives on a sub-MetaTree that the caller's meta.mu.RLock does NOT
// cover; lock it here before handing it to writeLine.
func writeFreeformLines(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name, prefix string, writeLine leafLineWriter) {
	var tb textbuf.Buffer
	child := tree.containers[name]
	if child == nil {
		return
	}

	childMeta := metaContainerChild(meta, name)

	// child is a separate Tree; lock it before reading its values map.
	child.mu.RLock()
	defer child.mu.RUnlock()
	if childMeta != nil {
		childMeta.mu.RLock()
		defer childMeta.mu.RUnlock()
	}

	keys := make([]string, 0, len(child.values))
	for k := range child.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := child.values[k]
		keyPrefix := tb.Reset().Str(prefix).Str(name).Byte(' ').Str(k).String()
		if v != configTrue {
			writeLine(b, childMeta, k, tb.Reset().Str(keyPrefix).Byte(' ').String(), quoteIfNeeded(v))
		} else {
			writeLine(b, childMeta, k, keyPrefix, "")
		}
	}
}

// writeFlexLines is the shared implementation for flex node serialization.
// Container/list children always use serializeSetMetaNode (which recurses
// through the standard child dispatch, reaching leaves that call writeLine).
func writeFlexLines(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *FlexNode, prefix string, writeLine leafLineWriter) {
	var tb textbuf.Buffer
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
		childPrefix := tb.Reset().Str(prefix).Str(name).Byte(' ').String()
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
			entryPrefix := tb.Reset().Str(prefix).Str(name).Byte(' ').Str(quoteIfNeeded(key)).Byte(' ').String()
			entryMeta := metaListEntry(meta, name, key)
			serializeSetMetaNode(b, entry, entryMeta, node, entryPrefix)
		}
	}
}

// writeInlineListLines is the shared implementation for inline list serialization.
func writeInlineListLines(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *InlineListNode, prefix string, writeLine leafLineWriter) {
	var tb textbuf.Buffer
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
		entryPrefix := tb.Reset().Str(prefix).Str(name).Byte(' ').Str(quoteIfNeeded(displayKey)).Byte(' ').String()
		entryMeta := metaListEntry(meta, name, key)

		// entry and entryMeta are separate nodes; lock each before
		// reading entry.values / entryMeta.entries.
		entry.mu.RLock()
		if entryMeta != nil {
			entryMeta.mu.RLock()
		}
		for _, childName := range node.Children() {
			v, ok := entry.values[childName]
			if !ok {
				continue
			}
			writeLine(b, entryMeta, childName, entryPrefix+childName+" ", quoteIfNeeded(v))
		}
		if entryMeta != nil {
			entryMeta.mu.RUnlock()
		}
		entry.mu.RUnlock()
	}
}

// serializeSetMetaExtraValues writes extra values with metadata.
func serializeSetMetaExtraValues(b *textbuf.Buffer, tree *Tree, meta *MetaTree, children []string, prefix string) {
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
func writeDeleteMetaLines(b *textbuf.Buffer, tree *Tree, meta *MetaTree, prefix string) {
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
				b.Str("set ")
				b.Str(prefix)
				b.Str(name)
				b.Str(" ")
				b.Str(quoteIfNeeded(e.Value))
			} else {
				// Session deleted the value.
				b.Str("delete ")
				b.Str(prefix)
				b.Str(name)
			}
			b.Str("\n")
		}
	}
}

// FilterSetByPath returns only the set/inactive lines whose path matches
// the given prefix. Returns all content when path is empty.
func FilterSetByPath(content string, path []string) string {
	if len(path) == 0 {
		return content
	}
	var tb textbuf.Buffer
	prefix := tb.Str("set ").Join(path, " ").String()
	inactivePrefix := tb.Reset().Str("inactive ").Join(path, " ").String()
	var buf textbuf.Buffer
	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == prefix || strings.HasPrefix(trimmed, prefix+" ") ||
			trimmed == inactivePrefix || strings.HasPrefix(trimmed, inactivePrefix+" ") {
			buf.Str(line)
			buf.Byte('\n')
		}
	}
	return buf.String()
}
