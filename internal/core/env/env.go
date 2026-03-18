// Design: docs/architecture/config/environment.md — centralized env var lookup
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

// Get returns the value of a Ze environment variable.
// key is the canonical dot-notation form (e.g. "ze.plugin.hub.host").
// Matching is case-insensitive and treats dots and underscores as equivalent.
// Aborts if the key was not registered via MustRegister (programming error).
func Get(key string) string {
	mustBeRegistered(key)
	ensureCache()
	cacheMu.RLock()
	v := cache[normalize(key)]
	cacheMu.RUnlock()
	return v
}

// Set updates a Ze environment variable in both the cache and os environment.
// key is the canonical dot-notation form. The os env var is set using the
// dot-notation key so that child processes inherit a canonical form.
// Aborts if the key was not registered via MustRegister (programming error).
func Set(key, value string) error {
	mustBeRegistered(key)
	ensureCache()
	cacheMu.Lock()
	cache[normalize(key)] = value
	cacheMu.Unlock()
	return os.Setenv(key, value)
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
		os.Stderr.WriteString("FATAL: env.Get called with unregistered key: " + key + "\n") //nolint:errcheck // pre-exit
		os.Exit(2)
	}
}

// ResetCache clears the cache, forcing a rebuild from os.Environ() on next access.
// Intended for tests that manipulate env vars via os.Setenv directly.
func ResetCache() {
	cacheMu.Lock()
	cacheOnce = sync.Once{}
	cache = nil
	cacheMu.Unlock()
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

// GetBool returns a boolean env var value: "true"/"1" = true, "false"/"0" = false.
// Returns defaultVal if unset or unrecognized.
func GetBool(key string, defaultVal bool) bool {
	s := Get(key)
	if s == "" {
		return defaultVal
	}
	v := strings.ToLower(s)
	if v == "true" || v == "1" {
		return true
	}
	if v == "false" || v == "0" {
		return false
	}
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
