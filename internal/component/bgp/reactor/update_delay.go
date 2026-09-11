// Design: docs/architecture/behavior/peer-lifecycle.md — the startup convergence hold
// Detail: peer_initial_sync.go — sendInitialRoutes, the initial routing update this hold owns
// Related: peer_run.go — startInitialRoutes and updateDelayPeerDown, the two sites that drive it
// Related: reactor_notify.go — the inbound End-of-RIB site that settles an expected peer
// RFC: rfc/short/rfc4724.md — deferred route selection, its exclusions and its upper-bound timer

package reactor

import (
	"context"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
)

// UpdateDelay is the startup convergence hold an operator configures under
// `bgp update-delay`. It answers one question: how long does this speaker defer
// its initial routing update so a reboot advertises one settled set of routes
// rather than a churn of partial ones.
//
// MaxDelay of zero means the operator configured nothing, and the hold is never
// armed. That is the single off switch, and it is checked in one place
// (updateDelayHold.arm), so an absent leaf leaves every peer on the path it took
// before this type existed.
//
// RFC 4724 Section 4.1 describes the same behavior for a Restarting Speaker and
// requires the outer bound this type carries: "To put an upper bound on the
// amount of time a router defers its route selection, an implementation MUST
// support a (configurable) timer that imposes this upper bound." MaxDelay is
// that timer, offered for a cold start as well as a graceful one.
type UpdateDelay struct {
	// MaxDelay is the outer bound. When it expires the hold releases whatever
	// the peers are doing. Zero disables the feature.
	MaxDelay time.Duration

	// EstablishWait is how long to wait for the expected peers to finish their
	// initial routing update before releasing on the peers that DID come up.
	// Zero means the operator set no early deadline, and only MaxDelay and full
	// convergence can release the hold. It MUST NOT exceed MaxDelay;
	// ParseUpdateDelay (internal/component/bgp/config/update_delay.go) refuses a
	// config where it does, so no larger value reaches this struct.
	EstablishWait time.Duration
}

// Enabled reports whether the operator asked for a hold at all.
//
// It is the ONE reading of the off switch. A caller that compared MaxDelay to
// zero itself would be a second copy of the rule, and the two would disagree the
// day a second field can enable the feature.
func (u UpdateDelay) Enabled() bool {
	return u.MaxDelay > 0
}

// updateDelayReleaseReason says which of the three conditions ended the hold.
// It is a named type rather than a bool or a string so a reader of the log line,
// of `show bgp update-delay` and of a future gauge sees the same closed set the
// code branches on.
type updateDelayReleaseReason uint8

const (
	// updateDelayNotReleased is the zero value and is NOT a reason: it says the
	// hold has not ended. releaseLocked refuses it, so a caller that forgets to
	// name a reason panics rather than writing "released for no reason" into the
	// log (ai/rules/principles.md).
	updateDelayNotReleased updateDelayReleaseReason = iota
	// updateDelayConverged: every expected peer finished its initial routing
	// update to this speaker.
	updateDelayConverged
	// updateDelayEstablishWait: the establish-wait deadline expired with at
	// least one peer held.
	updateDelayEstablishWait
	// updateDelayMaxDelay: the outer bound expired.
	updateDelayMaxDelay
)

// String names the reason for a log line and for the operational report.
func (r updateDelayReleaseReason) String() string {
	switch r {
	case updateDelayConverged:
		return "converged"
	case updateDelayEstablishWait:
		return "establish-wait"
	case updateDelayMaxDelay:
		return "max-delay"
	case updateDelayNotReleased:
		return plugin.UpdateDelayReasonNotReleased
	}
	return "unknown"
}

// updateDelayPeer is what the hold remembers about ONE expected peer between its
// establishment and its convergence.
//
// owed is the SET of address families whose End-of-RIB marker this peer still
// has to send, taken once at establishment from the negotiated capabilities. A
// set and not a count: RFC 4724 Section 4.1 defers route selection "for an
// address family", and a count is satisfied by two markers for one family, or
// by a marker for a family that was never negotiated, either of which settles
// the peer while a family it agreed to carry has sent nothing.
//
// An empty owed set with settled false cannot happen: every path that empties it
// settles the peer in the same call, because a peer that owes no marker has
// nothing left to wait for (ai/rules/principles.md, a zero is never an answer).
type updateDelayPeer struct {
	owed    map[family.Family]struct{}
	settled bool
}

