// Design: docs/architecture/config/environment.md — env var registry for CLI help
// Overview: env.go — centralized env var lookup

package env

import (
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// EnvEntry describes a Ze environment variable for documentation/help output.
type EnvEntry struct {
	Key         string   // Canonical dot-notation key (e.g. "ze.plugin.hub.host")
	Type        string   // "string", "int", "bool", "duration"
	Default     string   // Default value ("" if required or no default)
	Description string   // One-line description
	Private     bool     // If true, hidden from "ze env list" and autocomplete
	Secret      bool     // If true, cleared from OS env after first Get() (value stays in cache)
	Aliases     []string // Alternative keys that resolve to the same entry (e.g. YANG-aligned names)
	Deprecated  string   // If non-empty, warn on first use and suggest this replacement key
}

// registered holds all known env var keys.
var registered = make(map[string]EnvEntry)

// aliases maps alternative keys to the canonical key they resolve to.
var aliases = make(map[string]string)

// prefixes holds prefix patterns from entries like "ze.log.<subsystem>".
// "ze.log.<subsystem>" -> prefix "ze.log." matches any key starting with "ze.log.".
var prefixes []string

var (
	deprecatedWarned   = make(map[string]bool)
	deprecatedWarnedMu sync.Mutex
)

// warnDeprecated prints a one-time warning to stderr if key is marked Deprecated.
func warnDeprecated(key string) {
	e, ok := registered[key]
	if !ok || e.Deprecated == "" {
		return
	}
	deprecatedWarnedMu.Lock()
	defer deprecatedWarnedMu.Unlock()
	if deprecatedWarned[key] {
		return
	}
	deprecatedWarned[key] = true
	var tb textbuf.Buffer
	tb.Str("WARNING: env var ").Str(key).Str(" is deprecated, use ").Str(e.Deprecated).Str(" instead\n").StdErr() //nolint:errcheck // diagnostic
}

// MustRegister adds an env var entry to the registry.
// Called via package-level var initialization in each component.
func MustRegister(e EnvEntry) EnvEntry {
	registered[e.Key] = e

	for _, alias := range e.Aliases {
		if _, exists := registered[alias]; exists {
			panic("BUG: env alias collides with registered key")
		}
		if prev, exists := aliases[alias]; exists && prev != e.Key {
			panic("BUG: env alias already registered for another key")
		}
		aliases[alias] = e.Key
	}

	// If key contains angle brackets, extract the prefix for pattern matching.
	if idx := strings.Index(e.Key, "<"); idx > 0 {
		prefixes = append(prefixes, e.Key[:idx])
	}

	return e
}

// resolveAlias returns the canonical key if key is an alias, otherwise returns key unchanged.
func resolveAlias(key string) string {
	if canonical, ok := aliases[key]; ok {
		return canonical
	}
	return key
}

// IsRegistered returns true if the key matches a registered entry, alias, or prefix pattern.
func IsRegistered(key string) bool {
	if _, ok := registered[key]; ok {
		return true
	}
	if _, ok := aliases[key]; ok {
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

// IsSecret returns true if the key is registered with Secret: true.
func IsSecret(key string) bool {
	e, ok := registered[key]
	return ok && e.Secret
}

// AllEntries returns all registered env var entries including private ones (unordered).
func AllEntries() []EnvEntry {
	result := make([]EnvEntry, 0, len(registered))
	for _, e := range registered {
		result = append(result, e)
	}
	return result
}
