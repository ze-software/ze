// Design: docs/architecture/api/commands.md -- the quiesce barrier, what a peer still owes
// Overview: peer.go -- Peer, pendingSync
// Related: reactor_api.go -- DrainPeerSync, peersSynced
// Related: forward_pool.go -- fwdWorker.overflowPending, the per-peer forward counts

package reactor

import (
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// peerSyncOwed names what a peer still owes the wire, or that it owes nothing.
// Peer.syncOwed produces it, pendingSync reads it as a bool, and the
// bgp-peer-sync quiescer's timeout error prints it, so the error a caller reads
// names the same fact that kept the barrier closed.
//
// The zero value is unspecified and is never an answer: syncOwed returns
// peerSyncSettled when nothing is owed.
type peerSyncOwed uint8

const (
	peerSyncOwedUnspecified peerSyncOwed = iota
	// peerSyncSettled: the peer owes nothing this barrier waits for.
	peerSyncSettled
	// peerSyncOwedQueuedRoutes: Peer.opQueue holds routes queued before the
	// session reached Established.
	peerSyncOwedQueuedRoutes
	// peerSyncOwedInitialRoutes: the session's initial-sync gate is closed
	// (Session.sendingInitialRoutes).
	peerSyncOwedInitialRoutes
	// peerSyncOwedEndOfRIB: the session's End-of-RIB is still owed
	// (Session.initialSyncEOROwed).
	peerSyncOwedEndOfRIB
	// peerSyncOwedReplay: the replay fence is up (Peer.initialUpdateOwed), so a
	// process that replays routes to this peer on peer-up has not reported its
	// replay done.
	peerSyncOwedReplay
	// peerSyncOwedPolicyReplay: a final policy restoration holds the peer
	// (Peer.policyReplayHolds).
	peerSyncOwedPolicyReplay
	// peerSyncOwedForwards: the peer's forward worker still owes it items,
	// through overflow or its channel (Peer.fwdOverflowPending,
	// Peer.fwdChannelPending).
	peerSyncOwedForwards
	// peerSyncOwedHandshake: the session is in OpenSent or OpenConfirm
	// (handshakeInFlight).
	peerSyncOwedHandshake
	// peerSyncOwedRouteDecision: the session still owes a recovery election
	// or an admitted route update has not reached its flush (adjRIBOut.owed,
	// adjRIBOut.pending).
	peerSyncOwedRouteDecision
)

// String is the plain-words reason the quiesce timeout error prints.
func (o peerSyncOwed) String() string {
	switch o {
	case peerSyncOwedUnspecified:
		return "no answer was computed (a Ze defect)"
	case peerSyncSettled:
		return "nothing owed"
	case peerSyncOwedQueuedRoutes:
		return "routes queued before its session came up are not written yet"
	case peerSyncOwedInitialRoutes:
		return "its initial routes are still being written"
	case peerSyncOwedEndOfRIB:
		return "its End-of-RIB is still owed"
	case peerSyncOwedReplay:
		return "a peer-up replay to it has not reported done"
	case peerSyncOwedPolicyReplay:
		return "a policy change is still re-sending routes to it"
	case peerSyncOwedForwards:
		return "forwarded routes are still queued for it"
	case peerSyncOwedHandshake:
		return "its OPEN handshake is still in progress"
	case peerSyncOwedRouteDecision:
		return "its route decisions or updates are not flushed yet"
	}
	return "an unknown sync state (a Ze defect)"
}

// syncOwed reports what this peer still owes the wire, or peerSyncSettled.
// pendingSync is this answer read as a bool, so the two never disagree.
//
// The replay fence and the policy restoration hold are read only while the
// peer is Established, as forwardOrderHold reads them: resetAPISync clears the
// fence at the NEXT establishment, not at session end, so a peer that went down
// with its fence up holds a value no session owns. The forward counts are read
// in every state: an item queued for a peer that is going down is still
// unwritten until its worker has discarded it, and that never waits on a peer.
//
// Lock order: syncOwed snapshots the queue and session under p.mu, then releases
// it before reading the session's atomic facts or the destination table. It
// never nests p.mu with adjOut.mu or a forward-pool lock (fwdPool.mu,
// fwdWorker.overflowMu). forwardOrderHold, which reads the same atomic facts
// under fwdWorker.overflowMu, MUST NOT take p.mu: the peer's own goroutine takes
// p.mu before fwdWorker.overflowMu (takeOverflowReleased), so the reverse would
// deadlock.
func (p *Peer) syncOwed() peerSyncOwed {
	p.mu.RLock()
	queuedRoutes := len(p.opQueue) > 0
	session := p.session
	p.mu.RUnlock()
	if queuedRoutes {
		return peerSyncOwedQueuedRoutes
	}
	if session != nil {
		if session.sendingInitialRoutes.Load() != 0 {
			return peerSyncOwedInitialRoutes
		}
		if session.initialSyncEOROwed.Load() {
			return peerSyncOwedEndOfRIB
		}
	}
	if p.State() == PeerStateEstablished {
		if p.initialUpdateOwed.Load() {
			return peerSyncOwedReplay
		}
		if p.policyReplayHolds.Load() != 0 {
			return peerSyncOwedPolicyReplay
		}
	}
	if p.forwardOverflowPending() {
		return peerSyncOwedForwards
	}
	if p.forwardChannelPending() {
		return peerSyncOwedForwards
	}
	if handshakeInFlight(session) {
		return peerSyncOwedHandshake
	}
	if p.routeDecisionOwed(session) {
		return peerSyncOwedRouteDecision
	}
	return peerSyncSettled
}

// routeDecisionOwed reports only work framed for session, the current session
// syncOwed captured. A nil session or a table already reset or rebound owes
// nothing for that capture. The owed set survives the lookup and admission
// until the decision is flushed. An originated takeover can remove its owe
// earlier, so the table's pending write still keeps the barrier closed.
// MUST be called without p.mu, writeMu or a forward-pool lock held.
func (p *Peer) routeDecisionOwed(session *Session) bool {
	if session == nil {
		return false
	}
	table := &p.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	if table.session != session {
		return false
	}
	if len(table.owed) > 0 {
		return true
	}
	return table.pending
}

// replayFenceUnreported names the processes holding this peer's replay fence
// that have not reported their peer-up replay done. Takes p.mu for read, so the
// caller MUST NOT hold it.
func (p *Peer) replayFenceUnreported() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var unreported []string
	for _, name := range p.replayFence {
		if _, reported := p.apiSyncSignalled[name]; !reported {
			unreported = append(unreported, name)
		}
	}
	return unreported
}

