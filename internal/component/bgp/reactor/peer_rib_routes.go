// Design: docs/architecture/core-design.md — RIB route building for BGP UPDATEs
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// buildRIBRouteUpdate builds an UPDATE message from a RIB route.
// Used for re-announcing routes from Adj-RIB-Out on session re-establishment.
// Rebuilds the full set of required attributes since rib.Route may not store all.
// RFC 7911: addPath indicates ADD-PATH capability for NLRI encoding.
// RFC 6793: asn4 determines 2-byte vs 4-byte AS numbers in AS_PATH.
//
// A stored AS_PATH is emitted VERBATIM, with no local-AS prepend. That is
// deliberate, not an oversight: the only production caller that supplies one is
// the API announce path (rib.NewRouteWithASPath in reactor_api_batch.go), and the
// path it stores has ALREADY had the local AS applied by buildBatchASPath. RFC
// 4271 Section 5.1.2 is enforced there, once, for this rail. Prepending again
// here would double it. The prepend arm below is for locally-originated routes
// that carry no stored path at all.
//
// NOT pinned by TestRIBRouteUpdate_* in reactor_as4path_test.go, despite the
// name: those assert only the AS4_PATH attribute, and every case passes
// localAS == asns[0], so they cannot tell a verbatim emission from a conditional
// prepend. Anyone changing this arm needs a new test that inspects AS_PATH.
func buildRIBRouteUpdate(attrBuf []byte, route *rib.Route, localAS uint32, isIBGP, asn4, addPath bool) *message.Update {
	off := 0

	// Create encoding context for ASPath encoding
	dstCtx := bgpctx.EncodingContextForASN4(asn4)

	// 1. ORIGIN - use stored or default to IGP
	origin := attribute.OriginIGP
	for _, attr := range route.Attributes() {
		if o, ok := attr.(attribute.Origin); ok {
			origin = o
			break
		}
	}
	off += attribute.WriteAttrTo(origin, attrBuf, off)

	// 2. AS_PATH - use stored or build appropriate default
	storedASPath := route.ASPath()
	hasStoredASPath := storedASPath != nil && len(storedASPath.Segments) > 0

	var asPath *attribute.ASPath
	switch {
	case hasStoredASPath:
		asPath = storedASPath
	case isIBGP || localAS == 0:
		// iBGP or LocalAS not set: empty AS_PATH
		asPath = &attribute.ASPath{Segments: nil}
	default:
		// eBGP: prepend local AS
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{{
				Type: attribute.ASSequence,
				ASNs: []uint32{localAS},
			}},
		}
	}
	off += attribute.WriteAttrToWithContext(asPath, attrBuf, off, nil, dstCtx)

	// Determine NLRI handling based on address family
	routeNLRI := route.NLRI()
	fam := routeNLRI.Family()
	var nlriBytes []byte
	// Built in the MP branch below, written after the optional attributes so the
	// emitted order stays ascending by type code (COMMUNITIES 8 < MP_REACH 14).
	var mpReach *attribute.MPReachNLRI

	switch {
	case fam.AFI == family.AFIIPv4 && fam.SAFI == family.SAFIUnicast:
		// 3. NEXT_HOP for IPv4 unicast
		nh := &attribute.NextHop{Addr: route.NextHop()}
		off += attribute.WriteAttrTo(nh, attrBuf, off)

		// 4. MED if present (before LOCAL_PREF per RFC order)
		for _, attr := range route.Attributes() {
			if med, ok := attr.(attribute.MED); ok {
				off += attribute.WriteAttrTo(med, attrBuf, off)
				break
			}
		}

		// 5. LOCAL_PREF for iBGP - use stored value or default to 100
		if isIBGP {
			var localPref attribute.LocalPref = 100
			for _, attr := range route.Attributes() {
				if lp, ok := attr.(attribute.LocalPref); ok {
					localPref = lp
					break
				}
			}
			off += attribute.WriteAttrTo(localPref, attrBuf, off)
		}

		// IPv4 unicast: use inline NLRI field
		// RFC 7911: WriteNLRI uses ADD-PATH encoding when negotiated
		// Write NLRI into tail of attrBuf (no overlap with attrs growing from offset 0)
		nlriLen := nlri.LenWithContext(routeNLRI, addPath)
		nlriOff := len(attrBuf) - nlriLen
		nlri.WriteNLRI(routeNLRI, attrBuf, nlriOff, addPath)
		nlriBytes = attrBuf[nlriOff : nlriOff+nlriLen]
	default: // non-IPv4-unicast families
		// Other families: MP_REACH_NLRI goes at end (after all other attributes)
		// Write NLRI into tail of attrBuf; WriteAttrTo copies it into attrs region
		nlriLen := nlri.LenWithContext(routeNLRI, addPath)
		nlriOff := len(attrBuf) - nlriLen
		nlri.WriteNLRI(routeNLRI, attrBuf, nlriOff, addPath)
		nlriData := attrBuf[nlriOff : nlriOff+nlriLen]

		mpReach = attribute.NewMPReachNLRI(attribute.AFI(fam.AFI), attribute.SAFI(fam.SAFI), []netip.Addr{route.NextHop()}, nlriData)

		// MED if present (before LOCAL_PREF per RFC order)
		for _, attr := range route.Attributes() {
			if med, ok := attr.(attribute.MED); ok {
				off += attribute.WriteAttrTo(med, attrBuf, off)
				break
			}
		}

		// LOCAL_PREF for iBGP - use stored value or default to 100
		if isIBGP {
			var localPref attribute.LocalPref = 100
			for _, attr := range route.Attributes() {
				if lp, ok := attr.(attribute.LocalPref); ok {
					localPref = lp
					break
				}
			}
			off += attribute.WriteAttrTo(localPref, attrBuf, off)
		}
		// MP_REACH_NLRI (type 14) is NOT written here: it must sit between the
		// lower-coded optional attributes (COMMUNITIES 8) and the higher-coded
		// ones (EXT_COMMUNITIES 16). See the ordered writes below.
	}

	// Copy the stored route's optional attributes, in ascending type-code order
	// around MP_REACH_NLRI (type 14): codes below 14 first, then MP_REACH, then
	// codes above it.
	//
	// The ordering is load-bearing, not cosmetic. This builder has two siblings
	// that emit the SAME route, and both keep attributes in type-code order:
	// reactor_api_batch.go buildWireModeUpdate appends MP_REACH after the
	// lower-coded attributes (see appendAnnounceAS4Path's comment on type-code
	// order), and message/update_build.go sorts explicitly, "per RFC 4271
	// Appendix F.3". Which builder runs is decided by Peer.ShouldQueue()
	// (reactor_api_batch.go:111) -- that is, by scheduling: a route queued during
	// initial sync is drained through here, the same route sent after
	// establishment goes through the batch builder. Emitting MP_REACH at a
	// different position here therefore made one route encode to two different
	// byte strings depending on timing.
	writeOptionalAttrs := func(below bool) {
		for _, attr := range route.Attributes() {
			switch attr.(type) {
			case attribute.Origin, *attribute.ASPath, *attribute.NextHop, attribute.LocalPref, attribute.MED:
				// Already handled above
				continue
			case attribute.Communities,
				attribute.ExtendedCommunities, attribute.LargeCommunities,
				attribute.IPv6ExtendedCommunities,
				attribute.AtomicAggregate, *attribute.Aggregator,
				attribute.OriginatorID, attribute.ClusterList:
				if (attr.Code() < attribute.AttrMPReachNLRI) != below {
					continue
				}
				off += attribute.WriteAttrTo(attr, attrBuf, off)
			}
		}
	}

	// ATOMIC_AGGREGATE 6, AGGREGATOR 7, COMMUNITIES 8, ORIGINATOR_ID 9, CLUSTER_LIST 10.
	writeOptionalAttrs(true)
	if mpReach != nil {
		off += attribute.WriteAttrTo(mpReach, attrBuf, off)
	}
	// EXT_COMMUNITIES 16, IPV6_EXT_COMMUNITIES 25, LARGE_COMMUNITIES 32.
	writeOptionalAttrs(false)

	// AS4_PATH (RFC 6793 §4.2.2): re-announcing to an OLD (2-octet) peer, the
	// AS_PATH written above encoded any non-mappable four-octet AS as AS_TRANS;
	// carry the real AS numbers in an AS4_PATH so the peer can reconstruct the
	// path. Emitted last -- type code 17 is the highest attribute here -- and
	// built from the same AS_PATH (AS4Path.WriteTo drops confed segments per §3).
	if !asn4 && asPathHasNonMappableAS(asPath) {
		as4 := &attribute.AS4Path{Segments: asPath.Segments}
		off += attribute.WriteAttrTo(as4, attrBuf, off)
	}

	return &message.Update{
		PathAttributes: attrBuf[:off],
		NLRI:           nlriBytes,
	}
}

