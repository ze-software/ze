// Design: docs/architecture/route-selection.md -- Step 0, stale depreference
// RFC: rfc/short/rfc9494.md -- Section 4.3, processing a received LLGR_STALE route
// Overview: gr.go -- GR plugin entry point
// Related: gr_egress.go -- the egress decision this step's stale level reaches

package gr

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// llgrStaleLevel is the stale level the ingress step writes for a received
// LLGR_STALE route. It is the RIB's depreference threshold (rib storage
// DepreferenceThreshold), the level a route Ze marks LLGR-stale itself carries,
// so best path and LLGREgressFilter treat the two cases alike.
const llgrStaleLevel uint8 = 2

// LLGRIngressFilter marks a received LLGR_STALE route with stale level 2 in the
// route metadata, so the RIB stores it least preferred and LLGREgressFilter
// governs where it is advertised. It never rejects and never rewrites the
// payload.
//
// RFC 9494 Section 4.3: "A BGP speaker that has advertised the Long-Lived
// Graceful Restart Capability to a neighbor MUST perform the following upon
// receiving a route from that neighbor with the LLGR_STALE community".
//
// It runs at FilterStageProtocol, ahead of every operator policy, so an import
// policy that removes LLGR_STALE cannot hide the route from this step.
func LLGRIngressFilter(src filterapi.PeerFilterInfo, payload []byte, meta map[string]any) (bool, []byte) {
	if llgrStaleFromSource(src, payload) {
		meta["stale"] = llgrStaleLevel
	}
	return true, nil
}

// llgrStaleFromSource reports whether a received payload is a route the RFC 9494
// Section 4.3 procedures govern: Ze sent the LLGR Capability to the neighbor it
// came from AND its COMMUNITIES holds LLGR_STALE.
//
// Whether Ze advertised the capability is src.LLGRAdvertised, which the reactor
// reads from the OPEN this session sent, on the same goroutine that read the
// UPDATE. It is not read from this plugin's record of the sent-OPEN event: that
// event is delivered asynchronously, so a session's first UPDATEs could be judged
// before the record existed. The flag is tested first because it is a field read,
// so a route from a non-LLGR session costs nothing.
//
// Two callers, one predicate: LLGRIngressFilter on receipt, and
// advertisedStaleLevel for a route the relay rebuilt from storage, whose src the
// reactor fills from the source session's OPEN (resolveRelaySource).
func llgrStaleFromSource(src filterapi.PeerFilterInfo, payload []byte) bool {
	if !src.LLGRAdvertised {
		return false
	}
	return payloadCarriesLLGRStale(payload)
}

// payloadCarriesLLGRStale reports whether an UPDATE payload's COMMUNITIES
// attribute holds LLGR_STALE (0xFFFF0006). It reads the attribute section, so an
// NLRI octet run that spells the value is not mistaken for it, and a payload
// that does not parse answers false: the reactor parsed every received UPDATE
// once already (enforceRFC7606), so no route reaches here unread. The loop is
// bounded by the attribute length AttrFind checked, and a trailing partial value
// is not read.
func payloadCarriesLLGRStale(payload []byte) bool {
	sections, err := wire.ParseUpdateSections(payload)
	if err != nil {
		return false
	}
	attrs := sections.Attrs(payload)
	if attrs == nil {
		return false
	}
	_, _, value, found := attribute.AttrFind(attrs, attribute.AttrCommunity)
	if !found {
		return false
	}
	for i := 0; i+4 <= len(value); i += 4 {
		if attribute.Community(binary.BigEndian.Uint32(value[i:])) == attribute.CommunityLLGRStale {
			return true
		}
	}
	return false
}