// updateDelayHold defers the initial routing update of every peer that reaches
// Established while it is armed, and runs all of them when it releases.
//
// It owns NO goroutine. Both deadlines are clock.AfterFunc timers, so the hold
// runs on the caller's goroutine and on the timer's, and a test drives it with
// a fake clock rather than a wall-clock sleep. Both timers are stopped by
// releaseLocked, and both callbacks refuse to act once the reactor's context is
// done, so neither can outlive the reactor.
//
// Safe for concurrent use. Every field below is read and written under mu.
//
// What the hold does NOT do is suppress anything on the outbound path. It
// withholds the spawn of Peer.sendInitialRoutes, and the three suppressions that
// buys are the ones establishment already has: Peer.shouldQueue queues a route
// operation, Peer.forwardOrderHold parks a forwarded UPDATE in the destination
// worker's overflow buffer, and the End-of-RIB marker is not sent because the
// initial routing update has not completed. RFC 4724 Section 4.1: "Once the
// initial update is complete for an address family (including the case that
// there is no routing update to send), the End-of-RIB marker MUST be sent." A
// marker sent from under the hold would claim a completion that has not
// happened.
type updateDelayHold struct {
	mu sync.Mutex

	// settings is what the operator configured. Written once by arm.
	settings UpdateDelay

	// ctx is the reactor's context. The two timer callbacks read it and refuse
	// to release once it is done: a shutdown must stop the WAIT, and must not
	// put a routing update on a wire that is closing.
	ctx context.Context

	// armed says the hold is running: it owns the initial routing update of
	// every peer that establishes until released goes true.
	armed bool

	// released says the hold has ended. A peer that establishes after this is
	// on the unheld path, and arm refuses to run twice.
	released bool

	// reason is why the hold ended. updateDelayNotReleased until release.
	reason updateDelayReleaseReason

	// expected is the IDENTITY of every peer whose convergence this hold waits
	// for: the peers Reactor.StartPeers had in hand when it armed the hold. It
	// is a set of identities rather than a count, because held is populated by
	// any peer that establishes, dynamic-group members included, and a count
	// would let an inbound dynamic session satisfy a denominator that was
	// measured over the statically configured peers alone. A dynamic member is
	// HELD like every other peer and is never COUNTED.
	//
	// It never grows. A peer a later config reload adds is not part of this
	// startup's convergence, and counting it would move the finish line after
	// the race started.
	expected map[*Peer]struct{}

	// held is the set of peers whose initial routing update this hold owns.
	// Pruned by peerDown, so it means "is Established and still held" rather
	// than "has been Established at some point".
	held map[*Peer]struct{}

	// tracked carries the convergence state of the expected peers that have
	// established. A peer leaves it when it leaves Established, so a session
	// that came up and dropped stops counting toward the release.
	tracked map[*Peer]*updateDelayPeer

	// maxDelay and establishWait are the two deadlines, stopped on release.
	maxDelay      clock.Timer
	establishWait clock.Timer

	// finalExpected, finalHeld and finalConverged are the three counts as they
	// stood at the release, frozen by releaseLocked because it drops every map
	// in the same call. `show bgp update-delay` reads them once released.
	finalExpected  int
	finalHeld      int
	finalConverged int
}

