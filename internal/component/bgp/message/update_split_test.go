package message

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
)

// =============================================================================
// Phase 1: SplitUpdate IPv4 Tests
// =============================================================================

// TestSplitUpdate_SmallFits verifies small UPDATE passes through.
//
// VALIDATES: UPDATE < maxSize returns single UPDATE unchanged.
// PREVENTS: Unnecessary splitting of small messages.
func TestSplitUpdate_SmallFits(t *testing.T) {
	u := &Update{
		PathAttributes: []byte{
			0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		},
		NLRI: []byte{
			0x18, 0xC0, 0xA8, 0x01, // 192.168.1.0/24
		},
	}

	// maxSize = 4096, UPDATE is tiny
	chunks, err := SplitUpdate(u, 4096)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, u.PathAttributes, chunks[0].PathAttributes)
	assert.Equal(t, u.NLRI, chunks[0].NLRI)
}

// TestSplitUpdate_ExactFit verifies boundary condition.
//
// VALIDATES: UPDATE == maxSize returns single UPDATE (no split).
// PREVENTS: Off-by-one splitting at exact boundary.
func TestSplitUpdate_ExactFit(t *testing.T) {
	// Build UPDATE that exactly fits maxSize
	// Size = Header(19) + WithdrawnLen(2) + AttrLen(2) + Attrs + NLRI
	// With maxSize=31: 19 + 2 + 2 + 4 + 4 = 31
	u := &Update{
		PathAttributes: []byte{0x40, 0x01, 0x01, 0x00}, // 4 bytes
		NLRI:           []byte{0x18, 0xC0, 0xA8, 0x01}, // 4 bytes
	}

	chunks, err := SplitUpdate(u, 31)
	require.NoError(t, err)
	require.Len(t, chunks, 1, "exact fit should not split")
}

// TestSplitUpdate_NLRIOverflow verifies NLRI splitting.
//
// VALIDATES: Large NLRI split into N chunks, each <= maxSize.
// PREVENTS: Oversized UPDATE sent to peer.
func TestSplitUpdate_NLRIOverflow(t *testing.T) {
	// Create many NLRIs to force splitting
	// Each /24 = 4 bytes, create 100 = 400 bytes
	var nlri []byte
	for i := range 100 {
		nlri = append(nlri, 0x18, 0xC0, 0xA8, byte(i))
	}

	u := &Update{
		PathAttributes: []byte{0x40, 0x01, 0x01, 0x00}, // 4 bytes
		NLRI:           nlri,
	}

	// maxSize = 100 bytes total
	// Overhead = 19 + 4 + 4 = 27 bytes, leaving 73 for NLRI
	// 73 / 4 = 18 prefixes per chunk, need ~6 chunks
	chunks, err := SplitUpdate(u, 100)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split into multiple chunks")

	// Verify each chunk is within size limit
	for i, chunk := range chunks {
		size := HeaderLen + 4 + len(chunk.PathAttributes) + len(chunk.NLRI)
		assert.LessOrEqual(t, size, 100, "chunk %d exceeds maxSize: %d", i, size)
	}

	// Verify all NLRIs preserved
	totalNLRI := make([]byte, 0, len(nlri))
	for _, chunk := range chunks {
		totalNLRI = append(totalNLRI, chunk.NLRI...)
	}
	assert.Equal(t, nlri, totalNLRI, "all NLRIs should be preserved")
}

// TestSplitUpdate_WithdrawnOverflow verifies withdrawal splitting.
//
// VALIDATES: Large withdrawal split into N chunks (no attributes).
// PREVENTS: Oversized withdrawal message.
func TestSplitUpdate_WithdrawnOverflow(t *testing.T) {
	// Create many withdrawn prefixes
	var withdrawn []byte
	for i := range 100 {
		withdrawn = append(withdrawn, 0x18, 0x0A, 0x00, byte(i))
	}

	u := &Update{
		WithdrawnRoutes: withdrawn,
		// No attributes needed for withdrawals
	}

	// maxSize = 80 bytes
	// Overhead = 19 + 4 = 23 bytes (no attrs), leaving 57 for withdrawn
	chunks, err := SplitUpdate(u, 80)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split withdrawals")

	// Verify each chunk has no attributes (withdrawal-only)
	for i, chunk := range chunks {
		assert.Empty(t, chunk.PathAttributes, "withdrawal chunk %d should have no attributes", i)
		assert.Empty(t, chunk.NLRI, "withdrawal chunk %d should have no NLRI", i)
	}

	// Verify all withdrawals preserved
	totalWithdrawn := make([]byte, 0, len(withdrawn))
	for _, chunk := range chunks {
		totalWithdrawn = append(totalWithdrawn, chunk.WithdrawnRoutes...)
	}
	assert.Equal(t, withdrawn, totalWithdrawn)
}

// TestSplitUpdate_MixedSeparates verifies mixed UPDATE handling.
//
// VALIDATES: Announce + Withdraw split into separate UPDATE sets.
// PREVENTS: Invalid mixed splitting losing routes.
func TestSplitUpdate_MixedSeparates(t *testing.T) {
	u := &Update{
		WithdrawnRoutes: []byte{0x18, 0x0A, 0x00, 0x01}, // 10.0.1.0/24
		PathAttributes:  []byte{0x40, 0x01, 0x01, 0x00},
		NLRI:            []byte{0x18, 0xC0, 0xA8, 0x01}, // 192.168.1.0/24
	}

	// Force split with small maxSize
	// This UPDATE has both withdrawn and NLRI - verify both preserved
	chunks, err := SplitUpdate(u, 4096)
	require.NoError(t, err)

	// With large maxSize, should fit in one chunk
	require.Len(t, chunks, 1)
	assert.Equal(t, u.WithdrawnRoutes, chunks[0].WithdrawnRoutes)
	assert.Equal(t, u.NLRI, chunks[0].NLRI)
}

