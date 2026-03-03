// Design: docs/architecture/core-design.md — BGP reactor event loop

package reactor

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/message"
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
	return fmt.Sprintf("duplicate router-id %s in AS %d (conflicts with established peer %s)",
		bgpIDString(e.bgpID), e.peerAS, e.conflictAddr)
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
// RFC 4271 Section 4.2: The BGP Identifier "MUST be unique within an AS."
// Duplicate router-IDs within an AS indicate misconfiguration and can break:
//   - ORIGINATOR_ID loop detection in route reflection (RFC 4456)
//   - BGP Identifier tie-breaking in best path selection
//
// excludeKey is the peer being checked (to skip self in the peers map).
// Returns the conflicting peer's address if a conflict is found.
func checkRouterIDConflict(peers map[string]*Peer, excludeKey string, peerAS, bgpID uint32) (netip.Addr, bool) {
	for key, peer := range peers {
		if key == excludeKey {
			continue
		}
		if peer.settings.PeerAS != peerAS {
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
