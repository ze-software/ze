// Design: docs/architecture/config/syntax.md — inactive node pruning
// Related: tree.go — Tree data structure
// Related: yang_schema.go — inactive leaf injection
// Related: serialize.go — isInactiveTree shared helper

package config

// PruneInactive removes inactive containers and list entries from the tree.
// A node is inactive if its "inactive" leaf value is "true".
// Pruning is recursive: an inactive parent removes its entire subtree.
// The tree is modified in place. Call on a clone if the original must be preserved.
//
// The schema is required to distinguish containers from lists from leaves
// when walking the tree.
func PruneInactive(tree *Tree, schema *Schema) {
	if tree == nil || schema == nil {
		return
	}
	pruneNode(tree, schema.root)
}

// PruneActive removes active containers and list entries from the tree,
// keeping only inactive ones. The inverse of PruneInactive.
// The tree is modified in place. Call on a clone if the original must be preserved.
func PruneActive(tree *Tree, schema *Schema) {
	if tree == nil || schema == nil {
		return
	}
	pruneActiveNode(tree, schema.root)
}

// pruneActiveNode recursively removes active children from a tree node.
func pruneActiveNode(tree *Tree, node Node) {
	cp, ok := node.(childProvider)
	if !ok {
		return
	}

	for _, name := range cp.Children() {
		child := cp.Get(name)

		switch child.(type) {
		case *ContainerNode:
			sub := tree.GetContainer(name)
			if sub == nil {
				continue
			}
			if !isInactiveTree(sub) {
				tree.RemoveContainer(name)
				continue
			}
			// Keep inactive container but recurse to show its full subtree.

		case *ListNode:
			entries := tree.GetList(name)
			if entries == nil {
				continue
			}
			var toRemove []string
			for key, entry := range entries {
				if !isInactiveTree(entry) {
					toRemove = append(toRemove, key)
				}
			}
			for _, key := range toRemove {
				tree.RemoveListEntry(name, key)
			}
		}
	}
}

// pruneNode recursively removes inactive children from a tree node.
func pruneNode(tree *Tree, node Node) {
	cp, ok := node.(childProvider)
	if !ok {
		return
	}

	for _, name := range cp.Children() {
		child := cp.Get(name)

		switch child.(type) {
		case *ContainerNode:
			sub := tree.GetContainer(name)
			if sub == nil {
				continue
			}
			if isInactiveTree(sub) {
				tree.RemoveContainer(name)
				continue
			}
			pruneNode(sub, child)

		case *ListNode:
			entries := tree.GetList(name)
			if entries == nil {
				continue
			}
			// Collect keys to remove (avoid mutation during iteration).
			var toRemove []string
			for key, entry := range entries {
				if isInactiveTree(entry) {
					toRemove = append(toRemove, key)
				}
			}
			for _, key := range toRemove {
				tree.RemoveListEntry(name, key)
			}
			// Recurse into remaining entries.
			for _, entry := range tree.GetList(name) {
				pruneNode(entry, child)
			}
		}
	}
}