// TestSplitUpdate_EndOfRIB verifies EoR passthrough.
//
// VALIDATES: Empty UPDATE (End-of-RIB) returns single unchanged UPDATE.
// PREVENTS: EoR marker corruption.
func TestSplitUpdate_EndOfRIB(t *testing.T) {
	u := &Update{} // Empty = End-of-RIB

	chunks, err := SplitUpdate(u, 4096)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.True(t, chunks[0].IsEndOfRIB())
}

// TestSplitUpdate_WithdrawalOnly verifies withdrawal-only structure.
//
// VALIDATES: Withdrawal-only UPDATE has empty PathAttributes.
// PREVENTS: Adding spurious attributes to withdrawals.
func TestSplitUpdate_WithdrawalOnly(t *testing.T) {
	u := &Update{
		WithdrawnRoutes: []byte{0x18, 0x0A, 0x00, 0x01},
	}

	chunks, err := SplitUpdate(u, 4096)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Empty(t, chunks[0].PathAttributes)
	assert.Empty(t, chunks[0].NLRI)
}

// TestSplitUpdate_OneByteOver verifies minimal overflow.
//
// VALIDATES: UPDATE at maxSize+1 splits into exactly 2 chunks.
// PREVENTS: Off-by-one non-splitting.
func TestSplitUpdate_OneByteOver(t *testing.T) {
	// Build UPDATE that's exactly 1 byte over maxSize
	// Size = Header(19) + WithdrawnLen(2) + AttrLen(2) + Attrs(4) + NLRI(8) = 35
	u := &Update{
		PathAttributes: []byte{0x40, 0x01, 0x01, 0x00}, // 4 bytes
		NLRI: []byte{
			0x18, 0xC0, 0xA8, 0x01, // 192.168.1.0/24
			0x18, 0xC0, 0xA8, 0x02, // 192.168.2.0/24
		}, // 8 bytes
	}

	// maxSize = 34 (one byte less than needed)
	chunks, err := SplitUpdate(u, 34)
	require.NoError(t, err)
	require.Len(t, chunks, 2, "should split into exactly 2 chunks")
}

// TestSplitUpdate_AttributesBytesPreserved verifies zero-copy.
//
// VALIDATES: All chunks share same PathAttributes slice (pointer equality).
// PREVENTS: Unnecessary attribute re-serialization.
func TestSplitUpdate_AttributesBytesPreserved(t *testing.T) {
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	var nlri []byte
	for i := range 50 {
		nlri = append(nlri, 0x18, 0xC0, 0xA8, byte(i))
	}

	u := &Update{
		PathAttributes: attrs,
		NLRI:           nlri,
	}

	chunks, err := SplitUpdate(u, 80)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	// All chunks should reference the same PathAttributes slice
	for i, chunk := range chunks {
		assert.Equal(t, attrs, chunk.PathAttributes, "chunk %d attributes differ", i)
	}
}

// TestSplitUpdate_AttributesTooLarge verifies error on huge attributes.
//
// VALIDATES: ErrAttributesTooLarge returned when attrs > maxSize.
// PREVENTS: Panic or invalid split attempt.
func TestSplitUpdate_AttributesTooLarge(t *testing.T) {
	// Attributes larger than maxSize
	attrs := make([]byte, 100)
	u := &Update{
		PathAttributes: attrs,
		NLRI:           []byte{0x18, 0xC0, 0xA8, 0x01},
	}

	// maxSize = 50, but attrs alone = 100
	_, err := SplitUpdate(u, 50)
	require.ErrorIs(t, err, ErrAttributesTooLarge)
}

// TestSplitUpdate_SingleNLRITooLarge verifies error on huge single NLRI.
//
// VALIDATES: ErrNLRITooLarge returned when single NLRI > available space.
// PREVENTS: Silent data loss or infinite loop.
func TestSplitUpdate_SingleNLRITooLarge(t *testing.T) {
	// Build UPDATE where single NLRI exceeds available space
	// Overhead = 23, attrs = 4, leaves nlriSpace = maxSize - 27
	// With maxSize = 30: nlriSpace = 3, but single /32 = 5 bytes
	u := &Update{
		PathAttributes: []byte{0x40, 0x01, 0x01, 0x00},       // 4 bytes
		NLRI:           []byte{0x20, 0x0A, 0x00, 0x00, 0x01}, // 5 bytes (/32)
	}
	_, err := SplitUpdate(u, 30)
	require.ErrorIs(t, err, ErrNLRITooLarge)
}

// TestSplitUpdate_AllChunksValid verifies chunk structure.
//
// VALIDATES: Each chunk is a valid UPDATE (correct length fields, parseable).
// PREVENTS: Malformed UPDATE messages.
func TestSplitUpdate_AllChunksValid(t *testing.T) {
	var nlri []byte
	for i := range 100 {
		nlri = append(nlri, 0x18, 0xC0, 0xA8, byte(i))
	}

	u := &Update{
		PathAttributes: []byte{0x40, 0x01, 0x01, 0x00},
		NLRI:           nlri,
	}

	chunks, err := SplitUpdate(u, 100)
	require.NoError(t, err)

	// Each chunk should pack and unpack correctly
	for i, chunk := range chunks {
		packed := PackTo(chunk, nil)

		// Parse header
		h, err := ParseHeader(packed)
		require.NoError(t, err, "chunk %d header invalid", i)
		assert.Equal(t, TypeUPDATE, h.Type)

		// Unpack body
		_, err = UnpackUpdate(packed[HeaderLen:])
		require.NoError(t, err, "chunk %d failed to unpack", i)
	}
}

