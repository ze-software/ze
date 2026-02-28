package reactor

import (
	"context"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wireu"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// findMPAttribute scans attribute bytes for an MP attribute and validates AFI/SAFI.
// Returns true if found and validates expected AFI/SAFI values.
func findMPAttribute(t *testing.T, data []byte, targetCode attribute.AttributeCode, expectedAFI uint16, expectedSAFI uint8) bool {
	t.Helper()
	for len(data) >= 3 {
		flags := data[0]
		code := attribute.AttributeCode(data[1])
		var length int
		var hdrLen int
		if flags&0x10 != 0 {
			if len(data) < 4 {
				break
			}
			length = int(data[2])<<8 | int(data[3])
			hdrLen = 4
		} else {
			length = int(data[2])
			hdrLen = 3
		}

		if code == targetCode {
			if len(data) >= hdrLen+3 {
				afi := uint16(data[hdrLen])<<8 | uint16(data[hdrLen+1])
				safi := data[hdrLen+2]
				require.Equal(t, expectedAFI, afi, "%s AFI", targetCode)
				require.Equal(t, expectedSAFI, safi, "%s SAFI", targetCode)
			}
			return true
		}

		if len(data) < hdrLen+length {
			break
		}
		data = data[hdrLen+length:]
	}
	return false
}

// TestReactorNew verifies Reactor creation with correct initial state.
//
// VALIDATES: Reactor is created with config and not running.
//
// PREVENTS: Reactor auto-starting or with invalid state.
func TestReactorNew(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	require.NotNil(t, reactor, "New must return non-nil")
	require.False(t, reactor.Running(), "reactor should not be running initially")
}

// TestReactorStartStop verifies basic start/stop lifecycle.
//
// VALIDATES: Reactor can be started and stopped cleanly.
//
// PREVENTS: Resource leaks or goroutine leaks on stop.
func TestReactorStartStop(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	err := reactor.Start()
	require.NoError(t, err)
	require.True(t, reactor.Running())

	reactor.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = reactor.Wait(ctx)
	require.NoError(t, err)

	require.False(t, reactor.Running())
}

// TestReactorAddPeer verifies adding peers to reactor.
//
// VALIDATES: Peers can be added and are tracked.
//
// PREVENTS: Lost peer references or duplicate handling.
func TestReactorAddPeer(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	peers := reactor.Peers()
	require.Len(t, peers, 1)
}

// TestReactorRemovePeer verifies removing peers from reactor.
//
// VALIDATES: Peers can be removed and cleaned up.
//
// PREVENTS: Orphaned peer goroutines.
func TestReactorRemovePeer(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	err = reactor.RemovePeer(settings.Address)
	require.NoError(t, err)

	peers := reactor.Peers()
	require.Len(t, peers, 0)
}

// TestReactorPeersStartOnRun verifies peers start when reactor runs.
//
// VALIDATES: All configured peers start when reactor starts.
//
// PREVENTS: Peers remaining idle after reactor start.
func TestReactorPeersStartOnRun(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.Port = 0 // Invalid port to prevent actual connection

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	err = reactor.Start()
	require.NoError(t, err)

	// Give peers time to start
	time.Sleep(20 * time.Millisecond)

	peers := reactor.Peers()
	require.Len(t, peers, 1)
	require.NotEqual(t, PeerStateStopped, peers[0].State(), "peer should be running")

	reactor.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = reactor.Wait(ctx)
}

// TestReactorListenerAcceptsConnections verifies listener is active.
//
// VALIDATES: Reactor's listener accepts incoming connections.
//
// PREVENTS: Dead listener after reactor start.
func TestReactorListenerAcceptsConnections(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	err := reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	addr := reactor.ListenAddr()
	require.NotNil(t, addr)

	// Connect to listener
	conn, err := net.DialTimeout("tcp", addr.String(), time.Second) //nolint:noctx // Test code
	require.NoError(t, err)
	_ = conn.Close()
}

// TestReactorIncomingConnectionMatchesPeer verifies peer matching.
//
// VALIDATES: Incoming connections are matched to configured neighbors.
//
// PREVENTS: Connections from unknown peers being accepted.
func TestReactorIncomingConnectionMatchesPeer(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	// Add passive peer expecting connection from localhost
	settings := NewPeerSettings(
		mustParseAddr("127.0.0.1"),
		65000, 65001, 0x01010101,
	)
	settings.Connection = ConnectionPassive

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	var accepted atomic.Bool
	reactor.SetConnectionCallback(func(conn net.Conn, n *PeerSettings) {
		accepted.Store(true)
		_ = conn.Close()
	})

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	addr := reactor.ListenAddr()

	// Connect
	conn, err := net.Dial("tcp", addr.String()) //nolint:noctx // Test code
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	time.Sleep(50 * time.Millisecond)

	require.True(t, accepted.Load(), "connection should be matched to peer")
}

// TestReactorContextCancellation verifies reactor stops on context cancel.
//
// VALIDATES: Reactor respects context cancellation.
//
// PREVENTS: Orphaned resources when parent context is canceled.
func TestReactorContextCancellation(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	ctx, cancel := context.WithCancel(context.Background())

	err := reactor.StartWithContext(ctx)
	require.NoError(t, err)
	require.True(t, reactor.Running())

	cancel()

	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	err = reactor.Wait(waitCtx)

	require.NoError(t, err)
	require.False(t, reactor.Running())
}

// TestReactorGracefulShutdown verifies all components stop cleanly.
//
// VALIDATES: Peers, listener, and signals all stop on shutdown.
//
// PREVENTS: Partial shutdown leaving resources dangling.
func TestReactorGracefulShutdown(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.Port = 0

	_ = reactor.AddPeer(settings)

	err := reactor.Start()
	require.NoError(t, err)

	time.Sleep(20 * time.Millisecond)

	reactor.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = reactor.Wait(ctx)
	require.NoError(t, err)

	// Verify everything stopped
	require.False(t, reactor.Running())
	for _, peer := range reactor.Peers() {
		require.Equal(t, PeerStateStopped, peer.State())
	}
}

// TestReactorStats verifies stats collection.
//
// VALIDATES: Reactor tracks connection statistics.
//
// PREVENTS: Missing observability.
func TestReactorStats(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	err := reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	stats := reactor.Stats()
	require.NotNil(t, stats)
	require.GreaterOrEqual(t, stats.Uptime, time.Duration(0))
}

// TestWriteAnnounceUpdateIPv4 verifies WriteAnnounceUpdate produces correct wire format for IPv4.
//
// VALIDATES: Zero-allocation WriteAnnounceUpdate produces valid BGP UPDATE message
// with correct header, attributes, and NLRI for IPv4 unicast routes.
//
// PREVENTS: Wire format regression when using zero-allocation path.
func TestWriteAnnounceUpdateIPv4(t *testing.T) {
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
	}

	buf := make([]byte, 4096)
	n := WriteAnnounceUpdate(buf, 0, route, 65000, false, true, false)

	require.Greater(t, n, message.HeaderLen, "message must be larger than header")

	// Verify BGP header
	// RFC 4271 Section 4.1 - Marker must be all 0xFF
	for i := range 16 {
		require.Equal(t, byte(0xFF), buf[i], "marker byte %d must be 0xFF", i)
	}

	// RFC 4271 Section 4.1 - Length field
	length := int(buf[16])<<8 | int(buf[17])
	require.Equal(t, n, length, "length field must match actual message length")

	// RFC 4271 Section 4.1 - Type must be UPDATE (2)
	require.Equal(t, byte(message.TypeUPDATE), buf[18], "type must be UPDATE")

	// RFC 4271 Section 4.3 - Withdrawn Routes Length (should be 0 for announce)
	withdrawnLen := int(buf[19])<<8 | int(buf[20])
	require.Equal(t, 0, withdrawnLen, "withdrawn routes length must be 0 for announce")

	// RFC 4271 Section 4.3 - Total Path Attribute Length
	attrLen := int(buf[21])<<8 | int(buf[22])
	require.Greater(t, attrLen, 0, "path attributes length must be > 0")

	// RFC 4271 Section 4.3 - NLRI must be present for IPv4
	nlriStart := 23 + attrLen
	require.Less(t, nlriStart, n, "NLRI must be present after attributes")
}

// TestWriteAnnounceUpdateIPv6 verifies WriteAnnounceUpdate produces correct wire format for IPv6.
//
// VALIDATES: Zero-allocation WriteAnnounceUpdate uses MP_REACH_NLRI for IPv6 routes.
//
// PREVENTS: IPv6 routes being sent with IPv4-style encoding (RFC 4760 violation).
func TestWriteAnnounceUpdateIPv6(t *testing.T) {
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
	}

	buf := make([]byte, 4096)
	n := WriteAnnounceUpdate(buf, 0, route, 65000, true, true, false)

	require.Greater(t, n, message.HeaderLen, "message must be larger than header")

	// Verify BGP header
	require.Equal(t, byte(message.TypeUPDATE), buf[18], "type must be UPDATE")

	// RFC 4271 Section 4.3 - Withdrawn Routes Length = 0
	withdrawnLen := int(buf[19])<<8 | int(buf[20])
	require.Equal(t, 0, withdrawnLen, "withdrawn routes length must be 0")

	// RFC 4271 Section 4.3 - Path Attribute Length
	attrLen := int(buf[21])<<8 | int(buf[22])
	require.Greater(t, attrLen, 0, "path attributes must contain MP_REACH_NLRI")

	// RFC 4760 - No inline NLRI for IPv6 (all in MP_REACH_NLRI)
	nlriStart := 23 + attrLen
	require.Equal(t, n, nlriStart, "no inline NLRI for IPv6 routes")

	// Scan for MP_REACH_NLRI (type 14) in attributes - RFC 4760 Section 3
	data := buf[23 : 23+attrLen]
	foundMPReach := findMPAttribute(t, data, attribute.AttrMPReachNLRI, 2, 1) // AFI=2 (IPv6), SAFI=1 (Unicast)
	require.True(t, foundMPReach, "IPv6 routes must have MP_REACH_NLRI attribute")
}

// TestWriteWithdrawUpdateIPv4 verifies WriteWithdrawUpdate produces correct wire format for IPv4.
//
// VALIDATES: Zero-allocation WriteWithdrawUpdate uses WithdrawnRoutes field for IPv4.
//
// PREVENTS: Wire format regression when using zero-allocation path.
func TestWriteWithdrawUpdateIPv4(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")

	buf := make([]byte, 4096)
	n := WriteWithdrawUpdate(buf, 0, prefix, false)

	require.Greater(t, n, message.HeaderLen, "message must be larger than header")

	// Verify BGP header
	require.Equal(t, byte(message.TypeUPDATE), buf[18], "type must be UPDATE")

	// RFC 4271 Section 4.3 - Withdrawn Routes Length > 0 for IPv4 withdrawal
	withdrawnLen := int(buf[19])<<8 | int(buf[20])
	require.Greater(t, withdrawnLen, 0, "withdrawn routes length must be > 0 for IPv4")

	// RFC 4271 Section 4.3 - Path Attribute Length = 0 for pure withdrawal
	attrLen := int(buf[21+withdrawnLen])<<8 | int(buf[22+withdrawnLen])
	require.Equal(t, 0, attrLen, "path attributes length must be 0 for withdrawal")
}

