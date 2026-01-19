package attribute_test

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/zebgp/internal/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/zebgp/internal/bgp/context"
	"github.com/stretchr/testify/require"
)

// Test contexts for PackWithContext tests.
var (
	ctxASN4 = bgpctx.EncodingContextForASN4(true)
	ctxASN2 = bgpctx.EncodingContextForASN4(false)
)

// =============================================================================
// ASPath.PackWithContext Tests
// =============================================================================

// TestASPathPackWithContext_ASN4ToASN4 verifies 4-byte encoding passthrough.
//
// VALIDATES: 4-byte ASNs when both srcCtx.ASN4=true and dstCtx.ASN4=true.
//
// PREVENTS: Unnecessary transcoding when capabilities match.
func TestASPathPackWithContext_ASN4ToASN4(t *testing.T) {
	path := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002, 100000}},
		},
	}

	result := path.PackWithContext(ctxASN4, ctxASN4)

	// Expected: type(1) + count(1) + 3 ASNs * 4 bytes = 14 bytes
	require.Len(t, result, 14)

	// Check segment header
	require.Equal(t, byte(attribute.ASSequence), result[0])
	require.Equal(t, byte(3), result[1])

	// Check ASNs are 4-byte encoded
	require.Equal(t, uint32(65001), binary.BigEndian.Uint32(result[2:6]))
	require.Equal(t, uint32(65002), binary.BigEndian.Uint32(result[6:10]))
	require.Equal(t, uint32(100000), binary.BigEndian.Uint32(result[10:14]))
}

// TestASPathPackWithContext_ASN4ToASN2 verifies 2-byte encoding with AS_TRANS.
//
// VALIDATES: Large ASNs become AS_TRANS (23456) when dstCtx.ASN4=false.
//
// PREVENTS: Protocol errors when sending to legacy peers.
func TestASPathPackWithContext_ASN4ToASN2(t *testing.T) {
	path := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 100000}}, // 100000 > 65535
		},
	}

	result := path.PackWithContext(ctxASN4, ctxASN2)

	// Expected: type(1) + count(1) + 2 ASNs * 2 bytes = 6 bytes
	require.Len(t, result, 6)

	// Check segment header
	require.Equal(t, byte(attribute.ASSequence), result[0])
	require.Equal(t, byte(2), result[1])

	// Check ASNs are 2-byte encoded
	require.Equal(t, uint16(65001), binary.BigEndian.Uint16(result[2:4]))
	// 100000 should be AS_TRANS (23456)
	require.Equal(t, uint16(23456), binary.BigEndian.Uint16(result[4:6]))
}

// TestASPathPackWithContext_ASN2ToASN4 verifies upgrade path.
//
// VALIDATES: 4-byte encoding when dstCtx.ASN4=true regardless of source.
//
// PREVENTS: Wrong encoding after AS4_PATH merge.
func TestASPathPackWithContext_ASN2ToASN4(t *testing.T) {
	// After AS4_PATH merge, path contains real 4-byte ASNs
	path := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 200000}},
		},
	}

	result := path.PackWithContext(ctxASN2, ctxASN4)

	// Expected: 4-byte encoding
	require.Len(t, result, 10) // type + count + 2*4
	require.Equal(t, uint32(200000), binary.BigEndian.Uint32(result[6:10]))
}

// TestASPathPackWithContext_NilDstCtx verifies default to ASN4.
//
// VALIDATES: nil dstCtx defaults to 4-byte ASN encoding.
//
// PREVENTS: Panic on nil context.
func TestASPathPackWithContext_NilDstCtx(t *testing.T) {
	path := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		},
	}

	result := path.PackWithContext(nil, nil)

	// Should use 4-byte encoding (default)
	require.Len(t, result, 6) // type + count + 1*4
	require.Equal(t, uint32(65001), binary.BigEndian.Uint32(result[2:6]))
}

// =============================================================================
// Aggregator.PackWithContext Tests
// =============================================================================

// TestAggregatorPackWithContext_ASN4 verifies 8-byte format.
//
// VALIDATES: 8-byte format (4-byte ASN + 4-byte IP) when dstCtx.ASN4=true.
//
// PREVENTS: Parse errors on ASN4-capable peers.
func TestAggregatorPackWithContext_ASN4(t *testing.T) {
	agg := &attribute.Aggregator{
		ASN:     100000, // Large ASN
		Address: netip.MustParseAddr("192.0.2.1"),
	}

	result := agg.PackWithContext(ctxASN4, ctxASN4)

	require.Len(t, result, 8)
	require.Equal(t, uint32(100000), binary.BigEndian.Uint32(result[0:4]))
	require.Equal(t, []byte{192, 0, 2, 1}, result[4:8])
}

