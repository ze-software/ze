// Design: docs/architecture/hub-architecture.md — hub coordination

package hub

import (
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
