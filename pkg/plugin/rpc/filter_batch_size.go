// Design: docs/plugin-development/protocol.md -- The filter-update batch.
// Related: types.go -- FilterUpdateInput, the schema whose encoding this bounds.
// Related: message.go -- AppendRequest, which supplies the request frame overhead.
package rpc

import (
	"encoding/json"
	"math"
)

// filterUpdateInputJSONFixed includes every key, delimiter and quote, and the
// widest peer AS number. It deliberately includes both optional fields even
// when an input omits them, so omission only makes the bound more conservative.
var filterUpdateInputJSONFixed = filterUpdateInputJSONFixedSize()

func filterUpdateInputJSONFixedSize() int {
	// Unkeyed within the owning package on purpose: adding a schema field
	// MUST fail compilation here until its size contribution is accounted
	// for. JSON derives the keys and tags, so neither is copied into a bound.
	probe := FilterUpdateInput{"x", "x", "x", uint32(math.MaxUint32), json.RawMessage("null"), []byte{0}}
	encoded, err := json.Marshal(&probe)
	if err != nil {
		panic("BUG: filter-update size probe is not encodable")
	}
	// Nonempty ASCII strings contribute three content bytes, Fields four
	// bytes and Raw four base64 characters. All keys, quotes and delimiters
	// stay in the fixed part, including optional fields' overhead.
	return len(encoded) - 3 - 4 - 4
}

// JSONSizeBound answers an allocation-free upper bound on input's canonical
// JSON encoding, saturated at MaxMessageSize+1. It is a fast proof of fit, not
// a refusal: a larger bound requires measuring the actual encoded input.
// String escaping, including HTML, invalid UTF-8 and Unicode line separators,
// uses at most six output bytes per input byte. RawMessage compaction cannot
// increase that bound; Raw is base64 rather than a quoted byte string.
func (input *FilterUpdateInput) JSONSizeBound() int {
	bound := filterUpdateInputJSONFixed
	for _, octets := range [...]int{len(input.Filter), len(input.Direction), len(input.Peer), len(input.Fields)} {
		if octets > (MaxMessageSize-bound)/6 {
			return MaxMessageSize + 1
		}
		bound += 6 * octets
	}
	// Cap the source length before adding two or multiplying; this also
	// keeps the arithmetic safe on 32-bit platforms.
	if len(input.Raw) > MaxMessageSize {
		return MaxMessageSize + 1
	}
	bound += ((len(input.Raw) + 2) / 3) * 4
	return min(bound, MaxMessageSize+1)
}