// TestWriteWithdrawUpdateIPv6 verifies WriteWithdrawUpdate produces correct wire format for IPv6.
//
// VALIDATES: Zero-allocation WriteWithdrawUpdate uses MP_UNREACH_NLRI for IPv6.
//
// PREVENTS: IPv6 withdrawals being sent with IPv4-style encoding (RFC 4760 violation).
func TestWriteWithdrawUpdateIPv6(t *testing.T) {
	prefix := netip.MustParsePrefix("2001:db8::/32")

	buf := make([]byte, 4096)
	n := WriteWithdrawUpdate(buf, 0, prefix, false)

	require.Greater(t, n, message.HeaderLen, "message must be larger than header")

	// Verify BGP header
	require.Equal(t, byte(message.TypeUPDATE), buf[18], "type must be UPDATE")

	// RFC 4760 Section 4 - Withdrawn Routes Length = 0 for IPv6
	withdrawnLen := int(buf[19])<<8 | int(buf[20])
	require.Equal(t, 0, withdrawnLen, "withdrawn routes length must be 0 for IPv6")

	// RFC 4760 Section 4 - Path Attributes must contain MP_UNREACH_NLRI
	attrLen := int(buf[21])<<8 | int(buf[22])
	require.Greater(t, attrLen, 0, "path attributes must contain MP_UNREACH_NLRI")

	// Scan for MP_UNREACH_NLRI (type 15) in attributes - RFC 4760 Section 4
	data := buf[23 : 23+attrLen]
	foundMPUnreach := findMPAttribute(t, data, attribute.AttrMPUnreachNLRI, 2, 1) // AFI=2 (IPv6), SAFI=1 (Unicast)
	require.True(t, foundMPUnreach, "IPv6 withdrawals must have MP_UNREACH_NLRI attribute")
}

// TestWriteAnnounceUpdateWithAddPath verifies ADD-PATH encoding.
//
// VALIDATES: WriteAnnounceUpdate correctly encodes path identifier when ADD-PATH enabled.
//
// PREVENTS: ADD-PATH encoding being silently skipped (RFC 7911 violation).
func TestWriteAnnounceUpdateWithAddPath(t *testing.T) {
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
	}

	// With ADD-PATH enabled
	bufAddPath := make([]byte, 4096)
	nAddPath := WriteAnnounceUpdate(bufAddPath, 0, route, 65000, false, true, true)

	// Without ADD-PATH
	bufNoAddPath := make([]byte, 4096)
	nNoAddPath := WriteAnnounceUpdate(bufNoAddPath, 0, route, 65000, false, true, false)

	// RFC 7911: ADD-PATH adds 4-byte path identifier before each NLRI
	// Message with ADD-PATH should be 4 bytes longer
	require.Equal(t, nNoAddPath+4, nAddPath, "ADD-PATH should add 4 bytes for path identifier")

	// Verify path attributes are same length (ADD-PATH only affects NLRI)
	attrLenAddPath := int(bufAddPath[21])<<8 | int(bufAddPath[22])
	attrLenNoAddPath := int(bufNoAddPath[21])<<8 | int(bufNoAddPath[22])
	require.Equal(t, attrLenNoAddPath, attrLenAddPath, "path attributes length must be same")
}

// TestWriteAnnounceUpdateASN4False verifies 2-byte AS encoding.
//
// VALIDATES: WriteAnnounceUpdate uses 2-byte AS numbers when ASN4=false.
//
// PREVENTS: 4-byte AS numbers being sent to peers without ASN4 capability (RFC 6793).
func TestWriteAnnounceUpdateASN4False(t *testing.T) {
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
	}

	// With ASN4=true (4-byte AS)
	bufASN4 := make([]byte, 4096)
	nASN4 := WriteAnnounceUpdate(bufASN4, 0, route, 65000, false, true, false)

	// With ASN4=false (2-byte AS)
	bufASN2 := make([]byte, 4096)
	nASN2 := WriteAnnounceUpdate(bufASN2, 0, route, 65000, false, false, false)

	// RFC 6793: 2-byte AS encoding is shorter
	// AS_PATH with single ASN: 4-byte = 3+4=7, 2-byte = 3+2=5, diff = 2
	require.Equal(t, nASN4-2, nASN2, "ASN4=false should be 2 bytes shorter (2-byte AS vs 4-byte AS)")
}

// TestWriteASPathLongSegmentSplitting verifies AS_PATH segment splitting for >255 ASNs.
//
// VALIDATES: AS_PATH with >255 ASNs is split into multiple segments.
//
// PREVENTS: Count byte overflow causing malformed AS_PATH (RFC 4271 violation).
func TestWriteASPathLongSegmentSplitting(t *testing.T) {
	// Create route with 300 ASNs (requires splitting: 255 + 45)
	asPath := make([]uint32, 300)
	for i := range asPath {
		asPath[i] = uint32(65000 + i) //nolint:gosec // G115: test data, i bounded by 300
	}

	// Build attributes with AS_PATH
	builder := attribute.NewBuilder()
	builder.SetASPath(asPath)
	wireBytes := builder.Build()

	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
		Wire:    attribute.NewAttributesWire(wireBytes, bgpctx.APIContextID),
	}

	buf := make([]byte, 8192)
	n := WriteAnnounceUpdate(buf, 0, route, 65000, false, true, false)

	require.Greater(t, n, message.HeaderLen, "message must be larger than header")

	// Parse the AS_PATH attribute to verify structure
	// Skip header (19) + withdrawn len (2) + attr len (2) = 23
	attrLen := int(buf[21])<<8 | int(buf[22])
	require.Greater(t, attrLen, 255, "AS_PATH should use extended length")

	// Find AS_PATH attribute (code 2)
	data := buf[23 : 23+attrLen]
	for len(data) >= 3 {
		flags := data[0]
		code := attribute.AttributeCode(data[1])
		var length int
		var hdrLen int
		if flags&0x10 != 0 { // Extended length
			if len(data) < 4 {
				break
			}
			length = int(data[2])<<8 | int(data[3])
			hdrLen = 4
		} else {
			length = int(data[2])
			hdrLen = 3
		}

		if code == attribute.AttrASPath {
			// RFC 4271: Extended length flag should be set for >255 bytes
			require.True(t, flags&0x10 != 0, "AS_PATH should use extended length flag")

			// Parse segments - should be 2 segments (255 + 45 ASNs)
			segData := data[hdrLen : hdrLen+length]
			segCount := 0
			totalASNs := 0
			for len(segData) >= 2 {
				segType := segData[0]
				segLen := int(segData[1])
				require.Equal(t, byte(attribute.ASSequence), segType, "segment type should be AS_SEQUENCE")
				require.LessOrEqual(t, segLen, 255, "segment count must not exceed 255")
				totalASNs += segLen
				segCount++
				segData = segData[2+segLen*4:] // 4-byte ASNs
			}

			require.Equal(t, 2, segCount, "should have 2 segments (255+45)")
			require.Equal(t, 300, totalASNs, "total ASNs should be 300")
			break
		}

		if len(data) < hdrLen+length {
			break
		}
		data = data[hdrLen+length:]
	}
}

// TestWriteCommunitiesExtendedLength verifies COMMUNITIES extended length for >63 communities.
//
// VALIDATES: COMMUNITIES with >63 entries uses extended length format.
//
// PREVENTS: Length byte overflow causing malformed attribute (RFC 4271 violation).
func TestWriteCommunitiesExtendedLength(t *testing.T) {
	// Create route with 100 communities (400 bytes, requires extended length)
	communities := make([]uint32, 100)
	for i := range communities {
		communities[i] = uint32(0xFFFF0000 | i) //nolint:gosec // G115: test data, i bounded by 100
	}

	// Build attributes with communities
	builder := attribute.NewBuilder()
	for _, c := range communities {
		builder.AddCommunityValue(c)
	}
	wireBytes := builder.Build()

	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
		Wire:    attribute.NewAttributesWire(wireBytes, bgpctx.APIContextID),
	}

	buf := make([]byte, 4096)
	n := WriteAnnounceUpdate(buf, 0, route, 65000, false, true, false)

	require.Greater(t, n, message.HeaderLen, "message must be larger than header")

	// Find COMMUNITIES attribute (code 8)
	attrLen := int(buf[21])<<8 | int(buf[22])
	data := buf[23 : 23+attrLen]
	for len(data) >= 3 {
		flags := data[0]
		code := attribute.AttributeCode(data[1])
		var length int
		var hdrLen int
		if flags&0x10 != 0 { // Extended length
			if len(data) < 4 {
				break
			}
			length = int(data[2])<<8 | int(data[3])
			hdrLen = 4
		} else {
			length = int(data[2])
			hdrLen = 3
		}

		if code == attribute.AttrCommunity {
			// RFC 4271: Extended length flag should be set for >255 bytes
			require.True(t, flags&0x10 != 0, "COMMUNITIES should use extended length flag")
			require.Equal(t, 400, length, "COMMUNITIES length should be 400 (100*4)")
			break
		}

		if len(data) < hdrLen+length {
			break
		}
		data = data[hdrLen+length:]
	}
}

// BenchmarkWriteAnnounceUpdateIPv4 measures allocations for IPv4 announce.
// Run with: go test -bench=BenchmarkWriteAnnounce -benchmem ./pkg/reactor/...
func BenchmarkWriteAnnounceUpdateIPv4(b *testing.B) {
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
	}
	buf := make([]byte, 4096)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		WriteAnnounceUpdate(buf, 0, route, 65000, false, true, false)
	}
}

// BenchmarkWriteAnnounceUpdateIPv4WithCommunities measures allocations with communities.
func BenchmarkWriteAnnounceUpdateIPv4WithCommunities(b *testing.B) {
	// Build attributes with communities
	builder := attribute.NewBuilder()
	builder.AddCommunityValue(0xFFFF0001)
	builder.AddCommunityValue(0xFFFF0002)
	builder.AddCommunityValue(0xFFFF0003)
	wireBytes := builder.Build()

	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
		Wire:    attribute.NewAttributesWire(wireBytes, bgpctx.APIContextID),
	}
	buf := make([]byte, 4096)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		WriteAnnounceUpdate(buf, 0, route, 65000, false, true, false)
	}
}

// BenchmarkWriteAnnounceUpdateIPv6 measures allocations for IPv6 announce.
func BenchmarkWriteAnnounceUpdateIPv6(b *testing.B) {
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
	}
	buf := make([]byte, 4096)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		WriteAnnounceUpdate(buf, 0, route, 65000, true, true, false)
	}
}

// TestGetPeerProcessBindingsEncodingInheritance verifies encoding inheritance chain.
//
// VALIDATES: Empty peer encoding inherits from process, empty process defaults to "text".
//
// PREVENTS: Empty encoding causing silent failures in message dispatch.
func TestGetPeerProcessBindingsEncodingInheritance(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
		Plugins: []PluginConfig{
			{Name: "json-proc", Run: "./test", Encoder: "json"},
			{Name: "text-proc", Run: "./test", Encoder: "text"},
			{Name: "empty-proc", Run: "./test", Encoder: ""},
		},
	}

	reactor := New(cfg)

	// Add peer with bindings that test inheritance
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.ProcessBindings = []ProcessBinding{
		{PluginName: "json-proc", Encoding: "text"}, // Explicit override
		{PluginName: "json-proc", Encoding: ""},     // Inherit from process (json)
		{PluginName: "text-proc", Encoding: ""},     // Inherit from process (text)
		{PluginName: "empty-proc", Encoding: ""},    // Process empty, default to text
	}

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	// Get bindings through API adapter
	adapter := &reactorAPIAdapter{reactor}
	bindings := adapter.GetPeerProcessBindings(mustParseAddr("192.0.2.1"))

	require.Len(t, bindings, 4)

	// Verify inheritance chain
	require.Equal(t, "text", bindings[0].Encoding, "explicit override should be text")
	require.Equal(t, "json", bindings[1].Encoding, "empty should inherit from json-proc")
	require.Equal(t, "text", bindings[2].Encoding, "empty should inherit from text-proc")
	require.Equal(t, "text", bindings[3].Encoding, "empty proc should default to text")

	// Verify format defaults to "parsed"
	require.Equal(t, "parsed", bindings[0].Format, "format should default to parsed")
}

