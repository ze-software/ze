// Design: docs/architecture/api/process-protocol.md -- BARRIER (all plugins ready) before BGP peers start
// Related: reactor_shutdown_ownership_test.go -- newBorrowedPluginServer, startBorrowedPeers

package reactor

import (
	"context"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// peerStarted reports whether Peer.StartWithContext has run for p: it is the
// only writer of p.cancel.
func peerStarted(p *Peer) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cancel != nil
}

// TestDynamicPeerWaitsForPluginStartupBarrier checks that a dynamic-group
// member accepted between the reactor start and the plugin startup barrier is
// held until the barrier, as a configured peer is.
//
// Method: a borrow-mode reactor (the daemon's mode) with one passive dynamic
// group is started. Before StartPeers runs, the member is built through
// tryCreateDynamicPeer, which every listener path calls. It MUST NOT be started
// yet; once the borrowed server finishes startup and StartPeers runs, it MUST be.
//
// VALIDATES: journal row 2026-10-11 in plan/journal/false-synchronization-claim.md.
// In borrow mode a configured peer starts only from StartPeers, which the bgp
// plugin registers as the coordinator's post-startup callback
// (internal/component/bgp/plugin/register.go), after every plugin finished its
// 5-stage startup.
//
// PREVENTS: a dynamic peer reaching Established before the filter plugins its
// import chain names are connected, so its first UPDATE meets on-error=reject
// ("plugin not connected") and is dropped (bgp-dynamic-filter-swap-live).
func TestDynamicPeerWaitsForPluginStartupBarrier(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0"}) // Standalone=false: borrow mode
	srv := newBorrowedPluginServer(t, r)
	r.SetPluginServer(srv)

	dg := newTestDynamicGroup("ix-peers", []string{"127.0.0.0/8"}, 10)
	dg.Settings.Connection = ConnectionPassive
	r.SetDynamicGroups([]*DynamicGroupConfig{dg})

	require.NoError(t, r.StartWithContext(context.Background()))
	t.Cleanup(func() { stopAndWait(t, r) })

	// The listener is open, plugin startup has not been awaited, and StartPeers
	// has not run.
	peer := r.tryCreateDynamicPeer(netip.MustParseAddr("127.0.0.42"))
	require.NotNil(t, peer, "the dynamic group did not claim 127.0.0.42")
	assert.False(t, peerStarted(peer),
		"a dynamic peer started before the plugin startup barrier (StartPeers): its session can reach Established before its filter plugins connect")

	startBorrowedPeers(t, r, srv)
	assert.True(t, peerStarted(peer),
		"a dynamic peer accepted before the barrier was not started when the barrier released")
}

// barrierTestPeer is a passive configured peer with no local address, so adding
// it to a running reactor opens no listener and starting it dials nothing.
func barrierTestPeer(addr string) *PeerSettings {
	settings := NewPeerSettings(netip.MustParseAddr(addr), 65000, 65001, 0x01020304)
	settings.Connection = ConnectionPassive
	return settings
}

// peerAt returns the reactor's peer for addr, failing the test when there is none.
func peerAt(t *testing.T, r *Reactor, addr string) *Peer {
	t.Helper()
	want := netip.MustParseAddr(addr)
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, peer := range r.peers {
		if peer.Settings().Address == want {
			return peer
		}
	}
	require.FailNow(t, "no peer in the reactor", "address %s", addr)
	return nil
}

// barrierLifted reports whether the peer start barrier is open, under r.mu.
func barrierLifted(r *Reactor) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.peersStarted
}

// TestAddPeerWaitsForPluginStartupBarrier checks that a peer added through
// AddPeer to a running borrow-mode reactor is held until StartPeers, and started
// by it.
//
// Method: a borrow-mode reactor is started, which sets running. AddPeer then adds
// a peer before StartPeers: it MUST NOT be started. Once the borrowed server
// finishes startup and StartPeers runs, it MUST be.
//
// VALIDATES: AddPeer reads the barrier (Reactor.peersStarted), not running.
// PREVENTS: an API-added peer reaching Established before the plugins its policy
// names are connected.
func TestAddPeerWaitsForPluginStartupBarrier(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0"}) // Standalone=false: borrow mode
	srv := newBorrowedPluginServer(t, r)
	r.SetPluginServer(srv)

	require.NoError(t, r.StartWithContext(context.Background()))
	t.Cleanup(func() { stopAndWait(t, r) })

	require.NoError(t, r.AddPeer(barrierTestPeer("127.0.0.51")))
	peer := peerAt(t, r, "127.0.0.51")
	assert.False(t, peerStarted(peer),
		"AddPeer started a peer before the plugin startup barrier (StartPeers)")

	startBorrowedPeers(t, r, srv)
	assert.True(t, peerStarted(peer),
		"a peer AddPeer held before the barrier was not started when the barrier released")
}

