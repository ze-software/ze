// Design: docs/architecture/l2tp/subscriber-session-model.md -- original physical cleanup roots

package l2tp

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"math"
	"strconv"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
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
//
// Bound: every root it adds is a removed admission whose slot its tunnel
// still holds (L2TPTunnel.retainedCleanups, taken by removeSession), so a
// tunnel's roots never exceed its session limit, and a discarded tunnel that
// still holds roots keeps a max-tunnels slot (retainingTunnels). The map
// therefore holds at most max-tunnels times max-sessions roots, each limit
// read as its 16-bit ID space when configured 0.
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

// pruneRetiredSessionsLocked drops each root whose PPP and native release is
// confirmed, and frees the admission slot it held on its tunnel. Its walk is
// bounded by the map's bound (retainRemovedSessions). Caller holds tunnelsMu.
// Native status uses its worker lock, never ID lookup.
func (r *l2tpReactor) pruneRetiredSessionsLocked() {
	for session, tunnel := range r.retiredSessions {
		if retiredSessionCleanup(session) != nil {
			continue
		}
		if r.kernelWorker != nil {
			if r.kernelWorker.originalCleanupStatus(session) != nil {
				continue
			}
		}
		delete(r.retiredSessions, session)
		r.releaseCleanupSlotLocked(tunnel)
	}
}

// bindTunnelAdmissionLocked gives a new tunnel its session limit and the
// prune its admission runs before refusing (releaseConfirmedCleanups). Every
// tunnel the reactor creates MUST pass through it. Caller holds tunnelsMu.
func (r *l2tpReactor) bindTunnelAdmissionLocked(tunnel *L2TPTunnel) {
	tunnel.maxSessions = r.params.MaxSessions
	tunnel.releaseConfirmedCleanups = r.pruneRetiredSessionsLocked
}

// releaseCleanupSlotLocked frees the admission slot of one removed session
// whose cleanup is now confirmed. The last slot of a live tunnel clears its
// held-slots warning; the last slot of a discarded tunnel frees the
// max-tunnels slot it held. Caller holds tunnelsMu.
func (r *l2tpReactor) releaseCleanupSlotLocked(tunnel *L2TPTunnel) {
	if tunnel.retainedCleanups <= 0 {
		panic("BUG: l2tp released a cleanup slot its tunnel does not hold")
	}
	tunnel.retainedCleanups--
	if tunnel.retainedCleanups != 0 {
		return
	}
	if !tunnel.discarded {
		clearCleanupHeldSessionSlots(tunnel.localTID)
		return
	}
	r.retainingTunnels--
	if r.retainingTunnels == 0 {
		clearCleanupHeldTunnelSlots()
	}
}

// markTunnelDiscardedLocked records that discardTunnelLocked removed the
// tunnel. A tunnel still holding removed sessions' slots keeps a max-tunnels
// slot until they are confirmed, so a LAC that churns tunnels cannot grow the
// retained roots past max-tunnels times max-sessions. No admission reaches a
// discarded tunnel, so its held-sessions warning ends here. Caller holds
// tunnelsMu and has already cleared the tunnel's sessions.
func (r *l2tpReactor) markTunnelDiscardedLocked(tunnel *L2TPTunnel) {
	tunnel.discarded = true
	clearCleanupHeldSessionSlots(tunnel.localTID)
	if tunnel.retainedCleanups != 0 {
		r.retainingTunnels++
	}
}

// tunnelSlotsFullLocked reports whether max-tunnels admits no new tunnel, and
// logs the refusal it causes, naming it with refusal ("SCCRQ rejected") and
// the peer attribute. Live tunnels and discarded tunnels whose removed
// sessions' cleanup is unconfirmed share the limit (0 reads as the 16-bit
// tunnel ID space); before it refuses for the latter it prunes, so a tunnel
// whose cleanup was confirmed frees its slot at once. Caller holds tunnelsMu.
func (r *l2tpReactor) tunnelSlotsFullLocked(refusal string, peer slog.Attr) bool {
	limit := int(r.params.MaxTunnels)
	if limit == 0 {
		limit = math.MaxUint16
	}
	if len(r.tunnelsByLocalID)+r.retainingTunnels < limit {
		return false
	}
	if r.retainingTunnels != 0 {
		r.pruneRetiredSessionsLocked()
	}
	if len(r.tunnelsByLocalID)+r.retainingTunnels < limit {
		return false
	}
	if r.retainingTunnels != 0 {
		reportCleanupHeldTunnelSlots(r.logger, refusal, peer, limit, len(r.tunnelsByLocalID), r.retainingTunnels)
		return true
	}
	reportTunnelLimit(r.logger, refusal, peer, limit)
	return true
}

