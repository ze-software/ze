// VALIDATES: the startup convergence hold (`bgp update-delay`) withholds every
//            establishing peer's initial routing update until every EXPECTED
//            peer has finished its own initial routing update to this speaker,
//            until establish-wait finds at least one peer held, or until
//            max-delay expires, and never past max-delay.
// PREVENTS:  the churn a rebooting speaker causes today. It advertises the
//            instant each session reaches Established, before the RIB has
//            learned what its other neighbors will send it, so the neighbor
//            sees an initial route set and then a rapid add/withdraw sequence.
//
//            It also pins the four places a wrong set or a zero would answer for
//            a release condition (ai/rules/principles.md):
//            - a dynamic-group member is HELD but never COUNTED, so an inbound
//              dynamic session cannot satisfy a denominator measured over the
//              statically configured peers;
//            - a config with no static peers still gets a hold, rather than the
//              feature going silently inert because a count was zero;
//            - an establish-wait deadline that fires with nothing held releases
//              nothing;
//            - a peer that came up and dropped stops counting toward
//              convergence.
//
// METHOD:    every test drives the three production entry points --
//            Peer.startInitialRoutes (the FSM Established branch, peer_run.go),
//            Peer.updateDelayEndOfRIB (the inbound End-of-RIB decode,
//            reactor_notify.go) and Peer.updateDelayPeerDown (the FSM
//            leave-Established branch) -- and reads either the peer's
//            initial-sync gate or Reactor.UpdateDelayStatus, which is what
//            `show bgp update-delay` prints.
//
//            sendInitialRoutes on a peer with no negotiated capabilities clears
//            sendingInitialRoutes and returns, so the flag falling from 1 to 0
//            says the update RAN, and the flag staying at 1 says the hold kept
//            it. A peer that DOES carry negotiated capabilities is observed
//            through UpdateDelayStatus instead, because sendInitialRoutes would
//            then wait on plugin machinery this test does not build.
//
//            Both deadlines are clock.AfterFunc timers, so sim.FakeClock fires
//            them on Add() and no test sleeps on a wall clock.

package reactor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/test/sim"
)

// updateDelayFamilies is the pool updateDelayGRPeer negotiates from, in order.
// A test that sends a marker names the same family the peer agreed to carry.
var updateDelayFamilies = []family.Family{family.IPv4Unicast, family.IPv6Unicast, family.IPv4Multicast}

// updateDelayHolding and updateDelayReason read the hold through
// Reactor.UpdateDelayStatus, which is what `show bgp update-delay` reads and the
// only reader the product has. The hold used to carry two accessors for these
// two facts; both were unwired, and each was a second copy of an expression
// report() already computed (ai/rules/planning.md, unwired symbols).
func updateDelayHolding(r *Reactor) bool { return r.UpdateDelayStatus().Holding }

func updateDelayReason(r *Reactor) string { return r.UpdateDelayStatus().Reason }

func updateDelayClock() *sim.FakeClock {
	return sim.NewFakeClock(time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC))
}

// updateDelayTestPeer builds a peer attached to r and already published as
// Established, which is the state the FSM callback calls startInitialRoutes in.
// setState closes the initial-sync gate, so the flag is 1 before the call.
//
// It carries NO negotiated capabilities, which RFC 4724 Section 4.1 excludes
// from the End-of-RIB wait: a peer that advertised no Graceful Restart
// capability has promised no marker.
func updateDelayTestPeer(r *Reactor) *Peer {
	p := newTestPeer()
	p.reactor = r
	p.setState(PeerStateEstablished)
	return p
}

// updateDelayGRPeer is a peer that DID negotiate Graceful Restart over families
// address families, so the hold waits for that many End-of-RIB markers from it.
func updateDelayGRPeer(r *Reactor, families int) *Peer {
	p := newTestPeer()
	p.reactor = r
	negotiated := make(map[family.Family]bool, families)
	for i := range families {
		negotiated[updateDelayFamilies[i]] = true
	}
	p.negotiated.Store(&NegotiatedCapabilities{
		families:        negotiated,
		GracefulRestart: &capability.GracefulRestart{RestartTime: 120},
	})
	p.setState(PeerStateEstablished)
	return p
}