// buildWithdrawNLRI builds an UPDATE message to withdraw an NLRI.
// buf is a caller-provided buffer (from buildBufPool).
// For IPv4 unicast, NLRI is written at buf[0:]. For MP families, NLRI is
// written at a high offset to avoid overlap with the MP_UNREACH_NLRI header.
// RFC 4760: IPv4 unicast uses WithdrawnRoutes, others use MP_UNREACH_NLRI.
// RFC 7911: addPath indicates ADD-PATH capability for NLRI encoding.
func buildWithdrawNLRI(buf []byte, n nlri.NLRI, addPath bool) *message.Update {
	fam := n.Family()
	nlriLen := nlri.LenWithContext(n, addPath)

	if fam.AFI == family.AFIIPv4 && fam.SAFI == family.SAFIUnicast {
		// IPv4 unicast: write NLRI at start, use WithdrawnRoutes field
		nlri.WriteNLRI(n, buf, 0, addPath)
		return &message.Update{
			WithdrawnRoutes: buf[:nlriLen],
		}
	}

	// MP families: write NLRI at high offset so WriteAttrTo can build
	// the MP_UNREACH_NLRI attribute from buf[0:] without overlapping.
	const nlriRegion = 2048
	nlri.WriteNLRI(n, buf, nlriRegion, addPath)
	nlriData := buf[nlriRegion : nlriRegion+nlriLen]

	mpUnreach := &attribute.MPUnreachNLRI{
		AFI:  attribute.AFI(fam.AFI),
		SAFI: attribute.SAFI(fam.SAFI),
		NLRI: nlriData,
	}
	attrLen := attribute.WriteAttrTo(mpUnreach, buf, 0)

	return &message.Update{
		PathAttributes: buf[:attrLen],
	}
}
