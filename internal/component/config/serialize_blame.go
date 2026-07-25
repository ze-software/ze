// Design: docs/architecture/config/syntax.md — blame view serialization
// Overview: serialize_set.go — set-format serialization (shares helpers)
// Related: meta.go — MetaTree for metadata-aware serialization
// Related: serialize.go — hierarchical text serialization
// Related: serialize_annotated.go — column-aware annotated serialization

package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	blameUserWidth   = 14
	blameGutterWidth = 29 // user(14) + date(5) + " " + time(5) + "  " + marker(1) + " " (trailing space)
)

// computeBlameMarker returns the blame marker character based on MetaEntry state.
// '+' for new entries (no previous value), '*' for modified (had a previous value).
func computeBlameMarker(e MetaEntry) rune {
	if e.Previous != "" {
		return '*'
	}
	return '+'
}

// writeMetaEntryGutter writes the full blame gutter for a MetaEntry.
// Username is truncated to blameUserWidth to keep the gutter fixed-width.
func writeMetaEntryGutter(b *textbuf.Buffer, e MetaEntry) {
	user := truncateRunes(sanitizePrintable(e.User), blameUserWidth)
	date := e.Time.Format("01-02")
	timeStr := e.Time.Format("15:04")
	marker := computeBlameMarker(e)
	fmt.Fprintf(b, "%-*s%s %s  %c ", blameUserWidth, user, date, timeStr, marker) //nolint:errcheck // output
}

// writeEmptyGutter writes an empty gutter (all spaces).
func writeEmptyGutter(b *textbuf.Buffer) {
	b.Str(strings.Repeat(" ", blameGutterWidth))
}

// writeBlameGutter writes the blame gutter for a leaf with metadata, or empty padding.
func writeBlameGutter(b *textbuf.Buffer, meta *MetaTree, name string) {
	if meta != nil {
		if entries := meta.entries[name]; len(entries) > 0 {
			e := entries[len(entries)-1]
			if e.User != "" {
				writeMetaEntryGutter(b, e)
				return
			}
		}
	}
	writeEmptyGutter(b)
}

// writeOpenBraceGutter writes the gutter for an opening brace line.
// Inherits the full gutter (user/date/time/marker) from the first child
// in the subtree that has metadata, per spec.
func writeOpenBraceGutter(b *textbuf.Buffer, meta *MetaTree) {
	if e, ok := firstSubtreeEntry(meta); ok {
		writeMetaEntryGutter(b, e)
		return
	}
	writeEmptyGutter(b)
}

// writeCloseBraceGutter writes the gutter for a closing brace line.
// Inherits the full gutter from the last child in the subtree that has metadata.
func writeCloseBraceGutter(b *textbuf.Buffer, meta *MetaTree) {
	if e, ok := lastSubtreeEntry(meta); ok {
		writeMetaEntryGutter(b, e)
		return
	}
	writeEmptyGutter(b)
}

// firstSubtreeEntry finds the first MetaEntry with a non-empty User in the subtree.
// Traverses entries, then containers, then lists, in sorted key order.
//
// Self-locking: callers pass a sub-MetaTree that the caller is NOT holding a
// lock on. Each recursive call locks its own receiver's mutex (different
// mutex per level, never recursive on the same one).
func firstSubtreeEntry(meta *MetaTree) (MetaEntry, bool) {
	if meta == nil {
		return MetaEntry{}, false
	}
	meta.mu.RLock()
	defer meta.mu.RUnlock()
	keys := sortedEntryKeys(meta.entries)
	for _, k := range keys {
		for _, e := range meta.entries[k] {
			if e.User != "" {
				return e, true
			}
		}
	}
	for _, k := range sortedMapKeys(meta.containers) {
		if e, ok := firstSubtreeEntry(meta.containers[k]); ok {
			return e, true
		}
	}
	for _, k := range sortedMapKeys(meta.lists) {
		if e, ok := firstSubtreeEntry(meta.lists[k]); ok {
			return e, true
		}
	}
	return MetaEntry{}, false
}

