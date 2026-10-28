// Design: docs/architecture/config/syntax.md — inactive node pruning
// Related: tree.go — Tree data structure
// Related: schema.go — ListNode.UserOrdered selects semantic entry order

package config

import (
	"maps"
	"slices"
	"strings"
)

// PruneUnchanged removes semantically unchanged nodes, keeping only differences
// and the ancestors needed to locate them. Plain keyed-list order is display
// state; only ListNode.UserOrdered makes entry order part of the comparison.
// The tree is modified in place. Call on a clone if the original must be preserved.
// Inactive state and intentionally present empty containers remain configuration.
//
// Prune BOTH directions to build a diff view -- the working tree against the
// baseline for additions and modifications, the baseline against the working tree
// for removals. A node deleted from the working tree exists only in the baseline,
// so only the second direction retains it.
//
// A nil baseline prunes nothing: with nothing to compare against, every node is new.
//
// Comments do not change configuration. Comparison ignores them without copying
// or sorting either input; retained nodes keep their original comments and order.
func PruneUnchanged(tree, baseline *Tree, schema *Schema) {
	if tree == nil {
		return
	}
	if baseline == nil {
		return
	}
	if schema == nil {
		return
	}
	pruneUnchangedNode(tree, baseline, schema.root)
}

// WithoutComments answers a copy of tree with every comment removed for visual
// rendering. Semantic comparison uses SubtreesEqual and ignores comments without
// copying; callers retaining nodes keep the operator's original comments.
func WithoutComments(tree *Tree) *Tree {
	bare := tree.Clone()
	bare.dropCommentsDeep()
	return bare
}

// pruneUnchangedNode walks the finite parsed tree with its schema. A container
// emptied by this walk is context, not an intentional empty-container edit.
func pruneUnchangedNode(tree, baseline *Tree, node Node) {
	cp, ok := node.(ChildProvider)
	if !ok {
		return
	}
	for _, name := range cp.Children() {
		child := cp.Get(name)
		if pruneChildEqual(tree, baseline, name, child) {
			removeChild(tree, name)
			continue
		}
		sub, base := tree.GetContainer(name), baseline.GetContainer(name)
		if sub != nil && base != nil {
			hadChildren := !pruneTreeEmpty(sub)
			pruneUnchangedNode(sub, base, child)
			if hadChildren && pruneTreeEmpty(sub) && sub.IsInactive() == base.IsInactive() {
				tree.RemoveContainer(name)
			}
		}
		pruneUnchangedListEntries(tree, baseline, name, child)
	}
	pruneUnchangedExtraValues(tree, baseline, cp.Children())
}

// pruneUnchangedListEntries handles every keyed representation, including Flex
// and InlineList. A changed relative order keeps the whole list as order context;
// an insertion or deletion alone still prunes unchanged common entries.
func pruneUnchangedListEntries(tree, baseline *Tree, name string, node Node) {
	entries, baseEntries := tree.GetList(name), baseline.GetList(name)
	if list, ok := node.(*ListNode); ok && list.UserOrdered {
		if pruneListReordered(tree, baseline, name) {
			return
		}
	}
	for _, key := range listKeys(tree, name) {
		entry, base := entries[key], baseEntries[key]
		if entry == nil {
			continue
		}
		if base == nil {
			continue
		}
		if SubtreesEqual(entry, base, node) {
			tree.RemoveListEntry(name, key)
			continue
		}
		hadChildren := !pruneTreeEmpty(entry)
		pruneUnchangedNode(entry, base, node)
		if hadChildren && pruneTreeEmpty(entry) && entry.IsInactive() == base.IsInactive() {
			tree.RemoveListEntry(name, key)
		}
	}
}

func pruneTreeEmpty(tree *Tree) bool {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	if len(tree.values) != 0 {
		return false
	}
	if len(tree.multiValues) != 0 {
		return false
	}
	if len(tree.containers) != 0 {
		return false
	}
	for _, entries := range tree.lists {
		if len(entries) != 0 {
			return false
		}
	}
	return true
}

