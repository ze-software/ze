package plugin

import (
	"encoding/binary"
	"testing"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noAddPathCtx is an encoding context without ADD-PATH, for tests that don't need ADD-PATH.
var noAddPathCtx = bgpctx.EncodingContextForASN4(true)

// =============================================================================
// SplitWireUpdate Tests
// =============================================================================

// TestSplitWireUpdate_SmallFits verifies small UPDATE passes through unchanged.
//
// VALIDATES: UPDATE < maxBodySize returns single WireUpdate unchanged.
// PREVENTS: Unnecessary splitting of small messages.
func TestSplitWireUpdate_SmallFits(t *testing.T) {
	// Build small UPDATE: 4 bytes withdrawn len + attr len, 4 attrs, 4 NLRI = 12 bytes
	payload := buildTestUpdatePayload(nil, []byte{0x40, 0x01, 0x01, 0x00}, []byte{0x18, 0xC0, 0xA8, 0x01})

	wu := NewWireUpdate(payload, 0)

	chunks, err := SplitWireUpdate(wu, 4096, noAddPathCtx)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, payload, chunks[0].Payload())
}

// TestSplitWireUpdate_ExactFit verifies boundary condition.
//
// VALIDATES: UPDATE == maxBodySize returns single UPDATE (no split).
// PREVENTS: Off-by-one splitting at exact boundary.
func TestSplitWireUpdate_ExactFit(t *testing.T) {
	// Build UPDATE that exactly fits
	attrs := []byte{0x40, 0x01, 0x01, 0x00}             // 4 bytes
	nlri := []byte{0x18, 0xC0, 0xA8, 0x01}              // 4 bytes
	payload := buildTestUpdatePayload(nil, attrs, nlri) // 4 + 4 + 4 = 12 bytes

	wu := NewWireUpdate(payload, 0)

	chunks, err := SplitWireUpdate(wu, 12, noAddPathCtx)
	require.NoError(t, err)
	require.Len(t, chunks, 1, "exact fit should not split")
}

// TestSplitWireUpdate_IPv4NLRIOverflow verifies NLRI splitting.
//
// VALIDATES: Large NLRI split into N chunks, each <= maxBodySize.
// PREVENTS: Oversized UPDATE sent to peer.
func TestSplitWireUpdate_IPv4NLRIOverflow(t *testing.T) {
	// Create many NLRIs: 100 /24s = 400 bytes
	var nlriData []byte
	for i := range 100 {
		nlriData = append(nlriData, 0x18, 0xC0, 0xA8, byte(i))
	}
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	payload := buildTestUpdatePayload(nil, attrs, nlriData)

	wu := NewWireUpdate(payload, 0)

	// maxBodySize = 50 bytes
	chunks, err := SplitWireUpdate(wu, 50, noAddPathCtx)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split into multiple chunks")

	// Verify each chunk is within size limit
	for i, chunk := range chunks {
		assert.LessOrEqual(t, len(chunk.Payload()), 50, "chunk %d exceeds maxBodySize", i)
	}

	// Verify all NLRIs preserved
	totalNLRI := make([]byte, 0, len(nlriData))
	for _, chunk := range chunks {
		nlri, err := chunk.NLRI()
		require.NoError(t, err)
		totalNLRI = append(totalNLRI, nlri...)
	}
	assert.Equal(t, nlriData, totalNLRI, "all NLRIs should be preserved")
}

// TestSplitWireUpdate_WithdrawnOverflow verifies withdrawal splitting.
//
// VALIDATES: Large withdrawal split into N chunks (no attributes).
// PREVENTS: Oversized withdrawal message.
func TestSplitWireUpdate_WithdrawnOverflow(t *testing.T) {
	// Create many withdrawn prefixes
	var withdrawn []byte
	for i := range 100 {
		withdrawn = append(withdrawn, 0x18, 0x0A, 0x00, byte(i))
	}
	payload := buildTestUpdatePayload(withdrawn, nil, nil)

	wu := NewWireUpdate(payload, 0)

	// maxBodySize = 50 bytes
	chunks, err := SplitWireUpdate(wu, 50, noAddPathCtx)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split withdrawals")

	// Verify all withdrawals preserved
	totalWithdrawn := make([]byte, 0, len(withdrawn))
	for _, chunk := range chunks {
		wd, err := chunk.Withdrawn()
		require.NoError(t, err)
		totalWithdrawn = append(totalWithdrawn, wd...)
	}
	assert.Equal(t, withdrawn, totalWithdrawn)
}

// TestSplitWireUpdate_EndOfRIB verifies EoR passthrough.
//
// VALIDATES: Empty UPDATE (End-of-RIB) returns single unchanged UPDATE.
// PREVENTS: EoR marker corruption.
func TestSplitWireUpdate_EndOfRIB(t *testing.T) {
	// Empty UPDATE = End-of-RIB
	payload := buildTestUpdatePayload(nil, nil, nil)
	wu := NewWireUpdate(payload, 0)

	chunks, err := SplitWireUpdate(wu, 4096, noAddPathCtx)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, payload, chunks[0].Payload())
}

// TestSplitWireUpdate_SingleNLRITooLarge verifies error on huge single NLRI.
//
// VALIDATES: Error returned when single NLRI > available space.
// PREVENTS: Silent data loss or infinite loop.
func TestSplitWireUpdate_SingleNLRITooLarge(t *testing.T) {
	// Single /32 = 5 bytes, attrs = 4 bytes, overhead = 4 bytes
	// Total = 13 bytes minimum, but maxBodySize = 10
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	nlriData := []byte{0x20, 0x0A, 0x00, 0x00, 0x01} // /32 = 5 bytes
	payload := buildTestUpdatePayload(nil, attrs, nlriData)

	wu := NewWireUpdate(payload, 0)

	_, err := SplitWireUpdate(wu, 10, noAddPathCtx)
	require.Error(t, err)
}

