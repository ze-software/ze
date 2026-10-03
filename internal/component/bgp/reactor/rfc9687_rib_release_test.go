// Design: docs/architecture/plugin/rib-storage-design.md -- peer-down releases received routes.
package reactor

import (
	"bytes"
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgprib "github.com/ze-software/ze/internal/component/bgp/plugins/rib"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/test/sim"
)

// rfc9687RIBSnapshot observes the real RIB plugin through its public snapshot
// bridge. Neither this visitor nor the test can insert or delete RIB entries.
type rfc9687RIBSnapshot struct {
	peer bool
	routes int
}

func (v *rfc9687RIBSnapshot) OnPeer(address string, _ uint32, _ [4]byte, _ bool) uint16 {
	if address == "192.0.2.1" {
		v.peer = true
		return 1
	}
	return 0
}

func (v *rfc9687RIBSnapshot) OnRoute(peer, afi, safi uint16, length uint8, prefix, _ []byte) {
	if peer == 1 && afi == 1 && safi == 1 && length == 24 &&
		(bytes.Equal(prefix, []byte{203, 0, 113}) || bytes.Equal(prefix, []byte{198, 51, 100})) {
		v.routes++
	}
}

// TestRFC9687Event29ReleasesTheLivePeersRIB drives the actual Peer, reactor
// state dispatcher, registered bgp-rib plugin and received-route storage. The
// source sends both routes on TCP; no test helper manufactures a DOWN event.
// RFC requirement: RFC9687-4.3-3 positive -- after two real received routes are observed in the running RIB plugin, SendHoldTimer expiry releases the peer's session and purges both routes and their peer storage, in addition to session-local timer resources (§4.3).
func TestRFC9687Event29ReleasesTheLivePeersRIB(t *testing.T) {
	clock := sim.NewFakeClock(time.Now())
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	r.SetClock(clock)
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
	settings.SendHoldTime = rfc9687SendHold
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}
	require.NoError(t, EnsureProcessBinding(settings, "bgp-rib", "update state refresh", "update"))
	require.NoError(t, r.AddPeer(settings))
	peer := r.peers[settings.PeerKey()]
	peer.setReconnectDelay(time.Minute, time.Minute)
	srv := newBorrowedPluginServer(t, r, "bgp-rib")
	r.SetPluginServer(srv)
	require.NoError(t, r.StartWithContext(context.Background()))
	t.Cleanup(func() { stopAndWait(t, r) })
	require.NoError(t, r.StartPeers())
	require.Eventually(t, func() bool {
		return peer.currentSession() != nil && peer.SessionState() == fsm.StateActive
	}, 5*time.Second, time.Millisecond)
	session := peer.currentSession()
	server, client := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	wire, _ := startDrain(t, client)
	require.NoError(t, peer.acceptConnection(server))
	_, err := client.Write(rfc9687PeerOpen(90))
	require.NoError(t, err)
	require.Eventually(t, func() bool { return session.State() == fsm.StateOpenConfirm }, 5*time.Second, time.Millisecond)
	_, err = client.Write(message.PackTo(message.NewKeepalive(), nil))
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return peer.State() == PeerStateEstablished && !peer.pendingSync()
	}, 5*time.Second, time.Millisecond)
	update := &message.Update{
		PathAttributes: []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xea, 0x40, 3, 4, 192, 0, 2, 1},
		NLRI: []byte{24, 203, 0, 113, 24, 198, 51, 100},
	}
	_, err = client.Write(message.PackTo(update, nil))
	require.NoError(t, err)
	snapshot := func() rfc9687RIBSnapshot {
		var v rfc9687RIBSnapshot
		bgprib.RIBDumpBridge.DumpRIB(registry.RIBDumpVisitor{OnPeer: v.OnPeer, OnRoute: v.OnRoute})
		return v
	}
	require.Eventually(t, func() bool { return snapshot().routes == 2 }, 5*time.Second, time.Millisecond,
		"both wire routes must exist before the timer expires")
	require.NotZero(t, session.sendHoldDeadline.Load())
	clock.Add(rfc9687SendHold)
	require.Eventually(t, func() bool {
		v := snapshot()
		return !v.peer && v.routes == 0 && peer.currentSession() != session && session.State() == fsm.StateIdle
	}, 5*time.Second, time.Millisecond, "Event 29 must release the actual peer and its RIB storage")
	require.Nil(t, session.Conn())
	require.True(t, bytes.Contains(collectWire(wire), message.PackTo(&message.Notification{ErrorCode: message.NotifySendHoldTimerExpired}, nil)),
		"the connection ended with the Event 29 notification, not an unrelated failure")
}