// The refusal reports below carry the doctor-ppp-cleanup code under the PPP
// cleanup source, so the runtime ppp-resource-cleanup doctor check lists them
// beside the cleanup failures that hold the slots.
const (
	cleanupHeldSlotsWhy = "Removed L2TP sessions keep their admission slot until their PPP and kernel cleanup is confirmed, " +
		"so unconfirmed cleanup cannot pile up without limit"
	cleanupHeldSlotsFix           = ppp.CleanupFailureFix
	cleanupHeldTunnelSlotsSubject = "l2tp-admission/tunnel-slots"
)

func cleanupHeldSessionSlotsSubject(localTID uint16) string {
	return "l2tp-admission/session-slots/" + strconv.Itoa(int(localTID))
}

// reportCleanupHeldSessionSlots logs and raises a session admission refused
// because removed sessions with unconfirmed cleanup fill the tunnel's slots.
// Caller holds tunnelsMu; the report store takes no l2tp lock.
func reportCleanupHeldSessionSlots(logger *slog.Logger, refusal string, localTID uint16, limit, live, held int) {
	failure := opererr.New("L2TP "+refusal+" on tunnel "+strconv.Itoa(int(localTID))+": "+
		strconv.Itoa(held)+" of its "+strconv.Itoa(limit)+
		" session slots are held by removed sessions whose cleanup is unconfirmed",
		cleanupHeldSlotsWhy, cleanupHeldSlotsFix)
	logger.LogAttrs(context.Background(), slog.LevelWarn, failure.Problem(),
		failure.LogAttrs(slog.String("code", ppp.CodeDoctorPPPCleanup),
			slog.Uint64(reportFieldTunnelID, uint64(localTID)), slog.Int("max", limit),
			slog.Int("current", live), slog.Int("cleanup-pending", held))...)
	report.RaiseWarningFix(ppp.CleanupReportSource, ppp.CodeDoctorPPPCleanup,
		cleanupHeldSessionSlotsSubject(localTID),
		report.FitMessage(failure.Problem()+". "+failure.Why()), failure.Fix(),
		map[string]any{reportFieldTunnelID: localTID, "max": limit, "current": live, "cleanup-pending": held})
}

func clearCleanupHeldSessionSlots(localTID uint16) {
	report.ClearWarning(ppp.CleanupReportSource, ppp.CodeDoctorPPPCleanup,
		cleanupHeldSessionSlotsSubject(localTID))
}

// reportCleanupHeldTunnelSlots logs and raises a tunnel admission refused
// because discarded tunnels with unconfirmed session cleanup fill max-tunnels.
// Caller holds tunnelsMu; the report store takes no l2tp lock.
func reportCleanupHeldTunnelSlots(logger *slog.Logger, refusal string, peer slog.Attr, limit, live, held int) {
	failure := opererr.New("L2TP "+refusal+": "+strconv.Itoa(held)+" of the "+strconv.Itoa(limit)+
		" tunnel slots are held by closed tunnels whose session cleanup is unconfirmed",
		cleanupHeldSlotsWhy, cleanupHeldSlotsFix)
	logger.LogAttrs(context.Background(), slog.LevelWarn, failure.Problem(),
		failure.LogAttrs(slog.String("code", ppp.CodeDoctorPPPCleanup), peer,
			slog.Int("limit", limit), slog.Int("current", live), slog.Int("cleanup-pending", held))...)
	report.RaiseWarningFix(ppp.CleanupReportSource, ppp.CodeDoctorPPPCleanup,
		cleanupHeldTunnelSlotsSubject,
		report.FitMessage(failure.Problem()+". "+failure.Why()), failure.Fix(),
		map[string]any{"limit": limit, "current": live, "cleanup-pending": held})
}

func clearCleanupHeldTunnelSlots() {
	report.ClearWarning(ppp.CleanupReportSource, ppp.CodeDoctorPPPCleanup, cleanupHeldTunnelSlotsSubject)
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