// updateDelayRestartingPeer negotiated Graceful Restart with the Restart State
// bit set: it is itself restarting, and RFC 4724 Section 4.1 excludes it from
// the wait by name.
func updateDelayRestartingPeer(r *Reactor) *Peer {
	p := updateDelayGRPeer(r, 1)
	p.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{family.IPv4Unicast: true},
		GracefulRestart: &capability.GracefulRestart{RestartTime: 120, RestartState: true},
	})
	return p
}

// initialUpdateRan reports whether this peer's initial routing update has run.
// sendInitialRoutes stores 0 in sendingInitialRoutes on every exit path, and the
// hold leaves the flag at the 1 setState wrote.
func initialUpdateRan(p *Peer) bool {
	return p.sendingInitialRoutes.Load() == 0
}

// requireInitialUpdateRan waits for the per-session goroutine to finish. The
// unheld path and the release path both spawn one, so the observation is
// necessarily asynchronous.
func requireInitialUpdateRan(t *testing.T, p *Peer, why string) {
	t.Helper()
	require.Eventually(t, func() bool { return initialUpdateRan(p) },
		2*time.Second, time.Millisecond, why)
}

// requireInitialUpdateHeld asserts the peer's initial routing update does NOT
// run. It watches a window rather than reading the flag once: the unheld path
// spawns a goroutine, so a single read taken immediately after the call can see
// the flag before that goroutine has scheduled and would pass against a gate
// that withholds nothing.
func requireInitialUpdateHeld(t *testing.T, p *Peer, why string) {
	t.Helper()
	require.Never(t, func() bool { return initialUpdateRan(p) },
		100*time.Millisecond, time.Millisecond, why)
}

// ── Release on End-of-RIB, not on Established (RFC 4724 Section 4.1) ─────────

// TestUpdateDelayWaitsForEndOfRIBNotEstablished is the feature's definition.
// Established is when a neighbor answers the phone; End-of-RIB is when it has
// finished talking. Releasing on the first is the churn the feature exists to
// remove.
func TestUpdateDelayWaitsForEndOfRIBNotEstablished(t *testing.T) {
	r := &Reactor{}
	clk := updateDelayClock()
	first := updateDelayGRPeer(r, 2)
	second := updateDelayGRPeer(r, 1)
	require.True(t, r.updateDelay.arm(context.Background(), clk,
		UpdateDelay{MaxDelay: time.Hour}, []*Peer{first, second}))

	first.startInitialRoutes()
	second.startInitialRoutes()

	require.True(t, updateDelayHolding(r),
		"both peers Established must NOT release the hold: neither has sent an End-of-RIB")
	require.Equal(t, 0, r.UpdateDelayStatus().PeersConverged)

	first.updateDelayEndOfRIB(family.IPv4Unicast)
	require.True(t, updateDelayHolding(r),
		"one of the first peer's two families must not settle it")

	// A SECOND marker for a family already struck off is not progress. RFC 4724
	// Section 4.1 defers per address family, so a counter would settle the peer
	// here with IPv6 unicast still silent.
	first.updateDelayEndOfRIB(family.IPv4Unicast)
	require.Equal(t, 0, r.UpdateDelayStatus().PeersConverged,
		"a duplicate marker for one family must not settle a peer that owes two")

	// Nor is a marker for a family this session never negotiated. IPv6
	// multicast is outside updateDelayFamilies, so this peer never agreed to
	// carry it.
	first.updateDelayEndOfRIB(family.IPv6Multicast)
	require.Equal(t, 0, r.UpdateDelayStatus().PeersConverged,
		"a marker for an un-negotiated family must settle nothing")

	first.updateDelayEndOfRIB(family.IPv6Unicast)
	require.True(t, updateDelayHolding(r),
		"one peer of two settled must not release the hold")
	require.Equal(t, 1, r.UpdateDelayStatus().PeersConverged)

	second.updateDelayEndOfRIB(family.IPv4Unicast)
	require.False(t, updateDelayHolding(r),
		"the last expected peer's End-of-RIB must release the hold")
	require.Equal(t, updateDelayConverged.String(), updateDelayReason(r))
}

