package attribute

import (
	"net/netip"
	"testing"

	bgpctx "codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"
)

const ctxNameNil = "nil"

// TestAttrLenWithContext_MatchesPackWithContext verifies that attrLenWithContext
// returns the same length as len(attr.PackWithContext(nil, ctx)) for all
// context-dependent attributes.
//
// VALIDATES: attrLenWithContext is consistent with PackWithContext.
// PREVENTS: Buffer overflow or garbage when using WriteAttrToWithContext.
func TestAttrLenWithContext_MatchesPackWithContext(t *testing.T) {
	// Context-dependent attributes per RFC 6793
	testCases := []struct {
		name string
		attr Attribute
	}{
		{
			name: "ASPath_empty",
			attr: &ASPath{Segments: nil},
		},
		{
			name: "ASPath_single_AS",
			attr: &ASPath{Segments: []ASPathSegment{
				{Type: ASSequence, ASNs: []uint32{65001}},
			}},
		},
		{
			name: "ASPath_multiple_AS",
			attr: &ASPath{Segments: []ASPathSegment{
				{Type: ASSequence, ASNs: []uint32{65001, 65002, 65003}},
			}},
		},
		{
			name: "ASPath_large_AS",
			attr: &ASPath{Segments: []ASPathSegment{
				{Type: ASSequence, ASNs: []uint32{4200000001}}, // > 65535
			}},
		},
		{
			name: "ASPath_mixed_segments",
			attr: &ASPath{Segments: []ASPathSegment{
				{Type: ASSequence, ASNs: []uint32{65001, 65002}},
				{Type: ASSet, ASNs: []uint32{65003, 65004}},
			}},
		},
		{
			name: "Aggregator_small_AS",
			attr: &Aggregator{ASN: 65001, Address: netip.MustParseAddr("10.0.0.1")},
		},
		{
			name: "Aggregator_large_AS",
			attr: &Aggregator{ASN: 4200000001, Address: netip.MustParseAddr("10.0.0.1")},
		},
	}

	// Test both ASN4 contexts
	contexts := []struct {
		name string
		ctx  *bgpctx.EncodingContext
	}{
		{"nil", nil},
		{"ASN4=true", bgpctx.EncodingContextForASN4(true)},
		{"ASN4=false", bgpctx.EncodingContextForASN4(false)},
	}

	for _, tc := range testCases {
		for _, ctxCase := range contexts {
			name := tc.name + "_" + ctxCase.name
			t.Run(name, func(t *testing.T) {
				attr := tc.attr
				ctx := ctxCase.ctx

				// Get length via attrLenWithContext
				lenFromFunc := attrLenWithContext(attr, ctx)

				// Get length via PackWithContext
				packed := attr.PackWithContext(nil, ctx)
				lenFromPack := len(packed)

				if lenFromFunc != lenFromPack {
					t.Errorf("attrLenWithContext=%d but len(PackWithContext)=%d",
						lenFromFunc, lenFromPack)
				}
			})
		}
	}
}

// TestAttrLenWithContext_MatchesWriteToWithContext verifies that attrLenWithContext
// returns the same length as WriteToWithContext actually writes.
//
// VALIDATES: Buffer size from attrLenWithContext is exactly what WriteToWithContext needs.
// PREVENTS: Buffer overflow when WriteToWithContext writes more than predicted.
func TestAttrLenWithContext_MatchesWriteToWithContext(t *testing.T) {
	testCases := []struct {
		name string
		attr Attribute
	}{
		{"ASPath_single", &ASPath{Segments: []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001}}}}},
		{"ASPath_multiple", &ASPath{Segments: []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001, 65002}}}}},
		{"Aggregator", &Aggregator{ASN: 65001, Address: netip.MustParseAddr("10.0.0.1")}},
	}

	contexts := []*bgpctx.EncodingContext{
		nil,
		bgpctx.EncodingContextForASN4(true),
		bgpctx.EncodingContextForASN4(false),
	}

	for _, tc := range testCases {
		for _, ctx := range contexts {
			ctxName := ctxNameNil
			if ctx != nil {
				if ctx.ASN4() {
					ctxName = "ASN4=true"
				} else {
					ctxName = "ASN4=false"
				}
			}
			name := tc.name + "_" + ctxName

			t.Run(name, func(t *testing.T) {
				attr := tc.attr

				// Get predicted length
				predictedLen := attrLenWithContext(attr, ctx)

				// Allocate buffer and write
				buf := make([]byte, predictedLen+10) // Extra space to detect overflow
				written := attr.WriteToWithContext(buf, 0, nil, ctx)

				if written != predictedLen {
					t.Errorf("attrLenWithContext=%d but WriteToWithContext wrote %d bytes",
						predictedLen, written)
				}
			})
		}
	}
}

// TestWriteAttrToWithContext_MatchesPack verifies that WriteAttrToWithContext
// produces the same bytes as PackAttribute + PackWithContext.
//
// VALIDATES: WriteAttrToWithContext produces identical wire format.
// PREVENTS: Protocol errors from incorrect encoding.
func TestWriteAttrToWithContext_MatchesPack(t *testing.T) {
	testCases := []struct {
		name string
		attr Attribute
	}{
		{"Origin_IGP", Origin(0)},
		{"Origin_EGP", Origin(1)},
		{"ASPath_empty", &ASPath{Segments: nil}},
		{"ASPath_single", &ASPath{Segments: []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001}}}}},
		{"Aggregator", &Aggregator{ASN: 65001, Address: netip.MustParseAddr("10.0.0.1")}},
		{"LocalPref", LocalPref(100)},
		{"MED", MED(50)},
	}

	contexts := []*bgpctx.EncodingContext{
		nil,
		bgpctx.EncodingContextForASN4(true),
		bgpctx.EncodingContextForASN4(false),
	}

	for _, tc := range testCases {
		for _, ctx := range contexts {
			ctxName := ctxNameNil
			if ctx != nil {
				if ctx.ASN4() {
					ctxName = "ASN4=true"
				} else {
					ctxName = "ASN4=false"
				}
			}
			name := tc.name + "_" + ctxName

			t.Run(name, func(t *testing.T) {
				attr := tc.attr

				// Pack using old method
				value := attr.PackWithContext(nil, ctx)
				header := PackHeader(attr.Flags(), attr.Code(), uint16(len(value))) //nolint:gosec // test data, overflow not possible
				expected := append(header, value...)                                //nolint:gocritic // intentional: create new slice for comparison

				// Write using new method
				buf := make([]byte, len(expected)+10)
				written := WriteAttrToWithContext(attr, buf, 0, nil, ctx)

				if written != len(expected) {
					t.Errorf("WriteAttrToWithContext wrote %d bytes, expected %d",
						written, len(expected))
				}

				actual := buf[:written]
				if string(actual) != string(expected) {
					t.Errorf("Wire format mismatch:\n  got:  %x\n  want: %x",
						actual, expected)
				}
			})
		}
	}
}
