// Package ciformat provides shared utilities for parsing .ci test files.
package ci

import "strings"

// ParseKVPairs parses key=value pairs from colon-separated parts.
// Special handling for known keys that may contain colons in values (json, text, hex).
func ParseKVPairs(parts []string) map[string]string {
	kv := make(map[string]string)

	// Rejoin parts to handle values containing colons
	joined := strings.Join(parts, ":")

	// Known keys that may have complex values containing colons
	complexKeys := []string{"json=", "text=", "hex=", "pattern="}

	for _, ck := range complexKeys {
		if idx := strings.Index(joined, ck); idx != -1 {
			key := ck[:len(ck)-1] // Remove trailing =
			value := joined[idx+len(ck):]
			kv[key] = value
			// Remove this from joined for further parsing
			joined = joined[:idx]
			break
		}
	}

	// Parse remaining simple key=value pairs
	for part := range strings.SplitSeq(joined, ":") {
		if part == "" {
			continue
		}
		if before, after, ok := strings.Cut(part, "="); ok {
			key := before
			value := after
			kv[key] = value
		}
	}
	return kv
}
