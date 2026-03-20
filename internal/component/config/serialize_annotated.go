// Design: docs/architecture/config/syntax.md — annotated config display with column-aware gutter
// Related: serialize_blame.go — blame view (fixed gutter, shared tree-walking helpers)
// Related: serialize.go — bare hierarchical tree serialization
// Related: serialize_set.go — set-format serialization (metaContainerChild, metaListEntry)
// Related: meta.go — MetaTree for metadata-aware serialization

package config

import (
	"fmt"
	"sort"
	"strings"
)

// Column widths for annotated gutter segments.
const (
	annotatedAuthorWidth   = 14 // Username padded/truncated to this width.
	annotatedDateWidth     = 11 // "MM-DD HH:MM" format.
	annotatedSourceWidth   = 16 // Origin padded/truncated to this width.
	annotatedChangesWidth  = 1  // Single marker character (+/-/*).
	annotatedColumnSpacing = 2  // Spaces between columns.
)

// ShowColumns controls which metadata columns appear in annotated output.
// Each field corresponds to a gutter segment. Only enabled columns are emitted.
type ShowColumns struct {
	Author  bool // Username from MetaEntry.User
	Date    bool // Formatted from MetaEntry.Time (session start)
	Source  bool // Origin from MetaEntry.Source
	Changes bool // Diff marker (+/-/*)
}

// AnyEnabled returns true if at least one column is enabled.
func (c ShowColumns) AnyEnabled() bool {
	return c.Author || c.Date || c.Source || c.Changes
}

// SerializeAnnotatedTree produces a hierarchical tree view with a configurable
// metadata gutter. When no columns are enabled, delegates to Serialize for
// identical output.
func SerializeAnnotatedTree(tree *Tree, meta *MetaTree, schema *Schema, columns ShowColumns) string {
	if schema == nil || tree == nil {
		return ""
	}
	if !columns.AnyEnabled() {
		return Serialize(tree, schema)
	}
	var b strings.Builder
	serializeAnnotatedTree(&b, tree, meta, schema.root, columns, 0)
	return b.String()
}

// SerializeAnnotatedSet produces flat set commands with a configurable metadata gutter.
// When no columns are enabled, delegates to SerializeSet for identical output.
func SerializeAnnotatedSet(tree *Tree, meta *MetaTree, schema *Schema, columns ShowColumns) string {
	if schema == nil || tree == nil {
		return ""
	}
	if !columns.AnyEnabled() {
		return SerializeSet(tree, schema)
	}
	var b strings.Builder
	serializeAnnotatedSetNode(&b, tree, meta, schema.root, columns, "")
	return b.String()
}