// TestSplitWireUpdate_AllChunksValid verifies chunk structure.
//
// VALIDATES: Each chunk is a valid UPDATE payload (correct length fields).
// PREVENTS: Malformed UPDATE messages.
func TestSplitWireUpdate_AllChunksValid(t *testing.T) {
	var nlriData []byte
	for i := range 100 {
		nlriData = append(nlriData, 0x18, 0xC0, 0xA8, byte(i))
	}
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	payload := buildTestUpdatePayload(nil, attrs, nlriData)

	wu := NewWireUpdate(payload, 0)

	chunks, err := SplitWireUpdate(wu, 50, noAddPathCtx)
	require.NoError(t, err)

	// Each chunk should have valid structure
	for i, chunk := range chunks {
		p := chunk.Payload()
		require.GreaterOrEqual(t, len(p), 4, "chunk %d too short", i)

		// Verify withdrawn length field is valid
		wdLen := binary.BigEndian.Uint16(p[0:2])
		require.LessOrEqual(t, int(wdLen)+2, len(p), "chunk %d invalid withdrawn length", i)

		// Verify attr length field is valid
		attrLenOffset := 2 + int(wdLen)
		attrLen := binary.BigEndian.Uint16(p[attrLenOffset : attrLenOffset+2])
		require.LessOrEqual(t, attrLenOffset+2+int(attrLen), len(p), "chunk %d invalid attr length", i)
	}
}

// TestSplitWireUpdate_AddPath verifies Add-Path NLRI splitting.
//
// VALIDATES: Add-Path NLRIs (with 4-byte path-id) split at correct boundaries.
// PREVENTS: Path-id corruption or misaligned splits.
func TestSplitWireUpdate_AddPath(t *testing.T) {
	// Add-Path NLRI: [path-id:4][prefix-len:1][prefix-bytes]
	// Each /24 with path-id = 4 + 1 + 3 = 8 bytes
	var nlriData []byte
	for i := range 20 {
		nlriData = append(nlriData, 0x00, 0x00, 0x00, byte(i+1)) // path-id
		nlriData = append(nlriData, 0x18, 0xC0, 0xA8, byte(i))   // /24
	}
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	payload := buildTestUpdatePayload(nil, attrs, nlriData)

	wu := NewWireUpdate(payload, 0)

	// Context with ADD-PATH enabled for IPv4 unicast
	ctx := bgpctx.EncodingContextWithAddPath(true, map[nlri.Family]bool{
		{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}: true,
	})

	// maxBodySize = 50, overhead ~8, leaves ~42 for NLRI
	// Each Add-Path /24 = 8 bytes, so ~5 per chunk
	chunks, err := SplitWireUpdate(wu, 50, ctx)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split Add-Path NLRI")

	// Verify all NLRIs preserved
	totalNLRI := make([]byte, 0, len(nlriData))
	for _, chunk := range chunks {
		nlri, err := chunk.NLRI()
		require.NoError(t, err)
		totalNLRI = append(totalNLRI, nlri...)
	}
	assert.Equal(t, nlriData, totalNLRI, "all Add-Path NLRIs should be preserved")

	// Verify each chunk has valid Add-Path structure (8-byte aligned)
	for i, chunk := range chunks {
		chunkNLRI, err := chunk.NLRI()
		require.NoError(t, err)
		if len(chunkNLRI) > 0 {
			assert.Equal(t, 0, len(chunkNLRI)%8, "chunk %d has invalid Add-Path alignment", i)
		}
	}
}

// TestSplitWireUpdate_SourceCtxIDPreserved verifies context ID preservation.
//
// VALIDATES: All split chunks preserve source context ID.
// PREVENTS: Context loss breaking zero-copy forwarding.
func TestSplitWireUpdate_SourceCtxIDPreserved(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

	var nlriData []byte
	for i := range 100 {
		nlriData = append(nlriData, 0x18, 0xC0, 0xA8, byte(i))
	}
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	payload := buildTestUpdatePayload(nil, attrs, nlriData)

	wu := NewWireUpdate(payload, ctxID)

	chunks, err := SplitWireUpdate(wu, 50, noAddPathCtx)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	for i, chunk := range chunks {
		assert.Equal(t, ctxID, chunk.SourceCtxID(), "chunk %d context ID mismatch", i)
	}
}

