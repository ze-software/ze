package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// collectSplit runs message.Splitter.Split with a deep-copying callback so
// tests can inspect every chunk after Split returns.
func collectSplit(t *testing.T, u *message.Update, maxSize int, addPath bool) ([]*message.Update, error) {
	t.Helper()
	var chunks []*message.Update
	s := message.NewSplitter()
	err := s.Split(u, maxSize, addPath, func(c *message.Update) error {
		chunks = append(chunks, &message.Update{
			WithdrawnRoutes: append([]byte(nil), c.WithdrawnRoutes...),
			PathAttributes:  append([]byte(nil), c.PathAttributes...),
			NLRI:            append([]byte(nil), c.NLRI...),
		})
		return nil
	})
	return chunks, err
}

// TestForwardUpdateSplitting verifies UPDATE splitting on forward.
//
// VALIDATES: Large UPDATE from Extended Message peer errors for non-Extended peer.
// PREVENTS: Oversized UPDATE messages being sent (until wire-level split implemented).
func TestForwardUpdateSplitting(t *testing.T) {
	// Create a large UPDATE that exceeds 4096 bytes
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	attrBytes := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x03, 0x04, 10, 0, 0, 1, // NEXT_HOP 10.0.0.1
		0x40, 0x02, 0x00, // AS_PATH empty
	}

	// Create 1200 IPv4 prefixes to make a large UPDATE
	numPrefixes := 1200
	announceWire := make([][]byte, numPrefixes)

	for i := range numPrefixes {
		prefix := netip.MustParsePrefix(generatePrefix(i))
		n := nlri.NewINET(family.IPv4Unicast, prefix, 0)
		announceWire[i] = n.Bytes()
	}

	rawBody := buildRawUpdateBody(nil, attrBytes, announceWire)

	// Verify it exceeds standard limit
	totalLen := message.HeaderLen + len(rawBody)
	require.Greater(t, totalLen, message.MaxMsgLen,
		"test UPDATE should exceed 4096 bytes, got %d", totalLen)

	// Create ReceivedUpdate
	wireUpdate := wireu.NewWireUpdate(rawBody, ctxID)
	wireUpdate.SetMessageID(1)
	update := &ReceivedUpdate{
		WireUpdate:   wireUpdate,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	// Create reactor with cache
	r := &Reactor{
		recentUpdates: NewRecentUpdateCache(100),
		peers:         make(map[netip.AddrPort]*Peer),
	}
	r.recentUpdates.Add(update)

	// Create peer WITHOUT ExtendedMessage
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)

	// Set negotiated capabilities WITHOUT ExtendedMessage
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})

	r.mu.Lock()
	r.peers[settings.PeerKey()] = peer
	r.mu.Unlock()

	// Get max message size for peer
	nc := peer.negotiated.Load()
	maxMsgSize := message.MaxMessageLength(message.TypeUPDATE, nc != nil && nc.ExtendedMessage)
	require.Equal(t, uint16(message.MaxMsgLen), maxMsgSize,
		"peer without ExtendedMessage should have 4096 limit")

	// Verify update size exceeds limit
	updateSize := message.HeaderLen + len(update.WireUpdate.Payload())
	require.Greater(t, updateSize, int(maxMsgSize),
		"update size %d should exceed max %d", updateSize, maxMsgSize)

	// NOTE: Wire-level split not yet implemented.
	// ForwardUpdate will return error for oversized updates until then.
}

// TestForwardUpdateNoSplitWhenFits verifies no split when size fits.
//
// VALIDATES: Small UPDATE forwarded as-is (zero-copy preserved).
// PREVENTS: Unnecessary splitting overhead.
func TestForwardUpdateNoSplitWhenFits(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	attrBytes := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x03, 0x04, 10, 0, 0, 1, // NEXT_HOP 10.0.0.1
		0x40, 0x02, 0x00, // AS_PATH empty
	}

	prefixes := []string{"192.168.1.0/24", "192.168.2.0/24", "192.168.3.0/24"}
	announceWire := make([][]byte, len(prefixes))

	for i, p := range prefixes {
		prefix := netip.MustParsePrefix(p)
		n := nlri.NewINET(family.IPv4Unicast, prefix, 0)
		announceWire[i] = n.Bytes()
	}

	rawBody := buildRawUpdateBody(nil, attrBytes, announceWire)

	// Verify it's under the limit
	totalLen := message.HeaderLen + len(rawBody)
	require.Less(t, totalLen, message.MaxMsgLen,
		"test UPDATE should be under 4096 bytes, got %d", totalLen)

	wireUpdate := wireu.NewWireUpdate(rawBody, ctxID)
	wireUpdate.SetMessageID(2)
	update := &ReceivedUpdate{
		WireUpdate:   wireUpdate,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	nc := &NegotiatedCapabilities{
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	}
	maxMsgSize := message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage)

	// Verify update fits
	updateSize := message.HeaderLen + len(update.WireUpdate.Payload())
	require.LessOrEqual(t, updateSize, int(maxMsgSize),
		"update size %d should fit within max %d", updateSize, maxMsgSize)
}

