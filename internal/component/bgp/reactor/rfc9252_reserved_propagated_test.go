// Design: docs/architecture/core-design.md — egress attribute modification on the forward rails
// RFC: rfc/short/rfc9252.md — SRv6 Service TLVs, Section 2
// Related: rfc8669_nexthop_change_defect_test.go — rfc8669Relay, the relay harness these tests drive
//
// RFC 9252 Section 2: "If the BGP next hop is unchanged during the advertisement, the SRv6
// Service TLVs, including any unrecognized Types of Sub-TLV and Sub-Sub-TLV, SHOULD be
// propagated further. In addition, all Reserved fields in the TLV, Sub-TLV, or Sub-Sub-TLV
// MUST be propagated unchanged."

package reactor

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

// rfc9252ReservedServiceTLV builds an SRv6 Service TLV of the given type (5 is L3, 6 is
// L2) whose every Reserved field is non-zero, and which carries an unrecognized Sub-TLV
// and an unrecognized Sub-Sub-TLV:
//
//	TLV      Type(1) Length(2) RESERVED(1)=0xA5                         (Section 2)
//	Sub-TLV  Type 1 Length(2) Reserved1(1)=0x5A SID(16) Flags(1)=0x00
//	         Behavior(2)=0x0013 (End.DT4) Reserved2(1)=0xC3            (Section 3.1)
//	  Sub-Sub-TLV Type 1 Length 6: LBL 40, LNL 24, FL 16, AL 0,
//	              Transposition Length 0, Offset 0                       (Section 3.2.1)
//	  Sub-Sub-TLV Type 0xEE Length 2: 0x77 0x88 (unrecognized)
//	Sub-TLV  Type 0xF0 Length 3: 0x99 0x00 0x11 (unrecognized)
func rfc9252ReservedServiceTLV(tlvType byte) []byte {
	sid := netip.MustParseAddr("2001:db8::1").As16()
	subSub := []byte{
		0x01, 0x00, 0x06, 40, 24, 16, 0, 0, 0,
		0xEE, 0x00, 0x02, 0x77, 0x88,
	}
	info := slices.Concat([]byte{0x5A}, sid[:], []byte{0x00, 0x00, 0x13, 0xC3}, subSub)
	subTLVs := slices.Concat([]byte{0x01, 0x00, byte(len(info))}, info, []byte{0xF0, 0x00, 0x03, 0x99, 0x00, 0x11})
	value := slices.Concat([]byte{0xA5}, subTLVs)
	return slices.Concat([]byte{tlvType, 0x00, byte(len(value))}, value)
}

// TestRFC9252ReservedFieldsPropagatedUnchanged relays, over both relay rails with the next
// hop unchanged, a Prefix-SID whose L3 and L2 Service TLVs carry non-zero Reserved octets
// in the TLV (RESERVED) and the SID Information Sub-TLV (Reserved1, Reserved2), plus an
// unrecognized Sub-TLV and an unrecognized Sub-Sub-TLV, and reads attribute 40 on the wire.
//
// VALIDATES: the attribute arrives byte-identical to the one received, so each Reserved
// octet keeps its received value and the unrecognized Sub-TLV and Sub-Sub-TLV are still
// there; the next hop is really the source's.
// PREVENTS: a forward rail that re-encodes the Service TLVs and writes zero into a
// Reserved field, or drops a Sub-TLV or Sub-Sub-TLV it does not recognize. With every
// Reserved octet zero, as in the older fixtures, a zeroing encoder stayed green.
//
// RFC requirement: RFC9252-3.3-1 positive -- relayed with the next hop unchanged, on the general and the route-server rail, the L3 and L2 Service TLVs arrive byte-identical: TLV RESERVED 0xA5, Sub-TLV Reserved1 0x5A and Reserved2 0xC3 unchanged, the unrecognized Sub-TLV 0xF0 and Sub-Sub-TLV 0xEE still present.
func TestRFC9252ReservedFieldsPropagatedUnchanged(t *testing.T) {
	received := slices.Concat(rfc8669LabelIndexTLV, rfc9252ReservedServiceTLV(5), rfc9252ReservedServiceTLV(6))
	for _, rail := range []struct {
		name        string
		routeServer bool
	}{{name: "general rail"}, {name: "route-server rail", routeServer: true}} {
		t.Run(rail.name, func(t *testing.T) {
			attrs := decodeBodyAttrs(t, rfc8669Relay(t, rail.routeServer, NextHopUnchanged, received))
			assert.Equal(t, []byte{192, 0, 2, 99}, attrs[3], "the next hop is the source's")
			assert.Equal(t, received, attrs[prefixSIDCodeByte],
				"RFC 9252 Section 2: every Reserved field, and every unrecognized Sub-TLV and Sub-Sub-TLV, propagated unchanged")
		})
	}
}
