// Design: docs/features/srv6.md -- Prefix-SID transmission rules.
// Related: session_write.go -- owned final wire buffers on every UPDATE writer.
// Related: session_egress_normalize.go -- the attribute walk that calls this.
package reactor

import (
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// clearLabelIndexTLVs clears only Label-Index Reserved and Flags in the
// Prefix-SID attribute value of an owned outgoing UPDATE. attrs is the whole
// path attribute section the value sits in, read only for the route family.
// It is the Prefix-SID step of normalizeTransmittedUpdate, the one attribute
// walk every written UPDATE takes; callers MUST pass the session's private
// write buffer after policy; borrowed received bytes MUST NOT be modified.
//
// RFC 8669 Section 3.1: "RESERVED: 8-bit field. It MUST be clear on transmission
// and MUST be ignored on reception." "The Flags field MUST be clear on
// transmission and MUST be ignored on reception."
// Section 3.2: "The Originator SRGB TLV MUST NOT be changed during the
// propagation of the BGP update." Only type 1 is changed, including on relay.
//
// Only on IPv4 and IPv6 labeled unicast, the families RFC 8669 Section 3.1
// defines the TLV for: "It MUST be ignored when received for other BGP AFI/SAFI
// combinations." Elsewhere a type 1 TLV is unrecognized, and RFC 8669 Section 6
// says "For future extensibility, unknown TLVs MUST be ignored and propagated
// unmodified." The family (attribute.PrefixSIDRouteFamily) is read only once a
// TLV that needs clearing is met. When it cannot be read the value goes out
// unmodified: nothing then shows the TLV is one Ze recognizes.
//
// Label-Index TLV (RFC 8669 Section 3.1):
//
//	Offset    0     1..2       3        4..5       6..9
//	       +------+--------+----------+---------+-------------+
//	       |Type=1|Length=7| Reserved |  Flags  | Label Index |
//	       +------+--------+----------+---------+-------------+
//
// The TLV walk is bounded by the attribute length. Malformed framing is left
// to the validator, never repaired by guessing a subsequent TLV boundary.
func clearLabelIndexTLVs(attrs, value []byte) {
	familyChecked := false
	for len(value) != 0 {
		octets, ok := prefixSIDTLVOctets(value)
		if !ok {
			return
		}
		if value[0] == 1 && octets == 10 && value[3]|value[4]|value[5] != 0 {
			if !familyChecked {
				afi, safi, known := attribute.PrefixSIDRouteFamily(attrs)
				if !known {
					return
				}
				if !attribute.PrefixSIDLabeledUnicast(afi, safi) {
					return
				}
				familyChecked = true
			}
			clear(value[3:6])
		}
		value = value[octets:]
	}
}