// TestUpdateDelayExcludesAPeerThatAdvertisedNoGracefulRestart pins the RFC's own
// fallback. RFC 4724 Section 4.1 defers "until it either (a) receives the
// End-of-RIB marker from all its peers (excluding [...] the ones that do not
// advertise the graceful restart capability)". Waiting for a marker such a peer
// never promised would hold this speaker to max-delay against correct behavior.
func TestUpdateDelayExcludesAPeerThatAdvertisedNoGracefulRestart(t *testing.T) {
	r := &Reactor{}
	clk := updateDelayClock()
	plain := newTestPeer()
	plain.reactor = r
	require.True(t, r.updateDelay.arm(context.Background(), clk,
		UpdateDelay{MaxDelay: time.Hour}, []*Peer{plain}))

	plain.setState(PeerStateEstablished)
	plain.startInitialRoutes()

	require.False(t, updateDelayHolding(r),
		"a peer that promised no End-of-RIB must settle on Established, or the hold always runs to max-delay")
	require.Equal(t, updateDelayConverged.String(), updateDelayReason(r))
	requireInitialUpdateRan(t, plain, "the release must run the initial routing update it held")
}

// TestUpdateDelayExcludesARestartingPeer covers the other exclusion Section 4.1
// names: a peer whose capability carries the Restart State bit is deferring its
// own initial update, so its marker is not a thing to wait for.
func TestUpdateDelayExcludesARestartingPeer(t *testing.T) {
	r := &Reactor{}
	clk := updateDelayClock()
	restarting := updateDelayRestartingPeer(r)
	require.True(t, r.updateDelay.arm(context.Background(), clk,
		UpdateDelay{MaxDelay: time.Hour}, []*Peer{restarting}))

	restarting.startInitialRoutes()
	require.False(t, updateDelayHolding(r),
		"a peer with the Restart State bit set must be excluded from the End-of-RIB wait")
	require.Equal(t, updateDelayConverged.String(), updateDelayReason(r))
}

// ── The expected set is an identity set, not a count ─────────────────────────

// TestUpdateDelayDoesNotCountADynamicPeer is the defect this set exists to
// prevent. A dynamic-group member reaches the same FSM callback through
// createDynamicPeer, so it lands in the held set. It must never land in the
// expected one: a route server with one configured peer and one dynamic group
// would otherwise release as converged on its first inbound dynamic session,
// with the configured peer still down.
func TestUpdateDelayDoesNotCountADynamicPeer(t *testing.T) {
	r := &Reactor{}
	clk := updateDelayClock()
	configured := updateDelayGRPeer(r, 1)
	require.True(t, r.updateDelay.arm(context.Background(), clk,
		UpdateDelay{MaxDelay: time.Hour}, []*Peer{configured}))

	// The dynamic member was never given to arm.
	dynamic := updateDelayGRPeer(r, 1)
	dynamic.startInitialRoutes()
	dynamic.updateDelayEndOfRIB(family.IPv4Unicast)

	require.True(t, updateDelayHolding(r),
		"a dynamic peer's End-of-RIB must not satisfy a set measured over the configured peers")
	status := r.UpdateDelayStatus()
	require.Equal(t, 1, status.ExpectedPeers, "only the configured peer is expected")
	require.Equal(t, 1, status.PeersHeld, "the dynamic peer is HELD even though it is not counted")
	require.Equal(t, 0, status.PeersConverged)

	configured.startInitialRoutes()
	configured.updateDelayEndOfRIB(family.IPv4Unicast)
	require.False(t, updateDelayHolding(r), "the configured peer's End-of-RIB must release the hold")
}