// pruneListReordered compares the common keys in linear time. Membership edits
// alone do not change the relative order of surviving entries.
func pruneListReordered(tree, baseline *Tree, name string) bool {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	baseline.mu.RLock()
	defer baseline.mu.RUnlock()
	before := ListEntryKeys(baseline.lists[name], baseline.listOrder[name])
	after := ListEntryKeys(tree.lists[name], tree.listOrder[name])
	next := 0
	for _, key := range after {
		if _, common := baseline.lists[name][key]; !common {
			continue
		}
		for next < len(before) {
			old := before[next]
			next++
			if _, common := tree.lists[name][old]; !common {
				continue
			}
			if old != key {
				return true
			}
			break
		}
	}
	return false
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

// SubtreesEqual compares canonical configuration values without comments or
// stored secret spellings. Only ListNode.UserOrdered makes keyed-entry order
// significant. Scalar mirrors and equivalent parser storage are not changes.
// It never mutates either tree. Recursion follows the finite parsed tree; locks
// are acquired parent before child, as in Tree.Clone.
func SubtreesEqual(tree, baseline *Tree, node Node) bool {
	if pruneNodeEphemeral(node) {
		return true
	}
	if tree == baseline {
		return true
	}
	if tree == nil {
		return false
	}
	if baseline == nil {
		return false
	}
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	baseline.mu.RLock()
	defer baseline.mu.RUnlock()
	if tree.inactive != baseline.inactive {
		return false
	}
	cp, modeled := node.(ChildProvider)
	if modeled {
		for _, name := range cp.Children() {
			if !pruneChildEqualLocked(tree, baseline, name, cp.Get(name)) {
				return false
			}
		}
	}
	// Unknown fields are scalar values in the printer. Do not compare mirrors
	// of modeled leaf-lists here: their member store is authoritative.
	return pruneExtraValuesEqual(tree, baseline, node) &&
		pruneExtraValuesEqual(baseline, tree, node)
}

// pruneExtraValuesEqual requires both read locks. Checking both directions
// distinguishes an absent field from an intentionally empty scalar.
func pruneExtraValuesEqual(tree, baseline *Tree, node Node) bool {
	schema, modeled := node.(ChildProvider)
	_, freeform := node.(*FreeformNode)
	for name, value := range tree.values {
		if modeled && schema.Get(name) != nil {
			continue
		}
		base, present := baseline.values[name]
		if !present {
			return false
		}
		if freeform {
			if !pruneFreeformEqual(value, base) {
				return false
			}
		} else if value != base {
			return false
		}
		if tree.inactiveValues[name] != baseline.inactiveValues[name] {
			return false
		}
	}
	return true
}

// pruneFreeformEqual compares the bracket wrapping serializeFreeform emits
// without building that text. A true flag has no bracket body.
func pruneFreeformEqual(value, base string) bool {
	if value == base {
		return true
	}
	if value == configTrue {
		return false
	}
	if base == configTrue {
		return false
	}
	bracketed := strings.HasPrefix(value, "[ ") && strings.HasSuffix(value, " ]")
	baseBracketed := strings.HasPrefix(base, "[ ") && strings.HasSuffix(base, " ]")
	if bracketed == baseBracketed {
		return false
	}
	if bracketed {
		return len(value) == len(base)+4 && value[2:len(value)-2] == base
	}
	return len(base) == len(value)+4 && base[2:len(base)-2] == value
}

func pruneNodeEphemeral(node Node) bool {
	switch n := node.(type) {
	case *LeafNode:
		return n.Ephemeral
	case *ContainerNode:
		return n.Ephemeral
	case *ListNode:
		return n.Ephemeral
	}
	return false
}

func pruneChildEqual(tree, baseline *Tree, name string, node Node) bool {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	baseline.mu.RLock()
	defer baseline.mu.RUnlock()
	return pruneChildEqualLocked(tree, baseline, name, node)
}

func pruneChildEqualLocked(tree, baseline *Tree, name string, node Node) bool {
	if pruneNodeEphemeral(node) {
		return true
	}
	if !pruneScalarEqual(tree, baseline, name, node) {
		return false
	}
	if tree.inactiveValues[name] != baseline.inactiveValues[name] {
		return false
	}
	if !maps.Equal(tree.inactiveMembers[name], baseline.inactiveMembers[name]) {
		return false
	}
	if !SubtreesEqual(tree.containers[name], baseline.containers[name], node) {
		return false
	}
	return pruneListsEqual(tree, baseline, name, node)
}

// pruneScalarEqual projects each schema kind onto the value its printer reads.
// Both parents are read-locked. It reuses normalizeBool and bracketBody rather
// than serializing nodes, and never compares a derived leaf-list mirror.
func pruneScalarEqual(tree, baseline *Tree, name string, node Node) bool {
	value, present := tree.values[name]
	base, basePresent := baseline.values[name]
	switch n := node.(type) {
	case *LeafNode:
		if present != basePresent {
			return false
		}
		if n.Type == TypeEmpty {
			return true
		}
		return normalizeBool(value) == normalizeBool(base)
	case *MultiLeafNode:
		return present == basePresent && value == base
	case *BracketLeafListNode:
		members, baseMembers := tree.multiValues[name], baseline.multiValues[name]
		if len(members) > 0 && len(baseMembers) > 0 {
			return slices.Equal(members, baseMembers)
		}
		// Only legacy scalar-only bracket storage needs textual canonicalization.
		value, present = bracketBody(tree, name)
		base, basePresent = bracketBody(baseline, name)
		return present == basePresent && value == base
	case *ValueOrArrayNode:
		return slices.Equal(tree.multiValues[name], baseline.multiValues[name])
	case *FlexNode:
		return pruneFlexValuesEqual(tree, baseline, name)
	case *ContainerNode:
		if n.Presence {
			return pruneFlexValuesEqual(tree, baseline, name)
		}
		return true
	case *ListNode, *InlineListNode, *FreeformNode:
		// These kinds have no scalar representation on their parent.
		return true
	}
	return present == basePresent && value == base
}

// pruneFlexValuesEqual follows the scalar-before-sequence precedence shared by
// Flex and presence printers. A single replayed scalar is the same occurrence
// as a one-member hierarchical sequence, including multi-word values.
func pruneFlexValuesEqual(tree, baseline *Tree, name string) bool {
	value, present := tree.values[name]
	base, basePresent := baseline.values[name]
	if present {
		if basePresent {
			return value == base
		}
		members := baseline.multiValues[name]
		return len(members) == 1 && members[0] == value
	}
	if basePresent {
		members := tree.multiValues[name]
		return len(members) == 1 && members[0] == base
	}
	return slices.Equal(tree.multiValues[name], baseline.multiValues[name])
}

// pruneListsEqual requires both parents' read locks; entries lock themselves.
func pruneListsEqual(tree, baseline *Tree, name string, node Node) bool {
	entries, baseEntries := tree.lists[name], baseline.lists[name]
	if len(entries) != len(baseEntries) {
		return false
	}
	if list, ok := node.(*ListNode); ok && list.UserOrdered {
		if !slices.Equal(ListEntryKeys(entries, tree.listOrder[name]),
			ListEntryKeys(baseEntries, baseline.listOrder[name])) {
			return false
		}
	}
	for key, entry := range entries {
		if !SubtreesEqual(entry, baseEntries[key], node) {
			return false
		}
	}
	return true
}

// removeChild deletes every representation of name from tree.
//
// Presence containers and flex nodes can hold a value AND a container under one
// name, and a list shares the namespace too. The caller only reaches here when both
// trees hold name identically, so clearing every representation is correct.
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
	cp, ok := node.(ChildProvider)
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
	cp, ok := node.(ChildProvider)
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
