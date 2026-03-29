// Design: docs/architecture/zefs-format.md -- ZeFS key registry for centralized key definitions
// Overview: store.go -- BlobStore uses these keys for data storage
// Related: keys.go -- concrete key registrations

package zefs

import (
	"sort"
	"strings"
)

// KeyEntry describes a registered ZeFS key for documentation and validation.
type KeyEntry struct {
	Pattern     string // Key pattern (e.g. "meta/ssh/username" or "meta/history/{username}/{mode}")
	Description string // One-line description
	Private     bool   // If true, hidden from "ze data registered" listing
}

// Key returns the concrete key by substituting {placeholder} segments in order.
// For fixed keys (no placeholders), returns the Pattern unchanged.
// Panics if the number of params does not match the number of placeholders,
// or if any param is empty or contains "..". These are programming errors.
func (e KeyEntry) Key(params ...string) string {
	count := strings.Count(e.Pattern, "{")
	if count == 0 {
		if len(params) != 0 {
			panic("BUG: zefs.KeyEntry.Key called with params on fixed key " + e.Pattern)
		}
		return e.Pattern
	}
	if len(params) != count {
		panic("BUG: zefs.KeyEntry.Key param count mismatch for " + e.Pattern)
	}

	result := e.Pattern
	for _, p := range params {
		if p == "" {
			panic("BUG: zefs.KeyEntry.Key called with empty param for " + e.Pattern)
		}
		if strings.Contains(p, "..") {
			panic("BUG: zefs.KeyEntry.Key param contains '..' for " + e.Pattern)
		}
		start := strings.Index(result, "{")
		end := strings.Index(result, "}")
		result = result[:start] + p + result[end+1:]
	}
	return result
}

// Prefix returns the prefix for matching concrete keys against this pattern.
// For templates (containing {placeholder}), returns everything before the first "{".
// For fixed keys, returns Pattern + "/".
func (e KeyEntry) Prefix() string {
	if idx := strings.Index(e.Pattern, "{"); idx > 0 {
		return e.Pattern[:idx]
	}
	return e.Pattern + "/"
}

// Dir returns Prefix() without the trailing "/".
func (e KeyEntry) Dir() string {
	return strings.TrimRight(e.Prefix(), "/")
}

// registered holds all known ZeFS key entries.
// Written only during package-level var initialization (single-threaded).
// All subsequent access is read-only. No mutex needed.
var registered []KeyEntry

// keyPrefixes holds prefix patterns from template entries.
var keyPrefixes []string

// MustRegister adds a key entry to the registry.
// Called via package-level var initialization. Not safe for concurrent use.
func MustRegister(e KeyEntry) KeyEntry {
	if e.Pattern == "" {
		panic("BUG: zefs.MustRegister called with empty Pattern")
	}
	for _, existing := range registered {
		if existing.Pattern == e.Pattern {
			panic("BUG: zefs.MustRegister duplicate pattern: " + e.Pattern)
		}
	}

	registered = append(registered, e)

	if idx := strings.Index(e.Pattern, "{"); idx > 0 {
		keyPrefixes = append(keyPrefixes, e.Pattern[:idx])
	}

	return e
}

// Entries returns public registered key entries sorted by Pattern.
// Private entries are excluded from listing.
func Entries() []KeyEntry {
	result := make([]KeyEntry, 0, len(registered))
	for _, e := range registered {
		if !e.Private {
			result = append(result, e)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Pattern < result[j].Pattern
	})
	return result
}

// AllEntries returns all registered key entries including private ones, sorted by Pattern.
func AllEntries() []KeyEntry {
	result := make([]KeyEntry, len(registered))
	copy(result, registered)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Pattern < result[j].Pattern
	})
	return result
}

// IsRegistered returns true if the key matches a registered entry exactly
// or matches a template prefix pattern.
func IsRegistered(key string) bool {
	for _, e := range registered {
		if e.Pattern == key {
			return true
		}
	}
	for _, p := range keyPrefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}
