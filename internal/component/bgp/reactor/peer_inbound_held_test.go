// Design: docs/architecture/behavior/peer-lifecycle.md -- a connection held by a peer that has no run loop
// Related: peer_inbound_held.go -- takeUnstartedInbound, adoptHeldInbound, rejectHeldInbound
// Related: reload_rebuild_test.go -- the rebuild that replaces a held peer

package reactor

import (
	"context"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// heldTestConn is the reactor's end of a connection a held peer keeps. It
// records Close, so a test reads whether the reactor closed the connection as
// a state, with no wait.
type heldTestConn struct {
	net.Conn
	closed atomic.Bool
}

func (c *heldTestConn) Close() error {
	c.closed.Store(true)
	return c.Conn.Close()
}

// heldTestRead is everything the neighbor's end read until the reactor closed
// the connection, or until the deadline when nothing closed it.
type heldTestRead struct {
	data []byte
	err  error
}

// heldTestReactor starts a borrow-mode reactor (the daemon's mode) over the
// given peers and leaves them held by the start barrier: StartPeers has not
// run. The caller stops the reactor.
func heldTestReactor(t *testing.T, reload TreeReloadFunc, peers map[string]testPeer) (*Reactor, *reactorAPIAdapter) {
	t.Helper()
	r := New(&Config{ListenAddr: "127.0.0.1:0"}) // Standalone=false: borrow mode
	srv := newBorrowedPluginServer(t, r)
	r.SetPluginServer(srv)
	r.SetTreeReloadFunc(reload)
	require.NoError(t, r.StartWithContext(context.Background()))

	running := configRoot(makeBGPTree(peers))
	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.ApplyConfigDiff(running))
	adapter.SetConfigTree(running)
	return r, adapter
}

// holdTestConnection gives peer a connection the way the listener does for a
// peer that has no session (acceptOrReject calls setInboundConnection), and
// returns the reactor's end and the neighbor's end.
func holdTestConnection(t *testing.T, peer *Peer) (*heldTestConn, net.Conn) {
	t.Helper()
	require.False(t, peerStarted(peer), "the peer must be held by the start barrier")
	local, remote := net.Pipe()
	held := &heldTestConn{Conn: local}
	t.Cleanup(func() {
		require.NoError(t, remote.Close())
	})
	peer.setInboundConnection(held)
	return held, remote
}

// readHeldTestConnection reads the neighbor's end to its close on a goroutine
// of its own, because the reactor's write on a net.Pipe waits for the reader.
// The deadline bounds the read when the reactor never closes the connection,
// which is the defect.
func readHeldTestConnection(t *testing.T, remote net.Conn) <-chan heldTestRead {
	t.Helper()
	require.NoError(t, remote.SetReadDeadline(time.Now().Add(5*time.Second)))
	result := make(chan heldTestRead, 1)
	go func() {
		data, err := io.ReadAll(remote)
		result <- heldTestRead{data: data, err: err}
	}()
	return result
}

// TestHeldInboundMovesToTheReplacedPeer checks that a connection accepted
// before the start barrier opened survives the rebuild that replaces its peer.
//
// Method: a held peer is given a connection, as the listener gives it one.
// The tree reload then answers a different hold time for the same tree, which
// costs a restart, and RebuildRunningPeers replaces the peer. The replacement
// MUST hold that same connection, open, in its inbound slot, where its run
// loop takes it (runOnce calls takeInboundConnection).
//
// VALIDATES: reconcilePeersJournaled hands the connection a never-started
// peer holds to the peer that replaces it under the same key.
// PREVENTS: a neighbor that connected while Ze was starting left on an open
// TCP connection that no peer owns, waiting for an OPEN that never comes.
func TestHeldInboundMovesToTheReplacedPeer(t *testing.T) {
	var stageOneDeclared atomic.Bool
	base := testTreeReload(t)
	reload := func(tree map[string]any) ([]*PeerSettings, Globals, []*DynamicGroupConfig, error) {
		peers, globals, groups, err := base(tree)
		if err != nil {
			return peers, globals, groups, err
		}
		if stageOneDeclared.Load() {
			for _, ps := range peers {
				ps.ReceiveHoldTime = 30 * time.Second
			}
		}
		return peers, globals, groups, nil
	}
	r, _ := heldTestReactor(t, reload, map[string]testPeer{
		"peer1": {remoteIP: "10.0.0.1", remoteAS: "65002", localAS: "65001", holdTime: "90"},
	})
	t.Cleanup(func() { stopAndWait(t, r) })

	before := peerAt(t, r, "10.0.0.1")
	held, _ := holdTestConnection(t, before)

	stageOneDeclared.Store(true)
	require.NoError(t, r.RebuildRunningPeers())

	after := peerAt(t, r, "10.0.0.1")
	require.NotSame(t, before, after, "the rebuild must replace the peer, or this test proves nothing")
	require.False(t, peerStarted(after), "the replacement must stay held by the start barrier")
	require.False(t, held.closed.Load(), "the rebuild closed the connection the replaced peer held")

	arrival, isSocket := after.takeInboundConnection().(*incomingSocket)
	require.True(t, isSocket, "the replacement holds no connection: the one its predecessor held was not handed over")
	require.Same(t, held, arrival.conn, "the replacement holds another connection than the one its predecessor held")
	require.NoError(t, held.Close())
}

