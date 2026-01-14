package functional

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// isSupportedFamily returns true if the family is supported for JSON validation.
// Phase 1 supports only IPv4/IPv6 unicast.
func isSupportedFamily(family string) bool {
	normalized := strings.ReplaceAll(strings.ToLower(family), " ", "/")
	return normalized == "ipv4/unicast" || normalized == "ipv6/unicast"
}

// extractFamily extracts the address family from a zebgp decode envelope.
// Returns empty string for EOR (empty update) messages.
func extractFamily(envelope map[string]any) string {
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

// transformEnvelopeToPlugin converts zebgp decode envelope format to plugin format.
// Returns the transformed map and the detected family.
//
// Zebgp decode format:
//
//	{
//	  "type": "update",
//	  "neighbor": {
//	    "message": {
//	      "update": {
//	        "attribute": {"origin": "igp", ...},
//	        "announce": {"ipv4/unicast": {"10.0.1.254": [{"nlri": "10.0.0.0/24"}]}}
//	      }
//	    }
//	  }
//	}
//
// Plugin format:
//
//	{
//	  "message": {"type": "update"},
//	  "origin": "igp",
//	  "ipv4/unicast": [{"next-hop": "10.0.1.254", "action": "add", "nlri": ["10.0.0.0/24"]}]
//	}
func transformEnvelopeToPlugin(envelope map[string]any) (map[string]any, string) {
	result := make(map[string]any)

	// Set meta section with version and format
	result["meta"] = map[string]any{
		"version": "1.0.0",
		"format":  "zebgp",
	}

	// Set message type
	if msgType, ok := envelope["type"].(string); ok {
		result["message"] = map[string]any{"type": msgType}
	}

	// Navigate to update content
	neighbor, ok := envelope["neighbor"].(map[string]any)
	if !ok {
		return result, ""
	}

	message, ok := neighbor["message"].(map[string]any)
	if !ok {
		return result, ""
	}

	update, ok := message["update"].(map[string]any)
	if !ok {
		return result, ""
	}

	// Copy attributes to top level
	if attrs, ok := update["attribute"].(map[string]any); ok {
		for k, v := range attrs {
			result[k] = v
		}
	}

	// Track detected family
	var detectedFamily string

	// Transform announce section
	if announce, ok := update["announce"].(map[string]any); ok {
		for family, nhMap := range announce {
			detectedFamily = family
			if nhData, ok := nhMap.(map[string]any); ok {
				result[family] = transformAnnounce(nhData)
			}
		}
	}

	// Transform withdraw section
	if withdraw, ok := update["withdraw"].(map[string]any); ok {
		for family, prefixes := range withdraw {
			detectedFamily = family
			result[family] = transformWithdraw(prefixes)
		}
	}

	return result, detectedFamily
}

// transformAnnounce transforms the announce section from zebgp decode to plugin format.
// Zebgp decode: {"next-hop": [{"nlri": "prefix"}]}.
// Plugin: [{"next-hop": "...", "action": "add", "nlri": ["prefix"]}].
func transformAnnounce(nhMap map[string]any) []map[string]any {
	var result []map[string]any

	for nextHop, nlriList := range nhMap {
		var nlris []string

		if v, ok := nlriList.([]any); ok {
			for _, item := range v {
				if nlriMap, ok := item.(map[string]any); ok {
					if nlri, ok := nlriMap["nlri"].(string); ok {
						nlris = append(nlris, nlri)
					}
				}
			}
		}

		if len(nlris) > 0 {
			result = append(result, map[string]any{
				"next-hop": nextHop,
				"action":   "add",
				"nlri":     nlris,
			})
		}
	}

	return result
}

// transformWithdraw transforms the withdraw section from zebgp decode to plugin format.
// Zebgp decode formats:
// - IPv4 unicast: ["prefix1", "prefix2"]
// - IPv6/MP: [{"nlri": "prefix1"}, {"nlri": "prefix2"}]
// Plugin: [{"action": "del", "nlri": ["prefix1", "prefix2"]}].
func transformWithdraw(prefixes any) []map[string]any {
	var nlris []string

	if v, ok := prefixes.([]any); ok {
		for _, p := range v {
			// Handle {"nlri": "prefix"} format (IPv6/MP withdraws)
			if nlriMap, ok := p.(map[string]any); ok {
				if nlri, ok := nlriMap["nlri"].(string); ok {
					nlris = append(nlris, nlri)
				}
				continue
			}
			// Handle "prefix" format (IPv4 unicast withdraws)
			if prefix, ok := p.(string); ok {
				nlris = append(nlris, prefix)
			}
		}
	}

	if len(nlris) == 0 {
		return nil
	}

	return []map[string]any{
		{
			"action": "del",
			"nlri":   nlris,
		},
	}
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
