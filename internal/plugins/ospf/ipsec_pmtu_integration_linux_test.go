// VALIDATES: RFC4302-3.3.4-1 at the boundary Ze owns. Ze's AH is a native host
// implementation: transport mode, the host's own OSPFv3 traffic, Linux XFRM in the same
// kernel. RFC 4301 Section 8.2.1 says that for that case "Signaling of the PMTU
// information is internal to the host", and on Linux that signaling is the path MTU the
// kernel reports to a local sender whose egress goes through the AH transform. The state
// that produces it is the AH SA and the outbound require-policy buildIPsecSA and
// buildIPsecPolicies install, so this file installs them through the real installer and
// reads the kernel's own answer.
// PREVENTS: shipping the Section 3.3.4 verdict on a reading of the builder alone. A unit
// test can assert the SAParams and SPParams Ze BUILDS; only the kernel can say whether
// the state Ze installed is what the path-MTU computation reads, and an installer whose
// outbound policy never binds the OSPF flow to the AH transform leaves the local sender
// with the unprotected MTU and every unit test still green.

//go:build integration && linux

package ospf

import (
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
)

// ahHeaderOctets is the IPv6 AH header Linux prepends for the integrity algorithm this
// file configures: 12 octets of ip_auth_hdr plus the 16-octet ICV that
// xfrmAuthNames (ike/dataplane/xfrm_linux.go) asks for with "sha256", rounded up to
// the 8-octet alignment IPv6 requires. The test asserts the kernel took at least this
// much off the path MTU rather than exactly this much: the exact figure is the kernel's
// to choose, and asserting it would make a correct implementation red on the day the
// alignment rule changes.
const ahHeaderOctets = 12 + 16

// pmtuIfIndex is the interface this file installs its policies on and sends over. It is
// the veth pmtuLink creates, so the test reads it only after its first ospfPathMTU call
// has created that link.
//
// A real link rather than loopback, because the destination is ff02::5: loopback carries
// no IPv6 multicast route, a connect to ff02::5 over it answers ENETUNREACH, and the
// proof would become a skip. Measured in the QEMU guest on 2026-09-14.
//
// A veth is also honest for this assertion, which reads no counter.
// `ai/rules/platform-linux.md` requires a real remote egress for a probe that asserts on
// an `ip xfrm` byte counter, because a self-addressed packet matches no peer's policy
// and leaves that counter at zero for a working dataplane and a broken one alike. This
// test sends no packet at all: it asks the kernel which path MTU it computed for the
// bundle, which is the internal signaling the requirement is about.
var pmtuIfIndex int

// pmtuLink creates the veth pair, brings both ends up, and records the ifindex of the
// end this test sends over. It removes the pair when the test that called it ends.
func pmtuLink(t *testing.T) {
	t.Helper()
	const name = "zepmtu0"
	link := &netlink.Veth{Name: name, PeerName: name + "p"}
	if err := netlink.LinkAdd(link); err != nil {
		t.Skipf("creating the veth pair needs CAP_NET_ADMIN: %v", err)
	}
	t.Cleanup(func() {
		_ = netlink.LinkDel(link)
		pmtuIfIndex = 0
	})

	peer, err := netlink.LinkByName(link.PeerName)
	if err != nil {
		t.Fatalf("look up the veth peer %s: %v", link.PeerName, err)
	}
	if err := netlink.LinkSetUp(peer); err != nil {
		t.Fatalf("bring the veth peer up: %v", err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("bring %s up: %v", name, err)
	}

	live, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("look up %s: %v", name, err)
	}

	// The link-local source address is added by hand, with NODAD. A socket sending to
	// ff02::5 needs a source on the link, and a connect made before one exists answers
	// EADDRNOTAVAIL; the kernel's own autoconfigured address stays tentative until
	// duplicate address detection finishes, so waiting for it would put a timer in this
	// test. The address is the one setTransportSource reports to the installer.
	address := &netlink.Addr{
		IPNet: &net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
		Flags: unix.IFA_F_NODAD,
	}
	if err := netlink.AddrAdd(live, address); err != nil {
		t.Fatalf("add the link-local source address to %s: %v", name, err)
	}

	pmtuIfIndex = live.Attrs().Index
}

