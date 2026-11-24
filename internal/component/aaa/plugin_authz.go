// Design: docs/architecture/core-design.md -- plugin command authorization seam
// Related: reserved.go -- ReservedInternalPluginPrefix and PluginEndpointFromIdentity
// Related: ../authz/authz.go -- Store.authorize evaluates PluginCommandProfile via Entry.matches
//
// Why this exists: plugin/server compiles each bus participant's effective cli:
// send rules, and authz evaluates authorization profiles. Neither package
// imports the other; both already import aaa. This file holds the value type
// and atomic lookup seam connecting the publisher (plugin/server) to the
// evaluator (authz) without introducing an import cycle.

package aaa

import (
	"strings"
	"sync/atomic"
)

// PluginCommandProfile describes the CLI command prefixes a bus participant
// (plugin endpoint) is permitted to dispatch. AllowedPrefixes holds normalized
// CLI command prefixes without the "cli:" namespace prefix (for example
// "show bgp peer" or "request peer"). AllowNone marks an explicit empty
// allow-list; both AllowNone and an empty AllowedPrefixes slice deny every
// command (fail closed).
type PluginCommandProfile struct {
	Endpoint        string
	AllowNone       bool
	AllowedPrefixes []string
}

// PluginCommandProfileLookup resolves the active command profile for a plugin
// endpoint name. Implementations MUST be safe for concurrent use.
type PluginCommandProfileLookup func(endpoint string) (PluginCommandProfile, bool)

var pluginCommandProfileLookup atomic.Pointer[PluginCommandProfileLookup]

// SetPluginCommandProfileLookup publishes the live plugin command profile
// lookup (called by plugin/server when compiled bus filters are updated).
// Passing nil clears the lookup so every plugin identity fails closed.
// Safe for concurrent use.
func SetPluginCommandProfileLookup(fn PluginCommandProfileLookup) {
	if fn == nil {
		pluginCommandProfileLookup.Store(nil)
		return
	}
	pluginCommandProfileLookup.Store(&fn)
}

// LookupPluginCommandProfile resolves the active command profile for endpoint.
// Returns (zero, false) when no lookup is registered or endpoint has no
// profile. Safe for concurrent use.
func LookupPluginCommandProfile(endpoint string) (PluginCommandProfile, bool) {
	fnPtr := pluginCommandProfileLookup.Load()
	if fnPtr == nil {
		return PluginCommandProfile{}, false
	}
	fn := *fnPtr
	if fn == nil {
		return PluginCommandProfile{}, false
	}
	return fn(endpoint)
}

// NewCommandPrefixAuthorizer compiles a word-boundary, case-insensitive CLI
// command prefix matcher over allowedPrefixes (each without the "cli:"
// namespace prefix). It matches a command when a normalized prefix equals the
// command or is followed by a single space, mirroring authz.Entry.matches.
// An empty or whitespace-only prefix list returns a matcher that denies every
// command. Safe for concurrent use.
func NewCommandPrefixAuthorizer(allowedPrefixes []string) func(command string) bool {
	normalized := make([]string, 0, len(allowedPrefixes))
	for _, raw := range allowedPrefixes {
		prefix := strings.ToLower(strings.TrimSpace(raw))
		if prefix == "" {
			continue
		}
		normalized = append(normalized, prefix)
	}
	return func(command string) bool {
		if len(normalized) == 0 {
			return false
		}
		lowerCmd := strings.ToLower(command)
		for _, prefix := range normalized {
			if !strings.HasPrefix(lowerCmd, prefix) {
				continue
			}
			if len(lowerCmd) == len(prefix) {
				return true
			}
			if lowerCmd[len(prefix)] == ' ' {
				return true
			}
		}
		return false
	}
}
