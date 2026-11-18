// Design: docs/architecture/l2tp/subscriber-session-model.md -- original physical cleanup roots

package l2tp

import (
	"context"
	"errors"
	"maps"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// retiredSessionCleanup observes only the removed original admission. Caller
// MUST hold tunnelsMu. A constructor must finish before its absent attempt can
// mean no PPP ownership; a queued attempt must confirm its own release.
func retiredSessionCleanup(session *L2TPSession) error {
	if session.pppConstruction != nil {
		if err := session.pppConstruction.cleanupStatus(); err != nil {
			return err
		}
	}
	if session.pppAttempt == (ppp.SessionAttempt{}) {
		return nil
	}
	status, err := session.pppAttempt.CleanupStatus()
	if err != nil {
		return err
	}
	if status.Err != nil {
		return status.Err
	}
	if !status.Finished {
		return ErrSessionCleanupPending
	}
	if status.LeaseAcquired {
		if !status.Released {
			return ErrSessionCleanupPending
		}
	}
	return nil
}

// retainRemovedSessions runs before publication and preserves roots even when
// their tunnel was already reaped. Only confirmed old receipts are pruned.
func (r *l2tpReactor) retainRemovedSessions(downs []sessionDown) {
	r.tunnelsMu.Lock()
	defer r.tunnelsMu.Unlock()
	r.pruneRetiredSessionsLocked()
	for index := range downs {
		down := &downs[index]
		if down.session == nil {
			continue
		}
		if r.retiredSessions == nil {
			r.retiredSessions = make(map[*L2TPSession]*L2TPTunnel)
		}
		r.retiredSessions[down.session] = down.tunnel
	}
}

// Caller holds tunnelsMu. Native status uses its worker lock, never ID lookup.
func (r *l2tpReactor) pruneRetiredSessionsLocked() {
	for session := range r.retiredSessions {
		if retiredSessionCleanup(session) != nil {
			continue
		}
		if r.kernelWorker != nil {
			if r.kernelWorker.originalCleanupStatus(session) != nil {
				continue
			}
		}
		delete(r.retiredSessions, session)
	}
}

// cleanupCapturedOriginal ends only resources retained by the original row.
// An in-flight PPP Close is joined through its exact existing worker. Context
// cancellation and unconfirmed native owners leave the original root intact.
func (r *l2tpReactor) cleanupCapturedOriginal(ctx context.Context, tunnel *L2TPTunnel, session *L2TPSession) error {
	var nativeErr error
	if r.kernelWorker != nil {
		nativeErr = r.kernelWorker.cleanupOriginal(ctx, tunnel.localTID, session)
	}
	var pppErr error
	r.tunnelsMu.Lock()
	attempt := session.pppAttempt
	r.tunnelsMu.Unlock()
	if control, admitted := attempt.Control(); admitted {
		pppErr = control.CloseCanceledOwner(ctx)
		waitRetirement := pppErr == nil || errors.Is(pppErr, ppp.ErrCleanupInProgress)
		if waitRetirement {
			status, waitErr := control.WaitRetired(ctx)
			pppErr = errors.Join(waitErr, status.Err)
			if !status.Released {
				pppErr = errors.Join(pppErr, ErrSessionCleanupPending)
			}
		}
	}
	// An already-running native teardown may have finished while the exact
	// PPP worker retired. Only confirmed original status can clear that busy
	// result; raw descriptor failures remain stored on the original receipt.
	if errors.Is(nativeErr, ErrSessionCleanupPending) {
		currentErr := r.kernelWorker.originalCleanupStatus(session)
		if currentErr == nil {
			nativeErr = nil
		} else {
			nativeErr = errors.Join(nativeErr, currentErr)
		}
	}
	// Constructor completion may have published the original attempt while
	// external cleanup ran. Re-read that same row, never a current SID lookup.
	r.tunnelsMu.Lock()
	retiredErr := retiredSessionCleanup(session)
	r.tunnelsMu.Unlock()
	return errors.Join(nativeErr, pppErr, retiredErr)
}

// retireSessions is the stopped reactor's final removal producer. No worker
// queue is needed: subsystem shutdown immediately sweeps every native root.
func (r *l2tpReactor) retireSessions() {
	r.tunnelsMu.Lock()
	var downs []sessionDown
	for _, tunnel := range r.tunnelsByLocalID {
		tunnel.clearSessions(l2tpevents.TerminateCauseAdminReset)
		_, removed := tunnel.drainPendingTeardowns()
		downs = append(downs, removed...)
	}
	r.tunnelsMu.Unlock()
	r.deliverSessionDowns(downs)
}

// Caller holds Subsystem.mu after its workers and PPP drivers stopped.
func (s *Subsystem) retainStoppedReactorsLocked() {
	for _, reactor := range s.reactors {
		reactor.tunnelsMu.Lock()
		reactor.pruneRetiredSessionsLocked()
		pending := len(reactor.retiredSessions) != 0
		reactor.tunnelsMu.Unlock()
		if reactor.kernelWorker != nil {
			if reactor.kernelWorker.pendingCleanup() != nil {
				pending = true
			}
		}
		if pending {
			s.retiredReactors = append(s.retiredReactors, reactor)
		}
	}
}

// Caller holds Subsystem.mu. Retry each original generation before acquiring
// a replacement listener/backend; an uncertain raw close continues to refuse.
func (s *Subsystem) cleanupRetiredReactorsLocked(ctx context.Context) error {
	hadRetired := len(s.retiredReactors) != 0
	previousObservers := s.retiredObserversLocked()
	kept := s.retiredReactors[:0]
	var errs []error
	for _, reactor := range s.retiredReactors {
		if reactor.kernelWorker != nil {
			reactor.kernelWorker.teardownAll()
		}
		reactor.tunnelsMu.Lock()
		originals := maps.Clone(reactor.retiredSessions)
		reactor.tunnelsMu.Unlock()
		for session, tunnel := range originals {
			if err := reactor.cleanupCapturedOriginal(ctx, tunnel, session); err != nil {
				errs = append(errs, err)
			}
		}
		reactor.tunnelsMu.Lock()
		reactor.pruneRetiredSessionsLocked()
		pending := len(reactor.retiredSessions) != 0
		reactor.tunnelsMu.Unlock()
		if reactor.kernelWorker != nil {
			if err := reactor.kernelWorker.pendingCleanup(); err != nil {
				pending = true
				errs = append(errs, err)
			}
		}
		if pending {
			kept = append(kept, reactor)
		}
	}
	clear(s.retiredReactors[len(kept):])
	s.retiredReactors = kept
	s.closeUnretainedObserversLocked(previousObservers)
	if len(kept) != 0 {
		return errors.Join(append(errs, ErrSessionCleanupPending)...)
	}
	if hadRetired {
		return s.closeKernelBackend()
	}
	return nil
}

// retireObserverLocked moves observation ownership to the already retained
// native roots. Caller MUST hold Subsystem.mu after joining dispatch; the last
// original root MUST close the observer, never its replacement.
func (s *Subsystem) retireObserverLocked() {
	observer := s.observer
	s.observer = nil
	if observer == nil {
		return
	}
	observer.RetainKnownSessions()
	for _, reactor := range s.retiredReactors {
		if reactor.observer == observer {
			return
		}
	}
	observer.Stop()
}

func (s *Subsystem) retiredObserversLocked() []*Observer {
	observers := make([]*Observer, 0, len(s.retiredReactors))
	seen := make(map[*Observer]bool, len(s.retiredReactors))
	for _, reactor := range s.retiredReactors {
		observer := reactor.observer
		if observer == nil || seen[observer] {
			continue
		}
		seen[observer] = true
		observers = append(observers, observer)
	}
	return observers
}

// The scans are bounded by retained listener roots; this is not another owner
// registry. Caller MUST hold Subsystem.mu; unreferenced originals MUST close once.
func (s *Subsystem) closeUnretainedObserversLocked(previous []*Observer) {
	retained := make(map[*Observer]bool, len(s.retiredReactors))
	for _, reactor := range s.retiredReactors {
		retained[reactor.observer] = true
	}
	for _, observer := range previous {
		if observer == s.observer || retained[observer] {
			continue
		}
		observer.Stop()
	}
}
