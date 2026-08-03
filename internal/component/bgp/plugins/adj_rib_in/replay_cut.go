// Design: docs/architecture/api/process-protocol.md — peer-up replay cut (egress-rail)
// Overview: rib.go — Adj-RIB-In storage and buildReplayRoutes, which applies this cut
// Related: rib_commands.go — the replay command that parses a caller-supplied cut

package adj_rib_in

// replayCut bounds a peer-up replay by reactor MessageID.
//
// The bound is carried as PRESENCE plus value, never as a magic value, because 0 is
// a legitimate cut. bgp-rs captures the cut as its own seenMsgID
// (rs/server_handlers.go handleState), which is 0 for every peer that establishes
// before that plugin has taken delivery of its first UPDATE -- the ordinary case
// when peers come up together, measured on 39 of 40 runs of
// test/plugin/llgr-readvertise-multipeer.ci.
//
// While presence and value were conflated, a cut of 0 disabled the bound entirely:
// the replay carried a route AND selectForwardTargets (rs/server_forward.go, which
// excludes a peer only on `msgID <= ForwardFrom`, never true at ForwardFrom 0)
// forwarded the same route live. Both rails reach the reactor's forwardUpdateCore,
// so the peer received two byte-identical UPDATEs back to back
// (ai/rules/evidence.md: a zero value must never be a valid-looking
// answer).
type replayCut struct {
	// maxMsgID is the newest reactor MessageID the caller had taken delivery of at
	// the instant it made the target peer a live forward target. Meaningful only
	// when bounded is true.
	maxMsgID uint64
	// bounded reports whether maxMsgID is a real cut. False means the caller does
	// not track one and wants every stored route -- what this plugin's own
	// self-replay asks for when no other plugin owns peer-up replay.
	bounded bool
}

// unboundedReplay is the cut for a caller that tracks none: replay everything.
func unboundedReplay() replayCut { return replayCut{} }

// replayUpTo is the cut for a caller that tracks one, including a cut of 0: nothing
// has been taken delivery of yet, so the live rail owns every route with a known
// MessageID and the replay owns none of them.
func replayUpTo(maxMsgID uint64) replayCut {
	return replayCut{maxMsgID: maxMsgID, bounded: true}
}

// excludes reports whether a stored route is the live rail's to deliver rather than
// this replay's.
//
// A route whose MessageID is unknown (0 -- the legacy text/JSON ingest path carries
// none) is never excluded: it can be sent twice, never dropped.
func (c replayCut) excludes(msgID uint64) bool {
	return c.bounded && msgID != 0 && msgID > c.maxMsgID
}