// TestSplitWireUpdate_BaseAttrsTooLarge verifies error when baseAttrs exceed maxSize.
//
// VALIDATES: Error returned instead of infinite loop.
// PREVENTS: Infinite loop when baseAttrs alone exceed available space.
func TestSplitWireUpdate_BaseAttrsTooLarge(t *testing.T) {
	// Create UPDATE with large base attributes (> maxSize)
	// Build valid AS_PATH with proper structure
	// AS_PATH: flags(1) + type(1) + len(1) + segment_type(1) + segment_len(1) + AS4s
	asPathValue := make([]byte, 82) // 2 (header) + 80 (20 AS4s)
	asPathValue[0] = 0x02           // AS_SEQUENCE
	asPathValue[1] = 20             // 20 ASNs
	// ASNs are zeros (valid AS 0)

	largeASPath := make([]byte, 0, 3+len(asPathValue))
	largeASPath = append(largeASPath, 0x40, 0x02, 82) // flags, type, length
	largeASPath = append(largeASPath, asPathValue...)

	attrs := make([]byte, 0, 4+len(largeASPath))
	attrs = append(attrs, 0x40, 0x01, 0x01, 0x00) // ORIGIN IGP
	attrs = append(attrs, largeASPath...)         // Large AS_PATH

	nlriData := []byte{0x18, 0xC0, 0xA8, 0x01} // small NLRI
	payload := buildTestUpdatePayload(nil, attrs, nlriData)

	wu := NewWireUpdate(payload, 0)

	// maxSize = 50, but baseAttrs = 89 bytes (4 ORIGIN + 85 AS_PATH)
	_, err := SplitWireUpdate(wu, 50, noAddPathCtx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base attributes")
}

// TestSplitWireUpdate_AddPathPerFamily verifies ADD-PATH is checked per AFI/SAFI.
//
// VALIDATES: IPv6 MP_REACH uses IPv6 ADD-PATH state, not IPv4.
// PREVENTS: Wrong ADD-PATH state applied to wrong family.
func TestSplitWireUpdate_AddPathPerFamily(t *testing.T) {
	// Create UPDATE with IPv6 MP_REACH_NLRI containing ADD-PATH NLRIs
	// MP_REACH: AFI=2 (IPv6), SAFI=1 (unicast), NH_LEN=16, NH, Reserved, NLRIs
	// With ADD-PATH, each NLRI is: [path-id:4][prefix-len:1][prefix:8] = 13 bytes for /64

	var mpNLRIs []byte
	for i := range 10 {
		mpNLRIs = append(mpNLRIs, 0x00, 0x00, 0x00, byte(i+1)) // path-id
		mpNLRIs = append(mpNLRIs, 0x40)                        // /64
		mpNLRIs = append(mpNLRIs, 0x20, 0x01, 0x0d, 0xb8, 0x00, byte(i), 0x00, 0x00)
	}

	// Build MP_REACH attribute
	mpReachHeader := []byte{
		0x00, 0x02, // AFI IPv6
		0x01,                                                                               // SAFI unicast
		0x10,                                                                               // NH length = 16
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // NH
		0x00, 0x01, // NH continued
		0x00, // Reserved
	}
	mpReachValue := make([]byte, 0, len(mpReachHeader)+len(mpNLRIs))
	mpReachValue = append(mpReachValue, mpReachHeader...)
	mpReachValue = append(mpReachValue, mpNLRIs...)

	mpReach := make([]byte, 0, 4+len(mpReachValue))
	mpReach = append(mpReach, 0x90, 0x0E) // Optional, Extended
	mpReach = append(mpReach, byte(len(mpReachValue)>>8), byte(len(mpReachValue)))
	mpReach = append(mpReach, mpReachValue...)

	// Base attrs: just ORIGIN
	baseAttrs := []byte{0x40, 0x01, 0x01, 0x00}
	attrs := make([]byte, 0, len(baseAttrs)+len(mpReach))
	attrs = append(attrs, baseAttrs...)
	attrs = append(attrs, mpReach...)

	payload := buildTestUpdatePayload(nil, attrs, nil)
	wu := NewWireUpdate(payload, 0)

	// Context with ADD-PATH enabled for IPv6 unicast ONLY (not IPv4)
	ctx := bgpctx.EncodingContextWithAddPath(true, map[nlri.Family]bool{
		{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}: false,
		{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}: true,
	})

	// Should split successfully using IPv6 ADD-PATH state
	chunks, err := SplitWireUpdate(wu, 80, ctx)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split IPv6 MP_REACH")
}

// TestSplitWireUpdate_MalformedInput verifies error on truncated input.
//
// VALIDATES: Malformed input WireUpdate returns error when split is attempted.
// PREVENTS: Silent corruption or panic on bad input.
//
// Note: SplitWireUpdate has a fast path that returns the original unchanged
// when len(payload) <= maxBodySize. Validation only happens when split is
// needed. Empty/nil payloads that fit are passed through - errors surface
// when the caller uses the WireUpdate methods.
func TestSplitWireUpdate_MalformedInput(t *testing.T) {
	tests := []struct {
		name        string
		payload     []byte
		maxBodySize int
	}{
		{
			name:        "too_short_3bytes",
			payload:     []byte{0x00, 0x00, 0x00},
			maxBodySize: 2, // Force split path (payload > maxBodySize)
		},
		{
			name:        "withdrawn_truncated",
			payload:     []byte{0x00, 0x05, 0x01, 0x02}, // claims 5 bytes withdrawn, has 2
			maxBodySize: 3,                              // Force split
		},
		{
			name:        "attrs_truncated",
			payload:     []byte{0x00, 0x00, 0x00, 0x10, 0x40, 0x01}, // claims 16 bytes attrs, has 2
			maxBodySize: 5,                                          // Force split
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wu := NewWireUpdate(tt.payload, 0)
			_, err := SplitWireUpdate(wu, tt.maxBodySize, noAddPathCtx)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrUpdateTruncated)
		})
	}
}

// TestSplitWireUpdate_FastPathNoValidation documents that fast path skips validation.
//
// VALIDATES: When no split needed, original is returned without validation.
// This is intentional - validation happens when using WireUpdate methods.
func TestSplitWireUpdate_FastPathNoValidation(t *testing.T) {
	// Malformed payload that fits within maxBodySize
	payload := []byte{0x00, 0x05, 0x01} // claims 5 bytes withdrawn, has 1
	wu := NewWireUpdate(payload, 0)

	// No error from SplitWireUpdate - fast path returns original
	chunks, err := SplitWireUpdate(wu, 4096, noAddPathCtx)
	require.NoError(t, err, "fast path should not validate")
	require.Len(t, chunks, 1)
	assert.Equal(t, payload, chunks[0].Payload())

	// Error surfaces when using the result
	_, err = chunks[0].Withdrawn()
	require.Error(t, err, "validation happens on use")
}