// sanitizePrintable removes non-printable characters (below 0x20 or 0x7F-0x9F)
// from a string to prevent terminal escape sequence injection.
func sanitizePrintable(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x20 && r != 0x7F && (r < 0x80 || r > 0x9F) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// truncateRunes truncates a string to at most maxRunes runes, preserving valid UTF-8.
func truncateRunes(s string, maxRunes int) string {
	if len(s) <= maxRunes {
		// Fast path: byte length within limit means rune count is too.
		return s
	}
	count := 0
	for i := range s {
		if count >= maxRunes {
			return s[:i]
		}
		count++
	}
	return s
}

// writeAnnotatedGutter writes the enabled gutter columns for a MetaEntry.
func writeAnnotatedGutter(b *strings.Builder, e MetaEntry, columns ShowColumns) {
	if columns.Author {
		user := truncateRunes(sanitizePrintable(e.User), annotatedAuthorWidth)
		fmt.Fprintf(b, "%-*s  ", annotatedAuthorWidth, user)
	}
	if columns.Date {
		if !e.Time.IsZero() {
			b.WriteString(e.Time.Format("01-02 15:04"))
		} else {
			b.WriteString(strings.Repeat(" ", annotatedDateWidth))
		}
		b.WriteString("  ")
	}
	if columns.Source {
		src := truncateRunes(sanitizePrintable(e.Source), annotatedSourceWidth)
		fmt.Fprintf(b, "%-*s  ", annotatedSourceWidth, src)
	}
	if columns.Changes {
		marker := computeBlameMarker(e)
		fmt.Fprintf(b, "%c  ", marker)
	}
}

// writeEmptyAnnotatedGutter writes blank padding matching the enabled columns.
func writeEmptyAnnotatedGutter(b *strings.Builder, columns ShowColumns) {
	if columns.Author {
		b.WriteString(strings.Repeat(" ", annotatedAuthorWidth+annotatedColumnSpacing))
	}
	if columns.Date {
		b.WriteString(strings.Repeat(" ", annotatedDateWidth+annotatedColumnSpacing))
	}
	if columns.Source {
		b.WriteString(strings.Repeat(" ", annotatedSourceWidth+annotatedColumnSpacing))
	}
	if columns.Changes {
		b.WriteString(strings.Repeat(" ", annotatedChangesWidth+annotatedColumnSpacing))
	}
}

// writeAnnotatedLeafGutter writes the gutter for a leaf with metadata, or empty padding.
func writeAnnotatedLeafGutter(b *strings.Builder, meta *MetaTree, name string, columns ShowColumns) {
	if meta != nil {
		if entries := meta.entries[name]; len(entries) > 0 {
			e := entries[len(entries)-1]
			if e.User != "" {
				writeAnnotatedGutter(b, e, columns)
				return
			}
		}
	}
	writeEmptyAnnotatedGutter(b, columns)
}

// writeAnnotatedOpenBraceGutter writes the gutter for an opening brace.
// Inherits metadata from the first child in the subtree.
func writeAnnotatedOpenBraceGutter(b *strings.Builder, meta *MetaTree, columns ShowColumns) {
	if e, ok := firstSubtreeEntry(meta); ok {
		writeAnnotatedGutter(b, e, columns)
		return
	}
	writeEmptyAnnotatedGutter(b, columns)
}

// writeAnnotatedCloseBraceGutter writes the gutter for a closing brace.
// Inherits metadata from the last child in the subtree.
func writeAnnotatedCloseBraceGutter(b *strings.Builder, meta *MetaTree, columns ShowColumns) {
	if e, ok := lastSubtreeEntry(meta); ok {
		writeAnnotatedGutter(b, e, columns)
		return
	}
	writeEmptyAnnotatedGutter(b, columns)
}

// SerializeAnnotatedSubtree produces annotated hierarchical output for a subtree
// at a specific schema node (used when showing config at a sub-path).
func SerializeAnnotatedSubtree(tree *Tree, meta *MetaTree, parent childProvider, columns ShowColumns) string {
	var b strings.Builder
	serializeAnnotatedTree(&b, tree, meta, parent, columns, 0)
	return b.String()
}

// SerializeAnnotatedSubtreeSet produces annotated set commands for a subtree
// at a specific schema node (used when showing config at a sub-path in set format).
func SerializeAnnotatedSubtreeSet(tree *Tree, meta *MetaTree, parent childProvider, columns ShowColumns) string {
	var b strings.Builder
	serializeAnnotatedSetNode(&b, tree, meta, parent, columns, "")
	return b.String()
}

// --- Annotated Tree Serialization ---

// serializeAnnotatedTree walks schema children, emitting annotated hierarchical output.
func serializeAnnotatedTree(b *strings.Builder, tree *Tree, meta *MetaTree, parent childProvider, columns ShowColumns, indent int) {
	for _, name := range parent.Children() {
		child := parent.Get(name)
		serializeAnnotatedTreeNode(b, tree, meta, name, child, columns, indent)
	}
	serializeAnnotatedExtraValues(b, tree, meta, parent.Children(), columns, indent)
}

// serializeAnnotatedTreeNode dispatches annotated serialization by node type.
//
//nolint:cyclop // exhaustive switch over all node types is intentional
func serializeAnnotatedTreeNode(b *strings.Builder, tree *Tree, meta *MetaTree, name string, node Node, columns ShowColumns, indent int) {
	prefix := strings.Repeat("\t", indent)

	switch n := node.(type) {
	case *LeafNode:
		if v, ok := tree.values[name]; ok {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(quoteIfNeeded(normalizeBool(v)))
			b.WriteString("\n")
		}

	case *MultiLeafNode:
		if v, ok := tree.values[name]; ok {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(v)
			b.WriteString("\n")
		}

	case *BracketLeafListNode:
		if v, ok := tree.values[name]; ok {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" [ ")
			b.WriteString(v)
			b.WriteString(" ]\n")
		}

	case *ValueOrArrayNode:
		if items := tree.GetSlice(name); len(items) > 0 {
			writeAnnotatedLeafGutter(b, meta, name, columns)
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
		serializeAnnotatedContainer(b, tree, meta, name, n, columns, indent)

	case *ListNode:
		serializeAnnotatedList(b, tree, meta, name, n, columns, indent)

	case *FreeformNode:
		serializeAnnotatedFreeform(b, tree, meta, name, columns, indent)

	case *FlexNode:
		serializeAnnotatedFlex(b, tree, meta, name, n, columns, indent)

	case *InlineListNode:
		serializeAnnotatedInlineList(b, tree, meta, name, n, columns, indent)
	}
}

// serializeAnnotatedContainer handles container nodes in annotated tree view.
func serializeAnnotatedContainer(b *strings.Builder, tree *Tree, meta *MetaTree, name string, node *ContainerNode, columns ShowColumns, indent int) {
	prefix := strings.Repeat("\t", indent)

	if node.Presence {
		if v, ok := tree.values[name]; ok {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			b.WriteString(prefix)
			b.WriteString(name)
			if v != configTrue {
				b.WriteString(" ")
				b.WriteString(quoteIfNeeded(v))
			}
			b.WriteString("\n")
		}
	}
	if child := tree.containers[name]; child != nil {
		childMeta := metaContainerChild(meta, name)
		writeAnnotatedOpenBraceGutter(b, childMeta, columns)
		b.WriteString(prefix)
		b.WriteString(name)
		b.WriteString(" {\n")
		serializeAnnotatedTree(b, child, childMeta, node, columns, indent+1)
		writeAnnotatedCloseBraceGutter(b, childMeta, columns)
		b.WriteString(prefix)
		b.WriteString("}\n")
	}
}

// serializeAnnotatedList handles list nodes in annotated tree view.
func serializeAnnotatedList(b *strings.Builder, tree *Tree, meta *MetaTree, name string, node *ListNode, columns ShowColumns, indent int) {
	entries := tree.lists[name]
	if entries == nil {
		return
	}

	prefix := strings.Repeat("\t", indent)
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
		entryMeta := metaListEntry(meta, name, key)
		writeAnnotatedOpenBraceGutter(b, entryMeta, columns)
		b.WriteString(prefix)
		b.WriteString(name)
		if key != KeyDefault {
			b.WriteString(" ")
			b.WriteString(quoteIfNeeded(displayKey))
		}
		b.WriteString(" {\n")
		serializeAnnotatedTree(b, entry, entryMeta, node, columns, indent+1)
		writeAnnotatedCloseBraceGutter(b, entryMeta, columns)
		b.WriteString(prefix)
		b.WriteString("}\n")
	}
}

