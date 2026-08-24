// Design: docs/architecture/config/syntax.md — Tree data structure
// Related: parser.go — config parser core
// Related: serialize.go — Tree serialization to config text
// Related: meta.go — MetaTree mirrors Tree navigation for metadata
// Related: prune.go — inactive node pruning

package config

import (
	"fmt"
	"maps"
	"sort"
	"sync"

	"github.com/ze-software/ze/internal/core/configorder"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Tree represents parsed configuration data.
//
// Safe for concurrent use: mu protects every map and slice below. Every
// public method acquires the appropriate lock; walkers in the same package
// that touch the internal fields directly MUST hold t.mu before reading or
// writing them. Each Tree (including sub-containers and list entries) owns
// its own mutex, so recursion into a child never re-acquires the parent's
// lock.
type Tree struct {
	mu             sync.RWMutex
	inactive       bool // Container/list-entry level deactivation (set by parent via SetInactive)
	values         map[string]string
	valuesOrder    []string            // Preserves insertion order for value keys
	multiValues    map[string][]string // For multiple inline values (e.g., multiple mup entries)
	inactiveValues map[string]bool     // Leaf-level deactivation; sibling to values, not encoded in the value string
	// inactiveMembers records per-member leaf-list deactivation, keyed by
	// leaf name then member value. Sibling to multiValues -- the member value
	// stays clean in multiValues (never carries an "inactive:" prefix), and the
	// effective-config accessors (GetSlice/GetMultiValues/ToMap) exclude any
	// member marked here. Serialize/diff/reactor read the raw slice + this map
	// via GetMultiValuesState. Deactivation is keyed by value, so it is only
	// unambiguous when the value is unique: a set (the default) never repeats a
	// value, and an ordered ze:ordered sequence that DOES repeat one (AS_PATH
	// prepends, MPLS labels) refuses deactivation of that value in
	// DeactivateMultiValue rather than silently marking every copy inactive.
	inactiveMembers map[string]map[string]bool
	containers      map[string]*Tree
	lists           map[string]map[string]*Tree
	listOrder       map[string][]string // Preserves insertion order for list keys
}

// NewTree creates an empty config tree.
func NewTree() *Tree {
	return &Tree{
		values:          make(map[string]string),
		multiValues:     make(map[string][]string),
		inactiveValues:  make(map[string]bool),
		inactiveMembers: make(map[string]map[string]bool),
		containers:      make(map[string]*Tree),
		lists:           make(map[string]map[string]*Tree),
		listOrder:       make(map[string][]string),
	}
}

// SetInactive marks this tree node (container or list entry) as deactivated.
func (t *Tree) SetInactive(v bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.inactive = v
}

// IsInactive reports whether this tree node is deactivated.
func (t *Tree) IsInactive() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.inactive
}

// SetLeafInactive records (or clears) leaf-level deactivation for name.
// The leaf value in t.values is left untouched -- inactivity lives in
// a sibling map so that the value round-trips through deactivate/activate
// without encoding tricks. Idempotent. Pre-marking a name that has no
// value yet is permitted (parser may set the flag before parsing the
// value into t.values; ordering is not enforced).
func (t *Tree) SetLeafInactive(name string, inactive bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if inactive {
		t.inactiveValues[name] = true
	} else {
		delete(t.inactiveValues, name)
	}
}

// IsLeafInactive reports whether the leaf at name is deactivated.
func (t *Tree) IsLeafInactive(name string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.inactiveValues[name]
}

// ClearLeafInactive removes the deactivation marker for name.
// Equivalent to SetLeafInactive(name, false); kept as a separate verb
// for symmetry with Tree.Delete-style APIs and reader call sites.
func (t *Tree) ClearLeafInactive(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.inactiveValues, name)
}

