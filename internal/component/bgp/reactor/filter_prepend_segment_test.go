package reactor

import (
	"bytes"
	"encoding/hex"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestAspathHandlerPrependJoinsLeadingSequence pins where a policy prepend
// lands in the AS_PATH, for every shape of the path's first segment.
//
// RFC 4271 Section 5.1.2 b: "if the first path segment of the AS_PATH is of
// type AS_SEQUENCE, the local system prepends its own AS number as the last
// element of the sequence (put it in the leftmost position with respect to the
// position of octets in the protocol message)." An AS_SET first, an empty path,
// and a sequence that would pass 255 AS numbers each start a new AS_SEQUENCE.
//
// Method: aspathHandler is driven over hand-built operations, as
// prependASPathOps records them, and the emitted attribute is compared with the
// stated hex. Every case is also decoded and compared with
// attribute.ASPath.Prepend over the same base, which is the declaration the
// AS4_PATH derivation (prependAS4PathValue) uses, so AS_PATH and AS4_PATH
// cannot disagree about the segment layout.
//
// VALIDATES: a prepend joins a leading AS_SEQUENCE with room, at both widths,
// whether the base is the received value or an earlier Set.
// PREVENTS: every prepend written as an extra AS_SEQUENCE segment in front.
func TestAspathHandlerPrependJoinsLeadingSequence(t *testing.T) {
	handler := aspathHandler()
	code := byte(attribute.AttrASPath)
	prepend := func(h string) filterapi.AttrOp {
		return filterapi.AttrOp{Code: code, Action: filterapi.AttrModPrepend, Buf: nlriHex(t, h)}
	}
	set := func(h string) filterapi.AttrOp {
		return filterapi.AttrOp{Code: code, Action: filterapi.AttrModSet, Buf: nlriHex(t, h)}
	}
	// A 2-octet AS_SEQUENCE of 254 copies of AS 174, one short of the bound.
	full := "02fe" + strings.Repeat("00ae", 254)

	cases := []struct {
		name   string
		source string // AS_PATH value in the source UPDATE; "-" when absent
		ops    []filterapi.AttrOp
		want   string // emitted AS_PATH value
		asn4   bool
		asns   []uint32 // what the ops prepend, leftmost first
	}{
		{name: "sequence-two-octet", source: "0202 0003 00ae", ops: []filterapi.AttrOp{prepend("0202 fde8 fde8")},
			want: "0204 fde8 fde8 0003 00ae", asns: []uint32{65000, 65000}},
		{name: "sequence-four-octet", source: "0201 00000d1c", ops: []filterapi.AttrOp{prepend("0202 0000fde8 0000fde8")},
			want: "0203 0000fde8 0000fde8 00000d1c", asn4: true, asns: []uint32{65000, 65000}},
		{name: "set-first", source: "0102 0003 00ae", ops: []filterapi.AttrOp{prepend("0201 fde8")},
			want: "0201 fde8 0102 0003 00ae", asns: []uint32{65000}},
		{name: "empty-path", source: "", ops: []filterapi.AttrOp{prepend("0201 fde8")},
			want: "0201 fde8", asns: []uint32{65000}},
		{name: "absent-path", source: "-", ops: []filterapi.AttrOp{prepend("0201 fde8")},
			want: "0201 fde8", asns: []uint32{65000}},
		{name: "overflow", source: full, ops: []filterapi.AttrOp{prepend("0203 fde8 fde8 fde8")},
			want: "0202 fde8 fde8 02ff fde8" + strings.Repeat("00ae", 254), asns: []uint32{65000, 65000, 65000}},
		{name: "set-base", source: "-", ops: []filterapi.AttrOp{set("0201 fdea"), prepend("0201 fde8")},
			want: "0202 fde8 fdea", asns: []uint32{65000}},
		{name: "two-prepends", source: "0201 0003", ops: []filterapi.AttrOp{prepend("0201 fde8"), prepend("0201 fde9")},
			want: "0203 fde8 fde9 0003", asns: []uint32{65000, 65001}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var src []byte
			base := tc.source
			if tc.source != "-" {
				src = makeAttr(0x40, code, nlriHex(t, tc.source))
			}
			for _, op := range tc.ops {
				if op.Action == filterapi.AttrModSet {
					base = hex.EncodeToString(op.Buf)
				}
			}
			out, ok := planHandlerBytes(handler, code, src, tc.ops)
			if !ok {
				t.Fatalf("handler refused the plan")
			}
			value := out[3:]
			if out[0]&0x10 != 0 { // RFC 4271 Section 4.3: Extended Length, two length octets
				value = out[4:]
			}
			want := nlriHex(t, tc.want)
			if !bytes.Equal(value, want) {
				t.Fatalf("AS_PATH value = %x, want %x", value, want)
			}

			// The same layout attribute.ASPath.Prepend builds, one AS at a time
			// from the rightmost prepended AS number.
			path := &attribute.ASPath{}
			if base != "-" && base != "" {
				parsed, err := attribute.ParseASPath(nlriHex(t, base), tc.asn4)
				if err != nil {
					t.Fatalf("parse base: %v", err)
				}
				path = parsed
			}
			for _, asn := range slices.Backward(tc.asns) {
				path.Prepend(asn)
			}
			expect := make([]byte, path.LenWithASN4(tc.asn4))
			path.WriteToWithASN4(expect, 0, tc.asn4)
			if !bytes.Equal(value, expect) {
				t.Fatalf("AS_PATH value = %x, ASPath.Prepend builds %x", value, expect)
			}
		})
	}

	t.Run("malformed-prepend-refused", func(t *testing.T) {
		// A count of 2 over one AS number's bytes: no width reads it.
		ops := []filterapi.AttrOp{prepend("0202 fde8")}
		if _, ok := planHandlerBytes(handler, code, makeAttr(0x40, code, nlriHex(t, "0201 0003")), ops); ok {
			t.Fatalf("handler planned a malformed prepend segment")
		}
	})
}
