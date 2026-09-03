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

	"github.com/ze-software/ze/internal/core/textbuf"
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
	var b textbuf.Buffer
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
	var b textbuf.Buffer
	serializeAnnotatedSetNode(&b, tree, meta, schema.root, columns, "")
	return b.String()
}

// sanitizePrintable removes non-printable characters (below 0x20 or 0x7F-0x9F)
// from a string to prevent terminal escape sequence injection.
func sanitizePrintable(s string) string {
	var b textbuf.Buffer
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
func writeAnnotatedGutter(b *textbuf.Buffer, e MetaEntry, columns ShowColumns) {
	if columns.Author {
		user := truncateRunes(sanitizePrintable(e.User), annotatedAuthorWidth)
		fmt.Fprintf(b, "%-*s  ", annotatedAuthorWidth, user) //nolint:errcheck // output
	}
	if columns.Date {
		if !e.Time.IsZero() {
			b.Str(e.Time.Format("01-02 15:04"))
		} else {
			b.Str(strings.Repeat(" ", annotatedDateWidth))
		}
		b.Str("  ")
	}
	if columns.Source {
		src := truncateRunes(sanitizePrintable(e.Source), annotatedSourceWidth)
		fmt.Fprintf(b, "%-*s  ", annotatedSourceWidth, src) //nolint:errcheck // output
	}
	if columns.Changes {
		marker := computeBlameMarker(e)
		fmt.Fprintf(b, "%c  ", marker) //nolint:errcheck // output
	}
}

// writeEmptyAnnotatedGutter writes blank padding matching the enabled columns.
func writeEmptyAnnotatedGutter(b *textbuf.Buffer, columns ShowColumns) {
	if columns.Author {
		b.Str(strings.Repeat(" ", annotatedAuthorWidth+annotatedColumnSpacing))
	}
	if columns.Date {
		b.Str(strings.Repeat(" ", annotatedDateWidth+annotatedColumnSpacing))
	}
	if columns.Source {
		b.Str(strings.Repeat(" ", annotatedSourceWidth+annotatedColumnSpacing))
	}
	if columns.Changes {
		b.Str(strings.Repeat(" ", annotatedChangesWidth+annotatedColumnSpacing))
	}
}

// writeAnnotatedLeafGutter writes the gutter for a leaf with metadata, or empty padding.
func writeAnnotatedLeafGutter(b *textbuf.Buffer, meta *MetaTree, name string, columns ShowColumns) {
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
func writeAnnotatedOpenBraceGutter(b *textbuf.Buffer, meta *MetaTree, columns ShowColumns) {
	if e, ok := firstSubtreeEntry(meta); ok {
		writeAnnotatedGutter(b, e, columns)
		return
	}
	writeEmptyAnnotatedGutter(b, columns)
}

// writeAnnotatedCloseBraceGutter writes the gutter for a closing brace.
// Inherits metadata from the last child in the subtree.
func writeAnnotatedCloseBraceGutter(b *textbuf.Buffer, meta *MetaTree, columns ShowColumns) {
	if e, ok := lastSubtreeEntry(meta); ok {
		writeAnnotatedGutter(b, e, columns)
		return
	}
	writeEmptyAnnotatedGutter(b, columns)
}

// SerializeAnnotatedSubtree produces annotated hierarchical output for a subtree
// at a specific schema node (used when showing config at a sub-path).
func SerializeAnnotatedSubtree(tree *Tree, meta *MetaTree, parent childProvider, columns ShowColumns) string {
	var b textbuf.Buffer
	serializeAnnotatedTree(&b, tree, meta, parent, columns, 0)
	return b.String()
}

// SerializeAnnotatedSubtreeSet produces annotated set commands for a subtree
// at a specific schema node (used when showing config at a sub-path in set format).
func SerializeAnnotatedSubtreeSet(tree *Tree, meta *MetaTree, parent childProvider, columns ShowColumns) string {
	var b textbuf.Buffer
	serializeAnnotatedSetNode(&b, tree, meta, parent, columns, "")
	return b.String()
}

// --- Annotated Tree Serialization ---

// serializeAnnotatedTree walks schema children, emitting annotated hierarchical output.
//
// Holds tree.mu.RLock (and meta.mu.RLock when meta is non-nil) for the walk
// so callees can read tree / meta internals directly. Recursion into sub-
// trees and sub-metas acquires their own locks independently.
func serializeAnnotatedTree(b *textbuf.Buffer, tree *Tree, meta *MetaTree, parent childProvider, columns ShowColumns, indent int) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	if meta != nil {
		meta.mu.RLock()
		defer meta.mu.RUnlock()
	}
	for _, name := range parent.Children() {
		child := parent.Get(name)
		serializeAnnotatedTreeNode(b, tree, meta, name, child, columns, indent)
	}
	serializeAnnotatedExtraValues(b, tree, meta, parent.Children(), columns, indent)
}

// serializeAnnotatedTreeNode dispatches annotated serialization by node type.
//
//nolint:cyclop // exhaustive switch over all node types is intentional
func serializeAnnotatedTreeNode(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node Node, columns ShowColumns, indent int) {
	prefix := strings.Repeat("\t", indent)

	switch n := node.(type) {
	case *LeafNode:
		if n.Hidden || n.Ephemeral {
			break
		}
		if v, ok := tree.values[name]; ok {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			b.Str(prefix)
			if tree.inactiveValues[name] {
				b.Str("inactive: ")
			}
			b.Str(name)
			b.Str(" ")
			b.Str(quoteIfNeeded(normalizeBool(v)))
			b.Str("\n")
		}

	case *MultiLeafNode:
		if v, ok := tree.values[name]; ok {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			b.Str(prefix)
			if tree.inactiveValues[name] {
				b.Str("inactive: ")
			}
			b.Str(name)
			b.Str(" ")
			b.Str(v)
			b.Str("\n")
		}

	case *BracketLeafListNode:
		if v, ok := tree.values[name]; ok {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			b.Str(prefix)
			if tree.inactiveValues[name] {
				b.Str("inactive: ")
			}
			b.Str(name)
			b.Str(" [ ")
			b.Str(v)
			b.Str(" ]\n")
		}

	case *ValueOrArrayNode:
		// Direct access: caller holds tree.mu.RLock via serializeAnnotatedTree.
		if items := tree.multiValues[name]; len(items) > 0 {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			b.Str(prefix)
			if tree.inactiveValues[name] {
				b.Str("inactive: ")
			}
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
		}

	case *ContainerNode:
		if n.Hidden || n.Ephemeral {
			break
		}
		serializeAnnotatedContainer(b, tree, meta, name, n, columns, indent)

	case *ListNode:
		if n.Hidden || n.Ephemeral {
			break
		}
		serializeAnnotatedList(b, tree, meta, name, n, columns, indent, "")

	case *FreeformNode:
		serializeAnnotatedFreeform(b, tree, meta, name, columns, indent)

	case *FlexNode:
		serializeAnnotatedFlex(b, tree, meta, name, n, columns, indent)

	case *InlineListNode:
		serializeAnnotatedInlineList(b, tree, meta, name, n, columns, indent)
	}
}

// serializeAnnotatedContainer handles container nodes in annotated tree view.
func serializeAnnotatedContainer(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *ContainerNode, columns ShowColumns, indent int) {
	prefix := strings.Repeat("\t", indent)

	if node.Presence {
		if v, ok := tree.values[name]; ok {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			b.Str(prefix)
			b.Str(name)
			if v != configTrue {
				b.Str(" ")
				b.Str(quoteIfNeeded(v))
			}
			b.Str("\n")
		}
	}
	if child := tree.containers[name]; child != nil {
		childMeta := metaContainerChild(meta, name)
		switch {
		case node.Flatten && canFlattenContainer(child, node):
			// Flat form: "attach process looking-glass { ... }" (flatten.go).
			serializeAnnotatedFlattenedContainer(b, child, childMeta, name, node, columns, indent)
		case canInlineContainer(child):
			// Inline form: write gutter + "containerName childName value"
			writeAnnotatedOpenBraceGutter(b, childMeta, columns)
			serializeContainerInline(b, child, name, node, indent)
		default:
			writeAnnotatedOpenBraceGutter(b, childMeta, columns)
			b.Str(prefix)
			if child.IsInactive() {
				b.Str("inactive: ")
			}
			b.Str(name)
			b.Str(" {\n")
			serializeAnnotatedTree(b, child, childMeta, node, columns, indent+1)
			writeAnnotatedCloseBraceGutter(b, childMeta, columns)
			b.Str(prefix)
			b.Str("}\n")
		}
	}
}

// serializeAnnotatedList handles list nodes in annotated tree view.
// keyword is empty for a plain list, and holds the parent container's name for
// a child of a ze:flatten container (flatten.go).
func serializeAnnotatedList(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *ListNode, columns ShowColumns, indent int, keyword string) {
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
		b.Str(prefix)
		if entry.IsInactive() {
			b.Str("inactive: ")
		}
		if keyword != "" {
			b.Str(keyword)
			b.Str(" ")
		}
		b.Str(name)
		if key != KeyDefault {
			b.Str(" ")
			b.Str(quoteIfNeeded(displayKey))
		}
		// One line in `show configuration` is one line here too, so the two views
		// show one shape. The gutter written above already inherits the entry's
		// first child, which is the leaf-list this line carries.
		if members, ok := inlineLeafListEntry(entry, node, key); ok {
			writeInlineLeafListMembers(b, members)
			continue
		}
		b.Str(" {\n")
		serializeAnnotatedTree(b, entry, entryMeta, node, columns, indent+1)
		writeAnnotatedCloseBraceGutter(b, entryMeta, columns)
		b.Str(prefix)
		b.Str("}\n")
	}
}