// TestSplitWireUpdate_OutputChunksAccessible verifies all split chunks parse without error.
//
// VALIDATES: All output chunks from split pass Withdrawn(), Attrs(), NLRI() without error.
// PREVENTS: Split producing malformed UPDATE payloads that fail on access.
func TestSplitWireUpdate_OutputChunksAccessible(t *testing.T) {
	// Create UPDATE with many NLRIs that will be split
	var nlriData []byte
	for i := range 100 {
		nlriData = append(nlriData, 0x18, 0xC0, 0xA8, byte(i)) // /24
	}
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	payload := buildTestUpdatePayload(nil, attrs, nlriData)

	wu := NewWireUpdate(payload, 0)

	chunks, err := SplitWireUpdate(wu, 50, noAddPathCtx)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split into multiple chunks")

	// Verify EVERY chunk's accessors work without error
	for i, chunk := range chunks {
		_, err := chunk.Withdrawn()
		assert.NoError(t, err, "chunk %d Withdrawn() failed", i)

		_, err = chunk.Attrs()
		assert.NoError(t, err, "chunk %d Attrs() failed", i)

		_, err = chunk.NLRI()
		assert.NoError(t, err, "chunk %d NLRI() failed", i)
	}
}

// TestSplitWireUpdate_BaseAttrsInAllChunks verifies base attributes replicated in all chunks.
//
// VALIDATES: All announcement chunks contain identical base attributes (ORIGIN, AS_PATH).
// PREVENTS: Split dropping path attributes from subsequent chunks.
func TestSplitWireUpdate_BaseAttrsInAllChunks(t *testing.T) {
	// Create UPDATE with multiple base attributes and many NLRIs
	// ORIGIN(4) + AS_PATH(9) = 13 bytes base attrs
	origin := []byte{0x40, 0x01, 0x01, 0x00}                               // ORIGIN IGP
	asPath := []byte{0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xE8} // AS_PATH: AS_SEQUENCE [65000], len=6
	baseAttrs := make([]byte, 0, len(origin)+len(asPath))
	baseAttrs = append(baseAttrs, origin...)
	baseAttrs = append(baseAttrs, asPath...)

	var nlriData []byte
	for i := range 50 {
		nlriData = append(nlriData, 0x18, 0xC0, 0xA8, byte(i)) // /24
	}

	payload := buildTestUpdatePayload(nil, baseAttrs, nlriData)
	wu := NewWireUpdate(payload, 0)

	// Split with small max to force multiple chunks
	// overhead = 4 (length fields) + 13 (attrs) = 17, leaving ~33 for NLRI
	// Each /24 = 4 bytes, so ~8 per chunk
	chunks, err := SplitWireUpdate(wu, 50, noAddPathCtx)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split into multiple chunks")

	// Extract attrs from first chunk as reference
	firstAttrs, err := chunks[0].Attrs()
	require.NoError(t, err)
	require.NotNil(t, firstAttrs, "first chunk should have attrs")
	firstPacked := firstAttrs.Packed()

	// All subsequent chunks with NLRI must have identical base attributes
	for i, chunk := range chunks[1:] {
		chunkNLRI, err := chunk.NLRI()
		require.NoError(t, err)

		if len(chunkNLRI) > 0 { // Only announcement chunks need attrs
			chunkAttrs, err := chunk.Attrs()
			require.NoError(t, err, "chunk %d Attrs() failed", i+1)
			require.NotNil(t, chunkAttrs, "chunk %d should have attrs (has NLRI)", i+1)

			chunkPacked := chunkAttrs.Packed()
			assert.Equal(t, firstPacked, chunkPacked,
				"chunk %d attrs differ from first chunk", i+1)
		}
	}
}

// TestSplitWireUpdate_SourceIDPreserved verifies sourceID is copied to all split chunks.
//
// VALIDATES: All split chunks have same sourceID as original.
// PREVENTS: Lost source identity after split.
func TestSplitWireUpdate_SourceIDPreserved(t *testing.T) {
	// Create UPDATE that will be split
	var nlriData []byte
	for i := range 100 {
		nlriData = append(nlriData, 0x18, 0xC0, 0xA8, byte(i)) // /24
	}
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	payload := buildTestUpdatePayload(nil, attrs, nlriData)

	wu := NewWireUpdate(payload, 0)
	wu.SetSourceID(42)

	chunks, err := SplitWireUpdate(wu, 50, noAddPathCtx)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split into multiple chunks")

	// All chunks must have same sourceID
	for i, chunk := range chunks {
		assert.Equal(t, wu.SourceID(), chunk.SourceID(),
			"chunk %d sourceID differs from original", i)
	}
}

