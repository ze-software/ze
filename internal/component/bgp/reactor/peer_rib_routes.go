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
//
// Returns nil when the route does not fit attrBuf. Every write below goes into ONE
// pooled 4096-byte slot -- attributes growing up from 0, the NLRI parked at the
// tail -- and attrBuf is backing[off:off+4096] out of a 128-slot slab (session.go),
// so its CAP runs into the next peer's buffer. Until this guard, none of the
// eleven writes was bounded: a stored route carrying a long AS_PATH or a large
// LARGE_COMMUNITIES could push `off` past len and return `attrBuf[:off]`, which
// reslices into the neighboring session's memory rather than panicking, and the
// attribute writes themselves could reach the NLRI region and corrupt the prefix
// the UPDATE was announcing. The batch rail's insertAttrOrdered is the same guard
// for the same slab; this is the queued rail's half (ai/rules/fail-closed-guards.md).
func buildRIBRouteUpdate(attrBuf []byte, route *rib.Route, localAS uint32, isIBGP, asn4, addPath bool) *message.Update {
	// Create encoding context for ASPath encoding
	dstCtx := bgpctx.EncodingContextForASN4(asn4)

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
	w := attrWriter{buf: attrBuf, limit: nlriOff}

	// 1. ORIGIN - use stored or default to IGP
	origin := attribute.OriginIGP
	for _, attr := range route.Attributes() {
		if o, ok := attr.(attribute.Origin); ok {
			origin = o
			break
		}
	}
	w.write(origin)

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
	w.writeWithContext(asPath, dstCtx)

	// Determine NLRI handling based on address family
	var nlriBytes []byte
	// Built in the MP branch below, written after the optional attributes so the
	// emitted order stays ascending by type code (COMMUNITIES 8 < MP_REACH 14).
	var mpReach *attribute.MPReachNLRI

	switch {
	case fam.AFI == family.AFIIPv4 && fam.SAFI == family.SAFIUnicast:
		// 3. NEXT_HOP for IPv4 unicast
		nh := &attribute.NextHop{Addr: route.NextHop()}
		w.write(nh)

		// 4. MED if present (before LOCAL_PREF per RFC order)
		for _, attr := range route.Attributes() {
			if med, ok := attr.(attribute.MED); ok {
				w.write(med)
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
			w.write(localPref)
		}

		// IPv4 unicast: use inline NLRI field
		// RFC 7911: WriteNLRI uses ADD-PATH encoding when negotiated
		// Write NLRI into tail of attrBuf (no overlap with attrs growing from offset 0)
		nlri.WriteNLRI(routeNLRI, attrBuf, nlriOff, addPath)
		nlriBytes = attrBuf[nlriOff : nlriOff+nlriLen]
	default: // non-IPv4-unicast families
		// Other families: MP_REACH_NLRI goes at end (after all other attributes)
		// Write NLRI into tail of attrBuf; WriteAttrTo copies it into attrs region
		nlri.WriteNLRI(routeNLRI, attrBuf, nlriOff, addPath)
		nlriData := attrBuf[nlriOff : nlriOff+nlriLen]

		mpReach = attribute.NewMPReachNLRI(attribute.AFI(fam.AFI), attribute.SAFI(fam.SAFI), []netip.Addr{route.NextHop()}, nlriData)

		// MED if present (before LOCAL_PREF per RFC order)
		for _, attr := range route.Attributes() {
			if med, ok := attr.(attribute.MED); ok {
				w.write(med)
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
			w.write(localPref)
		}
		// MP_REACH_NLRI (type 14) is NOT written here: it must sit between the
		// lower-coded optional attributes (COMMUNITIES 8) and the higher-coded
		// ones (EXT_COMMUNITIES 16). See the ordered writes below.
	}

	// Copy the stored route's optional attributes in ascending type-code order,
	// interleaved with the two attributes this builder injects at fixed codes:
	// MP_REACH_NLRI (14) and AS4_PATH (17).
	//
	// The ordering is load-bearing, not cosmetic. This builder has two siblings
	// that emit the SAME route, and both keep attributes in type-code order:
	// reactor_api_batch.go buildWireModeUpdate places LOCAL_PREF, MP_REACH and
	// AS4_PATH at their type-code position via insertAttrOrdered, and
	// message/update_build.go sorts explicitly, "per RFC 4271 Appendix F.3". Which
	// builder runs is decided by Peer.ShouldQueue() (reactor_api_batch.go:111) --
	// that is, by scheduling: a route queued during initial sync is drained through
	// here, the same route sent after establishment goes through the batch builder.
	// Emitting an attribute at a different position here therefore makes one route
	// encode to two different byte strings depending on timing.
	//
	// Both rails have been caught doing it. The batch builder used to APPEND, so it
	// put LOCAL_PREF (5) and MP_REACH (14) after an EXTENDED_COMMUNITIES (16) the
	// caller supplied -- that is what made test/plugin/ddos-flowspec-announce.ci
	// fail intermittently. This builder then appended AS4_PATH (17) after
	// LARGE_COMMUNITIES (32), which a reviewer caught only because the fix to the
	// other rail made the two disagree.
	//
	// writeOptionalAttrs writes the stored optional attributes whose type code lies
	// in [lo, hi), so the injected attributes can be slotted between the ranges
	// instead of appended after all of them.
	// The type switch below used to be an ALLOW-LIST of eight attribute types, and
	// anything else was dropped. Two shapes matched neither case: *attribute.AIGP
	// (code 26, produced by Builder.SetAIGP and parsed back by AttributesWire.All)
	// and OpaqueAttribute, which is what every unknown TRANSITIVE attribute decodes
	// to (attribute/wire.go). The batch rail copies the caller's block verbatim and
	// keeps both, so the same route lost attributes on one rail and kept them on the
	// other -- selected by Peer.ShouldQueue(), i.e. by scheduling. That is the exact
	// divergence this ordering scheme exists to eliminate, and silently discarding a
	// transitive attribute also violates RFC 4271 Section 5's requirement to pass
	// unrecognized transitive attributes on.
	//
	// So the named cases now only EXCLUDE the attributes written above; everything
	// else is written at its type-code position. Fail-open on an unknown code is
	// correct here in a way it would not be for a guard: the alternative is dropping
	// data the peer is entitled to receive.
	writeOptionalAttrs := func(lo, hi attribute.AttributeCode) {
		for _, attr := range route.Attributes() {
			switch attr.(type) {
			case attribute.Origin, *attribute.ASPath, *attribute.NextHop, attribute.LocalPref, attribute.MED:
				// Already handled above
				continue
			case *attribute.MPReachNLRI, *attribute.MPUnreachNLRI, *attribute.AS4Path:
				// Injected by this builder at a fixed code (14, 17) or, for
				// MP_UNREACH, meaningless on an announce. A stored copy would
				// duplicate the authoritative one (RFC 7606 Section 3(g)).
				continue
			}
			if attr.Code() < lo || attr.Code() >= hi {
				continue
			}
			w.write(attr)
		}
	}

	// ATOMIC_AGGREGATE 6, AGGREGATOR 7, COMMUNITIES 8, ORIGINATOR_ID 9, CLUSTER_LIST 10.
	writeOptionalAttrs(0, attribute.AttrMPReachNLRI)
	if mpReach != nil {
		w.write(mpReach)
	}
	// EXT_COMMUNITIES 16 -- everything between MP_REACH (14) and AS4_PATH (17).
	writeOptionalAttrs(attribute.AttrMPReachNLRI, attribute.AttrAS4Path)

	// AS4_PATH (RFC 6793 §4.2.2): re-announcing to an OLD (2-octet) peer, the
	// AS_PATH written above encoded any non-mappable four-octet AS as AS_TRANS;
	// carry the real AS numbers in an AS4_PATH so the peer can reconstruct the
	// path. Built from the same AS_PATH (AS4Path.WriteTo drops confed segments
	// per §3).
	//
	// It goes at its type-code position, NOT last. The comment here used to say
	// "type code 17 is the highest attribute here", which was false for the same
	// reason the batch builder's identical claim was: IPV6_EXT_COMMUNITIES (25)
	// and LARGE_COMMUNITIES (32) both outrank it, and a stored route can carry
	// either. Appending produced [.. 32 17] on this rail against the batch rail's
	// [.. 17 32] -- the two-rail divergence this builder exists to avoid.
	if !asn4 && asPathHasNonMappableAS(asPath) {
		as4 := &attribute.AS4Path{Segments: asPath.Segments}
		w.write(as4)
	}

	// IPV6_EXT_COMMUNITIES 25, LARGE_COMMUNITIES 32.
	writeOptionalAttrs(attribute.AttrAS4Path, 255)

	// One check for all eleven writes: attrWriter latches the first overflow and
	// no-ops afterwards, so a partially-written block is never emitted.
	if !w.ok() {
		logRIBRouteTooLarge(routeNLRI, len(attrBuf), "attributes")
		return nil
	}

	return &message.Update{
		PathAttributes: attrBuf[:w.off],
		NLRI:           nlriBytes,
	}
}

// attrWriter appends attributes into buf[:limit], latching a failure the first
// time one does not fit and no-opping every write after it.
//
// The latch is the point. buildRIBRouteUpdate has eleven write sites spread over
// four conditional branches, and checking each one at its call site would be
// eleven early returns through code that also has to place the NLRI and keep the
// type-code ordering intact. Latching lets every site stay a single statement and
// puts one honest check at the end, and because a latched writer writes nothing,
// the buffer never holds a half-written attribute that a later reslice could
// expose.
//
// limit, not len(buf): the NLRI is parked at the tail of the SAME buffer, so the
// attribute region ends where the NLRI begins. Bounding on len(buf) would stop the
// out-of-slot write but still let the attributes overwrite the prefix being
// announced.
type attrWriter struct {
	buf   []byte
	limit int
	off   int
	full  bool
}

// write appends attr, or latches full when it does not fit.
func (w *attrWriter) write(attr attribute.Attribute) {
	if w.full {
		return
	}
	n := attrWireLen(attr)
	if n < 0 || w.off+n > w.limit {
		w.full = true
		return
	}
	w.off += attribute.WriteAttrTo(attr, w.buf, w.off)
}

// writeWithContext appends attr under dstCtx (RFC 6793 two- vs four-octet ASN
// encoding), or latches full when it does not fit. The size is taken from the
// same LenWithContext that WriteAttrToWithContext uses to write the header, so the
// bound and the write cannot disagree.
func (w *attrWriter) writeWithContext(attr *attribute.ASPath, dstCtx *bgpctx.EncodingContext) {
	if w.full {
		return
	}
	valueLen := attr.LenWithContext(nil, dstCtx)
	hdrLen := 3
	if valueLen > 255 || attr.Flags().IsExtLength() {
		hdrLen = 4
	}
	if valueLen < 0 || w.off+hdrLen+valueLen > w.limit {
		w.full = true
		return
	}
	w.off += attribute.WriteAttrToWithContext(attr, w.buf, w.off, nil, dstCtx)
}

// ok reports whether every write so far fitted.
func (w *attrWriter) ok() bool { return !w.full }

// logRIBRouteTooLarge records a queued-rail build this buffer could not hold. The
// caller drops the route rather than sending a truncated or out-of-slot UPDATE, so
// without this line the route would simply never arrive
// (ai/rules/fail-closed-guards.md, ai/rules/error-messages.md).
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
