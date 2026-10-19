// Design: docs/architecture/wire/attributes.md — AS_PATH segment types
// RFC: rfc/short/rfc9774.md — Deprecation of AS_SET and AS_CONFED_SET in BGP
// Related: session_write.go — writeUpdateBody, the one writer that calls the guard
// Related: session_aigp.go — the RFC 7311 send guard this mirrors
//
// The send half of RFC 9774 Section 3. Every UPDATE a producer hands the session
// (relay, API batch, static route, plugin raw wire, RIB replay) reaches writeUpdateBody,
// so one guard there covers all of them and no list of producers is written.

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	wireu "github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// asSetSendPhrase leads the send-guard warning, so an operator's log scanner matches
// one string. It names the consequence, the cause and the fix.
const asSetSendPhrase = "route withdrawn instead of advertised: RFC 9774 Section 3 forbids advertising an AS_SET or AS_CONFED_SET; remove the set segment from the route's AS_PATH or AS4_PATH"

// transmittedPathHoldsSet reports whether an UPDATE body about to be written carries an
// AS_SET or AS_CONFED_SET segment in its AS_PATH or its AS4_PATH.
//
// RFC 9774 Section 3: "BGP speakers: MUST NOT advertise BGP UPDATE messages containing
// AS_SETs or AS_CONFED_SETs". This is the question the writer asks before writing.
//
// The AS_PATH is read at the width the peer decodes it with: two octets unless 4-octet AS
// support was negotiated. A path misframed at that width answers false here, because the
// peer cannot read a set out of it either and handles it under RFC 7606 Section 7.2. An
// unparsable body answers false for the same reason: the writer's own checks refuse it.
// The walk reads attribute headers once, judging every AS_PATH and AS4_PATH it meets,
// and allocates nothing, so the clean path costs one header pass per UPDATE. The caller MUST hold writeMu, which protects negotiated.
func (s *Session) transmittedPathHoldsSet(body []byte) bool {
	sections, err := wireu.ParseUpdateSections(body)
	if err != nil {
		return false
	}
	attrs := sections.Attrs(body)
	if len(attrs) == 0 {
		return false
	}
	asnOctets := 2
	if neg := s.negotiated; neg != nil && neg.ASN4 {
		asnOctets = 4
	}
	// Every occurrence is judged, not only the first: a producer that writes an AS_PATH
	// twice (a configured raw AS_PATH beside the typed one) still puts the set in the
	// message, and Section 3 forbids advertising a message containing one.
	// The loop is bounded by the attribute field: Next stops at its end or at the
	// first header that overruns it.
	it := attribute.NewAttrIterator(attrs)
	for {
		code, _, value, ok := it.Next()
		if !ok {
			return false
		}
		//exhaustive:ignore // only the two path attributes can hold a segment
		switch code {
		case attribute.AttrASPath:
			if _, set := message.ASPathSetSegment(value, asnOctets); set {
				return true
			}
		case attribute.AttrAS4Path:
			if _, set := message.ASPathSetSegment(value, 4); set {
				return true
			}
		}
	}
}

// writeUpdateAsWithdrawal writes the withdrawal form of an UPDATE whose path holds a set,
// in place of the UPDATE.
//
// RFC 9774 Section 3: "MUST NOT advertise BGP UPDATE messages containing AS_SETs or
// AS_CONFED_SETs". A withdrawal rather than a silent drop: when the producer replaced a
// clean route with a set variant, the peer still holds the clean one, and only a
// withdrawal removes it. Each withdrawal body re-enters writeUpdateBody, whose ownership
// filter suppresses the withdrawal of a prefix the peer never held, so a route that was
// never advertised puts nothing on the wire. A withdrawal body carries no AS_PATH, so the
// re-entry cannot reach this function again: the recursion is one level deep.
//
// The withdrawn routes are counted on ze_bgp_as_set_withdrawn_total and reported in one
// warning per wellKnownScanLogInterval for this peer, because the producer, a config or a
// plugin, is the operator's to fix and nothing else tells them the route never left.
// The caller MUST hold writeMu.
func (s *Session) writeUpdateAsWithdrawal(body []byte) (bool, error) {
	routes := s.announcedRouteCount(body)
	if s.prefixMetrics != nil {
		s.prefixMetrics.asSetWithdrawn.Add(float64(routes))
	}
	if emit, suppressed := s.asSetSendLog.allow(s.clock.Now().UnixNano()); emit {
		sessionLogger().Warn(asSetSendPhrase,
			"peer", s.settings.Address, "routes", routes, "suppressed-since-last", suppressed)
	}
	for _, withdrawal := range message.SynthesizeWithdrawFamilies(body, nil, s.mpAddPathSendWire) {
		if _, err := s.writeUpdateBody(withdrawal, false); err != nil {
			return false, err
		}
	}
	return false, nil
}

// announcedRouteCount counts the routes an UPDATE body announces: the IPv4 unicast NLRI
// field and the MP_REACH_NLRI NLRI, each walked with its family's registered framing. It
// feeds a counter, so a section it cannot frame counts what it walked before the fault.
func (s *Session) announcedRouteCount(body []byte) int {
	sections, err := wireu.ParseUpdateSections(body)
	if err != nil {
		return 0
	}
	ipv4 := family.Family{AFI: family.AFI(attribute.AFIIPv4), SAFI: family.SAFI(attribute.SAFIUnicast)}
	routes := countPrefixEntries(familyKey(ipv4), sections.NLRI(body), s.mpAddPathSend(ipv4), false)
	_, _, reach, found := attribute.AttrFind(sections.Attrs(body), attribute.AttrMPReachNLRI)
	if !found {
		return routes
	}
	mp, err := attribute.ParseMPReachNLRI(reach)
	if err != nil {
		return routes
	}
	fam := family.Family{AFI: family.AFI(mp.AFI), SAFI: family.SAFI(mp.SAFI)}
	return routes + countPrefixEntries(familyKey(fam), mp.NLRI, s.mpAddPathSend(fam), false)
}

// mpAddPathSend returns whether the NLRI this session writes for a family lead with a
// path identifier. It reads the send encoding context, the one declaration the UPDATE
// splitter and the Adj-RIB-Out ownership filter in writeUpdateBody frame with, so the
// withdrawal names the path the peer holds. The caller MUST hold writeMu, which
// protects sendCtxID.
func (s *Session) mpAddPathSend(fam family.Family) bool {
	return bgpctx.Registry.Get(s.sendCtxID).AddPath(fam)
}

// mpAddPathSendWire is mpAddPathSend for an AFI and SAFI read out of a body, the form
// message.SynthesizeWithdrawFamilies asks for when it frames the MP NLRIs it withdraws.
func (s *Session) mpAddPathSendWire(afi uint16, safi uint8) bool {
	return s.mpAddPathSend(family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)})
}