// ospfPathMTU opens a raw IPv6 OSPF socket over pmtuIfIndex, connects it to dst, and
// returns the path MTU the kernel reports for it. It creates the link on its first call,
// so the caller measures the unprotected MTU before it reads pmtuIfIndex.
//
// A fresh socket is opened for each measurement on purpose: IPV6_MTU reads the
// destination cached on the socket, and a socket connected before a policy was installed
// would answer about the route it resolved then.
func ospfPathMTU(t *testing.T, dst netip.Addr) int {
	t.Helper()
	if pmtuIfIndex == 0 {
		pmtuLink(t)
	}
	fd, err := unix.Socket(unix.AF_INET6, unix.SOCK_RAW, int(ospfv3transport.Protocol))
	if err != nil {
		t.Skipf("a raw IPv6 OSPF socket needs CAP_NET_RAW: %v", err)
	}
	defer func() { _ = unix.Close(fd) }()

	sa := &unix.SockaddrInet6{Addr: dst.As16(), ZoneId: uint32(pmtuIfIndex)}
	if err := unix.Connect(fd, sa); err != nil {
		t.Fatalf("connect a raw OSPF socket to %s over ifindex %d: %v", dst, pmtuIfIndex, err)
	}
	mtu, err := unix.GetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_MTU)
	if err != nil {
		t.Fatalf("IPV6_MTU on the raw OSPF socket: %v", err)
	}
	return mtu
}

// RFC requirement: RFC4302-3.3.4-1 positive -- RFC 4302 Section 3.3.4: "In any case, an
// AH implementation MUST support generation of ICMP PMTU messages (or equivalent
// internal signaling for native host implementations) to minimize the likelihood of
// fragmentation." Ze installs transport-mode AH for the host's own OSPFv3, which is the
// native host case, and RFC 4301 Section 8.2.1 states the obligation for it: "Signaling
// of the PMTU information is internal to the host." The body drives the real installer
// so buildIPsecSA writes the AH state and buildIPsecPolicies writes the outbound
// require-policy, then asks the kernel for the path MTU of a raw OSPF socket over the
// scoped interface, and asserts the reported MTU dropped by at least the AH header the
// configured integrity algorithm needs.
// RFC requirement: RFC4302-3.3.4-1 negative -- the reduction is caused by the state Ze
// installs rather than by the link: the same socket over the same interface, measured
// before the AH interface is configured and again after onInterfaceDown removes it,
// reports the unreduced MTU both times, so an installer that signals a smaller MTU when
// no AH is configured fails here.
func TestAHPathMTUIsSignaledToTheLocalSender(t *testing.T) {
	requireXFRM(t)
	const spi = 0x4552c1
	dst := netip.MustParseAddr("ff02::5")

	clear := ospfPathMTU(t, dst)
	if clear <= ahHeaderOctets {
		t.Fatalf("the unprotected path MTU is %d, which leaves no room for a %d-octet AH header; "+
			"a comparison against it would prove nothing", clear, ahHeaderOctets)
	}

	inst := newIPsecInstaller(nil, nil)
	inst.setTransportSource(func(string) (netip.Addr, int, bool) {
		return netip.MustParseAddr("fe80::1"), pmtuIfIndex, true
	})
	inst.setConfig([]interfaceConfig{{
		Name:  "ipsec-pmtu",
		IPsec: &ipsecInterfaceConfig{SPI: spi, Protocol: "ah", AuthAlgo: "sha256", AuthKey: hexKey(32)},
	}})
	inst.onInterfaceUp(pmtuIfIndex, "ipsec-pmtu")
	if _, ok := inst.status("ipsec-pmtu"); !ok {
		t.Fatal("the installer recorded no AH state, so the kernel holds nothing to compute a path MTU from")
	}

	protected := ospfPathMTU(t, dst)
	if protected >= clear {
		t.Errorf("path MTU with the AH SA and the outbound require-policy installed = %d, "+
			"unprotected = %d; the kernel signaled no reduction, so a local sender would "+
			"build an OSPF packet the AH transform cannot carry whole", protected, clear)
	}
	if clear-protected < ahHeaderOctets {
		t.Errorf("path MTU dropped by %d octets (%d to %d), want at least %d for the AH header "+
			"and the 128-bit truncated ICV of sha256", clear-protected, clear, protected, ahHeaderOctets)
	}

	inst.onInterfaceDown(pmtuIfIndex, "ipsec-pmtu")
	if removed := ospfPathMTU(t, dst); removed != clear {
		t.Errorf("path MTU after the AH state was removed = %d, want the unprotected %d; "+
			"the reduction has to follow the state Ze installs", removed, clear)
	}
}

