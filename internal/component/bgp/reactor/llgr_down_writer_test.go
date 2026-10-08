// Design: docs/guide/graceful-restart.md -- retained routes cross the real peer-DOWN boundary.
// Related: llgr_lifecycle_writer_test.go -- final-writer receipts and asynchronous sent projection.
package reactor

import (
	"bytes"
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/gr"
	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestLLGRActualDownKeepsWriterOwner closes the source TCP connection only after
// its route has reached both the real RIB and recipient socket. Actual GR, RS,
// RIB and peer-loop handlers must retain that owner, then advertise LLGR_STALE
// without any intervening withdrawal. No test command marks or retains routes.
// The final writer receipt must remain unchanged through the LLGR replay.
func TestLLGRActualDownKeepsWriterOwner(t *testing.T) {
	for _, restart := range []uint16{0, 3} {
		name := "immediate"
		if restart != 0 {
			name = "timed"
		}
		t.Run(name, func(t *testing.T) {
			peers, rib := llgrDownLiveRouter(t, restart)
			source, receiver := peers[0], peers[1]
			t.Cleanup(func() {
				// Stop GR timers before the SDK and sockets are released.
				for _, peer := range peers {
					peer.peer.reactor.notifyPeerClosed(peer.peer, rpc.ReasonPeerRemoved)
				}
			})
			raw := []byte{24, 10, 0, 3}
			source.send(t, message.PackTo(&message.Update{
				PathAttributes: []byte{
					0x40, 1, 1, 0, // ORIGIN: IGP.
					0x40, 2, 0, // AS_PATH: empty iBGP path.
					0x40, 3, 4, 192, 0, 2, 1, // NEXT_HOP: source.
					0x40, 5, 4, 0, 0, 0, 100, // LOCAL_PREF.
				},
				NLRI: raw,
			}, nil))
			lowEventually(t, func() bool {
				rows := llgrWriterRoutes(t, rib, "received", source.peer.addrString)
				return len(rows) == 1 && rows[0].Prefix == "10.0.3.0/24" && rows[0].StaleLevel == 0
			}, "fresh received control route")
			lowEventually(t, func() bool {
				announced, stale, withdrawn := llgrDownWire(t, receiver, raw)
				return announced == 1 && stale == 0 && withdrawn == 0
			}, "initial control route on recipient TCP")
			lowEventually(t, func() bool {
				return len(llgrWriterRoutes(t, rib, "sent", receiver.peer.addrString)) == 1
			}, "actual sent projection before DOWN")
			writer := &ownershipRailFixture{destination: receiver.peer}
			receipt := llgrWriterReceipt(t, writer, raw, 0, source.peer)

			// TCP EOF drives peer_run's Established exit, apiStateObserver,
			// ordered GR/RS/RIB DOWN delivery and GR's own restart timer.
			require.NoError(t, source.remote.Close())
			lowEventually(t, func() bool {
				return source.peer.State() != PeerStateEstablished && retention.Family(source.peer.addrString, family.IPv4Unicast)
			}, "authoritative GR retention after actual TCP DOWN")
			lowEventually(t, func() bool {
				_, stale, withdrawn := llgrDownWire(t, receiver, raw)
				require.Zero(t, withdrawn, "a later stale announcement cannot excuse a transient withdrawal")
				return stale == 1
			}, "LLGR_STALE control announcement on recipient TCP")
			rows := llgrWriterRoutes(t, rib, "received", source.peer.addrString)
			require.Len(t, rows, 1)
			require.Equal(t, uint8(2), rows[0].StaleLevel)
			require.True(t, retention.Family(source.peer.addrString, family.IPv4Unicast))
			require.Equal(t, receipt, llgrWriterReceipt(t, writer, raw, 0, source.peer),
				"LLGR resend preserves the actual Session owner's receipt")
			require.Equal(t, PeerStateEstablished, receiver.peer.State())
		})
	}
}

// llgrDownLiveRouter keeps production event delivery and real TCP on both peers.
// The source has receive-only RS/RIB bindings, just like the strict timer fixture.
func llgrDownLiveRouter(t *testing.T, restart uint16) ([]*lowLivePeer, *process.Process) {
	t.Helper()
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65000),
		lowLiveSettings("192.0.2.2", 65000, 65000),
	}
	for i, s := range settings {
		s.RouteReflectorClient = true
		s.ClusterID = 0x0a0a0a0a
		// The local OPEN advertises 3600; the remote OPEN below advertises 60.
		caps, err := capability.Parse([]byte{64, 6, 0, 120, 0, 1, 1, 0x80, 71, 7, 0, 1, 1, 0x80, 0, 14, 16})
		require.NoError(t, err)
		s.Capabilities = append(s.Capabilities, caps...)
		for _, binding := range []struct{ name, receive string }{
			{"bgp-gr", "open state eor"},
			{"bgp-rib", "update state refresh"},
			{"bgp-rs", "update-received state open-received refresh"},
			{"bgp-adj-rib-in", "update-received state"},
		} {
			send := ""
			if i == 1 {
				send = "update"
			}
			require.NoError(t, EnsureProcessBinding(s, binding.name, binding.receive, send))
		}
		require.NoError(t, r.AddPeer(s))
	}
	srv := flowForwardPluginServer(t, r, plugin.PluginConfig{Name: "bgp-gr", Internal: true, Encoder: "json"})
	r.SetPluginServer(srv)
	require.NoError(t, r.StartWithContext(context.Background()))
	t.Cleanup(func() { stopAndWait(t, r) })
	startBorrowedPeers(t, r, srv)
	var peers []*lowLivePeer
	for _, s := range settings {
		peer := r.peers[s.PeerKey()]
		lowEventually(t, func() bool { return peer.currentSession() != nil && peer.SessionState() == fsm.StateActive }, "LLGR peer active")
		listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp4", "127.0.0.1:0")
		require.NoError(t, err)
		t.Cleanup(func() { _ = listener.Close() })
		remote, err := (&net.Dialer{}).DialContext(t.Context(), "tcp4", listener.Addr().String())
		require.NoError(t, err)
		server, err := listener.Accept()
		require.NoError(t, err)
		require.NoError(t, listener.Close())
		capture := &lowLivePeer{peer: peer, remote: remote, stopped: make(chan struct{})}
		go capture.readFrames()
		t.Cleanup(func() {
			_ = remote.Close()
			_ = server.Close()
			<-capture.stopped
		})
		require.NoError(t, peer.acceptConnection(server))
		params := []byte{
			2, 6, 65, 4, 0, 0, 0xfd, 0xe8, // ASN4: 65000.
			2, 6, 1, 4, 0, 1, 0, 1, // IPv4 unicast.
			2, 8, 64, 6, byte(restart >> 8), byte(restart), 0, 1, 1, 0x80,
			2, 9, 71, 7, 0, 1, 1, 0x80, 0, 0, 60, // LLST: 60 seconds.
		}
		capture.send(t, message.PackTo(&message.Open{
			Version: 4, MyAS: 65000, HoldTime: 90,
			BGPIdentifier: 0x0a000001 + uint32(len(peers)), OptionalParams: params,
		}, nil))
		lowEventually(t, func() bool { return peer.SessionState() == fsm.StateOpenConfirm }, "LLGR OPEN accepted")
		capture.send(t, message.PackTo(message.NewKeepalive(), nil))
		lowEventually(t, func() bool { return peer.State() == PeerStateEstablished && !peer.pendingSync() }, "LLGR initial sync")
		peers = append(peers, capture)
	}
	return peers, srv.ProcessManager().GetProcess("bgp-rib")
}

// llgrDownWire decodes complete UPDATEs; the community must accompany this NLRI,
// and every withdrawal in the captured TCP history remains visible.
func llgrDownWire(t *testing.T, peer *lowLivePeer, raw []byte) (announced, stale, withdrawn int) {
	t.Helper()
	peer.mu.Lock()
	defer peer.mu.Unlock()
	for _, frame := range peer.frames {
		if frame[18] != 2 {
			continue
		}
		update, err := message.UnpackUpdate(frame[message.HeaderLen:])
		require.NoError(t, err)
		if bytes.Equal(update.WithdrawnRoutes, raw) {
			withdrawn++
		}
		if !bytes.Equal(update.NLRI, raw) {
			continue
		}
		announced++
		_, _, communities, found := attribute.AttrFind(update.PathAttributes, attribute.AttrCommunity)
		if found {
			for offset := 0; offset+4 <= len(communities); offset += 4 {
				if bytes.Equal(communities[offset:offset+4], []byte{0xff, 0xff, 0, 6}) {
					stale++
				}
			}
		}
	}
	return announced, stale, withdrawn
}