// TestAggregatorPackWithContext_ASN2 verifies 6-byte format with AS_TRANS.
//
// VALIDATES: 6-byte format with AS_TRANS when dstCtx.ASN4=false and ASN > 65535.
//
// PREVENTS: Protocol errors when sending to legacy peers.
func TestAggregatorPackWithContext_ASN2(t *testing.T) {
	agg := &attribute.Aggregator{
		ASN:     100000, // Large ASN → AS_TRANS
		Address: netip.MustParseAddr("192.0.2.1"),
	}

	result := agg.PackWithContext(ctxASN4, ctxASN2)

	require.Len(t, result, 6)
	require.Equal(t, uint16(23456), binary.BigEndian.Uint16(result[0:2])) // AS_TRANS
	require.Equal(t, []byte{192, 0, 2, 1}, result[2:6])
}

// TestAggregatorPackWithContext_ASN2SmallASN verifies small ASN passthrough.
//
// VALIDATES: Small ASNs (<=65535) pass through without AS_TRANS.
//
// PREVENTS: Unnecessary AS_TRANS substitution.
func TestAggregatorPackWithContext_ASN2SmallASN(t *testing.T) {
	agg := &attribute.Aggregator{
		ASN:     65001, // Small ASN
		Address: netip.MustParseAddr("192.0.2.1"),
	}

	result := agg.PackWithContext(nil, ctxASN2)

	require.Len(t, result, 6)
	require.Equal(t, uint16(65001), binary.BigEndian.Uint16(result[0:2]))
}

// =============================================================================
// Simple Attributes (no context dependency)
// =============================================================================

// TestOriginPackWithContext verifies Origin ignores context.
//
// VALIDATES: Origin returns same as Pack() regardless of context.
//
// PREVENTS: Unexpected context-dependent behavior.
func TestOriginPackWithContext(t *testing.T) {
	origin := attribute.OriginIGP

	result := origin.PackWithContext(ctxASN4, ctxASN2)
	expected := origin.Pack()

	require.Equal(t, expected, result)
}

// TestMEDPackWithContext verifies MED ignores context.
//
// VALIDATES: MED returns same as Pack() regardless of context.
//
// PREVENTS: Unexpected context-dependent behavior.
func TestMEDPackWithContext(t *testing.T) {
	med := attribute.MED(100)

	result := med.PackWithContext(ctxASN4, ctxASN2)
	expected := med.Pack()

	require.Equal(t, expected, result)
}

// TestLocalPrefPackWithContext verifies LocalPref ignores context.
//
// VALIDATES: LocalPref returns same as Pack() regardless of context.
//
// PREVENTS: Unexpected context-dependent behavior.
func TestLocalPrefPackWithContext(t *testing.T) {
	lp := attribute.LocalPref(100)

	result := lp.PackWithContext(ctxASN4, ctxASN2)
	expected := lp.Pack()

	require.Equal(t, expected, result)
}

// TestNextHopPackWithContext verifies NextHop ignores context.
//
// VALIDATES: NextHop returns same as Pack() regardless of context.
//
// PREVENTS: Unexpected context-dependent behavior.
func TestNextHopPackWithContext(t *testing.T) {
	nh := &attribute.NextHop{Addr: netip.MustParseAddr("192.0.2.1")}

	result := nh.PackWithContext(ctxASN4, ctxASN2)
	expected := nh.Pack()

	require.Equal(t, expected, result)
}

// TestAtomicAggregatePackWithContext verifies AtomicAggregate ignores context.
//
// VALIDATES: AtomicAggregate returns same as Pack() regardless of context.
//
// PREVENTS: Unexpected context-dependent behavior.
func TestAtomicAggregatePackWithContext(t *testing.T) {
	aa := attribute.AtomicAggregate{}

	result := aa.PackWithContext(ctxASN4, ctxASN2)
	expected := aa.Pack()

	require.Equal(t, expected, result)
}

// TestOriginatorIDPackWithContext verifies OriginatorID ignores context.
//
// VALIDATES: OriginatorID returns same as Pack() regardless of context.
//
// PREVENTS: Unexpected context-dependent behavior.
func TestOriginatorIDPackWithContext(t *testing.T) {
	oid := attribute.OriginatorID(netip.MustParseAddr("192.0.2.1"))

	result := oid.PackWithContext(ctxASN4, ctxASN2)
	expected := oid.Pack()

	require.Equal(t, expected, result)
}

// TestClusterListPackWithContext verifies ClusterList ignores context.
//
// VALIDATES: ClusterList returns same as Pack() regardless of context.
//
// PREVENTS: Unexpected context-dependent behavior.
func TestClusterListPackWithContext(t *testing.T) {
	cl := attribute.ClusterList{1, 2, 3}

	result := cl.PackWithContext(ctxASN4, ctxASN2)
	expected := cl.Pack()

	require.Equal(t, expected, result)
}
