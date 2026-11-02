// Design: docs/architecture/bgp/structural-forwarding.md -- reactor-native RS forwarding
// Related: forward_rs.go -- reactorForwardRSSection, the rail that writes directly
// Related: forward_pool.go -- the worker rail the direct write bypasses

package reactor

import "github.com/ze-software/ze/internal/component/bgp/fsm"

// tryDirectWriteNoFlush writes UPDATE bodies directly to the destination peer's
// TCP bufWriter from the caller's goroutine (source peer's read goroutine),
// bypassing the forward pool channel send and goroutine context switch.
//
// Does NOT flush or set write deadlines. The caller batches flushes via
// Session.flushFwdDirty when the source bufReader has no more data.
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
		session.writeMu.Unlock()
		return false, false, nil
	}

	if !forwardSourceCurrent(item.receivedPeer, item.receivedGeneration) {
		session.writeMu.Unlock()
		return true, false, nil
	}
	if item.session != session {
		session.writeMu.Unlock()
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
		session.writeMu.Unlock()
	}()
	for _, body := range item.rawBodies {
		if err := session.writeRawUpdateBody(body); err != nil {
			return true, false, session
		}
		item.writeBody++
	}
	for _, update := range item.updates {
		// Pre-filtered: forwardUpdateCore already ran this peer's export chain
		// (and only then the EBGP prepend). See writeUpdatePreFiltered.
		if err := session.writeUpdatePreFiltered(update); err != nil {
			return true, false, session
		}
		item.writeBody++
	}
	return true, true, session
}