// removeValue deletes every leaf-level trace of name: the scalar value, its
// leaf-list members, the insertion-order entry, and the deactivation state that is
// sibling to both. Mirrors the field set pruneInactiveLeaves clears -- any leaf
// removal that misses one of these leaves the tree internally inconsistent
// (e.g. a valuesOrder entry with no value, which Values() would still report).
//
// Unexported because leaf state is package-internal: the exported API removes
// containers (RemoveContainer), list entries (RemoveListEntry) and leaf-list
// members (RemoveMultiValueMember), but never a whole leaf.
func (t *Tree) removeValue(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.values, name)
	delete(t.multiValues, name)
	delete(t.inactiveValues, name)
	delete(t.inactiveMembers, name)
	if len(t.valuesOrder) > 0 {
		filtered := t.valuesOrder[:0]
		for _, n := range t.valuesOrder {
			if n != name {
				filtered = append(filtered, n)
			}
		}
		t.valuesOrder = filtered
	}
}

// pruneInactiveLeaves removes every leaf entry whose name appears in
// inactiveValues from values, multiValues, valuesOrder, and the marker
// map itself. Called by pruneNode -- callers outside the prune walker
// should use PruneInactive instead.
func (t *Tree) pruneInactiveLeaves() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.inactiveValues) == 0 {
		return
	}
	for name := range t.inactiveValues {
		delete(t.values, name)
		delete(t.multiValues, name)
		delete(t.inactiveMembers, name)
	}
	if len(t.valuesOrder) > 0 {
		filtered := t.valuesOrder[:0]
		for _, n := range t.valuesOrder {
			if !t.inactiveValues[n] {
				filtered = append(filtered, n)
			}
		}
		t.valuesOrder = filtered
	}
	clear(t.inactiveValues)
}

// Get returns a leaf value.
func (t *Tree) Get(name string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	v, ok := t.values[name]
	return v, ok
}

// Set sets a leaf value.
func (t *Tree) Set(name, value string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.setLocked(name, value)
}

// setLocked is the lock-free core of Set. Caller MUST hold t.mu.Lock().
func (t *Tree) setLocked(name, value string) {
	if _, exists := t.values[name]; !exists {
		t.valuesOrder = append(t.valuesOrder, name)
	}
	t.values[name] = value
}

// AppendValue appends a value to the multi-values list (for Flex nodes with multiple entries).
func (t *Tree) AppendValue(name, value string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.multiValues[name] = append(t.multiValues[name], value)
}

// GetMultiValues returns the active (non-deactivated) members of a multi-value
// field in order. Deactivated members are excluded -- "deactivated" means "not
// in effect", consistent with the whole-leaf path. Callers needing every member
// plus its deactivation state (serialize, diff, reactor filter chain) must use
// GetMultiValuesState.
func (t *Tree) GetMultiValues(name string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.activeMembersLocked(name)
}

// SetSlice stores a leaf-list value as a string slice, preserving token
// boundaries. Replacing the whole leaf-list clears any stale per-member
// deactivation markers for the name.
func (t *Tree) SetSlice(name string, items []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.multiValues[name] = items
	delete(t.inactiveMembers, name)
}

// AppendSlice adds leaf-list members to any already stored under name,
// preserving insertion order and existing per-member deactivation markers, and
// skipping values already present (leaf-lists are sets). It is the accumulating
// counterpart to SetSlice: YANG models a leaf-list's repeated statements as
// additive (RFC 7950 sec 7.7), so the brace parser appends across `name value;`
// statements instead of the last one winning. The scalar mirror (values[name])
// is re-synced to the ACTIVE members, so it stays consistent with GetSlice even
// when an earlier statement deactivated a member (the caller must NOT also Set
// the mirror from the full list, which would leak the deactivated member).
func (t *Tree) AppendSlice(name string, items []string) {
	t.appendMembersLocked(name, items, true)
}

// AppendSequence adds members to an ordered-SEQUENCE leaf-list, preserving order
// AND duplicates. Unlike AppendSlice it never deduplicates, because the leaf-list
// models a sequence whose repeated values are load-bearing -- AS_PATH prepends
// (RFC 4271 Section 5.1.2) and MPLS label stacks (RFC 8277) -- not a set. Such
// nodes carry the ze:ordered YANG extension; the parser routes them here instead
// of AppendSlice. Without this, `as-path [ 65001 65001 65001 ]` would collapse to
// a single 65001 and silently drop the prepends.
func (t *Tree) AppendSequence(name string, items []string) {
	t.appendMembersLocked(name, items, false)
}

