// Design: docs/architecture/core-design.md — RIB route building for BGP UPDATEs
// RFC: rfc/short/rfc4271.md — UPDATE format, mandatory attributes, ascending emission order
// RFC: rfc/short/rfc4760.md — MP_REACH_NLRI for non-IPv4-unicast families
// RFC: rfc/short/rfc6793.md — AS4_PATH toward a two-octet peer
// Overview: peer.go — Peer struct and FSM state machine
// Related: announce_build.go — announceAttrs, the shared one-pass announce writer this rail plans into

package reactor

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
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
//
// Returns nil when the route does not fit attrBuf. Every write below goes into ONE
// pooled 4096-byte slot -- attributes growing up from 0, the NLRI parked at the
// tail -- and attrBuf is backing[off:off+4096] out of a 128-slot slab (session.go),
// so its CAP runs into the next peer's buffer. Until this guard, none of the
// writes was bounded: a stored route carrying a long AS_PATH or a large
// LARGE_COMMUNITIES could push `off` past len and return `attrBuf[:off]`, which
// reslices into the neighboring session's memory rather than panicking, and the
// attribute writes themselves could reach the NLRI region and corrupt the prefix
// the UPDATE was announcing. Both rails now take that bound as an explicit region
// argument to announceAttrs.emit (ai/rules/evidence.md).
func buildRIBRouteUpdate(attrBuf []byte, route *rib.Route, localAS uint32, isIBGP, asn4, addPath bool) *message.Update {
	// The destination encoding context for AS_PATH (RFC 6793 ASN width). Shared
	// rather than built per route: it is a pure function of asn4 and is immutable.
	dstCtx := announceDstCtx(asn4)

	// The NLRI is parked at the tail, so it is what bounds the attribute region:
	// reserve it FIRST and let no attribute write reach it.
	routeNLRI := route.NLRI()
	fam := routeNLRI.Family()
	nlriLen := nlri.LenWithContext(routeNLRI, addPath)
	nlriOff := len(attrBuf) - nlriLen
	if nlriLen < 0 || nlriOff < 0 {
		logRIBRouteTooLarge(routeNLRI, len(attrBuf), "nlri")
		return nil
	}

	plan := getAnnouncePlan()
	defer putAnnouncePlan(plan)

	// 1. ORIGIN - use stored or default to IGP
	origin := attribute.OriginIGP
	for _, attr := range route.Attributes() {
		if o, ok := attr.(attribute.Origin); ok {
			origin = o
			break
		}
	}
	plan.add(origin, nil)

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
	// The destination context is passed for AS_PATH ALONE, exactly as the retired
	// attrWriter.writeWithContext did. Every other attribute is encoded
	// context-free, which is what attribute.WriteAttrTo does and therefore what
	// this rail emitted before. Widening the context to AGGREGATOR here would
	// re-encode it two-octet toward an OLD peer -- arguably RFC 6793 Section 4.2.3
	// behavior, but a byte change this convergence has no business making, and one
	// the batch rail (which copies the caller's block verbatim) would not match.
	plan.add(asPath, dstCtx)

	// The NLRI goes into the tail region first: the IPv4 rail returns it as the
	// UPDATE's own NLRI field, and every other family carries it inside MP_REACH.
	// RFC 7911: WriteNLRI uses ADD-PATH encoding when negotiated.
	nlri.WriteNLRI(routeNLRI, attrBuf, nlriOff, addPath)
	nlriData := attrBuf[nlriOff : nlriOff+nlriLen]

	var nlriBytes []byte
	if fam.AFI == family.AFIIPv4 && fam.SAFI == family.SAFIUnicast {
		// 3. NEXT_HOP for IPv4 unicast
		plan.add(plan.nextHopFor(route.NextHop()), nil)
		nlriBytes = nlriData
	} else {
		// RFC 4760: every other family carries its next-hop and NLRI inside
		// MP_REACH_NLRI (type 14), which the writer places at its type-code position.
		plan.add(attribute.NewMPReachNLRI(attribute.AFI(fam.AFI), attribute.SAFI(fam.SAFI),
			[]netip.Addr{route.NextHop()}, nlriData), nil)
	}

	// 4. MED if present.
	for _, attr := range route.Attributes() {
		if med, ok := attr.(attribute.MED); ok {
			plan.add(med, nil)
			break
		}
	}

	// 5. LOCAL_PREF for iBGP - use stored value or default to 100.
	// RFC 4271 Section 5.1.5 keeps it off an external session, which is why a
	// stored LOCAL_PREF is dropped rather than replayed toward one.
	// localPrefAllowedTo (forward_local_pref.go) owns that answer for every rail.
	if localPrefAllowedTo(isIBGP) {
		var localPref attribute.LocalPref = 100
		for _, attr := range route.Attributes() {
			if lp, ok := attr.(attribute.LocalPref); ok {
				localPref = lp
				break
			}
		}
		plan.add(localPref, nil)
	}

	// The stored route's optional attributes. The writer places every contribution
	// at its ascending type-code position (RFC 4271 Section 5), so this loop no
	// longer has to interleave range passes around the attributes injected above --
	// the three writeOptionalAttrs(lo, hi) calls and the hand-placed MP_REACH and
	// AS4_PATH between them are what the shared writer absorbed.
	//
	// The type switch below only EXCLUDES the attributes handled above; everything
	// else is contributed. It used to be an ALLOW-LIST of eight types, so
	// *attribute.AIGP (code 26) and OpaqueAttribute -- what every unknown TRANSITIVE
	// attribute decodes to -- were silently dropped while the batch rail kept them.
	// Fail-open on an unknown code is correct here in a way it would not be for a
	// guard: the alternative is dropping data the peer is entitled to receive
	// (RFC 4271 Section 5 requires an unrecognized transitive attribute to be passed
	// on).
	for _, attr := range route.Attributes() {
		switch attr.(type) {
		case attribute.Origin, *attribute.ASPath, *attribute.NextHop, attribute.LocalPref, attribute.MED:
			// Already contributed above.
			continue
		case *attribute.MPReachNLRI, *attribute.MPUnreachNLRI, *attribute.AS4Path:
			// Injected by this rail at a fixed code (14, 17) or, for MP_UNREACH,
			// meaningless on an announce. A stored copy would duplicate the
			// authoritative one (RFC 7606 Section 3(g)).
			continue
		}
		plan.add(attr, nil)
	}

	// AS4_PATH (RFC 6793 Section 4.2.2): re-announcing to an OLD (2-octet) peer, the
	// AS_PATH above encoded any non-mappable four-octet AS as AS_TRANS; carry the
	// real AS numbers in an AS4_PATH so the peer can reconstruct the path. Built
	// from the same AS_PATH (AS4Path.WriteTo drops confed segments per Section 3).
	if !asn4 && asPathHasNonMappableAS(asPath) {
		plan.add(plan.as4PathFor(asPath.Segments), nil)
	}

	// One bound for every contribution: the region ENDS where the NLRI begins.
	// Passing attrBuf here instead of attrBuf[:nlriOff] would stop the out-of-slot
	// write and still let the attributes overwrite the prefix being announced
	// (ai/rules/evidence.md). attrBuf is backing[off:off+4096] out of a
	// 128-slot slab (session.go), so its CAP runs into the next peer's buffer.
	n, ok := plan.emit(nil, attrBuf[:nlriOff])
	if !ok {
		logRIBRouteTooLarge(routeNLRI, len(attrBuf), "attributes")
		return nil
	}

	return &message.Update{
		PathAttributes: attrBuf[:n],
		NLRI:           nlriBytes,
	}
}