// serializeAnnotatedFreeform handles freeform nodes in annotated tree view.
func serializeAnnotatedFreeform(b *strings.Builder, tree *Tree, meta *MetaTree, name string, columns ShowColumns, indent int) {
	child := tree.containers[name]
	if child == nil {
		return
	}

	prefix := strings.Repeat("\t", indent)
	innerPrefix := strings.Repeat("\t", indent+1)
	childMeta := metaContainerChild(meta, name)

	writeAnnotatedOpenBraceGutter(b, childMeta, columns)
	b.WriteString(prefix)
	b.WriteString(name)
	b.WriteString(" {\n")

	keys := make([]string, 0, len(child.values))
	for k := range child.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := child.values[k]
		writeAnnotatedLeafGutter(b, childMeta, k, columns)
		b.WriteString(innerPrefix)
		b.WriteString(k)
		if v != configTrue {
			if strings.HasPrefix(v, "[ ") && strings.HasSuffix(v, " ]") {
				b.WriteString(" ")
				b.WriteString(v)
			} else {
				b.WriteString(" [ ")
				b.WriteString(v)
				b.WriteString(" ]")
			}
		}
		b.WriteString("\n")
	}

	writeAnnotatedCloseBraceGutter(b, childMeta, columns)
	b.WriteString(prefix)
	b.WriteString("}\n")
}

