// Design: docs/architecture/core-design.md -- egress attribute modification on the forward rails
// RFC: rfc/short/rfc8669.md -- the Prefix-SID leaves the SR domain only when configured (Section 8)
// Related: peer_forward_facts.go -- the sibling applyFacts* egress decisions
// Related: forward_local_pref.go -- payloadHasAttr, and the same shape for RFC 4271 Section 5.1.5
// Related: reactor_api_forward.go -- forwardUpdateCore, the general forward rail
// Related: forward_rs.go -- reactorForwardRS, the route-server forward rail
// Related: peer_static_routes.go -- buildStaticRouteUpdateNew and toPluginParams, the origination rails
package reactor

import (
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// prefixSIDAllowedTo answers RFC 8669 Section 8 for one destination: "Prevent
// any undesired propagation of the BGP Prefix-SID attribute. By default, the
// BGP Prefix-SID is not advertised outside the boundary of a single
// SR/administrative domain that may include one or more ASes. The propagation
// to other ASes MUST be explicitly configured."
//
// The boundary is the SR/administrative domain, and Section 8 says that domain
// "may include one or more ASes". So the AS boundary is NOT the domain
// boundary: RFC 8670's deployment model, which the same section cites, is
// several ASes under one administration, and there the attribute is meant to
// cross every one of those EBGP sessions. Nothing in a session tells ze which
// side of the domain boundary the neighbor sits on, so the answer can only come
// from the operator, per neighbor. That is what "MUST be explicitly configured"
// asks for, and PeerSettings.PropagateSRv6PrefixSID is where the operator says
// it.
//
// An internal peer is inside the domain by construction: it shares this AS, so
// no propagation to another AS happens and the section does not reach it.
//
// Every egress rail asks HERE rather than re-deriving the answer, for the reason
// localPrefAllowedTo (forward_local_pref.go) carries: the rails that re-derived
// the LOCAL_PREF prohibition disagreed with each other for months.
func prefixSIDAllowedTo(isIBGP, propagate bool) bool { return isIBGP || propagate }

// prefixSIDAllowed answers prefixSIDAllowedTo for this peer. It is the form the
// ORIGINATION rails ask in (peer_initial_sync.go): they hold a *Peer rather
// than a peerForwardFacts snapshot.
//
// The AS comparison goes through Peer.IsIBGP, which takes p.mu, because PeerAS
// is written when a dynamic peer establishes (resolveDynamicPeerSettings,
// reactor_dynamic.go). PropagateSRv6PrefixSID is read without it: the leaf is
// set at peer construction and no writer exists, which is the contract on
// Peer.Settings (peer.go). A config edit to the leaf replaces the peer rather
// than mutating it (peerSettingsEqual, reactor_api.go).
func (p *Peer) prefixSIDAllowed() bool {
	return prefixSIDAllowedTo(p.IsIBGP(), p.settings.PropagateSRv6PrefixSID)
}

// prefixSIDOnWire reports whether a Prefix-SID attribute would still reach this
// destination, given the base payload and the operations recorded for it so far.
//
// It folds the operations rather than testing the base alone, because both
// directions are reachable. An egress filter can SET code 40 on a route whose
// source carried none, and applyFactsNextHop (peer_forward_facts.go) already
// suppresses code 40 for RFC 9252 Section 3.3 whenever the next-hop changes.
// Reading the fold is what keeps this rail from recording a second suppress for
// an attribute the first one removed: the accumulator holds eight operations
// before it spills to the heap (filterapi.opsInline), and an EBGP peer with
// next-hop-self and a community filter is already close to that.
//
// Last wins, which is the accumulator's own rule (filterapi.LastSetOrSuppress).
func prefixSIDOnWire(baseHasPrefixSID bool, mods *filterapi.ModAccumulator) bool {
	onWire := baseHasPrefixSID
	for _, op := range mods.Ops() {
		if op.Code != uint8(attribute.AttrPrefixSID) {
			continue
		}
		switch op.Action {
		case filterapi.AttrModSet:
			onWire = true
		case filterapi.AttrModSuppress:
			onWire = false
		}
	}
	return onWire
}

// applyFactsPrefixSID enforces RFC 8669 Section 8 on the forward rails: an
// UPDATE relayed to an EXTERNAL peer that the operator has not placed inside
// the SR domain carries no Prefix-SID attribute.
//
// Called AFTER the egress filter pass and after applyFactsNextHop on both
// rails, so this Suppress is the LAST operation on code 40 and
// filterapi.LastSetOrSuppress makes it win over a filter's Set. Winning is the
// conformant outcome: the prohibition is not a policy a filter may override.
//
// baseHasPrefixSID is computed ONCE per UPDATE by the caller rather than once
// per destination, and the guard below skips the operation when nothing would
// be removed. Recording it unconditionally would force every route to every
// external peer onto the payload-rebuild path, which is the cost the
// route-server fast path exists to avoid.
func applyFactsPrefixSID(f *peerForwardFacts, baseHasPrefixSID bool, mods *filterapi.ModAccumulator) {
	if prefixSIDAllowedTo(!f.isEBGP, f.propagatePrefixSID) {
		return
	}
	if !prefixSIDOnWire(baseHasPrefixSID, mods) {
		return
	}
	mods.Op(uint8(attribute.AttrPrefixSID), filterapi.AttrModSuppress, nil)
}

// rawAttrsWithoutPrefixSID returns raw, less any entry whose attribute code is
// the Prefix-SID, for a destination RFC 8669 Section 8 refuses it.
//
// The origination rails accept pre-built attribute wire bytes from the operator
// (the `attribute` leaf-list under a route's attribute block) and from a
// plugin, and neither is parsed into a typed field for code 40
// (config.parseRawAttributeInto keeps code 40 raw). Section 8 governs the
// attribute, not the route field that carried it, so a hand-written code 40
// crosses the AS boundary under the same condition as a configured one.
//
// It returns raw unchanged when no entry is a Prefix-SID, which is every route
// on every session that never configures one, so the common case allocates
// nothing. An entry shorter than the two octets of flags and code cannot state
// a code and is kept: judging a malformed attribute is the wire builder's
// decision, not this function's.
func rawAttrsWithoutPrefixSID(raw [][]byte) [][]byte {
	if !slices.ContainsFunc(raw, isRawPrefixSID) {
		return raw
	}

	out := make([][]byte, 0, len(raw))
	for _, attr := range raw {
		if isRawPrefixSID(attr) {
			continue
		}
		out = append(out, attr)
	}
	return out
}

// isRawPrefixSID reports whether pre-built attribute wire bytes state the
// Prefix-SID type code. The wire form is flags, then code, so an entry of one
// octet or none states no code and is not one.
func isRawPrefixSID(attr []byte) bool {
	const codeOffset = 1

	if len(attr) <= codeOffset {
		return false
	}
	return attr[codeOffset] == uint8(attribute.AttrPrefixSID)
}
