// Design: docs/architecture/config/syntax.md — inactive node pruning
// Related: tree.go — Tree data structure
// Related: serialize.go — serializeNode, the comparison of record for PruneUnchanged

package config

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// PruneUnchanged removes from tree every node that serializes identically in
// baseline, keeping only what differs plus the ancestors needed to locate it.
// The tree is modified in place. Call on a clone if the original must be preserved.
//
// "Unchanged" means "serializes identically": a child is pruned when serializeNode
// renders it the same for both trees. That is one rule for every node kind (leaf,
// leaf-list, container, presence container, list, flex, freeform, inline list), and
// it cannot drift from what the operator is shown, because it IS what the operator
// is shown. Deactivation comes for free: the serializer emits the "inactive: "
// prefix, so (de)activating a node changes its text and therefore keeps it.
//
// Prune BOTH directions to build a diff view -- the working tree against the
// baseline for additions and modifications, the baseline against the working tree
// for removals. A node deleted from the working tree exists only in the baseline,
// so only the second direction retains it.
//
// A nil baseline prunes nothing: with nothing to compare against, every node is new.
func PruneUnchanged(tree, baseline *Tree, schema *Schema) {
	if tree == nil || baseline == nil || schema == nil {
		return
	}
	pruneUnchangedNode(tree, baseline, schema.root)
}

// pruneUnchangedNode removes children that render identically in baseline, then
// recurses into the ones that differ so their unchanged descendants go too.
func pruneUnchangedNode(tree, baseline *Tree, node Node) {
	cp, ok := node.(childProvider)
	if !ok {
		return
	}

	for _, name := range cp.Children() {
		child := cp.Get(name)

		if childText(tree, name, child) == childText(baseline, name, child) {
			removeChild(tree, name)
			continue
		}

		// Differs: recurse so unchanged descendants inside it are pruned too.
		switch child.(type) {
		case *ContainerNode, *FlexNode:
			sub := tree.GetContainer(name)
			base := baseline.GetContainer(name)
			// A nil baseline container means the whole container is new:
			// keep every descendant, there is nothing to compare against.
			if sub != nil && base != nil {
				pruneUnchangedNode(sub, base, child)
			}

		case *ListNode:
			pruneUnchangedListEntries(tree, baseline, name, child)
		}
	}

	pruneUnchangedExtraValues(tree, baseline, cp.Children())
}

// pruneUnchangedListEntries prunes list entries that render identically and recurses
// into those that differ. An entry absent from the baseline is new and is kept whole.
func pruneUnchangedListEntries(tree, baseline *Tree, name string, node Node) {
	entries := tree.GetList(name)
	baseEntries := baseline.GetList(name)

	// Snapshot the keys first: the loop deletes from the same live map GetList
	// returns. See listKeys.
	for _, key := range listKeys(tree, name) {
		entry := entries[key]
		baseEntry := baseEntries[key]
		if entry == nil || baseEntry == nil {
			continue
		}
		if SerializeSubtree(entry, node) == SerializeSubtree(baseEntry, node) {
			tree.RemoveListEntry(name, key)
			continue
		}
		pruneUnchangedNode(entry, baseEntry, node)
	}
}

// pruneUnchangedExtraValues prunes values absent from the schema's children list
// (containers marked ze:allow-unknown-fields). serializeExtraValues renders these,
// so they need the same treatment as schema-known leaves.
func pruneUnchangedExtraValues(tree, baseline *Tree, children []string) {
	schemaNames := make(map[string]bool, len(children))
	for _, name := range children {
		schemaNames[name] = true
	}

	for _, k := range tree.Values() {
		if schemaNames[k] {
			continue
		}
		tv, tok := tree.Get(k)
		bv, bok := baseline.Get(k)
		if tok && bok && tv == bv && tree.IsLeafInactive(k) == baseline.IsLeafInactive(k) {
			tree.removeValue(k)
		}
	}
}

// childText renders one named child exactly as the serializer would, for comparison.
//
// Holds tree.mu.RLock because serializeNode reads tree.values / tree.containers /
// tree.lists directly -- the same contract serializeTree and serializeWithChildren
// honor. Recursion crosses into child trees, which lock their own mutexes.
func childText(tree *Tree, name string, node Node) string {
	if tree == nil {
		return ""
	}
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	var b textbuf.Buffer
	serializeNode(&b, tree, name, node, 0)
	return b.String()
}

// removeChild deletes every representation of name from tree.
//
// Presence containers and flex nodes can hold a value AND a container under one
// name, and a list shares the namespace too. The caller only reaches here when both
// trees render name identically, so clearing every representation is correct
// regardless of which one is in use -- and avoids a per-kind switch that would have
// to stay in step with serializeNode's nine cases.
func removeChild(tree *Tree, name string) {
	tree.RemoveContainer(name)
	for _, key := range listKeys(tree, name) {
		tree.RemoveListEntry(name, key)
	}
	tree.removeValue(name)
}

// listKeys snapshots the keys of a list before the caller mutates it.
//
// GetList returns the LIVE map (tree.go: it returns t.lists[name] and drops the
// read lock on return), so ranging it while RemoveListEntry write-locks and
// deletes from the same map means iterating unlocked over a map another call is
// mutating. Single-goroutine on a clone that is benign, but the safety would rest
// entirely on every caller cloning first. pruneNode collects keys for exactly this
// reason ("avoid mutation during iteration"); match it rather than rely on the
// caller.
func listKeys(tree *Tree, name string) []string {
	entries := tree.GetList(name)
	if len(entries) == 0 {
		return nil
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	return keys
}

// PruneInactive removes inactive containers and list entries from the tree.
// A node is inactive if its Tree.inactive flag is set.
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
			if !sub.IsInactive() {
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
				if !entry.IsInactive() {
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

	// Drop leaf-level inactive entries before walking structural children.
	// The marker is engine state on the parent Tree, not part of the YANG
	// schema, so this step has no schema-walk equivalent.
	tree.pruneInactiveLeaves()

	for _, name := range cp.Children() {
		child := cp.Get(name)

		switch child.(type) {
		case *ContainerNode:
			sub := tree.GetContainer(name)
			if sub == nil {
				continue
			}
			if sub.IsInactive() {
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
				if entry.IsInactive() {
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
