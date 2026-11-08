// VALIDATES: on a real Linux XFRM stack, the if_id 0 entries the IKE engine installs
// for a bound XFRM interface under `vpn ipsec unmatched discard`
// (engine/unmatched_xfrm_interface.go, xfrmInterfaceExemptions) let the router's own
// traffic routed into that interface past the DISCARD catch-all, and nothing else:
// IPv4 through the interface-index entry, IPv6 through the source-address entry, and
// IPv6 from another source still blocked.
// PREVENTS: the defect the lab scenario vti-ipv6-inner-ipv4-outer measured, where a
// ping6 routed into vti0 met the catch-all at if_id 0 and was lost, and an exemption
// that is shaped right in a unit test but not matched by the kernel. Which flow field
// the kernel fills before the if_id 0 lookup is kernel behavior that no reading proves.
//
// VALIDATES too: the interface SPD's own catch-all at the interface's if_id
// (engine/unmatched_xfrm_interface.go, interfaceSPDCatchAll) blocks what reaches the
// interface unmatched and ranks below a Child SA policy there, and a BYPASS at that
// if_id carries no cleartext (TestRFC4301InterfaceSPDDiscardsUnmatchedInKernel,
// TestInterfaceSPDBypassCarriesNoCleartextInKernel).

//go:build linux

package dataplane

import (
	"net"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// The test ranges, carried on an XFRM interface this test creates in its own
// namespace. The catch-all is scoped to them, so nothing else the namespace sends
// meets it.
const (
	xfrmiTestLink    = "zexfi0"
	xfrmiTestIfID    = 7
	xfrmiTestSelf6   = "2001:db8:7::1"
	xfrmiTestCIDR6   = "2001:db8:7::1/64"
	xfrmiTestPeer6   = "2001:db8:7::2"
	xfrmiTestRange6  = "2001:db8:7::/64"
	xfrmiTestOther6  = "2001:db8:8::1"
	xfrmiTestCIDR4   = "198.51.100.65/26"
	xfrmiTestPeer4   = "198.51.100.66"
	xfrmiTestRange4  = "198.51.100.64/26"
	xfrmiTestUDPPort = 9
)

// xfrmOutPolBlock reads the kernel's XfrmOutPolBlock counter, which an outbound lookup
// that met a block policy steps (xfrm_lookup_with_ifid). The write's own error is
// recorded too, but the counter is the instrument.
func xfrmOutPolBlock(t *testing.T) int {
	t.Helper()
	raw, err := os.ReadFile("/proc/net/xfrm_stat")
	if err != nil {
		t.Fatalf("reading /proc/net/xfrm_stat: %v; it is the instrument this test measures with", err)
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "XfrmOutPolBlock" {
			n, convErr := strconv.Atoi(fields[1])
			if convErr != nil {
				t.Fatalf("parsing XfrmOutPolBlock %q: %v", fields[1], convErr)
			}
			return n
		}
	}
	t.Fatal("XfrmOutPolBlock is absent from /proc/net/xfrm_stat")
	return 0
}

// blockedSend writes one datagram from source (nil: the kernel picks it) to target
// and reports how far XfrmOutPolBlock moved.
func blockedSend(t *testing.T, network string, source net.IP, target string) int {
	t.Helper()
	before := xfrmOutPolBlock(t)
	conn, err := net.ListenUDP(network, &net.UDPAddr{IP: source})
	if err != nil {
		t.Fatalf("open a %s socket from %v: %v", network, source, err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.WriteToUDP([]byte("ze-xfrmi-probe"), &net.UDPAddr{IP: net.ParseIP(target), Port: xfrmiTestUDPPort}); err != nil {
		t.Logf("write %v -> %s returned %v", source, target, err)
	}
	return xfrmOutPolBlock(t) - before
}

func addNoDADAddr(t *testing.T, link netlink.Link, cidr string) {
	t.Helper()
	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		t.Fatalf("parse %s: %v", cidr, err)
	}
	addr.Flags = unix.IFA_F_NODAD
	if err := netlink.AddrAdd(link, addr); err != nil {
		t.Fatalf("add %s to %s: %v", cidr, link.Attrs().Name, err)
	}
}

// setupXFRMInterface creates the XFRM interface with both test addresses, plus a
// second IPv6 address on loopback for the negative case, and answers its index.
func setupXFRMInterface(t *testing.T) int {
	t.Helper()
	link := &netlink.Xfrmi{LinkAttrs: netlink.LinkAttrs{Name: xfrmiTestLink}, Ifid: xfrmiTestIfID}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatalf("add xfrm interface %s: %v", xfrmiTestLink, err)
	}
	t.Cleanup(func() { _ = netlink.LinkDel(link) })
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("set %s up: %v", xfrmiTestLink, err)
	}
	addNoDADAddr(t, link, xfrmiTestCIDR6)
	addNoDADAddr(t, link, xfrmiTestCIDR4)
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("find lo: %v", err)
	}
	addNoDADAddr(t, lo, xfrmiTestOther6+"/128")
	made, err := netlink.LinkByName(xfrmiTestLink)
	if err != nil {
		t.Fatalf("read %s back: %v", xfrmiTestLink, err)
	}
	return made.Attrs().Index
}

