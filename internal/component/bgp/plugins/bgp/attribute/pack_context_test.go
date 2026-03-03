package attribute_test

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/context"
)

// Test contexts for WriteToWithContext tests.
var (
	ctxASN4 = bgpctx.EncodingContextForASN4(true)
	ctxASN2 = bgpctx.EncodingContextForASN4(false)
)

// =============================================================================
// ASPath.WriteToWithContext Tests
// =============================================================================

// TestASPathWriteToWithContext_ASN4ToASN4 verifies 4-byte encoding passthrough.
//
// VALIDATES: 4-byte ASNs when both srcCtx.ASN4=true and dstCtx.ASN4=true.
//
// PREVENTS: Unnecessary transcoding when capabilities match.
func TestASPathWriteToWithContext_ASN4ToASN4(t *testing.T) {
	path := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002, 100000}},
		},
	}

	buf := make([]byte, 4096)
	n := path.WriteToWithContext(buf, 0, ctxASN4, ctxASN4)
	result := buf[:n]

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

// TestASPathWriteToWithContext_ASN4ToASN2 verifies 2-byte encoding with AS_TRANS.
//
// VALIDATES: Large ASNs become AS_TRANS (23456) when dstCtx.ASN4=false.
//
// PREVENTS: Protocol errors when sending to legacy peers.
func TestASPathWriteToWithContext_ASN4ToASN2(t *testing.T) {
	path := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 100000}}, // 100000 > 65535
		},
	}

	buf := make([]byte, 4096)
	n := path.WriteToWithContext(buf, 0, ctxASN4, ctxASN2)
	result := buf[:n]

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

// TestASPathWriteToWithContext_ASN2ToASN4 verifies upgrade path.
//
// VALIDATES: 4-byte encoding when dstCtx.ASN4=true regardless of source.
//
// PREVENTS: Wrong encoding after AS4_PATH merge.
func TestASPathWriteToWithContext_ASN2ToASN4(t *testing.T) {
	// After AS4_PATH merge, path contains real 4-byte ASNs
	path := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 200000}},
		},
	}

	buf := make([]byte, 4096)
	n := path.WriteToWithContext(buf, 0, ctxASN2, ctxASN4)
	result := buf[:n]

	// Expected: 4-byte encoding
	require.Len(t, result, 10) // type + count + 2*4
	require.Equal(t, uint32(200000), binary.BigEndian.Uint32(result[6:10]))
}

// TestASPathWriteToWithContext_NilDstCtx verifies default to ASN4.
//
// VALIDATES: nil dstCtx defaults to 4-byte ASN encoding.
//
// PREVENTS: Panic on nil context.
func TestASPathWriteToWithContext_NilDstCtx(t *testing.T) {
	path := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		},
	}

	buf := make([]byte, 4096)
	n := path.WriteToWithContext(buf, 0, nil, nil)
	result := buf[:n]

	// Should use 4-byte encoding (default)
	require.Len(t, result, 6) // type + count + 1*4
	require.Equal(t, uint32(65001), binary.BigEndian.Uint32(result[2:6]))
}

// =============================================================================
// Aggregator.WriteToWithContext Tests
// =============================================================================

// TestAggregatorWriteToWithContext_ASN4 verifies 8-byte format.
//
// VALIDATES: 8-byte format (4-byte ASN + 4-byte IP) when dstCtx.ASN4=true.
//
// PREVENTS: Parse errors on ASN4-capable peers.
func TestAggregatorWriteToWithContext_ASN4(t *testing.T) {
	agg := &attribute.Aggregator{
		ASN:     100000, // Large ASN
		Address: netip.MustParseAddr("192.0.2.1"),
	}

	buf := make([]byte, 4096)
	n := agg.WriteToWithContext(buf, 0, ctxASN4, ctxASN4)
	result := buf[:n]

	require.Len(t, result, 8)
	require.Equal(t, uint32(100000), binary.BigEndian.Uint32(result[0:4]))
	require.Equal(t, []byte{192, 0, 2, 1}, result[4:8])
}

// TestAggregatorWriteToWithContext_ASN2 verifies 6-byte format with AS_TRANS.
//
// VALIDATES: 6-byte format with AS_TRANS when dstCtx.ASN4=false and ASN > 65535.
//
// PREVENTS: Protocol errors when sending to legacy peers.
func TestAggregatorWriteToWithContext_ASN2(t *testing.T) {
	agg := &attribute.Aggregator{
		ASN:     100000, // Large ASN -> AS_TRANS
		Address: netip.MustParseAddr("192.0.2.1"),
	}

	buf := make([]byte, 4096)
	n := agg.WriteToWithContext(buf, 0, ctxASN4, ctxASN2)
	result := buf[:n]

	require.Len(t, result, 6)
	require.Equal(t, uint16(23456), binary.BigEndian.Uint16(result[0:2])) // AS_TRANS
	require.Equal(t, []byte{192, 0, 2, 1}, result[2:6])
}