// serializeAnnotatedFlex handles flex nodes in annotated tree view.
func serializeAnnotatedFlex(b *strings.Builder, tree *Tree, meta *MetaTree, name string, node *FlexNode, columns ShowColumns, indent int) {
	prefix := strings.Repeat("\t", indent)

	if v, ok := tree.values[name]; ok {
		writeAnnotatedLeafGutter(b, meta, name, columns)
		b.WriteString(prefix)
		b.WriteString(name)
		if v != configTrue {
			b.WriteString(" ")
			b.WriteString(quoteIfNeeded(v))
		}
		b.WriteString("\n")
	} else if mv := tree.multiValues[name]; len(mv) > 0 {
		for _, v := range mv {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(v)
			b.WriteString("\n")
		}
	}

	if child := tree.containers[name]; child != nil {
		flexChildMeta := metaContainerChild(meta, name)
		writeAnnotatedOpenBraceGutter(b, flexChildMeta, columns)
		b.WriteString(prefix)
		b.WriteString(name)
		b.WriteString(" {\n")
		serializeAnnotatedTree(b, child, flexChildMeta, node, columns, indent+1)
		writeAnnotatedCloseBraceGutter(b, flexChildMeta, columns)
		b.WriteString(prefix)
		b.WriteString("}\n")
	}

	if entries := tree.lists[name]; entries != nil {
		keys := make([]string, 0, len(entries))
		for k := range entries {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			entry := entries[key]
			entryMeta := metaListEntry(meta, name, key)
			writeAnnotatedOpenBraceGutter(b, entryMeta, columns)
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(quoteIfNeeded(key))
			b.WriteString(" {\n")
			serializeAnnotatedTree(b, entry, entryMeta, node, columns, indent+1)
			writeAnnotatedCloseBraceGutter(b, entryMeta, columns)
			b.WriteString(prefix)
			b.WriteString("}\n")
		}
	}
}

// serializeAnnotatedInlineList handles inline list entries in annotated tree view.
func serializeAnnotatedInlineList(b *strings.Builder, tree *Tree, meta *MetaTree, name string, node *InlineListNode, columns ShowColumns, indent int) {
	entries := tree.lists[name]
	if entries == nil {
		return
	}

	prefix := strings.Repeat("\t", indent)
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		entry := entries[key]
		displayKey := StripListKeySuffix(key)
		entryMeta := metaListEntry(meta, name, key)

		useInline := len(entry.containers) == 0 && len(entry.lists) == 0
		if useInline && len(entry.values) > 0 {
			writeAnnotatedOpenBraceGutter(b, entryMeta, columns)
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(quoteIfNeeded(displayKey))
			for _, attrName := range node.Children() {
				if v, ok := entry.values[attrName]; ok {
					b.WriteString(" ")
					b.WriteString(attrName)
					b.WriteString(" ")
					b.WriteString(quoteIfNeeded(v))
				}
			}
			b.WriteString("\n")
		} else {
			writeAnnotatedOpenBraceGutter(b, entryMeta, columns)
			b.WriteString(prefix)
			b.WriteString(name)
			b.WriteString(" ")
			b.WriteString(quoteIfNeeded(displayKey))
			b.WriteString(" {\n")
			innerPrefix := strings.Repeat("\t", indent+1)
			for _, childName := range node.Children() {
				v, ok := entry.values[childName]
				if !ok {
					continue
				}
				writeAnnotatedLeafGutter(b, entryMeta, childName, columns)
				b.WriteString(innerPrefix)
				b.WriteString(childName)
				b.WriteString(" ")
				b.WriteString(quoteIfNeeded(v))
				b.WriteString("\n")
			}
			writeAnnotatedCloseBraceGutter(b, entryMeta, columns)
			b.WriteString(prefix)
			b.WriteString("}\n")
		}
	}
}

// serializeAnnotatedExtraValues writes extra tree values with annotated gutters.
func serializeAnnotatedExtraValues(b *strings.Builder, tree *Tree, meta *MetaTree, children []string, columns ShowColumns, indent int) {
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
	if len(extraKeys) == 0 {
		return
	}
	sort.Strings(extraKeys)

	prefix := strings.Repeat("\t", indent)
	for _, k := range extraKeys {
		writeAnnotatedLeafGutter(b, meta, k, columns)
		b.WriteString(prefix)
		b.WriteString(k)
		b.WriteString(" ")
		b.WriteString(quoteIfNeeded(tree.values[k]))
		b.WriteString("\n")
	}
}

// --- Annotated Set Serialization ---

// serializeAnnotatedSetNode walks schema children, emitting annotated set commands.
func serializeAnnotatedSetNode(b *strings.Builder, tree *Tree, meta *MetaTree, parent childProvider, columns ShowColumns, prefix string) {
	for _, name := range parent.Children() {
		child := parent.Get(name)
		serializeAnnotatedSetChild(b, tree, meta, name, child, columns, prefix)
	}
}