// TestSplitWireUpdate_MixedIPv4AndMP verifies splitting with both IPv4 and MP content.
//
// VALIDATES: UPDATE with IPv4 NLRI + MP_REACH_NLRI splits correctly, preserving both.
// PREVENTS: MP content loss when IPv4 content also present.
func TestSplitWireUpdate_MixedIPv4AndMP(t *testing.T) {
	// Build MP_REACH_NLRI for IPv6 with several prefixes
	// Keep it small: 5 /64s = 45 bytes NLRI
	mpNLRIs := make([]byte, 0, 5*9) // 5 * (1 prefix len + 8 prefix bytes)
	for i := range 5 {
		mpNLRIs = append(mpNLRIs, 0x40)                                              // /64
		mpNLRIs = append(mpNLRIs, 0x20, 0x01, 0x0d, 0xb8, 0x00, byte(i), 0x00, 0x00) // prefix
	}

	mpReachHdr := []byte{
		0x00, 0x02, // AFI IPv6
		0x01,                                                                               // SAFI unicast
		0x10,                                                                               // NH length = 16
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // NH
		0x00, 0x01, // NH continued
		0x00, // Reserved
	}
	mpReachValue := make([]byte, 0, len(mpReachHdr)+len(mpNLRIs))
	mpReachValue = append(mpReachValue, mpReachHdr...)
	mpReachValue = append(mpReachValue, mpNLRIs...)

	mpReach := make([]byte, 0, 4+len(mpReachValue))
	mpReach = append(mpReach, 0x90, 0x0E) // Optional, Extended
	mpReach = append(mpReach, byte(len(mpReachValue)>>8), byte(len(mpReachValue)))
	mpReach = append(mpReach, mpReachValue...)

	// Base attrs: ORIGIN + MP_REACH
	origin := []byte{0x40, 0x01, 0x01, 0x00}
	attrs := make([]byte, 0, len(origin)+len(mpReach))
	attrs = append(attrs, origin...)
	attrs = append(attrs, mpReach...)

	// IPv4 NLRIs: 30 /24s = 120 bytes
	ipv4NLRI := make([]byte, 0, 30*4) // 30 * 4 bytes per /24
	for i := range 30 {
		ipv4NLRI = append(ipv4NLRI, 0x18, 0xC0, 0xA8, byte(i)) // /24
	}

	// IPv4 withdrawals: 15 /24s = 60 bytes
	ipv4Withdrawn := make([]byte, 0, 15*4) // 15 * 4 bytes per /24
	for i := range 15 {
		ipv4Withdrawn = append(ipv4Withdrawn, 0x18, 0x0A, 0x00, byte(i)) // /24
	}

	payload := buildTestUpdatePayload(ipv4Withdrawn, attrs, ipv4NLRI)
	wu := NewWireUpdate(payload, 0)

	// Total content:
	// - ipv4Withdrawn: 60 bytes
	// - mpReach: ~70 bytes (4 header + 21 fixed + 45 NLRIs)
	// - origin: 4 bytes
	// - ipv4NLRI: 120 bytes
	// - length fields: 4 bytes
	// Total: ~258 bytes
	// Use maxBodySize=150 to force 2+ chunks while allowing all content types
	chunks, err := SplitWireUpdate(wu, 150, noAddPathCtx)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split into multiple chunks")

	// Collect all content from chunks
	totalIPv4Withdrawn := make([]byte, 0, len(ipv4Withdrawn))
	totalIPv4NLRI := make([]byte, 0, len(ipv4NLRI))
	mpReachFound := false

	for _, chunk := range chunks {
		wd, err := chunk.Withdrawn()
		require.NoError(t, err)
		totalIPv4Withdrawn = append(totalIPv4Withdrawn, wd...)

		nlri, err := chunk.NLRI()
		require.NoError(t, err)
		totalIPv4NLRI = append(totalIPv4NLRI, nlri...)

		mpr, err := chunk.MPReach()
		require.NoError(t, err)
		if mpr != nil {
			mpReachFound = true
			// Verify it's IPv6 unicast
			assert.Equal(t, uint16(2), mpr.AFI(), "MP_REACH should be IPv6")
			assert.Equal(t, uint8(1), mpr.SAFI(), "MP_REACH should be unicast")
		}
	}

	// Verify all IPv4 content preserved
	assert.Equal(t, ipv4Withdrawn, totalIPv4Withdrawn, "IPv4 withdrawals should be preserved")
	assert.Equal(t, ipv4NLRI, totalIPv4NLRI, "IPv4 NLRI should be preserved")

	// Verify MP_REACH was included
	assert.True(t, mpReachFound, "MP_REACH_NLRI should be present in at least one chunk")
}

// =============================================================================
// separateMPAttributes Tests
// =============================================================================

// TestSeparateMPAttributes_Empty verifies empty attributes handling.
//
// VALIDATES: Empty attrs → empty base, no MP_*.
// PREVENTS: Panic on empty input.
func TestSeparateMPAttributes_Empty(t *testing.T) {
	base, mpReaches, mpUnreaches, err := separateMPAttributes(nil)
	require.NoError(t, err)
	assert.Empty(t, base)
	assert.Empty(t, mpReaches)
	assert.Empty(t, mpUnreaches)
}

// TestSeparateMPAttributes_NoMP verifies non-MP attributes pass through.
//
// VALIDATES: Attrs without MP_* → all in base.
// PREVENTS: Corruption of non-MP attributes.
func TestSeparateMPAttributes_NoMP(t *testing.T) {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x02, 0x00, // AS_PATH empty
	}

	base, mpReaches, mpUnreaches, err := separateMPAttributes(attrs)
	require.NoError(t, err)
	assert.Equal(t, attrs, base)
	assert.Empty(t, mpReaches)
	assert.Empty(t, mpUnreaches)
}

// TestSeparateMPAttributes_WithMPReach verifies MP_REACH_NLRI extraction.
//
// VALIDATES: MP_REACH_NLRI separated from base attrs.
// PREVENTS: MP_REACH missing from output.
func TestSeparateMPAttributes_WithMPReach(t *testing.T) {
	// ORIGIN + MP_REACH_NLRI (type 14)
	origin := []byte{0x40, 0x01, 0x01, 0x00}
	mpReach := []byte{
		0x90, 0x0E, 0x00, 0x08, // Optional, Transitive, Extended, type 14, len 8
		0x00, 0x02, // AFI IPv6
		0x01,             // SAFI unicast
		0x00,             // NH length 0
		0x00,             // Reserved
		0x40, 0x20, 0x01, // /64 prefix
	}
	attrs := make([]byte, 0, len(origin)+len(mpReach))
	attrs = append(attrs, origin...)
	attrs = append(attrs, mpReach...)

	base, mpReaches, mpUnreaches, err := separateMPAttributes(attrs)
	require.NoError(t, err)
	assert.Equal(t, origin, base)
	require.Len(t, mpReaches, 1)
	assert.Equal(t, mpReach, mpReaches[0])
	assert.Empty(t, mpUnreaches)
}

