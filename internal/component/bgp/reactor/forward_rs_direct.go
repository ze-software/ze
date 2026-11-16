// Design: docs/architecture/bgp/structural-forwarding.md -- reactor-native RS forwarding
// Related: forward_rs.go -- reactorForwardRSSection, the rail that writes directly
// Related: forward_pool.go -- the worker rail the direct write bypasses

package reactor

import "github.com/ze-software/ze/internal/component/bgp/fsm"

// tryDirectWriteNoFlush writes UPDATE bodies directly to the destination peer's
// TCP bufWriter from the caller's goroutine (source peer's read goroutine),
// bypassing the forward pool channel send and goroutine context switch.
//
// Does NOT flush. The caller batches flushes via Session.flushFwdDirty when the
// source bufReader has no more data.
//
// The write is bounded by the session's write budget, like every other write
// to a session socket: a batch that overflows the destination's bufWriter
// writes to the socket right here, on the source's read goroutine, and a
// destination that reads nothing would hold that goroutine and its own write
// path for good. The budget is armed once per destination per batch, never
// once per UPDATE, and left standing when the path is given back
// (Session.armFastPathBudget, Session.releaseFastPathWrites). A write that
// outlasts it fails, which retires the destination (Session.retireWrite) and
// reports it as a peer that stopped reading; the source's loop then goes on.
//
// Returns (handled, delivered, dstSession):
//   - (true, true, session): bytes reached the peer's bufWriter; needs deferred flush
//   - (true, false, session): a write FAILED partway; the item is consumed, nothing
//     reliable reached the peer
//   - (true, false, nil): peer not Established; skip (caller must call done)
//   - (false, false, nil): TryLock failed or no session; fall back to pool
//
// handled and delivered are separate on purpose. handled answers "is this item
// finished with, do not re-dispatch it"; delivered answers "did the UPDATE reach
// the peer". Collapsing them is what let reactor_notify.go claim delivery -- and
// so let bgp-rs drop the UPDATE on its `default` arm -- for a peer that was not
// Established or whose write errored.
func tryDirectWriteNoFlush(item *fwdItem) (handled, delivered bool, dst *Session) {
	if !forwardSourceCurrent(item.receivedPeer, item.receivedGeneration) {
		return true, false, nil
	}
	peer := item.peer
	if peer == nil {
		return false, false, nil
	}

	peer.mu.RLock()
	session := peer.session
	peer.mu.RUnlock()
	if session == nil {
		return false, false, nil
	}

	session.mu.RLock()
	state := session.fsm.State()
	session.mu.RUnlock()
	if state != fsm.StateEstablished {
		return true, false, nil
	}

	if !session.writeMu.TryLock() {
		return false, false, nil
	}
	// A live policy restoration can raise its replay fence after dispatch
	// admission. It raises the fence under this same write lock.
	if peer.forwardOrderHold(item.initialUpdate) {
		session.releaseFastPathWrites()
		return false, false, nil
	}

	if !forwardSourceCurrent(item.receivedPeer, item.receivedGeneration) {
		session.releaseFastPathWrites()
		return true, false, nil
	}
	if item.session != session {
		session.releaseFastPathWrites()
		return true, false, nil
	}
	session.sentMeta = item.meta
	session.sentSourcePeerStr = item.sourcePeerStr
	session.sentSourceMessageID = item.sourceMessageID
	session.sentForward = item
	item.writeBody = 0
	defer func() {
		session.sentMeta = nil
		session.sentSourcePeerStr = ""
		session.sentSourceMessageID = 0
		session.sentForward = nil
		session.releaseFastPathWrites()
	}()
	if err := session.armFastPathBudget(fwdWriteDeadline()); err != nil {
		// A socket that refuses its deadline is closed: retire the session, as
		// the write to it would have (armWriteOrRetire does the same for the
		// other UPDATE writers). The item is consumed and reached nobody.
		session.retireWrite(err)
		return true, false, session
	}
	// RFC 4271 Section 9.1.3: one fenced group admits removals before its
	// surviving output or EOR and counts this destination only once. An active
	// RIB owner may settle the wire withdrawal or replacement later.
	// refuse-by-default: guards reactorForwardRSSectionResult -- handled consumes once; delivered=false refuses delivery and retry after partial output
	if err := session.writePolicyRemovals(item); err != nil {
		return true, false, session
	}
	for _, body := range item.rawBodies {
		// refuse-by-default: guards reactorForwardRSSectionResult -- handled consumes once; delivered=false refuses delivery and retry after partial output
		if err := session.writeRawUpdateBody(body); err != nil {
			return true, false, session
		}
		item.writeBody++
	}
	for _, update := range item.updates {
		// Pre-filtered: forwardUpdateCore already ran this peer's export chain
		// (and only then the EBGP prepend). See writeUpdatePreFiltered.
		// refuse-by-default: guards reactorForwardRSSectionResult -- handled consumes once; delivered=false refuses delivery and retry after partial output
		if err := session.writeUpdatePreFiltered(update); err != nil {
			return true, false, session
		}
		item.writeBody++
	}
	return true, true, session
}