// TestUpdateDelayArmsWithNoStaticPeers is the second half of the same defect.
// A config that declares only dynamic groups gives arm an empty expected set,
// and the hold must still exist: an enabled feature that becomes a no-op because
// a count happened to be zero is a guard failing open on precisely the
// deployment it was written for.
func TestUpdateDelayArmsWithNoStaticPeers(t *testing.T) {
	r := &Reactor{}
	clk := updateDelayClock()
	require.True(t, r.updateDelay.arm(context.Background(), clk,
		UpdateDelay{MaxDelay: 30 * time.Second}, nil),
		"an enabled hold with no static peers must still arm")
	require.True(t, updateDelayHolding(r))

	dynamic := updateDelayGRPeer(r, 1)
	dynamic.startInitialRoutes()
	dynamic.updateDelayEndOfRIB(family.IPv4Unicast)
	require.True(t, updateDelayHolding(r),
		"an empty expected set must never converge: every member of an empty set is settled")
	require.Equal(t, 1, r.UpdateDelayStatus().PeersHeld)

	clk.Add(30 * time.Second)
	require.False(t, updateDelayHolding(r), "max-delay must bound a hold that cannot converge")
	require.Equal(t, updateDelayMaxDelay.String(), updateDelayReason(r))
}

// ── Suppression, timers and the two guards ───────────────────────────────────

func TestUpdateDelaySuppressesOutbound(t *testing.T) {
	r := &Reactor{}
	clk := updateDelayClock()
	held := updateDelayTestPeer(r)
	other := updateDelayGRPeer(r, 1)
	require.True(t, r.updateDelay.arm(context.Background(), clk,
		UpdateDelay{MaxDelay: time.Hour}, []*Peer{held, other}))

	held.startInitialRoutes()

	require.True(t, held.shouldQueue(),
		"a held peer must still queue route operations, or a plugin route reaches the wire from under the hold")
	require.True(t, held.forwardOrderHold(),
		"a held peer must still park forwarded UPDATEs, or another peer's route reaches the wire from under the hold")
	require.True(t, held.pendingSync(),
		"a held peer must read as owing the wire its initial update, or a quiescer reports it settled while held")
	requireInitialUpdateHeld(t, held,
		"no End-of-RIB may be sent from under the hold: RFC 4724 Section 4.1 makes the marker a claim that the initial update is complete")
}

func TestUpdateDelayReleasesOnTimer(t *testing.T) {
	r := &Reactor{}
	clk := updateDelayClock()
	up := updateDelayTestPeer(r)
	never := updateDelayGRPeer(r, 1)
	require.True(t, r.updateDelay.arm(context.Background(), clk,
		UpdateDelay{MaxDelay: 30 * time.Second}, []*Peer{up, never}))

	// `up` settles on Established (no Graceful Restart), `never` never
	// establishes, so convergence is unreachable.
	up.startInitialRoutes()
	requireInitialUpdateHeld(t, up, "the peer must be held while the timer runs")
	require.True(t, updateDelayHolding(r))

	clk.Add(30 * time.Second)
	require.False(t, updateDelayHolding(r))
	require.Equal(t, updateDelayMaxDelay.String(), updateDelayReason(r),
		"a hold ended by its outer bound must name max-delay")
	requireInitialUpdateRan(t, up, "max-delay must release a hold that can never converge")
}

func TestUpdateDelayReleasesOnTimerWithNoPeerEstablished(t *testing.T) {
	r := &Reactor{}
	clk := updateDelayClock()
	never := updateDelayGRPeer(r, 1)
	require.True(t, r.updateDelay.arm(context.Background(), clk,
		UpdateDelay{MaxDelay: 30 * time.Second}, []*Peer{never}))

	clk.Add(30 * time.Second)
	require.False(t, updateDelayHolding(r),
		"max-delay must end a hold no peer ever joined, or the engine never advertises")
	require.Equal(t, updateDelayMaxDelay.String(), updateDelayReason(r))

	// A peer that establishes after the release takes the unheld path.
	late := updateDelayTestPeer(r)
	late.startInitialRoutes()
	requireInitialUpdateRan(t, late, "a peer establishing after the release must send its initial update at once")
}