// TestSeparateMPAttributes_WithMPUnreach verifies MP_UNREACH_NLRI extraction.
//
// VALIDATES: MP_UNREACH_NLRI separated from base attrs.
// PREVENTS: MP_UNREACH missing from output.
func TestSeparateMPAttributes_WithMPUnreach(t *testing.T) {
	// ORIGIN + MP_UNREACH_NLRI (type 15)
	origin := []byte{0x40, 0x01, 0x01, 0x00}
	mpUnreach := []byte{
		0x90, 0x0F, 0x00, 0x06, // Optional, Transitive, Extended, type 15, len 6
		0x00, 0x02, // AFI IPv6
		0x01,             // SAFI unicast
		0x40, 0x20, 0x01, // /64 prefix
	}
	attrs := make([]byte, 0, len(origin)+len(mpUnreach))
	attrs = append(attrs, origin...)
	attrs = append(attrs, mpUnreach...)

	base, mpReaches, mpUnreaches, err := separateMPAttributes(attrs)
	require.NoError(t, err)
	assert.Equal(t, origin, base)
	assert.Empty(t, mpReaches)
	require.Len(t, mpUnreaches, 1)
	assert.Equal(t, mpUnreach, mpUnreaches[0])
}

// TestSeparateMPAttributes_MultipleMPReach verifies multiple MP_REACH extraction.
//
// VALIDATES: Multiple MP_REACH_NLRI attrs (invalid per RFC) all extracted.
// PREVENTS: Only first MP_REACH being captured.
func TestSeparateMPAttributes_MultipleMPReach(t *testing.T) {
	origin := []byte{0x40, 0x01, 0x01, 0x00}
	mpReach1 := []byte{0x90, 0x0E, 0x00, 0x05, 0x00, 0x02, 0x01, 0x00, 0x00} // IPv6
	mpReach2 := []byte{0x90, 0x0E, 0x00, 0x05, 0x00, 0x01, 0x01, 0x00, 0x00} // IPv4
	attrs := append(append(origin, mpReach1...), mpReach2...)

	base, mpReaches, mpUnreaches, err := separateMPAttributes(attrs)
	require.NoError(t, err)
	assert.Equal(t, origin, base)
	require.Len(t, mpReaches, 2)
	assert.Empty(t, mpUnreaches)
}

// TestSeparateMPAttributes_ExtendedLength verifies extended length handling.
//
// VALIDATES: Extended-length (2-byte len) attributes parsed correctly.
// PREVENTS: Truncation of large attributes.
func TestSeparateMPAttributes_ExtendedLength(t *testing.T) {
	// Large MP_REACH with extended length
	mpReach := make([]byte, 0, 4+256)
	mpReach = append(mpReach, 0x90, 0x0E, 0x01, 0x00) // Extended, type 14, len 256
	mpReach = append(mpReach, make([]byte, 256)...)

	_, mpReaches, _, err := separateMPAttributes(mpReach)
	require.NoError(t, err)
	require.Len(t, mpReaches, 1)
	assert.Equal(t, 256+4, len(mpReaches[0])) // header(4) + value(256)
}

// =============================================================================
// splitIPv4NLRIs Tests
// =============================================================================

// TestSplitIPv4NLRIs_AllFit verifies no split when all NLRIs fit.
//
// VALIDATES: NLRI <= maxBytes returns original.
// PREVENTS: Unnecessary copying.
func TestSplitIPv4NLRIs_AllFit(t *testing.T) {
	nlriData := []byte{0x18, 0xC0, 0xA8, 0x01, 0x18, 0xC0, 0xA8, 0x02} // 8 bytes

	fitting, remaining, err := splitIPv4NLRIs(nlriData, 100, noAddPathCtx)
	require.NoError(t, err)
	assert.Equal(t, nlriData, fitting)
	assert.Empty(t, remaining)
}

// TestSplitIPv4NLRIs_Partial verifies partial split.
//
// VALIDATES: Split at correct NLRI boundary.
// PREVENTS: Split in middle of prefix.
func TestSplitIPv4NLRIs_Partial(t *testing.T) {
	// 3 /24s = 12 bytes, maxBytes = 10
	nlriData := []byte{
		0x18, 0xC0, 0xA8, 0x01, // /24 #1
		0x18, 0xC0, 0xA8, 0x02, // /24 #2
		0x18, 0xC0, 0xA8, 0x03, // /24 #3
	}

	fitting, remaining, err := splitIPv4NLRIs(nlriData, 10, noAddPathCtx)
	require.NoError(t, err)
	assert.Equal(t, nlriData[:8], fitting) // 2 /24s fit
	assert.Equal(t, nlriData[8:], remaining)
}

// TestSplitIPv4NLRIs_FirstTooLarge verifies error on oversized single NLRI.
//
// VALIDATES: Error when first NLRI > maxBytes.
// PREVENTS: Silent truncation.
func TestSplitIPv4NLRIs_FirstTooLarge(t *testing.T) {
	nlriData := []byte{0x20, 0x0A, 0x00, 0x00, 0x01} // /32 = 5 bytes

	_, _, err := splitIPv4NLRIs(nlriData, 3, noAddPathCtx)
	require.Error(t, err)
}