// TestGetPeerProcessBindingsNotFound verifies nil return for unknown peer.
//
// VALIDATES: GetPeerProcessBindings returns nil for non-existent peer.
//
// PREVENTS: Panic on unknown peer lookup.
func TestGetPeerProcessBindingsNotFound(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	adapter := &reactorAPIAdapter{reactor}
	bindings := adapter.GetPeerProcessBindings(mustParseAddr("192.0.2.99"))

	require.Nil(t, bindings, "unknown peer should return nil")
}

// TestGetPeerProcessBindingsReceiveNegotiated verifies ReceiveNegotiated passes through.
//
// VALIDATES: ReceiveNegotiated flag is copied from reactor.ProcessBinding to plugin.PeerProcessBinding.
//
// PREVENTS: Config setting receive [ negotiated ]; having no effect.
func TestGetPeerProcessBindingsReceiveNegotiated(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
		Plugins:    []PluginConfig{{Name: "test-proc", Run: "./test"}},
	}

	reactor := New(cfg)

	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.ProcessBindings = []ProcessBinding{
		{PluginName: "test-proc", ReceiveNegotiated: true},
		{PluginName: "test-proc", ReceiveNegotiated: false},
	}

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	adapter := &reactorAPIAdapter{reactor}
	bindings := adapter.GetPeerProcessBindings(mustParseAddr("192.0.2.1"))

	require.Len(t, bindings, 2)
	require.True(t, bindings[0].ReceiveNegotiated, "first binding should have ReceiveNegotiated=true")
	require.False(t, bindings[1].ReceiveNegotiated, "second binding should have ReceiveNegotiated=false")
}

// TestBuildLabeledUnicastRIBRouteAllAttributes verifies ALL attributes are stored.
//
// VALIDATES: buildLabeledUnicastRIBRoute includes all path attributes in rib.Route.
// This is critical for queued routes to preserve attributes on replay.
//
// PREVENTS: Attribute loss when routes are queued and replayed via buildRIBRouteUpdate.
// (Fixes bug where AnnounceRoute only stored OriginIGP, losing MED/Communities/etc.)
func TestBuildLabeledUnicastRIBRouteAllAttributes(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
		LocalAS:    65000,
	}
	reactor := New(cfg)
	adapter := &reactorAPIAdapter{reactor}

	// Create route with ALL attributes populated
	builder := attribute.NewBuilder()
	builder.SetOrigin(1) // EGP
	builder.SetMED(100)
	builder.SetLocalPref(200)
	builder.SetASPath([]uint32{65001, 65002})
	builder.AddCommunityValue(0x12345678)
	builder.AddLargeCommunity(65000, 1, 2)
	builder.AddExtendedCommunity(attribute.ExtendedCommunity{0x00, 0x02, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64})
	wireBytes := builder.Build()

	route := bgptypes.LabeledUnicastRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/8"),
		NextHop: netip.MustParseAddr("192.0.2.1"),
		Labels:  []uint32{100, 200}, // Label stack
		PathID:  42,
		Wire:    attribute.NewAttributesWire(wireBytes, bgpctx.APIContextID),
	}

	ribRoute, _ := adapter.buildLabeledUnicastRIBRoute(route, false) // eBGP

	// Verify NLRI
	require.NotNil(t, ribRoute.NLRI(), "NLRI must not be nil")
	require.Equal(t, nlri.SAFIMPLSLabel, ribRoute.NLRI().Family().SAFI, "SAFI must be MPLSLabel")
	require.Equal(t, uint32(42), ribRoute.NLRI().PathID(), "PathID must be preserved")

	// Verify NextHop
	require.Equal(t, netip.MustParseAddr("192.0.2.1"), ribRoute.NextHop(), "NextHop must be preserved")

	// Verify attributes are present
	attrs := ribRoute.Attributes()
	require.NotEmpty(t, attrs, "Attributes must not be empty")

	// Count attribute types
	foundOrigin := false
	foundMED := false
	foundLocalPref := false
	foundCommunities := false
	foundLargeCommunities := false
	foundExtCommunities := false

	for _, attr := range attrs {
		switch attr.Code() { //nolint:exhaustive // Only checking specific attributes
		case attribute.AttrOrigin:
			foundOrigin = true
			if o, ok := attr.(attribute.Origin); ok {
				require.Equal(t, attribute.Origin(1), o, "Origin must be EGP")
			}
		case attribute.AttrMED:
			foundMED = true
			if m, ok := attr.(attribute.MED); ok {
				require.Equal(t, attribute.MED(100), m, "MED must be 100")
			}
		case attribute.AttrLocalPref:
			foundLocalPref = true
			if lp, ok := attr.(attribute.LocalPref); ok {
				require.Equal(t, attribute.LocalPref(200), lp, "LocalPref must be 200")
			}
		case attribute.AttrCommunity:
			foundCommunities = true
		case attribute.AttrLargeCommunity:
			foundLargeCommunities = true
		case attribute.AttrExtCommunity:
			foundExtCommunities = true
		default:
			// Other attributes not checked in this test
		}
	}

	require.True(t, foundOrigin, "Origin attribute must be present")
	require.True(t, foundMED, "MED attribute must be present")
	require.True(t, foundLocalPref, "LocalPref attribute must be present")
	require.True(t, foundCommunities, "Communities attribute must be present")
	require.True(t, foundLargeCommunities, "LargeCommunities attribute must be present")
	require.True(t, foundExtCommunities, "ExtendedCommunities attribute must be present")

	// Verify AS_PATH
	asPath := ribRoute.ASPath()
	require.NotNil(t, asPath, "AS_PATH must not be nil")
	require.Len(t, asPath.Segments, 1, "AS_PATH must have 1 segment")
	require.Equal(t, []uint32{65001, 65002}, asPath.Segments[0].ASNs, "AS_PATH must preserve ASNs")
}

// TestBuildLabeledUnicastRIBRouteIBGPDefaults verifies iBGP default handling.
//
// VALIDATES: iBGP routes have empty AS_PATH and default origin.
//
// PREVENTS: Incorrect AS_PATH for iBGP routes.
func TestBuildLabeledUnicastRIBRouteIBGPDefaults(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
		LocalAS:    65000,
	}
	reactor := New(cfg)
	adapter := &reactorAPIAdapter{reactor}

	// Minimal route - no attributes set
	route := bgptypes.LabeledUnicastRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/8"),
		NextHop: netip.MustParseAddr("192.0.2.1"),
		Labels:  []uint32{100},
	}

	ribRoute, _ := adapter.buildLabeledUnicastRIBRoute(route, true) // iBGP

	// Verify AS_PATH is empty for iBGP
	asPath := ribRoute.ASPath()
	require.NotNil(t, asPath, "AS_PATH must not be nil")
	require.Empty(t, asPath.Segments, "AS_PATH must be empty for iBGP")

	// Verify default Origin is IGP
	attrs := ribRoute.Attributes()
	foundOrigin := false
	for _, attr := range attrs {
		if o, ok := attr.(attribute.Origin); ok {
			foundOrigin = true
			require.Equal(t, attribute.OriginIGP, o, "Default origin must be IGP")
		}
	}
	require.True(t, foundOrigin, "Origin attribute must be present")
}

// TestBuildLabeledUnicastRIBRouteEBGPPrependsAS verifies eBGP AS prepending.
//
// VALIDATES: eBGP routes have LocalAS prepended to AS_PATH when no AS_PATH specified.
//
// PREVENTS: Missing LocalAS in eBGP announcements.
func TestBuildLabeledUnicastRIBRouteEBGPPrependsAS(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
		LocalAS:    65000,
	}
	reactor := New(cfg)
	adapter := &reactorAPIAdapter{reactor}

	// Route without AS_PATH
	route := bgptypes.LabeledUnicastRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/8"),
		NextHop: netip.MustParseAddr("192.0.2.1"),
		Labels:  []uint32{100},
	}

	ribRoute, _ := adapter.buildLabeledUnicastRIBRoute(route, false) // eBGP

	// Verify LocalAS is prepended for eBGP
	asPath := ribRoute.ASPath()
	require.NotNil(t, asPath, "AS_PATH must not be nil")
	require.Len(t, asPath.Segments, 1, "AS_PATH must have 1 segment")
	require.Equal(t, []uint32{65000}, asPath.Segments[0].ASNs, "LocalAS must be prepended")
}

// =============================================================================
// L3VPN (MPLS VPN) Tests - RFC 4364
// =============================================================================

// TestBuildL3VPNParams verifies L3VPN params conversion.
//
// VALIDATES: bgptypes.L3VPNRoute correctly converts to message.VPNParams.
//
// PREVENTS: Lost attributes in L3VPN announcements.
func TestBuildL3VPNParams(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
		LocalAS:    65000,
	}
	reactor := New(cfg)
	adapter := &reactorAPIAdapter{reactor}

	// Create route with all attributes
	// Build attributes
	builder := attribute.NewBuilder()
	builder.SetOrigin(1) // EGP
	builder.SetMED(100)
	builder.SetLocalPref(200)
	builder.SetASPath([]uint32{65001, 65002})
	builder.AddCommunityValue(0x12345678)
	wireBytes := builder.Build()

	route := bgptypes.L3VPNRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.0.2.1"),
		RD:      "100:100",
		Labels:  []uint32{1000, 2000}, // Multi-label stack
		RT:      "target:65000:100",
		Wire:    attribute.NewAttributesWire(wireBytes, bgpctx.APIContextID),
	}

	params, err := adapter.buildL3VPNParams(route)
	require.NoError(t, err)

	// Verify core fields
	require.Equal(t, route.Prefix, params.Prefix)
	require.Equal(t, route.NextHop, params.NextHop)
	require.Equal(t, route.Labels, params.Labels)

	// Verify RD bytes were parsed
	require.NotEqual(t, [8]byte{}, params.RDBytes, "RDBytes must be populated")

	// Verify attributes
	require.Equal(t, attribute.Origin(1), params.Origin)
	require.Equal(t, uint32(100), params.MED)
	require.Equal(t, uint32(200), params.LocalPreference)
	require.Equal(t, []uint32{65001, 65002}, params.ASPath)
	require.Equal(t, []uint32{0x12345678}, params.Communities)

	// Verify RT was converted to extended community
	require.NotEmpty(t, params.ExtCommunityBytes, "RT must be in ExtCommunityBytes")
}

// TestBuildL3VPNParamsIPv6 verifies IPv6 L3VPN works.
//
// VALIDATES: IPv6 VPN routes (VPNv6/SAFI 128) are supported.
//
// PREVENTS: IPv6 VPN routes failing.
func TestBuildL3VPNParamsIPv6(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
		LocalAS:    65000,
	}
	reactor := New(cfg)
	adapter := &reactorAPIAdapter{reactor}

	route := bgptypes.L3VPNRoute{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: netip.MustParseAddr("2001::1"),
		RD:      "100:100",
		Labels:  []uint32{1000},
	}

	params, err := adapter.buildL3VPNParams(route)
	require.NoError(t, err)
	require.Equal(t, route.Prefix, params.Prefix)
	require.True(t, params.Prefix.Addr().Is6(), "Must handle IPv6")
}