func TestUpdateDelayEstablishWaitReleasesOnThePeersThatAreUp(t *testing.T) {
	r := &Reactor{}
	clk := updateDelayClock()
	up := updateDelayGRPeer(r, 1)
	never := updateDelayGRPeer(r, 1)
	require.True(t, r.updateDelay.arm(context.Background(), clk,
		UpdateDelay{MaxDelay: time.Hour, EstablishWait: 10 * time.Second}, []*Peer{up, never}))

	up.startInitialRoutes()
	require.True(t, updateDelayHolding(r))

	clk.Add(10 * time.Second)
	require.False(t, updateDelayHolding(r), "establish-wait must release the peers that did come up")
	require.Equal(t, updateDelayEstablishWait.String(), updateDelayReason(r),
		"a hold ended by the early deadline must name establish-wait, not max-delay")
}

// TestUpdateDelayEstablishWaitHoldsWhenNoPeerHeld pins the guard that stops
// establish-wait from becoming a plain shorter max-delay. The deadline fires
// against an empty held set, and an empty set is not a set of peers to
// advertise to.
func TestUpdateDelayEstablishWaitHoldsWhenNoPeerHeld(t *testing.T) {
	r := &Reactor{}
	clk := updateDelayClock()
	never := updateDelayGRPeer(r, 1)
	require.True(t, r.updateDelay.arm(context.Background(), clk,
		UpdateDelay{MaxDelay: 60 * time.Second, EstablishWait: 10 * time.Second}, []*Peer{never}))

	clk.Add(10 * time.Second)
	require.True(t, updateDelayHolding(r),
		"establish-wait must not end a hold no peer has joined: releasing on zero would advertise before the first neighbor answered")
	require.Equal(t, updateDelayNotReleased.String(), updateDelayReason(r))

	clk.Add(50 * time.Second)
	require.False(t, updateDelayHolding(r), "max-delay must still end the hold")
	require.Equal(t, updateDelayMaxDelay.String(), updateDelayReason(r),
		"the hold must be ended by its outer bound, not by the early deadline it declined")
}

// TestUpdateDelayPeerThatDropsStopsCounting pins the pruning. Without it the
// release condition reads "has been Established", and a peer that came up and
// dropped keeps counting toward convergence while its wire is gone.
func TestUpdateDelayPeerThatDropsStopsCounting(t *testing.T) {
	r := &Reactor{}
	clk := updateDelayClock()
	flapping := updateDelayGRPeer(r, 1)
	steady := updateDelayGRPeer(r, 1)
	require.True(t, r.updateDelay.arm(context.Background(), clk,
		UpdateDelay{MaxDelay: time.Hour}, []*Peer{flapping, steady}))

	flapping.startInitialRoutes()
	flapping.updateDelayEndOfRIB(family.IPv4Unicast)
	require.Equal(t, 1, r.UpdateDelayStatus().PeersConverged)

	flapping.updateDelayPeerDown()
	status := r.UpdateDelayStatus()
	require.Equal(t, 0, status.PeersConverged, "a peer that left Established must stop counting as converged")
	require.Equal(t, 0, status.PeersHeld, "and must stop being held")

	steady.startInitialRoutes()
	steady.updateDelayEndOfRIB(family.IPv4Unicast)
	require.True(t, updateDelayHolding(r),
		"the second peer alone must not converge a two-peer hold once the first has dropped")

	clk.Add(time.Hour)
	require.Equal(t, updateDelayMaxDelay.String(), updateDelayReason(r))
}

// ── The off switch, arming, idempotence and shutdown ─────────────────────────

