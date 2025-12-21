package migration

import (
	"strings"

	"github.com/exa-networks/zebgp/pkg/config"
)

// DetectVersion examines a Tree to determine its schema version.
// Uses heuristic detection based on config structure.
//
// Priority: Any deprecated (v2) syntax means config needs migration,
// even if it also contains some v3 syntax. Check v2 FIRST.
func DetectVersion(tree *config.Tree) ConfigVersion {
	if tree == nil {
		return VersionUnknown
	}

	// Check oldest to newest - any deprecated syntax = needs migration

	// v2: Has neighbor at root, template.neighbor, or peer glob at root
	// If ANY deprecated pattern exists, config needs migration
	if hasV2Patterns(tree) {
		return Version2
	}

	// v3: Has peer (not glob) at root, template.group, or template.match
	if hasV3Patterns(tree) {
		return Version3
	}

	// No patterns found = assume current (empty or minimal config)
	return VersionCurrent
}

// hasV3Patterns checks for v3 config structure.
func hasV3Patterns(tree *config.Tree) bool {
	// v3: template.group or template.match exist
	if tmpl := tree.GetContainer("template"); tmpl != nil {
		if hasListEntries(tmpl, "group") || hasListEntries(tmpl, "match") {
			return true
		}
	}

	// v3: has peer at root that is NOT a glob pattern
	peerList := tree.GetList("peer")
	for key := range peerList {
		if !isGlobPattern(key) {
			return true
		}
	}

	return false
}

// hasV2Patterns checks for v2 config structure.
func hasV2Patterns(tree *config.Tree) bool {
	// v2: has "neighbor" at root level
	if hasListEntries(tree, "neighbor") {
		return true
	}

	// v2: has peer glob at root level
	peerList := tree.GetList("peer")
	for key := range peerList {
		if isGlobPattern(key) {
			return true
		}
	}

	// v2: template.neighbor exists
	if tmpl := tree.GetContainer("template"); tmpl != nil {
		if hasListEntries(tmpl, "neighbor") {
			return true
		}
	}

	// v2: any peer/neighbor/template.group has static block (should use announce)
	if hasStaticBlocks(tree) {
		return true
	}

	return false
}

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

// hasListEntries returns true if the tree has any entries for the given list name.
func hasListEntries(tree *config.Tree, listName string) bool {
	list := tree.GetList(listName)
	return len(list) > 0
}

// isGlobPattern returns true if the pattern contains wildcards or CIDR notation.
func isGlobPattern(pattern string) bool {
	return strings.Contains(pattern, "*") || strings.Contains(pattern, "/")
}