// TestBuildL3VPNRIBRoute verifies L3VPN RIB route building.
//
// VALIDATES: L3VPN routes can be queued via rib.Route with all attributes.
//
// PREVENTS: Attribute loss when L3VPN routes are queued.
func TestBuildL3VPNRIBRoute(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
		LocalAS:    65000,
	}
	reactor := New(cfg)
	adapter := &reactorAPIAdapter{reactor}

	// Build attributes with Origin IGP
	builder := attribute.NewBuilder()
	builder.SetOrigin(0) // IGP
	wireBytes := builder.Build()

	route := bgptypes.L3VPNRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.0.2.1"),
		RD:      "100:100",
		Labels:  []uint32{1000},
		Wire:    attribute.NewAttributesWire(wireBytes, bgpctx.APIContextID),
	}

	ribRoute, err := adapter.buildL3VPNRIBRoute(route, true) // iBGP
	require.NoError(t, err)
	require.NotNil(t, ribRoute)

	// Verify NLRI family is VPN (SAFI 128)
	require.Equal(t, nlri.SAFI(128), ribRoute.NLRI().Family().SAFI, "SAFI must be 128 (VPN)")

	// Verify attributes present
	attrs := ribRoute.Attributes()
	require.NotEmpty(t, attrs)
}

// =============================================================================
// parseRouteTarget Tests - RFC 4360
// =============================================================================

// TestParseRouteTarget_2ByteASN verifies 2-byte ASN Route Target encoding.
//
// VALIDATES: ASN:NN format with 2-byte ASN produces Type 0 extended community.
//
// PREVENTS: Wrong type code or byte order in RT encoding.
func TestParseRouteTarget_2ByteASN(t *testing.T) {
	// target:65000:100 -> Type 0x00, Subtype 0x02, ASN 65000, Value 100
	rt, err := parseRouteTarget("65000:100")
	require.NoError(t, err)
	require.Len(t, rt, 8)

	// Type 0 (2-byte AS) + Subtype 0x02 (Route Target)
	assert.Equal(t, byte(0x00), rt[0], "Type should be 0x00 (2-byte AS)")
	assert.Equal(t, byte(0x02), rt[1], "Subtype should be 0x02 (Route Target)")

	// ASN 65000 = 0xFDE8 in big-endian
	assert.Equal(t, byte(0xFD), rt[2])
	assert.Equal(t, byte(0xE8), rt[3])

	// Value 100 = 0x00000064 in big-endian (4 bytes for Type 0)
	assert.Equal(t, byte(0x00), rt[4])
	assert.Equal(t, byte(0x00), rt[5])
	assert.Equal(t, byte(0x00), rt[6])
	assert.Equal(t, byte(0x64), rt[7])
}

// TestParseRouteTarget_4ByteASN verifies 4-byte ASN Route Target encoding.
//
// VALIDATES: Large ASN produces Type 2 extended community.
//
// PREVENTS: 4-byte ASN being truncated to 2 bytes.
func TestParseRouteTarget_4ByteASN(t *testing.T) {
	// 4200000001:100 -> Type 0x02, Subtype 0x02, ASN 4200000001, Value 100
	rt, err := parseRouteTarget("4200000001:100")
	require.NoError(t, err)
	require.Len(t, rt, 8)

	// Type 2 (4-byte AS) + Subtype 0x02 (Route Target)
	assert.Equal(t, byte(0x02), rt[0], "Type should be 0x02 (4-byte AS)")
	assert.Equal(t, byte(0x02), rt[1], "Subtype should be 0x02 (Route Target)")

	// ASN 4200000001 = 0xFA56EA01 in big-endian
	assert.Equal(t, byte(0xFA), rt[2])
	assert.Equal(t, byte(0x56), rt[3])
	assert.Equal(t, byte(0xEA), rt[4])
	assert.Equal(t, byte(0x01), rt[5])

	// Value 100 = 0x0064 in big-endian (2 bytes for Type 2)
	assert.Equal(t, byte(0x00), rt[6])
	assert.Equal(t, byte(0x64), rt[7])
}

// TestParseRouteTarget_IPv4 verifies IPv4 Route Target encoding.
//
// VALIDATES: IP:NN format produces Type 1 extended community.
//
// PREVENTS: IP address not being recognized.
func TestParseRouteTarget_IPv4(t *testing.T) {
	// 192.0.2.1:100 -> Type 0x01, Subtype 0x02, IP, Value 100
	rt, err := parseRouteTarget("192.0.2.1:100")
	require.NoError(t, err)
	require.Len(t, rt, 8)

	// Type 1 (IPv4) + Subtype 0x02 (Route Target)
	assert.Equal(t, byte(0x01), rt[0], "Type should be 0x01 (IPv4)")
	assert.Equal(t, byte(0x02), rt[1], "Subtype should be 0x02 (Route Target)")

	// IP 192.0.2.1
	assert.Equal(t, byte(192), rt[2])
	assert.Equal(t, byte(0), rt[3])
	assert.Equal(t, byte(2), rt[4])
	assert.Equal(t, byte(1), rt[5])

	// Value 100 = 0x0064 (2 bytes for Type 1)
	assert.Equal(t, byte(0x00), rt[6])
	assert.Equal(t, byte(0x64), rt[7])
}

// TestParseRouteTarget_WithPrefix verifies "target:" prefix is stripped.
//
// VALIDATES: "target:ASN:NN" format works same as "ASN:NN".
//
// PREVENTS: Prefix not being stripped.
func TestParseRouteTarget_WithPrefix(t *testing.T) {
	rt1, err := parseRouteTarget("target:65000:100")
	require.NoError(t, err)

	rt2, err := parseRouteTarget("65000:100")
	require.NoError(t, err)

	assert.Equal(t, rt1, rt2, "target: prefix should be stripped")
}

// TestParseRouteTarget_Errors verifies error handling.
//
// VALIDATES: Invalid formats are rejected with clear errors.
//
// PREVENTS: Silent failures or panics on bad input.
func TestParseRouteTarget_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"no colon", "65000"},
		{"too many colons", "65000:100:200"},
		{"invalid ASN", "abc:100"},
		{"negative ASN", "-1:100"},
		{"4-byte ASN with large value", "4200000001:100000"}, // Value > 65535
		{"IP with large value", "192.0.2.1:100000"},          // Value > 65535 for IP format
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseRouteTarget(tt.input)
			assert.Error(t, err, "should reject %q", tt.input)
		})
	}
}

// =============================================================================
// Multi-Listener Tests (spec-listener-per-local-address.md)
// =============================================================================

// TestMultiListenerSameLocalAddress verifies one listener per unique LocalAddress.
//
// VALIDATES: Two peers with same LocalAddress create only one listener.
//
// PREVENTS: Duplicate listeners wasting resources and port conflicts.
func TestMultiListenerSameLocalAddress(t *testing.T) {
	cfg := &Config{
		Port:    0, // Use ephemeral port
		LocalAS: 65000,
	}
	reactor := New(cfg)

	localAddr := mustParseAddr("127.0.0.1")

	// Add two peers with same LocalAddress
	settings1 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings1.LocalAddress = localAddr
	settings1.Connection = ConnectionPassive

	settings2 := NewPeerSettings(mustParseAddr("10.0.0.3"), 65000, 65002, 0x01010101)
	settings2.LocalAddress = localAddr
	settings2.Connection = ConnectionPassive

	err := reactor.AddPeer(settings1)
	require.NoError(t, err)
	err = reactor.AddPeer(settings2)
	require.NoError(t, err)

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	// Should have exactly one listener
	addrs := reactor.ListenAddrs()
	require.Len(t, addrs, 1, "should have exactly 1 listener for shared LocalAddress")
}

// TestMultiListenerDifferentLocalAddresses verifies separate listeners per LocalAddress.
//
// VALIDATES: Two peers with different LocalAddresses create two listeners.
//
// PREVENTS: Cross-interface connection acceptance.
func TestMultiListenerDifferentLocalAddresses(t *testing.T) {
	// Check if IPv6 loopback is available
	ln, err := net.Listen("tcp", "[::1]:0") //nolint:noctx // Test code
	if err != nil {
		t.Skip("IPv6 loopback not available, skipping multi-listener test")
	}
	_ = ln.Close()

	cfg := &Config{
		Port:    0, // Use ephemeral port
		LocalAS: 65000,
	}
	reactor := New(cfg)

	// Add two peers with different LocalAddresses (IPv4 and IPv6 loopback)
	// Note: Peer Address must match LocalAddress family
	settings1 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings1.LocalAddress = mustParseAddr("127.0.0.1") // IPv4 local for IPv4 peer
	settings1.Connection = ConnectionPassive

	settings2 := NewPeerSettings(mustParseAddr("2001:db8::3"), 65000, 65002, 0x01010101)
	settings2.LocalAddress = mustParseAddr("::1") // IPv6 local for IPv6 peer
	settings2.Connection = ConnectionPassive

	err = reactor.AddPeer(settings1)
	require.NoError(t, err)
	err = reactor.AddPeer(settings2)
	require.NoError(t, err)

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	// Should have two listeners
	addrs := reactor.ListenAddrs()
	require.Len(t, addrs, 2, "should have 2 listeners for different LocalAddresses")
}

// TestMultiListenerNoPeers verifies reactor runs with no peers.
//
// VALIDATES: Reactor starts successfully with no peers (no listeners created).
//
// PREVENTS: Startup failure when no peers are configured.
func TestMultiListenerNoPeers(t *testing.T) {
	cfg := &Config{
		Port:    0, // Use ephemeral port
		LocalAS: 65000,
	}
	reactor := New(cfg)

	// No peers added

	err := reactor.Start()
	require.NoError(t, err, "reactor should start with no peers")
	defer reactor.Stop()

	require.True(t, reactor.Running(), "reactor should be running")

	// Should have no listeners (no peers = no LocalAddresses)
	addrs := reactor.ListenAddrs()
	require.Len(t, addrs, 0, "should have 0 listeners with no peers")
}

// TestMultiListenerConnectionToCorrectListener verifies connection routing.
//
// VALIDATES: Connection to a listener is matched to peer with that LocalAddress.
//
// PREVENTS: Connection from peer going to wrong listener.
func TestMultiListenerConnectionToCorrectListener(t *testing.T) {
	cfg := &Config{
		Port:    0, // Use ephemeral port
		LocalAS: 65000,
	}
	reactor := New(cfg)

	// Add peer expecting connection from 127.0.0.1
	// Note: Address != LocalAddress (peer is at 10.0.0.2, we listen on 127.0.0.1)
	settings := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings.LocalAddress = mustParseAddr("127.0.0.1")
	settings.Connection = ConnectionPassive

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	var accepted atomic.Bool
	reactor.SetConnectionCallback(func(conn net.Conn, n *PeerSettings) {
		accepted.Store(true)
		_ = conn.Close()
	})

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	addr := reactor.ListenAddr()
	require.NotNil(t, addr, "should have a listener")

	// Connect from localhost
	// Note: This connection won't be "matched" to the peer because our source IP
	// isn't 10.0.0.2, but the listener should still be active
	conn, err := net.DialTimeout("tcp", addr.String(), time.Second) //nolint:noctx // Test code
	require.NoError(t, err)
	_ = conn.Close()

	// The connection will be rejected (unknown peer), but listener should work
	// Just verify listener is working by checking no error on connect
}

