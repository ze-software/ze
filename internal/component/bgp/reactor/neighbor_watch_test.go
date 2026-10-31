package reactor

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/network"
)

// fakeNeighborTable stands in for the interface component's registered
// neighbor table: a table the test edits, and a watch the test signals or
// ends. Each watch closes its channel when its ctx ends or the test ends it,
// as the network.NeighborWatcher contract requires.
type fakeNeighborTable struct {
	mu      sync.Mutex
	entries []network.Neighbor
	starts  int
	watches []fakeNeighborWatch
	// startErr, when set, is every watch start's answer.
	startErr error
	// endAtOnce closes every watch's channel as it starts, the shape of a
	// kernel subscription that keeps failing.
	endAtOnce bool
}

// fakeNeighborWatch is one started watch: the channel the reactor ranges over,
// and the signal that ends it the way a replaced backend does.
type fakeNeighborWatch struct {
	changes chan struct{}
	end     chan struct{}
}

// install replaces the reactor's table read and watch with this fake until
// the test ends.
func (f *fakeNeighborTable) install(t *testing.T) {
	t.Helper()
	savedRead, savedWatch := readNeighborTable, watchNeighborTable
	readNeighborTable = f.read
	watchNeighborTable = f.watch
	t.Cleanup(func() {
		readNeighborTable = savedRead
		watchNeighborTable = savedWatch
	})
}

func (f *fakeNeighborTable) read() ([]network.Neighbor, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]network.Neighbor(nil), f.entries...), nil
}

func (f *fakeNeighborTable) watch(ctx context.Context) (<-chan struct{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	if f.startErr != nil {
		return nil, f.startErr
	}
	w := fakeNeighborWatch{changes: make(chan struct{}, 1), end: make(chan struct{})}
	f.watches = append(f.watches, w)
	if f.endAtOnce {
		close(w.changes)
		return w.changes, nil
	}
	go closeFakeWatch(ctx, w)
	return w.changes, nil
}

// closeFakeWatch is the fake watch's goroutine: it closes the channel when ctx
// ends or the test ends the watch.
func closeFakeWatch(ctx context.Context, w fakeNeighborWatch) {
	select {
	case <-ctx.Done():
	case <-w.end:
	}
	close(w.changes)
}

func (f *fakeNeighborTable) add(entry network.Neighbor) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append(f.entries, entry)
}

func (f *fakeNeighborTable) startCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts
}

// latest answers the last watch started.
func (f *fakeNeighborTable) latest() fakeNeighborWatch {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.watches[len(f.watches)-1]
}

// addLivePeer adds a peer with a live forwarding snapshot to r, the state a
// peer holds while its session is established, so a refresh rebuilds it.
func addLivePeer(t *testing.T, r *Reactor) *Peer {
	t.Helper()
	s := NewPeerSettings(netip.MustParseAddr("2001:db8:1::2"), 65000, 65001, 0x01010101)
	s.LocalAddress = netip.MustParseAddr("2001:db8:1::1")
	peer := NewPeer(s)
	peer.SetReactor(r)
	peer.refreshLinkScopeFrom(nil)
	peer.fwdFacts.Store(peer.buildForwardFacts())
	r.mu.Lock()
	r.peers[s.PeerKey()] = peer
	r.mu.Unlock()
	return peer
}

// thirdPartyAnswer answers the third-party Link-Local the neighbor half of
// peer's current scope names for N, probed against the shared subnet.
func thirdPartyAnswer(peer *Peer) netip.Addr {
	scope := peer.llScope.Load()
	if scope == nil {
		return netip.Addr{}
	}
	connected := []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}
	probe := newLinkScopeWith(connected, scope.neighbors, peer.settings.Address)
	return probe.linkLocalNextHop(netip.Addr{}, netip.MustParseAddr(thirdPartyGlobal), nextHopRouterThirdParty, linkLocalConditionNextHopAndPeer)
}

// TestNeighbourWatchRunsWithoutEventBus is the defect's own test: a reactor
// started with NO event bus, the shape of a BGP-only config where no interface
// monitor runs, still hears a neighbor table change and pairs a router that
// resolved after its session came up. It drives the reactor's real start path
// (StartWithContext) with the interface component's table replaced by a fake,
// and stops the reactor to prove the watch ends with it.
//
// MUTATION: remove r.startNeighborWatch() from StartWithContext, or start the
// watch only from subscribeInterfaceEvents (which returns without a bus): the
// watch never starts and the router stays unpaired.
func TestNeighbourWatchRunsWithoutEventBus(t *testing.T) {
	table := &fakeNeighborTable{entries: []network.Neighbor{
		{Address: netip.MustParseAddr(thirdPartyGlobal), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid},
	}}
	table.install(t)

	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	require.Nil(t, r.eventBus, "the test needs a reactor with no event bus")
	require.NoError(t, r.StartWithContext(context.Background()))
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			stopAndWait(t, r)
		}
	})

	require.Eventually(t, func() bool { return r.neighborWatchState().status == neighborWatchRunning },
		5*time.Second, time.Millisecond, "the reactor started a neighbor table watch")
	peer := addLivePeer(t, r)
	require.False(t, thirdPartyAnswer(peer).IsValid(), "before the change the table names no Link-Local")

	// N's Link-Local resolves after the session came up.
	table.add(network.Neighbor{Address: netip.MustParseAddr(thirdPartyLinkLocal), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid})
	table.latest().changes <- struct{}{}
	require.Eventually(t, func() bool { return thirdPartyAnswer(peer).IsValid() },
		5*time.Second, time.Millisecond, "the watch signal re-read the table")
	assert.Equal(t, netip.MustParseAddr(thirdPartyLinkLocal), thirdPartyAnswer(peer))
	assert.Equal(t, 1, table.startCount(), "one watch for the reactor's life")

	stopAndWait(t, r)
	stopped = true
	_, open := <-table.latest().changes
	assert.False(t, open, "the watch ended with the reactor")
}

