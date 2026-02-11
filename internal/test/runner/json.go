package runner

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"sort"
	"strings"
)

// JSON envelope type constant.
const typeBGP = "bgp"

// isSupportedFamily returns true if the family is supported for JSON validation.
// Supports IPv4/IPv6 unicast and FlowSpec families.
func isSupportedFamily(family string) bool {
	normalized := strings.ReplaceAll(strings.ToLower(family), " ", "/")
	switch normalized {
	case "ipv4/unicast", "ipv6/unicast", "ipv4/flow", "ipv6/flow":
		return true
	default:
		return false
	}
}

// extractFamily extracts the address family from a ze bgp decode envelope.
// Returns empty string for EOR (empty update) messages.
func extractFamily(envelope map[string]any) string {
	// Handle new ze-bgp JSON format
	if envelope["type"] == typeBGP {
		bgp, ok := envelope["bgp"].(map[string]any)
		if !ok {
			return ""
		}
		update, ok := bgp["update"].(map[string]any)
		if !ok {
			return ""
		}
		// Family keys are directly in update (e.g., "ipv4/unicast")
		for key := range update {
			if key != "attr" && strings.Contains(key, "/") {
				return key
			}
		}
		return ""
	}

	// Legacy format fallback
	neighbor, ok := envelope["neighbor"].(map[string]any)
	if !ok {
		return ""
	}

	message, ok := neighbor["message"].(map[string]any)
	if !ok {
		return ""
	}

	update, ok := message["update"].(map[string]any)
	if !ok {
		return ""
	}

	// Check announce section first
	if announce, ok := update["announce"].(map[string]any); ok {
		for family := range announce {
			return family
		}
	}

	// Check withdraw section
	if withdraw, ok := update["withdraw"].(map[string]any); ok {
		for family := range withdraw {
			return family
		}
	}

	return ""
}

// transformEnvelopeToPlugin converts ze bgp decode envelope format to plugin format.
// Returns the transformed map and the detected family.
//
// Ze BGP decode format (ze-bgp JSON):
//
//	{
//	  "type": "bgp",
//	  "bgp": {
//	    "message": {"type": "update", "id": 0, "direction": "received"},
//	    "peer": {"address": "127.0.0.1", "asn": 65533},
//	    "update": {
//	      "attr": {"origin": "igp", "as-path": [65533]},
//	      "ipv4/unicast": [{"next-hop": "10.0.1.254", "action": "add", "nlri": ["10.0.0.0/24"]}]
//	    }
//	  }
//	}
//
// Plugin format (for test comparison):
//
//	{
//	  "meta": {"version": "1.0.0", "format": "ze-bgp"},
//	  "message": {"type": "update"},
//	  "origin": "igp",
//	  "ipv4/unicast": [{"next-hop": "10.0.1.254", "action": "add", "nlri": ["10.0.0.0/24"]}]
//	}
func transformEnvelopeToPlugin(envelope map[string]any) (map[string]any, string) {
	result := make(map[string]any)

	// Set meta section with version and format
	result["meta"] = map[string]any{
		"version": "1.0.0",
		"format":  "ze-bgp",
	}

	// Check for ze-bgp JSON format (has "bgp" wrapper with type="bgp")
	if envelope["type"] == typeBGP {
		return transformZeBGPFormat(envelope, result)
	}

	// Legacy format fallback (shouldn't happen with new decode)
	if msgType, ok := envelope["type"].(string); ok {
		result["message"] = map[string]any{"type": msgType}
	}

	return result, ""
}

// transformZeBGPFormat handles the ze-bgp JSON format.
func transformZeBGPFormat(envelope, result map[string]any) (map[string]any, string) {
	bgp, ok := envelope["bgp"].(map[string]any)
	if !ok {
		return result, ""
	}

	// Extract message type from bgp.message.type
	if msg, ok := bgp["message"].(map[string]any); ok {
		if msgType, ok := msg["type"].(string); ok {
			result["message"] = map[string]any{"type": msgType}
		}
	}

	// Get update container
	update, ok := bgp["update"].(map[string]any)
	if !ok {
		return result, ""
	}

	// Copy attributes to top level (from update.attr)
	if attrs, ok := update["attr"].(map[string]any); ok {
		maps.Copy(result, attrs)
	}

	// Track detected family
	var detectedFamily string

	// Copy NLRI operations directly (they're already in plugin format)
	// The ze-bgp format uses family keys directly in update: "ipv4/unicast", "ipv6/unicast", etc.
	for key, value := range update {
		if key == "attr" {
			continue // Already handled
		}
		// Check if this looks like a family key (contains /)
		if strings.Contains(key, "/") {
			detectedFamily = key
			result[key] = value
		}
	}

	return result, detectedFamily
}

// comparePluginJSON compares actual transformed JSON with expected JSON string.
// Ignores context-dependent fields: peer, direction.
func comparePluginJSON(actual map[string]any, expected string) error {
	var expectedMap map[string]any
	if err := json.Unmarshal([]byte(expected), &expectedMap); err != nil {
		return fmt.Errorf("invalid expected JSON: %w", err)
	}

	// Remove context-dependent fields from both
	contextFields := []string{"direction", "peer"}
	for _, f := range contextFields {
		delete(actual, f)
		delete(expectedMap, f)
	}

	// Normalize both for comparison
	actualNorm := normalizeForComparison(actual)
	expectedNorm := normalizeForComparison(expectedMap)

	if !reflect.DeepEqual(actualNorm, expectedNorm) {
		actualJSON, _ := json.MarshalIndent(actualNorm, "", "  ")
		expectedJSON, _ := json.MarshalIndent(expectedNorm, "", "  ")
		return fmt.Errorf("JSON mismatch:\nExpected:\n%s\nActual:\n%s", expectedJSON, actualJSON)
	}

	return nil
}

// normalizeForComparison normalizes a map for deep comparison.
// Converts all numeric types to float64 and sorts arrays for consistent comparison.
func normalizeForComparison(m map[string]any) map[string]any {
	result := make(map[string]any)

	for k, v := range m {
		result[k] = normalizeValue(v)
	}

	return result
}

// normalizeValue normalizes a value for comparison.
func normalizeValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return normalizeForComparison(val)
	case []any:
		return normalizeSlice(val)
	case []map[string]any:
		result := make([]any, len(val))
		for i, m := range val {
			result[i] = normalizeForComparison(m)
		}
		return sortSliceOfMaps(result)
	case []string:
		sorted := make([]string, len(val))
		copy(sorted, val)
		sort.Strings(sorted)
		result := make([]any, len(sorted))
		for i, s := range sorted {
			result[i] = s
		}
		return result
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case float32:
		return float64(val)
	default:
		return v
	}
}

// normalizeSlice normalizes a slice for comparison.
func normalizeSlice(s []any) []any {
	result := make([]any, len(s))
	for i, v := range s {
		result[i] = normalizeValue(v)
	}
	return sortSliceOfMaps(result)
}

// sortSliceOfMaps sorts a slice of maps by their JSON representation.
func sortSliceOfMaps(s []any) []any {
	type sortItem struct {
		key string
		val any
	}

	items := make([]sortItem, len(s))
	for i, v := range s {
		jsonBytes, _ := json.Marshal(v)
		items[i] = sortItem{key: string(jsonBytes), val: v}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].key < items[j].key
	})

	result := make([]any, len(items))
	for i, item := range items {
		result[i] = item.val
	}
	return result
}