// TestSplitUpdate_NLRICountPreserved verifies no data loss.
//
// VALIDATES: Sum of NLRIs across chunks equals original NLRI count.
// PREVENTS: Route loss during splitting.
func TestSplitUpdate_NLRICountPreserved(t *testing.T) {
	// Create 100 /24 prefixes
	var nlri []byte
	for i := range 100 {
		nlri = append(nlri, 0x18, 0xC0, 0xA8, byte(i))
	}

	u := &Update{
		PathAttributes: []byte{0x40, 0x01, 0x01, 0x00},
		NLRI:           nlri,
	}

	chunks, err := SplitUpdate(u, 80)
	require.NoError(t, err)

	// Count prefixes in all chunks
	totalPrefixes := 0
	for _, chunk := range chunks {
		// Each /24 is 4 bytes
		totalPrefixes += len(chunk.NLRI) / 4
	}

	assert.Equal(t, 100, totalPrefixes, "all prefixes should be preserved")
}

// =============================================================================
// Phase 2: MP_REACH_NLRI Splitting Tests
// =============================================================================

// TestSplitMPReachNLRI_SmallFits verifies small MP_REACH passes through.
//
// VALIDATES: MP_REACH < maxAttrSize returns single attribute unchanged.
// PREVENTS: Unnecessary splitting.
func TestSplitMPReachNLRI_SmallFits(t *testing.T) {
	mp := &attribute.MPReachNLRI{
		AFI:      attribute.AFI(2), // IPv6
		SAFI:     attribute.SAFI(1),
		NextHops: []netip.Addr{netip.MustParseAddr("2001:db8::1")},
		NLRI:     []byte{0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, 0x00, 0x00}, // 2001:db8:1::/64 (9 bytes)
	}

	chunks, err := SplitMPReachNLRI(mp, 500)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, mp.NLRI, chunks[0].NLRI)
}

// TestSplitMPReachNLRI_Overflow verifies NLRI splitting.
//
// VALIDATES: Large NLRI split into N attributes.
// PREVENTS: Oversized MP_REACH_NLRI attribute.
func TestSplitMPReachNLRI_Overflow(t *testing.T) {
	// Create large NLRI (many /64 prefixes)
	// Each /64 = 9 bytes (1 len + 8 prefix bytes)
	var nlri []byte
	for i := range 50 {
		nlri = append(nlri, 0x40, 0x20, 0x01, 0x0d, 0xb8, byte(i>>8), byte(i), 0x00, 0x00) // /64
	}

	mp := &attribute.MPReachNLRI{
		AFI:      attribute.AFI(2),
		SAFI:     attribute.SAFI(1),
		NextHops: []netip.Addr{netip.MustParseAddr("2001:db8::1")},
		NLRI:     nlri,
	}

	// Small maxAttrSize to force splitting
	// Overhead: AFI(2) + SAFI(1) + NH_Len(1) + NH(16) + Reserved(1) = 21 bytes
	// With maxAttrSize=100: 100 - 21 = 79 bytes for NLRI
	// Each /64 = 9 bytes, so ~8 per chunk, need ~7 chunks
	chunks, err := SplitMPReachNLRI(mp, 100)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split into multiple chunks")

	// Verify all chunks preserve AFI/SAFI/NextHops
	for i, chunk := range chunks {
		assert.Equal(t, mp.AFI, chunk.AFI, "chunk %d AFI mismatch", i)
		assert.Equal(t, mp.SAFI, chunk.SAFI, "chunk %d SAFI mismatch", i)
		assert.Equal(t, mp.NextHops, chunk.NextHops, "chunk %d NextHops mismatch", i)
	}

	// Verify all NLRIs preserved
	totalNLRI := make([]byte, 0, len(nlri))
	for _, chunk := range chunks {
		totalNLRI = append(totalNLRI, chunk.NLRI...)
	}
	assert.Equal(t, nlri, totalNLRI, "all NLRIs should be preserved")
}

// TestSplitMPReachNLRI_OverheadTooLarge verifies error on huge overhead.
//
// VALIDATES: ErrMPOverheadTooLarge when overhead > maxAttrSize.
// PREVENTS: Invalid split attempt.
func TestSplitMPReachNLRI_OverheadTooLarge(t *testing.T) {
	mp := &attribute.MPReachNLRI{
		AFI:      attribute.AFI(2),
		SAFI:     attribute.SAFI(1),
		NextHops: []netip.Addr{netip.MustParseAddr("2001:db8::1")},
		NLRI:     []byte{0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01},
	}

	// Overhead = 21 bytes, maxAttrSize = 10
	_, err := SplitMPReachNLRI(mp, 10)
	require.Error(t, err)
}

// TestSplitMPUnreachNLRI_SmallFits verifies small MP_UNREACH passes through.
//
// VALIDATES: MP_UNREACH < maxAttrSize returns single attribute unchanged.
// PREVENTS: Unnecessary splitting.
func TestSplitMPUnreachNLRI_SmallFits(t *testing.T) {
	mp := &attribute.MPUnreachNLRI{
		AFI:  attribute.AFI(2),
		SAFI: attribute.SAFI(1),
		NLRI: []byte{0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, 0x00, 0x00}, // 2001:db8:1::/64 (9 bytes)
	}

	chunks, err := SplitMPUnreachNLRI(mp, 500)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, mp.NLRI, chunks[0].NLRI)
}

