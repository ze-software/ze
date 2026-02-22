// Design: docs/architecture/config/syntax.md — config migration

package migration

import (
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config"
)

// hasStaticBlocks returns true if any peer, neighbor, template.group, or template.match has a static block.
// Note: peer+static and template.match+static are defensive checks for scenarios that don't exist in practice.
func hasStaticBlocks(tree *config.Tree) bool {
	// Check neighbor blocks
	for _, entry := range tree.GetList("neighbor") {
		if entry.GetContainer("static") != nil {
			return true
		}
	}

	// Check peer blocks (defensive)
	for _, entry := range tree.GetList("peer") {
		if entry.GetContainer("static") != nil {
			return true
		}
	}

	// Check template.group and template.match blocks
	if tmpl := tree.GetContainer("template"); tmpl != nil {
		for _, entry := range tmpl.GetList("group") {
			if entry.GetContainer("static") != nil {
				return true
			}
		}
		// template.match+static is defensive
		for _, entry := range tmpl.GetList("match") {
			if entry.GetContainer("static") != nil {
				return true
			}
		}
	}

	return false
}

// hasOldAPIBlocks returns true if any peer or template block has old-style api syntax.
// Old style includes:
//   - Anonymous process block: api { processes [ foo ]; } - uses KeyDefault as key
//   - Named block with processes: api speaking { processes [ foo ]; }
//   - Format flags in receive/send: receive [ parsed packets consolidate ];
//   - State flag: neighbor-changes
func hasOldAPIBlocks(tree *config.Tree) bool {
	// Check peer blocks
	for _, entry := range tree.GetList("peer") {
		if hasOldStyleAPI(entry) {
			return true
		}
	}

	// Check neighbor blocks (defensive - should be caught by neighbor detection)
	for _, entry := range tree.GetList("neighbor") {
		if hasOldStyleAPI(entry) {
			return true
		}
	}

	// Check template.group and template.match blocks
	if tmpl := tree.GetContainer("template"); tmpl != nil {
		for _, entry := range tmpl.GetList("group") {
			if hasOldStyleAPI(entry) {
				return true
			}
		}
		for _, entry := range tmpl.GetList("match") {
			if hasOldStyleAPI(entry) {
				return true
			}
		}
	}

	return false
}

// hasOldStyleAPI returns true if any process block in the tree uses old syntax.
func hasOldStyleAPI(tree *config.Tree) bool {
	apiList := tree.GetList("process")
	for _, apiTree := range apiList {
		if isOldStyleAPIBlock(apiTree) {
			return true
		}
	}
	return false
}

// isOldStyleAPIBlock returns true if an process block uses old syntax.
func isOldStyleAPIBlock(apiTree *config.Tree) bool {
	// Check for processes/processes-match field (leaf-list, stored in multiValues)
	if items := apiTree.GetSlice("processes"); len(items) > 0 {
		return true
	}
	if items := apiTree.GetSlice("processes-match"); len(items) > 0 {
		return true
	}

	// Check for neighbor-changes flag at api level (maps to receive [ state ];)
	if _, ok := apiTree.GetFlex("neighbor-changes"); ok {
		return true
	}

	// Check for format flags in receive block (parsed, packets, consolidate)
	if recv := apiTree.GetContainer("receive"); recv != nil {
		for _, flag := range []string{"parsed", "packets", "consolidate"} {
			if _, ok := recv.GetFlex(flag); ok {
				return true
			}
		}
	}

	// Check for format flags in send block (will be dropped during migration)
	if send := apiTree.GetContainer("send"); send != nil {
		for _, flag := range []string{"parsed", "packets", "consolidate"} {
			if _, ok := send.GetFlex(flag); ok {
				return true
			}
		}
	}

	return false
}

// hasListEntries returns true if the tree has any entries for the given list name.
func hasListEntries(tree *config.Tree, listName string) bool {
	list := tree.GetList(listName)
	return len(list) > 0
}

// isGlobPattern returns true if the pattern contains wildcards or CIDR notation.
func isGlobPattern(pattern string) bool {
	return strings.Contains(pattern, "*") || strings.Contains(pattern, "/")
}

// --- Individual detection functions for transformation registry ---

// hasNeighborAtRoot returns true if tree has neighbor entries at root level.
func hasNeighborAtRoot(tree *config.Tree) bool {
	return hasListEntries(tree, "neighbor")
}

// hasPeerGlobPattern returns true if tree has peer entries with glob patterns.
func hasPeerGlobPattern(tree *config.Tree) bool {
	peerList := tree.GetList("peer")
	for key := range peerList {
		if isGlobPattern(key) {
			return true
		}
	}
	return false
}

// hasTemplateNeighbor returns true if tree has template.neighbor entries.
func hasTemplateNeighbor(tree *config.Tree) bool {
	if tmpl := tree.GetContainer("template"); tmpl != nil {
		return hasListEntries(tmpl, "neighbor")
	}
	return false
}