// TestUpdateDelayNotArmedWithoutMaxDelay is AC-5: an absent leaf leaves every
// peer on the path it took before the feature existed.
func TestUpdateDelayNotArmedWithoutMaxDelay(t *testing.T) {
	r := &Reactor{}
	p := updateDelayTestPeer(r)
	require.False(t, r.updateDelay.arm(context.Background(), updateDelayClock(), UpdateDelay{}, []*Peer{p}),
		"a zero max-delay must arm nothing")
	require.False(t, updateDelayHolding(r))

	p.startInitialRoutes()
	requireInitialUpdateRan(t, p, "with no update-delay configured the initial routing update must be immediate")
}

// TestUpdateDelayArmedOnceOnly refuses a second arm, so a reload cannot restart
// a startup hold and withhold advertisement from a running speaker.
func TestUpdateDelayArmedOnceOnly(t *testing.T) {
	r := &Reactor{}
	clk := updateDelayClock()
	p := updateDelayGRPeer(r, 1)
	require.True(t, r.updateDelay.arm(context.Background(), clk, UpdateDelay{MaxDelay: time.Hour}, []*Peer{p}))
	require.False(t, r.updateDelay.arm(context.Background(), clk, UpdateDelay{MaxDelay: time.Hour}, []*Peer{p}),
		"the hold must arm once: a second arm would hold a speaker that is already advertising")
}

// TestUpdateDelayReleaseIsIdempotent covers the race the two deadlines and the
// convergence test can lose to each other: one initial routing update per peer
// must reach the wire, and the first reason recorded is the one that stands.
func TestUpdateDelayReleaseIsIdempotent(t *testing.T) {
	r := &Reactor{}
	clk := updateDelayClock()
	p := updateDelayTestPeer(r)
	require.True(t, r.updateDelay.arm(context.Background(), clk, UpdateDelay{MaxDelay: time.Hour}, []*Peer{p}))

	p.startInitialRoutes()
	requireInitialUpdateRan(t, p, "the single expected peer settling must end the hold")
	require.Equal(t, updateDelayConverged.String(), updateDelayReason(r))

	r.updateDelay.release(updateDelayMaxDelay)
	require.Equal(t, updateDelayConverged.String(), updateDelayReason(r),
		"a second release must not overwrite the reason the first one recorded")
}

// TestUpdateDelayStopsOnReactorShutdown proves the canceled context stops the
// hold from ever releasing, by driving the clock PAST max-delay after the
// cancel. Asserting the state alone would pass against a hold with no context
// check at all, because a parked deadline satisfies it either way.
func TestUpdateDelayStopsOnReactorShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &Reactor{}
	clk := updateDelayClock()
	p := updateDelayGRPeer(r, 1)
	require.True(t, r.updateDelay.arm(ctx, clk, UpdateDelay{MaxDelay: 30 * time.Second}, []*Peer{p}))
	p.startInitialRoutes()

	cancel()
	clk.Add(60 * time.Second)

	// A shutdown stops the WAIT. It does not advertise: a routing update put on
	// a wire that is closing is worse than one never sent.
	require.True(t, updateDelayHolding(r),
		"max-delay must not release a hold whose reactor context is done")
	require.Equal(t, updateDelayNotReleased.String(), updateDelayReason(r))
}

// TestUpdateDelayEnabled pins the single reading of the off switch.
func TestUpdateDelayEnabled(t *testing.T) {
	require.False(t, UpdateDelay{}.Enabled(), "the zero value must disable the hold")
	require.False(t, UpdateDelay{EstablishWait: time.Minute}.Enabled(),
		"establish-wait alone must not enable the hold: it can only end early a hold max-delay armed")
	require.True(t, UpdateDelay{MaxDelay: time.Second}.Enabled())
}

// TestUpdateDelayStatusUnconfigured is what `show bgp update-delay` prints on a
// daemon that never configured the feature. An operator reading it must be able
// to tell "not configured" from "held" without reading a log line the default
// WARN level suppresses.
func TestUpdateDelayStatusUnconfigured(t *testing.T) {
	r := &Reactor{config: &Config{}}
	status := r.UpdateDelayStatus()
	require.False(t, status.Configured)
	require.False(t, status.Holding)
	require.False(t, status.Released)
	require.Equal(t, "not-released", status.Reason)
	require.Equal(t, 0, status.ExpectedPeers)
}