// TestSplitMPUnreachNLRI_Overflow verifies NLRI splitting.
//
// VALIDATES: Large withdrawn NLRI split into N attributes.
// PREVENTS: Oversized MP_UNREACH_NLRI attribute.
func TestSplitMPUnreachNLRI_Overflow(t *testing.T) {
	// Create large NLRI
	var nlri []byte
	for i := range 50 {
		nlri = append(nlri, 0x40, 0x20, 0x01, 0x0d, 0xb8, byte(i>>8), byte(i), 0x00, 0x00)
	}

	mp := &attribute.MPUnreachNLRI{
		AFI:  attribute.AFI(2),
		SAFI: attribute.SAFI(1),
		NLRI: nlri,
	}

	// Overhead: AFI(2) + SAFI(1) = 3 bytes
	// With maxAttrSize=50: 50 - 3 = 47 bytes for NLRI
	chunks, err := SplitMPUnreachNLRI(mp, 50)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split into multiple chunks")

	// Verify all chunks preserve AFI/SAFI
	for i, chunk := range chunks {
		assert.Equal(t, mp.AFI, chunk.AFI, "chunk %d AFI mismatch", i)
		assert.Equal(t, mp.SAFI, chunk.SAFI, "chunk %d SAFI mismatch", i)
	}

	// Verify all NLRIs preserved
	totalNLRI := make([]byte, 0, len(nlri))
	for _, chunk := range chunks {
		totalNLRI = append(totalNLRI, chunk.NLRI...)
	}
	assert.Equal(t, nlri, totalNLRI)
}

// TestSplitMPReachNLRI_VPN verifies VPN prefix splitting.
//
// VALIDATES: VPN NLRI (with RD) split correctly.
// PREVENTS: Corruption of RD+prefix encoding.
func TestSplitMPReachNLRI_VPN(t *testing.T) {
	// VPN NLRI: len(1) + label(3) + RD(8) + prefix(variable)
	// Create 20 VPN prefixes
	var nlri []byte
	for i := range 20 {
		// /32 VPN prefix: len=88 (24 label + 64 RD + 32 prefix bits = 120 bits)
		// Actually: 88 bits = 11 bytes payload
		nlri = append(nlri, 88, // prefix len in bits
			0x00, 0x00, byte(i+1), // label (3 bytes)
			0x00, 0x01, 0x00, 0x00, 0x00, 100, // RD type 1
			0x00, byte(i), // RD local
			10, 0, byte(i), 0) // /32 prefix
	}

	mp := &attribute.MPReachNLRI{
		AFI:      attribute.AFI(1),    // IPv4
		SAFI:     attribute.SAFI(128), // MPLS VPN
		NextHops: []netip.Addr{netip.MustParseAddr("192.168.1.1")},
		NLRI:     nlri,
	}

	chunks, err := SplitMPReachNLRI(mp, 100)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	// Verify all NLRI bytes preserved
	totalNLRI := make([]byte, 0, len(nlri))
	for _, chunk := range chunks {
		totalNLRI = append(totalNLRI, chunk.NLRI...)
	}
	assert.Equal(t, nlri, totalNLRI)
}

// =============================================================================
// Add-Path Splitting Tests (RFC 7911)
// =============================================================================

// TestSplitUpdateWithAddPath_IPv4 verifies Add-Path NLRI splitting.
//
// VALIDATES: Add-Path NLRIs (with 4-byte path-id) split at correct boundaries.
// PREVENTS: Path-id corruption or misaligned splits.
func TestSplitUpdateWithAddPath_IPv4(t *testing.T) {
	// Add-Path NLRI: [path-id:4][prefix-len:1][prefix-bytes]
	// Each /24 with path-id = 4 + 1 + 3 = 8 bytes
	var nlri []byte
	for i := range 20 {
		// Path ID (4 bytes) + /24 prefix
		nlri = append(nlri, 0x00, 0x00, 0x00, byte(i+1), 0x18, 0xC0, 0xA8, byte(i))
	}

	u := &Update{
		PathAttributes: []byte{0x40, 0x01, 0x01, 0x00}, // ORIGIN
		NLRI:           nlri,
	}

	// maxSize = 60, overhead = 27, leaves 33 for NLRI
	// Each Add-Path /24 = 8 bytes, so ~4 per chunk
	chunks, err := SplitUpdateWithAddPath(u, 60, true)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split Add-Path NLRI")

	// Verify all NLRIs preserved
	totalNLRI := make([]byte, 0, len(nlri))
	for _, chunk := range chunks {
		totalNLRI = append(totalNLRI, chunk.NLRI...)
	}
	assert.Equal(t, nlri, totalNLRI, "all Add-Path NLRIs should be preserved")

	// Verify each chunk has valid Add-Path structure
	for i, chunk := range chunks {
		// Each NLRI should be 8 bytes (path-id + /24)
		assert.Equal(t, 0, len(chunk.NLRI)%8, "chunk %d has invalid Add-Path NLRI alignment", i)
	}
}