// peerSyncOwedReportMax bounds how many peers the quiesce timeout error names,
// so a box with many peers owed work still answers in a few lines.
const peerSyncOwedReportMax = 8

// peerSyncOwedError is the error the bgp-peer-sync quiescer returns when its
// drain ended with a peer still owed work (DrainPeerSync). It names each such
// peer by address and what it is owed, in plain words, so a caller can tell a
// replay still passing from an End-of-RIB still owed, and it names the process
// a replay waits for. cause is the drain's context error and stays its cause.
func peerSyncOwedError(peers []*Peer, cause error) error {
	var tb textbuf.Buffer
	named := 0
	owed := 0
	for _, p := range peers {
		reason := p.syncOwed()
		if reason == peerSyncSettled {
			continue
		}
		owed++
		if named == peerSyncOwedReportMax {
			continue
		}
		if named > 0 {
			tb.Str("; ")
		}
		named++
		tb.Str("peer ").Str(p.settings.Address.String()).Str(": ").Str(reason.String())
		if reason != peerSyncOwedReplay {
			continue
		}
		if waiting := p.replayFenceUnreported(); len(waiting) > 0 {
			tb.Str(" (waiting for ").Str(textbuf.Join(waiting, ", ")).Byte(')')
		}
	}
	if owed == 0 {
		// Every peer settled between the drain's last poll and this read.
		return fmt.Errorf("every peer settled just after the drain ended: %w", cause)
	}
	if owed > named {
		tb.Str("; and ").Int(int64(owed - named)).Str(" more peers")
	}
	return fmt.Errorf("%s: %w", tb.String(), cause)
}