// TestUpdateDelayStatusWhileHolding is the surface's reason to exist: a silent
// daemon that is holding says so, with the numbers that explain why.
func TestUpdateDelayStatusWhileHolding(t *testing.T) {
	r := &Reactor{config: &Config{UpdateDelay: UpdateDelay{MaxDelay: 30 * time.Second, EstablishWait: 10 * time.Second}}}
	clk := updateDelayClock()
	up := updateDelayGRPeer(r, 1)
	never := updateDelayGRPeer(r, 1)
	require.True(t, r.updateDelay.arm(context.Background(), clk, r.config.UpdateDelay, []*Peer{up, never}))
	up.startInitialRoutes()

	status := r.UpdateDelayStatus()
	require.True(t, status.Configured)
	require.True(t, status.Holding)
	require.False(t, status.Released)
	require.Equal(t, 30, status.MaxDelaySeconds)
	require.Equal(t, 10, status.EstablishWaitSeconds)
	require.Equal(t, 2, status.ExpectedPeers)
	require.Equal(t, 1, status.PeersHeld)
	require.Equal(t, 0, status.PeersConverged)
	require.Equal(t, "not-released", status.Reason)

	clk.Add(10 * time.Second)
	after := r.UpdateDelayStatus()
	require.False(t, after.Holding)
	require.True(t, after.Released)
	require.Equal(t, "establish-wait", after.Reason)
}

// TestUpdateDelayQueueOverrunIsReportedNotSwallowed is the hold's cost, made
// visible. A held peer has shouldQueue() true, so every announce lands in
// opQueue, and the hold stretches that window from milliseconds to as much as
// `max-delay 3600`. Past the cap the route reaches the peer NEVER, because the
// queue is the only path while the gate is closed.
//
// So the drop returns ErrOpQueueFull rather than logging and continuing. A
// caller that cannot tell a queued route from a dropped one reports success for
// a RIB the peer will never receive (ai/rules/principles.md), and
// announceBatchToPeers uses the error to stop counting the batch as accepted.
func TestUpdateDelayQueueOverrunIsReportedNotSwallowed(t *testing.T) {
	r := &Reactor{}
	clk := updateDelayClock()
	p := updateDelayGRPeer(r, 1)
	p.opQueueMax = 3
	require.True(t, r.updateDelay.arm(context.Background(), clk,
		UpdateDelay{MaxDelay: time.Hour}, []*Peer{p, updateDelayGRPeer(r, 1)}))

	p.startInitialRoutes()
	require.True(t, p.shouldQueue(), "a held peer must queue, or this test proves nothing about the hold")

	for i := range p.opQueueMax {
		require.NoError(t, p.QueueAnnounce(testRoute("10.0.0.0/24")),
			"announce %d must fit under the cap", i)
	}

	require.ErrorIs(t, p.QueueAnnounce(testRoute("10.0.1.0/24")), ErrOpQueueFull,
		"an announce past the cap must be REFUSED, not dropped in silence")
	require.ErrorIs(t, p.QueueWithdraw(testRoute("10.0.2.0/24").NLRI()), ErrOpQueueFull,
		"a withdrawal past the cap must be refused too: the peer keeps forwarding to a prefix this speaker took back")

	p.mu.RLock()
	queued := len(p.opQueue)
	p.mu.RUnlock()
	require.Equal(t, p.opQueueMax, queued, "the cap must hold: a refused operation must not be appended")
}

