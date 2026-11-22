// Design: docs/architecture/core-design.md -- initial route sending on BGP session establishment
// RFC: rfc/short/rfc4724.md -- Section 3, peer with Restart State bit set must not wait for End-of-RIB
// Related: peer_initial_sync.go -- Peer.sendInitialRoutes

package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC4724PeerRestartStateBitDoesNotDelayInitialRoutes verifies that when a
// peer sets the Graceful Restart "Restart State" (R) bit in its OPEN and has
// not yet sent an End-of-RIB marker, sendInitialRoutes advertises Ze's initial
// routing table and End-of-RIB immediately without waiting for the peer's
// End-of-RIB.
//
// RFC 4724 Section 3: "When set (value 1), this bit indicates that the BGP
// speaker has restarted, and its peer MUST NOT wait for the End-of-RIB marker
// from the speaker before advertising routing information to the speaker."
//
// VALIDATES: with the peer's GracefulRestart capability carrying RestartState
// true and zero End-of-RIB markers received from the peer, sendInitialRoutes
// writes the configured route UPDATE and End-of-RIB to the socket immediately.
// PREVENTS: adding a wait for the peer's End-of-RIB before initial sync, which
// deadlocks when both BGP speakers restart simultaneously.
//
// RFC requirement: RFC4724-3-5 positive -- when the peer advertised Graceful
// Restart with RestartState=true and has sent no End-of-RIB (EORReceived == 0),
// Peer.sendInitialRoutes immediately advertises the 0.0.0.0/0 UPDATE followed
// by End-of-RIB without blocking on the peer's End-of-RIB.
// RFC requirement: RFC4724-3-5 negative -- when negotiated capabilities are
// absent (peer.negotiated == nil), Peer.sendInitialRoutes aborts and writes no
// UPDATE or End-of-RIB, proving the immediate advertisement comes from the
// active sendInitialRoutes path.
func TestRFC4724PeerRestartStateBitDoesNotDelayInitialRoutes(t *testing.T) {
	eor := eorWire(family.IPv4Unicast)

	t.Run("restarting peer with R-bit set receives routes without sending EoR", func(t *testing.T) {
		peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
		peer.settings.LocalAddress = netip.MustParseAddr("10.0.0.1")
		peer.settings.DefaultOriginate = map[string]bool{"ipv4/unicast": true}
		localCaps := []capability.Capability{
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
			&capability.ASN4{ASN: 65000},
			&capability.GracefulRestart{RestartTime: 120},
		}
		remoteCaps := []capability.Capability{
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
			&capability.ASN4{ASN: 65001},
			&capability.GracefulRestart{
				RestartState: true,
				RestartTime:  120,
				Families: []capability.GracefulRestartFamily{{
					AFI:  capability.AFIIPv4,
					SAFI: capability.SAFIUnicast,
				}},
			},
		}
		peer.session.negotiated = capability.Negotiate(localCaps, remoteCaps, capability.PeerIdentity{
			LocalASN: 65000,
			PeerASN:  65001,
		})
		require.NotNil(t, peer.session.negotiated.GracefulRestart)
		require.True(t, peer.session.negotiated.GracefulRestart.RestartState)

		// Peer has sent no End-of-RIB marker.
		require.Zero(t, peer.Stats().EORReceived)

		peer.runInitialSyncNow()

		msgs := splitWireMessages(t, conn.written())
		require.Len(t, msgs, 2, "route UPDATE followed by End-of-RIB must leave without waiting for peer EoR")
		require.True(t, bytes.HasSuffix(msgs[0], []byte{0x00}), "first message advertises 0.0.0.0/0")
		require.Equal(t, eor, msgs[1], "second message is End-of-RIB")
		require.Equal(t, uint32(1), peer.Stats().EORSent)
	})

	t.Run("no negotiated capabilities writes nothing", func(t *testing.T) {
		peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
		peer.settings.LocalAddress = netip.MustParseAddr("10.0.0.1")
		peer.settings.DefaultOriginate = map[string]bool{"ipv4/unicast": true}
		peer.negotiated.Store(nil)

		peer.runInitialSyncNow()

		require.Empty(t, conn.written(), "aborted sync must not write routes or End-of-RIB")
		require.Zero(t, peer.Stats().EORSent)
	})
}
