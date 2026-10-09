// Design: docs/architecture/core-design.md -- egress attribute modification on the forward rails
// RFC: rfc/short/rfc8669.md -- the Prefix-SID leaves the SR domain only when configured (Section 8)
// Related: peer_forward_facts.go -- the sibling applyFacts* egress decisions
// Related: forward_local_pref.go -- payloadHasAttr, and the same shape for RFC 4271 Section 5.1.5
// Related: reactor_api_forward.go -- forwardUpdateCore, the general forward rail
// Related: forward_rs.go -- reactorForwardRS, the route-server forward rail
// Related: peer_static_routes.go -- buildStaticRouteUpdateNew and toPluginParams, the origination rails
package reactor

import (
	"encoding/binary"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
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
func prefixSIDAllowedTo(isIBGP, propagate bool) bool {
	return isIBGP || propagate
}

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
// directions are reachable: an egress filter can SET code 40 on a route whose
// source carried none, and a filter can SUPPRESS it. Reading the fold is what
// keeps this rail from recording a second suppress for an attribute a filter
// already removed: the accumulator holds eight operations before it spills to
// the heap (filterapi.opsInline), and an EBGP peer with next-hop-self and a
// community filter is already close to that.
//
// The next-hop change's Remove (applyEgressPrefixSIDNextHop) does not enter
// the fold: it is recorded only for a destination Section 8 allows.
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

// applyEgressPrefixSIDNextHop compares the received and effective outgoing
// next-hop entities after policy, configured rewriting and scope normalization.
// RFC 9252 Section 2: "If the BGP next hop is unchanged during the advertisement,
// the SRv6 Service TLVs, including any unrecognized Types of Sub-TLV and
// Sub-Sub-TLV, SHOULD be propagated further." "In addition, all Reserved fields
// in the TLV, Sub-TLV, or Sub-Sub-TLV MUST be propagated unchanged."
// "If the BGP next hop is changed, the TLVs, Sub-TLVs, and Sub-Sub-TLVs SHOULD
// be updated with the locally allocated SRv6 SID information. Any received
// Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed."
//
// Ze allocates no local Service SID, so a changed entity removes its received
// Service TLVs. RFC 8669 Section 3: "For future extensibility, unknown TLVs MUST
// be ignored and propagated unmodified." The registered attribute handler keeps
// every non-Service TLV. Domain suppression needs no additional Remove operation.
//
// received is always the original input, never a raw policy replacement. base
// names only the destination's actual fields; a configured operation for an
// absent companion field cannot change a route's next hop. RFC 9252 Sections
// 5.1-5.4 and 6 carry services in MP_REACH; a legacy sibling cannot replace that
// field's identity. Legacy-only input retains the existing propagation behavior.
func applyEgressPrefixSIDNextHop(f *peerForwardFacts, mods *filterapi.ModAccumulator, received, base nextHopValue, baseHasPrefixSID bool) {
	if !prefixSIDAllowedTo(!f.isEBGP, f.propagatePrefixSID) {
		return
	}
	if !prefixSIDOnWire(baseHasPrefixSID, mods) {
		return
	}
	previous, emitted := received.legacy, base.legacy
	written, _ := modsNextHop(mods)
	// The carrier can exist without a usable intermediate address: a raw
	// policy field may be repaired by the configured rewrite before admission.
	// Match egressNextHopWithheld's presence test, not base.mp.IsValid().
	if base.mpFamily != (family.Family{}) {
		previous, emitted = received.mp, base.mp
		if written.mp.IsValid() {
			emitted = written.mp
		}
		// RFC 2545 Section 3 adds or removes an optional Link-Local for the
		// same Global entity; it is not a new next hop. In RFC 4659 Section
		// 3.2.1.1's absent-global pair, the Link-Local identifies the entity.
		if previous.IsUnspecified() {
			previous = received.mpLL
		}
		if emitted.IsUnspecified() {
			emitted = base.mpLL
			if written.mp.IsValid() {
				emitted = written.mpLL
			}
		}
	} else if emitted.IsValid() && written.legacy.IsValid() {
		emitted = written.legacy
	}
	if !emitted.IsValid() {
		return
	}
	if emitted == previous {
		return
	}
	mods.Op(uint8(attribute.AttrPrefixSID), filterapi.AttrModRemove, srv6ServiceTLVTypes[:])
}

// srv6ServiceTLVTypes is the Buf of the Remove operation a next-hop change
// records for code 40 (applyEgressPrefixSIDNextHop): the TLV types RFC 9252
// Section 2 ties to the next hop. Package-level storage outlives every forward
// call, so no destination allocates an operation buffer.
var srv6ServiceTLVTypes = [...]byte{attribute.PrefixSIDTLVSRv6L3Service, attribute.PrefixSIDTLVSRv6L2Service}

// prefixSIDTLVHeaderOctets is a Prefix-SID TLV's Type (1 octet) and Length
// (2 octets), RFC 8669 Section 3.
const prefixSIDTLVHeaderOctets = 3

// prefixSIDNextHopHandler plans attribute 40 for one destination.
//
// A Set or Suppress behaves as the generic handler's does. An AttrModRemove
// names TLV types, one per octet of its Buf, and the attribute is rewritten
// without the TLVs of those types, every other TLV kept byte for byte in its
// received order:
//
//	+--------+----------------+-----------------+
//	| Type 1 | Length 2 (N)   | Value N octets  |   one TLV, RFC 8669 Section 3
//	+--------+----------------+-----------------+
//	offset 0  offset 1..2      offset 3..3+N-1
//
// RFC 8669 Section 3: "For future extensibility, unknown TLVs MUST be ignored
// and propagated unmodified." RFC 9252 Section 2: "If the BGP next hop is
// changed, the TLVs, Sub-TLVs, and Sub-Sub-TLVs SHOULD be updated with the
// locally allocated SRv6 SID information." Ze allocates no local SRv6 SID, so
// the next-hop change removes the Service TLVs and nothing else.
//
// Kept TLVs of the received attribute are fragments over the source bytes,
// written once into the destination's buffer by the rebuild, so the common
// case copies nothing beyond the rebuild itself. Kept TLVs of a value a filter
// SET are copied into the edit arena: the plan has no fragment over part of an
// operation's bytes, and a filter setting code 40 on a next-hop-changing
// destination is the rare case.
//
// When no TLV remains the attribute leaves: RFC 8669 Section 3 defines the
// attribute as a set of TLVs, so an empty one carries nothing to propagate.
func prefixSIDNextHopHandler() filterapi.AttrModHandler {
	return func(p *filterapi.AttrPlan) {
		ops := p.Ops()
		setIdx, suppress := lastSetOrSuppress(ops)
		if suppress {
			p.Drop()
			return
		}
		// A Remove recorded before the last Set was aimed at a value the Set
		// replaced, so only the ones after it apply.
		removeFrom := setIdx + 1
		if !prefixSIDRemovesAny(ops[removeFrom:]) {
			if setIdx < 0 {
				keepOrDrop(p)
				return
			}
			p.Op(setIdx)
			p.Emit(prefixSIDFlags, prefixSIDCodeByteWire)
			return
		}

		value := p.Value()
		fromSource := setIdx < 0
		if !fromSource {
			if ops[setIdx].GenIdx != 0 {
				// No producer generates code 40, so a generated value cannot be
				// walked here; refusing the route is louder than emitting it
				// with the TLVs the next hop no longer supports.
				p.Fail()
				return
			}
			value = ops[setIdx].Buf
		}
		if value == nil {
			p.Drop()
			return
		}

		kept := 0
		for off := 0; off < len(value); {
			tlvOctets, ok := prefixSIDTLVOctets(value[off:])
			if !ok {
				// RFC 8669 Section 6: a Prefix-SID "containing a TLV length that
				// would extend beyond the end of the attribute" is malformed, and
				// the speaker "MUST ignore the received BGP Prefix-SID attribute
				// and not advertise it to other BGP peers."
				p.Drop()
				return
			}
			if !prefixSIDTLVRemoved(ops[removeFrom:], value[off]) {
				if fromSource {
					p.Keep(off, tlvOctets)
				} else {
					p.New(value[off : off+tlvOctets])
				}
				kept++
			}
			off += tlvOctets
		}
		if kept == 0 {
			p.Drop()
			return
		}
		p.Emit(prefixSIDFlags, prefixSIDCodeByteWire)
	}
}

// prefixSIDFlags and prefixSIDCodeByteWire are the header the rewritten
// attribute is emitted under: Optional, Transitive (RFC 8669 Section 3).
const (
	prefixSIDFlags        = byte(attribute.FlagOptional | attribute.FlagTransitive)
	prefixSIDCodeByteWire = byte(attribute.AttrPrefixSID)
)

// prefixSIDTLVOctets returns the whole size of the TLV at the start of b, and
// false when b is too short to hold its header or the value it declares.
func prefixSIDTLVOctets(b []byte) (int, bool) {
	if len(b) < prefixSIDTLVHeaderOctets {
		return 0, false
	}
	n := prefixSIDTLVHeaderOctets + int(binary.BigEndian.Uint16(b[1:prefixSIDTLVHeaderOctets]))
	if n > len(b) {
		return 0, false
	}
	return n, true
}

// prefixSIDRemovesAny reports whether any operation removes TLVs.
func prefixSIDRemovesAny(ops []filterapi.AttrOp) bool {
	for i := range ops {
		if ops[i].Action == filterapi.AttrModRemove {
			return true
		}
	}
	return false
}

// prefixSIDTLVRemoved reports whether a Remove operation names tlvType.
func prefixSIDTLVRemoved(ops []filterapi.AttrOp, tlvType byte) bool {
	for i := range ops {
		if ops[i].Action != filterapi.AttrModRemove {
			continue
		}
		if slices.Contains(ops[i].Buf, tlvType) {
			return true
		}
	}
	return false
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
