// Design: docs/architecture/config/syntax.md — config parsing and loading
// Related: serialize.go — hierarchical serialization
// Related: serialize_annotated.go — column-aware annotated serialization
// Related: parser.go — automatic brace insertion, which parses the flat form

package config

import (
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// A ze:flatten container is written with its own name as a leading keyword on
// each child statement:
//
//	attach process looking-glass { receive [ update ] }
//
// rather than as a nested block:
//
//	attach { process looking-glass { receive [ update ] } }
//
// The parser reads both spellings for every container already (parser.go,
// automatic brace insertion), so the extension changes what ze PRINTS, not what
// it accepts. It exists so an operator sees back the spelling the documentation
// teaches: a config that reads one way and prints another is a diff on every
// commit.

// hasFlattenExtension reports whether a YANG entry carries ze:flatten.
func hasFlattenExtension(entry *gyang.Entry) bool {
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:flatten" || strings.HasSuffix(ext.Keyword, ":flatten") {
			return true
		}
	}
	return false
}

// canFlattenContainer reports whether a container's DATA has the one shape the
// flat form can carry: list entries and nothing else. A leaf, a nested
// container, an inactive marker on the container itself, or a list written as a
// grouped multi-block has no flat spelling, so the caller falls back to the
// nested block, which is always valid.
func canFlattenContainer(tree *Tree, node *ContainerNode) bool {
	tree.mu.RLock()
	defer tree.mu.RUnlock()

	if tree.inactive || len(tree.inactiveValues) > 0 || len(tree.inactiveMembers) > 0 {
		return false
	}
	if len(tree.values) > 0 || len(tree.multiValues) > 0 || len(tree.containers) > 0 {
		return false
	}
	if len(tree.lists) == 0 {
		return false
	}
	for name := range tree.lists {
		listNode, ok := node.Get(name).(*ListNode)
		if !ok {
			return false
		}
		if usesMultiBlockForm(listNode) {
			return false
		}
	}
	return true
}

// usesMultiBlockForm reports whether serializeNode writes this list as one
// grouped block ("name { key1 val1; key2 val2; }") rather than one block per
// entry. Keep it in step with the ListNode branch of serializeNode.
func usesMultiBlockForm(node *ListNode) bool {
	return node.KeyName != "" && len(node.Children()) <= 2 && allChildrenAreLeaves(node)
}

// serializeFlattenedContainer writes each entry of each list child with the
// container name in front: "attach process looking-glass { ... }".
func serializeFlattenedContainer(b *textbuf.Buffer, tree *Tree, name string, node *ContainerNode, indent int) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()

	for _, childName := range node.Children() {
		listNode, ok := node.Get(childName).(*ListNode)
		if !ok || listNode.Hidden || listNode.Ephemeral {
			continue
		}
		if entries := tree.lists[childName]; entries != nil {
			serializeListBlocks(b, entries, childName, listNode, indent, name)
		}
	}
}

// serializeAnnotatedFlattenedContainer is serializeFlattenedContainer for the
// annotated view: same blocks, each line carrying its gutter.
func serializeAnnotatedFlattenedContainer(b *textbuf.Buffer, tree *Tree, meta *MetaTree, name string, node *ContainerNode, columns ShowColumns, indent int) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	if meta != nil {
		meta.mu.RLock()
		defer meta.mu.RUnlock()
	}

	for _, childName := range node.Children() {
		listNode, ok := node.Get(childName).(*ListNode)
		if !ok || listNode.Hidden || listNode.Ephemeral {
			continue
		}
		serializeAnnotatedList(b, tree, meta, childName, listNode, columns, indent, name)
	}
}
