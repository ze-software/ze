// Design: docs/architecture/ospf/ospfv3-5-nssa-redist.md -- OSPFv3 NSSA forwarding address.
// Related: interface_addr.go -- interfaceIPv6ForwardingAddress, the production address source.
// Related: rfc5340_nssa_fa_select_test.go -- the same selection driven through the test seam.
// Related: rfc4302_ah_unknown_spi_linux_test.go -- ahProbeOwnNamespace, the namespace re-exec.
//
// VALIDATES: RFC 5340 Appendix A.4.8, "A global IPv6 address MUST be selected as forwarding
// address for NSSA-LSAs that are to be propagated by NSSA area border routers", over the
// function production calls when no test seam is set: interfaceIPv6ForwardingAddress reads
// the interface's addresses from the kernel through the iface component.
// PREVENTS: a production lookup that returns the link-local or loopback address a kernel
// lists, which only the seam-driven unit exercised before.
//
// No privilege is needed: ahProbeOwnNamespace re-runs the unit in a user and network
// namespace of its own, where the unit is root and the dummy link stays private.

//go:build linux

package ospf

import (
	"net/netip"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink"
)

const rfc5340FALink = "ospffa0"

// rfc5340FAAddrAdd adds prefix to link without duplicate address detection, so the address
// is usable at once.
func rfc5340FAAddrAdd(t *testing.T, link netlink.Link, prefix string) {
	t.Helper()
	addr, err := netlink.ParseAddr(prefix)
	if err != nil {
		t.Fatalf("parse %s: %v", prefix, err)
	}
	addr.Flags = syscall.IFA_F_NODAD
	if err := netlink.AddrAdd(link, addr); err != nil {
		t.Fatalf("add %s: %v", prefix, err)
	}
}

// TestRFC5340NSSAForwardingAddressFromKernelInterface builds a dummy link in a private
// namespace, gives it a link-local address, asks the production lookup, then adds a global
// address and asks again. The loopback link, which carries only ::1, is asked as well.
func TestRFC5340NSSAForwardingAddressFromKernelInterface(t *testing.T) {
	if !ahProbeOwnNamespace(t) {
		return
	}
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("lo lookup: %v", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatalf("lo up: %v", err)
	}
	link := &netlink.Dummy{}
	link.Name = rfc5340FALink
	if err := netlink.LinkAdd(link); err != nil {
		t.Skipf("dummy link unavailable: %v", err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("%s up: %v", rfc5340FALink, err)
	}
	rfc5340FAAddrAdd(t, link, "fe80::1/64")

	// RFC requirement: RFC5340-A.4.8-1 negative -- the production lookup never selects a
	// non-global address: a link carrying only the link-local fe80::1 yields no forwarding
	// address, and neither does the loopback link carrying only ::1.
	if addr, ok := interfaceIPv6ForwardingAddress(rfc5340FALink); ok {
		t.Fatalf("%s with only fe80::1 yielded forwarding address %s", rfc5340FALink, netip.AddrFrom16(addr))
	}
	if addr, ok := interfaceIPv6ForwardingAddress("lo"); ok {
		t.Fatalf("lo with only ::1 yielded forwarding address %s", netip.AddrFrom16(addr))
	}

	// RFC requirement: RFC5340-A.4.8-1 positive -- once the link also carries the global
	// 2001:db8:9::1, the production lookup selects that global address, and not fe80::1.
	rfc5340FAAddrAdd(t, link, "2001:db8:9::1/64")
	addr, ok := interfaceIPv6ForwardingAddress(rfc5340FALink)
	if !ok {
		t.Fatalf("%s with global 2001:db8:9::1 yielded no forwarding address", rfc5340FALink)
	}
	if got, want := netip.AddrFrom16(addr), netip.MustParseAddr("2001:db8:9::1"); got != want {
		t.Fatalf("%s forwarding address = %s, want %s", rfc5340FALink, got, want)
	}
}
