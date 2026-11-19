// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact original actor capture

package l2tp

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sync/atomic"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// SessionSelector is a cold selection DTO. SID is required; zero TID, absent
// Listener and zero Birth omit only their optional filters. It is not an owner.
type SessionSelector struct {
	SID      uint16
	TID      uint16
	Listener netip.AddrPort
	Birth    subscriberid.L2TPBirth
	// NetworkLifetime qualifies this capture to one existing network phase.
	// Zero preserves logical-session captures across LCP renegotiation.
	NetworkLifetime l2tpevents.NetworkLifetime
}

// ErrSessionAmbiguous refuses a readable selector matching distinct sessions.
var ErrSessionAmbiguous = errors.New("l2tp: session selector matches more than one logical session")

// ErrSessionCleanupPending refuses success before the original physical owner
// has confirmed release. Logical removal or queued deletion is not that proof.
var ErrSessionCleanupPending = errors.New("l2tp: original session native cleanup is not yet confirmed")

// CapturedSession is backed by an original private row capability. Commands
// may substitute their own Service implementation, but the production selector
// offers no public constructor or successful zero capability.
type CapturedSession interface {
	Birth() subscriberid.L2TPBirth
	Listener() netip.AddrPort
	Snapshot() (SessionSnapshot, error)
	FSMHistory() ([]FSMTransition, error)
	SessionEvents() ([]ObserverEvent, error)
	Counters() (ppp.SessionCounters, error)
	// TerminalCounters is valid only for an explicitly network-qualified
	// capture and MUST be awaited off the EventBus publication callback.
	TerminalCounters(context.Context) (l2tpevents.TrafficSample, error)
	RecordDisconnect(actor, reason string, cause uint32) error
	Teardown(context.Context, l2tpevents.TerminateCause, string) error
}

// capturedSession is an immutable reference to one admitted row. Its zero and
// nil values are invalid; only CaptureSession returns a successful capability.
// It borrows its original owners and never closes a replacement or current ID.
type capturedSession struct {
	reactor    *l2tpReactor
	tunnel     *L2TPTunnel
	session    *L2TPSession
	birth      subscriberid.L2TPBirth
	listener   netip.AddrPort
	observer   *Observer
	generation l2tpevents.ObservationGeneration
	// network is nonnil only when the selector explicitly qualified it.
	network         *sessionTimeoutLifetime
	teardownStarted atomic.Bool
}

