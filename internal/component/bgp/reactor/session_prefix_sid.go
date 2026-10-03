// Design: docs/features/srv6.md -- Prefix-SID transmission rules.
// Related: session_write.go -- owned final wire buffers on every UPDATE writer.
package reactor

import (
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// clearTransmittedLabelIndex clears only Label-Index Reserved and Flags in an
// owned outgoing UPDATE body. Callers MUST pass the session's private write
// buffer after policy; borrowed received bytes MUST NOT be modified.
//
// RFC 8669 Section 3.1: "RESERVED: 8-bit field. It MUST be clear on transmission
// and MUST be ignored on reception." "The Flags field MUST be clear on
// transmission and MUST be ignored on reception."
// Section 3.2: "The Originator SRGB TLV MUST NOT be changed during the
// propagation of the BGP update." Only type 1 is changed, including on relay.
//
// Label-Index TLV (RFC 8669 Section 3.1):
//
//	Offset    0     1..2       3        4..5       6..9
//	       +------+--------+----------+---------+-------------+
//	       |Type=1|Length=7| Reserved |  Flags  | Label Index |
//	       +------+--------+----------+---------+-------------+
//
// Both walks are bounded by their enclosing wire length. Malformed framing is
// left to the validator, never repaired by guessing a subsequent TLV boundary.
func clearTransmittedLabelIndex(body []byte) {
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return
	}
	_, _, value, found := attribute.AttrFind(sections.Attrs(body), attribute.AttrPrefixSID)
	if !found {
		return
	}
	for len(value) != 0 {
		octets, ok := prefixSIDTLVOctets(value)
		if !ok {
			return
		}
		if value[0] == 1 && octets == 10 {
			if value[3]|value[4]|value[5] != 0 {
				clear(value[3:6])
			}
		}
		value = value[octets:]
	}
}