// serializeAnnotatedSetChild dispatches annotated set serialization by node type.
//
//nolint:cyclop // exhaustive switch over all node types is intentional
func serializeAnnotatedSetChild(b *strings.Builder, tree *Tree, meta *MetaTree, name string, node Node, columns ShowColumns, prefix string) {
	path := prefix + name

	switch n := node.(type) {
	case *LeafNode:
		if v, ok := tree.values[name]; ok {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			fmt.Fprintf(b, "set %s %s\n", path, quoteIfNeeded(normalizeBool(v)))
		}

	case *MultiLeafNode:
		if v, ok := tree.values[name]; ok {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			fmt.Fprintf(b, "set %s %s\n", path, v)
		}

	case *BracketLeafListNode:
		if v, ok := tree.values[name]; ok {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			fmt.Fprintf(b, "set %s [ %s ]\n", path, v)
		}

	case *ValueOrArrayNode:
		if items := tree.GetSlice(name); len(items) > 0 {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			if len(items) == 1 {
				fmt.Fprintf(b, "set %s %s\n", path, quoteIfNeeded(items[0]))
			} else {
				b.WriteString("set ")
				b.WriteString(path)
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
		if n.Presence {
			if v, ok := tree.values[name]; ok {
				writeAnnotatedLeafGutter(b, meta, name, columns)
				if v == configTrue {
					fmt.Fprintf(b, "set %s\n", path)
				} else {
					fmt.Fprintf(b, "set %s %s\n", path, quoteIfNeeded(v))
				}
			}
		}
		if child := tree.containers[name]; child != nil {
			childMeta := metaContainerChild(meta, name)
			serializeAnnotatedSetNode(b, child, childMeta, n, columns, path+" ")
		}

	case *ListNode:
		serializeAnnotatedSetList(b, tree, meta, name, n, columns, path)

	case *FreeformNode:
		if child := tree.containers[name]; child != nil {
			childMeta := metaContainerChild(meta, name)
			keys := make([]string, 0, len(child.values))
			for k := range child.values {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				v := child.values[k]
				writeAnnotatedLeafGutter(b, childMeta, k, columns)
				if v == configTrue {
					fmt.Fprintf(b, "set %s %s\n", path, k)
				} else {
					fmt.Fprintf(b, "set %s %s [ %s ]\n", path, k, v)
				}
			}
		}

	case *FlexNode:
		serializeAnnotatedSetFlex(b, tree, meta, name, n, columns, path)

	case *InlineListNode:
		serializeAnnotatedSetInlineList(b, tree, meta, name, n, columns, path)
	}
}

// serializeAnnotatedSetList handles list nodes in annotated set format.
func serializeAnnotatedSetList(b *strings.Builder, tree *Tree, meta *MetaTree, name string, node *ListNode, columns ShowColumns, path string) {
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
		entryPath := path + " " + quoteIfNeeded(displayKey)
		entryMeta := metaListEntry(meta, name, key)
		serializeAnnotatedSetNode(b, entry, entryMeta, node, columns, entryPath+" ")
	}
}

// serializeAnnotatedSetFlex handles flex nodes in annotated set format.
func serializeAnnotatedSetFlex(b *strings.Builder, tree *Tree, meta *MetaTree, name string, node *FlexNode, columns ShowColumns, path string) {
	if v, ok := tree.values[name]; ok {
		writeAnnotatedLeafGutter(b, meta, name, columns)
		if v == configTrue {
			fmt.Fprintf(b, "set %s\n", path)
		} else {
			fmt.Fprintf(b, "set %s %s\n", path, quoteIfNeeded(v))
		}
	} else if mv := tree.multiValues[name]; len(mv) > 0 {
		for _, v := range mv {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			fmt.Fprintf(b, "set %s %s\n", path, v)
		}
	}

	if child := tree.containers[name]; child != nil {
		childMeta := metaContainerChild(meta, name)
		serializeAnnotatedSetNode(b, child, childMeta, node, columns, path+" ")
	}

	if entries := tree.lists[name]; entries != nil {
		keys := make([]string, 0, len(entries))
		for k := range entries {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			entry := entries[key]
			entryPath := path + " " + quoteIfNeeded(key)
			entryMeta := metaListEntry(meta, name, key)
			serializeAnnotatedSetNode(b, entry, entryMeta, node, columns, entryPath+" ")
		}
	}
}

// serializeAnnotatedSetInlineList handles inline list entries in annotated set format.
func serializeAnnotatedSetInlineList(b *strings.Builder, tree *Tree, meta *MetaTree, name string, node *InlineListNode, columns ShowColumns, path string) {
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
		entryPath := path + " " + quoteIfNeeded(displayKey)
		entryMeta := metaListEntry(meta, name, key)

		for _, childName := range node.Children() {
			v, ok := entry.values[childName]
			if !ok {
				continue
			}
			writeAnnotatedLeafGutter(b, entryMeta, childName, columns)
			fmt.Fprintf(b, "set %s %s %s\n", entryPath, childName, quoteIfNeeded(v))
		}
	}
}