// CaptureSession resolves the selector against one consistent set of original
// reactor rows. Subsystem order is also the lock order; duplicate roots are
// locked once. All loops are bounded by configured roots and their live rows.
// Safe for concurrent use. The caller owns no resource requiring release.
func (subsystem *Subsystem) CaptureSession(selector SessionSelector) (CapturedSession, error) {
	if subsystem == nil {
		return nil, ErrSubsystemNotStarted
	}
	if selector.SID == 0 {
		return nil, ErrInvalidID
	}
	subsystem.mu.Lock()
	reactors := make([]*l2tpReactor, 0, len(subsystem.reactors))
	seen := make(map[*l2tpReactor]bool, len(subsystem.reactors))
	for _, reactor := range subsystem.reactors {
		if reactor == nil {
			continue
		}
		if seen[reactor] {
			continue
		}
		seen[reactor] = true
		reactors = append(reactors, reactor)
	}
	for _, reactor := range subsystem.retiredReactors {
		if seen[reactor] {
			continue
		}
		seen[reactor] = true
		reactors = append(reactors, reactor)
	}
	for _, reactor := range reactors {
		reactor.tunnelsMu.Lock()
	}
	subsystem.mu.Unlock()
	defer func() {
		for _, reactor := range slices.Backward(reactors) {
			reactor.tunnelsMu.Unlock()
		}
	}()
	var selected *capturedSession
	for _, reactor := range reactors {
		reactor.pruneRetiredSessionsLocked()
		var endpoint netip.AddrPort
		if reactor.listener != nil {
			endpoint = reactor.listener.Addr()
		}
		if selector.Listener.IsValid() {
			if selector.Listener != endpoint {
				continue
			}
		}
		for _, tunnel := range reactor.tunnelsByLocalID {
			if selector.TID != 0 {
				if selector.TID != tunnel.localTID {
					continue
				}
			}
			session := tunnel.sessions[selector.SID]
			if session == nil {
				continue
			}
			birth := session.Birth()
			if !birth.IsValid() {
				continue
			}
			if selector.Birth.IsValid() {
				if selector.Birth != birth {
					continue
				}
			}
			var network *sessionTimeoutLifetime
			if selector.NetworkLifetime.IsValid() {
				network = session.timeoutLifetime
				if network == nil {
					continue
				}
				if network.network != selector.NetworkLifetime {
					continue
				}
			}
			if selected != nil {
				return nil, ErrSessionAmbiguous
			}
			selected = &capturedSession{reactor: reactor, tunnel: tunnel, session: session, birth: birth, listener: endpoint, observer: reactor.observer, generation: reactor.params.Generation, network: network}
		}
		for session, tunnel := range reactor.retiredSessions {
			if session.localSID != selector.SID {
				continue
			}
			if selector.TID != 0 {
				if selector.TID != tunnel.localTID {
					continue
				}
			}
			birth := session.Birth()
			if !birth.IsValid() {
				continue
			}
			if selector.Birth.IsValid() {
				if selector.Birth != birth {
					continue
				}
			}
			var network *sessionTimeoutLifetime
			if selector.NetworkLifetime.IsValid() {
				network = session.timeoutLifetime
				if network == nil {
					continue
				}
				if network.network != selector.NetworkLifetime {
					continue
				}
			}
			if selected != nil {
				return nil, ErrSessionAmbiguous
			}
			selected = &capturedSession{reactor: reactor, tunnel: tunnel, session: session, birth: birth, listener: endpoint, observer: reactor.observer, generation: reactor.params.Generation, network: network}
			selected.teardownStarted.Store(true)
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("%w: local-sid=%d", ErrSessionNotFound, selector.SID)
	}
	return selected, nil
}

// Birth returns the original immutable reference; nil has no reference.
func (captured *capturedSession) Birth() subscriberid.L2TPBirth {
	if captured == nil {
		return subscriberid.L2TPBirth{}
	}
	return captured.birth
}

// Listener returns the original bound endpoint, never reloaded configuration.
func (captured *capturedSession) Listener() netip.AddrPort {
	if captured == nil {
		return netip.AddrPort{}
	}
	return captured.listener
}

// currentLocked checks the exact original row. The caller MUST hold this
// capture's reactor.tunnelsMu; labels may locate but never authorize a row.
func (captured *capturedSession) currentLocked() bool {
	if captured == nil {
		return false
	}
	if captured.reactor == nil {
		return false
	}
	if captured.tunnel == nil {
		return false
	}
	if captured.session == nil {
		return false
	}
	if !captured.birth.IsValid() {
		return false
	}
	if captured.reactor.tunnelsByLocalID[captured.tunnel.localTID] != captured.tunnel {
		return false
	}
	if captured.tunnel.sessions[captured.session.localSID] != captured.session {
		return false
	}
	if captured.network != nil {
		if captured.session.timeoutLifetime != captured.network {
			return false
		}
	}
	return captured.session.Birth() == captured.birth
}

// Snapshot copies the original current row under its owner lock. A removed
// capture refuses even when another row now has identical readable labels.
func (captured *capturedSession) Snapshot() (SessionSnapshot, error) {
	if captured == nil {
		return SessionSnapshot{}, ErrSessionNotFound
	}
	if captured.reactor == nil {
		return SessionSnapshot{}, ErrSessionNotFound
	}
	captured.reactor.tunnelsMu.Lock()
	defer captured.reactor.tunnelsMu.Unlock()
	if !captured.currentLocked() {
		return SessionSnapshot{}, ErrSessionNotFound
	}
	return captured.reactor.sessionSnapshot(captured.tunnel.localTID, captured.session), nil
}

// FSMHistory copies only the captured row's bounded history ring.
func (captured *capturedSession) FSMHistory() ([]FSMTransition, error) {
	if captured == nil {
		return nil, ErrSessionNotFound
	}
	if captured.reactor == nil {
		return nil, ErrSessionNotFound
	}
	captured.reactor.tunnelsMu.Lock()
	defer captured.reactor.tunnelsMu.Unlock()
	if !captured.currentLocked() {
		return nil, ErrSessionNotFound
	}
	if captured.session.fsmHistory == nil {
		return nil, nil
	}
	return captured.session.fsmHistory.snapshot(), nil
}

// SessionEvents reads this captured original's observer without any SID relookup.
// The caller MUST retain its capture; the observer MUST never be replaced here.
func (captured *capturedSession) SessionEvents() ([]ObserverEvent, error) {
	if captured == nil {
		return nil, ErrSessionNotFound
	}
	if captured.reactor == nil {
		return nil, ErrSessionNotFound
	}
	captured.reactor.tunnelsMu.Lock()
	defer captured.reactor.tunnelsMu.Unlock()
	if !captured.currentLocked() {
		return nil, ErrSessionNotFound
	}
	if captured.observer == nil {
		return nil, nil
	}
	return captured.observer.SessionEvents(captured.birth), nil
}

// Counters borrows the original PPP control outside the actor lock. The exact
// attempt, control and network lifetime MUST survive the read before its result
// can be returned; no current interface name or ID is looked up afterwards.
func (captured *capturedSession) Counters() (ppp.SessionCounters, error) {
	if captured == nil {
		return ppp.SessionCounters{}, ErrSessionNotFound
	}
	if captured.reactor == nil {
		return ppp.SessionCounters{}, ErrSessionNotFound
	}
	reactor := captured.reactor
	reactor.tunnelsMu.Lock()
	if !captured.currentLocked() {
		reactor.tunnelsMu.Unlock()
		return ppp.SessionCounters{}, ErrSessionNotFound
	}
	attempt, network := captured.session.pppAttempt, captured.session.timeoutLifetime
	control, admitted := attempt.Control()
	reactor.tunnelsMu.Unlock()
	if !admitted {
		return ppp.SessionCounters{}, ErrSessionNotFound
	}
	if network == nil {
		return ppp.SessionCounters{}, ErrSessionNotFound
	}
	counters, err := control.Counters()
	if err != nil {
		return ppp.SessionCounters{}, err
	}
	reactor.tunnelsMu.Lock()
	defer reactor.tunnelsMu.Unlock()
	if !captured.currentLocked() {
		return ppp.SessionCounters{}, ErrSessionNotFound
	}
	if captured.session.pppAttempt != attempt {
		return ppp.SessionCounters{}, ErrSessionNotFound
	}
	if captured.session.timeoutLifetime != network {
		return ppp.SessionCounters{}, ErrSessionNotFound
	}
	original, found := captured.session.pppAttempt.Control()
	if !found {
		return ppp.SessionCounters{}, ErrSessionNotFound
	}
	if original != control {
		return ppp.SessionCounters{}, ErrSessionNotFound
	}
	if !control.Active() {
		return ppp.SessionCounters{}, ErrSessionNotFound
	}
	return counters, nil
}

// Teardown removes this exact row once and confirms its original physical
// receipts. A failed or canceled cleanup can be retried with this capture.
func (captured *capturedSession) Teardown(ctx context.Context, cause l2tpevents.TerminateCause, reason string) error {
	if ctx == nil {
		return errors.New("l2tp: captured teardown requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if captured == nil {
		return ErrSessionNotFound
	}
	if captured.reactor == nil {
		return ErrSessionNotFound
	}
	reactor := captured.reactor
	reactor.tunnelsMu.Lock()
	if captured.teardownStarted.Load() {
		reactor.tunnelsMu.Unlock()
		return reactor.cleanupCapturedOriginal(ctx, captured.tunnel, captured.session)
	}
	if !captured.currentLocked() {
		reactor.tunnelsMu.Unlock()
		return ErrSessionNotFound
	}
	outbound := captured.tunnel.teardownSession(captured.session, cdnResultAdministrative, cause, reactor.params.Clock(), reactor.logger)
	captured.teardownStarted.Store(true)
	teardowns, downs := captured.tunnel.drainPendingTeardowns()
	reactor.tunnelsMu.Unlock()
	reactor.deliverSessionDowns(downs)
	for _, request := range outbound {
		if err := reactor.listener.Send(request.to, request.bytes); err != nil {
			reportCapturedControlSendFailure(reactor.logger, request.to, err)
		}
	}
	if reason != "" {
		reactor.logger.Info("l2tp: captured session disconnect requested", "birth", captured.birth.String(), "reason", reason)
	}
	// Other original owners must progress before this command waits for its
	// captured PPP worker. Its own copy must not compete with exact cleanup.
	for index := range teardowns {
		if teardowns[index].session == captured.session {
			continue
		}
		reactor.enqueueKernelEvents(nil, teardowns[index:index+1])
	}
	// Synchronous cleanup also handles a stopped worker. Queue this original
	// only when its physical release remains unconfirmed.
	cleanupErr := reactor.cleanupCapturedOriginal(ctx, captured.tunnel, captured.session)
	if cleanupErr == nil {
		return nil
	}
	for index := range teardowns {
		if teardowns[index].session == captured.session {
			reactor.enqueueKernelEvents(nil, teardowns[index:index+1])
		}
	}
	return cleanupErr
}

// TerminalCounters uses only the original readonly receipt retained at capture.
// It remains valid after row removal, network replacement or physical Close;
// none of those states can redirect it to a new actor, handle or network.
// The caller MUST retain the capture until its accounting task completes.
func (captured *capturedSession) TerminalCounters(ctx context.Context) (l2tpevents.TrafficSample, error) {
	if captured == nil {
		return l2tpevents.TrafficSample{}, ErrSessionNotFound
	}
	if captured.network == nil {
		return l2tpevents.TrafficSample{}, errors.New("l2tp: terminal counters require a network-qualified capture")
	}
	return captured.network.counters.TerminalCounters(ctx)
}
