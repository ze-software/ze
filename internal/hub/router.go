// Design: docs/architecture/hub-architecture.md — hub coordination

package hub

import (
	"os"
	"path/filepath"
	"strings"
)

// matchEventPattern checks if an event matches a subscription pattern.
// Pattern supports wildcards: "bgp.*" matches "bgp.peer", "bgp.peer.*" matches "bgp.peer.up".
func matchEventPattern(pattern, event string) bool {
	// Exact match
	if pattern == event {
		return true
	}

	// Wildcard matching
	if before, ok := strings.CutSuffix(pattern, ".*"); ok {
		prefix := before
		// Pattern "bgp.*" matches "bgp.peer" but not "bgp" exactly
		if strings.HasPrefix(event, prefix+".") || event == prefix {
			return true
		}
	}

	// Full wildcard
	if pattern == "*" {
		return true
	}

	return false
}

// resolveSocketPath returns the socket path to use.
// Precedence:
// 1. Explicit path from config (if non-empty)
// 2. $XDG_RUNTIME_DIR/ze/api.sock
// 3. $HOME/.ze/api.sock
// 4. /var/run/ze/api.sock.
func resolveSocketPath(configPath string) string {
	if configPath != "" {
		return configPath
	}

	// Try XDG_RUNTIME_DIR first
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return filepath.Join(xdg, "ze", "api.sock")
	}

	// Try HOME
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".ze", "api.sock")
	}

	// Fallback to system path
	return "/var/run/ze/api.sock"
}
