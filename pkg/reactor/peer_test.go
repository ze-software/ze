package reactor

import (
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/stretchr/testify/require"
)

func mustParseAddr(s string) netip.Addr {
	return netip.MustParseAddr(s)
}

// TestPeerNew verifies Peer creation with correct initial state.
//
// VALIDATES: Peer starts in stopped state with nil session.
//
// PREVENTS: Peer starting automatically or with invalid state.
func TestPeerNew(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)

	peer := NewPeer(settings)

	require.NotNil(t, peer, "NewPeer must return non-nil")
	require.Equal(t, PeerStateStopped, peer.State(), "initial state must be Stopped")
	require.Equal(t, settings, peer.Settings(), "Settings() must return peer settings")
}

// TestPeerStartStop verifies basic start/stop lifecycle.
//
// VALIDATES: Peer can be started and stopped cleanly.
//
// PREVENTS: Resource leaks or goroutine leaks on stop.
func TestPeerStartStop(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.Port = 0 // Invalid port to prevent actual connection

	peer := NewPeer(settings)

	// Start peer
	peer.Start()

	// Give goroutine time to start
	time.Sleep(10 * time.Millisecond)

	require.NotEqual(t, PeerStateStopped, peer.State(), "state should change after Start")

	// Stop peer
	peer.Stop()

	// Wait for stop
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = peer.Wait(ctx)

	require.Equal(t, PeerStateStopped, peer.State(), "state must be Stopped after Stop")
}

// TestPeerReconnect verifies reconnection logic with backoff.
//
// VALIDATES: Peer attempts reconnection after connection failure.
//
// PREVENTS: Peer giving up after first failure or flooding with
// connection attempts without backoff.
func TestPeerReconnect(t *testing.T) {
	// Use a listener that immediately closes connections
	listener, err := net.Listen("tcp", "127.0.0.1:0") //nolint:noctx // Test code
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok, "expected TCPAddr")

	var connectCount atomic.Int32
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connectCount.Add(1)
			_ = conn.Close() // Immediately close to trigger reconnect
		}
	}()

	settings := NewPeerSettings(
		mustParseAddr("127.0.0.1"),
		65000, 65001, 0x01010101,
	)
	settings.Port = uint16(addr.Port) //nolint:gosec // Port fits in uint16

	peer := NewPeer(settings)
	peer.SetReconnectDelay(10*time.Millisecond, 50*time.Millisecond)

	peer.Start()

	// Wait for multiple reconnect attempts
	time.Sleep(100 * time.Millisecond)

	peer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = peer.Wait(ctx)

	count := connectCount.Load()
	require.GreaterOrEqual(t, count, int32(2), "peer should reconnect at least twice, got %d", count)
}

// TestPeerContextCancellation verifies peer stops on context cancellation.
//
// VALIDATES: Peer respects context cancellation for clean shutdown.
//
// PREVENTS: Orphaned goroutines when parent context is cancelled.
func TestPeerContextCancellation(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.Port = 0 // Invalid port

	peer := NewPeer(settings)

	ctx, cancel := context.WithCancel(context.Background())
	peer.StartWithContext(ctx)

	time.Sleep(10 * time.Millisecond)
	require.NotEqual(t, PeerStateStopped, peer.State())

	// Cancel context
	cancel()

	// Should stop within reasonable time
	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	err := peer.Wait(waitCtx)

	require.NoError(t, err, "peer should stop on context cancellation")
	require.Equal(t, PeerStateStopped, peer.State())
}

// TestPeerStateTransitions verifies state changes during connection lifecycle.
//
// VALIDATES: Peer reports correct state (Connecting, Connected, etc).
//
// PREVENTS: Incorrect state reporting to callers.
func TestPeerStateTransitions(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0") //nolint:noctx // Test code
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok, "expected TCPAddr")

	// Accept connections but don't respond (peer stays connecting)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Hold connection open without BGP handshake
			time.Sleep(time.Second)
			_ = conn.Close()
		}
	}()

	settings := NewPeerSettings(
		mustParseAddr("127.0.0.1"),
		65000, 65001, 0x01010101,
	)
	settings.Port = uint16(addr.Port) //nolint:gosec // Port fits in uint16

	peer := NewPeer(settings)
	peer.Start()

	// Should transition to Connecting
	time.Sleep(50 * time.Millisecond)
	state := peer.State()
	require.True(t, state == PeerStateConnecting || state == PeerStateActive,
		"state should be Connecting or Active, got %v", state)

	peer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = peer.Wait(ctx)
}

// TestPeerCallback verifies state change callbacks are invoked.
//
// VALIDATES: Callback is called on state transitions.
//
// PREVENTS: Missing notifications to observers.
func TestPeerCallback(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.Port = 0

	peer := NewPeer(settings)

	var transitions []PeerState
	peer.SetCallback(func(from, to PeerState) {
		transitions = append(transitions, to)
	})

	peer.Start()
	time.Sleep(20 * time.Millisecond)
	peer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = peer.Wait(ctx)

	require.NotEmpty(t, transitions, "callback should be invoked at least once")
}

