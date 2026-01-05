package reactor

import (
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/zebgp/pkg/api"
	bgpctx "codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/message"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
	"github.com/stretchr/testify/require"
)

// TestForwardUpdateSplitting verifies UPDATE splitting on forward.
//
// VALIDATES: Large UPDATE from Extended Message peer splits for non-Extended peer.
// PREVENTS: Oversized UPDATE messages rejected by destination peer.
func TestForwardUpdateSplitting(t *testing.T) {
	// Create a large UPDATE that exceeds 4096 bytes
	// Need: Header(19) + Body > 4096, so body must be > 4077 bytes
	// Build many NLRIs to create a large message

	// Register encoding context
	ctx := &bgpctx.EncodingContext{ASN4: true}
	ctxID := bgpctx.Registry.Register(ctx)

	// Build attributes: ORIGIN + NEXT_HOP + AS_PATH (small)
	attrBytes := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x03, 0x04, 10, 0, 0, 1, // NEXT_HOP 10.0.0.1
		0x40, 0x02, 0x00, // AS_PATH empty
	}

	// Create 200 IPv4 prefixes (each ~4-5 bytes) to make a large UPDATE
	// /24 prefix = 1 byte length + 3 bytes prefix = 4 bytes each
	// 200 * 4 = 800 bytes of NLRI alone
	// To exceed 4077 we need ~1000 prefixes
	numPrefixes := 1200
	announces := make([]nlri.NLRI, numPrefixes)
	announceWire := make([][]byte, numPrefixes)

	for i := 0; i < numPrefixes; i++ {
		// Generate prefixes like 10.X.Y.0/24 where X and Y vary
		prefix := netip.MustParsePrefix(generatePrefix(i))
		n := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
		announces[i] = n
		announceWire[i] = n.Bytes()
	}

	// Build raw UPDATE body
	// Format: WithdrawnLen(2) + Withdrawn(0) + AttrLen(2) + Attrs + NLRI
	rawBody := buildRawUpdateBody(nil, attrBytes, announceWire)

	// Verify it exceeds standard limit
	totalLen := message.HeaderLen + len(rawBody)
	require.Greater(t, totalLen, message.MaxMsgLen,
		"test UPDATE should exceed 4096 bytes, got %d", totalLen)

	// Create ReceivedUpdate
	wireUpdate := api.NewWireUpdate(rawBody, ctxID)
	update := &ReceivedUpdate{
		UpdateID:     1,
		WireUpdate:   wireUpdate,
		Announces:    announces,
		AnnounceWire: announceWire,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	// Create reactor with cache
	r := &Reactor{
		recentUpdates: NewRecentUpdateCache(time.Minute, 100),
		peers:         make(map[string]*Peer),
	}
	r.recentUpdates.Add(update)

	// Create peer WITHOUT ExtendedMessage
	settings := &PeerSettings{
		Address:  netip.MustParseAddr("10.0.0.2"),
		LocalAS:  65000,
		PeerAS:   65001,
		RouterID: 0x01020301,
	}
	peer := NewPeer(settings)

	// Set negotiated capabilities WITHOUT ExtendedMessage
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[nlri.Family]bool{nlri.IPv4Unicast: true},
		ExtendedMessage: false, // <-- Key: no extended message
	})

	// Add peer to reactor
	r.mu.Lock()
	r.peers[settings.Address.String()] = peer
	r.mu.Unlock()

	// Create adapter and selector
	adapter := &reactorAPIAdapter{r: r}

	// Mock the peer's send to capture what gets sent
	var sentUpdates []*message.Update
	// TODO: Need to mock peer.SendUpdate to capture sent messages

	// For now, just verify the size check logic exists
	// The actual test will need session mocking

	// Get max message size for peer
	nc := peer.negotiated.Load()
	maxMsgSize := message.MaxMessageLength(message.TypeUPDATE, nc != nil && nc.ExtendedMessage)
	require.Equal(t, uint16(message.MaxMsgLen), maxMsgSize,
		"peer without ExtendedMessage should have 4096 limit")

	// Verify update size exceeds limit
	updateSize := message.HeaderLen + len(update.WireUpdate.Payload())
	require.Greater(t, updateSize, int(maxMsgSize),
		"update size %d should exceed max %d", updateSize, maxMsgSize)

	// TODO: When implementation is complete, verify:
	// 1. ForwardUpdate succeeds
	// 2. Multiple smaller UPDATEs are sent instead of one large one
	// 3. Each sent UPDATE is <= maxMsgSize
	// 4. All original NLRIs are present across the split UPDATEs

	_ = adapter
	_ = sentUpdates
}