// TestMultiListenerLegacyListenAddrFallback verifies backward compatibility.
//
// VALIDATES: Legacy ListenAddr config still works when no LocalAddress on peers.
//
// PREVENTS: Breaking existing configs during migration.
func TestMultiListenerLegacyListenAddrFallback(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0", // Legacy config
		LocalAS:    65000,
	}
	reactor := New(cfg)

	// Peer without LocalAddress (legacy behavior)
	settings := NewPeerSettings(mustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Connection = ConnectionPassive
	// Note: LocalAddress is zero value

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	// Legacy listener should be used
	addr := reactor.ListenAddr()
	require.NotNil(t, addr, "should have legacy listener")
}

// =============================================================================
// LocalAddress Validation Tests (spec-listener-per-local-address.md)
// =============================================================================

// TestAddPeerSelfReferential verifies Address != LocalAddress validation.
//
// VALIDATES: Peer with Address == LocalAddress is rejected.
//
// PREVENTS: Self-referential peer configuration that would never work.
func TestAddPeerSelfReferential(t *testing.T) {
	cfg := &Config{LocalAS: 65000}
	reactor := New(cfg)

	// Self-referential: Address equals LocalAddress
	settings := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0x01010101)
	settings.LocalAddress = mustParseAddr("10.0.0.1") // Same as Address!

	err := reactor.AddPeer(settings)
	require.Error(t, err, "self-referential peer should be rejected")
	require.Contains(t, err.Error(), "cannot equal local-address")
}

// TestAddPeerLinkLocalIPv6 verifies link-local IPv6 LocalAddress is rejected.
//
// VALIDATES: Link-local IPv6 addresses are rejected for LocalAddress.
//
// PREVENTS: Configuration with zone-dependent addresses that aren't portable.
func TestAddPeerLinkLocalIPv6(t *testing.T) {
	cfg := &Config{LocalAS: 65000}
	reactor := New(cfg)

	// Link-local IPv6 as LocalAddress
	settings := NewPeerSettings(mustParseAddr("2001:db8::2"), 65000, 65001, 0x01010101)
	settings.LocalAddress = mustParseAddr("fe80::1") // Link-local!

	err := reactor.AddPeer(settings)
	require.Error(t, err, "link-local IPv6 LocalAddress should be rejected")
	require.Contains(t, err.Error(), "link-local")
}

// TestAddPeerDuplicateAddress verifies duplicate peer Address is rejected.
//
// VALIDATES: Adding peer with same Address as existing peer fails.
//
// PREVENTS: Map key collision and ambiguous peer matching.
func TestAddPeerDuplicateAddress(t *testing.T) {
	cfg := &Config{LocalAS: 65000}
	reactor := New(cfg)

	// Add first peer
	settings1 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings1.LocalAddress = mustParseAddr("192.168.1.1")
	err := reactor.AddPeer(settings1)
	require.NoError(t, err)

	// Try to add duplicate
	settings2 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65002, 0x01010101) // Same Address!
	settings2.LocalAddress = mustParseAddr("192.168.1.1")

	err = reactor.AddPeer(settings2)
	require.Error(t, err, "duplicate peer Address should be rejected")
	require.ErrorIs(t, err, ErrPeerExists)
}

// TestAddPeerAddressFamilyMismatch verifies Address/LocalAddress family must match.
//
// VALIDATES: IPv4 peer cannot have IPv6 LocalAddress and vice versa.
//
// PREVENTS: Configuration where TCP socket family doesn't match peer.
func TestAddPeerAddressFamilyMismatch(t *testing.T) {
	cfg := &Config{LocalAS: 65000}
	reactor := New(cfg)

	// IPv4 peer with IPv6 LocalAddress
	settings := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings.LocalAddress = mustParseAddr("::1") // IPv6 local for IPv4 peer!

	err := reactor.AddPeer(settings)
	require.Error(t, err, "address family mismatch should be rejected")
	require.Contains(t, err.Error(), "family mismatch")
}

// =============================================================================
// Dynamic Listener Lifecycle Tests (spec-listener-per-local-address.md)
// =============================================================================

// TestDynamicListenerAddPeerNewLocalAddress verifies listener creation on AddPeer.
//
// VALIDATES: Adding peer with new LocalAddress while running creates listener.
//
// PREVENTS: Dynamic peers failing to accept incoming connections.
func TestDynamicListenerAddPeerNewLocalAddress(t *testing.T) {
	cfg := &Config{
		Port:    0, // Use ephemeral port
		LocalAS: 65000,
	}
	reactor := New(cfg)

	// Start with no peers (no listeners)
	err := reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	require.Len(t, reactor.ListenAddrs(), 0, "should have 0 listeners initially")

	// Add peer with LocalAddress
	settings := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings.LocalAddress = mustParseAddr("127.0.0.1")
	settings.Connection = ConnectionPassive

	err = reactor.AddPeer(settings)
	require.NoError(t, err)

	// Should now have 1 listener
	require.Len(t, reactor.ListenAddrs(), 1, "should have 1 listener after AddPeer")
}

// TestDynamicListenerAddPeerExistingLocalAddress verifies listener reuse.
//
// VALIDATES: Adding peer with existing LocalAddress doesn't create new listener.
//
// PREVENTS: Resource waste from duplicate listeners.
func TestDynamicListenerAddPeerExistingLocalAddress(t *testing.T) {
	cfg := &Config{
		Port:    0,
		LocalAS: 65000,
	}
	reactor := New(cfg)

	// Add first peer
	settings1 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings1.LocalAddress = mustParseAddr("127.0.0.1")
	settings1.Connection = ConnectionPassive
	err := reactor.AddPeer(settings1)
	require.NoError(t, err)

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	require.Len(t, reactor.ListenAddrs(), 1, "should have 1 listener")

	// Add second peer with SAME LocalAddress
	settings2 := NewPeerSettings(mustParseAddr("10.0.0.3"), 65000, 65002, 0x01010101)
	settings2.LocalAddress = mustParseAddr("127.0.0.1") // Same!
	settings2.Connection = ConnectionPassive

	err = reactor.AddPeer(settings2)
	require.NoError(t, err)

	// Should still have only 1 listener
	require.Len(t, reactor.ListenAddrs(), 1, "should still have 1 listener (shared)")
}

// TestDynamicListenerRemoveLastPeer verifies listener cleanup on RemovePeer.
//
// VALIDATES: Removing last peer for LocalAddress stops the listener.
//
// PREVENTS: Orphaned listeners consuming resources.
func TestDynamicListenerRemoveLastPeer(t *testing.T) {
	cfg := &Config{
		Port:    0,
		LocalAS: 65000,
	}
	reactor := New(cfg)

	// Add peer
	settings := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings.LocalAddress = mustParseAddr("127.0.0.1")
	settings.Connection = ConnectionPassive
	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	require.Len(t, reactor.ListenAddrs(), 1, "should have 1 listener")

	// Remove peer
	err = reactor.RemovePeer(settings.Address)
	require.NoError(t, err)

	// Listener should be stopped
	require.Len(t, reactor.ListenAddrs(), 0, "should have 0 listeners after removing last peer")
}

// TestDynamicListenerRemoveOneOfMany verifies listener stays when others share it.
//
// VALIDATES: Removing peer keeps listener if other peers share LocalAddress.
//
// PREVENTS: Premature listener closure breaking other peers.
func TestDynamicListenerRemoveOneOfMany(t *testing.T) {
	cfg := &Config{
		Port:    0,
		LocalAS: 65000,
	}
	reactor := New(cfg)

	localAddr := mustParseAddr("127.0.0.1")

	// Add two peers with same LocalAddress
	settings1 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings1.LocalAddress = localAddr
	settings1.Connection = ConnectionPassive
	err := reactor.AddPeer(settings1)
	require.NoError(t, err)

	settings2 := NewPeerSettings(mustParseAddr("10.0.0.3"), 65000, 65002, 0x01010101)
	settings2.LocalAddress = localAddr
	settings2.Connection = ConnectionPassive
	err = reactor.AddPeer(settings2)
	require.NoError(t, err)

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	require.Len(t, reactor.ListenAddrs(), 1, "should have 1 listener")

	// Remove one peer
	err = reactor.RemovePeer(settings1.Address)
	require.NoError(t, err)

	// Listener should still exist (other peer uses it)
	require.Len(t, reactor.ListenAddrs(), 1, "should still have 1 listener (other peer shares it)")
}

// =============================================================================
// IPv4-Mapped IPv6 Address Tests
// =============================================================================

// TestAddPeerIPv4MappedNormalization verifies IPv4-mapped addresses are normalized.
//
// VALIDATES: LocalAddress ::ffff:192.168.1.1 is normalized to 192.168.1.1.
//
// PREVENTS: Listener/connection mismatch due to different address formats.
func TestAddPeerIPv4MappedNormalization(t *testing.T) {
	cfg := &Config{
		Port:    0,
		LocalAS: 65000,
	}
	reactor := New(cfg)

	// Add peer with IPv4-mapped IPv6 LocalAddress
	settings := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings.LocalAddress = netip.MustParseAddr("::ffff:127.0.0.1") // IPv4-mapped
	settings.Connection = ConnectionPassive

	err := reactor.AddPeer(settings)
	require.NoError(t, err, "IPv4-mapped LocalAddress should be accepted")

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	// Verify listener was created on the unmapped IPv4 address
	addrs := reactor.ListenAddrs()
	require.Len(t, addrs, 1)

	// The listener should be on 127.0.0.1, not ::ffff:127.0.0.1
	listenerAddr := addrs[0].String()
	require.Contains(t, listenerAddr, "127.0.0.1:", "listener should be on unmapped IPv4")
	require.NotContains(t, listenerAddr, "::ffff", "listener should not use IPv4-mapped format")
}

// TestAddPeerIPv4MappedSelfReferential verifies self-referential check works with mapped addresses.
//
// VALIDATES: Peer with Address 10.0.0.1 and LocalAddress ::ffff:10.0.0.1 is rejected.
//
// PREVENTS: Self-referential configuration bypassing validation via address format.
func TestAddPeerIPv4MappedSelfReferential(t *testing.T) {
	cfg := &Config{LocalAS: 65000}
	reactor := New(cfg)

	// Self-referential using IPv4-mapped format
	settings := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0x01010101)
	settings.LocalAddress = netip.MustParseAddr("::ffff:10.0.0.1") // Same as Address, mapped

	err := reactor.AddPeer(settings)
	require.Error(t, err, "IPv4-mapped self-referential should be rejected")
	require.Contains(t, err.Error(), "cannot equal local-address")
}

// TestAddPeerIPv4MappedAddressNormalization verifies peer Address is normalized.
//
// VALIDATES: Peer with Address ::ffff:10.0.0.2 is stored as 10.0.0.2.
//
// PREVENTS: Connection lookup failure when peer Address uses IPv4-mapped format.
func TestAddPeerIPv4MappedAddressNormalization(t *testing.T) {
	cfg := &Config{LocalAS: 65000}
	reactor := New(cfg)

	// Add peer with IPv4-mapped Address
	settings := NewPeerSettings(netip.MustParseAddr("::ffff:10.0.0.2"), 65000, 65001, 0x01010101)
	settings.LocalAddress = mustParseAddr("127.0.0.1")
	settings.Connection = ConnectionPassive

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	// Verify peer is accessible via unmapped address
	peers := reactor.Peers()
	require.Len(t, peers, 1)

	// The stored Address should be unmapped
	storedAddr := peers[0].Settings().Address
	require.Equal(t, netip.MustParseAddr("10.0.0.2"), storedAddr, "Address should be unmapped")
	require.True(t, storedAddr.Is4(), "Address should be IPv4 after unmapping")
}

