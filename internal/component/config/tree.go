// Design: docs/architecture/config/syntax.md — Tree data structure
// Related: parser.go — config parser core
// Related: serialize.go — Tree serialization to config text

package config

import (
	"fmt"
	"maps"
	"sort"
)

// Tree represents parsed configuration data.
type Tree struct {
	values      map[string]string
	valuesOrder []string            // Preserves insertion order for value keys
	multiValues map[string][]string // For multiple inline values (e.g., multiple mup entries)
	containers  map[string]*Tree
	lists       map[string]map[string]*Tree
	listOrder   map[string][]string // Preserves insertion order for list keys
}

// NewTree creates an empty config tree.
func NewTree() *Tree {
	return &Tree{
		values:      make(map[string]string),
		multiValues: make(map[string][]string),
		containers:  make(map[string]*Tree),
		lists:       make(map[string]map[string]*Tree),
		listOrder:   make(map[string][]string),
	}
}

// Get returns a leaf value.
func (t *Tree) Get(name string) (string, bool) {
	v, ok := t.values[name]
	return v, ok
}

// Set sets a leaf value.
func (t *Tree) Set(name, value string) {
	// Track insertion order for new keys
	if _, exists := t.values[name]; !exists {
		t.valuesOrder = append(t.valuesOrder, name)
	}
	t.values[name] = value
}

// AppendValue appends a value to the multi-values list (for Flex nodes with multiple entries).
func (t *Tree) AppendValue(name, value string) {
	t.multiValues[name] = append(t.multiValues[name], value)
}

// GetMultiValues returns all values for a multi-value field.
func (t *Tree) GetMultiValues(name string) []string {
	return t.multiValues[name]
}

// SetSlice stores a leaf-list value as a string slice, preserving token boundaries.
func (t *Tree) SetSlice(name string, items []string) {
	t.multiValues[name] = items
}

// GetSlice returns a leaf-list value as a string slice.
// Returns nil if the key is not set.
func (t *Tree) GetSlice(name string) []string {
	return t.multiValues[name]
}

// Clone creates a deep copy of the Tree.
// Used by migrations to safely transform config without affecting original.
func (t *Tree) Clone() *Tree {
	if t == nil {
		return nil
	}

	clone := NewTree()

	// Clone values
	maps.Copy(clone.values, t.values)

	// Clone multiValues
	for k, v := range t.multiValues {
		copied := make([]string, len(v))
		copy(copied, v)
		clone.multiValues[k] = copied
	}

	// Clone containers (deep)
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

	return clone
}

// GetFlex returns a value from either leaf values or the first multiValue.
// Used for Flex nodes that can be parsed as either Set() or AppendValue().
func (t *Tree) GetFlex(name string) (string, bool) {
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
	return t.containers[name]
}

// SetContainer sets a nested container.
func (t *Tree) SetContainer(name string, child *Tree) {
	t.containers[name] = child
}

// ContainerNames returns the names of all nested containers.
func (t *Tree) ContainerNames() []string {
	names := make([]string, 0, len(t.containers))
	for k := range t.containers {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// RemoveContainer removes a nested container and returns it.
// Returns nil if the container doesn't exist.
func (t *Tree) RemoveContainer(name string) *Tree {
	c := t.containers[name]
	delete(t.containers, name)
	return c
}

// MergeContainer merges a container into existing one (or creates if not exists).
// This handles the case of multiple same-named blocks in config (e.g., multiple announce blocks).
func (t *Tree) MergeContainer(name string, child *Tree) {
	existing := t.containers[name]
	if existing == nil {
		t.containers[name] = child
		return
	}
	// Merge values.
	maps.Copy(existing.values, child.values)
	// Merge multiValues (append).
	for k, v := range child.multiValues {
		existing.multiValues[k] = append(existing.multiValues[k], v...)
	}
	// Merge containers (recursively).
	for k, v := range child.containers {
		existing.MergeContainer(k, v)
	}
	// Merge lists (preserving order).
	for k, v := range child.lists {
		if existing.lists[k] == nil {
			existing.lists[k] = v
			existing.listOrder[k] = child.listOrder[k]
		} else {
			// Append new keys in child's order.
			for _, key := range child.listOrder[k] {
				if _, exists := existing.lists[k][key]; !exists {
					existing.listOrder[k] = append(existing.listOrder[k], key)
				}
				existing.lists[k][key] = v[key]
			}
		}
	}
}

// GetList returns a list (keyed map of trees).
func (t *Tree) GetList(name string) map[string]*Tree {
	return t.lists[name]
}

// AddListEntry adds an entry to a list.
// For duplicate keys, generates unique keys by appending #N suffix.
// This supports ADD-PATH routes with same prefix but different path-info.
func (t *Tree) AddListEntry(name, key string, entry *Tree) {
	if t.lists[name] == nil {
		t.lists[name] = make(map[string]*Tree)
	}

	// Generate unique key for duplicates
	uniqueKey := key
	if _, exists := t.lists[name][key]; exists {
		// Find next available suffix
		for i := 1; ; i++ {
			uniqueKey = fmt.Sprintf("%s#%d", key, i)
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
	// Return in insertion order if available, otherwise fallback to map order
	if len(t.valuesOrder) > 0 {
		return t.valuesOrder
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

// ClearList removes all entries from a list.
// Reserved for future migrations that need bulk list replacement.
// Current migration uses RemoveListEntry for order preservation.
func (t *Tree) ClearList(name string) {
	delete(t.lists, name)
	delete(t.listOrder, name)
}

// ToMap converts the Tree to a nested map[string]any suitable for JSON serialization.
// Used for plugin config delivery - plugins receive config as JSON and extract what they need.
func (t *Tree) ToMap() map[string]any {
	if t == nil {
		return nil
	}

	result := make(map[string]any)

	// Add leaf values
	for k, v := range t.values {
		result[k] = v
	}

	// Add multi-values as arrays
	for k, v := range t.multiValues {
		if len(v) == 1 {
			result[k] = v[0]
		} else if len(v) > 1 {
			result[k] = v
		}
	}

	// Add containers (recursively)
	for k, v := range t.containers {
		result[k] = v.ToMap()
	}

	// Add lists as nested objects (key -> subtree)
	for listName, entries := range t.lists {
		listMap := make(map[string]any)
		for key, tree := range entries {
			listMap[key] = tree.ToMap()
		}
		if len(listMap) > 0 {
			result[listName] = listMap
		}
	}

	return result
}