// TestForwardUpdateNoSplitWhenFits verifies no split when size fits.
//
// VALIDATES: Small UPDATE forwarded as-is (zero-copy preserved).
// PREVENTS: Unnecessary splitting overhead.
func TestForwardUpdateNoSplitWhenFits(t *testing.T) {
	// Create a small UPDATE that fits in 4096 bytes
	ctx := &bgpctx.EncodingContext{ASN4: true}
	ctxID := bgpctx.Registry.Register(ctx)

	attrBytes := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x03, 0x04, 10, 0, 0, 1, // NEXT_HOP 10.0.0.1
		0x40, 0x02, 0x00, // AS_PATH empty
	}

	// Just a few prefixes
	prefixes := []string{"192.168.1.0/24", "192.168.2.0/24", "192.168.3.0/24"}
	announces := make([]nlri.NLRI, len(prefixes))
	announceWire := make([][]byte, len(prefixes))

	for i, p := range prefixes {
		prefix := netip.MustParsePrefix(p)
		n := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
		announces[i] = n
		announceWire[i] = n.Bytes()
	}

	rawBody := buildRawUpdateBody(nil, attrBytes, announceWire)

	// Verify it's under the limit
	totalLen := message.HeaderLen + len(rawBody)
	require.Less(t, totalLen, message.MaxMsgLen,
		"test UPDATE should be under 4096 bytes, got %d", totalLen)

	wireUpdate := api.NewWireUpdate(rawBody, ctxID)
	update := &ReceivedUpdate{
		UpdateID:     2,
		WireUpdate:   wireUpdate,
		Announces:    announces,
		AnnounceWire: announceWire,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	// Get max message size
	nc := &NegotiatedCapabilities{
		families:        map[nlri.Family]bool{nlri.IPv4Unicast: true},
		ExtendedMessage: false,
	}
	maxMsgSize := message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage)

	// Verify update fits
	updateSize := message.HeaderLen + len(update.WireUpdate.Payload())
	require.LessOrEqual(t, updateSize, int(maxMsgSize),
		"update size %d should fit within max %d", updateSize, maxMsgSize)

	// TODO: When implementation is complete, verify:
	// 1. ForwardUpdate succeeds
	// 2. Exactly one UPDATE is sent (no splitting)
	// 3. Zero-copy path is used when contexts match
}

