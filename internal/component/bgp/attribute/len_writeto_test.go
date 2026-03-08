package attribute

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
)

// TestLenMatchesWriteTo verifies Len() returns exact bytes WriteTo will use.
//
// This is a critical invariant: callers use Len() to pre-allocate buffers,
// then call WriteTo. If Len() underestimates, WriteTo will panic on buffer overflow.
//
// VALIDATES: Len() accurately predicts WriteTo output size for all attribute types.
//
// PREVENTS: Buffer overflow from undersized allocation (RFC 4271 compliance).
func TestLenMatchesWriteTo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		attr Attribute
	}{
		// Origin
		{"Origin IGP", OriginIGP},
		{"Origin EGP", OriginEGP},
		{"Origin Incomplete", OriginIncomplete},

		// NextHop
		{"NextHop IPv4", &NextHop{Addr: netip.MustParseAddr("192.168.1.1")}},
		{"NextHop IPv6", &NextHop{Addr: netip.MustParseAddr("2001:db8::1")}},

		// MED
		{"MED zero", MED(0)},
		{"MED max", MED(0xFFFFFFFF)},

		// LocalPref
		{"LocalPref zero", LocalPref(0)},
		{"LocalPref 100", LocalPref(100)},

		// AtomicAggregate
		{"AtomicAggregate", AtomicAggregate{}},

		// Aggregator
		{"Aggregator", &Aggregator{ASN: 65001, Address: netip.MustParseAddr("192.168.1.1")}},

		// OriginatorID
		{"OriginatorID", OriginatorID(netip.MustParseAddr("192.168.1.1"))},

		// ClusterList ([]uint32 - stored as IP in uint32 form)
		{"ClusterList empty", ClusterList{}},
		{"ClusterList single", ClusterList{0xC0A80101}}, // 192.168.1.1
		{"ClusterList multiple", ClusterList{0xC0A80101, 0xC0A80102, 0xC0A80103}},

		// Communities
		{"Communities empty", Communities{}},
		{"Communities single", Communities{Community(0xFDE90064)}},
		{"Communities 63 (252 bytes)", makeCommunities(63)},
		{"Communities 64 (256 bytes - extended)", makeCommunities(64)},
		{"Communities 100 (400 bytes)", makeCommunities(100)},

		// ExtendedCommunities
		{"ExtCommunities empty", ExtendedCommunities{}},
		{"ExtCommunities single", ExtendedCommunities{{0x00, 0x02, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64}}},
		{"ExtCommunities 31 (248 bytes)", makeExtCommunities(31)},
		{"ExtCommunities 32 (256 bytes - extended)", makeExtCommunities(32)},

		// LargeCommunities
		{"LargeCommunities empty", LargeCommunities{}},
		{"LargeCommunities single", LargeCommunities{{65001, 100, 200}}},
		{"LargeCommunities 21 (252 bytes)", makeLargeCommunities(21)},
		{"LargeCommunities 22 (264 bytes - extended)", makeLargeCommunities(22)},

		// IPv6ExtendedCommunities
		{"IPv6ExtCommunities empty", IPv6ExtendedCommunities{}},
		{"IPv6ExtCommunities single", makeIPv6ExtCommunities(1)},
		{"IPv6ExtCommunities 12 (240 bytes)", makeIPv6ExtCommunities(12)},
		{"IPv6ExtCommunities 13 (260 bytes - extended)", makeIPv6ExtCommunities(13)},

		// ASPath
		{"ASPath empty", &ASPath{}},
		{"ASPath single ASN", &ASPath{Segments: []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001}}}}},
		{"ASPath multiple ASNs", &ASPath{Segments: []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001, 65002, 65003}}}}},
		{"ASPath multiple segments", &ASPath{Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: []uint32{65001, 65002}},
			{Type: ASSet, ASNs: []uint32{65003, 65004}},
		}}},
		{"ASPath 63 ASNs (254 bytes)", makeASPath(63)},
		{"ASPath 64 ASNs (258 bytes - extended)", makeASPath(64)},
		{"ASPath 100 ASNs (402 bytes)", makeASPath(100)},
		{"ASPath 255 ASNs (max segment)", makeASPath(255)},
		{"ASPath 300 ASNs (split)", makeASPath(300)},

		// AS4Path
		{"AS4Path empty", &AS4Path{}},
		{"AS4Path single", &AS4Path{Segments: []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001}}}}},

		// AS4Aggregator
		{"AS4Aggregator", &AS4Aggregator{ASN: 65001, Address: netip.MustParseAddr("192.168.1.1")}},

		// OpaqueAttribute
		{"OpaqueAttribute empty", NewOpaqueAttribute(FlagOptional|FlagTransitive, 99, nil)},
		{"OpaqueAttribute with data", NewOpaqueAttribute(FlagOptional|FlagTransitive, 99, []byte{0x01, 0x02, 0x03, 0x04})},
		{"OpaqueAttribute large", NewOpaqueAttribute(FlagOptional|FlagTransitive, 99, make([]byte, 300))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expectedLen := tt.attr.Len()

			buf := make([]byte, 65536)
			n := tt.attr.WriteTo(buf, 0)

			assert.Equal(t, expectedLen, n,
				"%T: Len()=%d but WriteTo()=%d", tt.attr, expectedLen, n)
		})
	}
}

