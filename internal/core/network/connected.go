// Design: docs/architecture/core-design.md — locally connected subnets, read from the host interface table
// Overview: network.go — package doc and the injectable dialer/listener abstractions

package network

import (
	"net"
	"net/netip"
)

// ConnectedPrefixes returns the masked prefix of every address assigned to a
// local interface, both families, in the order the host reports them.
//
// The result names the subnets this host is directly attached to, which is what
// "the BGP speaker shares a common subnet with X" resolves to (RFC 2545 Section
// 3). No address is filtered out: loopback and link-local prefixes are subnets
// the host is attached to, and a caller that wants a narrower set applies its
// own test to the result.
//
// A read error returns nil. A caller MUST read nil as "no shared subnet" rather
// than as "every subnet", because the alternative turns a failed read into a
// permissive answer.
//
// The call reads the kernel interface table, so it belongs at a session or
// configuration boundary, never on a per-message path.
func ConnectedPrefixes() []netip.Prefix {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	out := make([]netip.Prefix, 0, len(addrs))
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		addr, ok := netip.AddrFromSlice(ipNet.IP)
		if !ok {
			continue
		}
		ones, bits := ipNet.Mask.Size()
		if ones == 0 && bits == 0 {
			// Size reports 0,0 for a non-canonical mask it cannot describe as a
			// prefix length. Such an address names no subnet.
			continue
		}
		addr = addr.Unmap()
		if addr.Is4() && bits == 128 {
			// A 4-byte address carrying a 16-byte mask: the length counts bits
			// of the IPv4-mapped form, so rebase it on the 4-byte address.
			ones -= 96
		}
		prefix := netip.PrefixFrom(addr, ones)
		if !prefix.IsValid() {
			continue
		}
		out = append(out, prefix.Masked())
	}
	return out
}

// SharesSubnet reports whether addr lies inside one of prefixes.
//
// An invalid addr and an empty prefix list both report false, so a caller that
// could not read the interface table denies rather than permits.
func SharesSubnet(prefixes []netip.Prefix, addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