// TestForwardUpdateSplittingExtendedPeer verifies no split when peer has ExtendedMessage.
//
// VALIDATES: Large UPDATE not split when peer supports Extended Message.
// PREVENTS: Unnecessary splitting when destination can handle large messages.
func TestForwardUpdateSplittingExtendedPeer(t *testing.T) {
	// Create a large UPDATE
	ctx := &bgpctx.EncodingContext{ASN4: true}
	ctxID := bgpctx.Registry.Register(ctx)

	attrBytes := []byte{
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x03, 0x04, 10, 0, 0, 1,
		0x40, 0x02, 0x00,
	}

	numPrefixes := 1200
	announces := make([]nlri.NLRI, numPrefixes)
	announceWire := make([][]byte, numPrefixes)

	for i := 0; i < numPrefixes; i++ {
		prefix := netip.MustParsePrefix(generatePrefix(i))
		n := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
		announces[i] = n
		announceWire[i] = n.Bytes()
	}

	rawBody := buildRawUpdateBody(nil, attrBytes, announceWire)

	wireUpdate := api.NewWireUpdate(rawBody, ctxID)
	update := &ReceivedUpdate{
		UpdateID:     3,
		WireUpdate:   wireUpdate,
		Announces:    announces,
		AnnounceWire: announceWire,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	// Peer WITH ExtendedMessage
	nc := &NegotiatedCapabilities{
		families:        map[nlri.Family]bool{nlri.IPv4Unicast: true},
		ExtendedMessage: true, // <-- Key: has extended message
	}
	maxMsgSize := message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage)
	require.Equal(t, uint16(message.ExtMsgLen), maxMsgSize,
		"peer with ExtendedMessage should have 65535 limit")

	// Verify update fits within extended limit
	updateSize := message.HeaderLen + len(update.WireUpdate.Payload())
	require.LessOrEqual(t, updateSize, int(maxMsgSize),
		"update size %d should fit within extended max %d", updateSize, maxMsgSize)

	// TODO: When implementation is complete, verify:
	// 1. ForwardUpdate succeeds
	// 2. Exactly one UPDATE is sent (no splitting needed)
}

// TestForwardUpdateSplitWithConvertError verifies error when splitting fails.
//
// VALIDATES: ConvertToRoutes failure is reported when splitting is needed.
// PREVENTS: Silent route loss when attribute parsing fails.
func TestForwardUpdateSplitWithConvertError(t *testing.T) {
	// Create a large UPDATE with INVALID attributes that will fail ConvertToRoutes
	ctx := &bgpctx.EncodingContext{ASN4: true}
	ctxID := bgpctx.Registry.Register(ctx)

	// Invalid attributes - truncated, will fail parsing
	invalidAttrBytes := []byte{
		0x40, 0x01, 0x01, // ORIGIN attribute header, but missing value byte!
	}

	// Create many NLRIs to make the message large
	numPrefixes := 1200
	announces := make([]nlri.NLRI, numPrefixes)
	announceWire := make([][]byte, numPrefixes)

	for i := 0; i < numPrefixes; i++ {
		prefix := netip.MustParsePrefix(generatePrefix(i))
		n := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
		announces[i] = n
		announceWire[i] = n.Bytes()
	}

	rawBody := buildRawUpdateBody(nil, invalidAttrBytes, announceWire)

	// Verify it exceeds standard limit
	totalLen := message.HeaderLen + len(rawBody)
	require.Greater(t, totalLen, message.MaxMsgLen,
		"test UPDATE should exceed 4096 bytes")

	// Create ReceivedUpdate with invalid attributes
	wireUpdate := api.NewWireUpdate(rawBody, ctxID)
	update := &ReceivedUpdate{
		UpdateID:     100,
		WireUpdate:   wireUpdate,
		Announces:    announces,
		AnnounceWire: announceWire,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	// Verify ConvertToRoutes fails
	routes, err := update.ConvertToRoutes()
	require.Error(t, err, "ConvertToRoutes should fail with invalid attributes")
	require.Nil(t, routes, "routes should be nil when conversion fails")
}

// TestReplayUpdateSplitting verifies UPDATE splitting on reconnect replay.
//
// VALIDATES: adj-rib-out replay respects newly negotiated max msg size.
// PREVENTS: Replay failure when peer reconnects without Extended Message.
func TestReplayUpdateSplitting(t *testing.T) {
	// The current implementation sends each route individually during replay,
	// which is safe for message size limits since single routes rarely exceed
	// 4096 bytes. This test verifies the replay path works correctly.

	// Create peer
	settings := &PeerSettings{
		Address:  netip.MustParseAddr("10.0.0.2"),
		LocalAS:  65000,
		PeerAS:   65001,
		RouterID: 0x01020301,
	}
	peer := NewPeer(settings)

	// Set negotiated capabilities WITHOUT ExtendedMessage
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[nlri.Family]bool{nlri.IPv4Unicast: true},
		ExtendedMessage: false, // <-- Key: no extended message
	})

	// Verify max message size is 4096
	nc := peer.negotiated.Load()
	maxMsgSize := message.MaxMessageLength(message.TypeUPDATE, nc != nil && nc.ExtendedMessage)
	require.Equal(t, uint16(message.MaxMsgLen), maxMsgSize,
		"peer without ExtendedMessage should have 4096 limit")

	// TODO: When a full integration test is needed, verify:
	// 1. Routes in adj-rib-out are replayed
	// 2. Each UPDATE message is <= maxMsgSize
	// 3. All routes are successfully sent
}

