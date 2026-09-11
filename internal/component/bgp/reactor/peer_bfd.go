// Design: rfc/short/rfc5882.md -- BFD client contract
// RFC: rfc/drafts/draft-ietf-idr-bgp-bfd-strict-mode.txt -- Sections 4 and 7
// Overview: peer.go -- Peer struct and lifecycle
// Related: peer_run.go -- FSM callback that starts/stops the BFD client
// Related: session_bfd_strict.go -- the wire half of the strict-mode draft
//
// BFD client glue for a BGP peer. When the operator opts into BFD via
// `bgp peer connection bfd { ... }`, the peer opens one BFD session and
// subscribes to its state channel. The client:
//
//  1. Calls api.GetService() to reach the in-process BFD engine. If
//     nil (BFD plugin not loaded), a non-strict peer runs without BFD
//     and logs a warning -- the BGP session is not blocked.
//  2. Builds a SessionRequest from PeerSettings and calls
//     Service.EnsureSession.
//  3. Subscribes to state changes on the returned handle.
//  4. Runs a per-peer subscriber goroutine that turns each BFD state
//     change into one of FSM events 30 to 32 and delivers it to the
//     live session (session_bfd_strict.go, handleBFDEvent). A Down in
//     Established becomes a teardown carrying RFC 9384 Cease subcode
//     10 ("BFD Down"), which is what ze has always done; the other
//     states now have the answers draft-ietf-idr-bgp-bfd-strict-mode
//     Section 8 gives them.
//
// WHEN the session opens depends on strict mode, and this is the whole
// difference draft-ietf-idr-bgp-bfd-strict-mode Section 7 asks for:
//
//   - Non-strict: on entry to Established, released on exit. BFD is a
//     failure detector for a session that is already up (RFC 5882
//     Section 10.2), so there is nothing to detect before then.
//   - Strict: before the BGP FSM starts, and held until the peer stops.
//     "Implementations SHOULD start the BFD session associated with the
//     BGP BFD strict-mode session prior to the BGP FSM starting", and
//     "implementations SHOULD NOT immediately destroy BFD sessions when
//     associated BGP connections transition to Idle."
//
// Lifecycle: the subscriber goroutine is a per-session worker (not
// per-event) per rules/goroutine-lifecycle.md. It exits when either
// stopBFDClient closes the stop channel or the subscription channel
// closes (handle released or loop torn down). stopBFDClient waits on
// the done channel so the goroutine has exited by the time it
// returns.
package reactor

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
)

// bfdClient holds the per-peer BFD session state. Zero value is safe
// (unused until the peer opts into BFD). The handle fields are
// protected by mu because stopBFDClient may be called concurrently
// with the subscriber goroutine draining the subscription channel.
type bfdClient struct {
	mu     sync.Mutex
	svc    api.Service
	handle api.SessionHandle
	sub    <-chan api.StateChange
	stop   chan struct{}
	done   chan struct{}

	// starting says a call is between EnsureSession and the store below,
	// so a concurrent caller declines rather than opening a second
	// session whose subscriber nothing would ever stop.
	starting bool

	// state is the draft-ietf-idr-bgp-bfd-strict-mode Section 3
	// bfd.SessionState attribute, as an api.State. It is atomic rather
	// than under mu because the OPEN rail reads it on the session's
	// read goroutine while the subscriber writes it (Peer.bfdSessionState).
	state atomic.Int32

	// live says whether a BFD session is open at all. A closed session
	// and a session in Down state are different facts: the strict-mode
	// gate holds for both, but only the first means no forwarding-path
	// check is running (session_bfd_strict.go, bfdStrictHolds).
	live atomic.Bool

	// since is when the session entered the state `state` holds, as Unix
	// nanoseconds. It is what draft-ietf-idr-bgp-bfd-strict-mode Section
	// 10 measures -- "the BFD session has been Up for the desired amount
	// of time" -- and it is a property of the BFD session rather than of
	// any one BGP connection, which is why it lives here: Section 7
	// keeps the BFD session open across a teardown, so a link that has
	// been up for a minute owes no hold-down on the next attempt.
	since atomic.Int64
}

// bfdStrict reports whether this peer runs the strict-mode procedures of
// draft-ietf-idr-bgp-bfd-strict-mode. It answers the draft's BfdEnabled and
// BfdStrictEnabled session attributes (Section 3, items 16 and 17) together,
// because item 17 says "If BfdEnabled is not TRUE for this BGP session, this
// attribute has no impact".
//
// It says nothing about BfdStrictNegotiated: whether the PEER agreed is
// decided by the OPEN exchange and lives on the session's FSM.
func (p *Peer) bfdStrict() bool {
	cfg := p.settings.BFD
	return cfg != nil && cfg.Enabled && cfg.Strict
}