// xfrmStat reads one counter out of /proc/net/xfrm_stat. The kernel publishes no
// other view of a policy that matched and found no state, and that event is the
// difference between traffic Ze protects and traffic Ze black-holes.
func xfrmStat(t *testing.T, name string) uint64 {
	t.Helper()
	raw, err := os.ReadFile("/proc/net/xfrm_stat")
	if err != nil {
		t.Skipf("/proc/net/xfrm_stat needs CONFIG_XFRM_STATISTICS: %v", err)
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		field, value, found := strings.Cut(line, "\t")
		if !found || strings.TrimSpace(field) != name {
			continue
		}
		count, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		if err != nil {
			t.Fatalf("parse %s from /proc/net/xfrm_stat: %v", name, err)
		}
		return count
	}
	t.Fatalf("/proc/net/xfrm_stat carries no %s counter", name)
	return 0
}

// TestAHPolicyResolvesToTheSAItNames sends one OSPF datagram through the outbound
// require-policy the RFC 4552 installer writes, and asks the kernel whether it found
// the AH state that policy's template names.
//
// The question is whether the wildcard SA of RFC 4552 Section 7 is reachable. Ze
// installs ONE state with Src = Dst = :: plus a {::/0, ::/0, proto 89} state selector,
// and buildIPsecSA's comment says the kernel resolves it for every OSPF destination.
// The kernel hashes a transport-mode state by its destination address, so that claim is
// about a lookup this test makes the kernel perform for real.
//
// XfrmOutNoStates is the counter that answers it. A policy that matches and resolves no
// state advances it and the packet is dropped, which looks exactly like a working
// dataplane from inside Ze: the send succeeds, no error is logged, and every unit test
// over the builder stays green.
func TestAHPolicyResolvesToTheSAItNames(t *testing.T) {
	requireXFRM(t)
	const spi = 0x4552c2
	pmtuLink(t)

	inst := newIPsecInstaller(nil, nil)
	inst.setTransportSource(func(string) (netip.Addr, int, bool) {
		return netip.MustParseAddr("fe80::1"), pmtuIfIndex, true
	})
	inst.setConfig([]interfaceConfig{{
		Name:  "ipsec-resolve",
		IPsec: &ipsecInterfaceConfig{SPI: spi, Protocol: "ah", AuthAlgo: "sha256", AuthKey: hexKey(32)},
	}})
	inst.onInterfaceUp(pmtuIfIndex, "ipsec-resolve")
	if _, ok := inst.status("ipsec-resolve"); !ok {
		t.Fatal("the installer recorded no AH state; the kernel refused the install")
	}
	t.Cleanup(func() { inst.onInterfaceDown(pmtuIfIndex, "ipsec-resolve") })

	before := xfrmStat(t, "XfrmOutNoStates")

	fd, err := unix.Socket(unix.AF_INET6, unix.SOCK_RAW, int(ospfv3transport.Protocol))
	if err != nil {
		t.Skipf("a raw IPv6 OSPF socket needs CAP_NET_RAW: %v", err)
	}
	defer func() { _ = unix.Close(fd) }()
	to := &unix.SockaddrInet6{Addr: netip.MustParseAddr("ff02::5").As16(), ZoneId: uint32(pmtuIfIndex)}
	sendErr := unix.Sendto(fd, make([]byte, 64), 0, to)

	after := xfrmStat(t, "XfrmOutNoStates")
	if after != before {
		t.Errorf("XfrmOutNoStates advanced by %d while sending one OSPF datagram: the outbound "+
			"require-policy matched and the kernel found no AH state for it, so the packet was "+
			"dropped rather than protected. The wildcard Src = Dst = :: state is hashed under the "+
			"unspecified address and a transport-mode lookup for ff02::5 never reaches it",
			after-before)
	}
	if sendErr != nil {
		t.Errorf("sending one OSPF datagram through the AH policy: %v", sendErr)
	}
}