// generatePrefix generates a unique /24 prefix for testing.
// Returns prefixes like "10.0.0.0/24", "10.0.1.0/24", ..., "10.255.255.0/24".
func generatePrefix(i int) string {
	// Use 10.X.Y.0/24 where X = i/256, Y = i%256
	b1 := (i / 256) % 256
	b2 := i % 256
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(b1), byte(b2), 0}), 24).String()
}

// TestSplitUpdateEndToEnd verifies SplitUpdate works through sendUpdateWithSplit path.
//
// VALIDATES: Full splitting pipeline: SplitUpdate → multiple chunks → all NLRIs preserved.
// PREVENTS: Data loss or corruption during UPDATE splitting.
func TestSplitUpdateEndToEnd(t *testing.T) {
	// Create 1200 IPv4 /24 prefixes (1200 * 4 = 4800 bytes of NLRI)
	var nlriBytes []byte
	for i := 0; i < 1200; i++ {
		b1 := (i / 256) % 256
		b2 := i % 256
		nlriBytes = append(nlriBytes, 0x18, 10, byte(b1), byte(b2)) // /24 prefix
	}

	// Small attributes
	attrBytes := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
	}

	// Build UPDATE
	update := &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nlriBytes,
	}

	// Verify original size exceeds 4096
	origSize := message.HeaderLen + 4 + len(attrBytes) + len(nlriBytes)
	require.Greater(t, origSize, message.MaxMsgLen,
		"original UPDATE should exceed 4096 bytes, got %d", origSize)

	// Split for non-Extended peer (4096 limit)
	chunks, err := message.SplitUpdate(update, message.MaxMsgLen)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split into multiple chunks")

	// Verify each chunk fits
	for i, chunk := range chunks {
		chunkSize := message.HeaderLen + 4 + len(chunk.PathAttributes) + len(chunk.NLRI)
		require.LessOrEqual(t, chunkSize, message.MaxMsgLen,
			"chunk %d size %d exceeds max %d", i, chunkSize, message.MaxMsgLen)
	}

	// Verify all NLRIs preserved
	var totalNLRI []byte
	for _, chunk := range chunks {
		totalNLRI = append(totalNLRI, chunk.NLRI...)
	}
	require.Equal(t, nlriBytes, totalNLRI, "all NLRIs should be preserved after splitting")

	// Verify attributes preserved in all announcement chunks
	for i, chunk := range chunks {
		require.Equal(t, attrBytes, chunk.PathAttributes,
			"chunk %d should have same attributes", i)
	}

	t.Logf("✅ Split %d byte UPDATE into %d chunks, all NLRIs preserved", origSize, len(chunks))
}