// bfdSessionState answers the draft's bfd.SessionState session attribute for
// this peer. The second return is false when no BFD session is open, which is
// the fact bfdStrictHolds needs and which no api.State can carry.
func (p *Peer) bfdSessionState() (api.State, time.Time, bool) {
	if !p.bfd.live.Load() {
		return api.StateAdminDown, time.Time{}, false
	}
	state := api.State(p.bfd.state.Load()) //nolint:gosec // api.State is a uint8 written by this file alone
	since := time.Time{}
	if nanos := p.bfd.since.Load(); nanos != 0 {
		since = time.Unix(0, nanos)
	}
	return state, since, true
}

// bfdSubState answers the draft-ietf-idr-bgp-bfd-strict-mode Section 8.1
// sub-state of this peer's live session, as the name Section 11 asks a display
// to show: "This draft introduces sub-states in the existing BGP finite state
// machine for tracking BFD session status inputs for strict mode operation.
// Implementations SHOULD provide visibility for these sub-states in its display
// of the BGP finite state machine."
//
// It answers the empty string where there is nothing to show, which is every
// peer that is not waiting for BFD, so a display renders the sub-state only
// where one exists rather than printing NONE against every peer in the table.
func (p *Peer) bfdSubState() string {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()
	if session == nil {
		return ""
	}
	sub := session.fsm.BfdSubState()
	if sub == fsm.SubStateNone {
		return ""
	}
	return sub.String()
}

// startBFDClient opens a BFD session for this peer when the peer has
// opted in via config and the BFD plugin is running in the same
// process. Idempotent: a second call while a session is open is a
// no-op, which is what lets the strict path open the session before
// the FSM starts and the FSM callback still call it on Established.
//
// No-op if:
//
//   - PeerSettings.BFD is nil (operator did not opt in)
//   - PeerSettings.BFD.Enabled is false (opt-in suspended)
//   - api.GetService returns nil (BFD plugin not loaded)
//   - EnsureSession returns an error (logged, peer runs without BFD)
//
// For a NON-strict peer each of these lets the BGP session continue
// normally; BFD is strictly additive to the BGP hold-timer detection.
// For a STRICT peer the last two are a hard failure of what the
// operator configured, so they are logged at error and the session
// gate holds the peer out of Established (session_bfd_strict.go).
func (p *Peer) startBFDClient() {
	cfg := p.settings.BFD
	if cfg == nil || !cfg.Enabled {
		return
	}

	// The check and the store below are ONE critical section, held by
	// starting. Splitting them let two concurrent callers -- the strict
	// early start in runOnce and the FSM callback on Established -- both
	// see no handle, both call EnsureSession, and the second store orphan
	// the first subscriber goroutine with no stop channel to close it.
	p.bfd.mu.Lock()
	if p.bfd.handle != nil || p.bfd.starting {
		p.bfd.mu.Unlock()
		return
	}
	p.bfd.starting = true
	p.bfd.mu.Unlock()

	started := false
	defer func() {
		if started {
			return
		}
		p.bfd.mu.Lock()
		p.bfd.starting = false
		p.bfd.mu.Unlock()
	}()

	svc := api.GetService()
	if svc == nil {
		if cfg.Strict {
			peerLogger().Error("bfd strict-mode configured but BFD plugin not loaded; peer is held down",
				"peer", p.settings.Address)
			return
		}
		peerLogger().Warn("bfd configured on peer but BFD plugin not loaded; peer runs without BFD",
			"peer", p.settings.Address)
		return
	}
	req := bfdRequestFor(p.settings)
	handle, err := svc.EnsureSession(req)
	if err != nil {
		if cfg.Strict {
			peerLogger().Error("bfd strict-mode EnsureSession failed; peer is held down",
				"peer", p.settings.Address, "err", err)
			return
		}
		peerLogger().Warn("bfd EnsureSession failed; peer runs without BFD",
			"peer", p.settings.Address, "err", err)
		return
	}
	sub := handle.Subscribe()
	stop := make(chan struct{})
	done := make(chan struct{})

	p.bfd.mu.Lock()
	p.bfd.svc = svc
	p.bfd.handle = handle
	p.bfd.sub = sub
	p.bfd.stop = stop
	p.bfd.done = done
	p.bfd.starting = false
	p.bfd.mu.Unlock()
	started = true

	// Seed from the engine's own snapshot rather than from an assumption.
	// api.SessionHandle.Subscribe delivers the session's current state as the
	// first value on the channel, marked Initial, so a session another client
	// already brought Up is read as Up here. EnsureSession on an existing key
	// only bumps a refcount, so without this a strict peer sharing a session
	// with a top-level `bfd { session ... }` entry read Down until the next
	// TRANSITION -- which for a stable link never comes -- and never
	// established.
	//
	// Read before live is published, so no reader can ever observe the zero
	// value of the atomic: api.StateAdminDown is 0, and bfdStrictHolds reads
	// AdminDown as "proceed" (ai/rules/principles.md).
	state, since := api.StateDown, p.clock.Now()
	select {
	case snapshot := <-sub:
		if snapshot.Initial {
			state = snapshot.State
			since = bfdChangeTime(snapshot, since)
			break
		}
		// Not a snapshot: a real transition arrived first, which means this
		// Service does not deliver one. Apply it and carry on rather than
		// dropping a state change on the floor.
		state = snapshot.State
	default:
		// No snapshot at all. A Service that predates the Initial contract
		// leaves the opening state unknown, and RFC 5880 Section 6.8.1's own
		// answer -- a session starts in Down -- is the safe one: a strict peer
		// waits rather than establishing on a check nobody performed.
		peerLogger().Debug("bfd service delivered no initial state; assuming down",
			"peer", p.settings.Address)
	}
	p.bfd.state.Store(int32(state))
	p.bfd.since.Store(since.UnixNano())
	p.bfd.live.Store(true)

	peerLogger().Info("bfd session opened for peer",
		"peer", p.settings.Address,
		"multi-hop", cfg.MultiHop,
		"strict", cfg.Strict,
		"profile", cfg.Profile)

	go p.runBFDSubscriber(handle, sub, stop, done)
}

