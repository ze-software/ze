// Design: docs/architecture/behavior/peer-lifecycle.md -- inbound connection handling for passive peers
// Related: reactor_dynamic_startup_barrier_test.go -- peerStarted, peerAt, barrierLifted
// Related: peer_test.go -- freePort
// Related: reactor_shutdown_ownership_test.go -- stopAndWait

package reactor

import (
	"context"
	"io"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/network"
)

// startWindowHold is the announce delay the start-window tests configure. No
// other reactor wait asks the clock for this duration, so startWindowClock can
// tell the standalone start's announce-delay hold from every other wait.
const startWindowHold = 97*time.Hour + 13*time.Minute

// startWindowBound is the real-time bound on each wait for another goroutine:
// the start reaching its hold, the start returning, a connection being handed
// to the peer. It is never a delay the tests wait out.
const startWindowBound = 10 * time.Second

// startWindowLocal is the local address every start-window peer listens on.
var startWindowLocal = netip.MustParseAddr("127.0.0.1")

// startWindowClock is the real clock, except for the standalone start's
// announce-delay hold (StartWithContext, ze.bgp.announce.delay): that wait
// closes held and lasts until the test sends on release.
//
// Safe for concurrent use.
type startWindowClock struct {
	clock.RealClock
	held     chan struct{}
	heldOnce sync.Once
	release  chan time.Time
}

func newStartWindowClock() *startWindowClock {
	return &startWindowClock{held: make(chan struct{}), release: make(chan time.Time, 1)}
}

// After answers the hold's channel for startWindowHold, and a real timer for
// any other duration.
func (c *startWindowClock) After(d time.Duration) <-chan time.Time {
	if d != startWindowHold {
		return c.RealClock.After(d)
	}
	c.heldOnce.Do(func() { close(c.held) })
	return c.release
}

// startWindow is a standalone reactor held inside its start: the listeners
// have started and the plugin waits are over, and the announce-delay hold
// keeps running false and the peer start barrier closed (Reactor.running,
// Reactor.peersStarted). r.mu is free, so AddPeer runs as a reload or an API
// call would.
type startWindow struct {
	reactor *Reactor
	clock   *startWindowClock
	cancel  context.CancelFunc
	started chan error
}

// openStartWindow configures the hold, starts r, and returns once the start
// waits in the hold. The caller MUST end the window with release or abort; a
// test that fails first leaves the cleanup to cancel the start.
func openStartWindow(t *testing.T, r *Reactor) *startWindow {
	t.Helper()
	require.NoError(t, env.Set("ze.bgp.announce.delay", startWindowHold.String()))
	t.Cleanup(func() {
		_ = os.Unsetenv("ze.bgp.announce.delay")
		env.ResetCache()
	})

	clk := newStartWindowClock()
	r.SetClock(clk)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	w := &startWindow{reactor: r, clock: clk, cancel: cancel, started: make(chan error, 1)}
	go func() { w.started <- r.StartWithContext(ctx) }()

	select {
	case <-clk.held:
	case err := <-w.started:
		require.FailNow(t, "the standalone start returned before its announce-delay hold", "error: %v", err)
	case <-time.After(startWindowBound):
		require.FailNow(t, "the standalone start never reached its announce-delay hold")
	}
	require.False(t, barrierLifted(r), "the peer start barrier was open inside the announce-delay hold")
	return w
}

// awaitStart returns what StartWithContext returned.
func (w *startWindow) awaitStart(t *testing.T) error {
	t.Helper()
	select {
	case err := <-w.started:
		return err
	case <-time.After(startWindowBound):
		require.FailNow(t, "the standalone start did not return after its announce-delay hold ended")
		return nil
	}
}

// release ends the hold: the start opens the peer start barrier and returns.
// The reactor is stopped when the test ends.
func (w *startWindow) release(t *testing.T) {
	t.Helper()
	w.clock.release <- time.Time{}
	require.NoError(t, w.awaitStart(t), "the standalone start failed after its announce-delay hold")
	require.True(t, barrierLifted(w.reactor), "the standalone start returned with the peer start barrier closed")
	t.Cleanup(func() { stopAndWait(t, w.reactor) })
}