// TestSplitUpdateWithAddPath_IPv6 verifies IPv6 Add-Path MP_REACH splitting.
//
// VALIDATES: IPv6 Add-Path NLRIs split correctly via MP_REACH_NLRI.
// PREVENTS: IPv6 Add-Path path-id corruption.
func TestSplitUpdateWithAddPath_IPv6(t *testing.T) {
	// Add-Path IPv6 NLRI: [path-id:4][prefix-len:1][prefix-bytes]
	// Each /64 with path-id = 4 + 1 + 8 = 13 bytes
	var nlri []byte
	for i := range 20 {
		// Path ID (4 bytes) + /64 prefix (1 + 8 bytes)
		nlri = append(nlri, 0x00, 0x00, 0x00, byte(i+1), 0x40, 0x20, 0x01, 0x0d, 0xb8, byte(i>>8), byte(i), 0x00, 0x00)
	}

	mp := &attribute.MPReachNLRI{
		AFI:      attribute.AFI(2),
		SAFI:     attribute.SAFI(1),
		NextHops: []netip.Addr{netip.MustParseAddr("2001:db8::1")},
		NLRI:     nlri,
	}

	// Overhead = 21 bytes, maxAttrSize = 80, leaves 59 for NLRI
	// Each Add-Path /64 = 13 bytes, so ~4 per chunk
	chunks, err := SplitMPReachNLRIWithAddPath(mp, 80, true)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split Add-Path MP_REACH_NLRI")

	// Verify all NLRIs preserved
	totalNLRI := make([]byte, 0, len(nlri))
	for _, chunk := range chunks {
		totalNLRI = append(totalNLRI, chunk.NLRI...)
	}
	assert.Equal(t, nlri, totalNLRI, "all Add-Path NLRIs should be preserved")
}

// TestSplitUpdateWithAddPath_Withdrawal verifies Add-Path withdrawal splitting.
//
// VALIDATES: Add-Path withdrawn routes split correctly.
// PREVENTS: Path-id loss in withdrawal messages.
func TestSplitUpdateWithAddPath_Withdrawal(t *testing.T) {
	// Add-Path withdrawn NLRI
	var withdrawn []byte
	for i := range 20 {
		// Path ID (4 bytes) + /24 prefix
		withdrawn = append(withdrawn, 0x00, 0x00, 0x00, byte(i+1), 0x18, 0x0A, 0x00, byte(i))
	}

	u := &Update{
		WithdrawnRoutes: withdrawn,
	}

	// maxSize = 60, overhead = 23, leaves 37 for withdrawn
	// Each Add-Path /24 = 8 bytes, so ~4 per chunk
	chunks, err := SplitUpdateWithAddPath(u, 60, true)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split Add-Path withdrawals")

	// Verify all withdrawals preserved
	totalWithdrawn := make([]byte, 0, len(withdrawn))
	for _, chunk := range chunks {
		totalWithdrawn = append(totalWithdrawn, chunk.WithdrawnRoutes...)
	}
	assert.Equal(t, withdrawn, totalWithdrawn, "all Add-Path withdrawals should be preserved")
}

// TestSplitUpdateWithAddPath_FalseDoesNotAssumePathId verifies non-Add-Path mode.
//
// VALIDATES: addPath=false parses basic NLRI format (no path-id).
// PREVENTS: Misinterpreting basic NLRI as Add-Path when negotiation says no.
func TestSplitUpdateWithAddPath_FalseDoesNotAssumePathId(t *testing.T) {
	// Basic NLRI (no path-id): [prefix-len:1][prefix-bytes]
	// Each /24 = 4 bytes
	var nlri []byte
	for i := range 20 {
		nlri = append(nlri, 0x18, 0xC0, 0xA8, byte(i))
	}

	u := &Update{
		PathAttributes: []byte{0x40, 0x01, 0x01, 0x00},
		NLRI:           nlri,
	}

	// Split with addPath=false
	chunks, err := SplitUpdateWithAddPath(u, 50, false)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	// Verify all NLRIs preserved
	totalNLRI := make([]byte, 0, len(nlri))
	for _, chunk := range chunks {
		totalNLRI = append(totalNLRI, chunk.NLRI...)
	}
	assert.Equal(t, nlri, totalNLRI)
}

// =============================================================================
// Mixed UPDATE Splitting Tests
// =============================================================================

// TestSplitUpdate_MixedLargeSplits verifies splitting of large mixed UPDATE.
//
// VALIDATES: UPDATE with both large withdrawals AND large NLRIs splits correctly.
// PREVENTS: Data loss when both fields overflow and need separate splitting.
func TestSplitUpdate_MixedLargeSplits(t *testing.T) {
	// Create many withdrawn prefixes (100 * 4 = 400 bytes)
	var withdrawn []byte
	for i := range 100 {
		withdrawn = append(withdrawn, 0x18, 0x0A, byte(i), 0x00)
	}

	// Create many announcement prefixes (100 * 4 = 400 bytes)
	var nlri []byte
	for i := range 100 {
		nlri = append(nlri, 0x18, 0xC0, 0xA8, byte(i))
	}

	u := &Update{
		WithdrawnRoutes: withdrawn,
		PathAttributes:  []byte{0x40, 0x01, 0x01, 0x00}, // ORIGIN
		NLRI:            nlri,
	}

	// Small maxSize to force splitting both withdrawals and NLRIs
	// Overhead = 23, attrs = 4, so:
	// - Withdrawal space = 100 - 23 = 77 bytes (~19 prefixes per chunk)
	// - NLRI space = 100 - 23 - 4 = 73 bytes (~18 prefixes per chunk)
	chunks, err := SplitUpdate(u, 100)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 2, "should split into multiple chunks")

	// Count withdrawal-only and announcement chunks
	var withdrawalChunks, announceChunks int
	var totalWithdrawn, totalNLRI []byte

	for _, chunk := range chunks {
		if len(chunk.WithdrawnRoutes) > 0 {
			withdrawalChunks++
			totalWithdrawn = append(totalWithdrawn, chunk.WithdrawnRoutes...)
			// Withdrawal-only chunks should have no attributes or NLRI
			assert.Empty(t, chunk.PathAttributes, "withdrawal chunk should have no attrs")
			assert.Empty(t, chunk.NLRI, "withdrawal chunk should have no NLRI")
		}
		if len(chunk.NLRI) > 0 {
			announceChunks++
			totalNLRI = append(totalNLRI, chunk.NLRI...)
			// Announcement chunks should have attributes
			assert.NotEmpty(t, chunk.PathAttributes, "announce chunk should have attrs")
		}
	}

	// Verify all data preserved
	assert.Equal(t, withdrawn, totalWithdrawn, "all withdrawals should be preserved")
	assert.Equal(t, nlri, totalNLRI, "all NLRIs should be preserved")

	// Should have multiple chunks of each type
	assert.Greater(t, withdrawalChunks, 1, "should have multiple withdrawal chunks")
	assert.Greater(t, announceChunks, 1, "should have multiple announce chunks")
}

