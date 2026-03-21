// Design: (none -- predates documentation)
// Overview: decorator.go -- plugin entry point using merge for event emission.

package rpki_decorator

import "encoding/json"

// mergeUpdateRPKI merges an UPDATE event (primary) with an RPKI validation event (secondary).
// The result is a single JSON event with the UPDATE data and the rpki section injected.
// If secondary is empty (timeout), the result is the UPDATE with message type changed to "update-rpki".
// If primary is unparseable or structurally invalid, returns empty string.
// If secondary is unparseable, returns UPDATE without rpki section.
//
// Note: JSON round-trip through map[string]any reorders keys and converts integers to float64.
// Message IDs up to 2^53 are preserved exactly. This is safe since ze's msgID counter would
// need ~285 years at 1M msg/sec to reach that limit.
func mergeUpdateRPKI(primary, secondary string) string {
	var priMap map[string]any
	if err := json.Unmarshal([]byte(primary), &priMap); err != nil {
		return ""
	}

	bgp, ok := priMap["bgp"].(map[string]any)
	if !ok {
		return ""
	}

	// Change message type to "update-rpki". If message section is missing or
	// malformed, the event is structurally invalid -- return empty.
	msg, ok := bgp["message"].(map[string]any)
	if !ok {
		return ""
	}
	msg["type"] = "update-rpki"

	// Inject rpki section from secondary (if available and parseable).
	if secondary != "" {
		var secMap map[string]any
		if err := json.Unmarshal([]byte(secondary), &secMap); err == nil {
			if secBGP, ok := secMap["bgp"].(map[string]any); ok {
				if rpki, ok := secBGP["rpki"]; ok {
					bgp["rpki"] = rpki
				}
			}
		}
	}

	data, err := json.Marshal(priMap)
	if err != nil {
		return ""
	}
	return string(data)
}