// lastSubtreeEntry finds the last MetaEntry with a non-empty User in the subtree.
// Traverses lists, then containers, then entries, in reverse sorted key order.
//
// Self-locking: same contract as firstSubtreeEntry.
func lastSubtreeEntry(meta *MetaTree) (MetaEntry, bool) {
	if meta == nil {
		return MetaEntry{}, false
	}
	meta.mu.RLock()
	defer meta.mu.RUnlock()
	for _, k := range reverseSortedMapKeys(meta.lists) {
		if e, ok := lastSubtreeEntry(meta.lists[k]); ok {
			return e, true
		}
	}
	for _, k := range reverseSortedMapKeys(meta.containers) {
		if e, ok := lastSubtreeEntry(meta.containers[k]); ok {
			return e, true
		}
	}
	keys := sortedEntryKeys(meta.entries)
	for i := len(keys) - 1; i >= 0; i-- {
		entries := meta.entries[keys[i]]
		for j := len(entries) - 1; j >= 0; j-- {
			if entries[j].User != "" {
				return entries[j], true
			}
		}
	}
	return MetaEntry{}, false
}

// sortedEntryKeys returns sorted keys from a map[string][]MetaEntry.
func sortedEntryKeys(m map[string][]MetaEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedMapKeys returns sorted keys from a map[string]*MetaTree.
func sortedMapKeys(m map[string]*MetaTree) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// reverseSortedMapKeys returns reverse-sorted keys from a map[string]*MetaTree.
func reverseSortedMapKeys(m map[string]*MetaTree) []string {
	keys := sortedMapKeys(m)
	for i, j := 0, len(keys)-1; i < j; i, j = i+1, j-1 {
		keys[i], keys[j] = keys[j], keys[i]
	}
	return keys
}

// SerializeBlame produces a hierarchical tree view with a fixed-width blame gutter
// showing authorship and change markers for each leaf.
func SerializeBlame(tree *Tree, meta *MetaTree, schema *Schema) string {
	var b textbuf.Buffer
	serializeBlameTree(&b, tree, meta, schema.root, 0)
	return b.String()
}

// serializeBlameTree walks schema children, emitting blame-annotated hierarchical output.
//
// Holds tree.mu.RLock (and meta.mu.RLock when meta is non-nil) for the walk
// so callees can read tree / meta internals directly. Recursion into sub-
// trees / sub-metas acquires their own locks independently.
func serializeBlameTree(b *textbuf.Buffer, tree *Tree, meta *MetaTree, parent childProvider, indent int) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	if meta != nil {
		meta.mu.RLock()
		defer meta.mu.RUnlock()
	}
	for _, name := range parent.Children() {
		child := parent.Get(name)
		serializeBlameTreeNode(b, tree, meta, name, child, indent)
	}
	serializeBlameExtraValues(b, tree, meta, parent.Children(), indent)
}

