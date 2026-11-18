// Design: docs/architecture/l2tp/subscriber-session-model.md -- original tunnel index ownership

package l2tp

import (
	"io"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDiscardTunnelUsesOriginalPeerIndexKey follows an admitted tunnel through
// a source update, discard and a fresh SCCRQ. Another peer's same-numbered
// tunnel must keep its index entry throughout. The unchanged source is the
// control for ordinary discard; the two changed sources exercise the actual
// dispatch assignment rather than editing the tunnel's peer address in the test.
func TestDiscardTunnelUsesOriginalPeerIndexKey(t *testing.T) {
	originalPeer := netip.MustParseAddrPort("192.0.2.10:1701")
	for _, test := range []struct {
		name      string
		from      netip.AddrPort
		otherPeer netip.AddrPort
	}{
		{
			name:      "unchanged source",
			from:      originalPeer,
			otherPeer: netip.MustParseAddrPort("192.0.2.20:1701"),
		},
		{
			name:      "changed source port",
			from:      netip.MustParseAddrPort("192.0.2.10:2701"),
			otherPeer: netip.MustParseAddrPort("192.0.2.10:2701"),
		},
		{
			name:      "changed source address",
			from:      netip.MustParseAddrPort("192.0.2.11:1701"),
			otherPeer: netip.MustParseAddrPort("192.0.2.11:1701"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			// Construction alone opens no socket or worker. A ZLB produces no
			// outbound send, so dispatch still uses the real reactor code here.
			listener := newUDPListener(netip.MustParseAddrPort("127.0.0.1:1701"), logger)
			reactor := newL2TPReactor(listener, logger, reactorParams{})
			const remoteTID uint16 = 42
			original := admitPeerIndexTunnel(t, reactor, originalPeer)
			other := admitPeerIndexTunnel(t, reactor, test.otherPeer)
			require.NotSame(t, original, other)

			reactor.handle(rxPacket{
				from:    test.from,
				bytes:   buildZLBAck(original.localTID, 0, 0),
				release: func() {}, // The test owns these bytes; no receive slot is borrowed.
			})
			select {
			case update := <-reactor.updateCh:
				require.Equal(t, original.localTID, update.tunnelID,
					"the source-changing packet must finish normal dispatch")
			default:
				t.Fatal("the source-changing packet did not finish normal dispatch")
			}

			func() {
				reactor.tunnelsMu.Lock()
				defer reactor.tunnelsMu.Unlock()
				require.Equal(t, test.from, original.peerAddr,
					"normal dispatch must have applied the packet's source")
				teardowns, downs := reactor.discardTunnelLocked(original, "peer-index regression")
				require.Empty(t, teardowns)
				require.Empty(t, downs)
				require.Equal(t, L2TPTunnelClosed, original.state)
				require.NotContains(t, reactor.tunnelsByLocalID, original.localTID)
				require.NotContains(t, reactor.tunnelsByPeer,
					peerKey{addr: originalPeer, tid: remoteTID},
					"discard must remove the key used when this original tunnel was indexed")
				require.Same(t, other, reactor.tunnelsByLocalID[other.localTID])
				require.Same(t, other,
					reactor.tunnelsByPeer[peerKey{addr: test.otherPeer, tid: remoteTID}],
					"discard must preserve the other tunnel's same-numbered peer entry")
			}()

			fresh := admitPeerIndexTunnel(t, reactor, originalPeer)
			require.NotSame(t, original, fresh,
				"a fresh SCCRQ must allocate a new tunnel instead of finding the closed original")
			require.Equal(t, L2TPTunnelIdle, fresh.state)
			require.Same(t, other, admitPeerIndexTunnel(t, reactor, test.otherPeer),
				"the unaffected peer's SCCRQ must still find its original tunnel")
		})
	}
}

// TestDiscardTunnelPreservesPeerIndexReplacement repeats an original discard
// after a fresh SCCRQ has claimed its old peer key. The fresh row is the only
// index owner, so repeating old cleanup must leave its admission reachable.
func TestDiscardTunnelPreservesPeerIndexReplacement(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	listener := newUDPListener(netip.MustParseAddrPort("127.0.0.1:1701"), logger)
	reactor := newL2TPReactor(listener, logger, reactorParams{})
	peer := netip.MustParseAddrPort("192.0.2.10:1701")
	const remoteTID uint16 = 42
	original := admitPeerIndexTunnel(t, reactor, peer)

	func() {
		reactor.tunnelsMu.Lock()
		defer reactor.tunnelsMu.Unlock()
		teardowns, downs := reactor.discardTunnelLocked(original, "first discard")
		require.Empty(t, teardowns)
		require.Empty(t, downs)
		require.NotContains(t, reactor.tunnelsByLocalID, original.localTID)
		require.NotContains(t, reactor.tunnelsByPeer, peerKey{addr: peer, tid: remoteTID})
	}()
	fresh := admitPeerIndexTunnel(t, reactor, peer)
	require.NotSame(t, original, fresh)

	func() {
		reactor.tunnelsMu.Lock()
		defer reactor.tunnelsMu.Unlock()
		teardowns, downs := reactor.discardTunnelLocked(original, "repeated original discard")
		require.Empty(t, teardowns)
		require.Empty(t, downs)
		require.Same(t, fresh, reactor.tunnelsByLocalID[fresh.localTID])
		require.Same(t, fresh, reactor.tunnelsByPeer[peerKey{addr: peer, tid: remoteTID}],
			"the repeated original discard must preserve the replacement's peer entry")
	}()
	require.Same(t, fresh, admitPeerIndexTunnel(t, reactor, peer),
		"the replacement must remain the current SCCRQ target")
}

// TestDiscardTunnelPreservesLocalIndexReplacement repeats old cleanup after
// actual SCCRQ admission has reused its numeric local TID. Both fresh entries
// must survive, so the primary-owner check is tested independently of the
// secondary key's ownership guard.
func TestDiscardTunnelPreservesLocalIndexReplacement(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	listener := newUDPListener(netip.MustParseAddrPort("127.0.0.1:1701"), logger)
	reactor := newL2TPReactor(listener, logger, reactorParams{})
	peer := netip.MustParseAddrPort("192.0.2.10:1701")
	const remoteTID uint16 = 42
	original := admitPeerIndexTunnel(t, reactor, peer)

	func() {
		reactor.tunnelsMu.Lock()
		defer reactor.tunnelsMu.Unlock()
		teardowns, downs := reactor.discardTunnelLocked(original, "first discard")
		require.Empty(t, teardowns)
		require.Empty(t, downs)
		require.NotContains(t, reactor.tunnelsByLocalID, original.localTID)
		require.NotContains(t, reactor.tunnelsByPeer, peerKey{addr: peer, tid: remoteTID})
		// Use the real allocator's next draw to exercise a freed numeric slot.
		reactor.nextLocalTID = original.localTID - 1
	}()
	fresh := admitPeerIndexTunnel(t, reactor, peer)
	require.Equal(t, original.localTID, fresh.localTID)
	require.NotSame(t, original, fresh)

	func() {
		reactor.tunnelsMu.Lock()
		defer reactor.tunnelsMu.Unlock()
		teardowns, downs := reactor.discardTunnelLocked(original, "repeated original discard")
		require.Empty(t, teardowns)
		require.Empty(t, downs)
		require.Same(t, fresh, reactor.tunnelsByLocalID[fresh.localTID],
			"the repeated original discard must preserve the replacement's local entry")
		require.Same(t, fresh, reactor.tunnelsByPeer[peerKey{addr: peer, tid: remoteTID}],
			"the repeated original discard must preserve the replacement's peer entry")
	}()
	require.Same(t, fresh, admitPeerIndexTunnel(t, reactor, peer),
		"the replacement must remain the current SCCRQ target")
}

// admitPeerIndexTunnel validates an actual SCCRQ body and uses the production
// registration path. Its reactor remains unstarted and owns no native resource.
func admitPeerIndexTunnel(t *testing.T, reactor *l2tpReactor, from netip.AddrPort) *L2TPTunnel {
	t.Helper()
	const remoteTID uint16 = 42
	packet := buildSCCRQ(t, remoteTID, "peer-index-test")
	header, err := ParseMessageHeader(packet)
	require.NoError(t, err)
	request, err := parseSCCRQ(packet[header.PayloadOff:int(header.Length)])
	require.NoError(t, err)
	reactor.tunnelsMu.Lock()
	defer reactor.tunnelsMu.Unlock()
	tunnel, teardowns, downs := reactor.locateTunnelLocked(from, header, &request)
	require.NotNil(t, tunnel)
	require.Empty(t, teardowns)
	require.Empty(t, downs)
	require.Same(t, tunnel, reactor.tunnelsByLocalID[tunnel.localTID])
	require.Same(t, tunnel, reactor.tunnelsByPeer[peerKey{addr: from, tid: remoteTID}])
	return tunnel
}