// TestStandaloneAddPeerAtBarrierStarts checks that on the standalone path a peer
// AddPeer adds after the barrier copy is taken is started, never left inert.
//
// Method: a standalone reactor holds one configured peer whose mutex the test
// holds for reading. Family validation only reads it, so StartWithContext passes
// validation, lifts the barrier, and blocks in that peer's start, which writes it. AddPeer then adds a second peer in that window: it is not in the
// barrier's copy, so AddPeer itself MUST start it.
//
// VALIDATES: the standalone StartWithContext sets running in the critical section
// that lifts the barrier (Reactor.running, Reactor.peersStarted).
// PREVENTS: a peer added between the barrier copy and the running write being in
// neither start path, so it never starts and nothing says so.
func TestStandaloneAddPeerAtBarrierStarts(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	require.NoError(t, r.AddPeer(barrierTestPeer("127.0.0.61")))
	blocker := peerAt(t, r, "127.0.0.61")

	blocker.mu.RLock()
	started := make(chan error, 1)
	go func() { started <- r.StartWithContext(context.Background()) }()
	require.Eventually(t, func() bool { return barrierLifted(r) }, 10*time.Second, time.Millisecond,
		"the standalone start never lifted the peer start barrier")

	addErr := r.AddPeer(barrierTestPeer("127.0.0.62"))
	blocker.mu.RUnlock()
	require.NoError(t, <-started)
	t.Cleanup(func() { stopAndWait(t, r) })
	require.NoError(t, addErr)

	assert.True(t, peerStarted(peerAt(t, r, "127.0.0.62")),
		"a peer AddPeer added after the barrier copy was never started")
}

// TestPeerSliceExcludesDynamicMembers checks that the update-delay expected set
// holds the configured peers only.
//
// Method: a start map holding one configured peer and one dynamic-group member,
// as StartPeers sees it when the member was accepted while the barrier was closed.
//
// VALIDATES: peerSlice drops IsDynamic peers (update_delay.go, arm).
// PREVENTS: a member accepted during plugin startup becoming part of startup
// convergence, so the hold waits on a peer the operator never configured.
func TestPeerSliceExcludesDynamicMembers(t *testing.T) {
	configured := NewPeer(barrierTestPeer("127.0.0.71"))
	memberSettings := barrierTestPeer("127.0.0.72")
	memberSettings.IsDynamic = true
	member := NewPeer(memberSettings)

	got := peerSlice(map[netip.AddrPort]*Peer{
		netip.MustParseAddrPort("127.0.0.71:179"): configured,
		netip.MustParseAddrPort("127.0.0.72:179"): member,
	})
	addrs := make([]netip.Addr, 0, len(got))
	for _, peer := range got {
		addrs = append(addrs, peer.Settings().Address)
	}
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("127.0.0.71")}, addrs,
		"the update-delay expected set does not hold exactly the configured peer")
}

// TestRestartClosesPeerStartBarrier checks that a reactor started again after a
// stop holds its peers until that run's StartPeers.
//
// Method: a borrow-mode reactor runs through StartPeers, stops, and starts again.
// The barrier MUST be closed, and a peer AddPeer adds then MUST NOT be started.
//
// VALIDATES: StartWithContext clears peersStarted.
// PREVENTS: a second run starting peers before its own plugin startup finished,
// on the strength of the previous run's barrier.
func TestRestartClosesPeerStartBarrier(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0"}) // Standalone=false: borrow mode
	srv := newBorrowedPluginServer(t, r)
	r.SetPluginServer(srv)

	require.NoError(t, r.StartWithContext(context.Background()))
	startBorrowedPeers(t, r, srv)
	require.True(t, barrierLifted(r), "StartPeers did not lift the barrier")
	stopAndWait(t, r)

	require.NoError(t, r.StartWithContext(context.Background()))
	t.Cleanup(func() { stopAndWait(t, r) })
	assert.False(t, barrierLifted(r), "a restarted reactor kept the previous run's barrier open")

	require.NoError(t, r.AddPeer(barrierTestPeer("127.0.0.81")))
	assert.False(t, peerStarted(peerAt(t, r, "127.0.0.81")),
		"a restarted reactor started a peer before its own plugin startup barrier")
}

// TestRefusedPeerStartNamesHeldPeers checks that a refused barrier leaves no
// held peer silent.
//
// Method: a borrow-mode reactor holds a configured peer whose family has no
// decoder, so StartPeers refuses (validatePeerFamilies). The daemon keeps
// running after that (bgp/plugin/register.go), so the error log MUST name the
// held peer, and a peer AddPeer adds afterwards MUST be held and named too.
//
// VALIDATES: Reactor.peerStartRefusal and logPeerHeldByRefusalLocked.
// PREVENTS: peers waiting forever behind a barrier that will never lift, with
// only the refusal itself in the log.
func TestRefusedPeerStartNamesHeldPeers(t *testing.T) {
	var sink syncBuffer
	prev := reactorLogger
	reactorLogger = func() *slog.Logger {
		return slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelError}))
	}
	t.Cleanup(func() { reactorLogger = prev })

	r := New(&Config{ListenAddr: "127.0.0.1:0"}) // Standalone=false: borrow mode
	srv := newBorrowedPluginServer(t, r)
	r.SetPluginServer(srv)
	undecodable := barrierTestPeer("127.0.0.91")
	undecodable.Capabilities = []capability.Capability{&capability.Multiprotocol{AFI: 99, SAFI: 99}}
	require.NoError(t, r.AddPeer(undecodable))

	require.NoError(t, r.StartWithContext(context.Background()))
	t.Cleanup(func() { stopAndWait(t, r) })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, srv.WaitForStartupComplete(ctx))
	require.Error(t, r.StartPeers(), "StartPeers accepted a family with no decoder")

	require.NoError(t, r.AddPeer(barrierTestPeer("127.0.0.92")))
	assert.False(t, peerStarted(peerAt(t, r, "127.0.0.92")), "a peer started behind a refused barrier")
	logged := sink.String()
	assert.Contains(t, logged, "peer=127.0.0.91", "the refusal did not name the peer it holds")
	assert.Contains(t, logged, "peer=127.0.0.92", "a peer added after the refusal was held in silence")
}