// arm starts the hold. It MUST be called before any peer can reach Established,
// which is why StartPeers calls it before it starts a single peer.
//
// peers is the expected set: the peers the reactor is about to start. An EMPTY
// set still arms the hold. That separation is deliberate and it is two jobs, not
// one: whether the feature runs is decided by the operator's config alone, and
// whether CONVERGENCE can end the hold is decided by the expected set
// (releaseIfConvergedLocked refuses an empty one). Folding them together made
// the feature silently inert on a config that declares only dynamic groups,
// which is the deployment it exists for, and it did so through a guard whose
// comment claimed it failed closed.
//
// Returns false and arms nothing when the operator configured no hold, or when
// the hold has already run.
func (h *updateDelayHold) arm(ctx context.Context, clk clock.Clock, settings UpdateDelay, peers []*Peer) bool {
	if !settings.Enabled() {
		return false
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.armed || h.released {
		return false
	}

	h.settings = settings
	h.ctx = ctx
	h.armed = true
	h.expected = make(map[*Peer]struct{}, len(peers))
	for _, p := range peers {
		h.expected[p] = struct{}{}
	}
	h.held = make(map[*Peer]struct{}, len(peers))
	h.tracked = make(map[*Peer]*updateDelayPeer, len(peers))

	reactorLogger().Info("update-delay armed: the initial routing update is held",
		"expected-peers", len(h.expected),
		"max-delay", settings.MaxDelay,
		"establish-wait", settings.EstablishWait)

	h.startDeadlinesLocked(clk)
	return true
}

// startDeadlinesLocked arms the two deadlines. The caller MUST hold h.mu.
//
// clock.AfterFunc rather than a goroutine selecting on two channels: the hold
// has nothing to do while it waits, so a parked goroutine would be a worker with
// no work, and a fake clock fires an AfterFunc while it leaves a NewTimer
// channel inert (internal/test/sim). The timers are stopped by releaseLocked.
func (h *updateDelayHold) startDeadlinesLocked(clk clock.Clock) {
	h.maxDelay = clk.AfterFunc(h.settings.MaxDelay, func() {
		h.release(updateDelayMaxDelay)
	})

	// A zero EstablishWait means the operator set no early deadline, so no timer
	// is armed for it at all.
	if h.settings.EstablishWait > 0 {
		h.establishWait = clk.AfterFunc(h.settings.EstablishWait, h.releaseIfAnyHeld)
	}
}

// holdInitialUpdate registers this peer's initial routing update with the hold,
// and reports whether the hold now owns it.
//
// A true answer is a promise: this hold WILL run p.sendInitialRoutes when it
// releases, and the caller MUST NOT run it. A false answer means the caller owns
// it as it always did.
//
// EVERY establishing peer is held, whether or not it is expected. The hold is
// about what this speaker advertises, and a dynamic-group member's session is
// exactly as premature as a configured peer's. Only the RELEASE condition
// distinguishes them.
//
// Registering and evaluating the release happen under one lock, so a peer can
// neither be counted after the release that would have carried it nor be left
// behind by a release that started between the two halves.
func (h *updateDelayHold) holdInitialUpdate(p *Peer) bool {
	h.mu.Lock()

	if !h.armed || h.released {
		h.mu.Unlock()
		return false
	}

	h.held[p] = struct{}{}
	if _, expected := h.expected[p]; expected {
		h.trackLocked(p)
	}

	peers := h.releaseIfConvergedLocked()
	h.mu.Unlock()
	runHeldInitialUpdates(peers)
	return true
}

// trackLocked starts counting one expected peer's convergence, and settles it at
// once when RFC 4724 excludes it from the wait. The caller MUST hold h.mu.
//
// RFC 4724 Section 4.1 names both exclusions, and this is the same sentence the
// deferral itself comes from: the speaker defers "until it either (a) receives
// the End-of-RIB marker from all its peers (excluding the ones with the "Restart
// State" bit set in the received capability and excluding the ones that do not
// advertise the graceful restart capability) or (b) the Selection_Deferral_Timer
// referred to below has expired."
//
// So a peer that advertised no Graceful Restart capability has undertaken
// nothing about a marker, and waiting for one it never promised would hold this
// speaker to max-delay against a peer that is behaving correctly. It is settled
// on Established. A peer whose capability carries the Restart State bit is
// itself restarting and its own initial update is deferred, so it is excluded
// for the same reason.
func (h *updateDelayHold) trackLocked(p *Peer) {
	record := &updateDelayPeer{}
	h.tracked[p] = record

	nc := p.negotiated.Load()
	if nc == nil || nc.GracefulRestart == nil || nc.GracefulRestart.RestartState {
		record.settled = true
		return
	}

	families := nc.Families()
	if len(families) == 0 {
		// A session with no negotiated family owes no marker for any of them,
		// so there is nothing this peer can still send. Settled here rather
		// than left with an empty set, so no later membership test reads that
		// emptiness as a wait that can never end.
		record.settled = true
		return
	}
	record.owed = make(map[family.Family]struct{}, len(families))
	for _, fam := range families {
		record.owed[fam] = struct{}{}
	}
}

// peerEndOfRIB records the End-of-RIB marker this peer sent FOR ONE FAMILY, and
// releases the hold when that was the last family an expected peer owed.
//
// fam is what the marker itself names (WireUpdate.IsEOR), not a count. RFC 4724
// Section 4.1 defers "for an address family", so a second marker for a family
// already struck off changes nothing, and a marker for a family this session
// never negotiated settles nothing.
//
// Called from the reactor's inbound decode (reactor_notify.go), beside the
// peer's own EOR counter, so the hold cannot disagree with those counters about
// what arrived. The plugin server decodes the marker a second time for its own
// `eor` event (server/events.go, onMessageReceived and onMessageBatchReceived);
// that decode feeds subscribers and never reaches this hold.
//
// A peer nothing is tracking (not expected, or not currently Established) is
// ignored: the marker is real, but no release condition is measured over it.
//
// ORDERING. trackLocked runs before this can, and one goroutine is what
// guarantees it. Both the KEEPALIVE that establishes the session and the UPDATE
// carrying a marker are read by the session's own read loop, in wire order
// (processMessage, session_read.go), which calls onMessageReceived
// SYNCHRONOUSLY and then hands the message to handleKeepalive or handleUpdate.
// The FSM's callback contract completes the Established callback before Event
// returns in the uncontended case (fsm.go, SetCallback), so startInitialRoutes
// and trackLocked have run before the loop reads the next message. A marker
// cannot precede the KEEPALIVE that establishes the session, so it cannot
// precede the tracking.
//
// The FSM names one exception: a transition racing another goroutine's
// transition may run its callback ordered-async on the draining goroutine. A
// marker arriving inside that window finds no record and is dropped, this peer
// never settles, and the hold runs to max-delay. That is the safe direction --
// a hold held too long, never one released early -- and max-delay is the bound
// that makes it safe. Recorded here rather than defended against, because
// remembering markers for peers nothing tracks costs a second map that every
// ordinary session would pay for.
func (h *updateDelayHold) peerEndOfRIB(p *Peer, fam family.Family) {
	h.mu.Lock()

	if !h.armed || h.released {
		h.mu.Unlock()
		return
	}

	record, tracked := h.tracked[p]
	if !tracked || record.settled {
		h.mu.Unlock()
		return
	}

	if _, owed := record.owed[fam]; !owed {
		// Either a duplicate for a family already struck off, or a family this
		// session never negotiated. Neither is progress toward convergence.
		h.mu.Unlock()
		return
	}
	delete(record.owed, fam)
	if len(record.owed) > 0 {
		h.mu.Unlock()
		return
	}
	record.settled = true

	peers := h.releaseIfConvergedLocked()
	h.mu.Unlock()
	runHeldInitialUpdates(peers)
}

// peerDown drops this peer out of the hold when it leaves Established.
//
// Without it the release condition would read "has been Established", and a peer
// that came up and dropped again would keep counting toward convergence while
// the wire it was counted for is gone. Its initial routing update is dropped
// too: sendInitialRoutes would refuse a torn-down peer anyway, and holding a
// dead peer in the release set makes the log line's peer count a lie.
func (h *updateDelayHold) peerDown(p *Peer) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.armed || h.released {
		return
	}
	delete(h.held, p)
	delete(h.tracked, p)
}