// TestNeighbourWatchStartsAgainAfterItEnds proves a watch that closes its
// channel (its kernel subscription failing; a replaced backend never closes
// it) is published as stopped and started again after the retry delay. The
// table is read again when the stop is published and when the new watch
// runs, so a change made while no watch ran reaches the peer.
func TestNeighbourWatchStartsAgainAfterItEnds(t *testing.T) {
	table := &fakeNeighborTable{entries: []network.Neighbor{
		{Address: netip.MustParseAddr(thirdPartyGlobal), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid},
	}}
	table.install(t)
	r := newIfaceTestReactor(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go runNeighborWatchWorker(ctx, r, done)
	t.Cleanup(func() {
		cancel()
		<-done
	})

	require.Eventually(t, func() bool { return r.neighborWatchState().status == neighborWatchRunning },
		5*time.Second, time.Millisecond, "the first watch runs")
	peer := addLivePeer(t, r)

	// The change lands while no watch runs: the first watch ends unsignalled.
	table.add(network.Neighbor{Address: netip.MustParseAddr(thirdPartyLinkLocal), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid})
	close(table.latest().end)
	require.Eventually(t, func() bool {
		state := r.neighborWatchState()
		return state.status == neighborWatchStopped && errors.Is(state.err, errNeighborWatchEnded)
	}, 5*time.Second, time.Millisecond, "the ended watch is published as stopped")

	require.Eventually(t, func() bool { return table.startCount() == 2 && r.neighborWatchState().status == neighborWatchRunning },
		5*time.Second, time.Millisecond, "a new watch starts")
	require.Eventually(t, func() bool { return thirdPartyAnswer(peer).IsValid() },
		5*time.Second, time.Millisecond, "the restart re-read the table")
}

// TestNeighbourWatchRetryDoublesForAFailingSource proves a watch that starts
// and then ends at once, every time, is retried after a delay that doubles to
// neighborWatchRetryMax and stays there, so a failing kernel subscription
// costs one start and one warning per delay rather than one per second.
// Method: a table whose every watch is closed at once, and a wait that
// records each delay without sleeping and stops the worker after nine.
//
// MUTATION: reset retry to neighborWatchRetryFirst on every successful start:
// every recorded delay is one second.
func TestNeighbourWatchRetryDoublesForAFailingSource(t *testing.T) {
	table := &fakeNeighborTable{endAtOnce: true}
	table.install(t)
	var delays []time.Duration
	saved := waitNeighborWatch
	waitNeighborWatch = func(_ context.Context, delay time.Duration) bool {
		delays = append(delays, delay)
		return len(delays) < 9
	}
	t.Cleanup(func() { waitNeighborWatch = saved })

	r := newIfaceTestReactor(t)
	r.neighborWatchWorker(context.Background())

	want := []time.Duration{
		time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second,
		32 * time.Second, time.Minute, time.Minute, time.Minute,
	}
	assert.Equal(t, want, delays, "the delay between two starts of a failing watch")
	assert.Equal(t, 9, table.startCount(), "one start per delay")
}

// TestNeighbourWatchWorkerKeepsANewerRunsState proves a worker that ends
// clears only the state it published itself: once a newer run has published
// its own, the older worker's exit leaves that state in place.
//
// MUTATION: clear with r.neighborWatch.Store(nil) on exit: the newer run's
// state is erased and reads as not started.
func TestNeighbourWatchWorkerKeepsANewerRunsState(t *testing.T) {
	table := &fakeNeighborTable{}
	table.install(t)
	r := newIfaceTestReactor(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go runNeighborWatchWorker(ctx, r, done)
	require.Eventually(t, func() bool { return r.neighborWatchState().status == neighborWatchRunning },
		5*time.Second, time.Millisecond, "the worker's watch runs")

	newer := neighborWatchState{status: neighborWatchStopped, err: network.ErrNeighborTableNotRegistered}
	r.neighborWatch.Store(&newer)
	cancel()
	<-done
	got := r.neighborWatchState()
	assert.Equal(t, neighborWatchStopped, got.status, "the newer run's state survives the older worker's exit")
	assert.ErrorIs(t, got.err, network.ErrNeighborTableNotRegistered)
}

// TestNeighbourWatchStartFailureIsPublished proves a watch that cannot start
// is published as stopped with the reason, never as running, so the warning
// on a later miss can name it.
func TestNeighbourWatchStartFailureIsPublished(t *testing.T) {
	table := &fakeNeighborTable{startErr: network.ErrNeighborTableNotRegistered}
	table.install(t)
	r := newIfaceTestReactor(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go runNeighborWatchWorker(ctx, r, done)
	t.Cleanup(func() {
		cancel()
		<-done
	})
	require.Eventually(t, func() bool {
		state := r.neighborWatchState()
		return state.status == neighborWatchStopped && errors.Is(state.err, network.ErrNeighborTableNotRegistered)
	}, 5*time.Second, time.Millisecond, "the failed start is published")
}

// runNeighborWatchWorker runs r's worker under ctx and closes done after it.
func runNeighborWatchWorker(ctx context.Context, r *Reactor, done chan<- struct{}) {
	r.neighborWatchWorker(ctx)
	close(done)
}

// captureReactorLog swaps the reactor logger for one writing warnings to the
// returned buffer, and resets the third-party Link-Local rate limits so the
// test's first line is not suppressed by an earlier test's.
func captureReactorLog(t *testing.T) *syncBuffer {
	t.Helper()
	var sink syncBuffer
	prev := reactorLogger
	reactorLogger = func() *slog.Logger {
		return slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelWarn}))
	}
	resetThirdPartyLinkLocalLog()
	t.Cleanup(func() {
		reactorLogger = prev
		resetThirdPartyLinkLocalLog()
	})
	return &sink
}