// abort cancels the start inside the hold, which aborts it (abortStartup).
func (w *startWindow) abort(t *testing.T) {
	t.Helper()
	w.cancel()
	require.ErrorIs(t, w.awaitStart(t), context.Canceled, "a start cancelled in its announce-delay hold did not abort")
}

// startWindowPeer is a passive peer at addr that listens on
// startWindowLocal:listenPort. The port is not the BGP port, so a listener
// with this peer as its only claimant hands it every connection
// (listenerPeerKey).
func startWindowPeer(addr string, listenPort int) *PeerSettings {
	settings := NewPeerSettings(netip.MustParseAddr(addr), 65000, 65001, 0x01020304)
	settings.Connection = ConnectionPassive
	settings.LocalAddress = startWindowLocal
	settings.LocalPort = uint16(listenPort) //nolint:gosec // freePort answers a TCP port, which fits a uint16
	return settings
}

// startWindowEndpoint is the listener key for startWindowLocal:listenPort.
func startWindowEndpoint(listenPort int) string {
	return net.JoinHostPort(startWindowLocal.String(), strconv.Itoa(listenPort))
}

// reactorListener returns the reactor's listener for endpoint, nil when it has
// none.
func reactorListener(r *Reactor, endpoint string) *Listener {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.listeners[endpoint]
}

// reactorListenerCount is how many per-address listeners the reactor holds.
func reactorListenerCount(r *Reactor) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.listeners)
}

// reactorHasPeer reports whether the reactor's peer set holds settings' key.
func reactorHasPeer(r *Reactor, settings *PeerSettings) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.peers[settings.PeerKey()]
	return ok
}

// peerHoldsInbound reports whether p's inbound slot holds a connection
// (setInboundConnection).
func peerHoldsInbound(p *Peer) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.inbound.conn != nil
}

// requireEndpointFree fails the test when something listens on endpoint: the
// kernel then refuses a bind, which only a listening socket causes here.
func requireEndpointFree(t *testing.T, endpoint string) {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", endpoint)
	require.NoError(t, err, "a socket listens on %s", endpoint)
	require.NoError(t, ln.Close())
}

// dialStartWindow connects to endpoint and closes the connection when the
// test ends.
func dialStartWindow(t *testing.T, endpoint string) net.Conn {
	t.Helper()
	dialer := net.Dialer{Timeout: startWindowBound}
	conn, err := dialer.DialContext(context.Background(), "tcp", endpoint)
	require.NoError(t, err, "no listener accepted a connection on %s", endpoint)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// TestPassivePeerAddedBeforeBarrierGetsListener checks that a passive peer
// added during the standalone start, after the listeners started and before
// the peer start barrier opens, has a listener on its address and port.
//
// Method: a standalone reactor is held in its announce-delay hold
// (openStartWindow). AddPeer adds a passive peer on a free port, and a TCP
// connection to that address and port MUST be accepted while the barrier is
// still closed.
//
// VALIDATES: AC-11. addPeerLocked runs its listener block once the listeners
// have started, not once running is set.
// PREVENTS: journal row 2026-10-13 in plan/journal/unwired-feature.md, a
// passive peer added in the window accepting nothing.
func TestPassivePeerAddedBeforeBarrierGetsListener(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	w := openStartWindow(t, r)

	listenPort := freePort(t)
	require.NoError(t, r.AddPeer(startWindowPeer("127.0.0.2", listenPort)))
	dialStartWindow(t, startWindowEndpoint(listenPort))
	assert.False(t, barrierLifted(r), "the peer start barrier opened before the connection was made")

	w.release(t)
}

// TestPassivePeerAddedBeforeBarrierHoldsInboundUntilBarrier checks that a
// connection accepted for a peer added in the start window waits in the
// peer's inbound slot, and reaches the peer's session only once the barrier
// opens.
//
// Method: a peer is added and connected to inside the hold. The connection
// MUST land in the peer's inbound slot (acceptOrReject, setInboundConnection)
// with the peer not started and holding no session. Once the hold ends, the
// peer's session MUST take the connection and send its OPEN on it.
//
// VALIDATES: A-4 and AC-11. An unstarted peer has no session, so
// Peer.acceptConnection answers ErrNotConnected and the connection waits in
// the slot; only Peer.run drains it, and the barrier is what starts Peer.run.
// PREVENTS: a peer added in the window opening a session before the plugins
// its policy names are connected.
func TestPassivePeerAddedBeforeBarrierHoldsInboundUntilBarrier(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	w := openStartWindow(t, r)

	listenPort := freePort(t)
	settings := startWindowPeer("127.0.0.2", listenPort)
	require.NoError(t, r.AddPeer(settings))
	peer := peerAt(t, r, "127.0.0.2")
	conn := dialStartWindow(t, startWindowEndpoint(listenPort))

	require.Eventually(t, func() bool { return peerHoldsInbound(peer) }, startWindowBound, time.Millisecond,
		"the accepted connection never reached the peer's inbound slot")
	assert.False(t, peerStarted(peer), "a peer added in the start window started before the barrier")
	assert.Nil(t, peer.currentSession(), "a peer added in the start window held a session before the barrier")

	w.release(t)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(startWindowBound)))
	header := make([]byte, message.HeaderLen)
	_, err := io.ReadFull(conn, header)
	require.NoError(t, err, "the peer's session sent nothing on the held connection after the barrier opened")
	for i := range message.MarkerLen {
		require.Equal(t, byte(0xFF), header[i], "byte %d of the BGP marker", i)
	}
	assert.Equal(t, byte(msgtype.TypeOPEN), header[message.MarkerLen+2],
		"the first message on the held connection is not an OPEN")
}