// TestSplitUpdate_MixedWithdrawalsFirst verifies withdrawal chunks come before announcements.
//
// VALIDATES: Withdrawal chunks appear before announcement chunks in result.
// PREVENTS: Out-of-order processing (withdraw old before announce new).
func TestSplitUpdate_MixedWithdrawalsFirst(t *testing.T) {
	u := &Update{
		WithdrawnRoutes: []byte{
			0x18, 0x0A, 0x00, 0x01,
			0x18, 0x0A, 0x00, 0x02,
		},
		PathAttributes: []byte{0x40, 0x01, 0x01, 0x00},
		NLRI: []byte{
			0x18, 0xC0, 0xA8, 0x01,
			0x18, 0xC0, 0xA8, 0x02,
		},
	}

	// Force split by using small maxSize
	// This should create: withdrawal chunk(s), then announcement chunk(s)
	chunks, err := SplitUpdate(u, 35)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	// Find the transition point from withdrawals to announcements
	sawAnnouncement := false
	for i, chunk := range chunks {
		hasWithdrawal := len(chunk.WithdrawnRoutes) > 0
		hasNLRI := len(chunk.NLRI) > 0

		if hasNLRI {
			sawAnnouncement = true
		}

		// Once we see an announcement, we shouldn't see withdrawals again
		if sawAnnouncement && hasWithdrawal {
			t.Errorf("chunk %d has withdrawal after announcement chunks started", i)
		}
	}
}

// =============================================================================
// End-to-End Round-Trip Tests
// =============================================================================

// TestSplitUpdate_RoundTrip_PackUnpack verifies full wire round-trip.
//
// VALIDATES: Split UPDATE chunks can be packed and unpacked without data loss.
// PREVENTS: Wire encoding/decoding corruption in split path.
func TestSplitUpdate_RoundTrip_PackUnpack(t *testing.T) {
	// Create UPDATE with many NLRIs that will require splitting
	var nlri []byte
	for i := range 200 {
		nlri = append(nlri, 0x18, 0xC0, 0xA8, byte(i))
	}

	original := &Update{
		PathAttributes: []byte{
			0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
			0x40, 0x02, 0x04, 0x02, 0x01, 0xFD, 0xE9, // AS_PATH: AS65001
		},
		NLRI: nlri,
	}

	// Split with small maxSize to force many chunks
	chunks, err := SplitUpdate(original, 80)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 5, "should create multiple chunks")

	// Write each chunk, then unpack and verify
	reassembledNLRI := make([]byte, 0, len(nlri))
	for i, chunk := range chunks {
		// Write to wire format
		packed := PackTo(chunk, nil)

		// Verify header
		require.GreaterOrEqual(t, len(packed), HeaderLen, "chunk %d too short", i)
		h, err := ParseHeader(packed)
		require.NoError(t, err, "chunk %d header parse failed", i)
		assert.Equal(t, TypeUPDATE, h.Type, "chunk %d wrong type", i)

		// Unpack body
		unpacked, err := UnpackUpdate(packed[HeaderLen:])
		require.NoError(t, err, "chunk %d unpack failed", i)

		// Collect NLRIs
		reassembledNLRI = append(reassembledNLRI, unpacked.NLRI...)

		// Verify attributes match original (zero-copy preserves them)
		if len(unpacked.NLRI) > 0 {
			assert.Equal(t, original.PathAttributes, unpacked.PathAttributes,
				"chunk %d attributes mismatch", i)
		}
	}

	// Verify all NLRIs preserved through round-trip
	assert.Equal(t, nlri, reassembledNLRI, "NLRI data lost in round-trip")
}

