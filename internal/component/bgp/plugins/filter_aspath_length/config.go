// Design: docs/architecture/core-design.md -- AS-path length filter config parsing
// Related: filter_aspath_length.go -- SDK entry point and handleFilterUpdate
// Related: aspath_length.go -- path length evaluation and AS-path extraction

package filter_aspath_length

import (
	"fmt"
	"strconv"
)

const maxNameLen = 256

type asPathLengthDef struct {
	name string
	max  int // -1 means no max constraint
	min  int // -1 means no min constraint
}

func parseAsPathLengthDefs(bgpCfg map[string]any) (map[string]*asPathLengthDef, error) {
	result := make(map[string]*asPathLengthDef)

	policyBlock, ok := bgpCfg["policy"].(map[string]any)
	if !ok {
		return result, nil
	}
	block, ok := policyBlock["as-path-length"].(map[string]any)
	if !ok {
		return result, nil
	}

	for name, raw := range block {
		if len(name) > maxNameLen {
			return nil, fmt.Errorf("as-path-length name %q exceeds maximum length %d", name, maxNameLen)
		}
		defMap, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("as-path-length %q: not a map", name)
		}

		def := &asPathLengthDef{name: name, max: -1, min: -1}

		for key := range defMap {
			switch key {
			case "max", "min":
			default:
				return nil, fmt.Errorf("as-path-length %q: unknown key %q", name, key)
			}
		}

		if v, ok := readUint16(defMap["max"]); ok {
			def.max = int(v)
		}
		if v, ok := readUint16(defMap["min"]); ok {
			def.min = int(v)
		}

		if def.max < 0 && def.min < 0 {
			return nil, fmt.Errorf("as-path-length %q: at least one of 'max' or 'min' required", name)
		}
		if def.max >= 0 && def.min >= 0 && def.min > def.max {
			return nil, fmt.Errorf("as-path-length %q: min (%d) exceeds max (%d)", name, def.min, def.max)
		}

		result[name] = def
	}
	return result, nil
}

func readUint16(v any) (uint16, bool) {
	if v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		if n < 0 || n > 65535 {
			return 0, false
		}
		return uint16(n), true //nolint:gosec // G115: bounds-checked above
	case int:
		if n < 0 || n > 65535 {
			return 0, false
		}
		return uint16(n), true //nolint:gosec // G115: bounds-checked above
	case int64:
		if n < 0 || n > 65535 {
			return 0, false
		}
		return uint16(n), true //nolint:gosec // G115: bounds-checked above
	case uint64:
		if n > 65535 {
			return 0, false
		}
		return uint16(n), true //nolint:gosec // G115: bounds-checked above
	case string:
		x, err := strconv.ParseUint(n, 10, 16)
		if err != nil {
			return 0, false
		}
		return uint16(x), true //nolint:gosec // G115: bounded by ParseUint 16-bit
	}
	return 0, false
}