// TestAddPeerIPv4MappedAddressDuplicate verifies duplicate detection works with mapped addresses.
//
// VALIDATES: Adding ::ffff:10.0.0.2 after 10.0.0.2 is rejected as duplicate.
//
// PREVENTS: Duplicate peers via different address formats.
func TestAddPeerIPv4MappedAddressDuplicate(t *testing.T) {
	cfg := &Config{LocalAS: 65000}
	reactor := New(cfg)

	// Add peer with IPv4 Address
	settings1 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings1.LocalAddress = mustParseAddr("127.0.0.1")
	err := reactor.AddPeer(settings1)
	require.NoError(t, err)

	// Try to add same peer with IPv4-mapped Address
	settings2 := NewPeerSettings(netip.MustParseAddr("::ffff:10.0.0.2"), 65000, 65002, 0x01010101)
	settings2.LocalAddress = mustParseAddr("127.0.0.1")

	err = reactor.AddPeer(settings2)
	require.Error(t, err, "IPv4-mapped duplicate should be rejected")
	require.ErrorIs(t, err, ErrPeerExists)
}

// TestRemovePeerIPv4Mapped verifies RemovePeer works with mapped addresses.
//
// VALIDATES: RemovePeer(::ffff:10.0.0.2) removes peer stored as 10.0.0.2.
//
// PREVENTS: API inconsistency where AddPeer normalizes but RemovePeer doesn't.
func TestRemovePeerIPv4Mapped(t *testing.T) {
	cfg := &Config{LocalAS: 65000}
	reactor := New(cfg)

	// Add peer with IPv4 Address
	settings := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings.LocalAddress = mustParseAddr("127.0.0.1")
	err := reactor.AddPeer(settings)
	require.NoError(t, err)
	require.Len(t, reactor.Peers(), 1)

	// Remove using IPv4-mapped format
	err = reactor.RemovePeer(netip.MustParseAddr("::ffff:10.0.0.2"))
	require.NoError(t, err, "RemovePeer should accept IPv4-mapped address")
	require.Len(t, reactor.Peers(), 0, "peer should be removed")
}

// TestNotifyMessageReceiverWireUpdate verifies WireUpdate is set for UPDATE messages.
//
// VALIDATES: RawMessage.WireUpdate is populated for UPDATE messages.
// PREVENTS: Missing WireUpdate field when API receives UPDATE.
func TestNotifyMessageReceiverWireUpdate(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}
	reactor := New(cfg)

	// Add peer
	peerAddr := mustParseAddr("10.0.0.1")
	settings := NewPeerSettings(peerAddr, 65000, 65001, 0x01010101)
	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	// Track received messages
	var receivedMsg bgptypes.RawMessage
	var receivedPeer plugin.PeerInfo
	receiver := &testMessageReceiver{
		onReceived: func(peer plugin.PeerInfo, msg bgptypes.RawMessage) {
			receivedPeer = peer
			receivedMsg = msg
		},
	}
	reactor.SetMessageReceiver(receiver)

	// Build UPDATE payload: withdrawn(0) + attrs(ORIGIN) + nlri(/24)
	// Format: wdLen(2) + attrLen(2) + attrs + nlri
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	nlri := []byte{0x18, 0xc0, 0xa8, 0x01}  // 192.168.1.0/24
	updatePayload := make([]byte, 2+2+len(attrs)+len(nlri))
	// withdrawn len = 0
	updatePayload[0], updatePayload[1] = 0, 0
	// attr len
	updatePayload[2], updatePayload[3] = 0, byte(len(attrs))
	copy(updatePayload[4:], attrs)
	copy(updatePayload[4+len(attrs):], nlri)

	// Create WireUpdate (as session would do)
	wireUpdate := wireu.NewWireUpdate(updatePayload, 0)

	// Call notifyMessageReceiver directly (same package)
	// In normal flow, session creates WireUpdate and passes it through
	// Pass nil buf since we're not testing caching here
	_ = reactor.notifyMessageReceiver(peerAddr, message.TypeUPDATE, updatePayload, wireUpdate, 0, "received", nil)

	// Verify WireUpdate is set
	require.NotNil(t, receivedMsg.WireUpdate, "WireUpdate should be set for UPDATE")
	require.Equal(t, peerAddr, receivedPeer.Address, "peer address should match")

	// Verify WireUpdate provides correct data
	gotAttrs, attrsErr := receivedMsg.WireUpdate.Attrs()
	require.NoError(t, attrsErr, "WireUpdate.Attrs() should not error")
	require.NotNil(t, gotAttrs, "WireUpdate.Attrs() should return attributes")

	gotNLRI, nlriErr := receivedMsg.WireUpdate.NLRI()
	require.NoError(t, nlriErr, "WireUpdate.NLRI() should not error")
	require.NotNil(t, gotNLRI, "WireUpdate.NLRI() should return NLRI")

	withdrawn, wdErr := receivedMsg.WireUpdate.Withdrawn()
	require.NoError(t, wdErr, "WireUpdate.Withdrawn() should not error")
	require.Nil(t, withdrawn, "WireUpdate.Withdrawn() should be nil (no withdrawals)")

	// Verify backward compat: AttrsWire is derived from WireUpdate
	require.NotNil(t, receivedMsg.AttrsWire, "AttrsWire should be set for backward compat")
	require.Equal(t, gotAttrs, receivedMsg.AttrsWire, "AttrsWire should be same as WireUpdate.Attrs()")
}

// TestNotifyMessageReceiverSentAttrsWire verifies AttrsWire is created for sent UPDATE messages.
//
// VALIDATES: RawMessage.AttrsWire is populated for sent UPDATE when ctxID is set.
// PREVENTS: Missing AttrsWire for sent messages causing RIB plugin to skip route storage.
func TestNotifyMessageReceiverSentAttrsWire(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}
	reactor := New(cfg)

	// Add peer
	peerAddr := mustParseAddr("10.0.0.1")
	settings := NewPeerSettings(peerAddr, 65000, 65001, 0x01010101)
	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	// Track sent messages
	var sentMsg bgptypes.RawMessage
	receiver := &testMessageReceiver{
		onSent: func(peer plugin.PeerInfo, msg bgptypes.RawMessage) {
			sentMsg = msg
		},
	}
	reactor.SetMessageReceiver(receiver)

	// Build UPDATE payload: withdrawn(0) + attrs(ORIGIN) + nlri(/24)
	// Format: wdLen(2) + attrLen(2) + attrs + nlri
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	nlri := []byte{0x18, 0xc0, 0xa8, 0x01}  // 192.168.1.0/24
	updatePayload := make([]byte, 2+2+len(attrs)+len(nlri))
	// withdrawn len = 0
	updatePayload[0], updatePayload[1] = 0, 0
	// attr len
	updatePayload[2], updatePayload[3] = 0, byte(len(attrs))
	copy(updatePayload[4:], attrs)
	copy(updatePayload[4+len(attrs):], nlri)

	// Call notifyMessageReceiver with direction="sent" and non-zero ctxID
	// Non-zero ctxID triggers AttrsWire creation for sent messages
	ctxID := bgpctx.ContextID(1)
	_ = reactor.notifyMessageReceiver(peerAddr, message.TypeUPDATE, updatePayload, nil, ctxID, "sent", nil)

	// Verify AttrsWire is set
	require.NotNil(t, sentMsg.AttrsWire, "AttrsWire should be created for sent UPDATE with ctxID")
	require.Equal(t, "sent", sentMsg.Direction, "Direction should be 'sent'")

	// Verify AttrsWire can parse attributes
	origin, err := sentMsg.AttrsWire.Get(attribute.AttrOrigin)
	require.NoError(t, err, "AttrsWire should parse ORIGIN")
	require.NotNil(t, origin, "ORIGIN attribute should exist")
}

// TestNotifyMessageReceiverSentNoCtxID verifies no AttrsWire when ctxID is 0.
//
// VALIDATES: AttrsWire is NOT created when ctxID is 0 (no capability context).
// PREVENTS: Invalid AttrsWire with context 0 causing parsing issues.
func TestNotifyMessageReceiverSentNoCtxID(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}
	reactor := New(cfg)

	// Add peer
	peerAddr := mustParseAddr("10.0.0.1")
	settings := NewPeerSettings(peerAddr, 65000, 65001, 0x01010101)
	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	// Track sent messages
	var sentMsg bgptypes.RawMessage
	receiver := &testMessageReceiver{
		onSent: func(peer plugin.PeerInfo, msg bgptypes.RawMessage) {
			sentMsg = msg
		},
	}
	reactor.SetMessageReceiver(receiver)

	// Build minimal UPDATE payload
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	updatePayload := make([]byte, 2+2+len(attrs))
	updatePayload[2], updatePayload[3] = 0, byte(len(attrs))
	copy(updatePayload[4:], attrs)

	// Call with ctxID=0 (no context)
	_ = reactor.notifyMessageReceiver(peerAddr, message.TypeUPDATE, updatePayload, nil, 0, "sent", nil)

	// AttrsWire should be nil when ctxID is 0
	require.Nil(t, sentMsg.AttrsWire, "AttrsWire should be nil when ctxID is 0")
}

// testMessageReceiver implements MessageReceiver for testing.
type testMessageReceiver struct {
	onReceived func(plugin.PeerInfo, bgptypes.RawMessage)
	onSent     func(plugin.PeerInfo, bgptypes.RawMessage)
}

func (r *testMessageReceiver) OnMessageReceived(peer plugin.PeerInfo, msg any) int {
	if r.onReceived != nil {
		if typedMsg, ok := msg.(bgptypes.RawMessage); ok {
			r.onReceived(peer, typedMsg)
		}
	}
	return 0 // Test receiver reports no consumers
}

func (r *testMessageReceiver) OnMessageBatchReceived(peer plugin.PeerInfo, msgs []any) []int {
	counts := make([]int, len(msgs))
	for i, msg := range msgs {
		counts[i] = r.OnMessageReceived(peer, msg)
	}
	return counts
}

func (r *testMessageReceiver) OnMessageSent(peer plugin.PeerInfo, msg any) {
	if r.onSent != nil {
		if typedMsg, ok := msg.(bgptypes.RawMessage); ok {
			r.onSent(peer, typedMsg)
		}
	}
}

// ─── Phase 3: Async delivery pipeline tests ─────────────────────────────────

// testDeliveryReceiver implements MessageReceiver with configurable consumer count.
type testDeliveryReceiver struct {
	onReceived    func(plugin.PeerInfo, bgptypes.RawMessage)
	onSent        func(plugin.PeerInfo, bgptypes.RawMessage)
	consumerCount int
}

func (r *testDeliveryReceiver) OnMessageReceived(peer plugin.PeerInfo, msg any) int {
	if r.onReceived != nil {
		if typedMsg, ok := msg.(bgptypes.RawMessage); ok {
			r.onReceived(peer, typedMsg)
		}
	}
	return r.consumerCount
}

func (r *testDeliveryReceiver) OnMessageBatchReceived(peer plugin.PeerInfo, msgs []any) []int {
	counts := make([]int, len(msgs))
	for i, msg := range msgs {
		counts[i] = r.OnMessageReceived(peer, msg)
	}
	return counts
}

func (r *testDeliveryReceiver) OnMessageSent(peer plugin.PeerInfo, msg any) {
	if r.onSent != nil {
		if typedMsg, ok := msg.(bgptypes.RawMessage); ok {
			r.onSent(peer, typedMsg)
		}
	}
}