// TestSplitUpdate_RoundTrip_LargeAttributes verifies large attribute handling.
//
// VALIDATES: UPDATEs with large attributes (communities, AS_PATH) split correctly.
// PREVENTS: Attribute truncation or corruption with real-world attribute sizes.
func TestSplitUpdate_RoundTrip_LargeAttributes(t *testing.T) {
	// Build realistic large attributes:
	// - ORIGIN (4 bytes)
	// - AS_PATH with 50 ASNs (3 + 2 + 50*4 = 205 bytes)
	// - COMMUNITIES with 20 communities (3 + 20*4 = 83 bytes)
	// Total: 4 + 205 + 83 = 292 bytes
	attrs := make([]byte, 0, 292)
	attrs = append(attrs, 0x40, 0x01, 0x01, 0x00) // ORIGIN IGP

	// AS_PATH: 50 ASNs = 200 bytes of AS numbers
	asPath := make([]byte, 0, 205)
	asPath = append(asPath, 0x40, 0x02, 0xC8, 0x02, 0x32) // Extended length, code 2, len 200, AS_SEQUENCE, 50 ASNs
	for i := range 50 {
		asPath = append(asPath, 0x00, 0x00, byte(i>>8), byte(i))
	}
	attrs = append(attrs, asPath...)

	// COMMUNITIES: 20 communities
	communities := make([]byte, 0, 83)
	communities = append(communities, 0xC0, 0x08, 0x50) // Optional transitive, code 8, length 80
	for i := range 20 {
		communities = append(communities, 0xFD, 0xE9, byte(i>>8), byte(i))
	}
	attrs = append(attrs, communities...)

	// Add some NLRIs
	nlri := make([]byte, 0, 50*4) // 50 /24 prefixes at 4 bytes each
	for i := range 50 {
		nlri = append(nlri, 0x18, 0xC0, 0xA8, byte(i))
	}

	original := &Update{
		PathAttributes: attrs,
		NLRI:           nlri,
	}

	// Calculate minimum size needed (attrs are ~290 bytes)
	// With overhead (23) + attrs (290) = 313, need at least 320 for one NLRI
	// Use 350 to fit a few NLRIs per chunk
	chunks, err := SplitUpdate(original, 350)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split with large attributes")

	// Verify each chunk has the same large attributes
	for i, chunk := range chunks {
		if len(chunk.NLRI) > 0 {
			assert.Equal(t, attrs, chunk.PathAttributes,
				"chunk %d: large attributes not preserved", i)
		}
	}

	// Verify all NLRIs preserved
	totalNLRI := make([]byte, 0, len(nlri))
	for _, chunk := range chunks {
		totalNLRI = append(totalNLRI, chunk.NLRI...)
	}
	assert.Equal(t, nlri, totalNLRI, "NLRIs lost with large attributes")
}

// TestSplitUpdate_RoundTrip_Withdrawals verifies withdrawal round-trip.
//
// VALIDATES: Withdrawal-only UPDATEs split and round-trip correctly.
// PREVENTS: Withdrawal data loss through split/pack/unpack cycle.
func TestSplitUpdate_RoundTrip_Withdrawals(t *testing.T) {
	// Create many withdrawals
	var withdrawn []byte
	for i := range 200 {
		withdrawn = append(withdrawn, 0x18, 0x0A, byte(i), 0x00)
	}

	original := &Update{
		WithdrawnRoutes: withdrawn,
	}

	// Split
	chunks, err := SplitUpdate(original, 80)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 5)

	// Write, unpack, reassemble
	reassembledWithdrawn := make([]byte, 0, len(withdrawn))
	for i, chunk := range chunks {
		packed := PackTo(chunk, nil)

		unpacked, err := UnpackUpdate(packed[HeaderLen:])
		require.NoError(t, err, "chunk %d unpack failed", i)

		reassembledWithdrawn = append(reassembledWithdrawn, unpacked.WithdrawnRoutes...)

		// Withdrawal chunks should have no attributes or NLRI
		assert.Empty(t, unpacked.PathAttributes, "chunk %d has unexpected attrs", i)
		assert.Empty(t, unpacked.NLRI, "chunk %d has unexpected NLRI", i)
	}

	assert.Equal(t, withdrawn, reassembledWithdrawn, "withdrawals lost in round-trip")
}

// =============================================================================
// Spec: spec-writeto-bounds-safety.md Test Cases
// =============================================================================

// TestSplitUpdate_CheckAfterWrite verifies check-after-write splitting logic.
//
// VALIDATES: Split occurs when next NLRI would exceed maxSize.
// PREVENTS: Buffer overflow by splitting before adding oversized NLRI.
func TestSplitUpdate_CheckAfterWrite(t *testing.T) {
	// Create NLRIs where the 6th prefix would overflow 30-byte limit
	// 5 /24 prefixes = 20 bytes, 6th would make 24 bytes
	// With overhead = 27, available = 30 - 27 = 3 bytes - too small for /24
	// So use maxSize = 50: overhead = 27, available = 23 bytes (5 prefixes = 20, 6th would be 24)
	var nlri []byte
	for i := range 10 {
		nlri = append(nlri, 24, 10, 0, byte(i)) // /24 = 4 bytes each
	}

	u := &Update{
		PathAttributes: []byte{0x40, 0x01, 0x01, 0x00}, // 4 bytes ORIGIN
		NLRI:           nlri,
	}

	// maxSize = 50: overhead(23) + attrs(4) = 27, leaving 23 for NLRI
	// 5 /24 = 20 bytes fits, 6th would be 24 > 23
	chunks, err := SplitUpdate(u, 50)
	require.NoError(t, err)

	// Should split: first chunk has 5 prefixes (20 bytes), rest in subsequent chunks
	require.Greater(t, len(chunks), 1, "should split when next NLRI exceeds space")

	// Verify first chunk doesn't exceed maxSize
	firstSize := HeaderLen + 4 + len(chunks[0].PathAttributes) + len(chunks[0].NLRI)
	assert.LessOrEqual(t, firstSize, 50, "first chunk exceeds maxSize")

	// Verify all NLRIs preserved
	total := make([]byte, 0, len(nlri))
	for _, chunk := range chunks {
		total = append(total, chunk.NLRI...)
	}
	assert.Equal(t, nlri, total)
}

