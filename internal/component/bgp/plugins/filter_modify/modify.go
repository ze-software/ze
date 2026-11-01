// Design: docs/architecture/core-design.md -- route attribute modifier
// Related: config.go -- modify definition config parsing
// Related: filter_modify.go -- SDK entry point and evaluateModify
// RFC: rfc/short/rfc4271.md -- Sections 9.1.1 and 9.1.2.2 decide the absent-attribute bases
// RFC: rfc/short/rfc7311.md -- Sections 3.4.1 and 4.1 are why aigp gets no base
//
// A definition answers its operations as filter sets (rpc.FilterSet), built
// once at config time, so a route costs the plugin no formatting. The engine
// applies them (policychain Sets): a set writes the value, an increment or a
// decrement resolves against the route's value, or, for an absent
// LOCAL_PREF or MULTI_EXIT_DISC, against the chain's base from
// bgp { defaults { attribute { } } }. An AIGP increment of a route with no
// AIGP changes nothing, since RFC 7311 gives no number that could stand in
// for an absent metric.
//
// The plugin returns "modify" for a route that meets the definition's match
// condition, and "accept" for one that does not. See filter_modify.go,
// evaluateModify. An earlier match filter in the chain stays available, as a
// policy { match X { ... } } instance the chain names:
// "filter { import { filters [ X modify:Y ]; final accept; } }".
package filter_modify

import (
	"encoding/json"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// modifyDef is a named modifier definition loaded from config. Every field is
// written by parseModifyDefs before the definition is installed, and read
// only after (defsByName), so it is safe for concurrent use.
type modifyDef struct {
	name  string
	match matchCond // when stated, the condition a route meets to be modified
	// sets are the operations every route the definition applies to gets.
	sets []rpc.FilterSet
	// importSets are sets plus the med remove set, the operations an import
	// route gets when medRemove holds. The removal is RFC 4271 Section 5.1.4's
	// configured MULTI_EXIT_DISC removal, honored on an import chain only, and
	// the direction is known per call rather than at config time
	// (medRemoveFor).
	importSets []rpc.FilterSet
	medRemove  bool
	// delta and importDelta are the operations as text, for the "modify apply"
	// log line, in the same two variants.
	delta       string
	importDelta string
}

// setNumber answers the set of a numeric field: set, increment or decrement.
func setNumber(field string, action rpc.FilterSetAction, value uint32) rpc.FilterSet {
	return rpc.FilterSet{Field: field, Action: action, Value: json.RawMessage(strconv.AppendUint(nil, uint64(value), 10))}
}

// setText answers the set of a field whose value is one string. value is a
// config leaf already validated by its parser, so its JSON encoding cannot
// fail.
func setText(field string, action rpc.FilterSetAction, value string) rpc.FilterSet {
	quoted, err := json.Marshal(value)
	if err != nil {
		panic("BUG: filter-modify: a string has no JSON encoding: " + err.Error())
	}
	return rpc.FilterSet{Field: field, Action: action, Value: quoted}
}

// medRemoveSet is the removal of MULTI_EXIT_DISC, whose value is null.
func medRemoveSet() rpc.FilterSet {
	return rpc.FilterSet{Field: medAttr, Action: rpc.FilterSetActionRemove, Value: json.RawMessage("null")}
}

// deltaText writes the delta text of one operation into buf: the leaf name,
// a space and the value, with a sign for an increment or a decrement.
func deltaText(buf *textbuf.Buffer, leaf string, sign byte, value string) {
	if buf.Len() > 0 {
		buf.Byte(' ')
	}
	buf.Str(leaf).Byte(' ')
	if sign != 0 {
		buf.Byte(sign)
	}
	buf.Str(value)
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