// TestListenerRebuiltForMD5PeerAddedBeforeBarrier checks that a passive peer
// with an MD5 key, added in the start window on a pair that already has a
// listener, gets that listener rebuilt with its key.
//
// Method: a configured passive peer without a key gives the start a listener
// on a free port. Inside the hold, AddPeer adds a second passive peer with a
// key on the same address and port. The listener for that pair MUST then be
// built by a factory that carries the second peer's key, because the key set
// is read once, when the socket is created (newListenerFactory).
//
// VALIDATES: AC-12, the MD5 arm of the listener block of addPeerLocked.
// PREVENTS: an MD5 peer added in the window whose SYN the kernel drops for
// want of its key on the listening socket.
func TestListenerRebuiltForMD5PeerAddedBeforeBarrier(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" {
		t.Skip("setTCPMD5Sig (internal/core/network) sets TCP_MD5SIG only on Linux and FreeBSD; elsewhere the rebuilt listener cannot open")
	}
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	listenPort := freePort(t)
	require.NoError(t, r.AddPeer(startWindowPeer("127.0.0.2", listenPort)))
	w := openStartWindow(t, r)

	endpoint := startWindowEndpoint(listenPort)
	require.NotNil(t, reactorListener(r, endpoint), "the start opened no listener for the configured peer")

	keyed := startWindowPeer("127.0.0.3", listenPort)
	keyed.MD5Key = "start-window-key"
	require.NoError(t, r.AddPeer(keyed))

	rebuilt := reactorListener(r, endpoint)
	require.NotNil(t, rebuilt, "no listener on %s after the MD5 peer was added", endpoint)
	want := network.RealListenerFactory{MD5Peers: []network.MD5Peer{{
		Addr: keyed.Address.AsSlice(),
		Key:  keyed.MD5Key,
	}}}
	r.mu.RLock()
	got := rebuilt.listenerFactory
	r.mu.RUnlock()
	assert.Equal(t, want, got, "the listener on %s was not built with the added peer's MD5 key", endpoint)

	w.release(t)
}

