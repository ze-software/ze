package reactor

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/test/sim"
)

// triggerClock is a clock whose After() channel the test fires by hand, so a
// timeout branch can be reached with no wall-clock dependence at all.
//
// sim.FakeClock alone cannot do this: its After() returns a channel that never
// fires and cannot be advanced (sim.go:102). Asserting on elapsed time instead
// would make the test a load detector rather than a behavior test
// (ai/rules/fix-dont-record.md).
// It also reports WHEN a wait blocks: After() is evaluated as the select's
// operand, so a receive on waiting proves the caller reached the wait and has
// not passed it. That turns "is sendInitialRoutes blocked in the barrier?" into
// a deterministic observation instead of a sleep-and-hope.
type triggerClock struct {
	clock.Clock
	fire    chan time.Time
	waiting chan struct{}
}

func newTriggerClock() *triggerClock {
	return &triggerClock{
		Clock:   sim.NewFakeClock(time.Now()),
		fire:    make(chan time.Time, 1),
		waiting: make(chan struct{}, 8),
	}
}

func (c *triggerClock) After(time.Duration) <-chan time.Time {
	select {
	case c.waiting <- struct{}{}:
	default:
	}
	return c.fire
}

// expire fires the pending After() channel, which is the only way a
// waitPeerUpBarrier on this clock can reach its timeout branch.
func (c *triggerClock) expire() { c.fire <- time.Now() }

// newBarrierPeer returns a peer with the peer-up barrier freshly reset for a
// session, on a fake clock so the barrier's timeout can never fire.
//
// The fake clock is what makes these tests assertions rather than coincidences:
// sim.FakeClock.After never fires, so waitPeerUpBarrier can end for exactly one
// reason, the acknowledgements arriving. Without it a test would pass on the
// timeout and prove nothing.
func newBarrierPeer(t *testing.T) *Peer {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65000, 65001, 0x01020304)
	peer := NewPeer(settings)
	peer.SetClock(sim.NewFakeClock(time.Now()))
	peer.ResetPeerUpBarrier()
	return peer
}

// waitBarrier runs waitPeerUpBarrier on its own goroutine and reports the
// channel that closes when it returns, plus the result.
func waitBarrier(t *testing.T, p *Peer) (<-chan struct{}, *bool) {
	t.Helper()

	done := make(chan struct{})
	opened := new(bool)
	go func() {
		*opened = p.waitPeerUpBarrier()
		close(done)
	}()
	// Never let the waiter outlive the test when the barrier legitimately stays
	// shut: the fake clock has no timeout to release it.
	t.Cleanup(func() { p.SetPeerUpBarrier(0) })
	return done, opened
}

// barrierShut asserts the barrier has NOT opened, by reading its state rather
// than by waiting out a window. SignalPeerUpBarrier closes the channel under
// p.mu before it returns, so once it has returned the state is settled and this
// read is race-free -- and, unlike a fixed sleep, it cannot pass an
// implementation that opens the barrier slightly later.
func barrierShut(t *testing.T, p *Peer) {
	t.Helper()

	p.mu.RLock()
	ready := p.peerUpReady
	p.mu.RUnlock()
	require.False(t, chanClosed(ready)(), "barrier opened before every plugin acknowledged")
}

// VALIDATES: the initial-sync End-of-RIB waits until EVERY barrier plugin has
// acknowledged the peer-up event, not just the first.
// PREVENTS: releasing the marker after one acknowledgement when two plugins
// register the peer, which would put the End-of-RIB back ahead of a
// registration and re-open the window this barrier exists to close.
func TestPeerUpBarrierWaitsForEveryAcknowledgement(t *testing.T) {
	peer := newBarrierPeer(t)
	peer.SetPeerUpBarrier(2)

	done, opened := waitBarrier(t, peer)

	peer.SignalPeerUpBarrier()
	barrierShut(t, peer)
	require.False(t, chanClosed(done)(), "the waiter returned after 1 of 2 acknowledgements")

	peer.SignalPeerUpBarrier()
	require.Eventually(t, chanClosed(done), 2*time.Second, time.Millisecond,
		"barrier must open once every expected plugin has acknowledged")
	require.True(t, *opened, "barrier that opened on acknowledgements must report success")
}

