// Design: docs/features/interfaces.md -- Kernel routing-table readback
// Overview: ifacenetlink.go -- package hub
// Related: neighbor_linux.go -- sibling OS read (ARP/ND)
// Related: show_linux.go -- sibling OS read (interfaces)

//go:build linux

package ifacenetlink

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"syscall"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// Well-known Linux rtm_protocol values that map to readable names. Any
// other protocol number is rendered as its decimal string so operators
// still see the disambiguating tag.
//
// See linux/rtnetlink.h RTPROT_* and Ze's producer-specific rtproto IDs.
var rtProtoNames = map[int]string{
	syscall.RTPROT_UNSPEC:   "unspec",
	syscall.RTPROT_REDIRECT: "redirect",
	syscall.RTPROT_KERNEL:   "kernel",
	syscall.RTPROT_BOOT:     "boot",
	syscall.RTPROT_STATIC:   "static",
	// Dynamic-routing daemon ranges: Quagga/FRR allocates 186..254.
	11:  "zebra", // RTPROT_ZEBRA (legacy Quagga)
	42:  "ra",    // kernel accept_ra default routes
	16:  "dhcp",  // RTPROT_DHCP
	186: "bgp",   // RTPROT_BGP (FRR)
	187: "isis",  // RTPROT_ISIS
	188: "ospf",  // RTPROT_OSPF
	189: "rip",   // RTPROT_RIP
	192: "eigrp", // RTPROT_EIGRP
	193: "babel", // RTPROT_BABEL
}

// ListKernelRoutes returns up to `limit` routes from the kernel's main
// table (netlink RTM_GETROUTE across all families). filterPrefix (when
// non-empty) restricts the output to the single CIDR match on the
// Destination field -- use "default" to match the 0.0.0.0/0 / ::/0
// entries. An empty filter returns every route. limit == 0 means
// unbounded; positive values stop populating the Go-side slice once
// reached so a full-DFZ dump does not produce a gigabytes-sized
// allocation downstream.
//
// Connected routes (no Gw, proto=kernel) are returned with an empty
// NextHop and the egress device filled in; the operator distinguishes
// them by protocol=kernel.
func (b *netlinkBackend) ListKernelRoutes(filterPrefix string, limit int) ([]iface.KernelRoute, error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("iface: route list: %w", err)
	}

	// Build index->name map for device resolution.
	links, lerr := listLinks()
	if lerr != nil {
		return nil, fmt.Errorf("iface: link list: %w", lerr)
	}
	idxName := make(map[int]string, len(links))
	for _, l := range links {
		idxName[l.Attrs().Index] = l.Attrs().Name
	}

	// Cap the allocation capacity against the caller's limit so a
	// full-DFZ dump does not allocate a 1M-entry backing array on the
	// way to returning 100k rows. Zero means "no cap".
	capHint := len(routes)
	if limit > 0 && limit < capHint {
		capHint = limit
	}
	wantDefault := filterPrefix == "default"
	result := make([]iface.KernelRoute, 0, capHint)
	for i := range routes {
		if limit > 0 && len(result) >= limit {
			break
		}
		dst := routes[i].Dst
		destStr := "default"
		if dst != nil {
			destStr = dst.String()
		}

		if filterPrefix != "" {
			if wantDefault {
				if dst != nil {
					continue
				}
			} else if destStr != filterPrefix {
				continue
			}
		}

		fam := "ipv4" //nolint:goconst // AFI label; see ifacenetlink.go for siblings
		if dst != nil && dst.IP.To4() == nil {
			fam = "ipv6" //nolint:goconst // AFI label; see ifacenetlink.go for siblings
		} else if dst == nil && routes[i].Gw != nil && routes[i].Gw.To4() == nil {
			fam = "ipv6" //nolint:goconst // AFI label; see ifacenetlink.go for siblings
		}

		entry := iface.KernelRoute{
			Destination: destStr,
			Device:      idxName[routes[i].LinkIndex],
			Protocol:    protocolName(int(routes[i].Protocol)),
			Metric:      routes[i].Priority,
			Family:      fam,
		}
		if routes[i].Gw != nil {
			entry.NextHop = routes[i].Gw.String()
		}
		if routes[i].Src != nil {
			entry.Source = routes[i].Src.String()
		}
		result = append(result, entry)
	}
	return result, nil
}

// RouteLookup performs a longest-prefix-match lookup for the given
// destination IP using netlink RouteGet.
func (b *netlinkBackend) RouteLookup(dest netip.Addr) (map[string]any, error) {
	routes, err := netlink.RouteGet(net.IP(dest.AsSlice()))
	if err != nil {
		return nil, fmt.Errorf("route lookup: %w", err)
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("route lookup: no route to %s", dest)
	}
	r := routes[0]

	result := map[string]any{
		"destination": dest.String(),
	}
	if r.Dst != nil {
		prefix, ok := netip.AddrFromSlice(r.Dst.IP)
		if ok {
			ones, _ := r.Dst.Mask.Size()
			result["prefix"] = netip.PrefixFrom(prefix, ones).String()
		}
	} else {
		if dest.Is4() {
			result["prefix"] = "0.0.0.0/0"
		} else {
			result["prefix"] = "::/0"
		}
	}
	if r.Gw != nil {
		gw, ok := netip.AddrFromSlice(r.Gw)
		if ok {
			result["next-hop"] = gw.String()
		}
	}
	if r.LinkIndex > 0 {
		link, linkErr := netlink.LinkByIndex(r.LinkIndex)
		if linkErr == nil {
			result["interface"] = link.Attrs().Name
		}
	}
	result["protocol"] = protocolName(int(r.Protocol))
	result["metric"] = r.Priority
	result["table"] = r.Table

	return result, nil
}

// AddressIsLocal reports whether dest is an address this box terminates. The
// kernel classifies a route to an address the box owns as RTN_LOCAL (it resolves
// in the local routing table); a forwarded destination resolves as RTN_UNICAST via
// a gateway or connected route. Used to route DDoS mitigation (INPUT vs FORWARD).
// A destination with no route is reported not-local (remote is the fail-safe).
func (b *netlinkBackend) AddressIsLocal(dest netip.Addr) (bool, error) {
	routes, err := netlink.RouteGet(net.IP(dest.AsSlice()))
	if err != nil {
		return false, fmt.Errorf("address-is-local lookup for %s: %w", dest, err)
	}
	if len(routes) == 0 {
		return false, nil
	}
	return routes[0].Type == unix.RTN_LOCAL, nil
}

func protocolName(p int) string {
	if name, ok := rtproto.Name(p); ok {
		return name
	}
	if name, ok := rtProtoNames[p]; ok {
		return name
	}
	return strconv.Itoa(p)
}