// TestBuildMPReachNLRIUnicast verifies MP_REACH_NLRI generation for IPv6 unicast.
//
// VALIDATES: IPv6 unicast routes produce correct MP_REACH_NLRI attribute with
// proper AFI=2, SAFI=1, next-hop, and NLRI encoding.
//
// PREVENTS: IPv6 routes being sent without MP_REACH_NLRI (which would be
// silently dropped by peers).
func TestBuildMPReachNLRIUnicast(t *testing.T) {
	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("2001:db8::1/128"),
		NextHop: netip.MustParseAddr("2001:db8::ffff"),
		Origin:  0, // IGP
	}

	attrBytes := buildMPReachNLRIUnicast(route)
	require.NotEmpty(t, attrBytes, "must produce attribute bytes")

	// Parse attribute header
	require.GreaterOrEqual(t, len(attrBytes), 3, "must have at least header")
	flags := attrBytes[0]
	code := attrBytes[1]

	require.Equal(t, byte(attribute.AttrMPReachNLRI), code, "code must be MP_REACH_NLRI (14)")
	require.True(t, flags&0x80 != 0, "must be optional")

	// Determine value offset based on extended length
	var valueOffset int
	if flags&0x10 != 0 {
		valueOffset = 4 // flags + code + 2-byte length
	} else {
		valueOffset = 3 // flags + code + 1-byte length
	}

	value := attrBytes[valueOffset:]
	require.GreaterOrEqual(t, len(value), 5, "value must have AFI/SAFI/NH_Len")

	// Parse AFI/SAFI
	afi := binary.BigEndian.Uint16(value[0:2])
	safi := value[2]
	nhLen := value[3]

	require.Equal(t, uint16(2), afi, "AFI must be IPv6 (2)")
	require.Equal(t, byte(1), safi, "SAFI must be Unicast (1)")
	require.Equal(t, byte(16), nhLen, "next-hop length must be 16 for IPv6")

	// Verify next-hop
	require.GreaterOrEqual(t, len(value), 4+16+1, "must have next-hop + reserved")
	nhBytes := value[4 : 4+16]
	expectedNH := route.NextHop.As16()
	require.Equal(t, expectedNH[:], nhBytes, "next-hop must match")

	// Verify reserved byte
	require.Equal(t, byte(0), value[4+16], "reserved byte must be 0")

	// Verify NLRI is present
	nlriOffset := 4 + 16 + 1
	require.Greater(t, len(value), nlriOffset, "must have NLRI")
}

// TestBuildStaticRouteUpdateIPv6 verifies UPDATE generation for IPv6 unicast.
//
// VALIDATES: IPv6 unicast routes include MP_REACH_NLRI attribute and have
// empty inline NLRI field.
//
// PREVENTS: IPv6 routes being sent with inline NLRI (which violates RFC 4760).
func TestBuildStaticRouteUpdateIPv6(t *testing.T) {
	route := StaticRoute{
		Prefix:          netip.MustParsePrefix("2001:db8::1/128"),
		NextHop:         netip.MustParseAddr("2001:db8::ffff"),
		Origin:          0,
		LocalPreference: 100,
	}

	update := buildStaticRouteUpdate(route, 65000, true, true) // iBGP, asn4

	// IPv6 routes must NOT have inline NLRI
	require.Empty(t, update.NLRI, "IPv6 route must not have inline NLRI")

	// Path attributes must contain MP_REACH_NLRI
	require.NotEmpty(t, update.PathAttributes, "must have path attributes")

	// Look for MP_REACH_NLRI (code 14) in attributes
	found := false
	offset := 0
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		flags := update.PathAttributes[offset]
		code := update.PathAttributes[offset+1]

		var attrLen int
		if flags&0x10 != 0 {
			if offset+4 > len(update.PathAttributes) {
				break
			}
			attrLen = int(binary.BigEndian.Uint16(update.PathAttributes[offset+2 : offset+4]))
			offset += 4
		} else {
			if offset+3 > len(update.PathAttributes) {
				break
			}
			attrLen = int(update.PathAttributes[offset+2])
			offset += 3
		}

		if code == byte(attribute.AttrMPReachNLRI) {
			found = true
			break
		}
		offset += attrLen
	}

	require.True(t, found, "IPv6 UPDATE must contain MP_REACH_NLRI attribute")
}

// TestBuildStaticRouteUpdateWithCommunities verifies communities are included.
//
// VALIDATES: Routes with communities produce COMMUNITIES attribute (code 8).
//
// PREVENTS: Communities being silently dropped from announcements.
func TestBuildStaticRouteUpdateWithCommunities(t *testing.T) {
	route := StaticRoute{
		Prefix:      netip.MustParsePrefix("192.0.2.0/24"),
		NextHop:     netip.MustParseAddr("192.0.2.1"),
		Origin:      0,
		Communities: []uint32{0x78140000, 0x78147814}, // 30740:0, 30740:30740
	}

	update := buildStaticRouteUpdate(route, 65000, false, true) // eBGP, asn4
	require.NotEmpty(t, update.PathAttributes, "must have path attributes")

	// Look for COMMUNITIES (code 8) in attributes
	found := false
	offset := 0
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		flags := update.PathAttributes[offset]
		code := update.PathAttributes[offset+1]

		var attrLen int
		if flags&0x10 != 0 {
			if offset+4 > len(update.PathAttributes) {
				break
			}
			attrLen = int(binary.BigEndian.Uint16(update.PathAttributes[offset+2 : offset+4]))
			offset += 4
		} else {
			if offset+3 > len(update.PathAttributes) {
				break
			}
			attrLen = int(update.PathAttributes[offset+2])
			offset += 3
		}

		if code == byte(attribute.AttrCommunity) {
			found = true
			require.Equal(t, 8, attrLen, "communities length must be 8 (2 x 4 bytes)")
			break
		}
		offset += attrLen
	}

	require.True(t, found, "UPDATE with communities must contain COMMUNITIES attribute")
}