// TestAggregatorWriteToWithContext_ASN2SmallASN verifies small ASN passthrough.
//
// VALIDATES: Small ASNs (<=65535) pass through without AS_TRANS.
//
// PREVENTS: Unnecessary AS_TRANS substitution.
func TestAggregatorWriteToWithContext_ASN2SmallASN(t *testing.T) {
	agg := &attribute.Aggregator{
		ASN:     65001, // Small ASN
		Address: netip.MustParseAddr("192.0.2.1"),
	}

	buf := make([]byte, 4096)
	n := agg.WriteToWithContext(buf, 0, nil, ctxASN2)
	result := buf[:n]

	require.Len(t, result, 6)
	require.Equal(t, uint16(65001), binary.BigEndian.Uint16(result[0:2]))
}

// =============================================================================
// Simple Attributes (no context dependency)
// =============================================================================

// TestOriginWriteToWithContext verifies Origin ignores context.
//
// VALIDATES: Origin returns same as WriteTo() regardless of context.
//
// PREVENTS: Unexpected context-dependent behavior.
func TestOriginWriteToWithContext(t *testing.T) {
	origin := attribute.OriginIGP

	buf := make([]byte, 4096)
	n := origin.WriteToWithContext(buf, 0, ctxASN4, ctxASN2)
	result := buf[:n]

	buf2 := make([]byte, 4096)
	n2 := origin.WriteTo(buf2, 0)
	expected := buf2[:n2]

	require.Equal(t, expected, result)
}

// TestMEDWriteToWithContext verifies MED ignores context.
//
// VALIDATES: MED returns same as WriteTo() regardless of context.
//
// PREVENTS: Unexpected context-dependent behavior.
func TestMEDWriteToWithContext(t *testing.T) {
	med := attribute.MED(100)

	buf := make([]byte, 4096)
	n := med.WriteToWithContext(buf, 0, ctxASN4, ctxASN2)
	result := buf[:n]

	buf2 := make([]byte, 4096)
	n2 := med.WriteTo(buf2, 0)
	expected := buf2[:n2]

	require.Equal(t, expected, result)
}

// TestLocalPrefWriteToWithContext verifies LocalPref ignores context.
//
// VALIDATES: LocalPref returns same as WriteTo() regardless of context.
//
// PREVENTS: Unexpected context-dependent behavior.
func TestLocalPrefWriteToWithContext(t *testing.T) {
	lp := attribute.LocalPref(100)

	buf := make([]byte, 4096)
	n := lp.WriteToWithContext(buf, 0, ctxASN4, ctxASN2)
	result := buf[:n]

	buf2 := make([]byte, 4096)
	n2 := lp.WriteTo(buf2, 0)
	expected := buf2[:n2]

	require.Equal(t, expected, result)
}

// TestNextHopWriteToWithContext verifies NextHop ignores context.
//
// VALIDATES: NextHop returns same as WriteTo() regardless of context.
//
// PREVENTS: Unexpected context-dependent behavior.
func TestNextHopWriteToWithContext(t *testing.T) {
	nh := &attribute.NextHop{Addr: netip.MustParseAddr("192.0.2.1")}

	buf := make([]byte, 4096)
	n := nh.WriteToWithContext(buf, 0, ctxASN4, ctxASN2)
	result := buf[:n]

	buf2 := make([]byte, 4096)
	n2 := nh.WriteTo(buf2, 0)
	expected := buf2[:n2]

	require.Equal(t, expected, result)
}

// TestAtomicAggregateWriteToWithContext verifies AtomicAggregate ignores context.
//
// VALIDATES: AtomicAggregate returns same as WriteTo() regardless of context.
//
// PREVENTS: Unexpected context-dependent behavior.
func TestAtomicAggregateWriteToWithContext(t *testing.T) {
	aa := attribute.AtomicAggregate{}

	buf := make([]byte, 4096)
	n := aa.WriteToWithContext(buf, 0, ctxASN4, ctxASN2)
	result := buf[:n]

	buf2 := make([]byte, 4096)
	n2 := aa.WriteTo(buf2, 0)
	expected := buf2[:n2]

	require.Equal(t, expected, result)
}

// TestOriginatorIDWriteToWithContext verifies OriginatorID ignores context.
//
// VALIDATES: OriginatorID returns same as WriteTo() regardless of context.
//
// PREVENTS: Unexpected context-dependent behavior.
func TestOriginatorIDWriteToWithContext(t *testing.T) {
	oid := attribute.OriginatorID(netip.MustParseAddr("192.0.2.1"))

	buf := make([]byte, 4096)
	n := oid.WriteToWithContext(buf, 0, ctxASN4, ctxASN2)
	result := buf[:n]

	buf2 := make([]byte, 4096)
	n2 := oid.WriteTo(buf2, 0)
	expected := buf2[:n2]

	require.Equal(t, expected, result)
}

// TestClusterListWriteToWithContext verifies ClusterList ignores context.
//
// VALIDATES: ClusterList returns same as WriteTo() regardless of context.
//
// PREVENTS: Unexpected context-dependent behavior.
func TestClusterListWriteToWithContext(t *testing.T) {
	cl := attribute.ClusterList{1, 2, 3}

	buf := make([]byte, 4096)
	n := cl.WriteToWithContext(buf, 0, ctxASN4, ctxASN2)
	result := buf[:n]

	buf2 := make([]byte, 4096)
	n2 := cl.WriteTo(buf2, 0)
	expected := buf2[:n2]

	require.Equal(t, expected, result)
}