// serializeAnnotatedFreeform handles freeform nodes in annotated tree view.
func serializeAnnotatedFreeform(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, columns ShowColumns, indent int) {
	child := tree.containers[name]
	if child == nil {
		return
	}

	prefix := strings.Repeat("\t", indent)
	innerPrefix := strings.Repeat("\t", indent+1)
	childMeta := metaContainerChild(meta, name)

	// firstSubtreeEntry (used by writeAnnotatedOpenBraceGutter) self-locks
	// childMeta briefly -- must NOT hold childMeta.mu here.
	writeAnnotatedOpenBraceGutter(b, childMeta, columns)
	b.Str(prefix)
	b.Str(name)
	b.Str(" {\n")

	// Lock child + childMeta for the loop body's direct map reads and the
	// writeAnnotatedLeafGutter calls (which read childMeta.entries).
	child.mu.RLock()
	if childMeta != nil {
		childMeta.mu.RLock()
	}

	keys := make([]string, 0, len(child.values))
	for k := range child.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := child.values[k]
		writeAnnotatedLeafGutter(b, childMeta, k, columns)
		b.Str(innerPrefix)
		b.Str(k)
		if v != configTrue {
			if strings.HasPrefix(v, "[ ") && strings.HasSuffix(v, " ]") {
				b.Str(" ")
				b.Str(v)
			} else {
				b.Str(" [ ")
				b.Str(v)
				b.Str(" ]")
			}
		}
		b.Str("\n")
	}

	if childMeta != nil {
		childMeta.mu.RUnlock()
	}
	child.mu.RUnlock()

	// lastSubtreeEntry (used by writeAnnotatedCloseBraceGutter) self-locks
	// childMeta briefly -- must NOT hold childMeta.mu here.
	writeAnnotatedCloseBraceGutter(b, childMeta, columns)
	b.Str(prefix)
	b.Str("}\n")
}