// serializeBlameTreeNode dispatches blame serialization by node type in hierarchical format.
//
//nolint:cyclop // exhaustive switch over all node types is intentional
func serializeBlameTreeNode(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node Node, indent int) {
	prefix := strings.Repeat("\t", indent)

	switch n := node.(type) {
	case *LeafNode:
		if n.Hidden || n.Ephemeral {
			break
		}
		if v, ok := tree.values[name]; ok {
			writeBlameGutter(b, meta, name)
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
			writeBlameGutter(b, meta, name)
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
			writeBlameGutter(b, meta, name)
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
		// Direct access: caller holds tree.mu.RLock via serializeBlameTree.
		if items := tree.multiValues[name]; len(items) > 0 {
			// Same form as the plain hierarchical serializer: bare members
			// plus one `inactive: <leaf> <member>` line per deactivated
			// member, never the internal "inactive:" prefix.
			bare, inactiveMembers := splitInactiveMembers(items, tree.inactiveMembers[name])
			writeBlameGutter(b, meta, name)
			b.Str(prefix)
			if tree.inactiveValues[name] {
				b.Str("inactive: ")
			}
			b.Str(name)
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
			for _, member := range inactiveMembers {
				writeBlameGutter(b, meta, name)
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
		serializeBlameContainer(b, tree, meta, name, n, indent)

	case *ListNode:
		if n.Hidden || n.Ephemeral {
			break
		}
		serializeBlameList(b, tree, meta, name, n, indent)

	case *FreeformNode:
		serializeBlameFreeform(b, tree, meta, name, indent)

	case *FlexNode:
		serializeBlameFlex(b, tree, meta, name, n, indent)

	case *InlineListNode:
		serializeBlameInlineList(b, tree, meta, name, n, indent)
	}
}

// serializeBlameContainer handles container nodes in hierarchical blame view.
func serializeBlameContainer(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *ContainerNode, indent int) {
	prefix := strings.Repeat("\t", indent)

	if node.Presence {
		if v, ok := tree.values[name]; ok {
			writeBlameGutter(b, meta, name)
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
		writeOpenBraceGutter(b, childMeta)
		b.Str(prefix)
		if child.IsInactive() {
			b.Str("inactive: ")
		}
		b.Str(name)
		b.Str(" {\n")
		serializeBlameTree(b, child, childMeta, node, indent+1)
		writeCloseBraceGutter(b, childMeta)
		b.Str(prefix)
		b.Str("}\n")
	}
}

// serializeBlameList handles list nodes in hierarchical blame view.
func serializeBlameList(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *ListNode, indent int) {
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
		writeOpenBraceGutter(b, entryMeta)
		b.Str(prefix)
		if entry.IsInactive() {
			b.Str("inactive: ")
		}
		b.Str(name)
		if key != KeyDefault {
			b.Str(" ")
			b.Str(quoteIfNeeded(displayKey))
		}
		b.Str(" {\n")
		serializeBlameTree(b, entry, entryMeta, node, indent+1)
		writeCloseBraceGutter(b, entryMeta)
		b.Str(prefix)
		b.Str("}\n")
	}
}

// serializeBlameFreeform handles freeform nodes in hierarchical blame view.
func serializeBlameFreeform(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, indent int) {
	child := tree.containers[name]
	if child == nil {
		return
	}

	prefix := strings.Repeat("\t", indent)
	innerPrefix := strings.Repeat("\t", indent+1)
	childMeta := metaContainerChild(meta, name)

	// firstSubtreeEntry in writeOpenBraceGutter self-locks childMeta.
	// Do not hold childMeta.mu here.
	writeOpenBraceGutter(b, childMeta)
	b.Str(prefix)
	b.Str(name)
	b.Str(" {\n")

	// Lock child + childMeta for the loop body's direct reads.
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
		writeBlameGutter(b, childMeta, k)
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

	writeCloseBraceGutter(b, childMeta)
	b.Str(prefix)
	b.Str("}\n")
}

// serializeBlameFlex handles flex nodes in hierarchical blame view.
func serializeBlameFlex(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *FlexNode, indent int) {
	prefix := strings.Repeat("\t", indent)

	if v, ok := tree.values[name]; ok {
		writeBlameGutter(b, meta, name)
		b.Str(prefix)
		b.Str(name)
		if v != configTrue {
			b.Str(" ")
			b.Str(quoteIfNeeded(v))
		}
		b.Str("\n")
	} else if mv := tree.multiValues[name]; len(mv) > 0 {
		for _, v := range mv {
			writeBlameGutter(b, meta, name)
			b.Str(prefix)
			b.Str(name)
			b.Str(" ")
			b.Str(v)
			b.Str("\n")
		}
	}

	if child := tree.containers[name]; child != nil {
		flexChildMeta := metaContainerChild(meta, name)
		writeOpenBraceGutter(b, flexChildMeta)
		b.Str(prefix)
		b.Str(name)
		b.Str(" {\n")
		serializeBlameTree(b, child, flexChildMeta, node, indent+1)
		writeCloseBraceGutter(b, flexChildMeta)
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
			writeOpenBraceGutter(b, entryMeta)
			b.Str(prefix)
			b.Str(name)
			b.Str(" ")
			b.Str(quoteIfNeeded(key))
			b.Str(" {\n")
			serializeBlameTree(b, entry, entryMeta, node, indent+1)
			writeCloseBraceGutter(b, entryMeta)
			b.Str(prefix)
			b.Str("}\n")
		}
	}
}

// serializeBlameInlineList handles inline list entries in hierarchical blame view.
func serializeBlameInlineList(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *InlineListNode, indent int) {
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

		// Snapshot inline/values decision under entry's lock; release
		// before firstSubtreeEntry (self-locks entryMeta).
		entry.mu.RLock()
		useInline := len(entry.containers) == 0 && len(entry.lists) == 0
		hasValues := useInline && len(entry.values) > 0
		entry.mu.RUnlock()

		writeOpenBraceGutter(b, entryMeta)
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
				writeBlameGutter(b, entryMeta, childName)
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
			writeCloseBraceGutter(b, entryMeta)
			b.Str(prefix)
			b.Str("}\n")
		}
	}
}

// serializeBlameExtraValues writes extra tree values with blame gutters.
func serializeBlameExtraValues(b *textbuf.Buffer, tree *Tree, meta *MetaTree, children []string, indent int) {
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
		writeBlameGutter(b, meta, k)
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