// TestUpdateDelayShutdownStopsTheConvergenceRelease covers the third release
// path, and it is the one an EXTERNAL event drives. The two deadlines are timers
// this hold owns; convergence is a peer's message, so an End-of-RIB arriving
// after the reactor's context is canceled would otherwise release the hold and
// spawn the initial routing update for every held peer, onto a wire that is
// closing.
func TestUpdateDelayShutdownStopsTheConvergenceRelease(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &Reactor{}
	p := updateDelayGRPeer(r, 1)
	require.True(t, r.updateDelay.arm(ctx, updateDelayClock(),
		UpdateDelay{MaxDelay: time.Hour}, []*Peer{p}))
	p.startInitialRoutes()

	cancel()
	p.updateDelayEndOfRIB(family.IPv4Unicast)

	require.True(t, updateDelayHolding(r),
		"a peer's End-of-RIB must not release a hold whose reactor context is done")
	require.Equal(t, updateDelayNotReleased.String(), updateDelayReason(r))
	requireInitialUpdateHeld(t, p, "and no initial routing update may reach a closing wire")
}

// TestUpdateDelayStatusAfterReleaseAgreesWithItself pins the record against
// itself. The counts and the reason are one answer, and reading the counts out
// of maps the release had already dropped printed reason=converged beside
// peers-converged=0.
func TestUpdateDelayStatusAfterReleaseAgreesWithItself(t *testing.T) {
	r := &Reactor{config: &Config{UpdateDelay: UpdateDelay{MaxDelay: time.Hour}}}
	first := updateDelayGRPeer(r, 1)
	second := updateDelayGRPeer(r, 1)
	require.True(t, r.updateDelay.arm(context.Background(), updateDelayClock(),
		r.config.UpdateDelay, []*Peer{first, second}))

	first.startInitialRoutes()
	second.startInitialRoutes()
	first.updateDelayEndOfRIB(family.IPv4Unicast)
	second.updateDelayEndOfRIB(family.IPv4Unicast)

	status := r.UpdateDelayStatus()
	require.Equal(t, "converged", status.Reason)
	require.Equal(t, 2, status.ExpectedPeers)
	require.Equal(t, 2, status.PeersConverged,
		"a hold that reports converged must report every expected peer converged")
	require.Equal(t, 2, status.PeersHeld,
		"and must report the peers whose initial update the release carried")
}

// TestUpdateDelayReleaseDropsItsPeerReferences pins the retention. The hold is a
// startup object that lives as long as the reactor, and every map it kept held a
// *Peer for every peer the daemon started with. A released hold reads none of
// them again.
func TestUpdateDelayReleaseDropsItsPeerReferences(t *testing.T) {
	r := &Reactor{config: &Config{UpdateDelay: UpdateDelay{MaxDelay: time.Hour}}}
	p := updateDelayGRPeer(r, 1)
	require.True(t, r.updateDelay.arm(context.Background(), updateDelayClock(),
		r.config.UpdateDelay, []*Peer{p}))
	p.startInitialRoutes()
	p.updateDelayEndOfRIB(family.IPv4Unicast)

	r.updateDelay.mu.Lock()
	defer r.updateDelay.mu.Unlock()
	require.Nil(t, r.updateDelay.expected, "the expected set must go at release")
	require.Nil(t, r.updateDelay.held, "the held set must go at release")
	require.Nil(t, r.updateDelay.tracked, "the tracked records must go at release")
}

// TestUpdateDelayReasonWords pins the WORD each release reason renders, as a
// literal.
//
// Every other reason assertion in this file compares updateDelayReason(r) to
// reason.String(), which is the same producer on both sides: a String() that
// collapsed every reason to one word would pass all of them. `show bgp
// update-delay`, its documentation and an operator's eye all read the word, so
// the word is the contract and one test states it.
//
// The population is the const block itself, so a reason added without a word
// here is a compile-time neighbor rather than a silent gap.
func TestUpdateDelayReasonWords(t *testing.T) {
	for _, tc := range []struct {
		reason updateDelayReleaseReason
		want   string
	}{
		{updateDelayNotReleased, "not-released"},
		{updateDelayConverged, "converged"},
		{updateDelayEstablishWait, "establish-wait"},
		{updateDelayMaxDelay, "max-delay"},
		{updateDelayReleaseReason(200), "unknown"},
	} {
		require.Equal(t, tc.want, tc.reason.String())
	}
}
