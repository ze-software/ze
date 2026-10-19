package wireu

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestBuildUpdatePayloadMPAttributeFirst pins the attribute order of every
// UPDATE body SplitWireUpdate assembles.
//
// RFC 7606 Section 5.1: "The MP_REACH_NLRI or MP_UNREACH_NLRI attribute (if
// present) SHALL be encoded as the very first path attribute in an UPDATE
// message."
//
// VALIDATES: buildUpdatePayload writes MP_REACH_NLRI, and MP_UNREACH_NLRI, ahead
// of the base attributes, and a withdrawal-only body carries no base attribute.
// PREVENTS: the base attributes being copied ahead of the MP attribute, the
// order this builder used before the three builders were aligned.
//
// RFC requirement: RFC7606-5.1-1 positive -- buildUpdatePayload, which assembles every UPDATE body SplitWireUpdate emits, writes MP_REACH_NLRI as the first path attribute ahead of the ORIGIN and AS_PATH base attributes, and writes a withdrawal-only body with MP_UNREACH_NLRI as its only path attribute.
func TestBuildUpdatePayloadMPAttributeFirst(t *testing.T) {
	base := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00}                                        // ORIGIN, AS_PATH
	mpReach := []byte{0x80, 0x0e, 0x0a, 0x00, 0x02, 0x01, 0x00, 0x00, 0x20, 0x20, 0x01, 0x0d, 0xb8} // no next hop, 2001:db8::/32
	mpUnreach := []byte{0x80, 0x0f, 0x08, 0x00, 0x02, 0x01, 0x20, 0x20, 0x01, 0x0d, 0xb8}           // 2001:db8::/32

	cases := []struct {
		name      string
		mpUnreach []byte
		mpReach   []byte
		want      []byte
	}{
		{name: "announce", mpReach: mpReach, want: []byte{14, 1, 2}},
		{name: "withdraw", mpUnreach: mpUnreach, want: []byte{15}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := buildUpdatePayload(nil, base, tc.mpUnreach, tc.mpReach, nil)
			attrLen := int(binary.BigEndian.Uint16(body[2:4]))
			attrs := body[4 : 4+attrLen]
			var codes []byte
			for off := 0; off < len(attrs); {
				if off+3 > len(attrs) {
					t.Fatalf("attribute at %d runs past the block %x", off, attrs)
					return
				}
				codes = append(codes, attrs[off+1])
				off += 3 + int(attrs[off+2])
			}
			if !bytes.Equal(codes, tc.want) {
				t.Errorf("attribute codes = %v, want %v", codes, tc.want)
			}
		})
	}
}
