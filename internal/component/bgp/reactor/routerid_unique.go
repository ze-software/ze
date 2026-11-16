// Design: docs/architecture/core-design.md — router-ID conflict detection
// Overview: peer.go — Peer struct and FSM state machine
// Related: session_open_validation.go — the OPEN rails that run this check
// RFC: rfc/short/rfc6286.md — AS-wide unique BGP Identifier

package reactor

import (
	"encoding/binary"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// routerIDConflictError is returned when a peer's BGP Identifier conflicts
// with another peer in the same AS.
// Implements NotifyCodes() for session.go NOTIFICATION dispatch.
type routerIDConflictError struct {
	conflictAddr netip.Addr
	peerAS       uint32
	bgpID        uint32
}

func (e *routerIDConflictError) Error() string {
	var b textbuf.Buffer
	return b.Reset().Str("duplicate router-id ").Str(bgpIDString(e.bgpID)).Str(" in AS ").Int(int64(e.peerAS)).Str(" (conflicts with peer ").Str(e.conflictAddr.String()).Byte(')').String()
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

// routerIDKey identifies a BGP Identifier within one autonomous system.
// RFC 6286 Section 2.1 scopes uniqueness to an AS, so the same identifier in a
// different AS is not a conflict.
type routerIDKey struct {
	peerAS uint32
	bgpID  uint32
}

// routerIDHolder records which peer holds a key, plus the address to name in the
// conflict error. The address is snapshotted at claim time so reporting a conflict
// never has to reach back into the holding peer (and never takes its lock).
type routerIDHolder struct {
	peer *Peer
	addr netip.Addr
}

// routerIDClaims is the reactor's AS-wide BGP Identifier registry.
//
// RFC 6286 Section 2.1: the BGP Identifier "should be unique within an AS". Ze enforces
// that SHOULD by default; an operator opts out with bgp/session/allow-shared-router-id,
// in which case the registry is never consulted (Peer.validateOpen skips the claim) and
// ze performs no AS-wide identifier check at all. Because uniqueness is only a SHOULD,
// both behaviors are conformant. Duplicate identifiers within an AS otherwise indicate
// misconfiguration and can break:
//   - ORIGINATOR_ID loop detection in route reflection (RFC 4456)
//   - BGP Identifier tie-breaking in best path selection
//
// The claim is taken when the peer's OPEN is validated -- NOT when its session reaches
// Established. That is what makes the outcome independent of scheduling: two peers of one
// AS presenting one identifier at the same moment race for a single map entry, and exactly
// one wins. The previous design scanned for an ESTABLISHED peer carrying the identifier, so
// two concurrent OPENs each saw a not-yet-established peer and BOTH were accepted.
//
// Ownership is a Peer and one of its Sessions, never an address/port key. A
// Peer can have its incumbent and one admitted incoming candidate at once.
// Each holds its own claim until its own cleanup; ending one connection must
// not free the other's identifier. These stable pointers also survive dynamic
// settings replacement. Peer removal releases every claim of that Peer.
// The Peer's single pending slot bounds each ownership map at two Sessions.
//
// The mutex is a LEAF: no reactor or peer lock is ever taken while it is held, so a claim or
// release is safe from any context (including a peer teardown running under RemovePeer).
type routerIDClaims struct {
	mu      sync.Mutex
	byID    map[routerIDKey]routerIDHolder
	byOwner map[*Peer]map[*Session]routerIDKey
}

// claim records this Session's claim, rejecting only another Peer's identifier.
// p and session MUST name the factory-created connection being validated;
// neither may be nil. A zero registry is ready for use. Reclaiming on the same
// Session replaces only that Session's former key. Safe for concurrent use.
// RFC 6286 Section 2.1: "The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS."
func (c *routerIDClaims) claim(p *Peer, session *Session, addr netip.Addr, peerAS, bgpID uint32) (netip.Addr, bool) {
	key := routerIDKey{peerAS: peerAS, bgpID: bgpID}

	c.mu.Lock()
	defer c.mu.Unlock()

	if holder, taken := c.byID[key]; taken && holder.peer != p {
		return holder.addr, false
	}

	if c.byID == nil {
		c.byID = make(map[routerIDKey]routerIDHolder)
		c.byOwner = make(map[*Peer]map[*Session]routerIDKey)
	}

	owned := c.byOwner[p]
	if owned == nil {
		owned = make(map[*Session]routerIDKey)
		c.byOwner[p] = owned
	}
	if previous, held := owned[session]; held {
		if previous != key {
			delete(owned, session)
			c.releaseUnheldKeyLocked(p, previous, owned)
		}
	}

	c.byID[key] = routerIDHolder{peer: p, addr: addr}
	owned[session] = key
	return netip.Addr{}, true
}

// releaseSession releases only the ending connection's claim. The caller MUST
// join that Session's OPEN validation before calling, so it cannot claim again.
// A sibling candidate or incumbent retaining the same key keeps it reserved.
// Safe for concurrent use; a Session holding nothing is a no-op.
// RFC 6286 Section 2.1: "The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS."
func (c *routerIDClaims) releaseSession(p *Peer, session *Session) {
	c.mu.Lock()
	defer c.mu.Unlock()
	owned := c.byOwner[p]
	key, held := owned[session]
	if !held {
		return
	}
	delete(owned, session)
	c.releaseUnheldKeyLocked(p, key, owned)
	if len(owned) == 0 {
		delete(c.byOwner, p)
	}
}

// releaseUnheldKeyLocked drops a key only after the last connection of p gave
// it up. The caller MUST hold c.mu. The owning Peer admits at most two Sessions,
// so the scan is bounded at two and needs no second reference-count ledger.
// RFC 6286 Section 2.1: "The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS."
func (c *routerIDClaims) releaseUnheldKeyLocked(p *Peer, key routerIDKey, owned map[*Session]routerIDKey) {
	for _, retained := range owned {
		if retained == key {
			return
		}
	}
	if holder, held := c.byID[key]; held {
		if holder.peer == p {
			delete(c.byID, key)
		}
	}
}

// release drops all claims currently recorded for p. Removal uses it for an
// immediate release; the lifecycle owner MUST release again after joining OPEN
// work, so a validation already in flight cannot leave a later claim behind.
// Safe for concurrent use and for a Peer holding nothing.
func (c *routerIDClaims) release(p *Peer) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, key := range c.byOwner[p] {
		if holder, taken := c.byID[key]; taken {
			if holder.peer == p {
				delete(c.byID, key)
			}
		}
	}
	delete(c.byOwner, p)
}

// holder returns the peer currently holding (peerAS, bgpID), for tests and diagnostics.
func (c *routerIDClaims) holder(peerAS, bgpID uint32) (*Peer, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	holder, taken := c.byID[routerIDKey{peerAS: peerAS, bgpID: bgpID}]
	if !taken {
		return nil, false
	}
	return holder.peer, true
}