// installForTest installs p and removes it at cleanup. mustCIDR is
// xfrm_transport_integration_linux_test.go's.
func installForTest(t *testing.T, b *xfrmBackend, p SPParams) {
	t.Helper()
	if err := b.InstallPolicy(p); err != nil {
		t.Fatalf("install %v -> %v dir %d ifindex %d: %v", p.Src, p.Dst, p.Dir, p.IfIndex, err)
	}
	t.Cleanup(func() { _ = b.RemovePolicyParams(p) })
}

// TestXFRMInterfaceExemptionPassesTheCatchAllInKernel measures the four flows.
//
// The exemption shapes are written out here rather than imported, for the reason
// ikeBypassOut gives: the engine imports this package. TestXFRMInterfaceExemptionSelectors
// (engine) pins the engine-side shape; this pins what the kernel does with it.
func TestXFRMInterfaceExemptionPassesTheCatchAllInKernel(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	index := setupXFRMInterface(t)
	b := &xfrmBackend{}
	any4, any6 := mustCIDR(t, "0.0.0.0/0"), mustCIDR(t, "::/0")
	self6 := net.ParseIP(xfrmiTestSelf6)

	// The catch-all at if_id 0, as unmatchedPolicies builds it but scoped to the test
	// ranges.
	installForTest(t, b, SPParams{Src: any6, Dst: mustCIDR(t, xfrmiTestRange6), Dir: SADirOut, Action: SPActionDiscard, Priority: PriorityUnmatched})
	installForTest(t, b, SPParams{Src: any4, Dst: mustCIDR(t, xfrmiTestRange4), Dir: SADirOut, Action: SPActionDiscard, Priority: PriorityUnmatched})

	// VACUITY GUARD: with the catch-all alone, traffic routed into the XFRM interface
	// meets it at if_id 0. Without this, "the counter did not move" below proves nothing.
	if delta := blockedSend(t, "udp6", nil, xfrmiTestPeer6); delta == 0 {
		t.Fatal("an IPv6 write routed into the XFRM interface did not meet the if_id 0 catch-all; this test cannot tell an exemption from an absent block")
	}
	if delta := blockedSend(t, "udp4", nil, xfrmiTestPeer4); delta == 0 {
		t.Fatal("an IPv4 write routed into the XFRM interface did not meet the if_id 0 catch-all; this test cannot tell an exemption from an absent block")
	}

	// The interface-index entries, out, both families.
	installForTest(t, b, SPParams{Src: any4, Dst: any4, Dir: SADirOut, Action: SPActionBypass, Priority: PriorityUnmatchedXFRMInterface, IfIndex: index})
	installForTest(t, b, SPParams{Src: any6, Dst: any6, Dir: SADirOut, Action: SPActionBypass, Priority: PriorityUnmatchedXFRMInterface, IfIndex: index})

	if delta := blockedSend(t, "udp4", nil, xfrmiTestPeer4); delta != 0 {
		t.Errorf("the interface-index entry did not pass IPv4 host traffic routed into the XFRM interface (XfrmOutPolBlock +%d)", delta)
	}
	// Recorded, not asserted: why the source entry exists. IPv6 host output carries no
	// output device into the if_id 0 lookup.
	t.Logf("IPv6 host traffic with the interface-index entry alone: XfrmOutPolBlock +%d", blockedSend(t, "udp6", nil, xfrmiTestPeer6))

	// The source-address entry for the interface's IPv6 address.
	installForTest(t, b, SPParams{
		Src: &net.IPNet{IP: self6, Mask: net.CIDRMask(128, 128)}, Dst: any6, Dir: SADirOut,
		Action: SPActionBypass, Priority: PriorityUnmatchedXFRMInterface,
	})
	if delta := blockedSend(t, "udp6", nil, xfrmiTestPeer6); delta != 0 {
		t.Errorf("the source-address entry did not pass IPv6 host traffic routed into the XFRM interface (XfrmOutPolBlock +%d): the vti-ipv6-inner-ipv4-outer defect", delta)
	}

	// THE EXEMPTION DID NOT WIDEN: IPv6 from another address of the host, routed into
	// the same interface, still meets the catch-all.
	if delta := blockedSend(t, "udp6", net.ParseIP(xfrmiTestOther6), xfrmiTestPeer6); delta == 0 {
		t.Error("IPv6 traffic from an address the XFRM interface does not hold passed the catch-all; the exemption is wider than the interface's own addresses")
	}
}

