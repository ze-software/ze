// Design: docs/architecture/ospf/ospf-ext-4-extended-link-prefix.md -- the RFC 7684 Extended Link Opaque
// LSA (Opaque Type 8) consumer.
// RFC: rfc/short/rfc7684.md -- sec 3 (Extended Link Opaque LSA), sec 3.1 (one Extended Link
// TLV per LSA SHALL; Link Type/ID/Data mirror the Router-LSA link), sec 5 (malformed rules).
// RFC: rfc/short/rfc2328.md App A.4.2 (Router-LSA link encoding).
//
// This is the ext-4 consumer of the ext-1 opaque carrier for Opaque Type 8. Origination reads
// this router's Router-LSA point-to-point and transit links from the LSDB and emits exactly
// one Extended Link Opaque LSA per link, each carrying a single Extended Link TLV whose Link
// Type / Link ID / Link Data equal the Router-LSA link (so a receiver correlates the Extended
// Link LSA with the base Router-LSA link). Reception decodes the body, enforces the sec 5
// malformed rules and the sec 3.1 one-TLV-per-LSA SHALL (uses the first, logs extras), and
// dispatches sub-TLVs to registered codecs. Type 8 is always area-scoped (RFC 7684 sec 3).

package ospf

import (
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// extLinkOnOriginate is the RFC 5250 sec 3 pull-model OnOriginate for Opaque Type 8. Each
// self-LSA pass it returns one Extended Link Opaque LSA per advertised transit/point-to-point
// link plus a withdraw for any link no longer present (AC-13). RFC 7684 sec 3.1: exactly one
// Extended Link TLV per LSA (one LSA per link).
func (e *engine) extLinkOnOriginate(router types.RouterID) []opaqueOrigination {
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()

	o := e.extOrig
	o.mu.Lock()
	defer o.mu.Unlock()

	var out []opaqueOrigination
	desired := map[extLinkKey]extOrig{}
	if cfg.ExtendedLink && e.lsdb != nil {
		for _, v := range e.lsdb.LSAViewsByType(types.LSTypeRouter) {
			if v.AdvertisingRouter != router {
				continue
			}
			rl, err := packet.DecodeRouterLSA(v.Body)
			if err != nil {
				continue
			}
			for _, link := range rl.Links {
				// RFC 7684 sec 3.1: correlate with a real Router-LSA link. Stub (3) and
				// virtual (4) links carry no adjacency to describe, so only point-to-point (1)
				// and transit (2) links get an Extended Link LSA (matching FRR).
				if link.Type != packet.RouterLinkTypeP2P && link.Type != packet.RouterLinkTypeTransit {
					continue
				}
				key := extLinkKey{area: v.Area, linkType: link.Type, linkID: [4]byte(link.LinkID), linkData: link.LinkData}
				id := o.linkIDFor(key)
				tlv := packet.ExtLinkTLV{LinkType: link.Type, LinkID: [4]byte(link.LinkID), LinkData: link.LinkData}
				tlv.SubTLVs = buildLinkSubTLVs(
					extSubTLVContext{Router: router, LinkType: link.Type, LinkID: [4]byte(link.LinkID), LinkData: link.LinkData},
					func() { e.ext.subtlvErrors.With(extRegistryLink).Inc() })
				body := packet.EncodeExtLinkLSA(tlv)
				// Type 8 is area-scope only (RFC 7684 sec 3): use the registered scope (area).
				out = append(out, opaqueOrigination{OpaqueID: id, Area: v.Area, Body: body})
				if _, existed := o.prevLink[key]; !existed {
					e.ext.originations.With(opaqueTypeLabel(packet.ExtLinkOpaqueType)).Inc()
				}
				desired[key] = extOrig{id: id, area: v.Area, scope: OpaqueScopeArea}
			}
		}
	}
	for key, prev := range o.prevLink {
		if _, ok := desired[key]; !ok {
			out = append(out, opaqueOrigination{OpaqueID: prev.id, Area: prev.area, Withdraw: true})
		}
	}
	o.prevLink = desired
	e.refreshExtMetrics()
	return out
}

// extLinkOnReceive is the RFC 5250 sec 3 reception hook for Opaque Type 8. A malformed body is
// counted and not applied (RFC 7684 sec 5); an LSA carrying more than one Extended Link TLV
// uses the first and logs the extras (sec 3.1 SHALL); registered sub-TLVs are dispatched.
func (e *engine) extLinkOnReceive(r opaqueReceived) {
	if r.OpaqueType != packet.ExtLinkOpaqueType {
		return
	}
	if r.Withdrawn {
		// The carrier already purged the LSA from the opaque store; recompute the
		// ze_ospf_ext_link_lsas gauge so a withdrawn link is reflected immediately
		// (mirrors the prefix path), rather than waiting for the next origination pass.
		e.refreshExtMetrics()
		return
	}
	lsa, err := packet.DecodeExtLinkLSA(r.Body)
	if err != nil {
		e.ext.malformed.With(opaqueTypeLabel(r.OpaqueType)).Inc()
		return
	}
	if !lsa.HasLink {
		return
	}
	if lsa.ExtraLinkTLVs > 0 {
		// RFC 7684 sec 3.1: only one Extended Link TLV SHALL be advertised per LSA; use the
		// first and log the extras.
		logger().Warn("ospf ext-link: multiple Extended Link TLVs in one LSA, using first",
			"router", r.AdvertisingRouter.String(), "opaque-id", r.OpaqueID, "extra", lsa.ExtraLinkTLVs)
	}
	for _, s := range lsa.Link.SubTLVs {
		dispatchLinkSubTLV(s, func() { e.ext.subtlvErrors.With(extRegistryLink).Inc() })
	}
	e.refreshExtMetrics()
}