// runBFDSubscriber is the per-peer subscriber worker. It drains the
// subscription channel until either stop is signaled or the channel
// closes (handle released), and turns each BFD state change into the
// FSM event draft-ietf-idr-bgp-bfd-strict-mode Section 4 names for it.
//
//	BFD Up        -> Event 32, BfdUp
//	BFD Down      -> Event 31, BfdDown
//	BFD AdminDown -> Event 30, BfdAdminDown
//	BFD Init      -> no event: the draft names none, because Init is a
//	                 handshake step rather than a verdict on the path.
//
// What each event DOES is the FSM's decision, state by state (fsm/fsm.go),
// and the NOTIFICATION each one owes is the session's (session_bfd_strict.go).
// This function decides nothing but the translation.
func (p *Peer) runBFDSubscriber(
	handle api.SessionHandle,
	sub <-chan api.StateChange,
	stop <-chan struct{},
	done chan<- struct{},
) {
	_ = handle // retained for future Shutdown/Enable integration
	defer close(done)
	for {
		select {
		case <-stop:
			return
		case change, ok := <-sub:
			if !ok {
				return
			}
			// The entry time moves with the state, because Section 10
			// measures how long the session has held the state it is in.
			//
			// From change.When, the ENGINE's stamp for this change, and not
			// from this peer's clock. The snapshot's When comes from the same
			// place, so both paths answer on one clock: mixing them is
			// invisible in production, where both are real, and makes the
			// hold-down remainder nonsense the moment a test injects one.
			if api.State(p.bfd.state.Swap(int32(change.State))) != change.State { //nolint:gosec // api.State is a uint8
				p.bfd.since.Store(bfdChangeTime(change, p.clock.Now()).UnixNano())
			}
			if change.Initial {
				// startBFDClient already consumed the snapshot in the normal
				// case; one arriving here means a Service delivered it late.
				// It is a STATE, not a transition, so it raises no FSM event.
				// The draft's events are transitions by definition -- Section
				// 4 defines Event 31 as "The BFD session ... has transitioned
				// to the Down state" -- so a snapshot raises none of them, and
				// that is what makes the suppression conformant rather than a
				// judgement about Down.
				//
				// Section 8.3.2 is the supporting reading, not the rule: its
				// normative arm ignores BfdDown in the Connect state, and its
				// explanation is that "A BFD session can transition to Down
				// from the Init state, indicating the session has failed to
				// come Up". Section 8.5.2 DOES close an OpenSent session on a
				// real BfdDown when strict is negotiated, which is why only
				// the snapshot is suppressed here.
				//
				// Recording it above is the whole job; the OPEN rail reads it.
				p.bfd.since.Store(bfdChangeTime(change, p.clock.Now()).UnixNano())
				continue
			}
			event, mapped := bfdEventFor(change.State)
			if !mapped {
				peerLogger().Debug("bfd state change with no FSM event",
					"peer", p.settings.Address,
					"bfd-state", change.State.String())
				continue
			}
			peerLogger().Debug("bfd state change",
				"peer", p.settings.Address,
				"bfd-state", change.State.String(),
				"bfd-diag", change.Diag.String(),
				"fsm-event", event.String())
			p.deliverBFDEvent(event)
		}
	}
}

