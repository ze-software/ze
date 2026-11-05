// Design: docs/architecture/config/syntax.md — config parsing and loading
//
// Package config provides configuration parsing and diffing for ze.
package config

import (
	"reflect"
	"slices"
)

// ConfigDiff holds the internal difference between two config maps.
// Public plugin sections MUST project values and omit private intent paths.
type ConfigDiff struct {
	Added   map[string]any // Keys present in new but not old
	Removed map[string]any // Keys present in old but not new
	// Changed also carries private inactive intent at nonmaterialized
	// <real-leaf>/!inactive-members paths. Such intent MUST NOT enter Added or
	// Removed: config-owner startup and deletion consume those maps.
	Changed map[string]DiffPair
}

// DiffPair holds old and new values for a changed key.
type DiffPair struct {
	Old any `json:"old"`
	New any `json:"new"`
}

// DiffMaps computes a deep diff between two map[string]any.
// Nested maps are compared recursively with slash-separated key paths.
// Returns a ConfigDiff with Added, Removed, and Changed maps.
// Inactive running intent is attributed to its real leaf owner through private
// Changed paths, including beneath whole added/removed/type-replaced ancestors.
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
	diffInactiveMembers(old, new, prefix, diff)

	// Check for removed keys (in old but not new)
	for k, oldVal := range old {
		if IsInactiveMemberMetadata(k, oldVal) {
			continue
		}
		key := joinPath(prefix, k)
		newVal, exists := new[k]
		newIsOrdinary := exists && !IsInactiveMemberMetadata(k, newVal)
		if !newIsOrdinary {
			diff.Removed[key] = oldVal
			diffInactiveSubtree(oldVal, nil, key, diff)
		}
	}

	// Check for added and changed keys
	for k, newVal := range new {
		if IsInactiveMemberMetadata(k, newVal) {
			continue
		}
		key := joinPath(prefix, k)
		oldVal, exists := old[k]

		oldIsOrdinary := exists && !IsInactiveMemberMetadata(k, oldVal)
		if !oldIsOrdinary {
			diff.Added[key] = newVal
			diffInactiveSubtree(nil, newVal, key, diff)
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
			diffInactiveSubtree(oldVal, newVal, key, diff)
		case !deepEqual(oldVal, newVal):
			// Values differ
			diff.Changed[key] = DiffPair{Old: oldVal, New: newVal}
		}
	}
}

// diffInactiveMembers attributes private intent to its real leaf owner before
// reload participant selection. Additions and removals of intent are Changes,
// never public config additions/deletions. Both inputs are immutable projections.
func diffInactiveMembers(old, next map[string]any, prefix string, diff *ConfigDiff) {
	before, oldErr := runningMemberIntents(old)
	after, newErr := runningMemberIntents(next)
	validMetadata := oldErr == nil && newErr == nil
	if !validMetadata {
		// Keep malformed metadata visibly changed so the normal configuration
		// verifier rejects it instead of treating it as an unchanged candidate.
		diff.Changed[AppendPath(prefix, inactiveMembersKey)] = DiffPair{
			Old: old[inactiveMembersKey], New: next[inactiveMembersKey],
		}
		return
	}
	if len(before) == 0 {
		if len(after) == 0 {
			return
		}
	}
	prior := make(map[string]inactiveMemberIntent, len(before))
	current := make(map[string]inactiveMemberIntent, len(after))
	for _, intent := range before {
		prior[intent.leaf] = intent
	}
	for _, intent := range after {
		current[intent.leaf] = intent
	}
	for leaf, intent := range prior {
		replacement, present := current[leaf]
		if present {
			if slices.Equal(intent.members, replacement.members) {
				continue
			}
		}
		var replacementRecord any
		if present {
			replacementRecord = []any{replacement.record}
		}
		path := AppendPath(AppendPath(prefix, leaf), inactiveMembersKey)
		diff.Changed[path] = DiffPair{Old: []any{intent.record}, New: replacementRecord}
	}
	for leaf, intent := range current {
		if _, present := prior[leaf]; present {
			continue
		}
		path := AppendPath(AppendPath(prefix, leaf), inactiveMembersKey)
		diff.Changed[path] = DiffPair{New: []any{intent.record}}
	}
}

// diffInactiveSubtree visits metadata beneath an ancestor the ordinary diff
// records whole, so an all-inactive leaf absent from its active map still
// selects its own participant. This recursion is bounded by parsed config depth.
func diffInactiveSubtree(old, next any, prefix string, diff *ConfigDiff) {
	before, oldMap := old.(map[string]any)
	after, newMap := next.(map[string]any)
	if !oldMap {
		if !newMap {
			return
		}
	}
	diffInactiveMembers(before, after, prefix, diff)
	for name, value := range before {
		if IsInactiveMemberMetadata(name, value) {
			continue
		}
		diffInactiveSubtree(value, after[name], AppendPath(prefix, name), diff)
	}
	for name, value := range after {
		if _, present := before[name]; present {
			continue
		}
		if IsInactiveMemberMetadata(name, value) {
			continue
		}
		diffInactiveSubtree(nil, value, AppendPath(prefix, name), diff)
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
