// Design: docs/architecture/core-design.md — route server plugin
// Overview: server.go — RouteServer uses PeerState for peer tracking and forwarding decisions

package rs

import "github.com/ze-software/ze/internal/core/family"

// PeerState tracks the state of a BGP peer.
type PeerState struct {
	Address string // Peer IP address
	ASN     uint32 // Peer AS number
	Up      bool   // Session is established
	// StateSeen records that handleState has processed at least one state event
	// for this peer, which is what makes Up == false READABLE.
	//
	// Without it, `!Up` conflates two opposite situations. A PeerState is created
	// by handleOpen (server_handlers.go) as well as by handleState, and the engine
	// relays a peer's OPEN and its first UPDATE from the session read path while
	// the state transition travels the FSM goroutine -- so an UPDATE can reach a
	// worker while the peer is still `Up == false` and merely NOT YET UP. Reading
	// that as "down" made processForward discard the whole UPDATE, and because
	// dispatchStructured has already advanced seenMsgID past it, the cut
	// handleStateUp then captures excludes it from the replay as well: permanent
	// route loss on a healthy session. StateSeen is false only in the not-yet
	// window; a peer that went down keeps its entry with StateSeen true.
	StateSeen bool
	// Replaying is true from handleStateUp until replayForPeer finishes. It is NOT
	// consulted by selectForwardTargets: a replaying peer IS a live-forward target
	// on purpose, because excluding it loses routes when peers connect together
	// (TestReplayingPeerIncludedInForwardTargets, plan/learned/630-rs-fastpath-3-passthrough.md).
	// Its only readers are the replay goroutine's own generation bookkeeping.
	Replaying bool // In-flight RIB replay; see note above

	ReplayGen    uint64                 // Incremented on each handleStateUp, guards stale goroutines
	Capabilities map[string]bool        // Negotiated capabilities (e.g., "route-refresh": true)
	Families     map[family.Family]bool // Supported AFI/SAFI

	// ForwardFrom is the peer-up CUT: the newest reactor MessageID this plugin
	// had seen at the instant this peer became a live forward target. It is
	// captured under rs.mu in the same critical section that sets Up, so there is
	// no instant at which the peer is a forward target without a cut, or has a
	// cut without being a forward target.
	//
	// It partitions every route exactly once:
	//   msgID <= ForwardFrom  -> the peer's Adj-RIB-In replay delivers it
	//   msgID >  ForwardFrom  -> the live forward delivers it
	//
	// MessageID is the right quantity because it is the only one bgp-rs and
	// bgp-adj-rib-in both observe per route (reactor.nextMsgID stamps it on the
	// RawMessage both plugins are handed). A wall-clock instant or either
	// plugin's own counter would need the two event streams to be ordered
	// against each other, which they are not.
	//
	// Zero means "no UPDATE seen before this peer came up", which makes the
	// replay unbounded -- correct, since nothing this plugin forwarded can
	// predate the cut.
	ForwardFrom uint64
}

// HasCapability returns true if peer supports the given capability.
func (p *PeerState) HasCapability(cap string) bool {
	if p.Capabilities == nil {
		return false
	}
	return p.Capabilities[cap]
}

// SupportsFamily returns true if peer supports the given AFI/SAFI.
// A nil Families map (no OPEN received yet) is treated as "accept all" to avoid
// dropping routes during the window between state-up and OPEN processing.
func (p *PeerState) SupportsFamily(fam family.Family) bool {
	if p.Families == nil {
		return true
	}
	return p.Families[fam]
}