// VALIDATES: a peer whose peer-up event reaches no barrier-declaring plugin
// does not wait at all.
// PREVENTS: the barrier adding latency to every session establishment. With
// nothing expected there is nothing to wait for, and a peer must not pay the
// timeout (or any part of it) for a plugin that was never going to signal.
func TestPeerUpBarrierNoExpectationDoesNotWait(t *testing.T) {
	peer := newBarrierPeer(t)

	done, opened := waitBarrier(t, peer)

	require.Eventually(t, chanClosed(done), time.Second, time.Millisecond,
		"a peer with no barrier plugin must not wait")
	require.True(t, *opened)
}

// VALIDATES: the barrier releases the End-of-RIB when a plugin never
// acknowledges, and reports that it did.
// PREVENTS: a wedged plugin holding a session's initial sync open forever. The
// bound is the whole safety story: correctness degrades to the pre-barrier
// behavior (marker out, registration unconfirmed) rather than to a hang.
//
// The timeout is fired by hand through triggerClock, so this asserts on the
// branch taken, never on elapsed time.
//
// The WARN the timeout branch emits is not asserted here: it goes to the
// bgp.routes subsystem logger (reactor.go:88 slogutil.LazyLogger), which is
// bound at first use and is not slog.Default, so the warnRecorder used by the
// reactor-level tests below cannot see it. The returned false IS the branch,
// and it is the part callers act on.
func TestPeerUpBarrierTimeoutReleases(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.2"), 65000, 65001, 0x01020304)
	peer := NewPeer(settings)
	tc := newTriggerClock()
	peer.SetClock(tc)
	peer.ResetPeerUpBarrier()
	peer.SetPeerUpBarrier(1) // nobody will acknowledge

	done := make(chan struct{})
	opened := new(bool)
	go func() {
		*opened = peer.waitPeerUpBarrier()
		close(done)
	}()

	tc.expire()

	require.Eventually(t, chanClosed(done), 2*time.Second, time.Millisecond,
		"the timeout must release the end-of-rib rather than wedge establishment")
	// test-relax: the Warn assertion this line replaces could never pass. The
	// timeout branch logs to the bgp.routes subsystem logger, not slog.Default,
	// so warnRecorder cannot observe it (see the doc comment above). Written, not
	// weakened: the assertion below is on the branch actually taken.
	require.False(t, *opened, "an unacknowledged barrier must report that it timed out")
}

// VALIDATES: the two readiness barriers are independent. A route-sending
// plugin's "plugin session ready" must not satisfy a registrar's obligation,
// and a registrar's acknowledgement must not satisfy a route sender's -- while
// each signal still opens its OWN barrier.
// PREVENTS: the fail-open a shared counter would create. With one counter,
// a config running both bgp-rib (bound, sends routes) and bgp-rs (registers the
// forward target) would let whichever signaled first cover the other, and the
// End-of-RIB could again precede registration -- the exact defect, restored in
// the configuration where it matters most (ai/rules/fail-closed-guards.md).
//
// Each half asserts BOTH directions: the wrong barrier stayed shut AND the
// right one opened. Absence alone would also pass if the signal were inert.
func TestPeerUpBarrierAndAPISyncAreIndependent(t *testing.T) {
	// A route sender signaling must leave the registrar barrier shut, and must
	// release its own.
	peer := newBarrierPeer(t)
	peer.ResetAPISync(1)
	peer.SetPeerUpBarrier(1)

	peer.SignalAPIReady()
	barrierShut(t, peer)

	apiDone := make(chan struct{})
	go func() {
		peer.waitForAPISync()
		close(apiDone)
	}()
	require.Eventually(t, chanClosed(apiDone), 2*time.Second, time.Millisecond,
		"SignalAPIReady must release the API sync it belongs to")

	// And the reverse: a registrar acknowledgement must not release the API
	// sync, but must open the peer-up barrier.
	peer2 := newBarrierPeer(t)
	peer2.ResetAPISync(1)
	peer2.SetPeerUpBarrier(1)

	api2 := make(chan struct{})
	go func() {
		peer2.waitForAPISync()
		close(api2)
	}()
	t.Cleanup(peer2.SignalAPIReady)

	peer2.SignalPeerUpBarrier()
	require.False(t, chanClosed(api2)(), "SignalPeerUpBarrier released waitForAPISync: shared counters")

	barrierDone, _ := waitBarrier(t, peer2)
	require.Eventually(t, chanClosed(barrierDone), 2*time.Second, time.Millisecond,
		"SignalPeerUpBarrier must open the peer-up barrier it belongs to")
}

