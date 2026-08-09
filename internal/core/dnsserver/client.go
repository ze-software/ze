// Design: docs/architecture/dns/server-harness.md -- EDNS0/packet client-IP resolution
// RFC: rfc/short/rfc7871.md -- EDNS0 Client Subnet

package dnsserver

import (
	"net"
	"net/netip"

	"github.com/miekg/dns"
)

// ClientIP resolves the client IP used for source-based answer selection, per
// mode. RFC 7871: when present and mode allows it, the EDNS0 client-subnet
// option's network is the client view; otherwise (per mode) the packet
// source is used.
//
// mode is one of "packet" (ignore ECS, always use the packet source),
// "edns0" (ECS only -- no answer without it), or any other value (treated as
// edns0-then-packet: prefer ECS, fall back to the packet source).
func ClientIP(r *dns.Msg, packetSrc netip.Addr, mode string) (netip.Addr, bool) {
	if mode != "packet" {
		if opt := r.IsEdns0(); opt != nil {
			for _, o := range opt.Option {
				if ecs, ok := o.(*dns.EDNS0_SUBNET); ok {
					if a, ok := netip.AddrFromSlice(ecs.Address); ok {
						return a.Unmap(), true
					}
				}
			}
		}
		if mode == "edns0" {
			return netip.Addr{}, false
		}
	}
	if packetSrc.IsValid() {
		return packetSrc.Unmap(), true
	}
	return netip.Addr{}, false
}

// RemoteAddr extracts the client IP from a Peer's remote address (the UDP/TCP
// packet source, before any EDNS0 client-subnet lookup). Unmapped to plain
// IPv4 form when applicable, matching ClientIP's own normalization: a 4-in-6
// address (::ffff:a.b.c.d) fed raw into netip.Prefix.Contains against a v4
// prefix never matches (family mismatch), which would wrongly deny a
// legitimate v4 client to a consumer (e.g. as112's allow-from matcher) that
// uses this result directly instead of routing it through ClientIP.
func RemoteAddr(p Peer) netip.Addr {
	host, _, err := net.SplitHostPort(p.RemoteAddr().String())
	if err != nil {
		return netip.Addr{}
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}