// xfrmStat reads one counter of /proc/net/xfrm_stat by name.
func xfrmStat(t *testing.T, name string) int {
	t.Helper()
	raw, err := os.ReadFile("/proc/net/xfrm_stat")
	if err != nil {
		t.Fatalf("reading /proc/net/xfrm_stat: %v; it is the instrument this test measures with", err)
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == name {
			n, convErr := strconv.Atoi(fields[1])
			if convErr != nil {
				t.Fatalf("parsing %s %q: %v", name, fields[1], convErr)
			}
			return n
		}
	}
	t.Fatalf("%s is absent from /proc/net/xfrm_stat", name)
	return 0
}

// interfaceSPDCatchAllForTest installs the out entries of the interface SPD's catch-all,
// both families, at the test interface's if_id, as engine/unmatched.go
// spdCatchAllEntries builds them (written out here for the reason
// TestXFRMInterfaceExemptionPassesTheCatchAllInKernel gives).
func interfaceSPDCatchAllForTest(t *testing.T, b *xfrmBackend, action SPAction) {
	t.Helper()
	any4, any6 := mustCIDR(t, "0.0.0.0/0"), mustCIDR(t, "::/0")
	installForTest(t, b, SPParams{Src: any4, Dst: any4, Dir: SADirOut, Action: action, Priority: PriorityUnmatched, IfID: xfrmiTestIfID})
	installForTest(t, b, SPParams{Src: any6, Dst: any6, Dir: SADirOut, Action: action, Priority: PriorityUnmatched, IfID: xfrmiTestIfID})
}