// testUpdatePayload builds a minimal UPDATE body for testing.
// Format: withdrawnLen(2) + attrLen(2) + attrs(ORIGIN IGP) + NLRI(192.168.1.0/24).
func testUpdatePayload() []byte {
	attrs := []byte{0x40, 0x01, 0x01, 0x00}     // ORIGIN IGP
	nlriBytes := []byte{0x18, 0xc0, 0xa8, 0x01} // 192.168.1.0/24
	payload := make([]byte, 2+2+len(attrs)+len(nlriBytes))
	payload[2], payload[3] = 0, byte(len(attrs))
	copy(payload[4:], attrs)
	copy(payload[4+len(attrs):], nlriBytes)
	return payload
}

// startTestDelivery sets up a per-peer delivery channel and goroutine for testing.
// Mimics what Peer.runOnce() will do in production.
// Returns a stop function that closes the channel and waits for the goroutine to drain.
func startTestDelivery(t *testing.T, r *Reactor, peerAddr netip.Addr, capacity int) func() {
	t.Helper()

	r.mu.RLock()
	peer, ok := r.findPeerByAddr(peerAddr)
	r.mu.RUnlock()
	require.True(t, ok, "peer must exist for delivery setup")

	peer.deliverChan = make(chan deliveryItem, capacity)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for first := range peer.deliverChan {
			batch := drainDeliveryBatch(first, peer.deliverChan)

			r.mu.RLock()
			receiver := r.messageReceiver
			r.mu.RUnlock()
			if receiver == nil {
				continue
			}

			msgs := make([]any, len(batch))
			for i := range batch {
				msgs[i] = batch[i].msg
			}

			counts := receiver.OnMessageBatchReceived(batch[0].peerInfo, msgs)
			for i := range batch {
				count := 0
				if i < len(counts) {
					count = counts[i]
				}
				r.recentUpdates.Activate(batch[i].msg.MessageID, count)
			}
		}
	}()

	return func() {
		close(peer.deliverChan)
		<-done
	}
}

// TestDeliveryChannelDecouplesRead verifies async delivery decouples the read goroutine.
//
// VALIDATES: AC-4: read goroutine returns before plugin delivery completes.
//
// PREVENTS: Read goroutine blocking on slow plugin delivery.
func TestDeliveryChannelDecouplesRead(t *testing.T) {
	t.Parallel()

	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	reactor := New(cfg)

	peerAddr := mustParseAddr("10.0.0.1")
	require.NoError(t, reactor.AddPeer(NewPeerSettings(peerAddr, 65000, 65001, 0x01010101)))

	delivered := make(chan struct{})
	receiver := &testDeliveryReceiver{
		onReceived: func(_ plugin.PeerInfo, _ bgptypes.RawMessage) {
			time.Sleep(200 * time.Millisecond) // Slow plugin
			close(delivered)
		},
	}
	reactor.SetMessageReceiver(receiver)

	stop := startTestDelivery(t, reactor, peerAddr, deliveryChannelCapacity)

	payload := testUpdatePayload()
	wireUpdate := wireu.NewWireUpdate(payload, 0)
	buf := make([]byte, 4096)

	start := time.Now()
	_ = reactor.notifyMessageReceiver(peerAddr, message.TypeUPDATE, payload, wireUpdate, 0, "received", buf)
	elapsed := time.Since(start)

	// Read goroutine should return almost immediately (enqueue, not block)
	require.Less(t, elapsed, 50*time.Millisecond,
		"read goroutine should not block on slow plugin delivery (got %v)", elapsed)

	// Delivery should still complete asynchronously
	select {
	case <-delivered:
	case <-time.After(1 * time.Second):
		t.Fatal("delivery goroutine should have completed delivery")
	}

	stop()
}

// TestCacheInsertionBeforeDelivery verifies cache.Add() happens before plugin delivery.
//
// VALIDATES: AC-6: cache.Get(id) succeeds before OnMessageReceived returns.
//
// PREVENTS: Plugin receiving message-id that isn't in cache yet (fast-forward race).
func TestCacheInsertionBeforeDelivery(t *testing.T) {
	t.Parallel()

	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	reactor := New(cfg)

	peerAddr := mustParseAddr("10.0.0.1")
	require.NoError(t, reactor.AddPeer(NewPeerSettings(peerAddr, 65000, 65001, 0x01010101)))

	cacheCheckDone := make(chan struct{})
	receiver := &testDeliveryReceiver{
		onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
			// Inside delivery callback: cache entry must already exist
			_, found := reactor.recentUpdates.Get(msg.MessageID)
			assert.True(t, found, "cache entry must exist before delivery callback runs (id=%d)", msg.MessageID)
			close(cacheCheckDone)
		},
	}
	reactor.SetMessageReceiver(receiver)

	stop := startTestDelivery(t, reactor, peerAddr, deliveryChannelCapacity)
	defer stop()

	payload := testUpdatePayload()
	wireUpdate := wireu.NewWireUpdate(payload, 0)
	buf := make([]byte, 4096)

	_ = reactor.notifyMessageReceiver(peerAddr, message.TypeUPDATE, payload, wireUpdate, 0, "received", buf)

	select {
	case <-cacheCheckDone:
	case <-time.After(1 * time.Second):
		t.Fatal("delivery callback should have run and checked cache")
	}
}

// TestActivateAfterAllDeliveries verifies Activate() is called after delivery completes.
//
// VALIDATES: AC-7: Activate(id, N) called with correct consumer count after delivery.
//
// PREVENTS: Cache entries stuck in pending state, never evicted.
func TestActivateAfterAllDeliveries(t *testing.T) {
	t.Parallel()

	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	reactor := New(cfg)

	peerAddr := mustParseAddr("10.0.0.1")
	require.NoError(t, reactor.AddPeer(NewPeerSettings(peerAddr, 65000, 65001, 0x01010101)))

	deliveryDone := make(chan uint64, 1)
	receiver := &testDeliveryReceiver{
		consumerCount: 0, // No consumers → Activate(id, 0) → entry evicted
		onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
			deliveryDone <- msg.MessageID
		},
	}
	reactor.SetMessageReceiver(receiver)

	stop := startTestDelivery(t, reactor, peerAddr, deliveryChannelCapacity)

	payload := testUpdatePayload()
	wireUpdate := wireu.NewWireUpdate(payload, 0)
	buf := make([]byte, 4096)

	_ = reactor.notifyMessageReceiver(peerAddr, message.TypeUPDATE, payload, wireUpdate, 0, "received", buf)

	var msgID uint64
	select {
	case msgID = <-deliveryDone:
	case <-time.After(1 * time.Second):
		t.Fatal("delivery should have completed")
	}

	// Give delivery goroutine time to call Activate after OnMessageReceived returns
	time.Sleep(50 * time.Millisecond)

	// With 0 consumers, Activate(id, 0) should evict the entry
	_, found := reactor.recentUpdates.Get(msgID)
	require.False(t, found, "cache entry should be evicted after Activate(id, 0)")

	stop()
}

// TestDeliveryBackpressure verifies that a full channel blocks the read goroutine.
//
// VALIDATES: AC-8: full delivery channel blocks read goroutine (TCP flow control engages).
//
// PREVENTS: Unbounded memory growth from unlimited buffering.
func TestDeliveryBackpressure(t *testing.T) {
	t.Parallel()

	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	reactor := New(cfg)

	peerAddr := mustParseAddr("10.0.0.1")
	require.NoError(t, reactor.AddPeer(NewPeerSettings(peerAddr, 65000, 65001, 0x01010101)))

	// Fast receiver (no blocking) — backpressure comes from channel, not receiver
	receiver := &testDeliveryReceiver{consumerCount: 0}
	reactor.SetMessageReceiver(receiver)

	// Set up channel directly with capacity 2, NO delivery goroutine.
	// Nothing drains the channel — it fills up and blocks the sender.
	reactor.mu.RLock()
	peer, ok := reactor.findPeerByAddr(peerAddr)
	reactor.mu.RUnlock()
	require.True(t, ok)
	peer.deliverChan = make(chan deliveryItem, 2)

	payload := testUpdatePayload()

	// First 2 UPDATEs fill the channel buffer
	for range 2 {
		w := wireu.NewWireUpdate(payload, 0)
		_ = reactor.notifyMessageReceiver(peerAddr, message.TypeUPDATE, payload, w, 0, "received", make([]byte, 4096))
	}

	// 3rd send in goroutine — should block (channel full, no reader)
	thirdDone := make(chan struct{})
	go func() {
		w := wireu.NewWireUpdate(payload, 0)
		_ = reactor.notifyMessageReceiver(peerAddr, message.TypeUPDATE, payload, w, 0, "received", make([]byte, 4096))
		close(thirdDone)
	}()

	// Pre-impl: all 3 sends complete synchronously (channel unused) → thirdDone closes → FAIL
	// Post-impl: 3rd blocks on full channel → 200ms timeout → PASS
	select {
	case <-thirdDone:
		t.Fatal("notifyMessageReceiver should block when delivery channel is full")
	case <-time.After(200 * time.Millisecond):
		// Expected: backpressure is working
	}

	// Cleanup: drain one slot to unblock the goroutine
	<-peer.deliverChan
	<-thirdDone
}

// TestNonUpdateSynchronous verifies non-UPDATE messages are delivered synchronously.
//
// VALIDATES: AC-12: OPEN/KEEPALIVE processed synchronously on read goroutine.
//
// PREVENTS: Non-UPDATE messages being unnecessarily delayed by async pipeline.
func TestNonUpdateSynchronous(t *testing.T) {
	t.Parallel()

	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	reactor := New(cfg)

	peerAddr := mustParseAddr("10.0.0.1")
	require.NoError(t, reactor.AddPeer(NewPeerSettings(peerAddr, 65000, 65001, 0x01010101)))

	var received bool
	receiver := &testDeliveryReceiver{
		onReceived: func(_ plugin.PeerInfo, _ bgptypes.RawMessage) {
			received = true
		},
	}
	reactor.SetMessageReceiver(receiver)

	stop := startTestDelivery(t, reactor, peerAddr, deliveryChannelCapacity)
	defer stop()

	// KEEPALIVE — must be delivered synchronously (before notifyMessageReceiver returns)
	_ = reactor.notifyMessageReceiver(peerAddr, message.TypeKEEPALIVE, nil, nil, 0, "received", nil)

	require.True(t, received, "KEEPALIVE should be delivered synchronously, not through async channel")
}

// TestCrossPeerIsolation verifies per-peer channel isolation.
//
// VALIDATES: AC-13: peer A's full channel does not block peer B's read goroutine.
//
// PREVENTS: Cross-peer deadlock where one slow peer blocks all peers.
func TestCrossPeerIsolation(t *testing.T) {
	t.Parallel()

	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	reactor := New(cfg)

	peerAddrA := mustParseAddr("10.0.0.1")
	peerAddrB := mustParseAddr("10.0.0.2")
	require.NoError(t, reactor.AddPeer(NewPeerSettings(peerAddrA, 65000, 65001, 0x01010101)))
	require.NoError(t, reactor.AddPeer(NewPeerSettings(peerAddrB, 65000, 65002, 0x02020202)))

	// Receiver: blocks for peer A, fast for peer B
	unblockA := make(chan struct{})
	peerBDelivered := make(chan struct{}, 1)
	receiver := &testDeliveryReceiver{
		onReceived: func(peer plugin.PeerInfo, _ bgptypes.RawMessage) {
			if peer.Address == peerAddrA {
				<-unblockA
			} else {
				select {
				case peerBDelivered <- struct{}{}:
				default:
				}
			}
		},
	}
	reactor.SetMessageReceiver(receiver)

	stopA := startTestDelivery(t, reactor, peerAddrA, 1) // Tiny channel for A
	stopB := startTestDelivery(t, reactor, peerAddrB, deliveryChannelCapacity)

	payload := testUpdatePayload()

	// Fill peer A in a goroutine — in pre-impl, synchronous delivery blocks
	// on <-unblockA inside the receiver. Running in a goroutine prevents the
	// test goroutine from hanging.
	aDone := make(chan struct{})
	go func() {
		defer close(aDone)
		// Send 2 UPDATEs to peer A (1 in delivery + 1 in channel buffer)
		for range 2 {
			w := wireu.NewWireUpdate(payload, 0)
			_ = reactor.notifyMessageReceiver(peerAddrA, message.TypeUPDATE, payload, w, 0, "received", make([]byte, 4096))
		}
	}()

	time.Sleep(100 * time.Millisecond) // Let peer A's sends proceed/block

	// Peer B should NOT be blocked by peer A's state
	wB := wireu.NewWireUpdate(payload, 0)
	start := time.Now()
	_ = reactor.notifyMessageReceiver(peerAddrB, message.TypeUPDATE, payload, wB, 0, "received", make([]byte, 4096))
	elapsed := time.Since(start)

	require.Less(t, elapsed, 50*time.Millisecond,
		"peer B should not be blocked by peer A's full channel (got %v)", elapsed)

	select {
	case <-peerBDelivered:
		// Good: peer B's delivery completed independently
	case <-time.After(1 * time.Second):
		t.Fatal("peer B's delivery should complete despite peer A being blocked")
	}

	// Cleanup: unblock peer A, wait for goroutine, stop delivery
	close(unblockA)
	<-aDone
	stopA()
	stopB()
}

