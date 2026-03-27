// Design: docs/architecture/config/environment.md — env var registry for CLI help
// Overview: env.go — centralized env var lookup

package env

import "strings"

// EnvEntry describes a Ze environment variable for documentation/help output.
type EnvEntry struct {
	Key         string // Canonical dot-notation key (e.g. "ze.plugin.hub.host")
	Type        string // "string", "int", "bool", "duration"
	Default     string // Default value ("" if required or no default)
	Description string // One-line description
	Private     bool   // If true, hidden from "ze env list" and autocomplete
}

// registered holds all known env var keys.
var registered = make(map[string]EnvEntry)

// prefixes holds prefix patterns from entries like "ze.log.<subsystem>".
// "ze.log.<subsystem>" -> prefix "ze.log." matches any key starting with "ze.log.".
var prefixes []string

// MustRegister adds an env var entry to the registry.
// Called via package-level var initialization in each component.
func MustRegister(e EnvEntry) EnvEntry {
	registered[e.Key] = e

	// If key contains angle brackets, extract the prefix for pattern matching.
	if idx := strings.Index(e.Key, "<"); idx > 0 {
		prefixes = append(prefixes, e.Key[:idx])
	}

	return e
}

// IsRegistered returns true if the key matches a registered entry or prefix pattern.
func IsRegistered(key string) bool {
	if _, ok := registered[key]; ok {
		return true
	}
	for _, p := range prefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

// Entries returns public registered env var entries (unordered).
// Private entries are excluded from listing and autocomplete.
func Entries() []EnvEntry {
	result := make([]EnvEntry, 0, len(registered))
	for _, e := range registered {
		if !e.Private {
			result = append(result, e)
		}
	}
	return result
}

// AllEntries returns all registered env var entries including private ones (unordered).
func AllEntries() []EnvEntry {
	result := make([]EnvEntry, 0, len(registered))
	for _, e := range registered {
		result = append(result, e)
	}
	return result
}
