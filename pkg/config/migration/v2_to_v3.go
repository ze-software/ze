package migration

import (
	"github.com/exa-networks/zebgp/pkg/config"
)

// MigrateV2ToV3 transforms a v2 config tree to v3 format.
//
// Changes applied:
//   - neighbor <IP> → peer <IP>
//   - peer <glob> (root) → template { match <glob> }
//   - template { neighbor <name> } → template { group <name> }
//
// Returns a new tree; original is not modified.
func MigrateV2ToV3(tree *config.Tree) (*config.Tree, error) {
	// Already v3? Return clone without changes
	if DetectVersion(tree) == Version3 {
		return tree.Clone(), nil
	}

	result := tree.Clone()

	// Step 1: Move root peer globs → template.match (preserve order)
	peerGlobs := collectPeerGlobs(result)
	for _, entry := range peerGlobs {
		// Remove from root peer list
		result.RemoveListEntry("peer", entry.Key)

		// Add to template.match
		tmpl := result.GetOrCreateContainer("template")
		tmpl.AddListEntry("match", entry.Key, entry.Value)
	}

	// Step 2: Rename neighbor → peer at root level (preserve order)
	neighborEntries := result.GetListOrdered("neighbor")
	for _, entry := range neighborEntries {
		// Remove from neighbor list
		result.RemoveListEntry("neighbor", entry.Key)

		// Add to peer list
		result.AddListEntry("peer", entry.Key, entry.Value)
	}

	// Step 3: Rename template.neighbor → template.group
	if tmpl := result.GetContainer("template"); tmpl != nil {
		templateNeighbors := tmpl.GetListOrdered("neighbor")
		for _, entry := range templateNeighbors {
			// Remove from template.neighbor
			tmpl.RemoveListEntry("neighbor", entry.Key)

			// Add to template.group
			tmpl.AddListEntry("group", entry.Key, entry.Value)
		}
	}

	return result, nil
}

// collectPeerGlobs returns all peer entries that are glob patterns (contain * or /).
// Returns entries in insertion order.
func collectPeerGlobs(tree *config.Tree) []struct {
	Key   string
	Value *config.Tree
} {
	var globs []struct {
		Key   string
		Value *config.Tree
	}

	for _, entry := range tree.GetListOrdered("peer") {
		if isGlobPattern(entry.Key) {
			globs = append(globs, entry)
		}
	}

	return globs
}