// TestRFC4301InterfaceSPDDiscardsUnmatchedInKernel measures AC-1, AC-3 and A-1 on a
// real XFRM stack. With no if_id 0 entry at all, traffic routed into the XFRM interface
// passes the if_id 0 lookup and is looked up again at the interface's if_id by
// xfrmi_xmit.
//
//   - Without the interface catch-all the kernel drops it for want of a state, and no
//     policy counter moves: nothing the operator can see names the drop (the vacuity
//     leg: the block counter staying still is what the catch-all then changes).
//   - With the catch-all, the write meets it and XfrmOutPolBlock rises (AC-3).
//   - A Child SA policy at PriorityChildSA for one destination outranks it: the write
//     meets the policy, finds no state (XfrmOutNoStates rises), and not the block (A-1).
func TestRFC4301InterfaceSPDDiscardsUnmatchedInKernel(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	setupXFRMInterface(t)
	b := &xfrmBackend{}

	if delta := blockedSend(t, "udp4", nil, xfrmiTestPeer4); delta != 0 {
		t.Fatalf("with no interface catch-all a write into the XFRM interface met a block policy (XfrmOutPolBlock +%d); this test cannot tell the catch-all's block from another", delta)
	}

	interfaceSPDCatchAllForTest(t, b, SPActionDiscard)
	if delta := blockedSend(t, "udp4", nil, xfrmiTestPeer4); delta == 0 {
		t.Error("an IPv4 write routed into the XFRM interface did not meet the interface SPD catch-all (XfrmOutPolBlock unchanged)")
	}
	if delta := blockedSend(t, "udp6", net.ParseIP(xfrmiTestSelf6), xfrmiTestPeer6); delta == 0 {
		t.Error("an IPv6 write routed into the XFRM interface did not meet the interface SPD catch-all (XfrmOutPolBlock unchanged)")
	}

	// A Child SA policy of the interface's SPD for one destination, with its template.
	installForTest(t, b, SPParams{
		Src: mustCIDR(t, "0.0.0.0/0"), Dst: mustCIDR(t, xfrmiTestPeer4+"/32"), Dir: SADirOut,
		Proto: ProtoESP, Mode: ModeTunnel, IfID: xfrmiTestIfID, ReqID: 7,
		TunnelSrc: net.ParseIP("192.0.2.1"), TunnelDst: net.ParseIP("192.0.2.2"),
		Priority: PriorityChildSA,
	})
	noStates := xfrmStat(t, "XfrmOutNoStates")
	if delta := blockedSend(t, "udp4", nil, xfrmiTestPeer4); delta != 0 {
		t.Errorf("a write the Child SA policy covers met the interface catch-all (XfrmOutPolBlock +%d): the catch-all outranks the Child SA", delta)
	}
	if xfrmStat(t, "XfrmOutNoStates") == noStates {
		t.Error("a write the Child SA policy covers did not reach that policy (XfrmOutNoStates unchanged); the test did not measure the ranking")
	}
}

// TestInterfaceSPDBypassCarriesNoCleartextInKernel measures A-5: a template-free BYPASS
// at an interface's if_id passes nothing in the clear. xfrmi_xmit drops a packet whose
// lookup returns no state (`if (!x) goto tx_err_link_failure;`), so the interface counts
// an error and transmits nothing. That is why the interface catch-all discards under
// both values of `unmatched`: a bypass there would be listed as bypass and drop anyway.
func TestInterfaceSPDBypassCarriesNoCleartextInKernel(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	setupXFRMInterface(t)
	b := &xfrmBackend{}
	interfaceSPDCatchAllForTest(t, b, SPActionBypass)

	before := xfrmiStats(t)
	if delta := blockedSend(t, "udp4", nil, xfrmiTestPeer4); delta != 0 {
		t.Fatalf("the bypass entry was not the entry matched (XfrmOutPolBlock +%d)", delta)
	}
	after := xfrmiStats(t)
	if after.TxPackets != before.TxPackets {
		t.Errorf("the XFRM interface transmitted %d packets under a bypass entry; A-5 says it carries none", after.TxPackets-before.TxPackets)
	}
	failed := (after.TxErrors + after.TxCarrierErrors + after.TxDropped) - (before.TxErrors + before.TxCarrierErrors + before.TxDropped)
	if failed == 0 {
		t.Error("the write never reached the XFRM interface (no tx error or drop counted); the test did not measure xfrmi_xmit")
	}
}

// xfrmiStats reads the test interface's link counters.
func xfrmiStats(t *testing.T) netlink.LinkStatistics {
	t.Helper()
	link, err := netlink.LinkByName(xfrmiTestLink)
	if err != nil {
		t.Fatalf("read %s: %v", xfrmiTestLink, err)
	}
	if link.Attrs().Statistics == nil {
		t.Fatalf("%s reports no link statistics", xfrmiTestLink)
	}
	return *link.Attrs().Statistics
}
