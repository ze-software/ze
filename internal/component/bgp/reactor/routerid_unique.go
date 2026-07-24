// Design: docs/architecture/core-design.md — router-ID conflict detection
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"encoding/binary"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// routerIDConflictError is returned when a peer's BGP Identifier conflicts
// with an established peer in the same AS.
// Implements NotifyCodes() for session.go NOTIFICATION dispatch.
type routerIDConflictError struct {
	conflictAddr netip.Addr
	peerAS       uint32
	bgpID        uint32
}

func (e *routerIDConflictError) Error() string {
	var b textbuf.Buffer
	return b.Reset().Str("duplicate router-id ").Str(bgpIDString(e.bgpID)).Str(" in AS ").Int(int64(e.peerAS)).Str(" (conflicts with established peer ").Str(e.conflictAddr.String()).Byte(')').String()
}

// NotifyCodes returns OPEN Message Error / Bad BGP Identifier.
// RFC 4271 Section 6.2: Bad BGP Identifier is the closest match for
// a router-ID that is valid syntactically but conflicts with another peer.
func (e *routerIDConflictError) NotifyCodes() (uint8, uint8) {
	return uint8(message.NotifyOpenMessage), message.NotifyOpenBadBGPID
}

// bgpIDString formats a BGP Identifier uint32 as a dotted-decimal IP string.
func bgpIDString(id uint32) string {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], id)
	return netip.AddrFrom4(b).String()
}

// checkRouterIDConflict checks if any ESTABLISHED peer in the same ASN
// already has the given BGP Identifier (router-ID).
//
// RFC 6286 Section 2.1: the BGP Identifier SHOULD be unique within an AS. Ze
// enforces that SHOULD by default (this function); an operator opts out via
// bgp/session/allow-shared-router-id, in which case validateOpen skips this call.
// Duplicate router-IDs within an AS otherwise indicate misconfiguration and can break:
//   - ORIGINATOR_ID loop detection in route reflection (RFC 4456)
//   - BGP Identifier tie-breaking in best path selection
//
// excludeKey is the peer being checked (to skip self in the peers map).
// Returns the conflicting peer's address if a conflict is found.
func checkRouterIDConflict(peers map[netip.AddrPort]*Peer, excludeKey netip.AddrPort, peerAS, bgpID uint32) (netip.Addr, bool) {
	for key, peer := range peers {
		if key == excludeKey {
			continue
		}
		// PeerAS via the p.mu-guarded accessor: a dynamic peer's PeerAS is written under
		// p.mu on establishment (resolveDynamicPeerSettings), and this runs on another
		// peer's OPEN-validation goroutine. Caller holds r.mu.RLock, so this keeps the
		// existing r.mu -> p.mu order (no new edge).
		if peer.PeerAS() != peerAS {
			continue
		}
		peer.mu.RLock()
		session := peer.session
		peer.mu.RUnlock()
		if session == nil {
			continue
		}
		if session.State() != fsm.StateEstablished {
			continue
		}
		session.mu.RLock()
		peerOpen := session.peerOpen
		session.mu.RUnlock()
		if peerOpen == nil {
			continue
		}
		if peerOpen.BGPIdentifier == bgpID {
			return peer.settings.Address, true
		}
	}
	return netip.Addr{}, false
}