// appendMembersLocked is the shared body of AppendSlice (dedup=true, set
// semantics) and AppendSequence (dedup=false, ordered-sequence semantics).
func (t *Tree) appendMembersLocked(name string, items []string, dedup bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	members := t.multiValues[name]
	if !dedup {
		members = append(members, items...)
	} else {
		seen := make(map[string]struct{}, len(members)+len(items))
		for _, v := range members {
			seen[v] = struct{}{}
		}
		for _, v := range items {
			if _, dup := seen[v]; dup {
				continue
			}
			seen[v] = struct{}{}
			members = append(members, v)
		}
	}
	t.multiValues[name] = members
	t.syncMultiValueToValueLocked(name)
}

// GetSlice returns the active (non-deactivated) members of a leaf-list as a
// string slice in order. Returns nil if the key is not set or every member is
// deactivated. See GetMultiValues for the deactivation contract.
func (t *Tree) GetSlice(name string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.activeMembersLocked(name)
}

// MemberState is one leaf-list member together with its deactivation state,
// returned by GetMultiValuesState for consumers that need the full ordered
// member list (serialize, diff, the reactor filter chain).
type MemberState struct {
	Value    string
	Inactive bool
}

// GetMultiValuesState returns every member of a leaf-list in order, each tagged
// with whether it is deactivated. Unlike GetSlice/GetMultiValues it does NOT
// drop deactivated members -- it is the structural view.
func (t *Tree) GetMultiValuesState(name string) []MemberState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	items := t.multiValues[name]
	if len(items) == 0 {
		return nil
	}
	im := t.inactiveMembers[name]
	out := make([]MemberState, len(items))
	for i, v := range items {
		out[i] = MemberState{Value: v, Inactive: im[v]}
	}
	return out
}

