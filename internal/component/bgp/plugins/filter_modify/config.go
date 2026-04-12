// Design: docs/architecture/core-design.md -- route modify filter config parsing
// Related: modify.go -- delta building and attribute encoding
// Related: filter_modify.go -- SDK entry point and handleFilterUpdate
//
// Config parsing for the bgp-filter-modify plugin.
//
// Reads named modify definitions out of the BGP config subtree:
//
//	bgp { policy { modify NAME { set { local-preference 200; med 50; } } } }
//
// Each definition becomes a *modifyDef with a pre-built delta string.
// The delta is constructed at config load time so the hot path only
// returns the pre-built string.
package filter_modify

import "fmt"

const (
	// maxNameLen is the maximum allowed modifier name length.
	maxNameLen = 256
)

// parseModifyDefs walks bgp { policy { modify ... } } and returns a
// map of name -> *modifyDef ready for runtime use.
func parseModifyDefs(bgpCfg map[string]any) (map[string]*modifyDef, error) {
	result := make(map[string]*modifyDef)

	policyBlock, ok := bgpCfg["policy"].(map[string]any)
	if !ok {
		return result, nil
	}
	modBlock, ok := policyBlock["modify"].(map[string]any)
	if !ok {
		return result, nil
	}

	for name, raw := range modBlock {
		if len(name) > maxNameLen {
			return nil, fmt.Errorf("modify name %q exceeds maximum length %d", name, maxNameLen)
		}
		defMap, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("modify %q: not a map", name)
		}

		setBlock, ok := defMap["set"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("modify %q: missing 'set' container", name)
		}

		delta := buildDelta(setBlock)
		if delta == "" {
			return nil, fmt.Errorf("modify %q: 'set' container has no attributes", name)
		}

		result[name] = &modifyDef{name: name, delta: delta}
	}
	return result, nil
}