// serializeAnnotatedFlex handles flex nodes in annotated tree view.
func serializeAnnotatedFlex(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *FlexNode, columns ShowColumns, indent int) {
	prefix := strings.Repeat("\t", indent)

	if v, ok := tree.values[name]; ok {
		writeAnnotatedLeafGutter(b, meta, name, columns)
		b.Str(prefix)
		b.Str(name)
		if v != configTrue {
			b.Str(" ")
			b.Str(quoteIfNeeded(v))
		}
		b.Str("\n")
	} else if mv := tree.multiValues[name]; len(mv) > 0 {
		for _, v := range mv {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			b.Str(prefix)
			b.Str(name)
			b.Str(" ")
			b.Str(v)
			b.Str("\n")
		}
	}

	if child := tree.containers[name]; child != nil {
		flexChildMeta := metaContainerChild(meta, name)
		writeAnnotatedOpenBraceGutter(b, flexChildMeta, columns)
		b.Str(prefix)
		b.Str(name)
		b.Str(" {\n")
		serializeAnnotatedTree(b, child, flexChildMeta, node, columns, indent+1)
		writeAnnotatedCloseBraceGutter(b, flexChildMeta, columns)
		b.Str(prefix)
		b.Str("}\n")
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
			b.Str(prefix)
			b.Str(name)
			b.Str(" ")
			b.Str(quoteIfNeeded(key))
			b.Str(" {\n")
			serializeAnnotatedTree(b, entry, entryMeta, node, columns, indent+1)
			writeAnnotatedCloseBraceGutter(b, entryMeta, columns)
			b.Str(prefix)
			b.Str("}\n")
		}
	}
}

