// Design: docs/architecture/forward-congestion-pool.md -- the replay fence, End-of-RIB behind the forwards
// RFC: rfc/short/rfc4724.md -- End-of-RIB marker (Section 2)
// Related: reactor_api_forward.go -- AnnounceEOR queues the marker here
// Related: forward_pool.go -- fwdBatchHandler writes and settles the marker

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/family"
)

// fwdEndOfRIB is what an End-of-RIB item carries through the forward queue:
// the session it was queued for, the family it closes, and whether it holds
// that family's claim (Peer.claimInitialSyncEOR).
type fwdEndOfRIB struct {
	session *Session
	family  family.Family
	// ownsClaim is true when the call that queued this marker took the
	// family's claim. An explicit request is queued whether or not it took the
	// claim, so a failed write hands the claim back only when this is true:
	// releasing a claim another producer holds would let a second automatic
	// marker out for the family.
	ownsClaim bool
}

// queueEndOfRIB puts peer's End-of-RIB marker for fam at the tail of the
// peer's forward queue, so the worker writes it only after every item queued
// before it: the peer-up replay parked in overflow, and whatever already sits in
// the worker's channel. While the session's queueing gate is closed the item is
// held in overflow like every other (Peer.forwardOrderHold), so a marker asked
// for during the initial sync is written after the operations the sync drains.
//
// ownsClaim says whether the caller took fam's claim (claimInitialSyncEOR). The
// worker settles an owned claim (settleEndOfRIB). False means the pool has
// stopped, and an owned claim is still the caller's to hand back.
//
// RFC 4724 Section 2: the End-of-RIB marker is used "to indicate to its peer the
// completion of the initial routing update after the session is established".
// Written at once, the marker reached the peer ahead of replay items still in
// the queue, and a receiver acting on it (a graceful-restart helper purging
// stale routes) read an initial update as complete that was not.
//
// The item is marked as part of the initial update, so it passes the replay
// fence the way the replay ahead of it does (Peer.forwardOrderHold): the marker
// ends the initial update, and the live changes the fence holds follow it.
func (a *reactorAPIAdapter) queueEndOfRIB(target announceTarget, fam family.Family, update *message.Update, ownsClaim bool) bool {
	return a.r.fwdPool.dispatchOverflow(fwdKey{peerAddr: target.peer.settings.PeerKey()}, &fwdItem{
		updates:            []*message.Update{update},
		peer:               target.peer,
		session:            target.session,
		destinationOwner:   target.destinationOwner,
		destinationSession: target.destinationSession,
		initialUpdate:      true,
		endOfRIB:           &fwdEndOfRIB{session: target.session, family: fam, ownsClaim: ownsClaim},
	})
}

// claimEndOfRIB cannot claim a family on a successor of the captured Session.
func (target announceTarget) claimEndOfRIB(fam family.Family) (bool, error) {
	target.peer.mu.Lock()
	defer target.peer.mu.Unlock()
	if target.peer.session != target.session {
		return false, errAdjOutSession
	}
	if target.peer.initialSyncEOR == nil {
		target.peer.initialSyncEOR = make(map[family.Family]bool, 2)
	}
	if target.peer.initialSyncEOR[fam] {
		return false, nil
	}
	target.peer.initialSyncEOR[fam] = true
	return true, nil
}

// releaseEndOfRIBClaim cannot release a successor's independent family claim.
// The captured Session check and claim removal share the lifecycle lock.
func (target announceTarget) releaseEndOfRIBClaim(fam family.Family) {
	target.peer.mu.Lock()
	defer target.peer.mu.Unlock()
	if target.peer.session != target.session {
		return
	}
	delete(target.peer.initialSyncEOR, fam)
}

// settleEndOfRIB settles the End-of-RIB markers one batch carried for session.
// A marker that reached the socket is metered. One that did not hands back the
// claim it owns (fwdEndOfRIB.ownsClaim), so another producer may still deliver
// the family's automatic marker (Peer.claimInitialSyncEOR); one that owns no
// claim hands back nothing. A marker queued for an earlier session settles
// nothing: that session's claims were cleared at its teardown, and the current
// one's are not its to touch.
func settleEndOfRIB(peer *Peer, session *Session, items []fwdItem, written bool) {
	for i := range items {
		mark := items[i].endOfRIB
		if mark == nil || mark.session != session {
			continue
		}
		if written {
			peer.incrEORSent()
			continue
		}
		if mark.ownsClaim {
			announceTarget{peer: peer, session: mark.session}.releaseEndOfRIBClaim(mark.family)
		}
	}
}