// release ends the hold for the named reason and runs every held peer's initial
// routing update. It is idempotent: a second call after the hold has ended finds
// released set and does nothing, so the two deadlines and the convergence test
// can race without producing two initial updates for one peer.
//
// A release requested after the reactor's context is done is refused. Shutdown
// stops the WAIT; it does not advertise.
func (h *updateDelayHold) release(reason updateDelayReleaseReason) {
	h.mu.Lock()
	if !h.armed || h.released || h.stoppingLocked() {
		h.mu.Unlock()
		return
	}
	peers := h.releaseLocked(reason)
	h.mu.Unlock()
	runHeldInitialUpdates(peers)
}

// releaseIfAnyHeld ends the hold when at least one peer is held, and does
// nothing when none is. It is the establish-wait deadline's callback.
//
// The guard is that deadline's whole meaning: the operator asked to wait for
// peers to come up, so a deadline that arrives with no peer up has nothing to
// release ON. Releasing on an empty set would turn establish-wait into a plain
// shorter max-delay and would advertise before the first neighbor answered,
// which is the churn the feature exists to remove.
func (h *updateDelayHold) releaseIfAnyHeld() {
	h.mu.Lock()
	if !h.armed || h.released || h.stoppingLocked() {
		h.mu.Unlock()
		return
	}
	if len(h.held) == 0 {
		reactorLogger().Info("update-delay establish-wait expired with no peer established: holding until max-delay",
			"max-delay", h.settings.MaxDelay)
		h.mu.Unlock()
		return
	}
	peers := h.releaseLocked(updateDelayEstablishWait)
	h.mu.Unlock()
	runHeldInitialUpdates(peers)
}