// logRIBRouteTooLarge records a queued-rail build this buffer could not hold. The
// caller drops the route rather than sending a truncated or out-of-slot UPDATE, so
// without this line the route would simply never arrive
// (ai/rules/evidence.md, ai/rules/cli.md).
func logRIBRouteTooLarge(n nlri.NLRI, bufLen int, stage string) {
	routesLogger().Warn("queued route rejected: does not fit the build buffer",
		"family", n.Family(), "nlri", n.String(),
		"buffer-bytes", bufLen, "stage", stage,
		"action", "route not sent to this peer; reduce the route's attributes")
}

// buildWithdrawNLRI builds an UPDATE message to withdraw an NLRI.
// buf is a caller-provided buffer (from buildBufPool).
// For IPv4 unicast, NLRI is written at buf[0:]. For MP families, NLRI is
// written at a high offset to avoid overlap with the MP_UNREACH_NLRI header.
// RFC 4760: IPv4 unicast uses WithdrawnRoutes, others use MP_UNREACH_NLRI.
// RFC 7911: addPath indicates ADD-PATH capability for NLRI encoding.
//
// Returns nil when the NLRI does not fit, for the same reason
// buildRIBRouteUpdate does. The two regions here share one pooled slot and BOTH
// bounds matter: an NLRI longer than the tail region walks past len(buf) (a panic,
// via WriteNLRI's index writes), and an attribute block longer than nlriRegion
// overwrites the NLRI bytes it is still copying FROM -- an overlapping copy that
// corrupts the withdrawal rather than failing it.
func buildWithdrawNLRI(buf []byte, n nlri.NLRI, addPath bool) *message.Update {
	fam := n.Family()
	nlriLen := nlri.LenWithContext(n, addPath)
	if nlriLen < 0 {
		logRIBRouteTooLarge(n, len(buf), "withdraw-nlri")
		return nil
	}

	if fam.AFI == family.AFIIPv4 && fam.SAFI == family.SAFIUnicast {
		// IPv4 unicast: write NLRI at start, use WithdrawnRoutes field
		if nlriLen > len(buf) {
			logRIBRouteTooLarge(n, len(buf), "withdraw-nlri")
			return nil
		}
		nlri.WriteNLRI(n, buf, 0, addPath)
		return &message.Update{
			WithdrawnRoutes: buf[:nlriLen],
		}
	}

	// MP families: write NLRI at high offset so WriteAttrTo can build
	// the MP_UNREACH_NLRI attribute from buf[0:] without overlapping.
	const nlriRegion = 2048
	// MP_UNREACH_NLRI is 4 header + AFI(2) + SAFI(1) + the NLRI, all written from
	// offset 0, so it must stop before nlriRegion or it clobbers its own source.
	const mpUnreachOverhead = 4 + 3
	if nlriRegion+nlriLen > len(buf) || mpUnreachOverhead+nlriLen > nlriRegion {
		logRIBRouteTooLarge(n, len(buf), "withdraw-mp-unreach")
		return nil
	}
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