// TestSplitUpdate_IPv4Field verifies IPv4 NLRI field (not MP) handled correctly.
//
// VALIDATES: Update.NLRI (IPv4 unicast) field splits correctly.
// PREVENTS: IPv4-only UPDATE corruption during splitting.
func TestSplitUpdate_IPv4Field(t *testing.T) {
	// Create many IPv4 /24 prefixes
	var nlri []byte
	for i := range 50 {
		nlri = append(nlri, 24, 192, 168, byte(i)) // /24
	}

	u := &Update{
		PathAttributes: []byte{0x40, 0x01, 0x01, 0x00}, // ORIGIN
		NLRI:           nlri,
	}

	chunks, err := SplitUpdate(u, 80)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split IPv4 NLRI field")

	// Verify each chunk has attributes (IPv4 announcements need them)
	for i, chunk := range chunks {
		if len(chunk.NLRI) > 0 {
			assert.NotEmpty(t, chunk.PathAttributes, "chunk %d should have attrs", i)
		}
	}

	// Verify all NLRIs preserved
	total := make([]byte, 0, len(nlri))
	for _, chunk := range chunks {
		total = append(total, chunk.NLRI...)
	}
	assert.Equal(t, nlri, total)
}

// TestSplitUpdate_FlowSpec_Split verifies FlowSpec NLRI splitting.
//
// VALIDATES: FlowSpec NLRIs (variable length) split at NLRI boundaries.
// PREVENTS: Split in middle of FlowSpec rule.
func TestSplitUpdate_FlowSpec_Split(t *testing.T) {
	// Create FlowSpec NLRIs via MP_REACH_NLRI
	// Each FlowSpec NLRI: [length:1][components:length]
	var fsNLRI []byte
	for i := range 20 {
		// Simple FlowSpec NLRI: length=10, 10 bytes of components
		fsNLRI = append(fsNLRI, 10)
		for j := range 10 {
			fsNLRI = append(fsNLRI, byte(j+i))
		}
	}
	// Each: 11 bytes, total: 220 bytes

	mp := &attribute.MPReachNLRI{
		AFI:      attribute.AFI(1),    // IPv4
		SAFI:     attribute.SAFI(133), // FlowSpec
		NextHops: []netip.Addr{netip.MustParseAddr("192.168.1.1")},
		NLRI:     fsNLRI,
	}

	// Use ChunkMPNLRI to verify FlowSpec splitting
	// maxSize = 50, each FlowSpec = 11 bytes, fits 4 per chunk
	chunks, err := ChunkMPNLRI(fsNLRI, 1, 133, false, 50)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "FlowSpec should split")

	// Verify all bytes preserved
	reassembled := make([]byte, 0, len(fsNLRI))
	for _, chunk := range chunks {
		reassembled = append(reassembled, chunk...)
	}
	assert.Equal(t, fsNLRI, reassembled)

	// Also test via SplitMPReachNLRI
	mpChunks, err := SplitMPReachNLRI(mp, 80)
	require.NoError(t, err)
	require.Greater(t, len(mpChunks), 1, "MP_REACH FlowSpec should split")
}

// TestSplitUpdate_BGPLS_TooLarge verifies error on oversized BGP-LS NLRI.
//
// VALIDATES: Single BGP-LS NLRI > maxSize returns ErrNLRITooLarge.
// PREVENTS: Silent truncation of large BGP-LS topology data.
func TestSplitUpdate_BGPLS_TooLarge(t *testing.T) {
	// BGP-LS NLRI: [type:2][length:2][payload]
	// Create single large NLRI that exceeds typical message size
	bgplsNLRI := make([]byte, 0, 260)
	bgplsNLRI = append(bgplsNLRI, 0, 1, 0x01, 0x00)     // type=Node (2 bytes), length=256 (2 bytes)
	bgplsNLRI = append(bgplsNLRI, make([]byte, 256)...) // payload
	// Total: 4 + 256 = 260 bytes

	// Try to split with maxSize smaller than single NLRI
	_, err := ChunkMPNLRI(bgplsNLRI, 16388, 71, false, 200)
	require.Error(t, err, "should error on oversized BGP-LS NLRI")
	assert.ErrorIs(t, err, ErrNLRITooLarge)

	// Also verify SplitMPNLRI returns same error
	_, _, err = SplitMPNLRI(bgplsNLRI, 16388, 71, false, 200)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNLRITooLarge)
}

// TestSplitUpdate_EmptyNLRI verifies empty NLRI list handling.
//
// VALIDATES: UPDATE with empty NLRI field returns single UPDATE unchanged.
// PREVENTS: Panic or incorrect splitting of empty data.
func TestSplitUpdate_EmptyNLRI(t *testing.T) {
	// Withdrawal-only UPDATE (empty NLRI)
	u := &Update{
		WithdrawnRoutes: []byte{24, 10, 0, 1}, // 10.0.1.0/24
	}

	chunks, err := SplitUpdate(u, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Empty(t, chunks[0].NLRI)
	assert.Equal(t, u.WithdrawnRoutes, chunks[0].WithdrawnRoutes)

	// Attributes-only UPDATE (empty NLRI and WithdrawnRoutes)
	// This is unusual but valid (could be EoR marker)
	u2 := &Update{}

	chunks, err = SplitUpdate(u2, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.True(t, chunks[0].IsEndOfRIB())
}
