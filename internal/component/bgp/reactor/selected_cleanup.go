// Design: docs/architecture/update-building.md -- engine-owned selected retirement.
// Related: selected_flush.go -- exact current-slot withdrawals through the writer.
// RFC 4271 Section 9.2 -- see rfc/short/rfc4271.md.
package reactor

import (
	"context"
	"errors"
	"math"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
)

// selectedCleanupPermit is engine-owned removal authority after revocation.
// Its zero value and nil pointer are invalid. Only retireSelectedOwner creates
// one for a captured destination session; it grants no announcement authority.
// source nil retires the exact deciding incarnation's whole current frontier.
type selectedCleanupPermit struct {
	session *Session
	source  *Peer
}

// RetireSelectedOwner is the process-generation cleanup seam. The server that
// owns the ended process supplies its own reactor, never a global replacement.
// A detached reactor has already stopped and joined all of its sessions.
func (a *reactorAPIAdapter) RetireSelectedOwner(ctx context.Context, owner plugin.SelectedOwner) error {
	return a.r.retireSelectedOwner(ctx, owner, nil)
}

// retireSelectedOwner runs outside reactor/peer locks after source authority
// is revoked and its admitted writes join. It snapshots only actual current
// destinations; no route or process history is retained by cleanup.
func (r *Reactor) retireSelectedOwner(ctx context.Context, owner plugin.SelectedOwner, source *Peer) error {
	r.mu.RLock()
	peers := make([]*Peer, 0, len(r.peers))
	for _, peer := range r.peers {
		peers = append(peers, peer)
	}
	r.mu.RUnlock()
	var failures []error
	for _, peer := range peers {
		session := peer.currentSession()
		if session == nil {
			continue
		}
		if !peer.adjOut.hasSelectedOwner(session, owner, source) {
			if source != nil {
				continue
			}
			if !session.hasSelectedRefreshOwner(owner) {
				continue
			}
		}
		if err := session.retireSelectedOwner(ctx, peer, owner, source); err != nil {
			// Cleanup cannot hand an unretired frontier to the successor.
			// End this captured session using the existing close/join boundary.
			session.seal()
			session.closeConnWithin(0)
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (table *adjRIBOut) hasSelectedOwner(session *Session, owner plugin.SelectedOwner, source *Peer) bool {
	table.mu.Lock()
	defer table.mu.Unlock()
	if table.session != session {
		return false
	}
	for _, entry := range table.routes {
		if !owner.SameIncarnation(entry.selection.owner) {
			continue
		}
		if source != nil {
			if entry.owner.source != source {
				continue
			}
		}
		return true
	}
	return false
}

func (s *Session) retireSelectedOwner(ctx context.Context, peer *Peer, owner plugin.SelectedOwner, source *Peer) error {
	var overdue *selectedRefreshPending
	lockErr := s.selectedMu.LockContext(ctx)
	if lockErr != nil {
		if source != nil {
			return lockErr
		}
		overdue = s.pendingSelectedRefreshOwner(owner)
		if overdue == nil {
			return lockErr
		}
		// A previous destination may have spent this shared deadline.
		// Full-owner pending cleanup is terminal: stop new output and
		// still join the actual selected stream before reporting overdue.
		s.seal()
		s.selectedMu.Lock()
	}
	defer s.selectedMu.Unlock()
	if source == nil {
		pending := s.pendingSelectedRefreshOwner(owner)
		if pending == nil {
			pending = overdue
		}
		if pending != nil {
			// The completed Begin belongs to this captured Session even if
			// its replay has no routes. selectedMu keeps the exact owner
			// obligation stable. Normal close may bound its wait for an
			// existing claimant, so then join the retained exact claim.done.
			// The ended SDK cannot authorize End or a replacement Session.
			s.seal()
			s.closeConnWithin(0)
			closeErr := pending.joinSelectedRefreshClose(ctx)
			writerErr := s.joinSelectedRefreshWriter(ctx)
			s.selectedRefresh.Store(nil)
			return errors.Join(lockErr, closeErr, writerErr)
		}
	}
	permit := selectedCleanupPermit{session: s, source: source}
	work := selectedWrite{output: selectedOutput{owner: owner, revision: math.MaxUint64},
		session: s, destinationSession: s.initialReplay.Load(), absent: true, cleanup: &permit}
	item := fwdItem{peer: peer, session: s, authority: adjOutSelected, selected: &work}
	return s.sendUpdateCounted(ctx, &message.Update{}, nil, false, &item,
		func(session *Session, _ *message.Update) (result error) {
			defer func() {
				if result != nil {
					session.retireWrite(result)
				}
				session.sentSourcePeerStr, session.sentSourceMessageID = "", 0
			}()
			if err := work.current(session); err != nil {
				return err
			}
			// The selected-stream lock and writeMu prevent additions during
			// this walk. Deleting the current map member is safe in Go.
			peer.adjOut.mu.Lock()
			if peer.adjOut.session != session {
				peer.adjOut.mu.Unlock()
				return errAdjOutSession
			}
			keys := make([]fwdPathKey, 0, len(peer.adjOut.selectedPaths))
			for key := range peer.adjOut.selectedPaths {
				keys = append(keys, key)
			}
			peer.adjOut.mu.Unlock()
			session.adjOut = &peer.adjOut
			for _, key := range keys {
				work.output.key = key
				session.sentForward = &item
				if err := session.retireSelected(&work); err != nil {
					return err
				}
			}
			return nil
		})
}