// TestAddPeerBeforeBarrierListenerFailureRollsBack checks that AddPeer in the
// start window fails, and leaves no peer behind, when the peer's listener
// cannot start.
//
// Method: the test holds a socket on a free port. Inside the hold, AddPeer
// adds a passive peer on that address and port. It MUST return the listen
// error naming the address and port, the peer MUST be absent from the peer
// set, and the start MUST still finish (release).
//
// VALIDATES: AC-13 and R-7. The window takes the contract AddPeer already has
// while running.
// PREVENTS: an AddPeer that answers success for a peer that can accept
// nothing.
func TestAddPeerBeforeBarrierListenerFailureRollsBack(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	w := openStartWindow(t, r)

	listenPort := freePort(t)
	endpoint := startWindowEndpoint(listenPort)
	var lc net.ListenConfig
	occupier, err := lc.Listen(context.Background(), "tcp", endpoint)
	require.NoError(t, err)
	t.Cleanup(func() { _ = occupier.Close() })

	settings := startWindowPeer("127.0.0.2", listenPort)
	addErr := r.AddPeer(settings)
	require.ErrorContains(t, addErr, "listen on "+endpoint,
		"AddPeer did not return the listen error naming the address and port for a peer whose listener cannot start")
	assert.False(t, reactorHasPeer(r, settings), "the peer whose listener failed stayed in the peer set")

	w.release(t)
}

// TestAddPeerBeforeStartOpensNoListener checks that a passive peer added
// before StartWithContext gets no listener from AddPeer.
//
// Method: AddPeer adds a passive peer with a local address to a reactor that
// was never started. The peer MUST be recorded, the reactor MUST hold no
// listener, and nothing MUST listen on the peer's address and port.
//
// VALIDATES: AC-13b, the start's own listener pass (startMultiListeners) is
// what covers a peer added during the configuration load.
// PREVENTS: the configuration load binding sockets before the start.
func TestAddPeerBeforeStartOpensNoListener(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	listenPort := freePort(t)
	settings := startWindowPeer("127.0.0.2", listenPort)

	require.NoError(t, r.AddPeer(settings))
	assert.True(t, reactorHasPeer(r, settings), "the peer was not recorded")
	assert.Zero(t, reactorListenerCount(r), "AddPeer before the start opened a listener")
	requireEndpointFree(t, startWindowEndpoint(listenPort))
}

// TestAddPeerAfterStopOpensNoListener checks that a passive peer added to a
// reactor whose run has ended gets no listener, whether the run ended in a
// stop or in an aborted start.
//
// Method: in each case the reactor's run ends first: a full start then
// stopAndWait, or a start cancelled inside its announce-delay hold, which
// aborts after the listeners started. AddPeer then adds a passive peer with a
// local address. The peer MUST be recorded, the reactor MUST hold no
// listener, and nothing MUST listen on the peer's address and port.
//
// VALIDATES: AC-13b and R-10. stopAllListeners, which both the stop
// (cleanup) and the abort (abortStartup) call, clears the fact that the
// listeners started.
// PREVENTS: an AddPeer on a stopped reactor opening a socket nothing closes.
func TestAddPeerAfterStopOpensNoListener(t *testing.T) {
	t.Run("stop", func(t *testing.T) {
		r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
		require.NoError(t, r.StartWithContext(context.Background()))
		stopAndWait(t, r)

		listenPort := freePort(t)
		settings := startWindowPeer("127.0.0.2", listenPort)
		require.NoError(t, r.AddPeer(settings))
		assert.True(t, reactorHasPeer(r, settings), "the peer was not recorded")
		assert.Zero(t, reactorListenerCount(r), "AddPeer after a stop opened a listener")
		requireEndpointFree(t, startWindowEndpoint(listenPort))
	})

	t.Run("aborted start", func(t *testing.T) {
		r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
		configuredPort := freePort(t)
		require.NoError(t, r.AddPeer(startWindowPeer("127.0.0.3", configuredPort)))
		w := openStartWindow(t, r)
		require.NotNil(t, reactorListener(r, startWindowEndpoint(configuredPort)),
			"the start opened no listener before its hold, so the abort has none to close")
		w.abort(t)

		listenPort := freePort(t)
		settings := startWindowPeer("127.0.0.2", listenPort)
		require.NoError(t, r.AddPeer(settings))
		assert.True(t, reactorHasPeer(r, settings), "the peer was not recorded")
		assert.Zero(t, reactorListenerCount(r), "AddPeer after an aborted start opened a listener")
		requireEndpointFree(t, startWindowEndpoint(listenPort))
	})
}
