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
// The owner is the *Peer rather than its address/port key because a dynamic peer's settings
// can be replaced during its life, while the pointer is stable from creation to teardown --
// a release keyed on settings could miss and leak a claim, which would reject a legitimate
// later peer forever.
//
// The mutex is a LEAF: no reactor or peer lock is ever taken while it is held, so a claim or
// release is safe from any context (including a peer teardown running under RemovePeer).
type routerIDClaims struct {
	mu      sync.Mutex
	byID    map[routerIDKey]routerIDHolder
	byOwner map[*Peer]routerIDKey
}

// claim records p as the holder of (peerAS, bgpID), reporting a conflict when another peer
// already holds it. addr is p's address, recorded for the conflict message of a later claimant.
//
// Returns the conflicting peer's address and false when the identifier is taken; the zero
// address and true when the claim is granted. Re-claiming with the same peer is always granted
// (a reconnecting peer, or a second OPEN on the same session), and re-claiming with a different
// identifier releases the peer's previous one.
func (c *routerIDClaims) claim(p *Peer, addr netip.Addr, peerAS, bgpID uint32) (netip.Addr, bool) {
	key := routerIDKey{peerAS: peerAS, bgpID: bgpID}

	c.mu.Lock()
	defer c.mu.Unlock()

	if holder, taken := c.byID[key]; taken && holder.peer != p {
		return holder.addr, false
	}

	if c.byID == nil {
		c.byID = make(map[routerIDKey]routerIDHolder)
		c.byOwner = make(map[*Peer]routerIDKey)
	}

	// The peer may have held a different identifier on a previous session.
	//
	// The holder check mirrors release: it is only safe to drop byID[previous]
	// if p is still the recorded holder there. Today that is implied, because
	// byID and byOwner are only ever written together under this lock, so
	// byID[K].peer == p iff byOwner[p] == K. The guard defends the invariant
	// rather than relying on it -- a single future write to byID that skips
	// byOwner would otherwise turn this into a wrong-peer free, handing one
	// peer's identifier to another.
	if previous, held := c.byOwner[p]; held && previous != key {
		if holder, taken := c.byID[previous]; taken && holder.peer == p {
			delete(c.byID, previous)
		}
	}

	c.byID[key] = routerIDHolder{peer: p, addr: addr}
	c.byOwner[p] = key
	return netip.Addr{}, true
}

// release drops whatever identifier p holds. Called on every session teardown, so an
// identifier is available to another peer as soon as its holder's session ends.
// Safe to call for a peer that holds nothing.
func (c *routerIDClaims) release(p *Peer) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key, held := c.byOwner[p]
	if !held {
		return
	}
	delete(c.byOwner, p)
	if holder, taken := c.byID[key]; taken && holder.peer == p {
		delete(c.byID, key)
	}
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