// serializeAnnotatedInlineList handles inline list entries in annotated tree view.
func serializeAnnotatedInlineList(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *InlineListNode, columns ShowColumns, indent int) {
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

		// Snapshot the inline/values decision under entry's lock, then
		// release to avoid recursive RLock when writeAnnotatedOpenBraceGutter
		// calls firstSubtreeEntry (which self-locks entryMeta).
		entry.mu.RLock()
		useInline := len(entry.containers) == 0 && len(entry.lists) == 0
		hasValues := useInline && len(entry.values) > 0
		entry.mu.RUnlock()

		writeAnnotatedOpenBraceGutter(b, entryMeta, columns)
		if hasValues {
			b.Str(prefix)
			b.Str(name)
			b.Str(" ")
			b.Str(quoteIfNeeded(displayKey))
			entry.mu.RLock()
			for _, attrName := range node.Children() {
				if v, ok := entry.values[attrName]; ok {
					b.Str(" ")
					b.Str(attrName)
					b.Str(" ")
					b.Str(quoteIfNeeded(v))
				}
			}
			entry.mu.RUnlock()
			b.Str("\n")
		} else {
			b.Str(prefix)
			b.Str(name)
			b.Str(" ")
			b.Str(quoteIfNeeded(displayKey))
			b.Str(" {\n")
			innerPrefix := strings.Repeat("\t", indent+1)
			entry.mu.RLock()
			if entryMeta != nil {
				entryMeta.mu.RLock()
			}
			for _, childName := range node.Children() {
				v, ok := entry.values[childName]
				if !ok {
					continue
				}
				writeAnnotatedLeafGutter(b, entryMeta, childName, columns)
				b.Str(innerPrefix)
				b.Str(childName)
				b.Str(" ")
				b.Str(quoteIfNeeded(v))
				b.Str("\n")
			}
			if entryMeta != nil {
				entryMeta.mu.RUnlock()
			}
			entry.mu.RUnlock()
			writeAnnotatedCloseBraceGutter(b, entryMeta, columns)
			b.Str(prefix)
			b.Str("}\n")
		}
	}
}

