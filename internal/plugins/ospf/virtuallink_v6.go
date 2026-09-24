// Design: docs/architecture/ospf/ospf-ext-7-virtual-links.md -- OSPFv3 virtual-link endpoint resolution.
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
// neighbor's LSA. Both endpoints must advertise an LA host prefix; otherwise the
// link remains down. Runs under e.mu.
func (e *engine) v6ResolveVirtualEndpointLocked(rt *virtualLinkRuntime) (src, dst netip.Addr, ok bool) {
	if e.lsdb == nil {
		return netip.Addr{}, netip.Addr{}, false
	}
	src, sok := v6RouterGlobalAddr(e.lsdb, rt.cfg.TransitArea, e.cfg.RouterID)
	dst, dok := v6RouterGlobalAddr(e.lsdb, rt.cfg.TransitArea, rt.cfg.RemoteRouterID)
	return src, dst, sok && dok
}

// v6RouterGlobalAddr selects a global LA host prefix from the router's own
// Intra-Area-Prefix-LSA. RFC 5340 Section 4.8.1 selects the first LA prefix;
// Section 4.4.3.9 requires a /128 address for a configured virtual link.
func v6RouterGlobalAddr(db *ospflsdb.LSDB, area types.AreaID, router types.RouterID) (netip.Addr, bool) {
	if db == nil {
		return netip.Addr{}, false
	}
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
		if err != nil || body.ReferencedLSType != ospfv3types.LSTypeRouter || body.ReferencedAdvRouter != ospfv3types.RouterID(router) {
			continue
		}
		for _, p := range body.Prefixes {
			if p.Length != ospfv3types.MaxPrefixLength || p.Options&ospfv3types.OptPrefixLA == 0 {
				continue
			}
			pfx, ok := v6PrefixToNetip(p, afIPv6Unicast)
			if !ok {
				continue
			}
			a := pfx.Addr()
			if !a.Is6() || a.Is4In6() || !a.IsGlobalUnicast() {
				continue
			}
			return a, true
		}
	}
	return netip.Addr{}, false
}

// v6AddVirtualEndpoint advertises a global interface address for configured
// virtual links (RFC 5340 Section 4.4.3.9), even on a transit broadcast segment.
func v6AddVirtualEndpoint(prefixes []ospfv3packet.Prefix, ifaces []ospflsdb.InterfaceInfo) []ospfv3packet.Prefix {
	for _, prefix := range prefixes {
		if prefix.Length == ospfv3types.MaxPrefixLength && prefix.Options.Has(ospfv3types.OptPrefixLA) {
			return prefixes
		}
	}
	var best ospfv3packet.Prefix
	var address netip.Addr
	for _, iface := range ifaces {
		if !v6AdvertiseInterface(iface) || iface.NetworkType == types.NetworkVirtual {
			continue
		}
		for _, host := range v6HostPrefixes(iface) {
			prefix, ok := v6PrefixToNetip(host, afIPv6Unicast)
			if ok && (!address.IsValid() || prefix.Addr().Less(address)) {
				best, address = host, prefix.Addr()
			}
		}
	}
	if address.IsValid() {
		return append(prefixes, best)
	}
	return prefixes
}