// TestHeldInboundIsRejectedWhenThePeerIsRemoved checks that a connection held
// by a peer that never started is refused when the peer leaves the
// configuration.
//
// Method: two held peers, and the first is given a connection. A config apply
// then keeps only the second. The neighbor's end MUST read exactly one
// NOTIFICATION, Cease (6) with subcode Connection Rejected (5), and then the
// close.
//
// VALIDATES: RFC 4486 Section 4, "Connection Rejected", on the connection a
// de-configured peer held (stopWithCease, rejectHeldInbound).
// PREVENTS: the removed peer's connection staying open with no owner.
func TestHeldInboundIsRejectedWhenThePeerIsRemoved(t *testing.T) {
	r, adapter := heldTestReactor(t, testTreeReload(t), map[string]testPeer{
		"peer1": {remoteIP: "10.0.0.1", remoteAS: "65002", localAS: "65001"},
		"peer2": {remoteIP: "10.0.0.2", remoteAS: "65003", localAS: "65001"},
	})
	t.Cleanup(func() { stopAndWait(t, r) })

	held, remote := holdTestConnection(t, peerAt(t, r, "10.0.0.1"))
	result := readHeldTestConnection(t, remote)

	require.NoError(t, adapter.ApplyConfigDiff(configRoot(makeBGPTree(map[string]testPeer{
		"peer2": {remoteIP: "10.0.0.2", remoteAS: "65003", localAS: "65001"},
	}))))

	read := <-result
	require.NoError(t, read.err, "the removed peer's connection was not closed")
	// RFC 4271 Section 4.1 header (16 octets of marker, length 21, type 3),
	// then RFC 4271 Section 4.5: error code 6 (Cease), and RFC 4486 Section 3
	// subcode 5 (Connection Rejected).
	want := []byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0x00, 0x15, 0x03, 0x06, 0x05,
	}
	require.Equal(t, want, read.data)
	require.True(t, held.closed.Load())
}

// TestHeldInboundIsClosedWhenTheReactorStopsBeforeItsPeers checks that a
// connection held for a peer is closed when the daemon stops before the start
// barrier opened.
//
// Method: a held peer is given a connection, and the reactor is stopped
// without StartPeers ever running. The neighbor's end MUST read the close,
// with no message before it: the stop serves StopForRestart too, which owes
// silence, and no OPEN was exchanged on this connection.
//
// VALIDATES: Peer.Stop closes what a peer with no run loop holds.
// PREVENTS: a stopped daemon leaving the connection to the process exit, or
// to the garbage collector when the reactor stops and the process stays.
func TestHeldInboundIsClosedWhenTheReactorStopsBeforeItsPeers(t *testing.T) {
	r, _ := heldTestReactor(t, testTreeReload(t), map[string]testPeer{
		"peer1": {remoteIP: "10.0.0.1", remoteAS: "65002", localAS: "65001"},
	})

	held, remote := holdTestConnection(t, peerAt(t, r, "10.0.0.1"))
	result := readHeldTestConnection(t, remote)

	stopAndWait(t, r)

	read := <-result
	require.NoError(t, read.err, "the stop did not close the connection the held peer kept")
	require.Empty(t, read.data, "the stop wrote to a connection that carried no OPEN")
	require.True(t, held.closed.Load())
}
