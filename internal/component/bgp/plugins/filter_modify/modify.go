// Design: docs/architecture/core-design.md -- route attribute modifier
// Related: config.go -- modify definition config parsing
// Related: filter_modify.go -- SDK entry point and handleFilterUpdate
//
// The modifier builds a text delta containing only the declared attributes.
// The engine handles merging via applyFilterDelta (text-level overlay) and
// textDeltaToModOps -> buildModifiedPayload (wire-level rewriting).
//
// The plugin always returns "modify" action (it unconditionally sets the
// declared attributes). For conditional modification, compose with match
// filters earlier in the chain: "filter import prefix-list:X modify:Y".
package filter_modify

import (
	"fmt"
	"strconv"
	"strings"
)

// modifyDef is a named modifier definition loaded from config.
type modifyDef struct {
	name  string
	delta string // pre-built delta text: "local-preference 200 med 50"
}

// buildDelta constructs the text delta from the config leaves.
// Only present (non-nil) attributes are included. The delta format matches
// the filter text protocol so applyFilterDelta can overlay it directly.
func buildDelta(setBlock map[string]any) string {
	var parts []string

	if v, ok := readOptionalUint32(setBlock["local-preference"]); ok {
		parts = append(parts, fmt.Sprintf("local-preference %d", v))
	}
	if v, ok := readOptionalUint32(setBlock["med"]); ok {
		parts = append(parts, fmt.Sprintf("med %d", v))
	}
	if s, ok := setBlock["origin"].(string); ok && s != "" {
		parts = append(parts, fmt.Sprintf("origin %s", s))
	}
	if s, ok := setBlock["next-hop"].(string); ok && s != "" {
		parts = append(parts, fmt.Sprintf("next-hop %s", s))
	}
	if v, ok := readOptionalUint32(setBlock["as-path-prepend"]); ok && v >= 1 && v <= 32 {
		parts = append(parts, fmt.Sprintf("as-path-prepend %d", v))
	}

	return strings.Join(parts, " ")
}

// readOptionalUint32 coerces config values (float64, int, string) to uint32.
// Returns (0, false) if the value is nil or not a recognized numeric form.
func readOptionalUint32(v any) (uint32, bool) {
	if v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		if n < 0 || n > 4294967295 {
			return 0, false
		}
		return uint32(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return uint32(n), true //nolint:gosec // G115: bounds-checked above
	case int64:
		if n < 0 || n > 4294967295 {
			return 0, false
		}
		return uint32(n), true //nolint:gosec // G115: bounds-checked above
	case uint64:
		if n > 4294967295 {
			return 0, false
		}
		return uint32(n), true //nolint:gosec // G115: bounds-checked above
	case string:
		x, err := strconv.ParseUint(n, 10, 32)
		if err != nil {
			return 0, false
		}
		return uint32(x), true //nolint:gosec // G115: bounded by ParseUint 32-bit
	}
	return 0, false
}