func resetThirdPartyLinkLocalLog() {
	reset := func(w *logWindow) {
		w.nextAllowed.Store(0)
		w.suppressed.Store(0)
	}
	reset(&thirdPartyLinkLocalLog.unread)
	reset(&thirdPartyLinkLocalLog.unwatched)
	for i := range thirdPartyLinkLocalLog.outcomes {
		reset(&thirdPartyLinkLocalLog.outcomes[i])
	}
}

// TestThirdPartyMissWarningTellsTheTruth proves the warning for a next hop
// sent without its router's Link-Local distinguishes a table nobody watches
// from a watched table with no matching entry, and names the fix for each.
// The defect it guards: with no watch running, the line said the table held
// no entry while `ip -6 neigh` showed the entry bound, and sent the operator
// to the next-hop router.
func TestThirdPartyMissWarningTellsTheTruth(t *testing.T) {
	global := netip.MustParseAddr(thirdPartyGlobal)
	peer := netip.MustParseAddr("2001:db8:1::2")
	connected := []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}
	noEntry := network.NewNeighborLinkLocals(nil)

	for _, tc := range []struct {
		name    string
		view    neighborView
		want    []string
		notWant []string
	}{
		{
			name:    "watched, no entry",
			view:    neighborView{links: noEntry, watch: neighborWatchState{status: neighborWatchRunning}},
			want:    []string{"the neighbor table holds no entry for the next hop", "must answer neighbor discovery"},
			notWant: []string{"not watched"},
		},
		{
			name: "unwatched, no table registered",
			view: neighborView{links: noEntry, watch: neighborWatchState{status: neighborWatchStopped, err: network.ErrNeighborTableNotRegistered}},
			want: []string{"the neighbor table is not watched", "use a ze build that includes the interface component"},
		},
		{
			name: "unwatched, watch failed",
			view: neighborView{links: noEntry, watch: neighborWatchState{status: neighborWatchStopped, err: errors.New("subscribe to kernel neighbor changes: permission denied")}},
			want: []string{"the neighbor table is not watched", "permission denied", "netlink interface backend"},
		},
		{
			name: "unwatched, never started",
			view: neighborView{links: noEntry},
			want: []string{"the neighbor table is not watched", errNeighborWatchNotStarted.Error()},
		},
		{
			name:    "unread, no table registered",
			view:    neighborView{err: network.ErrNeighborTableNotRegistered, watch: neighborWatchState{status: neighborWatchRunning}},
			want:    []string{"the neighbor table cannot be read", "use a ze build that includes the interface component"},
			notWant: []string{"not watched"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sink := captureReactorLog(t)
			scope := newLinkScopeWith(connected, tc.view, peer)
			got := scope.linkLocalNextHop(netip.MustParseAddr("fe80::1"), global, nextHopRouterThirdParty, linkLocalConditionNextHopAndPeer)
			require.False(t, got.IsValid(), "a miss sends the global alone")
			line := sink.String()
			for _, want := range tc.want {
				assert.True(t, strings.Contains(line, want), "the warning names %q:\n%s", want, line)
			}
			for _, notWant := range tc.notWant {
				assert.False(t, strings.Contains(line, notWant), "the warning must not say %q:\n%s", notWant, line)
			}
		})
	}
}
