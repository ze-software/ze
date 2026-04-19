// Design: docs/features/interfaces.md -- VPP FIB readback via ip_route_v2_dump
// Overview: ifacevpp.go -- ListKernelRoutes declaration lives on the backend
// Related: query.go -- sibling VPP dump (SwInterfaceDump)

package ifacevpp

import (
	"fmt"
	"net/netip"
	"strconv"

	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// familyIPv4 / familyIPv6 are the canonical family strings emitted in
// iface.KernelRoute.Family and iface.NeighborInfo.Family. Centralizing
// them here prevents drift between the FIB and neighbor readback paths.
const (
	familyIPv4 = "ipv4"
	familyIPv6 = "ipv6"
)

// Well-known VPP fib_api_source values (u8, MSB of the ip_route_v2 Src
// field). These match ../vpp/src/vnet/fib/fib_source.h; the upstream header
// is the definitive list. Any value not in this map is rendered as its
// decimal string so operators still see the disambiguating tag.
//
// Names are deliberately aligned with the netlink backend's protocol column
// (bgp, static, dhcp, ra, ...) so an operator inspecting `show ip routes`
// sees the same vocabulary on either backend.
var vppFibSrcNames = map[uint8]string{
	0:  "special",
	1:  "default-route",
	2:  "interface",
	3:  "proxy",
	4:  "attached-host",
	5:  "api",
	6:  "cli",
	7:  "lisp",
	8:  "6rd",
	9:  "classify",
	10: "dhcp",
	11: "ip6-nd",
	12: "adj",
	13: "map",
	14: "sixrd",
	15: "mpls",
	16: "ae",
	17: "bier",
	18: "urpf-exempt",
	19: "bgp",
	20: "rr",
	21: "uri",
}

// ListKernelRoutes returns routes from the VPP FIB via ip_route_v2_dump.
// Both IPv4 and IPv6 tables are dumped because iface.KernelRoute exposes a
// family field and operators expect a single call to surface every entry.
// filterPrefix, when non-empty, restricts the output to the exact CIDR
// match on the Destination field; "default" matches 0.0.0.0/0 and ::/0.
// limit == 0 means unbounded; positive values cap the result so a full
// DFZ dump does not produce a gigabytes-sized allocation downstream.
//
// VPP is authoritative on this backend: the "kernel" in the method name is
// preserved from the Backend interface contract, but the data comes from
// the VPP FIB, not from Linux's kernel routing table. Protocol strings are
// translated from VPP's fib_api_source enum to human-readable names
// (bgp/static/dhcp/...) so operators see familiar vocabulary.
func (b *vppBackendImpl) ListKernelRoutes(filterPrefix string, limit int) ([]iface.KernelRoute, error) {
	if err := b.ensureChannel(); err != nil {
		return nil, err
	}

	wantDefault := filterPrefix == "default"
	// Cap the initial allocation against the caller's limit so a full-FIB
	// dump does not allocate an unreasonably large backing array.
	capHint := 256
	if limit > 0 && limit < capHint {
		capHint = limit
	}
	result := make([]iface.KernelRoute, 0, capHint)

	// Dump IPv4 then IPv6. IPRouteV2Dump filters by IPTable{IsIP6}; passing
	// an empty Name / TableID == 0 dumps the default table of that family.
	for _, isIP6 := range []bool{false, true} {
		if limit > 0 && len(result) >= limit {
			break
		}
		req := &ip.IPRouteV2Dump{Table: ip.IPTable{IsIP6: isIP6}}
		ctx := b.ch.SendMultiRequest(req)
		for {
			if limit > 0 && len(result) >= limit {
				// Drain remaining replies so the channel's state machine
				// does not see an abandoned multi-request. GoVPP requires
				// ReceiveReply to be called until lastReplyReceived=true.
				for {
					var drain ip.IPRouteV2Details
					last, drainErr := ctx.ReceiveReply(&drain)
					if drainErr != nil || last {
						break
					}
				}
				break
			}
			details := &ip.IPRouteV2Details{}
			last, err := ctx.ReceiveReply(details)
			if err != nil {
				return nil, fmt.Errorf("ifacevpp: IPRouteV2Dump(ip6=%v): %w", isIP6, err)
			}
			if last {
				break
			}
			entry, ok := routeV2ToKernelRoute(&details.Route, b.names.LookupName)
			if !ok {
				continue
			}
			if filterPrefix != "" {
				if wantDefault {
					if entry.Destination != "0.0.0.0/0" && entry.Destination != "::/0" {
						continue
					}
				} else if entry.Destination != filterPrefix {
					continue
				}
			}
			result = append(result, entry)
		}
	}
	return result, nil
}

// routeV2ToKernelRoute converts a single VPP ip_route_v2 entry to the
// iface.KernelRoute shape. Returns false when the route has no paths
// (malformed reply) or an unusable prefix. lookupName resolves a
// SwIfIndex to a ze interface name; when absent (unmapped VPP ports) the
// Device field is left empty rather than showing an opaque integer index.
func routeV2ToKernelRoute(r *ip.IPRouteV2, lookupName func(uint32) (string, bool)) (iface.KernelRoute, bool) {
	destStr, fam := prefixToString(r.Prefix)
	if destStr == "" {
		return iface.KernelRoute{}, false
	}
	entry := iface.KernelRoute{
		Destination: destStr,
		Protocol:    fibSourceName(r.Src),
		Family:      fam,
	}
	if len(r.Paths) > 0 {
		// Preferred path = lowest Preference value (VPP convention: 0 is
		// preferred, higher numbers are back-ups). For display purposes we
		// surface the first path -- matches how netlink surfaces the
		// primary gateway for ECMP routes.
		p := &r.Paths[0]
		if nh := fibNhString(&p.Nh, p.Proto); nh != "" {
			entry.NextHop = nh
		}
		if p.SwIfIndex != 0 {
			if name, ok := lookupName(p.SwIfIndex); ok {
				entry.Device = name
			}
		}
		entry.Metric = int(p.Weight)
	}
	return entry, true
}

// prefixToString renders an ip_types.Prefix as a CIDR string and family.
// Returns empty strings when the prefix is malformed (wrong family byte).
func prefixToString(p ip_types.Prefix) (string, string) {
	switch p.Address.Af {
	case ip_types.ADDRESS_IP4:
		ip4 := p.Address.Un.GetIP4()
		addr := netip.AddrFrom4(ip4)
		prefix := netip.PrefixFrom(addr, int(p.Len))
		return prefix.String(), familyIPv4
	case ip_types.ADDRESS_IP6:
		ip6 := p.Address.Un.GetIP6()
		addr := netip.AddrFrom16(ip6)
		prefix := netip.PrefixFrom(addr, int(p.Len))
		return prefix.String(), familyIPv6
	}
	return "", ""
}

// fibNhString renders the next-hop address stored in a FibPathNh's union
// according to the path's protocol. Connected/directly-attached paths have
// an all-zero next-hop and produce an empty string (netlink backend uses
// the same convention). Uses AddressUnion.GetIP4 / GetIP6 so the codec's
// versioned decode runs (mirrors neighbor.go); reading XXX_UnionData
// directly would silently misbehave if GoVPP grew a length prefix or
// union tag on the field.
func fibNhString(nh *fib_types.FibPathNh, proto fib_types.FibPathNhProto) string {
	switch proto {
	case fib_types.FIB_API_PATH_NH_PROTO_IP4:
		ip4 := nh.Address.GetIP4()
		if ip4 == (ip_types.IP4Address{}) {
			return ""
		}
		return netip.AddrFrom4(ip4).String()
	case fib_types.FIB_API_PATH_NH_PROTO_IP6:
		ip6 := nh.Address.GetIP6()
		if ip6 == (ip_types.IP6Address{}) {
			return ""
		}
		return netip.AddrFrom16(ip6).String()
	case fib_types.FIB_API_PATH_NH_PROTO_MPLS,
		fib_types.FIB_API_PATH_NH_PROTO_ETHERNET,
		fib_types.FIB_API_PATH_NH_PROTO_BIER:
		// Non-IP next-hops: MPLS label, raw L2, or BIER are not renderable
		// as an IP string; surface as empty so the caller shows only the
		// device and protocol columns.
		return ""
	}
	return ""
}

// fibSourceName maps a VPP fib_api_source value (the ip_route_v2 Src byte)
// to a human name. Unknown values surface as decimal so the operator
// still sees a disambiguating hint.
func fibSourceName(src uint8) string {
	if name, ok := vppFibSrcNames[src]; ok {
		return name
	}
	return strconv.Itoa(int(src))
}
