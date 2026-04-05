// Design: docs/architecture/testing/ci-format.md — test runner framework

package runner

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
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
//	    "peer": {"address": "127.0.0.1", "remote": {"as": 65533}},
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
		return fmt.Errorf("JSON mismatch:\n%s", jsonFieldDiff(expectedNorm, actualNorm, ""))
	}

	return nil
}

// jsonFieldDiff produces a field-level diff between two normalized maps.
// Labels each difference as changed/added/removed with the field path.
func jsonFieldDiff(expected, actual map[string]any, prefix string) string {
	var diffs []string

	// Collect all keys from both maps
	allKeys := make(map[string]bool)
	for k := range expected {
		allKeys[k] = true
	}
	for k := range actual {
		allKeys[k] = true
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		path := k
		if prefix != "" {
			path = config.AppendPath(prefix, k)
		}

		expVal, inExp := expected[k]
		actVal, inAct := actual[k]

		switch {
		case !inExp && inAct:
			diffs = append(diffs, fmt.Sprintf("  added:   %s = %s", path, formatJSONValue(actVal)))
		case inExp && !inAct:
			diffs = append(diffs, fmt.Sprintf("  removed: %s = %s", path, formatJSONValue(expVal)))
		case !reflect.DeepEqual(expVal, actVal):
			// Both present but different — recurse into nested structures
			expMap, expIsMap := expVal.(map[string]any)
			actMap, actIsMap := actVal.(map[string]any)
			expSlice, expIsSlice := expVal.([]any)
			actSlice, actIsSlice := actVal.([]any)
			switch {
			case expIsMap && actIsMap:
				if nested := jsonFieldDiff(expMap, actMap, path); nested != "" {
					diffs = append(diffs, nested)
				}
			case expIsSlice && actIsSlice:
				if nested := jsonSliceDiff(expSlice, actSlice, path); nested != "" {
					diffs = append(diffs, nested)
				}
			default:
				diffs = append(diffs, fmt.Sprintf("  changed: %s = %s (expected %s)", path, formatJSONValue(actVal), formatJSONValue(expVal)))
			}
		}
	}

	return strings.Join(diffs, "\n")
}

// jsonSliceDiff produces element-level diff between two slices.
func jsonSliceDiff(expected, actual []any, path string) string {
	var diffs []string
	maxLen := max(len(expected), len(actual))
	for i := range maxLen {
		elemPath := fmt.Sprintf("%s[%d]", path, i)
		switch {
		case i >= len(expected):
			diffs = append(diffs, fmt.Sprintf("  added:   %s = %s", elemPath, formatJSONValue(actual[i])))
		case i >= len(actual):
			diffs = append(diffs, fmt.Sprintf("  removed: %s = %s", elemPath, formatJSONValue(expected[i])))
		case !reflect.DeepEqual(expected[i], actual[i]):
			expMap, expIsMap := expected[i].(map[string]any)
			actMap, actIsMap := actual[i].(map[string]any)
			expSlice, expIsSlice := expected[i].([]any)
			actSlice, actIsSlice := actual[i].([]any)
			switch {
			case expIsMap && actIsMap:
				if nested := jsonFieldDiff(expMap, actMap, elemPath); nested != "" {
					diffs = append(diffs, nested)
				}
			case expIsSlice && actIsSlice:
				if nested := jsonSliceDiff(expSlice, actSlice, elemPath); nested != "" {
					diffs = append(diffs, nested)
				}
			default:
				diffs = append(diffs, fmt.Sprintf("  changed: %s = %s (expected %s)", elemPath, formatJSONValue(actual[i]), formatJSONValue(expected[i])))
			}
		}
	}
	return strings.Join(diffs, "\n")
}

// formatJSONValue formats a value for display in diff output.
func formatJSONValue(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
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