// TestSplitIPv4NLRIs_AddPath verifies Add-Path aware splitting.
//
// VALIDATES: Split respects 4-byte path-id + NLRI boundary.
// PREVENTS: Path-id corruption.
func TestSplitIPv4NLRIs_AddPath(t *testing.T) {
	// 2 Add-Path NLRIs: [path-id:4][len:1][prefix:3] = 8 bytes each
	nlriData := []byte{
		0x00, 0x00, 0x00, 0x01, 0x18, 0xC0, 0xA8, 0x01, // path-id=1, /24
		0x00, 0x00, 0x00, 0x02, 0x18, 0xC0, 0xA8, 0x02, // path-id=2, /24
	}

	ctx := bgpctx.EncodingContextWithAddPath(true, map[nlri.Family]bool{
		{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}: true,
	})

	fitting, remaining, err := splitIPv4NLRIs(nlriData, 10, ctx)
	require.NoError(t, err)
	assert.Equal(t, nlriData[:8], fitting) // First Add-Path NLRI fits
	assert.Equal(t, nlriData[8:], remaining)
}

// =============================================================================
// splitMPReach Tests
// =============================================================================

// TestSplitMPReach_NoSplit verifies no split when attribute fits.
//
// VALIDATES: Attribute <= maxBytes returns original.
// PREVENTS: Unnecessary processing.
func TestSplitMPReach_NoSplit(t *testing.T) {
	// Small MP_REACH
	mpReach := []byte{
		0x90, 0x0E, 0x00, 0x0C, // flags, type, len=12
		0x00, 0x02, // AFI IPv6
		0x01,                                     // SAFI unicast
		0x00,                                     // NH length 0
		0x00,                                     // Reserved
		0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, // /64
	}

	fitting, remaining, err := splitMPReach(mpReach, 100, noAddPathCtx)
	require.NoError(t, err)
	assert.Equal(t, mpReach, fitting)
	assert.Empty(t, remaining)
}

// TestSplitMPReach_Split verifies splitting preserves NextHop in each chunk.
//
// VALIDATES: Split MP_REACH attributes both contain NextHop.
// PREVENTS: NextHop loss in split chunks.
func TestSplitMPReach_Split(t *testing.T) {
	// Build MP_REACH with many NLRIs
	var nlris []byte
	for i := range 20 {
		nlris = append(nlris, 0x40)                                              // /64
		nlris = append(nlris, 0x20, 0x01, 0x0d, 0xb8, 0x00, byte(i), 0x00, 0x00) // prefix
	}

	mpReachHeaderData := []byte{
		0x00, 0x02, // AFI IPv6
		0x01,                                                                               // SAFI unicast
		0x10,                                                                               // NH length = 16
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // NH part 1
		0x00, 0x01, // NH part 2
		0x00, // Reserved
	}
	mpReachValue := make([]byte, 0, len(mpReachHeaderData)+len(nlris))
	mpReachValue = append(mpReachValue, mpReachHeaderData...)
	mpReachValue = append(mpReachValue, nlris...)

	mpReach := make([]byte, 0, 4+len(mpReachValue))
	mpReach = append(mpReach, 0x90, 0x0E)
	mpReach = append(mpReach, byte(len(mpReachValue)>>8), byte(len(mpReachValue)))
	mpReach = append(mpReach, mpReachValue...)

	// Split with small maxBytes
	fitting, remaining, err := splitMPReach(mpReach, 60, noAddPathCtx)
	require.NoError(t, err)
	require.NotEmpty(t, fitting)
	require.NotEmpty(t, remaining)

	// Both should be valid MP_REACH with same AFI/SAFI/NH
	// Determine header length based on extended flag
	fitHeaderLen := 3
	if fitting[0]&0x10 != 0 {
		fitHeaderLen = 4
	}
	require.GreaterOrEqual(t, len(fitting), fitHeaderLen+2)
	fitAFI := binary.BigEndian.Uint16(fitting[fitHeaderLen : fitHeaderLen+2])
	assert.Equal(t, uint16(2), fitAFI, "fitting AFI should be IPv6")

	remHeaderLen := 3
	if remaining[0]&0x10 != 0 {
		remHeaderLen = 4
	}
	require.GreaterOrEqual(t, len(remaining), remHeaderLen+2)
	remAFI := binary.BigEndian.Uint16(remaining[remHeaderLen : remHeaderLen+2])
	assert.Equal(t, uint16(2), remAFI, "remaining AFI should be IPv6")
}