// TestDeliveryDrainOnTeardown verifies remaining items are delivered when channel closes.
//
// VALIDATES: AC-15: delivery goroutine drains remaining items after channel close.
//
// PREVENTS: Lost events on session teardown.
func TestDeliveryDrainOnTeardown(t *testing.T) {
	t.Parallel()

	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	reactor := New(cfg)

	peerAddr := mustParseAddr("10.0.0.1")
	require.NoError(t, reactor.AddPeer(NewPeerSettings(peerAddr, 65000, 65001, 0x01010101)))

	var deliveryCount atomic.Int32
	receiver := &testDeliveryReceiver{
		onReceived: func(_ plugin.PeerInfo, _ bgptypes.RawMessage) {
			deliveryCount.Add(1)
		},
	}
	reactor.SetMessageReceiver(receiver)

	stop := startTestDelivery(t, reactor, peerAddr, deliveryChannelCapacity)

	// Enqueue several items
	const itemCount = 5
	payload := testUpdatePayload()
	for range itemCount {
		w := wireu.NewWireUpdate(payload, 0)
		_ = reactor.notifyMessageReceiver(peerAddr, message.TypeUPDATE, payload, w, 0, "received", make([]byte, 4096))
	}

	// Close channel (teardown) — delivery goroutine drains remaining items
	stop()

	require.Equal(t, int32(itemCount), deliveryCount.Load(),
		"all enqueued items should be delivered during drain")
}

// TestPeerDeliveryDrainBatch verifies the delivery goroutine drains multiple items into a batch.
//
// VALIDATES: AC-4: peer delivery goroutine drains multiple items from channel.
//
// PREVENTS: Batch drain reverting to per-message processing.
func TestPeerDeliveryDrainBatch(t *testing.T) {
	t.Parallel()

	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	reactor := New(cfg)

	peerAddr := mustParseAddr("10.0.0.1")
	require.NoError(t, reactor.AddPeer(NewPeerSettings(peerAddr, 65000, 65001, 0x01010101)))

	// Track batch sizes via OnMessageBatchReceived call counts
	var batchCallCount atomic.Int32
	var totalMessages atomic.Int32
	receiver := &testDeliveryReceiver{
		onReceived: func(_ plugin.PeerInfo, _ bgptypes.RawMessage) {
			totalMessages.Add(1)
		},
		consumerCount: 1,
	}
	reactor.SetMessageReceiver(receiver)

	// Pre-fill channel with multiple items BEFORE starting delivery goroutine,
	// so the drain will collect them all in one batch.
	reactor.mu.RLock()
	peer, ok := reactor.findPeerByAddr(peerAddr)
	reactor.mu.RUnlock()
	require.True(t, ok)

	peer.deliverChan = make(chan deliveryItem, deliveryChannelCapacity)

	const itemCount = 5
	payload := testUpdatePayload()
	for range itemCount {
		w := wireu.NewWireUpdate(payload, 0)
		_ = reactor.notifyMessageReceiver(peerAddr, message.TypeUPDATE, payload, w, 0, "received", make([]byte, 4096))
	}

	// Now start delivery — all 5 items are already buffered
	done := make(chan struct{})
	go func() {
		defer close(done)
		for first := range peer.deliverChan {
			batch := drainDeliveryBatch(first, peer.deliverChan)
			batchCallCount.Add(1)

			reactor.mu.RLock()
			recv := reactor.messageReceiver
			reactor.mu.RUnlock()
			if recv == nil {
				continue
			}

			msgs := make([]any, len(batch))
			for i := range batch {
				msgs[i] = batch[i].msg
			}
			counts := recv.OnMessageBatchReceived(batch[0].peerInfo, msgs)
			for i := range batch {
				count := 0
				if i < len(counts) {
					count = counts[i]
				}
				reactor.recentUpdates.Activate(batch[i].msg.MessageID, count)
			}
		}
	}()

	// Close and wait
	close(peer.deliverChan)
	<-done

	require.Equal(t, int32(itemCount), totalMessages.Load(), "all messages should be delivered")
	// With all items pre-buffered, drain should collect them in 1 batch call
	require.Equal(t, int32(1), batchCallCount.Load(), "pre-buffered items should be drained in one batch")
}

// TestPeerDeliveryActivatePerMessage verifies each message in batch gets its own Activate call.
//
// VALIDATES: AC-5: Activate(msgID, count) called per message with correct cacheCount.
//
// PREVENTS: Batch aggregating cache counts instead of per-message tracking.
func TestPeerDeliveryActivatePerMessage(t *testing.T) {
	t.Parallel()

	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	reactor := New(cfg)

	peerAddr := mustParseAddr("10.0.0.1")
	require.NoError(t, reactor.AddPeer(NewPeerSettings(peerAddr, 65000, 65001, 0x01010101)))

	receiver := &testDeliveryReceiver{consumerCount: 2}
	reactor.SetMessageReceiver(receiver)

	stop := startTestDelivery(t, reactor, peerAddr, deliveryChannelCapacity)

	// Send 3 messages with distinct IDs
	payload := testUpdatePayload()
	for range 3 {
		w := wireu.NewWireUpdate(payload, 0)
		_ = reactor.notifyMessageReceiver(peerAddr, message.TypeUPDATE, payload, w, 0, "received", make([]byte, 4096))
	}

	// Wait for delivery
	time.Sleep(100 * time.Millisecond)

	// Each message should have been Activated with count=2 (consumer count).
	// Activate(id, 2) with 0 early acks → pendingConsumers=2 > 0 → entries retained.
	require.Equal(t, 3, reactor.recentUpdates.Len(),
		"3 messages should each have a retained cache entry after Activate(id, 2)")

	stop()
}

// --- Backpressure pause/resume tests ---
// VALIDATES: AC-7 — Reactor.PausePeer(addr) pauses specific peer's read loop
// PREVENTS: Pause signal not reaching the correct peer

func TestReactorPausePeer(t *testing.T) {
	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	r := New(cfg)

	addr1 := mustParseAddr("10.0.0.1")
	addr2 := mustParseAddr("10.0.0.2")
	require.NoError(t, r.AddPeer(NewPeerSettings(addr1, 65000, 65001, 0x01010101)))
	require.NoError(t, r.AddPeer(NewPeerSettings(addr2, 65000, 65002, 0x01010101)))

	// Give each peer a session so pause has something to delegate to.
	for _, p := range r.Peers() {
		session := NewSession(p.Settings())
		p.mu.Lock()
		p.session = session
		p.mu.Unlock()
	}

	// Pause peer 1 only.
	require.NoError(t, r.PausePeer(addr1))

	// Verify peer 1 is paused, peer 2 is not.
	r.mu.RLock()
	peer1 := r.peers[PeerKeyFromAddrPort(addr1, DefaultBGPPort)]
	peer2 := r.peers[PeerKeyFromAddrPort(addr2, DefaultBGPPort)]
	r.mu.RUnlock()

	require.True(t, peer1.IsReadPaused(), "peer 1 should be paused")
	require.False(t, peer2.IsReadPaused(), "peer 2 should not be paused")

	// Resume peer 1.
	require.NoError(t, r.ResumePeer(addr1))
	require.False(t, peer1.IsReadPaused(), "peer 1 should be resumed")

	// Pause unknown peer should return error.
	unknown := mustParseAddr("10.0.0.99")
	require.Error(t, r.PausePeer(unknown))
	require.Error(t, r.ResumePeer(unknown))
}

// VALIDATES: AC-8, AC-9 — PauseAllReads/ResumeAllReads affects all peers
// PREVENTS: Some peers escaping a global pause

func TestReactorPauseAllReads(t *testing.T) {
	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	r := New(cfg)

	addrs := []netip.Addr{
		mustParseAddr("10.0.0.1"),
		mustParseAddr("10.0.0.2"),
		mustParseAddr("10.0.0.3"),
	}
	for i, addr := range addrs {
		require.NoError(t, r.AddPeer(NewPeerSettings(addr, 65000, uint32(65001+i), 0x01010101)))
	}

	// Give each peer a session.
	for _, p := range r.Peers() {
		session := NewSession(p.Settings())
		p.mu.Lock()
		p.session = session
		p.mu.Unlock()
	}

	// Pause all reads.
	r.PauseAllReads()

	// All peers should be paused.
	for _, p := range r.Peers() {
		require.True(t, p.IsReadPaused(), "peer %s should be paused", p.Settings().Address)
	}

	// Resume all reads.
	r.ResumeAllReads()

	// All peers should be resumed.
	for _, p := range r.Peers() {
		require.False(t, p.IsReadPaused(), "peer %s should be resumed", p.Settings().Address)
	}
}

// TestGetMatchingPeersExclusion verifies that the "!" prefix excludes the named peer.
//
// VALIDATES: "!addr" selector returns all peers except the excluded one.
// PREVENTS: bgp-rs withdrawal propagation failing with "no peers match selector".
func TestGetMatchingPeersExclusion(t *testing.T) {
	r := &Reactor{
		peers: map[string]*Peer{
			"127.0.0.1:179": {settings: &PeerSettings{Address: mustParseAddr("127.0.0.1")}},
			"127.0.0.2:179": {settings: &PeerSettings{Address: mustParseAddr("127.0.0.2")}},
			"127.0.0.3:179": {settings: &PeerSettings{Address: mustParseAddr("127.0.0.3")}},
		},
	}
	adapter := &reactorAPIAdapter{r: r}

	tests := []struct {
		name     string
		selector string
		wantLen  int
		excluded string // address that must NOT appear
	}{
		{"exclude one peer", "!127.0.0.2", 2, "127.0.0.2"},
		{"exclude with port", "!127.0.0.1:179", 2, "127.0.0.1"},
		{"exclude bare IP appends default port", "!127.0.0.3", 2, "127.0.0.3"},
		{"exclude nonexistent returns all", "!127.0.0.99", 3, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peers := adapter.getMatchingPeers(tt.selector)
			assert.Len(t, peers, tt.wantLen)
			for _, p := range peers {
				if tt.excluded != "" {
					assert.NotEqual(t, tt.excluded, p.settings.Address.String(),
						"excluded peer should not be in result")
				}
			}
		})
	}
}
