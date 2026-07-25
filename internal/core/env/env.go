// Design: docs/architecture/config/environment.md — centralized env var lookup
// Detail: registry.go — env var registration and entry listing
//
// Package env provides centralized environment variable lookup for Ze.
//
// Lookup is case-insensitive and separator-agnostic. Any of these set the same var:
//   - ze.plugin.hub.host
//   - ze_plugin_hub_host
//   - ZE_PLUGIN_HUB_HOST
//   - ZE_plUGin_HUB_host
//
// All keys are normalized to lowercase underscores for matching.
// A cache is built from os.Environ() on first access and updated via Set().
// Callers always pass the dot-notation key.
//
// Example:
//
//	env.Get("ze.plugin.hub.host")  // finds any case/separator variant
package env

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// cache maps normalized keys (lowercase underscores) to values.
// Built lazily from os.Environ() on first access, updated by Set().
// Protected by cacheMu for concurrent Get/Set safety.
var (
	cache     map[string]string
	cacheOnce sync.Once
	cacheMu   sync.RWMutex
)

// normalize converts any key form to lowercase underscores for cache lookup.
func normalize(key string) string {
	return strings.ToLower(strings.ReplaceAll(key, ".", "_"))
}

// ensureCache populates the cache from os.Environ() on first call.
func ensureCache() {
	cacheOnce.Do(func() {
		cache = make(map[string]string)
		for _, entry := range os.Environ() {
			envKey, envVal, ok := strings.Cut(entry, "=")
			if !ok {
				continue
			}
			cache[normalize(envKey)] = envVal
		}
	})
}

// secretCleared tracks which Secret keys have already been cleared from OS env.
// Prevents repeated os.Unsetenv calls on subsequent Get() invocations.
// Protected by cacheMu.
var secretCleared = make(map[string]bool)

// Get returns the value of a Ze environment variable.
// key is the canonical dot-notation form (e.g. "ze.plugin.hub.host").
// Aliases registered via EnvEntry.Aliases resolve to the canonical key.
// Matching is case-insensitive and treats dots and underscores as equivalent.
// Aborts if the key was not registered via MustRegister (programming error).
// For vars registered with Secret: true, the first Get() clears the var from the
// OS environment (removes it from /proc/<pid>/environ). The value stays in cache.
func Get(key string) string {
	canonical := resolveAlias(key)
	mustBeRegistered(canonical)
	ensureCache()

	var v string

	// For secret vars, use write lock to protect both cache read and secretCleared.
	if IsSecret(canonical) {
		cacheMu.Lock()
		var ok bool
		v, ok = cache[normalize(canonical)]
		if !ok && key != canonical {
			v = cache[normalize(key)]
		}
		if !secretCleared[canonical] {
			clearSecretFromEnv(canonical)
			secretCleared[canonical] = true
		}
		cacheMu.Unlock()
	} else {
		cacheMu.RLock()
		var ok bool
		v, ok = cache[normalize(canonical)]
		if !ok && key != canonical {
			v = cache[normalize(key)]
		}
		cacheMu.RUnlock()
	}

	if v != "" {
		warnDeprecated(canonical)
	}
	return v
}

// Set updates a Ze environment variable in both the cache and os environment.
// key is the canonical dot-notation form. The os env var is set using the
// dot-notation key so that child processes inherit a canonical form.
// Aborts if the key was not registered via MustRegister (programming error).
func Set(key, value string) error {
	canonical := resolveAlias(key)
	mustBeRegistered(canonical)
	ensureCache()
	cacheMu.Lock()
	cache[normalize(canonical)] = value
	cacheMu.Unlock()
	return os.Setenv(canonical, value)
}

// SetInt sets an integer Ze environment variable.
func SetInt(key string, value int) error {
	return Set(key, strconv.Itoa(value))
}

// SetBool sets a boolean Ze environment variable ("true" or "false").
func SetBool(key string, value bool) error {
	return Set(key, strconv.FormatBool(value))
}

// mustBeRegistered aborts if key is not in the registry.
// This catches typos and ensures all env vars are documented.
func mustBeRegistered(key string) {
	if !IsRegistered(key) {
		// Unregistered env var is a programming error.
		// Use os.Stderr + os.Exit since this package cannot import slogutil (circular).
		var tb textbuf.Buffer
		os.Stderr.WriteString(tb.Str("FATAL: env.Get called with unregistered key: ").Str(key).Byte('\n').Slice()) //nolint:errcheck // pre-exit
		os.Exit(2)
	}
}

// clearSecretFromEnv removes all OS environment forms of a dot-notation key.
// The value remains in the in-process cache for subsequent Get() calls.
func clearSecretFromEnv(dotKey string) {
	norms := map[string]bool{normalize(dotKey): true}
	if e, ok := registered[dotKey]; ok {
		for _, alias := range e.Aliases {
			norms[normalize(alias)] = true
		}
	}
	for _, entry := range os.Environ() {
		envKey, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if norms[normalize(envKey)] {
			_ = os.Unsetenv(envKey) //nolint:errcheck // best-effort cleanup
		}
	}
}

// ResetCache clears the cache, forcing a rebuild from os.Environ() on next access.
// Intended for tests that manipulate env vars via os.Setenv directly.
// Also resets the secret-cleared tracking so secrets are re-cleared on next Get().
func ResetCache() {
	cacheMu.Lock()
	cacheOnce = sync.Once{}
	cache = nil
	secretCleared = make(map[string]bool)
	cacheMu.Unlock()

	deprecatedWarnedMu.Lock()
	deprecatedWarned = make(map[string]bool)
	deprecatedWarnedMu.Unlock()
}

// GetInt returns an integer env var value, or defaultVal if unset or invalid.
func GetInt(key string, defaultVal int) int {
	s := Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

// GetInt64 returns an int64 env var value, or defaultVal if unset or invalid.
func GetInt64(key string, defaultVal int64) int64 {
	s := Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return defaultVal
	}
	return v
}

// GetBool returns a boolean env var value.
// True values: "true", "1", "yes", "on", "enable", "enabled" (case-insensitive).
// False values: "false", "0", "no", "off", "disable", "disabled" (case-insensitive).
// Returns defaultVal if unset, empty, or not one of the recognized values above.
// Unrecognized non-empty values are logged to stderr as a diagnostic since the env
// package cannot import slog (circular dependency).
func GetBool(key string, defaultVal bool) bool {
	s := Get(key)
	if s == "" {
		return defaultVal
	}
	v := strings.ToLower(s)
	if v == "true" || v == "1" || v == "yes" || v == "on" || v == "enable" || v == "enabled" {
		return true
	}
	if v == "false" || v == "0" || v == "no" || v == "off" || v == "disable" || v == "disabled" {
		return false
	}
	// Unrecognized value: warn and fall back to default.
	var tb textbuf.Buffer
	os.Stderr.WriteString(tb.Str("WARNING: env var ").Str(key).Str(" has unrecognized boolean value ").Str(s).Str(", using default\n").Slice()) //nolint:errcheck // pre-exit diagnostic
	return defaultVal
}

// IsEnabled returns true if the env var is set to an enabling value:
// "1", "true", "yes", "on", "enable", "enabled" (case-insensitive).
// Returns false if unset, empty, or any other value.
func IsEnabled(key string) bool {
	v := strings.ToLower(Get(key))
	return v == "1" || v == "true" || v == "yes" || v == "on" || v == "enable" || v == "enabled"
}

// GetDuration returns a duration env var value, or defaultVal if unset or invalid.
func GetDuration(key string, defaultVal time.Duration) time.Duration {
	s := Get(key)
	if s == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultVal
	}
	return d
}
