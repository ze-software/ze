// Design: docs/architecture/config/syntax.md — config parsing and loading
//
// Package config provides configuration parsing and diffing for ze.
package config

import (
	"reflect"
)

// ConfigDiff holds the difference between two config maps.
// Used by plugins to determine what changed on reload.
type ConfigDiff struct {
	Added   map[string]any      // Keys present in new but not old
	Removed map[string]any      // Keys present in old but not new
	Changed map[string]DiffPair // Keys present in both with different values
}

// DiffPair holds old and new values for a changed key.
type DiffPair struct {
	Old any `json:"old"`
	New any `json:"new"`
}

// DiffMaps computes a deep diff between two map[string]any.
// Nested maps are compared recursively with dotted key paths.
// Returns a ConfigDiff with Added, Removed, and Changed maps.
func DiffMaps(old, new map[string]any) *ConfigDiff {
	diff := &ConfigDiff{
		Added:   make(map[string]any),
		Removed: make(map[string]any),
		Changed: make(map[string]DiffPair),
	}

	diffMapsRecursive(old, new, "", diff)
	return diff
}

// diffMapsRecursive performs recursive comparison with path prefix.
func diffMapsRecursive(old, new map[string]any, prefix string, diff *ConfigDiff) {
	// Handle nil maps
	if old == nil {
		old = make(map[string]any)
	}
	if new == nil {
		new = make(map[string]any)
	}

	// Check for removed keys (in old but not new)
	for k, oldVal := range old {
		key := joinPath(prefix, k)
		if _, exists := new[k]; !exists {
			diff.Removed[key] = oldVal
		}
	}

	// Check for added and changed keys
	for k, newVal := range new {
		key := joinPath(prefix, k)
		oldVal, exists := old[k]

		if !exists {
			// Key added
			diff.Added[key] = newVal
			continue
		}

		// Key exists in both - check if values differ
		oldMap, oldIsMap := oldVal.(map[string]any)
		newMap, newIsMap := newVal.(map[string]any)

		switch {
		case oldIsMap && newIsMap:
			// Both are maps - recurse
			diffMapsRecursive(oldMap, newMap, key, diff)
		case oldIsMap != newIsMap:
			// Type changed (one is map, other isn't)
			diff.Changed[key] = DiffPair{Old: oldVal, New: newVal}
		case !deepEqual(oldVal, newVal):
			// Values differ
			diff.Changed[key] = DiffPair{Old: oldVal, New: newVal}
		}
	}
}

// joinPath joins prefix and key with the config path separator.
func joinPath(prefix, key string) string {
	return AppendPath(prefix, key)
}

// deepEqual compares two values for equality.
// Uses reflect.DeepEqual for complex types.
func deepEqual(a, b any) bool {
	return reflect.DeepEqual(a, b)
}
