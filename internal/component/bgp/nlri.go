// Design: docs/architecture/plugin/rib-storage-design.md — NLRI value parsing
// Related: route.go — Route struct uses parsed NLRI values
// Related: event.go — event parsing and family operations
// Related: format.go — route command formatting
package bgp

// ParseNLRIValue extracts prefix and path-id from an NLRI value.
// Handles both new format {"prefix":"...", "path-id":N} and legacy string format.
func ParseNLRIValue(v any) (prefix string, pathID uint32) {
	switch val := v.(type) {
	case string:
		// Legacy string format: just the prefix.
		return val, 0
	case map[string]any:
		// New structured format: {"prefix":"...", "path-id":N}.
		if p, ok := val["prefix"].(string); ok {
			prefix = p
		}
		if pid, ok := val["path-id"].(float64); ok {
			pathID = uint32(pid)
		}
		return prefix, pathID
	default: // unrecognized type
		return "", 0
	}
}