// releaseIfConvergedLocked ends the hold when every expected peer has finished
// its initial routing update to this speaker. The caller MUST hold h.mu and MUST
// run the returned peers after it drops the lock.
//
// An EMPTY expected set never converges. "Every expected peer is settled" is
// true of an empty set, so a hold armed on a config with no statically
// configured peers would release the instant its first dynamic session
// established and report that it converged. The emptiness is refused HERE,
// where the arithmetic is, rather than at arm time, so a config that declares
// only dynamic groups still gets a hold, bounded by its two timers
// (ai/rules/principles.md).
//
// It carries the shutdown guard too, and that is not decoration: this is the one
// release path an EXTERNAL event drives. An End-of-RIB arriving after the
// reactor's context is canceled would otherwise release the hold and spawn the
// initial routing update for every held peer, onto a wire that is closing, which
// is exactly what this type's own comment says a shutdown must not do. The other
// two paths are timers this hold owns; this one is a peer's message.
//
// Returns the peers to run, nil when nothing was released. It returned a second
// bool nobody read.
func (h *updateDelayHold) releaseIfConvergedLocked() []*Peer {
	if h.stoppingLocked() {
		return nil
	}
	if len(h.expected) == 0 {
		return nil
	}
	settled := 0
	for _, record := range h.tracked {
		if record.settled {
			settled++
		}
	}
	if settled < len(h.expected) {
		return nil
	}
	return h.releaseLocked(updateDelayConverged)
}

// releaseLocked marks the hold ended, stops both deadlines, and returns the
// peers whose initial routing update it owed. The caller MUST hold h.mu and MUST
// run the returned peers after it drops the lock.
//
// The set is handed back rather than used here because sendInitialRoutes takes
// p.mu and p.staticMu and can block on the wire; holding h.mu across it would
// stall every other peer's Established transition behind one slow socket.
func (h *updateDelayHold) releaseLocked(reason updateDelayReleaseReason) []*Peer {
	if reason == updateDelayNotReleased {
		panic("BUG: update-delay released with no reason")
	}

	h.released = true
	h.reason = reason
	if h.maxDelay != nil {
		h.maxDelay.Stop()
	}
	if h.establishWait != nil {
		h.establishWait.Stop()
	}

	peers := make([]*Peer, 0, len(h.held))
	for p := range h.held {
		peers = append(peers, p)
	}

	// The counts are FROZEN into the final report before the maps go, so
	// `show bgp update-delay` on a released hold answers the numbers that were
	// true when it released. Reading them from the maps afterwards printed
	// `reason=converged` beside `peers-converged=0`, two fields of one record
	// contradicting each other.
	h.finalExpected = len(h.expected)
	h.finalHeld = len(peers)
	h.finalConverged = h.settledCountLocked()

	reactorLogger().Info("update-delay released: the initial routing update runs now",
		"reason", reason.String(),
		"peers", h.finalHeld,
		"expected", h.finalExpected)

	// Every map goes. Each holds a *Peer for every peer this startup began with,
	// and a hold that has released will never read one again; keeping them
	// pinned every peer the daemon started for the reactor's whole lifetime.
	h.held = nil
	h.tracked = nil
	h.expected = nil

	return peers
}

// settledCountLocked is how many expected peers have finished their initial
// routing update. The caller MUST hold h.mu.
func (h *updateDelayHold) settledCountLocked() int {
	settled := 0
	for _, record := range h.tracked {
		if record.settled {
			settled++
		}
	}
	return settled
}

// stoppingLocked reports whether the reactor's context is done. The caller MUST
// hold h.mu.
func (h *updateDelayHold) stoppingLocked() bool {
	return h.ctx != nil && h.ctx.Err() != nil
}