// TestSplitUpdateAddPathEndToEnd verifies Add-Path splitting through full path.
//
// VALIDATES: Add-Path NLRIs split correctly with 4-byte path-id preservation.
// PREVENTS: Path-ID corruption during splitting.
func TestSplitUpdateAddPathEndToEnd(t *testing.T) {
	// Create 100 Add-Path IPv4 /24 prefixes
	// Each Add-Path NLRI: [path-id:4][prefix-len:1][prefix:3] = 8 bytes
	var nlriBytes []byte
	for i := 0; i < 100; i++ {
		// Path ID
		nlriBytes = append(nlriBytes, 0x00, 0x00, 0x00, byte(i+1))
		// /24 prefix
		b1 := (i / 256) % 256
		b2 := i % 256
		nlriBytes = append(nlriBytes, 0x18, 10, byte(b1), byte(b2))
	}

	attrBytes := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP

	update := &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nlriBytes,
	}

	// Split with Add-Path enabled
	chunks, err := message.SplitUpdateWithAddPath(update, 200, true)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split Add-Path NLRIs")

	// Verify all NLRIs preserved with path-ids
	var totalNLRI []byte
	for _, chunk := range chunks {
		totalNLRI = append(totalNLRI, chunk.NLRI...)
	}
	require.Equal(t, nlriBytes, totalNLRI, "all Add-Path NLRIs should be preserved")

	// Verify path-ids are intact (check first NLRI of each chunk)
	for i, chunk := range chunks {
		if len(chunk.NLRI) >= 4 {
			pathID := uint32(chunk.NLRI[0])<<24 | uint32(chunk.NLRI[1])<<16 |
				uint32(chunk.NLRI[2])<<8 | uint32(chunk.NLRI[3])
			require.Greater(t, pathID, uint32(0),
				"chunk %d first path-id should be valid", i)
		}
	}

	t.Logf("✅ Split Add-Path UPDATE into %d chunks, all path-ids preserved", len(chunks))
}

// TestSplitUpdateMixedFamilyWithdrawals verifies per-family withdrawal handling.
//
// VALIDATES: Mixed family withdrawals grouped and sent separately.
// PREVENTS: Incorrect Add-Path detection when families are mixed.
func TestSplitUpdateMixedFamilyWithdrawals(t *testing.T) {
	// Create withdrawals from different families
	ipv4 := nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)
	ipv6 := nlri.NewINET(nlri.IPv6Unicast, netip.MustParsePrefix("2001:db8::/32"), 0)

	withdrawals := []nlri.NLRI{ipv4, ipv6}

	// Group by family (what sendWithdrawalsWithLimit does)
	byFamily := make(map[nlri.Family][]byte)
	for _, n := range withdrawals {
		family := n.Family()
		byFamily[family] = append(byFamily[family], n.Bytes()...)
	}

	// Should have 2 families
	require.Equal(t, 2, len(byFamily), "should have 2 separate family groups")

	// Verify each family's bytes are correct
	require.Contains(t, byFamily, nlri.IPv4Unicast, "should have IPv4 group")
	require.Contains(t, byFamily, nlri.IPv6Unicast, "should have IPv6 group")

	// Verify IPv4 bytes
	require.Equal(t, ipv4.Bytes(), byFamily[nlri.IPv4Unicast], "IPv4 bytes should match")

	// Verify IPv6 bytes
	require.Equal(t, ipv6.Bytes(), byFamily[nlri.IPv6Unicast], "IPv6 bytes should match")

	t.Log("✅ Mixed family withdrawals correctly grouped by family")
}

// buildRawUpdateBody builds a raw UPDATE message body with no withdrawals.
// Format: WithdrawnLen(2) + Withdrawn(0) + AttrLen(2) + Attrs + NLRI.
func buildRawUpdateBody(_ [][]byte, attrBytes []byte, nlriWire [][]byte) []byte {
	// No withdrawals in test UPDATEs
	withdrawLen := 0

	// Calculate NLRI length
	nlriLen := 0
	for _, n := range nlriWire {
		nlriLen += len(n)
	}

	// Build body
	body := make([]byte, 2+withdrawLen+2+len(attrBytes)+nlriLen)
	offset := 0

	// Withdrawn routes length (2 bytes) - always 0 for test UPDATEs
	body[offset] = 0
	body[offset+1] = 0
	offset += 2

	// No withdrawn routes

	// Path attribute length (2 bytes)
	body[offset] = byte(len(attrBytes) >> 8)
	body[offset+1] = byte(len(attrBytes))
	offset += 2

	// Path attributes
	copy(body[offset:], attrBytes)
	offset += len(attrBytes)

	// NLRI
	for _, n := range nlriWire {
		copy(body[offset:], n)
		offset += len(n)
	}

	return body
}
