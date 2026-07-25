// Design: plan/learned/920-mpls-ldp.md -- local FEC origination (AC-3)
// Related: lib.go -- EnsureLocal allocates the per-FEC local label
// Related: register.go -- OnStarted programs egress pop and advertises mappings
//
// An LSR originates label bindings for the FECs it is the egress for: its own
// LSR-ID (the router address other LSRs build LSPs toward) and the prefixes
// directly connected on the interfaces LDP runs on (RFC 5036 Section 1.3 -- the
// FEC "address prefix"). These are advertised downstream-unsolicited once a
// session is operational, and each gets an egress pop entry in the kernel FIB.
//
// LDP runs in-process (a goroutine sharing the daemon's iface backend), so it
// resolves the configured interfaces it advertises FECs for through the shared
// iface resolver (honoring the os-name / mac-match selectors). The all-interfaces
// reachability scan still reads the OS table directly, as it is a system-wide
// query rather than a single configured-name resolution.
package ldp

import (
	"net"
	"net/netip"

	"github.com/ze-software/ze/internal/component/iface"
)

// localFECs returns the FECs this LSR originates: the LSR-ID as a host route
// followed by the connected prefixes, normalised to their network address and
// de-duplicated. Invalid prefixes are dropped; order is stable (LSR-ID first).
func localFECs(lsrID netip.Addr, connected []netip.Prefix) []netip.Prefix {
	seen := make(map[string]struct{}, len(connected)+1)
	out := make([]netip.Prefix, 0, len(connected)+1)
	add := func(p netip.Prefix) {
		if !p.IsValid() {
			return
		}
		p = p.Masked()
		key := p.String()
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	if lsrID.IsValid() {
		add(netip.PrefixFrom(lsrID, lsrID.BitLen()))
	}
	for _, p := range connected {
		add(p)
	}
	return out
}

// prefixFromIPNet converts a net.IPNet to a netip.Prefix, returning false for
// addresses that must not be advertised as FECs: loopback, link-local,
// unspecified and multicast.
func prefixFromIPNet(n *net.IPNet) (netip.Prefix, bool) {
	addr, ok := netip.AddrFromSlice(n.IP)
	if !ok {
		return netip.Prefix{}, false
	}
	ones, _ := n.Mask.Size()
	return advertisablePrefix(addr, ones)
}

// advertisablePrefix normalizes addr/ones to a masked prefix and reports whether
// it may be advertised as a FEC: loopback, link-local, unspecified and multicast
// addresses are excluded (RFC 5036 Section 1.3 address prefixes).
func advertisablePrefix(addr netip.Addr, ones int) (netip.Prefix, bool) {
	addr = addr.Unmap()
	if addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsUnspecified() || addr.IsMulticast() {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(addr, ones).Masked(), true
}

// allConnectedPrefixes returns the connected prefixes across all interfaces, used
// to test whether a candidate next hop is directly reachable. Read errors yield nil.
func allConnectedPrefixes() []netip.Prefix {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	out := make([]netip.Prefix, 0, len(addrs))
	for _, a := range addrs {
		if ipNet, ok := a.(*net.IPNet); ok {
			if p, ok := prefixFromIPNet(ipNet); ok {
				out = append(out, p)
			}
		}
	}
	return out
}

// pickNextHop chooses the IP next hop for a label binding. The session transport
// address is preferred when it is directly connected (the common single-hop case);
// otherwise the first peer interface address (from its Address message) that lies
// on a local connected subnet is used; failing both, the transport address is
// returned as a last resort.
func pickNextHop(transport netip.Addr, peerAddrs []netip.Addr, localPrefixes []netip.Prefix) netip.Addr {
	covered := func(a netip.Addr) bool {
		if !a.IsValid() {
			return false
		}
		for _, p := range localPrefixes {
			if p.Contains(a) {
				return true
			}
		}
		return false
	}
	if covered(transport) {
		return transport
	}
	for _, a := range peerAddrs {
		if covered(a) {
			return a
		}
	}
	return transport
}

// interfacePrefixes returns the advertisable connected prefixes of the named
// interface. A missing interface or read error returns the error to the caller.
func interfacePrefixes(name string) ([]netip.Prefix, error) {
	addrs, err := iface.Addresses(name)
	if err != nil {
		return nil, err
	}
	out := make([]netip.Prefix, 0, len(addrs))
	for _, a := range addrs {
		addr, perr := netip.ParseAddr(a.Address)
		if perr != nil {
			continue
		}
		if p, ok := advertisablePrefix(addr, a.PrefixLength); ok {
			out = append(out, p)
		}
	}
	return out, nil
}
