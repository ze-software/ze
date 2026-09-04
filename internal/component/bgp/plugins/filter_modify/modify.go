// Design: docs/architecture/core-design.md -- route attribute modifier
// Related: config.go -- modify definition config parsing
// Related: filter_modify.go -- SDK entry point and handleFilterUpdate
// RFC: rfc/short/rfc4271.md -- Sections 9.1.1 and 9.1.2.2 decide the absent-attribute bases
// RFC: rfc/short/rfc7311.md -- Sections 3.4.1 and 4.1 are why aigp gets no base
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
// The plugin returns "modify" for a route that meets the definition's match
// condition, and "accept" for one that does not. See filter_modify.go,
// handleFilterUpdate. An earlier match filter in the chain stays available:
// "filter import prefix-list:X modify:Y".
package filter_modify

import (
	"math"
	"strconv"
	"strings"
	"sync"

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
	// medRemove emits the med-remove directive, which is RFC 4271 Section
	// 5.1.4's configured MULTI_EXIT_DISC removal. It is not part of delta
	// because the engine honors it on an import chain only, and the direction
	// is known per call rather than at config time (handleFilterUpdate).
	medRemove bool
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

	if v, ok := readOptionalUint32(setBlock[localPreferenceAttr]); ok {
		parts = append(parts, textbuf.StrInt(localPreferenceAttr+" ", int64(v)))
	}
	if v, ok := readOptionalUint32(setBlock[medAttr]); ok {
		parts = append(parts, textbuf.StrInt(medAttr+" ", int64(v)))
	}
	if s, ok := setBlock[originAttr].(string); ok && s != "" {
		parts = append(parts, originAttr+" "+s)
	}
	if s, ok := setBlock[nextHopAttr].(string); ok && s != "" {
		parts = append(parts, nextHopAttr+" "+s)
	}
	if v, ok := readOptionalUint32(setBlock[asPathPrependAttr]); ok && v >= 1 && v <= 32 {
		parts = append(parts, textbuf.StrInt(asPathPrependAttr+" ", int64(v)))
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
		current, run := currentForArithmetic(updateText, op.attr)
		if !run {
			continue
		}
		sum := min(uint64(current)+uint64(op.value), math.MaxUint32)
		if buf.Len() > 0 {
			buf.Byte(' ')
		}
		buf.Str(op.attr).Byte(' ').Int(int64(sum))
	}

	for i := range def.decrements {
		op := &def.decrements[i]
		current, run := currentForArithmetic(updateText, op.attr)
		if !run {
			continue
		}
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

// attrReading says what the subject holds for one attribute name. A single
// uint32 cannot: a returned 0 would mean "the route carries 0", "the route
// carries nothing" and "the value did not parse" at once, and the three want
// different answers (ai/rules/principles.md).
type attrReading uint8

const (
	attrAbsent     attrReading = iota // the subject does not name the attribute
	attrPresent                       // named, and the value parsed as a uint32
	attrUnreadable                    // named, but the value did not parse
)

// absentBaseFor returns the value the arithmetic starts from when the route
// carries no such attribute, and whether one is declared for it. An attribute
// with NO ENTRY has no base, and its arithmetic is refused rather than computed
// from a zero nobody chose.
//
// The values are the operator's, and the numbers behind them are declared in
// ze-bgp-conf.yang at bgp/defaults/attribute. Each leaf's ze:help carries the
// RFC reasoning for its own number, and the container's description says why
// aigp has no leaf at all. Nothing here restates those sentences
// (ai/rules/principles.md).
//
// Before OnConfigure has delivered a config section, the answer is the schema's
// own defaults. declaredAbsentBase reads the same container with no operator
// value over it, so a route handled in that window computes from the declared
// number rather than from a zero nobody chose.
func absentBaseFor(attrName string) (uint32, bool) {
	base := absentBase.Load()
	if base == nil {
		base = declaredAbsentBase()
	}
	if base == nil {
		return 0, false
	}
	value, declared := (*base)[attrName]
	return value, declared
}

// declaredAbsentBase reads the schema defaults once, for the window between
// daemon start and the first OnConfigure delivery.
//
// It answers nil when the schema cannot be read. Refusing the arithmetic is
// what an attribute with no declared base already gets, and it is the only safe
// answer: a substituted zero would write a metric the route never had.
var declaredAbsentBase = sync.OnceValue(func() *map[string]uint32 {
	base, err := parseAttributeDefaults(nil)
	if err != nil {
		logger().Error("filter-modify: attribute defaults unreadable, arithmetic on an absent attribute is refused",
			"error", err)
		return nil
	}
	return &base
})

// readUint32Attr reads one attribute's value out of the update text, and says
// which of the three cases it found.
func readUint32Attr(updateText, attrName string) (uint32, attrReading) {
	prefix := attrName + " "
	_, rest, ok := strings.Cut(updateText, prefix)
	if !ok {
		return 0, attrAbsent
	}
	token, _, _ := strings.Cut(rest, " ")
	v, err := strconv.ParseUint(token, 10, 32)
	if err != nil {
		return 0, attrUnreadable
	}
	return uint32(v), attrPresent //nolint:gosec // G115: bounded by ParseUint 32-bit
}

// currentForArithmetic returns the value an increment or a decrement computes
// from, and whether the operation runs at all.
//
// It runs on the value the route carries, or on the declared base for an
// attribute the route does not carry. It does NOT run when no base is declared,
// and it does NOT run on a value ze rendered and cannot read back: computing
// from a substituted 0 there would write a metric the route never had.
func currentForArithmetic(updateText, attrName string) (uint32, bool) {
	value, reading := readUint32Attr(updateText, attrName)
	switch reading {
	case attrPresent:
		return value, true
	case attrAbsent:
		base, declared := absentBaseFor(attrName)
		if !declared {
			return 0, false
		}
		return base, true
	case attrUnreadable:
		logger().Warn("filter-modify: attribute value did not parse, arithmetic skipped",
			"attribute", attrName)
		return 0, false
	}
	return 0, false
}

// readBool coerces a config presence value to a boolean. Config delivery uses
// a Go bool, while a hand-written or migrated tree can carry the text "true".
// Nothing else is accepted: a leaf nobody set must not remove an attribute.
func readBool(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true"
	}
	return false
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
		// int is 64 bits on every platform ze builds for, so the upper bound is
		// owed here exactly as it is on the int64 branch. Without it a value
		// above the range truncates and reports true, and the caller reads a
		// small metric the operator never wrote.
		if n < 0 || n > 4294967295 {
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
