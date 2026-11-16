// Design: docs/architecture/behavior/peer-lifecycle.md -- incoming offers across reload.
// Related: peer_settings_negotiation.go -- the existing declaration judgment.
// Related: incoming_candidate.go -- typed admission and handoff ownership.
// RFC 5492 Section 1 -- see rfc/short/rfc5492.md.
package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// openSessionSnapshot retains an exact connection identity across receipt moves.
// candidate is nil when only the current Session owns this connection. The
// admitted OPEN lives once on Session.peerOpen under mu, before negotiation.
type openSessionSnapshot struct {
	session   *Session
	candidate *incomingCandidate
}

type peerOpenSnapshot struct {
	peer     *Peer
	current  *Session
	sessions []openSessionSnapshot
}

// snapshotOpenSessions captures current, pending and typed inbound owners in
// one p.mu hold. At most three Session pointers are deduplicated. A constructor
// hidden behind pendingSocket must check the current store after construction
// and before takeover; every earlier fresh check belongs to this census.
func (p *Peer) snapshotOpenSessions() peerOpenSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	snapshot := peerOpenSnapshot{peer: p, current: p.session, sessions: make([]openSessionSnapshot, 0, 3)}
	add := func(session *Session, candidate *incomingCandidate) {
		if session == nil {
			return
		}
		for i := range snapshot.sessions {
			if snapshot.sessions[i].session != session {
				continue
			}
			if candidate != nil {
				snapshot.sessions[i].candidate = candidate
			}
			return
		}
		snapshot.sessions = append(snapshot.sessions, openSessionSnapshot{session: session, candidate: candidate})
	}
	add(p.session, nil)
	switch pending := p.pending.(type) {
	case nil, *pendingSocket, *closingIncoming:
		// No published Session, or a terminal connection that cannot promote.
	case *incomingCandidate:
		add(pending.session, pending)
	}
	switch arrival := p.inbound.(type) {
	case nil, *incomingSocket, *bufferedIncoming:
		// The next ordinary Session has not prepared a local OPEN here.
	case *candidateOpen:
		add(arrival.candidate.session, arrival.candidate)
	case *candidateReady:
		add(arrival.candidate.session, arrival.candidate)
	}
	return snapshot
}

// reconcilePluginOffers checks every retained connection after releasing all
// Peer locks. A captured candidate can have moved into runOnce, so the exact
// Session is sealed rather than looking up whichever Session is current now.
// Current-only epochs use the ordinary configuration-change teardown; typed
// incoming receipts use Connection Rejected. Both retain their existing owner.
func (snapshot peerOpenSnapshot) reconcilePluginOffers() {
	for _, epoch := range snapshot.sessions {
		if epoch.session.pluginCapabilitiesKept() {
			continue
		}
		if epoch.candidate == nil {
			// Teardown reports its bounded write failures and always returns nil.
			_ = epoch.session.Teardown(message.NotifyCeaseOtherConfigChange, "") //nolint:errcheck // The teardown owns reporting.
			continue
		}
		epoch.session.refuseStalePluginOffer()
		snapshot.peer.discardStaleInbound(epoch.candidate)
	}
}

// discardStaleInbound consumes only the exact receipt this reload refused.
// A selected Session remains runOnce's; a pending reader keeps its own cleanup.
func (p *Peer) discardStaleInbound(candidate *incomingCandidate) {
	p.mu.Lock()
	if p.session == candidate.session {
		p.mu.Unlock()
		return
	}
	if inboundCandidate(p.inbound) != candidate {
		p.mu.Unlock()
		return
	}
	arrival := p.inbound
	p.inbound = nil
	p.collisionHandoff = false
	select {
	case <-p.inboundNotify:
	default: // No queued wakeup remains for the refused receipt.
	}
	p.mu.Unlock()
	discardInbound(arrival)
}

// refuseStalePluginOffer seals before its bounded NOTIFICATION can wait, so a
// blocked policy callback cannot subsequently publish this exact connection.
// Event23 accounts for the failed connection without resetting peer history.
func (s *Session) refuseStalePluginOffer() {
	// The shared teardown reports write failures itself and always returns nil.
	_ = s.teardownWith(teardownNotice{code: message.NotifyCease, subcode: message.NotifyCeaseConnectionRejected}, fsm.EventOpenCollisionDump) //nolint:errcheck // The teardown owns reporting.
}