// VALIDATES: ResetPeerUpBarrier gives each session its own barrier, so a
// previous session's acknowledgements cannot open the new one.
// PREVENTS: a rapid reconnect inheriting the old session's satisfied barrier
// and emitting its End-of-RIB before the plugins have re-registered the peer.
func TestPeerUpBarrierResetPerSession(t *testing.T) {
	peer := newBarrierPeer(t)
	peer.SetPeerUpBarrier(1)
	peer.SignalPeerUpBarrier()
	require.True(t, peer.waitPeerUpBarrier(), "first session's barrier opens")

	// New session: same peer, fresh barrier, no acknowledgement yet.
	peer.ResetPeerUpBarrier()
	peer.SetPeerUpBarrier(1)
	barrierShut(t, peer)

	done, _ := waitBarrier(t, peer)
	peer.SignalPeerUpBarrier()
	require.Eventually(t, chanClosed(done), 2*time.Second, time.Millisecond,
		"the new session's own acknowledgement must open its barrier")
}

// VALIDATES: an acknowledgement that arrives before the count is set does not
// open the barrier by itself, and is not lost either -- the later count settles
// it.
// PREVENTS: the zero-value trap. "Not armed yet" and "armed, expecting none"
// both read as expected==0, so without the armed flag a single early signal
// satisfies `count >= expected`, closes the channel and spends the sync.Once,
// leaving the session's barrier open for good with no way back
// (ai/rules/fail-closed-guards.md).
func TestPeerUpBarrierSignalBeforeArmDoesNotOpenIt(t *testing.T) {
	peer := newBarrierPeer(t)

	peer.SignalPeerUpBarrier()
	barrierShut(t, peer)

	// Two expected, one already in hand: still shut.
	peer.SetPeerUpBarrier(2)
	barrierShut(t, peer)

	// The early acknowledgement was counted, not discarded: one more opens it.
	done, _ := waitBarrier(t, peer)
	peer.SignalPeerUpBarrier()
	require.Eventually(t, chanClosed(done), 2*time.Second, time.Millisecond,
		"an acknowledgement received before arming must still count toward the total")
}

// VALIDATES: lowering the expected count opens the barrier at once.
// PREVENTS: a peer paying the full barrier timeout after the dispatcher has
// already established that no further acknowledgement can arrive (a refused or
// failed delivery). Waiting there cannot make the guarantee truer, it only
// delays the End-of-RIB.
func TestPeerUpBarrierLoweredCountReleasesImmediately(t *testing.T) {
	peer := newBarrierPeer(t)
	peer.SetPeerUpBarrier(2)

	done, opened := waitBarrier(t, peer)
	peer.SignalPeerUpBarrier()
	barrierShut(t, peer)

	// The dispatcher reports it only obtained one acknowledgement.
	peer.SetPeerUpBarrier(1)

	require.Eventually(t, chanClosed(done), 2*time.Second, time.Millisecond,
		"lowering the count to what was achieved must release the end-of-rib now")
	require.True(t, *opened)
}

