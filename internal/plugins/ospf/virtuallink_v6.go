// Design: plan/learned/1043-ospf-ext-7-virtual-links.md -- OSPFv3 virtual-link endpoint resolution.
// Related: virtual_link.go -- the AF-neutral manager that consumes this resolver.
// RFC: rfc/short/rfc5340.md (sec 2.9 routed global src/dst, App A.4.10 Intra-Area-Prefix-LSA)
//
// RFC 5340 sec 2.9: OSPFv3 virtual-link packets are unicast to the neighbor's GLOBAL IPv6
// address from a local GLOBAL source, routed through the transit area (hop limit > 1), not
// link-local. Neither address is in the transit-area SPF Result (its next hops are the
// transit link-locals), so this file resolves both from the transit area's
// Intra-Area-Prefix-LSAs (App A.4.10): the local source from this router's own LSA and the
// destination from the neighbor's.

package ospf

import (
	"net/netip"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// v6ResolveVirtualEndpointLocked resolves the routed OSPFv3 virtual-link source and
// destination for rt from the transit area's Intra-Area-Prefix-LSAs (RFC 5340 sec 2.9): the
// local GLOBAL source from this router's own LSA, the neighbor's GLOBAL destination from the
// neighbor's LSA. ok is false until both are learned intra-area (the link then keeps its
// prior addresses and re-resolves on the next transit SPF run). Runs under e.mu.
func (e *engine) v6ResolveVirtualEndpointLocked(rt *virtualLinkRuntime) (src, dst netip.Addr, ok bool) {
	if e.lsdb == nil {
		return netip.Addr{}, netip.Addr{}, false
	}
	src, sok := v6RouterGlobalAddr(e.lsdb, rt.cfg.TransitArea, e.cfg.RouterID)
	dst, dok := v6RouterGlobalAddr(e.lsdb, rt.cfg.TransitArea, rt.cfg.RemoteRouterID)
	return src, dst, sok && dok
}

// v6RouterGlobalAddr returns a routable GLOBAL IPv6 address advertised by router in area's
// Router-referencing Intra-Area-Prefix-LSAs (RFC 5340 App A.4.10). A host (/128) prefix is
// preferred; otherwise the first global prefix's address is used.
// NOTE: a non-/128 prefix yields the subnet address; the exact on-wire host address is
// refined from the neighbor's Link-LSA under QEMU (the routed raw send is Linux-only).
func v6RouterGlobalAddr(db *ospflsdb.LSDB, area types.AreaID, router types.RouterID) (netip.Addr, bool) {
	if db == nil {
		return netip.Addr{}, false
	}
	var best netip.Addr
	for _, h := range db.Summary(area) {
		if h.Age.IsMaxAge() || ospfv3types.LSType(h.Type) != ospfv3types.LSTypeIntraAreaPrefix || h.AdvertisingRouter != router {
			continue
		}
		lsa, ok := db.LookupLSA(area, h.Key())
		if !ok {
			continue
		}
		decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
		if err != nil {
			continue
		}
		body, err := decoded.DecodeIntraAreaPrefix()
		if err != nil || body.ReferencedLSType != ospfv3types.LSTypeRouter {
			continue
		}
		for _, p := range body.Prefixes {
			pfx, ok := v6PrefixToNetip(p, afIPv6Unicast)
			if !ok {
				continue
			}
			a := pfx.Addr()
			if !a.Is6() || !a.IsGlobalUnicast() {
				continue
			}
			if pfx.Bits() == 128 {
				return a, true
			}
			if !best.IsValid() {
				best = a
			}
		}
	}
	return best, best.IsValid()
}