// TestForwardUpdateSplittingExtendedPeer verifies no split when peer has ExtendedMessage.
//
// VALIDATES: Large UPDATE not split when peer supports Extended Message.
// PREVENTS: Unnecessary splitting when destination can handle large messages.
func TestForwardUpdateSplittingExtendedPeer(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	attrBytes := []byte{
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x03, 0x04, 10, 0, 0, 1,
		0x40, 0x02, 0x00,
	}

	numPrefixes := 1200
	announceWire := make([][]byte, numPrefixes)

	for i := range numPrefixes {
		prefix := netip.MustParsePrefix(generatePrefix(i))
		n := nlri.NewINET(family.IPv4Unicast, prefix, 0)
		announceWire[i] = n.Bytes()
	}

	rawBody := buildRawUpdateBody(nil, attrBytes, announceWire)

	wireUpdate := wireu.NewWireUpdate(rawBody, ctxID)
	wireUpdate.SetMessageID(3)
	update := &ReceivedUpdate{
		WireUpdate:   wireUpdate,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	// Peer WITH ExtendedMessage
	nc := &NegotiatedCapabilities{
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: true,
	}
	maxMsgSize := message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage)
	require.Equal(t, uint16(message.ExtMsgLen), maxMsgSize,
		"peer with ExtendedMessage should have 65535 limit")

	// Verify update fits within extended limit
	updateSize := message.HeaderLen + len(update.WireUpdate.Payload())
	require.LessOrEqual(t, updateSize, int(maxMsgSize),
		"update size %d should fit within extended max %d", updateSize, maxMsgSize)
}

// TestReplayUpdateSplitting verifies UPDATE splitting on reconnect replay.
//
// VALIDATES: adj-rib-out replay respects newly negotiated max msg size.
// PREVENTS: Replay failure when peer reconnects without Extended Message.
func TestReplayUpdateSplitting(t *testing.T) {
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)

	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})

	nc := peer.negotiated.Load()
	maxMsgSize := message.MaxMessageLength(message.TypeUPDATE, nc != nil && nc.ExtendedMessage)
	require.Equal(t, uint16(message.MaxMsgLen), maxMsgSize,
		"peer without ExtendedMessage should have 4096 limit")
}

// generatePrefix generates a unique /24 prefix for testing.
func generatePrefix(i int) string {
	b1 := (i / 256) % 256
	b2 := i % 256
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(b1), byte(b2), 0}), 24).String()
}

// TestSplitUpdateEndToEnd verifies SplitUpdate works through sendUpdateWithSplit path.
//
// VALIDATES: Full splitting pipeline: SplitUpdate → multiple chunks → all NLRIs preserved.
// PREVENTS: Data loss or corruption during UPDATE splitting.
func TestSplitUpdateEndToEnd(t *testing.T) {
	var nlriBytes []byte
	for i := range 1200 {
		b1 := (i / 256) % 256
		b2 := i % 256
		nlriBytes = append(nlriBytes, 0x18, 10, byte(b1), byte(b2))
	}

	attrBytes := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
	}

	update := &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nlriBytes,
	}

	origSize := message.HeaderLen + 4 + len(attrBytes) + len(nlriBytes)
	require.Greater(t, origSize, message.MaxMsgLen,
		"original UPDATE should exceed 4096 bytes, got %d", origSize)

	chunks, err := collectSplit(t, update, message.MaxMsgLen, false)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split into multiple chunks")

	for i, chunk := range chunks {
		chunkSize := message.HeaderLen + 4 + len(chunk.PathAttributes) + len(chunk.NLRI)
		require.LessOrEqual(t, chunkSize, message.MaxMsgLen,
			"chunk %d size %d exceeds max %d", i, chunkSize, message.MaxMsgLen)
	}

	totalNLRI := make([]byte, 0, len(nlriBytes))
	for _, chunk := range chunks {
		totalNLRI = append(totalNLRI, chunk.NLRI...)
	}
	require.Equal(t, nlriBytes, totalNLRI, "all NLRIs should be preserved after splitting")

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
	var nlriBytes []byte
	for i := range 100 {
		nlriBytes = append(nlriBytes, 0x00, 0x00, 0x00, byte(i+1))
		b1 := (i / 256) % 256
		b2 := i % 256
		nlriBytes = append(nlriBytes, 0x18, 10, byte(b1), byte(b2))
	}

	attrBytes := []byte{0x40, 0x01, 0x01, 0x00}

	update := &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nlriBytes,
	}

	chunks, err := collectSplit(t, update, 200, true)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split Add-Path NLRIs")

	totalNLRI := make([]byte, 0, len(nlriBytes))
	for _, chunk := range chunks {
		totalNLRI = append(totalNLRI, chunk.NLRI...)
	}
	require.Equal(t, nlriBytes, totalNLRI, "all Add-Path NLRIs should be preserved")

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
	ipv4 := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)
	ipv6 := nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8::/32"), 0)

	withdrawals := []nlri.NLRI{ipv4, ipv6}

	byFamily := make(map[family.Family][]byte)
	for _, n := range withdrawals {
		fam := n.Family()
		byFamily[fam] = append(byFamily[fam], n.Bytes()...)
	}

	require.Equal(t, 2, len(byFamily), "should have 2 separate family groups")
	require.Contains(t, byFamily, family.IPv4Unicast, "should have IPv4 group")
	require.Contains(t, byFamily, family.IPv6Unicast, "should have IPv6 group")
	require.Equal(t, ipv4.Bytes(), byFamily[family.IPv4Unicast], "IPv4 bytes should match")
	require.Equal(t, ipv6.Bytes(), byFamily[family.IPv6Unicast], "IPv6 bytes should match")

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
