// Design: docs/architecture/forward-congestion-pool.md -- the replay fence, End-of-RIB behind the forwards
// RFC: rfc/short/rfc4724.md -- End-of-RIB marker (Section 2)
// Related: reactor_api_forward.go -- AnnounceEOR queues the marker here
// Related: forward_pool.go -- fwdBatchHandler writes and settles the marker

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/family"
)

// fwdEndOfRIB is the claim an End-of-RIB item carries through the forward
// queue: the session it was claimed for, and the family it closes.
type fwdEndOfRIB struct {
	session *Session
	family  family.Family
}

// queueEndOfRIB puts peer's End-of-RIB marker for fam at the tail of the
// peer's forward queue, so the worker writes it only after every item queued
// before it: the peer-up replay parked in overflow, and whatever already sits in
// the worker's channel. The caller has claimed fam (claimInitialSyncEOR), and
// the worker settles that claim (settleEndOfRIB). False means the pool has
// stopped and the claim is still the caller's to hand back.
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
func (a *reactorAPIAdapter) queueEndOfRIB(peer *Peer, fam family.Family, update *message.Update) bool {
	peer.mu.RLock()
	session := peer.session
	peer.mu.RUnlock()
	return a.r.fwdPool.dispatchOverflow(fwdKey{peerAddr: peer.settings.PeerKey()}, fwdItem{
		updates:       []*message.Update{update},
		peer:          peer,
		initialUpdate: true,
		endOfRIB:      &fwdEndOfRIB{session: session, family: fam},
	})
}

// settleEndOfRIB settles the End-of-RIB markers one batch carried for session.
// A marker that reached the socket is metered. One that did not hands its claim
// back, so another producer may still deliver it (Peer.claimInitialSyncEOR). A
// marker claimed for an earlier session settles nothing: that session's claims
// were cleared at its teardown, and the current one's are not its to touch.
//
// RFC 4724 Section 2 defines one marker that signals "the completion of the
// initial routing update", so a claim whose marker never reached the peer must
// not stand.
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
		peer.releaseInitialSyncEOR(mark.family)
	}
}