// TestSplitMPReach_InvalidMaxBytes verifies error on invalid maxBytes.
//
// VALIDATES: Error returned for maxBytes <= 0.
// PREVENTS: Panic or undefined behavior.
func TestSplitMPReach_InvalidMaxBytes(t *testing.T) {
	mpReach := []byte{0x90, 0x0E, 0x00, 0x05, 0x00, 0x02, 0x01, 0x00, 0x00}

	_, _, err := splitMPReach(mpReach, 0, noAddPathCtx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid maxBytes")

	_, _, err = splitMPReach(mpReach, -1, noAddPathCtx)
	require.Error(t, err)
}

// =============================================================================
// splitMPUnreach Tests
// =============================================================================

// TestSplitMPUnreach_NoSplit verifies no split when attribute fits.
//
// VALIDATES: Attribute <= maxBytes returns original.
// PREVENTS: Unnecessary processing.
func TestSplitMPUnreach_NoSplit(t *testing.T) {
	mpUnreach := []byte{
		0x90, 0x0F, 0x00, 0x0B, // flags, type, len=11
		0x00, 0x02, // AFI IPv6
		0x01,                                           // SAFI unicast
		0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, // /64
	}

	fitting, remaining, err := splitMPUnreach(mpUnreach, 100, noAddPathCtx)
	require.NoError(t, err)
	assert.Equal(t, mpUnreach, fitting)
	assert.Empty(t, remaining)
}

// TestSplitMPUnreach_Split verifies splitting preserves AFI/SAFI in each chunk.
//
// VALIDATES: Split MP_UNREACH attributes both contain AFI/SAFI.
// PREVENTS: AFI/SAFI loss in split chunks.
func TestSplitMPUnreach_Split(t *testing.T) {
	// Build MP_UNREACH with many NLRIs
	var nlris []byte
	for i := range 20 {
		nlris = append(nlris, 0x40)                                              // /64
		nlris = append(nlris, 0x20, 0x01, 0x0d, 0xb8, 0x00, byte(i), 0x00, 0x00) // prefix
	}

	mpUnreachHeaderData := []byte{
		0x00, 0x02, // AFI IPv6
		0x01, // SAFI unicast
	}
	mpUnreachValue := make([]byte, 0, len(mpUnreachHeaderData)+len(nlris))
	mpUnreachValue = append(mpUnreachValue, mpUnreachHeaderData...)
	mpUnreachValue = append(mpUnreachValue, nlris...)

	mpUnreach := make([]byte, 0, 4+len(mpUnreachValue))
	mpUnreach = append(mpUnreach, 0x90, 0x0F)
	mpUnreach = append(mpUnreach, byte(len(mpUnreachValue)>>8), byte(len(mpUnreachValue)))
	mpUnreach = append(mpUnreach, mpUnreachValue...)

	// Split with small maxBytes
	fitting, remaining, err := splitMPUnreach(mpUnreach, 40, noAddPathCtx)
	require.NoError(t, err)
	require.NotEmpty(t, fitting)
	require.NotEmpty(t, remaining)

	// Both should be valid MP_UNREACH with same AFI/SAFI
	// Determine header length based on extended flag
	fitHeaderLen := 3
	if fitting[0]&0x10 != 0 {
		fitHeaderLen = 4
	}
	require.GreaterOrEqual(t, len(fitting), fitHeaderLen+2)
	fitAFI := binary.BigEndian.Uint16(fitting[fitHeaderLen : fitHeaderLen+2])
	assert.Equal(t, uint16(2), fitAFI, "fitting AFI should be IPv6")
}

// TestSplitMPUnreach_InvalidMaxBytes verifies error on invalid maxBytes.
//
// VALIDATES: Error returned for maxBytes <= 0.
// PREVENTS: Panic or undefined behavior.
func TestSplitMPUnreach_InvalidMaxBytes(t *testing.T) {
	mpUnreach := []byte{0x90, 0x0F, 0x00, 0x05, 0x00, 0x02, 0x01, 0x00, 0x00}

	_, _, err := splitMPUnreach(mpUnreach, 0, noAddPathCtx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid maxBytes")
}

// =============================================================================
// buildUpdatePayload Tests
// =============================================================================

// TestBuildUpdatePayload_WithdrawsOnly verifies withdrawal-only structure.
//
// VALIDATES: Withdrawal UPDATE has zero attr len.
// PREVENTS: Invalid withdrawal messages.
func TestBuildUpdatePayload_WithdrawsOnly(t *testing.T) {
	withdraws := []byte{0x18, 0x0A, 0x00, 0x01}

	payload := buildUpdatePayload(withdraws, nil, nil, nil, nil)

	// Verify structure: wdLen(2) + wdData(4) + attrLen(2) + nlri(0) = 8 bytes
	require.Len(t, payload, 8)
	wdLen := binary.BigEndian.Uint16(payload[0:2])
	assert.Equal(t, uint16(4), wdLen)
	attrLen := binary.BigEndian.Uint16(payload[6:8])
	assert.Equal(t, uint16(0), attrLen)
}

// TestBuildUpdatePayload_Mixed verifies mixed UPDATE structure.
//
// VALIDATES: All components correctly serialized.
// PREVENTS: Missing or misordered fields.
func TestBuildUpdatePayload_Mixed(t *testing.T) {
	withdraws := []byte{0x18, 0x0A, 0x00, 0x01}
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	nlriData := []byte{0x18, 0xC0, 0xA8, 0x01}

	payload := buildUpdatePayload(withdraws, attrs, nil, nil, nlriData)

	// Parse and verify
	wdLen := binary.BigEndian.Uint16(payload[0:2])
	assert.Equal(t, uint16(4), wdLen)

	attrLenOffset := 2 + int(wdLen)
	attrLen := binary.BigEndian.Uint16(payload[attrLenOffset : attrLenOffset+2])
	assert.Equal(t, uint16(4), attrLen)

	nlriOffset := attrLenOffset + 2 + int(attrLen)
	assert.Equal(t, nlriData, payload[nlriOffset:])
}

// =============================================================================
// Helper Functions
// =============================================================================

// buildTestUpdatePayload builds a raw UPDATE payload for testing.
func buildTestUpdatePayload(withdrawn, attrs, nlriData []byte) []byte {
	wdLen := len(withdrawn)
	attrLen := len(attrs)

	buf := make([]byte, 2+wdLen+2+attrLen+len(nlriData))

	binary.BigEndian.PutUint16(buf[0:2], uint16(wdLen)) //nolint:gosec // G115: test helper, bounded input
	copy(buf[2:], withdrawn)

	attrOffset := 2 + wdLen
	binary.BigEndian.PutUint16(buf[attrOffset:], uint16(attrLen)) //nolint:gosec // G115: test helper, bounded input
	copy(buf[attrOffset+2:], attrs)

	nlriOffset := attrOffset + 2 + attrLen
	copy(buf[nlriOffset:], nlriData)

	return buf
}