// TestLenMatchesWriteToWithContext verifies context-dependent Len matches WriteTo.
//
// Context-dependent attributes (AS_PATH, Aggregator) have different sizes based on ASN4.
//
// VALIDATES: LenWithASN4/Len matches WriteToWithContext output size.
//
// PREVENTS: Buffer overflow when encoding for 2-byte vs 4-byte ASN peers.
func TestLenMatchesWriteToWithContext(t *testing.T) {
	t.Parallel()
	contexts := []*bgpctx.EncodingContext{
		nil,
		bgpctx.EncodingContextForASN4(true),
		bgpctx.EncodingContextForASN4(false),
	}

	// AS_PATH - context-dependent
	aspaths := []*ASPath{
		{},
		{Segments: []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001, 65002}}}},
		makeASPath(100),
		makeASPath(300),
	}

	for _, path := range aspaths {
		for _, ctx := range contexts {
			name := "nil"
			if ctx != nil {
				name = "ASN4=" + boolStr(ctx.ASN4())
			}

			t.Run("ASPath_"+name, func(t *testing.T) {
				t.Parallel()
				asn4 := ctx == nil || ctx.ASN4()
				expectedLen := path.LenWithASN4(asn4)

				buf := make([]byte, 65536)
				n := path.WriteToWithContext(buf, 0, nil, ctx)

				assert.Equal(t, expectedLen, n,
					"ASPath: LenWithASN4(%v)=%d but WriteToWithContext()=%d",
					asn4, expectedLen, n)
			})
		}
	}

	// Aggregator - context-dependent (8 bytes for ASN4, 6 bytes for 2-byte)
	agg := &Aggregator{ASN: 65001, Address: netip.MustParseAddr("192.168.1.1")}
	for _, ctx := range contexts {
		name := "nil"
		expectedLen := 8 // Default ASN4
		if ctx != nil {
			name = "ASN4=" + boolStr(ctx.ASN4())
			if !ctx.ASN4() {
				expectedLen = 6
			}
		}

		t.Run("Aggregator_"+name, func(t *testing.T) {
			t.Parallel()
			buf := make([]byte, 65536)
			n := agg.WriteToWithContext(buf, 0, nil, ctx)

			assert.Equal(t, expectedLen, n,
				"Aggregator: expected %d but WriteToWithContext()=%d", expectedLen, n)
		})
	}
}

// TestWriteAttrToLenMatchesOutput verifies WriteAttrTo total matches header+value.
//
// VALIDATES: WriteAttrTo produces correct total length (header + Len()).
//
// PREVENTS: Header/value length mismatch causing parse errors.
func TestWriteAttrToLenMatchesOutput(t *testing.T) {
	t.Parallel()
	attrs := []Attribute{
		OriginIGP,
		MED(100),
		LocalPref(200),
		Communities{Community(0xFDE90064)},
		makeCommunities(100),
		&ASPath{Segments: []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001, 65002}}}},
		makeASPath(100),
	}

	for _, attr := range attrs {
		t.Run(attr.Code().String(), func(t *testing.T) {
			t.Parallel()
			buf := make([]byte, 65536)
			n := WriteAttrTo(attr, buf, 0)

			// Parse header to verify
			_, _, valueLen, hdrLen, err := ParseHeader(buf[:n])
			assert.NoError(t, err)

			// Verify: total = header + value
			assert.Equal(t, n, hdrLen+int(valueLen),
				"WriteAttrTo: total=%d but header(%d)+value(%d)=%d",
				n, hdrLen, valueLen, hdrLen+int(valueLen))

			// Verify: valueLen matches Len()
			assert.Equal(t, attr.Len(), int(valueLen),
				"WriteAttrTo: Len()=%d but header says %d", attr.Len(), valueLen)
		})
	}
}

// Helper functions

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func makeExtCommunities(n int) ExtendedCommunities {
	ecs := make(ExtendedCommunities, n)
	for i := range ecs {
		ecs[i] = ExtendedCommunity{0x00, 0x02, byte(i >> 8), byte(i), 0x00, 0x00, 0x00, byte(i)}
	}
	return ecs
}

func makeLargeCommunities(n int) LargeCommunities {
	lcs := make(LargeCommunities, n)
	for i := range lcs {
		lcs[i] = LargeCommunity{
			GlobalAdmin: uint32(65000 + i), //nolint:gosec // G115: test helper, i bounded
			LocalData1:  uint32(i),         //nolint:gosec // G115: test helper, i bounded
			LocalData2:  uint32(i * 2),     //nolint:gosec // G115: test helper, i bounded
		}
	}
	return lcs
}

func makeIPv6ExtCommunities(n int) IPv6ExtendedCommunities {
	ecs := make(IPv6ExtendedCommunities, n)
	for i := range ecs {
		ecs[i] = IPv6ExtendedCommunity{
			0x00, byte(i),
			0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, byte(i >> 8), byte(i),
			0x00, byte(i),
		}
	}
	return ecs
}
