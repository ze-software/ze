// Design: docs/architecture/core-design.md -- route attribute modifier
// Related: config.go -- modify definition config parsing
// Related: filter_modify.go -- SDK entry point and handleFilterUpdate
//
// The modifier builds a text delta containing only the declared attributes.
// The engine handles merging via applyFilterDelta (text-level overlay) and
// textDeltaToModOps -> buildModifiedPayload (wire-level rewriting).
//
// Static operations (set) produce a pre-built delta at config time.
// Dynamic operations (increment, decrement) read the current value from
// the update text at runtime and compute the new absolute value.
// Community operations (community-add, community-remove) emit dedicated
// text directives that textDeltaToModOps maps to AttrModAdd/AttrModRemove.
//
// The plugin always returns "modify" action (it unconditionally applies the
// declared operations). For conditional modification, compose with match
// filters earlier in the chain: "filter import prefix-list:X modify:Y".
package filter_modify

import (
	"math"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// modifyDef is a named modifier definition loaded from config.
type modifyDef struct {
	name       string
	match      matchCond // when stated, the condition a route meets to be modified
	delta      string    // pre-built delta for static set operations
	increments []incdec  // runtime: increment operations
	decrements []incdec  // runtime: decrement operations
	commOps    []commOp  // runtime: community add/remove directives
}

// isDynamic returns true if this modifier needs the update text at runtime.
func (d *modifyDef) isDynamic() bool {
	return len(d.increments) > 0 || len(d.decrements) > 0 || len(d.commOps) > 0
}

type incdec struct {
	attr  string // "local-preference", "med", "aigp"
	value uint32
}

type commOp struct {
	directive string // text directive: "community-add", "large-community-remove", etc.
	values    string // space-separated values: "65000:100 65000:200"
}

// buildDelta constructs the static text delta from the set block leaves.
func buildDelta(setBlock map[string]any) string {
	var parts []string

	if v, ok := readOptionalUint32(setBlock["local-preference"]); ok {
		parts = append(parts, textbuf.StrInt("local-preference ", int64(v)))
	}
	if v, ok := readOptionalUint32(setBlock["med"]); ok {
		parts = append(parts, textbuf.StrInt("med ", int64(v)))
	}
	if s, ok := setBlock["origin"].(string); ok && s != "" {
		parts = append(parts, "origin "+s)
	}
	if s, ok := setBlock["next-hop"].(string); ok && s != "" {
		parts = append(parts, "next-hop "+s)
	}
	if v, ok := readOptionalUint32(setBlock["as-path-prepend"]); ok && v >= 1 && v <= 32 {
		parts = append(parts, textbuf.StrInt("as-path-prepend ", int64(v)))
	}

	return textbuf.Join(parts, " ")
}

// buildDynamicDelta computes the full delta text at runtime, combining the
// static delta with dynamically computed inc/dec values and community ops.
func buildDynamicDelta(def *modifyDef, updateText string) string {
	buf := textbuf.Get()
	defer buf.Release()

	if def.delta != "" {
		buf.Str(def.delta)
	}

	for i := range def.increments {
		op := &def.increments[i]
		current := extractUint32Attr(updateText, op.attr)
		sum := min(uint64(current)+uint64(op.value), math.MaxUint32)
		if buf.Len() > 0 {
			buf.Byte(' ')
		}
		buf.Str(op.attr).Byte(' ').Int(int64(sum))
	}

	for i := range def.decrements {
		op := &def.decrements[i]
		current := extractUint32Attr(updateText, op.attr)
		if buf.Len() > 0 {
			buf.Byte(' ')
		}
		if current <= op.value {
			buf.Str(op.attr).Str(" 0")
		} else {
			buf.Str(op.attr).Byte(' ').Int(int64(current - op.value))
		}
	}

	for i := range def.commOps {
		cop := &def.commOps[i]
		if buf.Len() > 0 {
			buf.Byte(' ')
		}
		buf.Str(cop.directive).Byte(' ').Str(cop.values)
	}

	return buf.String()
}

// extractUint32Attr extracts a uint32 attribute value from filter text.
// Returns 0 if the attribute is not found or cannot be parsed.
func extractUint32Attr(updateText, attrName string) uint32 {
	prefix := attrName + " "
	_, rest, ok := strings.Cut(updateText, prefix)
	if !ok {
		return 0
	}
	token, _, _ := strings.Cut(rest, " ")
	v, err := strconv.ParseUint(token, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(v) //nolint:gosec // G115: bounded by ParseUint 32-bit
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
