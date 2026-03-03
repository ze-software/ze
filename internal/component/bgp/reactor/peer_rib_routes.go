// Design: docs/architecture/core-design.md — RIB route building for BGP UPDATEs
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/rib"
)

// buildRIBRouteUpdate builds an UPDATE message from a RIB route.
// Used for re-announcing routes from Adj-RIB-Out on session re-establishment.
// Rebuilds the full set of required attributes since rib.Route may not store all.
// RFC 7911: addPath indicates ADD-PATH capability for NLRI encoding.
// RFC 6793: asn4 determines 2-byte vs 4-byte AS numbers in AS_PATH.
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
	family := routeNLRI.Family()
	var nlriBytes []byte

	switch {
	case family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIUnicast:
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

		mpReach := &attribute.MPReachNLRI{
			AFI:      attribute.AFI(family.AFI),
			SAFI:     attribute.SAFI(family.SAFI),
			NextHops: []netip.Addr{route.NextHop()},
			NLRI:     nlriData,
		}

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

		// MP_REACH_NLRI at end (after all other path attributes)
		off += attribute.WriteAttrTo(mpReach, attrBuf, off)
	}

	// Copy optional attributes from stored route (communities, etc.)
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
			// Write optional attributes
			off += attribute.WriteAttrTo(attr, attrBuf, off)
		}
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
	family := n.Family()
	nlriLen := nlri.LenWithContext(n, addPath)

	if family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIUnicast {
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
		AFI:  attribute.AFI(family.AFI),
		SAFI: attribute.SAFI(family.SAFI),
		NLRI: nlriData,
	}
	attrLen := attribute.WriteAttrTo(mpUnreach, buf, 0)

	return &message.Update{
		PathAttributes: buf[:attrLen],
	}
}