// serializeAnnotatedExtraValues writes extra tree values with annotated gutters.
func serializeAnnotatedExtraValues(b *textbuf.Buffer, tree *Tree, meta *MetaTree, children []string, columns ShowColumns, indent int) {
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

// --- Annotated Set Serialization ---

// serializeAnnotatedSetNode walks schema children, emitting annotated set commands.
//
// Holds tree.mu.RLock (and meta.mu.RLock when meta is non-nil) for the walk.
// Recursion into sub-trees / sub-metas acquires their own locks.
func serializeAnnotatedSetNode(b *textbuf.Buffer, tree *Tree, meta *MetaTree, parent childProvider, columns ShowColumns, prefix string) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	if meta != nil {
		meta.mu.RLock()
		defer meta.mu.RUnlock()
	}
	for _, name := range parent.Children() {
		child := parent.Get(name)
		serializeAnnotatedSetChild(b, tree, meta, name, child, columns, prefix)
	}
}

// serializeAnnotatedSetChild dispatches annotated set serialization by node type.
//
//nolint:cyclop // exhaustive switch over all node types is intentional
func serializeAnnotatedSetChild(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node Node, columns ShowColumns, prefix string) {
	path := prefix + name

	switch n := node.(type) {
	case *LeafNode:
		if n.Hidden || n.Ephemeral {
			break
		}
		if v, ok := tree.values[name]; ok {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			b.Str(setOrNop(tree, name))
			b.Str(path)
			b.Str(" ")
			b.Str(quoteIfNeeded(normalizeBool(v)))
			b.Str("\n")
		}

	case *MultiLeafNode:
		if v, ok := tree.values[name]; ok {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			b.Str(setOrNop(tree, name))
			b.Str(path)
			b.Str(" ")
			b.Str(v)
			b.Str("\n")
		}

	case *BracketLeafListNode:
		if v, ok := tree.values[name]; ok {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			b.Str(setOrNop(tree, name))
			b.Str(path)
			b.Str(" [ ")
			b.Str(v)
			b.Str(" ]\n")
		}

	case *ValueOrArrayNode:
		if items := tree.multiValues[name]; len(items) > 0 {
			bare, inactive := splitInactiveMembers(items, tree.inactiveMembers[name])
			if len(inactive) > 0 || tree.inactiveValues[name] {
				inactiveSet := make(map[string]bool, len(inactive))
				for _, m := range inactive {
					inactiveSet[m] = true
				}
				wholeInactive := tree.inactiveValues[name]
				for _, member := range bare {
					writeAnnotatedLeafGutter(b, meta, name, columns)
					if inactiveSet[member] || wholeInactive {
						b.Str("nop ")
					} else {
						b.Str("set ")
					}
					b.Str(path)
					b.Str(" ")
					b.Str(quoteIfNeeded(member))
					b.Str("\n")
				}
			} else {
				writeAnnotatedLeafGutter(b, meta, name, columns)
				b.Str("set ")
				b.Str(path)
				if len(bare) == 1 {
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
				b.Str("\n")
			}
		}

	case *ContainerNode:
		if n.Hidden || n.Ephemeral {
			break
		}
		if n.Presence {
			if v, ok := tree.values[name]; ok {
				writeAnnotatedLeafGutter(b, meta, name, columns)
				if v == configTrue {
					fmt.Fprintf(b, "set %s\n", path) //nolint:errcheck // output
				} else {
					fmt.Fprintf(b, "set %s %s\n", path, quoteIfNeeded(v)) //nolint:errcheck // output
				}
			}
		}
		if child := tree.containers[name]; child != nil {
			if !n.Presence && child.IsInactive() {
				writeEmptyAnnotatedGutter(b, columns)
				b.Str("nop ")
				b.Str(path)
				b.Str("\n")
			}
			childMeta := metaContainerChild(meta, name)
			var tb textbuf.Buffer
			childPrefix := tb.Str(path).Byte(' ').String()
			serializeAnnotatedSetNode(b, child, childMeta, n, columns, childPrefix)
		}

	case *ListNode:
		if n.Hidden || n.Ephemeral {
			break
		}
		serializeAnnotatedSetList(b, tree, meta, name, n, columns, path)

	case *FreeformNode:
		if child := tree.containers[name]; child != nil {
			childMeta := metaContainerChild(meta, name)
			// child and childMeta are separate nodes; lock before direct access.
			child.mu.RLock()
			if childMeta != nil {
				childMeta.mu.RLock()
			}
			keys := make([]string, 0, len(child.values))
			for k := range child.values {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				v := child.values[k]
				writeAnnotatedLeafGutter(b, childMeta, k, columns)
				if v == configTrue {
					fmt.Fprintf(b, "set %s %s\n", path, k) //nolint:errcheck // output
				} else {
					fmt.Fprintf(b, "set %s %s [ %s ]\n", path, k, v) //nolint:errcheck // output
				}
			}
			if childMeta != nil {
				childMeta.mu.RUnlock()
			}
			child.mu.RUnlock()
		}

	case *FlexNode:
		serializeAnnotatedSetFlex(b, tree, meta, name, n, columns, path)

	case *InlineListNode:
		serializeAnnotatedSetInlineList(b, tree, meta, name, n, columns, path)
	}
}

// serializeAnnotatedSetList handles list nodes in annotated set format.
func serializeAnnotatedSetList(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *ListNode, columns ShowColumns, path string) {
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
		entryPath := tb.Reset().Str(path).Byte(' ').Str(quoteIfNeeded(displayKey)).String()
		if entry.IsInactive() {
			writeEmptyAnnotatedGutter(b, columns)
			b.Str("nop ")
			b.Str(entryPath)
			b.Str("\n")
		}
		entryMeta := metaListEntry(meta, name, key)
		entryPrefix := tb.Reset().Str(entryPath).Byte(' ').String()
		serializeAnnotatedSetNode(b, entry, entryMeta, node, columns, entryPrefix)
	}
}