// bfdChangeTime answers when a BFD change happened, preferring the ENGINE's own
// stamp so the snapshot path and the transition path agree on one clock.
//
// draft-ietf-idr-bgp-bfd-strict-mode Section 10 measures how long the session
// has been Up, and that subtraction is only meaningful between two readings of
// the same clock. fallback covers a Service that leaves When unset, which is
// the only case where this peer's own clock is the best available answer.
func bfdChangeTime(change api.StateChange, fallback time.Time) time.Time {
	if change.When.IsZero() {
		return fallback
	}
	return change.When
}

// bfdEventFor maps a BFD session state onto the FSM event
// draft-ietf-idr-bgp-bfd-strict-mode Section 4 defines for it. The second
// return is false for a state the draft gives no event, which is Init alone.
func bfdEventFor(state api.State) (fsm.Event, bool) {
	switch state {
	case api.StateUp:
		return fsm.EventBfdUp, true
	case api.StateDown:
		return fsm.EventBfdDown, true
	case api.StateAdminDown:
		return fsm.EventBfdAdminDown, true
	case api.StateInit:
		return 0, false
	}
	return 0, false
}

// deliverBFDEvent hands one FSM event to the peer's live session. A peer with
// no session right now has nowhere to deliver it, and that is normal for a
// strict peer: its BFD session outlives every BGP connection
// (draft-ietf-idr-bgp-bfd-strict-mode Section 7), so events arrive while the
// peer is between connection attempts. The state was already stored, and the
// next OPEN reads it (bfdStrictHolds).
func (p *Peer) deliverBFDEvent(event fsm.Event) {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()
	if session == nil {
		return
	}
	if err := session.handleBFDEvent(event); err != nil {
		peerLogger().Debug("bfd-driven FSM event failed",
			"peer", p.settings.Address, "event", event.String(), "err", err)
	}
}

// stopBFDClient releases the BFD handle, waits for the subscriber
// goroutine to exit, and clears the per-peer state so a subsequent
// startBFDClient is race-free. Idempotent: a no-op when no BFD session
// is currently open.
//
// Called from the FSM callback on exit from StateEstablished for a
// NON-strict peer, and from Peer.cleanup for a strict one:
// draft-ietf-idr-bgp-bfd-strict-mode Section 7 says implementations
// "SHOULD NOT immediately destroy BFD sessions when associated BGP
// connections transition to Idle", and a strict peer that tore its BFD
// session down on every failed attempt would restart the BFD handshake
// on each retry and never converge.
func (p *Peer) stopBFDClient() {
	p.bfd.mu.Lock()
	svc := p.bfd.svc
	handle := p.bfd.handle
	sub := p.bfd.sub
	stop := p.bfd.stop
	done := p.bfd.done
	p.bfd.svc = nil
	p.bfd.handle = nil
	p.bfd.sub = nil
	p.bfd.stop = nil
	p.bfd.done = nil
	p.bfd.mu.Unlock()

	if handle == nil {
		return
	}
	p.bfd.live.Store(false)
	if stop != nil {
		close(stop)
	}
	handle.Unsubscribe(sub)
	if svc != nil {
		if err := svc.ReleaseSession(handle); err != nil {
			peerLogger().Debug("bfd ReleaseSession failed",
				"peer", p.settings.Address, "err", err)
		}
	}
	if done != nil {
		<-done
	}
	peerLogger().Info("bfd session closed for peer", "peer", p.settings.Address)
}

// bfdRequestFor builds an api.SessionRequest from PeerSettings. The
// peer's Address and LocalAddress supply the session tuple; the BFD
// block supplies mode, min-TTL, and the optional egress interface.
// Timer fields are left zero so the BFD plugin uses its profile-driven
// defaults (the profile name is not carried in the SessionRequest
// because api.SessionRequest is timer-valued; profile resolution
// happens on the plugin side in a future pass).
func bfdRequestFor(s *PeerSettings) api.SessionRequest {
	mode := api.SingleHop
	if s.BFD != nil && s.BFD.MultiHop {
		mode = api.MultiHop
	}
	req := api.SessionRequest{
		Peer:  s.Address,
		Local: s.LocalAddress,
		Mode:  mode,
	}
	if s.BFD != nil {
		req.Interface = s.BFD.Interface
		req.MinTTL = s.BFD.MinTTL
	}
	return req
}