// runHeldInitialUpdates starts the initial routing update of every peer the hold
// owed, one per-session lifecycle goroutine each, exactly as the unheld path
// starts it (Peer.startInitialRoutes).
//
// A peer whose session died while it was held is not filtered out here.
// sendInitialRoutes refuses it: the CAS on sendingInitialRoutes fails once
// cleanup has cleared the flag, and the negotiated-capabilities read refuses a
// peer with no session. One refusal, in the function that owns the fact, rather
// than a second liveness test here that would disagree with it.
func runHeldInitialUpdates(peers []*Peer) {
	for _, p := range peers {
		go p.sendInitialRoutes() //nolint:goroutine-lifecycle // per-session lifecycle, not per-event
	}
}

// UpdateDelayStatus reports the startup convergence hold for `show bgp
// update-delay` (internal/component/bgp/plugins/cmd/peer). It is the
// plugin.ReactorIntrospector half of the feature.
//
// A nil config answers "not configured" rather than panicking. Reactor.New
// always sets one, so this arm is unreachable from the daemon; it is here
// because the caller is a CLI handler, and an operator asking a diagnostic
// question must never be the thing that takes the process down.
func (r *Reactor) UpdateDelayStatus() plugin.UpdateDelayStatus {
	var settings UpdateDelay
	if r.config != nil {
		settings = r.config.UpdateDelay
	}
	return r.updateDelay.report(settings)
}

// report renders the hold's state. settings is passed in rather than read from
// h, because an unarmed hold has none: a daemon that has parsed the leaves and
// not yet started its peers must still print what it will do.
func (h *updateDelayHold) report(settings UpdateDelay) plugin.UpdateDelayStatus {
	h.mu.Lock()
	defer h.mu.Unlock()

	out := plugin.UpdateDelayStatus{
		Configured:           settings.Enabled(),
		MaxDelaySeconds:      int(settings.MaxDelay / time.Second),
		EstablishWaitSeconds: int(settings.EstablishWait / time.Second),
		Holding:              h.armed && !h.released,
		Released:             h.released,
		Reason:               h.reason.String(),
	}
	if h.released {
		// The maps are gone. releaseLocked froze the three counts as they stood
		// at the release, which is the only moment they describe.
		out.ExpectedPeers = h.finalExpected
		out.PeersHeld = h.finalHeld
		out.PeersConverged = h.finalConverged
		return out
	}
	out.ExpectedPeers = len(h.expected)
	out.PeersHeld = len(h.held)
	out.PeersConverged = h.settledCountLocked()
	return out
}

// startInitialRoutes starts this peer's initial routing update, or hands it to
// the startup convergence hold.
//
// It is the ONE door onto sendInitialRoutes for an establishing peer. The FSM
// callback calls it in place of the bare spawn it used to make (peer_run.go), so
// the hold cannot be bypassed by a second establishment path: a path that
// spawned the goroutine itself would advertise from under a hold that believes
// it owns the peer.
//
// A reactor-less peer (a unit test that builds a Peer with no reactor) takes the
// unheld path, because there is no hold to ask. p.reactor is read WITHOUT p.mu,
// like every other reader of it (peer_run.go, wakeForwardOverflow).
func (p *Peer) startInitialRoutes() {
	r := p.reactor
	if r != nil && r.updateDelay.holdInitialUpdate(p) {
		return
	}
	go p.sendInitialRoutes() //nolint:goroutine-lifecycle // per-session lifecycle, not per-event
}

// updateDelayPeerDown tells the startup convergence hold this peer has left
// Established. A no-op when no hold is armed, and on a reactor-less peer.
func (p *Peer) updateDelayPeerDown() {
	r := p.reactor
	if r == nil {
		return
	}
	r.updateDelay.peerDown(p)
}

// updateDelayEndOfRIB tells the startup convergence hold this peer sent the
// End-of-RIB marker for fam. A no-op when no hold is armed.
//
// The FAMILY travels, not a count. RFC 4724 Section 4.1 defers "for an address
// family", so the hold strikes that family off the set this peer owes; a second
// marker for it, or one for a family the session never negotiated, changes
// nothing.
func (p *Peer) updateDelayEndOfRIB(fam family.Family) {
	r := p.reactor
	if r == nil {
		return
	}
	r.updateDelay.peerEndOfRIB(p, fam)
}