// serializeAnnotatedSetFlex handles flex nodes in annotated set format.
func serializeAnnotatedSetFlex(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *FlexNode, columns ShowColumns, path string) {
	var tb textbuf.Buffer
	if v, ok := tree.values[name]; ok {
		writeAnnotatedLeafGutter(b, meta, name, columns)
		if v == configTrue {
			fmt.Fprintf(b, "set %s\n", path) //nolint:errcheck // output
		} else {
			fmt.Fprintf(b, "set %s %s\n", path, quoteIfNeeded(v)) //nolint:errcheck // output
		}
	} else if mv := tree.multiValues[name]; len(mv) > 0 {
		for _, v := range mv {
			writeAnnotatedLeafGutter(b, meta, name, columns)
			fmt.Fprintf(b, "set %s %s\n", path, v) //nolint:errcheck // output
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
			entryPath := tb.Reset().Str(path).Byte(' ').Str(quoteIfNeeded(key)).String()
			entryMeta := metaListEntry(meta, name, key)
			serializeAnnotatedSetNode(b, entry, entryMeta, node, columns, entryPath+" ")
		}
	}
}

// serializeAnnotatedSetInlineList handles inline list entries in annotated set format.
func serializeAnnotatedSetInlineList(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *InlineListNode, columns ShowColumns, path string) {
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
		entryPath := tb.Reset().Str(path).Byte(' ').Str(quoteIfNeeded(displayKey)).String()
		entryMeta := metaListEntry(meta, name, key)

		// entry and entryMeta are separate nodes; lock before direct access.
		entry.mu.RLock()
		if entryMeta != nil {
			entryMeta.mu.RLock()
		}
		for _, childName := range node.Children() {
			v, ok := entry.values[childName]
			if !ok {
				continue
			}
			writeAnnotatedLeafGutter(b, entryMeta, childName, columns)
			fmt.Fprintf(b, "set %s %s %s\n", entryPath, childName, quoteIfNeeded(v)) //nolint:errcheck // output
		}
		if entryMeta != nil {
			entryMeta.mu.RUnlock()
		}
		entry.mu.RUnlock()
	}
}