// VALIDATES: the reactor routes a barrier SIGNAL to a peer configured on a
// non-default BGP port when the dispatcher names it by bare IP.
// PREVENTS: the miss SignalPeerAPIReady already had (parsePeerAddrToKey assumes
// port 179, PeerKey uses the real port), which here would leave the peer's
// End-of-RIB waiting out the barrier timeout.
//
// The barrier is armed DIRECTLY on the peer, not through r.SetPeerUpBarrier:
// arming through the routing under test would make the test vacuous, since a
// lookup miss would leave expected at 0 and the wait would return immediately.
func TestReactorPeerUpBarrierSignalRoutesToNonDefaultPort(t *testing.T) {
	r := New(&Config{})
	addr := netip.MustParseAddr("192.0.2.20")

	settings := NewPeerSettings(addr, 65000, 65001, 0x01020304)
	settings.Port = 1179
	require.NoError(t, r.AddPeer(settings))

	r.mu.RLock()
	peer := r.peers[settings.PeerKey()]
	r.mu.RUnlock()
	require.NotNil(t, peer)

	peer.SetClock(sim.NewFakeClock(time.Now()))
	peer.ResetPeerUpBarrier()
	peer.SetPeerUpBarrier(1)

	done, _ := waitBarrier(t, peer)
	barrierShut(t, peer)

	r.SignalPeerUpBarrier(addr.String())
	require.Eventually(t, chanClosed(done), 2*time.Second, time.Millisecond,
		"a peer on a non-default port must receive its barrier signal")
}

// VALIDATES: the reactor arms the barrier of a peer named by bare IP on a
// non-default port.
// PREVENTS: a lookup miss on the arming call, which is the worse of the two
// misses: the peer silently reverts to emitting its End-of-RIB with no barrier
// at all, rather than merely being delayed.
func TestReactorPeerUpBarrierSetRoutesToNonDefaultPort(t *testing.T) {
	r := New(&Config{})
	addr := netip.MustParseAddr("192.0.2.21")

	settings := NewPeerSettings(addr, 65000, 65001, 0x01020304)
	settings.Port = 1179
	require.NoError(t, r.AddPeer(settings))

	r.mu.RLock()
	peer := r.peers[settings.PeerKey()]
	r.mu.RUnlock()
	require.NotNil(t, peer)

	peer.SetClock(sim.NewFakeClock(time.Now()))
	peer.ResetPeerUpBarrier()

	r.SetPeerUpBarrier(addr.String(), 1)

	peer.mu.RLock()
	armed, expected := peer.peerUpArmed, peer.peerUpExpected
	peer.mu.RUnlock()
	require.True(t, armed, "the arming call must reach a peer on a non-default port")
	require.Equal(t, int32(1), expected)
}

// VALIDATES: a barrier call naming a peer the reactor does not hold reaches
// nobody AND is observable.
// PREVENTS: a stale or typo'd address silently reverting a peer to the
// unguarded behavior, leaving no evidence at all -- the miss the layer that
// knows about it is the only one able to report
// (ai/rules/fail-closed-guards.md, "or say something").
func TestReactorPeerUpBarrierUnknownPeerWarns(t *testing.T) {
	rec := &warnRecorder{}
	old := slog.Default()
	slog.SetDefault(slog.New(rec))
	t.Cleanup(func() { slog.SetDefault(old) })

	r := New(&Config{})

	r.SetPeerUpBarrier("198.51.100.98", 1)
	r.SignalPeerUpBarrier("198.51.100.97")

	warned := rec.warnedPeers()
	require.Contains(t, warned, "198.51.100.98", "an unroutable arming call must be logged at Warn")
	require.Contains(t, warned, "198.51.100.97", "an unroutable signal must be logged at Warn")
}