// activeMembersLocked returns a copy of the active (non-deactivated) members of
// a leaf-list in order. Caller MUST hold t.mu (read or write).
func (t *Tree) activeMembersLocked(name string) []string {
	items := t.multiValues[name]
	if len(items) == 0 {
		return nil
	}
	im := t.inactiveMembers[name]
	if len(im) == 0 {
		out := make([]string, len(items))
		copy(out, items)
		return out
	}
	out := make([]string, 0, len(items))
	for _, v := range items {
		if !im[v] {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Clone creates a deep copy of the Tree.
// Used by migrations to safely transform config without affecting original.
func (t *Tree) Clone() *Tree {
	if t == nil {
		return nil
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	clone := NewTree()
	clone.inactive = t.inactive

	// Clone values
	maps.Copy(clone.values, t.values)

	// Clone multiValues
	for k, v := range t.multiValues {
		copied := make([]string, len(v))
		copy(copied, v)
		clone.multiValues[k] = copied
	}

	// Clone inactiveValues (leaf-level deactivation markers)
	maps.Copy(clone.inactiveValues, t.inactiveValues)

	// Clone inactiveMembers (per-member leaf-list deactivation markers)
	for name, members := range t.inactiveMembers {
		copied := make(map[string]bool, len(members))
		maps.Copy(copied, members)
		clone.inactiveMembers[name] = copied
	}

	// Clone containers (deep). v.Clone() takes v.mu.RLock(); a different
	// mutex from t.mu, so no reentrancy risk.
	for k, v := range t.containers {
		clone.containers[k] = v.Clone()
	}

	// Clone lists (deep)
	for listName, entries := range t.lists {
		clone.lists[listName] = make(map[string]*Tree)
		for entryKey, entryTree := range entries {
			clone.lists[listName][entryKey] = entryTree.Clone()
		}
	}

	// Clone listOrder
	for k, v := range t.listOrder {
		copied := make([]string, len(v))
		copy(copied, v)
		clone.listOrder[k] = copied
	}

	// Clone valuesOrder
	if len(t.valuesOrder) > 0 {
		clone.valuesOrder = make([]string, len(t.valuesOrder))
		copy(clone.valuesOrder, t.valuesOrder)
	}

	return clone
}

// GetFlex returns a value from either leaf values or the first multiValue.
// Used for Flex nodes that can be parsed as either Set() or AppendValue().
func (t *Tree) GetFlex(name string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if v, ok := t.values[name]; ok {
		return v, true
	}
	if mv := t.multiValues[name]; len(mv) > 0 {
		return mv[0], true
	}
	return "", false
}

// GetContainer returns a nested container.
func (t *Tree) GetContainer(name string) *Tree {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.containers[name]
}

// GetContainerPath resolves a PathSep-separated container path (e.g.
// "traffic/usage"), walking one container level per segment. Returns nil if any
// segment is absent. A single-segment path is equivalent to GetContainer.
func (t *Tree) GetContainerPath(path string) *Tree {
	cur := t
	for _, seg := range SplitPath(path) {
		if cur == nil {
			return nil
		}
		cur = cur.GetContainer(seg)
	}
	return cur
}

// SetContainer sets a nested container.
func (t *Tree) SetContainer(name string, child *Tree) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.containers[name] = child
}

// ContainerNames returns the names of all nested containers.
func (t *Tree) ContainerNames() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	names := make([]string, 0, len(t.containers))
	for k := range t.containers {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// CollectContainerPaths returns all container paths present in the tree,
// recursively walking nested containers. Paths use the config package's
// PathSep ("/"), matching ExtractConfigSubtree and the plugin auto-loader.
// Example: tree with fib { kernel { } } returns ["fib", "fib/kernel"].
// Plugin ConfigRoots MUST use the same separator for auto-load matching.
func CollectContainerPaths(t *Tree) []string {
	var paths []string
	collectPaths(t, "", &paths)
	return paths
}

func collectPaths(t *Tree, prefix string, paths *[]string) {
	for _, name := range t.ContainerNames() {
		path := name
		if prefix != "" {
			path = AppendPath(prefix, name)
		}
		*paths = append(*paths, path)
		child := t.GetContainer(name)
		if child != nil {
			collectPaths(child, path, paths)
		}
	}
}

// RemoveContainer removes a nested container and returns it.
// Returns nil if the container doesn't exist.
func (t *Tree) RemoveContainer(name string) *Tree {
	t.mu.Lock()
	defer t.mu.Unlock()
	c := t.containers[name]
	delete(t.containers, name)
	return c
}

// MergeContainer merges a container into existing one (or creates if not exists).
// This handles the case of multiple same-named blocks in config (e.g., multiple announce blocks).
//
// Called during config parsing/migration. Holds t.mu across the whole
// operation so a concurrent RemoveContainer/SetContainer cannot orphan
// `existing` mid-merge; recursive merges into sub-containers acquire the
// sub-container's own lock in parent-then-child order. `child` is the
// caller's (not yet shared) tree and is not locked.
func (t *Tree) MergeContainer(name string, child *Tree) {
	t.mu.Lock()
	defer t.mu.Unlock()
	existing := t.containers[name]
	if existing == nil {
		t.containers[name] = child
		return
	}
	existing.mergeFrom(child)
}

// mergeFrom merges the contents of `other` into t. Acquires t.mu itself;
// caller MUST NOT hold t.mu on t, but MAY hold t's ancestor's lock --
// this is how MergeContainer drives the recursive descent.
//
// Lock order invariant: parent first, child second. Every caller of
// mergeFrom must respect this order. There is no code path that takes
// a child mutex before its parent; if one is added, deadlock is possible.
// `other` is assumed caller-owned and unshared.
func (t *Tree) mergeFrom(other *Tree) {
	t.mu.Lock()
	defer t.mu.Unlock()

	maps.Copy(t.values, other.values)
	for k, v := range other.multiValues {
		t.multiValues[k] = append(t.multiValues[k], v...)
	}
	for k, v := range other.containers {
		if existing := t.containers[k]; existing != nil {
			// existing.mu is a different lock from t.mu; recurse in
			// parent-then-child order (we still hold t.mu here).
			existing.mergeFrom(v)
		} else {
			t.containers[k] = v
		}
	}
	for k, v := range other.lists {
		if t.lists[k] == nil {
			t.lists[k] = v
			t.listOrder[k] = other.listOrder[k]
			continue
		}
		for _, key := range other.listOrder[k] {
			if _, exists := t.lists[k][key]; !exists {
				t.listOrder[k] = append(t.listOrder[k], key)
			}
			t.lists[k][key] = v[key]
		}
	}
}

// GetList returns a list (keyed map of trees).
func (t *Tree) GetList(name string) map[string]*Tree {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lists[name]
}

// AddListEntry adds an entry to a list.
// For duplicate keys, generates unique keys by appending #N suffix.
// This supports ADD-PATH routes with same prefix but different path-info.
func (t *Tree) AddListEntry(name, key string, entry *Tree) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.lists[name] == nil {
		t.lists[name] = make(map[string]*Tree)
	}

	// Generate unique key for duplicates
	uniqueKey := key
	if _, exists := t.lists[name][key]; exists {
		// Find next available suffix
		for i := 1; ; i++ {
			var bk textbuf.Buffer
			uniqueKey = bk.Reset().Str(key).Byte('#').Int(int64(i)).String()
			if _, exists := t.lists[name][uniqueKey]; !exists {
				break
			}
		}
	}

	t.listOrder[name] = append(t.listOrder[name], uniqueKey)
	t.lists[name][uniqueKey] = entry
}

// GetListOrdered returns list entries in insertion order.
func (t *Tree) GetListOrdered(name string) []struct {
	Key   string
	Value *Tree
} {
	t.mu.RLock()
	defer t.mu.RUnlock()
	order := t.listOrder[name]
	list := t.lists[name]
	if list == nil {
		return nil
	}
	result := make([]struct {
		Key   string
		Value *Tree
	}, 0, len(order))
	for _, key := range order {
		if entry, ok := list[key]; ok {
			result = append(result, struct {
				Key   string
				Value *Tree
			}{key, entry})
		}
	}
	return result
}

// ListKeys returns the keys for a list (e.g., neighbor IPs).
func (t *Tree) ListKeys(name string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	list := t.lists[name]
	if list == nil {
		return nil
	}
	keys := make([]string, 0, len(list))
	for k := range list {
		keys = append(keys, k)
	}
	return keys
}

// Values returns all value keys in insertion order (for iterating Freeform entries).
func (t *Tree) Values() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.valuesOrder) > 0 {
		out := make([]string, len(t.valuesOrder))
		copy(out, t.valuesOrder)
		return out
	}
	keys := make([]string, 0, len(t.values))
	for k := range t.values {
		keys = append(keys, k)
	}
	return keys
}

// GetOrCreateContainer returns an existing container or creates a new one.
// Used by migrations to ensure a container exists before adding to it.
func (t *Tree) GetOrCreateContainer(name string) *Tree {
	t.mu.Lock()
	defer t.mu.Unlock()
	if c := t.containers[name]; c != nil {
		return c
	}
	c := NewTree()
	t.containers[name] = c
	return c
}

// RemoveListEntry removes and returns a specific list entry.
// Returns nil if the entry doesn't exist.
func (t *Tree) RemoveListEntry(listName, key string) *Tree {
	t.mu.Lock()
	defer t.mu.Unlock()

	list := t.lists[listName]
	if list == nil {
		return nil
	}
	entry, exists := list[key]
	if !exists {
		return nil
	}
	delete(list, key)

	// Remove from order
	newOrder := make([]string, 0, len(t.listOrder[listName]))
	for _, k := range t.listOrder[listName] {
		if k != key {
			newOrder = append(newOrder, k)
		}
	}
	t.listOrder[listName] = newOrder

	return entry
}

// RenameListEntry changes the key of a list entry, preserving its subtree and position.
// Returns an error if the old key does not exist or the new key already exists.
func (t *Tree) RenameListEntry(listName, oldKey, newKey string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	list := t.lists[listName]
	if list == nil {
		return fmt.Errorf("list %s not found", listName)
	}
	entry, exists := list[oldKey]
	if !exists {
		return fmt.Errorf("%s not found in %s", oldKey, listName)
	}
	if _, exists := list[newKey]; exists {
		return fmt.Errorf("%s already exists in %s", newKey, listName)
	}

	// Move entry to new key
	list[newKey] = entry
	delete(list, oldKey)

	// Replace old key with new key at same position in order
	for i, k := range t.listOrder[listName] {
		if k == oldKey {
			t.listOrder[listName][i] = newKey
			break
		}
	}

	return nil
}

// CopyListEntry clones a list entry under a new key, appended after the source.
// Returns an error if the source key does not exist or the target key already exists.
func (t *Tree) CopyListEntry(listName, srcKey, dstKey string) error {
	t.mu.Lock()

	list := t.lists[listName]
	if list == nil {
		t.mu.Unlock()
		return fmt.Errorf("list %s not found", listName)
	}
	entry, exists := list[srcKey]
	if !exists {
		t.mu.Unlock()
		return fmt.Errorf("%s not found in %s", srcKey, listName)
	}
	if _, exists := list[dstKey]; exists {
		t.mu.Unlock()
		return fmt.Errorf("%s already exists in %s", dstKey, listName)
	}

	// Release t.mu before entry.Clone() to avoid holding t.mu while
	// acquiring entry.mu.RLock(). Lock order: clone under entry.mu, then
	// re-acquire t.mu to mutate our own maps.
	t.mu.Unlock()
	cloned := entry.Clone()
	t.mu.Lock()
	defer t.mu.Unlock()

	// Re-check invariants after re-acquiring the lock. srcKey may have
	// been removed or rename-collapsed; dstKey may have been inserted
	// concurrently.
	list = t.lists[listName]
	if list == nil {
		return fmt.Errorf("list %s vanished during clone", listName)
	}
	if _, stillThere := list[srcKey]; !stillThere {
		return fmt.Errorf("%s removed from %s during clone", srcKey, listName)
	}
	if _, exists := list[dstKey]; exists {
		return fmt.Errorf("%s already exists in %s", dstKey, listName)
	}
	list[dstKey] = cloned
	order := t.listOrder[listName]
	for i, k := range order {
		if k != srcKey {
			continue
		}
		newOrder := make([]string, 0, len(order)+1)
		newOrder = append(newOrder, order[:i+1]...)
		newOrder = append(newOrder, dstKey)
		newOrder = append(newOrder, order[i+1:]...)
		t.listOrder[listName] = newOrder
		break
	}

	return nil
}

// Insert position constants for InsertMultiValue.
const (
	InsertFirst  = "first"
	InsertLast   = "last"
	InsertBefore = "before"
	InsertAfter  = "after"
)

// syncMultiValueToValueLocked updates the values map to match multiValues for
// a key, mirroring only the ACTIVE members (deactivated members are excluded,
// consistent with the effective-config accessors). Caller MUST hold t.mu.Lock().
func (t *Tree) syncMultiValueToValueLocked(name string) {
	active := t.activeMembersLocked(name)
	if len(active) == 0 {
		delete(t.values, name)
		return
	}
	t.setLocked(name, textbuf.Join(active, " "))
}

// InsertMultiValue inserts a value into a multi-value list at the specified position.
// position must be "first", "last", "before", or "after".
// ref is the reference value for before/after (ignored for first/last).
func (t *Tree) InsertMultiValue(name, value, position, ref string) error {
	if !isValidInsertPosition(position) {
		return fmt.Errorf("invalid position %q (use first, last, before, after)", position)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	items := t.multiValues[name]
	if multiValueIndex(items, value) >= 0 {
		return fmt.Errorf("%q already exists in %s", value, name)
	}

	switch position {
	case InsertFirst:
		t.multiValues[name] = append([]string{value}, items...)
	case InsertLast:
		t.multiValues[name] = append(items, value)
	case InsertBefore, InsertAfter:
		idx := multiValueIndex(items, ref)
		if idx < 0 {
			return fmt.Errorf("%q not found in %s", ref, name)
		}
		insertAt := idx
		if position == InsertAfter {
			insertAt = idx + 1
		}
		newItems := make([]string, 0, len(items)+1)
		newItems = append(newItems, items[:insertAt]...)
		newItems = append(newItems, value)
		newItems = append(newItems, items[insertAt:]...)
		t.multiValues[name] = newItems
	}

	t.syncMultiValueToValueLocked(name)
	return nil
}

// AddMultiValueMember appends a member to a leaf-list if not already present
// (JunOS add-member semantics: each set appends, duplicates are no-ops).
// A member already present in deactivated form also counts as present — set
// does not re-activate it (the deactivation marker is left untouched). Keeps
// the scalar values map in sync (joined copy) like InsertMultiValue.
func (t *Tree) AddMultiValueMember(name, value string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	items := t.multiValues[name]
	if multiValueIndex(items, value) >= 0 {
		return
	}
	t.multiValues[name] = append(items, value)
	t.syncMultiValueToValueLocked(name)
}

// HasMultiValueMember reports whether a leaf-list contains a member
// (in active or deactivated form).
func (t *Tree) HasMultiValueMember(name, value string) bool {
	present, _ := t.MultiValueMemberState(name, value)
	return present
}

// MultiValueMemberState reports whether a leaf-list member is present and
// whether it is deactivated. Used by commit-time apply logic to make member
// operations idempotent (desired state already reached is not an error).
func (t *Tree) MultiValueMemberState(name, value string) (present, inactive bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if multiValueIndex(t.multiValues[name], value) >= 0 {
		return true, t.inactiveMembers[name][value]
	}
	return false, false
}

// RemoveMultiValueMember removes one member from a leaf-list. Returns false if
// the member is not present. Also clears any deactivation marker for it. Keeps
// the scalar values map in sync; removing the last member clears the maps.
func (t *Tree) RemoveMultiValueMember(name, value string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	items := t.multiValues[name]
	idx := multiValueIndex(items, value)
	if idx < 0 {
		return false
	}
	t.multiValues[name] = append(items[:idx], items[idx+1:]...)
	if im := t.inactiveMembers[name]; im != nil {
		delete(im, value)
		if len(im) == 0 {
			delete(t.inactiveMembers, name)
		}
	}
	if len(t.multiValues[name]) == 0 {
		delete(t.multiValues, name)
	}
	t.syncMultiValueToValueLocked(name)
	return true
}

// DeactivateMultiValue marks a leaf-list member deactivated out-of-band (the
// member value stays clean in multiValues). Returns an error if the value is
// already deactivated or not found.
func (t *Tree) DeactivateMultiValue(name, value string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if multiValueIndex(t.multiValues[name], value) < 0 {
		return fmt.Errorf("%q not found in %s", value, name)
	}
	// A repeated value cannot be deactivated unambiguously: inactiveMembers is
	// value-keyed, so marking it would deactivate EVERY copy. A set never repeats a
	// value; only an ordered ze:ordered sequence (AS_PATH, MPLS labels) can, and
	// there the operator must edit the whole list, not one ambiguous copy. Fail
	// closed so a stray inactive: on a duplicate cannot silently blank the sequence.
	if n := multiValueOccurrences(t.multiValues[name], value); n > 1 {
		return fmt.Errorf("cannot deactivate %q in %s: it appears %d times; deactivation is ambiguous for a repeated member of an ordered leaf-list", value, name, n)
	}
	if t.inactiveMembers[name][value] {
		return fmt.Errorf("%q is already deactivated in %s", value, name)
	}
	if t.inactiveMembers[name] == nil {
		t.inactiveMembers[name] = make(map[string]bool)
	}
	t.inactiveMembers[name][value] = true
	t.syncMultiValueToValueLocked(name)
	return nil
}

// ActivateMultiValue clears the deactivation marker for a leaf-list member.
// Returns an error if the member is not currently deactivated.
func (t *Tree) ActivateMultiValue(name, value string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.inactiveMembers[name][value] {
		return fmt.Errorf("inactive:%s not found in %s", value, name)
	}
	delete(t.inactiveMembers[name], value)
	if len(t.inactiveMembers[name]) == 0 {
		delete(t.inactiveMembers, name)
	}
	t.syncMultiValueToValueLocked(name)
	return nil
}

// multiValueIndex returns the index of value in items, or -1 if not found.
func multiValueIndex(items []string, value string) int {
	for i, item := range items {
		if item == value {
			return i
		}
	}
	return -1
}

// multiValueOccurrences counts how many times value appears in items. Used to
// detect a repeated member, which value-keyed deactivation cannot address
// unambiguously (see DeactivateMultiValue).
func multiValueOccurrences(items []string, value string) int {
	n := 0
	for _, item := range items {
		if item == value {
			n++
		}
	}
	return n
}

// isValidInsertPosition returns true if position is a valid insert position keyword.
func isValidInsertPosition(position string) bool {
	return position == InsertFirst || position == InsertLast || position == InsertBefore || position == InsertAfter
}

// ToMap converts the Tree to a nested map[string]any suitable for JSON serialization.
//
// This is the general-purpose lowering. gNMI Get and Subscribe, `ze config show`,
// the web config handler, the support bundle and ValidateTreeAllModules all read
// its output, so every key in it is a name the YANG schema declares. It carries
// no entry order: a Go map has none, and none of those readers could take a key
// that is not in the schema.
//
// ToPluginMap is the lowering for a plugin's config payload, and it is the one
// that carries the order.
func (t *Tree) ToMap() map[string]any {
	return t.toMap(false)
}

// ToPluginMap converts the Tree to the nested map[string]any that a plugin
// receives, which is ToMap plus the entry order of every list holding two or
// more entries.
//
// The order travels beside its list, under configorder.OrderKey(listName), and
// configorder.Entries is the reader. It is emitted for every such list rather
// than only for the lists YANG declares `ordered-by user`, because no lowering
// call site holds the schema and building one costs a full YANG load. A reader
// that does not want the order never looks the key up.
//
// One entry needs no order, so a single-entry list is delivered exactly as
// ToMap delivers it. That keeps the reserved key out of the payload for the
// common case, and configorder.Entries answers a one-entry list without it.
func (t *Tree) ToPluginMap() map[string]any {
	return t.toMap(true)
}

// toMap is the one walker behind both lowerings. withListOrder decides whether
// a multi-entry list is followed by its order.
func (t *Tree) toMap(withListOrder bool) map[string]any {
	if t == nil {
		return nil
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]any)

	for k, v := range t.values {
		result[k] = v
	}

	// Emit only ACTIVE members: a deactivated leaf-list member is not part of
	// the effective config delivered to plugins (consistent with GetSlice and
	// the whole-leaf inactive path). Never leaks an "inactive:" string.
	for k := range t.multiValues {
		active := t.activeMembersLocked(k)
		switch len(active) {
		case 0:
			// wholly deactivated leaf-list: omit
		case 1:
			result[k] = active[0]
		default:
			result[k] = active
		}
	}

	// v.toMap() locks v.mu separately.
	for k, v := range t.containers {
		result[k] = v.toMap(withListOrder)
	}

	for listName, entries := range t.lists {
		listMap := make(map[string]any)
		for key, tree := range entries {
			listMap[key] = tree.toMap(withListOrder)
		}
		if len(listMap) == 0 {
			continue
		}
		result[listName] = listMap
		if withListOrder && len(listMap) > 1 {
			// A nil order is not an empty order: it means listOrder does not
			// cover this list, so emitting nothing makes configorder.Entries
			// refuse the list and name it. Emitting a partial or invented
			// order would hand the reader a first-match-wins list in an order
			// no operator wrote.
			if order := t.entryOrderLocked(listName, listMap); order != nil {
				result[configorder.OrderKey(listName)] = order
			}
		}
	}

	return result
}

// entryOrderLocked returns listName's entry keys in the order the operator
// wrote them. listMap is the lowered list, and it decides which entries are in
// the answer, so a key listOrder still names after its entry was removed is
// left out.
//
// It returns nil when the recorded order does not account for every entry in
// listMap. GetList hands out the live entry map, so a caller CAN add an entry
// without going through AddListEntry and leave listOrder short. Completing the
// answer here, by any rule, would invent an order the operator did not write
// and the reader could not tell from a real one. Nil says "no order was
// recorded", which is the one answer configorder.Entries refuses.
//
// Caller MUST hold t.mu.
func (t *Tree) entryOrderLocked(listName string, listMap map[string]any) []string {
	order := make([]string, 0, len(listMap))
	seen := make(map[string]bool, len(listMap))

	for _, key := range t.listOrder[listName] {
		if _, ok := listMap[key]; !ok || seen[key] {
			continue
		}
		seen[key] = true
		order = append(order, key)
	}
	if len(order) != len(listMap) {
		return nil
	}
	return order
}
