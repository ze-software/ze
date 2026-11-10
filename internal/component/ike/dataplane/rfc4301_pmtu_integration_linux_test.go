// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Path MTU of a Child SA
// Related: xfrm_linux.go -- xfrmStateFromParams installs the tunnel-mode ESP state whose overhead the kernel subtracts
//
// VALIDATES: RFC 4301 Section 8.2 over the tunnel-mode ESP SA pair Ze's XFRM
// backend installs, between network namespaces with a narrower link in the
// middle. The kernel holds the SA's PMTU on the outer route and derives the inner
// one from it, and these tests read what it does with the state Ze installed:
//
//   - Section 8.2.1: an ICMP Fragmentation Needed about an ESP packet updates the
//     PMTU of the outer route (esp4_err and xfrmi4_err in Linux, which look the SA up
//     by SPI and call ipv4_update_pmtu); the inner MTU then accounts for the ESP
//     header, the IV, the ICV, the padding and the outer IP header
//     (xfrm_state_mtu, net/xfrm/xfrm_state.c); a DF-set IPv4 packet over it is
//     discarded with an ICMP Fragmentation Needed, a DF-clear one is fragmented
//     after encryption and forwarded with no ICMP, and an IPv6 one is discarded with
//     a Packet Too Big (xfrmi_xmit2, net/xfrm/xfrm_interface_core.c).
//   - Section 8.2.2: the learned PMTU expires after net.ipv4.route.mtu_expires
//     (__ip_rt_update_pmtu, net/ipv4/route.c, stamps the exception with it), the
//     SA's PMTU returns to the first-hop link MTU, and the next oversized packet
//     relearns it.
//
// PREVENTS: an ESP state whose overhead or DF handling the kernel cannot account
// for, which black-holes every packet between the inner and the outer MTU while
// the SA reports up.
// METHOD: five namespaces in a line, host - a - r - b, with the r-b link at
// pmtuNarrowMTU. a and b each carry an XFRM interface bound to the Child SA's
// if_id and install their half of a tunnel-mode ESP SA pair (outer a <-> b) and
// catch-all policies of both inner families through xfrmBackend, as a
// route-based tunnel does. host sends UDP probes of pmtuProbeOctets to inner
// addresses on b's XFRM interface, and the tests read the PMTU each namespace
// learned with an RTM_GETROUTE, the probes b received, and the IP counters.
//
// Each test runs in a user and network namespace of its own (userns.Enter), so it
// needs no host privilege. net.ipv4.route.mtu_expires is per network namespace
// and the kernel exports it to a namespace a user namespace owns
// (sysctl_route_net_init keeps the whole table there). The IPv6 route table is
// exported there with max_size alone (ipv6_route_sysctl_table_size), so the
// IPv6-outer aging leg needs the initial user namespace and lives in
// rfc4301_pmtu_ipv6_integration_linux_test.go.

//go:build linux

package dataplane

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

const (
	pmtuIfID  = 0x2b
	pmtuReqID = 0x2b2b
	pmtuSPIAB = 0x2b2b0001 // carries a's packets to b
	pmtuSPIBA = 0x2b2b0002 // carries b's packets to a
	pmtuPort  = 47030

	// pmtuLinkMTU is every link's MTU but r-b, and so the first-hop link MTU of
	// the SA at a.
	pmtuLinkMTU = 1500
	// pmtuNarrowMTU is the r-b link, the narrowest hop of the outer path.
	pmtuNarrowMTU = 1400
	// pmtuProbeOctets is the IP total length of every probe. It fits the inner
	// MTU a derives from pmtuLinkMTU (1446 over an IPv4 outer, 1426 over an IPv6
	// one), so a sends it into the SA until it learns the narrow hop, and it
	// exceeds the inner MTU a derives from pmtuNarrowMTU.
	pmtuProbeOctets = 1420

	// pmtuInnerMTUOuter4 is the inner MTU of the SA once a learned
	// pmtuNarrowMTU over an IPv4 outer. xfrm_state_mtu (net/xfrm/xfrm_state.c)
	// for a tunnel-mode ESP state: header_len is the ESP header (8) plus the
	// rfc4106(gcm(aes)) IV (8) plus the outer IPv4 header (20), esp_init_state in
	// net/ipv4/esp4.c; the ICV is the 16 octets xfrmStateFromParams installs
	// (ICVLen 128); the GCM block size aligns to 4; the trailer takes 2.
	// ((1400 - 36 - 16) &^ 3) - 2 = 1346.
	pmtuInnerMTUOuter4 = 1346
	// pmtuInnerMTUOuter6 is the same with the outer IPv6 header (40), from
	// esp6_init_state in net/ipv6/esp6.c: ((1400 - 56 - 16) &^ 3) - 2 = 1326.
	pmtuInnerMTUOuter6 = 1326
)

// The addressing plan. Every link carries both families, so one topology serves
// an IPv4 and an IPv6 outer pair and both inner families.
var (
	pmtuHostLAN4, pmtuHostGW4 = net.ParseIP("192.0.2.1"), net.ParseIP("192.0.2.254")
	pmtuHostLAN6, pmtuHostGW6 = net.ParseIP("2001:db8:1::1"), net.ParseIP("2001:db8:1::fe")
	pmtuA4, pmtuRLeft4        = net.ParseIP("198.51.100.1"), net.ParseIP("198.51.100.254")
	pmtuA6, pmtuRLeft6        = net.ParseIP("2001:db8:a::1"), net.ParseIP("2001:db8:a::fe")
	pmtuB4, pmtuRRight4       = net.ParseIP("203.0.113.1"), net.ParseIP("203.0.113.254")
	pmtuB6, pmtuRRight6       = net.ParseIP("2001:db8:b::1"), net.ParseIP("2001:db8:b::fe")

	// Inner addresses on b's XFRM interface. Each probe of a test goes to its
	// own address, because a host that learned a PMTU for one destination
	// refuses or fragments the next probe to it locally, before a sees it.
	pmtuInnerDF4   = net.ParseIP("10.99.0.1")
	pmtuInnerNoDF4 = net.ParseIP("10.99.0.2")
	pmtuInner6     = net.ParseIP("fd00:99::1")
)

// pmtuSide names which end of the SA pmtuInstall installs.
type pmtuSide int

const (
	pmtuSideA pmtuSide = iota + 1
	pmtuSideB
)

// pmtuNet is the five-namespace line. base is the namespace the test thread
// started in; every helper returns the thread to it.
type pmtuNet struct {
	base, host, a, r, b netns.NsHandle
	// ifID is the Child SA's if_id: pmtuIfID for a route-based tunnel over XFRM
	// interfaces, 0 for a policy-based one in the node-wide SPD (a peer with no
	// vti bind, resolveIfID in engine/established.go).
	ifID           uint32
	outerA, outerB net.IP
}

// VALIDATES: RFC4301-8.2.1-1 cases 1 and 3, and RFC4301-5.1-3. A PMTU message
// about the SA's ESP packets updates the SA's PMTU, with the ESP and outer-header
// overhead taken off, and an oversized DF-set IPv4 packet and an oversized IPv6
// packet are discarded and answered with a PMTU ICMP carrying that inner MTU.
// RFC requirement: RFC4301-8.2.1-1 positive -- a DF-set IPv4 packet and an IPv6 packet over the SA's learned PMTU are not delivered, and the sender learns the inner MTU the ESP and outer-header overhead leave.
// RFC requirement: RFC4301-5.1-3 positive -- an ICMP Fragmentation Needed about the SA's ESP packets sets the SA's outer PMTU, and the inner MTU the sender is told takes the ESP and outer-header overhead off it.
func TestXFRMTunnelPMTUDiscardsAndSignalsOverTheSAPMTU(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	encapNetns(t)
	pmtuDiscardAndSignal(t, true, true, pmtuInnerMTUOuter4)
}

// VALIDATES: RFC4301-8.2.1-1 cases 1 and 3, and RFC4301-5.1-3, over an IPv6 outer
// pair, where the outer header the inner MTU accounts for is 40 octets and the PMTU
// message is an ICMPv6 Packet Too Big (esp6_err, net/ipv6/esp6.c).
// RFC requirement: RFC4301-8.2.1-1 positive -- over an IPv6-outer SA, a DF-set IPv4 packet and an IPv6 packet over the SA's learned PMTU are not delivered, and the sender learns the inner MTU the ESP and outer IPv6 header overhead leave.
// RFC requirement: RFC4301-5.1-3 positive -- an ICMPv6 Packet Too Big about an IPv6-outer SA's ESP packets sets the SA's outer PMTU, and the inner MTU the sender is told takes the ESP and outer IPv6 header overhead off it.
func TestXFRMTunnelPMTUDiscardsAndSignalsOverAnIPv6OuterSA(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	encapNetns(t)
	pmtuDiscardAndSignal(t, false, true, pmtuInnerMTUOuter6)
}

// VALIDATES: RFC4301-8.2.1-1 cases 1 and 3, and RFC4301-5.1-3, for a policy-based SA
// (if_id 0, a peer with no vti bind): the PMTU check then runs in ip_forward and
// ip6_forward against the bundle MTU rather than in xfrmi_xmit2.
// RFC requirement: RFC4301-8.2.1-1 positive -- over a policy-based SA, a DF-set IPv4 packet and an IPv6 packet over the SA's learned PMTU are not delivered, and the sender learns the inner MTU the ESP and outer-header overhead leave.
// RFC requirement: RFC4301-5.1-3 positive -- an ICMP Fragmentation Needed about a policy-based SA's ESP packets sets the SA's outer PMTU, and the inner MTU the sender is told takes the ESP and outer-header overhead off it.
func TestXFRMPolicyPMTUDiscardsAndSignalsOverTheSAPMTU(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	encapNetns(t)
	pmtuDiscardAndSignal(t, true, false, pmtuInnerMTUOuter4)
}

// VALIDATES: the same over a policy-based SA with an IPv6 outer pair.
// RFC requirement: RFC4301-8.2.1-1 positive -- over a policy-based IPv6-outer SA, a DF-set IPv4 packet and an IPv6 packet over the SA's learned PMTU are not delivered, and the sender learns the inner MTU the ESP and outer IPv6 header overhead leave.
func TestXFRMPolicyPMTUDiscardsAndSignalsOverAnIPv6OuterSA(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	encapNetns(t)
	pmtuDiscardAndSignal(t, false, false, pmtuInnerMTUOuter6)
}

// pmtuDiscardAndSignal builds the line with the outer family given, teaches a the
// narrow hop, and checks Section 8.2.1 cases 1 and 3 against innerMTU, the inner
// MTU xfrm_state_mtu leaves from pmtuNarrowMTU for that outer family.
func pmtuDiscardAndSignal(t *testing.T, outer4, routeBased bool, innerMTU int) {
	t.Helper()
	n := pmtuBuild(t, outer4, routeBased)
	sink4 := pmtuListen(t, n, n.b, pmtuInnerDF4)
	sink6 := pmtuListen(t, n, n.b, pmtuInner6)

	// Probe 1 fits the inner MTU a holds from the first-hop link, so a encrypts
	// it. An IPv4 outer packet copies the inner DF bit (xfrm4_tunnel_encap_add,
	// net/xfrm/xfrm_output.c) and no router fragments an IPv6 one, so r cannot
	// forward it onto the narrow link, and its PMTU message about an ESP packet
	// reaches a.
	if err := pmtuSend(t, n, pmtuInnerDF4, true); err != nil {
		t.Fatalf("send the first DF-set probe from the host: %v", err)
	}
	pmtuWaitRouteMTU(t, n, n.a, n.outerB, pmtuNarrowMTU, "a learning the narrow hop of the outer path from r's ICMP about the ESP packet")

	// Case 1. Probe 2 is over the inner MTU a now derives, so a discards it and
	// sends the host an ICMP Fragmentation Needed carrying that MTU.
	if err := pmtuSend(t, n, pmtuInnerDF4, true); err != nil {
		t.Fatalf("send the second DF-set probe from the host: %v", err)
	}
	// The inner MTU depends on the OUTER family only, not on the inner one: an IPv4
	// probe over an IPv6-outer SA learns innerMTU (pmtuInnerMTUOuter6), because
	// esp6_init_state counts the 40-octet outer IPv6 header in header_len.
	pmtuWaitRouteMTU(t, n, n.host, pmtuInnerDF4, innerMTU, "the host learning the SA's inner MTU from a's ICMP Fragmentation Needed")
	if pmtuReceived(t, sink4) {
		t.Fatalf("b received a DF-set probe of %d octets over an SA whose inner MTU is %d; it should have been discarded",
			pmtuProbeOctets, innerMTU)
	}

	// Case 3. a already holds the narrow outer PMTU, so the first IPv6 probe is
	// over the inner MTU and a answers it with a Packet Too Big.
	if err := pmtuSend(t, n, pmtuInner6, true); err != nil {
		t.Fatalf("send the IPv6 probe from the host: %v", err)
	}
	pmtuWaitRouteMTU(t, n, n.host, pmtuInner6, innerMTU, "the host learning the SA's inner MTU from a's Packet Too Big")
	if pmtuReceived(t, sink6) {
		t.Fatalf("b received an IPv6 probe of %d octets over an SA whose inner MTU is %d; it should have been discarded",
			pmtuProbeOctets, innerMTU)
	}
}

// VALIDATES: RFC4301-8.2.1-1 case 2. An oversized DF-clear IPv4 packet over the
// SA's learned PMTU is not discarded and draws no PMTU ICMP: a fragments the ESP
// packet after encryption and b reassembles and delivers it.
// RFC requirement: RFC4301-8.2.1-1 negative -- a DF-clear IPv4 packet over the SA's learned PMTU is neither discarded nor answered with a PMTU ICMP: a fragments it after encryption and b delivers it.
// RFC requirement: RFC4301-5.1-3 negative -- a DF-clear IPv4 packet over the SA's learned PMTU is not discarded and draws no PMTU ICMP: a fragments it after encryption and b delivers it.
func TestXFRMTunnelPMTUFragmentsADFClearPacket(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	encapNetns(t)
	pmtuFragmentDFClear(t, true)
}

// VALIDATES: RFC4301-8.2.1-1 case 2 for a policy-based SA (if_id 0).
// RFC requirement: RFC4301-8.2.1-1 negative -- over a policy-based SA, a DF-clear IPv4 packet over the SA's learned PMTU is neither discarded nor answered with a PMTU ICMP: a fragments it after encryption and b delivers it.
func TestXFRMPolicyPMTUFragmentsADFClearPacket(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	encapNetns(t)
	pmtuFragmentDFClear(t, false)
}

// pmtuFragmentDFClear checks Section 8.2.1 case 2 over an IPv4 outer pair.
func pmtuFragmentDFClear(t *testing.T, routeBased bool) {
	t.Helper()
	n := pmtuBuild(t, true, routeBased)
	sink := pmtuListen(t, n, n.b, pmtuInnerNoDF4)

	// a learns the narrow hop first, from a DF-set probe to another inner
	// address, so the DF-clear probe meets an SA PMTU it exceeds.
	if err := pmtuSend(t, n, pmtuInnerDF4, true); err != nil {
		t.Fatalf("send the DF-set probe that teaches a the narrow hop: %v", err)
	}
	pmtuWaitRouteMTU(t, n, n.a, n.outerB, pmtuNarrowMTU, "a learning the narrow hop of the outer path from r's ICMP about the ESP packet")

	fragBefore := pmtuIPCounter(t, n, n.a, "FragCreates")
	if err := pmtuSend(t, n, pmtuInnerNoDF4, false); err != nil {
		t.Fatalf("send the DF-clear probe from the host: %v", err)
	}
	if !pmtuReceived(t, sink) {
		t.Fatalf("b never received the DF-clear probe of %d octets; Section 8.2.1 case 2 fragments and forwards it", pmtuProbeOctets)
	}
	if created := pmtuIPCounter(t, n, n.a, "FragCreates") - fragBefore; created < 2 {
		t.Fatalf("a created %d fragments for the DF-clear probe; its ESP packet exceeds the outer PMTU %d, so a fragments it after encryption",
			created, pmtuNarrowMTU)
	}
	if mtu := pmtuRouteMTU(t, n, n.host, pmtuInnerNoDF4); mtu != 0 {
		t.Fatalf("the host learned a PMTU of %d for the DF-clear destination; Section 8.2.1 case 2 sends no PMTU ICMP", mtu)
	}
}

// VALIDATES: RFC4301-8.2.2-1 and RFC4301-7-2 over an IPv4 outer pair. The PMTU a
// learns for the SA expires after net.ipv4.route.mtu_expires, the SA's PMTU is
// the first-hop link MTU again (a encrypts the next oversized probe instead of
// refusing it), and that probe relearns the narrow hop.
// RFC requirement: RFC4301-8.2.2-1 positive -- the PMTU learned for the SA's outer path is aged out after the configured period, and the next oversized packet relearns it.
// RFC requirement: RFC4301-7-2 positive -- after the configured period the SA's PMTU is the first-hop link MTU again and normal PMTU discovery lowers it anew.
func TestXFRMTunnelPMTUAgesBackToTheLinkMTU(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	encapNetns(t)
	pmtuAgeAndRelearn(t, pmtuBuild(t, true, true), "net/ipv4/route/mtu_expires", pmtuInnerDF4)
}

// VALIDATES: RFC4301-8.2.2-1 and RFC4301-7-2 for a policy-based SA (if_id 0).
// RFC requirement: RFC4301-8.2.2-1 positive -- the PMTU learned for a policy-based SA's outer path is aged out after the configured period, and the next oversized packet relearns it.
// RFC requirement: RFC4301-7-2 positive -- after the configured period a policy-based SA's PMTU is the first-hop link MTU again and normal PMTU discovery lowers it anew.
func TestXFRMPolicyPMTUAgesBackToTheLinkMTU(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	encapNetns(t)
	pmtuAgeAndRelearn(t, pmtuBuild(t, true, false), "net/ipv4/route/mtu_expires", pmtuInnerDF4)
}

// pmtuAgeAndRelearn sets a's PMTU aging period short, learns the narrow hop,
// waits for it to expire, and relearns it. The period is set before the first
// learn: __ip_rt_update_pmtu and rt6_do_update_pmtu stamp an exception with the
// period in force when it is learned.
func pmtuAgeAndRelearn(t *testing.T, n *pmtuNet, periodKey string, inner net.IP) {
	t.Helper()
	const period = 4 * time.Second
	pmtuSysctl(t, n, n.a, periodKey, strconv.Itoa(int(period/time.Second)))

	if err := pmtuSend(t, n, inner, true); err != nil {
		t.Fatalf("send the probe that teaches a the narrow hop: %v", err)
	}
	learnedAt := time.Now()
	pmtuWaitRouteMTU(t, n, n.a, n.outerB, pmtuNarrowMTU, "a learning the narrow hop of the outer path")

	// The exception expires at the period; the wait allows the period and five
	// seconds for the lookup that drops it.
	deadline := learnedAt.Add(period + 5*time.Second)
	for {
		mtu := pmtuRouteMTU(t, n, n.a, n.outerB)
		if mtu == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("a still holds a PMTU of %d for the SA's outer path %s after the %s aging period (%s=%d)",
				mtu, time.Since(learnedAt).Round(time.Millisecond), period, periodKey, int(period/time.Second))
		}
		time.Sleep(100 * time.Millisecond) // sleep(poll-interval): the loop ends when RTM_GETROUTE on a reports no PMTU for the outer destination
	}
	if got := pmtuLinkMTUOf(t, n, n.a, n.outerB); got != pmtuLinkMTU {
		t.Fatalf("a's first hop toward %s has MTU %d, want %d", n.outerB, got, pmtuLinkMTU)
	}

	// Relearn. The host never received a PMTU ICMP (a forwarded the first probe),
	// so it sends this one at full size. a, back at the first-hop link MTU,
	// encrypts it rather than refusing it, and r's ICMP lowers the PMTU again.
	if err := pmtuSend(t, n, inner, true); err != nil {
		t.Fatalf("send the probe after the aging period: %v", err)
	}
	pmtuWaitRouteMTU(t, n, n.a, n.outerB, pmtuNarrowMTU, "a relearning the narrow hop after the aging period")
	if mtu := pmtuRouteMTU(t, n, n.host, inner); mtu != 0 {
		t.Fatalf("a answered the probe after the aging period with a PMTU ICMP of %d; its SA PMTU should have been back at the first-hop link MTU", mtu)
	}
}

// pmtuBuild creates the line from the thread's current namespace and installs the SA
// pair route-based (XFRM interfaces on the if_id) or policy-based (if_id 0),
// with an IPv4 outer pair when outer4 is true and an IPv6 one
// otherwise.
func pmtuBuild(t *testing.T, outer4, routeBased bool) *pmtuNet {
	t.Helper()
	encapNetnsUsable(t)
	base := pmtuCurrent(t)
	n := &pmtuNet{base: base, outerA: pmtuA6, outerB: pmtuB6}
	if routeBased {
		n.ifID = pmtuIfID
	}
	if outer4 {
		n.outerA, n.outerB = pmtuA4, pmtuB4
	}
	n.host, n.a, n.r, n.b = pmtuNewNetns(t, base), pmtuNewNetns(t, base), pmtuNewNetns(t, base), pmtuNewNetns(t, base)

	pmtuVeth(t, n, n.host, "h0", n.a, "a0", pmtuLinkMTU)
	pmtuVeth(t, n, n.a, "a1", n.r, "r0", pmtuLinkMTU)
	pmtuVeth(t, n, n.r, "r1", n.b, "b0", pmtuNarrowMTU)

	pmtuIn(t, n, n.host, func() {
		pmtuAddrs(t, "h0", pmtuHostLAN4, pmtuHostLAN6)
		pmtuRoute(t, nil, pmtuHostGW4, 0)
		pmtuRoute(t, nil, pmtuHostGW6, 0)
	})
	pmtuIn(t, n, n.r, func() {
		pmtuRouter(t)
		pmtuAddrs(t, "r0", pmtuRLeft4, pmtuRLeft6)
		pmtuAddrs(t, "r1", pmtuRRight4, pmtuRRight6)
	})
	pmtuIn(t, n, n.a, func() {
		pmtuRouter(t)
		pmtuAddrs(t, "a0", pmtuHostGW4, pmtuHostGW6)
		pmtuAddrs(t, "a1", pmtuA4, pmtuA6)
		pmtuRoute(t, pmtuNetOf("203.0.113.0/24"), pmtuRLeft4, 0)
		pmtuRoute(t, pmtuNetOf("2001:db8:b::/64"), pmtuRLeft6, 0)
		if routeBased {
			tunnel := pmtuXFRMInterface(t, "a1")
			pmtuRoute(t, pmtuNetOf("10.99.0.0/24"), nil, tunnel)
			pmtuRoute(t, pmtuNetOf("fd00:99::/64"), nil, tunnel)
		} else {
			// Policy-based: any route to the inner prefixes lets the lookup reach
			// the outbound policy, which replaces the route with the SA bundle.
			pmtuRoute(t, pmtuNetOf("10.99.0.0/24"), pmtuRLeft4, 0)
			pmtuRoute(t, pmtuNetOf("fd00:99::/64"), pmtuRLeft6, 0)
		}
		pmtuInstall(t, n, n.outerA, n.outerB, pmtuSPIAB, pmtuSPIBA, pmtuSideA)
	})
	pmtuIn(t, n, n.b, func() {
		pmtuAddrs(t, "b0", pmtuB4, pmtuB6)
		pmtuRoute(t, pmtuNetOf("198.51.100.0/24"), pmtuRRight4, 0)
		pmtuRoute(t, pmtuNetOf("2001:db8:a::/64"), pmtuRRight6, 0)
		innerLink := "lo"
		if routeBased {
			pmtuXFRMInterface(t, "b0")
			innerLink = "b0x"
		}
		for _, inner := range []net.IP{pmtuInnerDF4, pmtuInnerNoDF4, pmtuInner6} {
			link, err := netlink.LinkByName(innerLink)
			if err != nil {
				t.Fatalf("find %s on b: %v", innerLink, err)
			}
			crossFamilyAddr(t, link, inner, crossFamilyHostBits(inner))
		}
		pmtuInstall(t, n, n.outerB, n.outerA, pmtuSPIBA, pmtuSPIAB, pmtuSideB)
	})
	return n
}

// pmtuNewNetns creates a namespace and returns the thread to base.
func pmtuNewNetns(t *testing.T, base netns.NsHandle) netns.NsHandle {
	t.Helper()
	ns, err := netns.New()
	if err != nil {
		t.Fatalf("create a namespace of the line: %v", err)
	}
	t.Cleanup(func() { _ = ns.Close() })
	if err := netns.Set(base); err != nil {
		t.Fatalf("return to the base namespace: %v", err)
	}
	return ns
}

// pmtuIn runs fn with the thread in target, then returns it to the base
// namespace. Sockets fn opens stay in target.
func pmtuIn(t *testing.T, n *pmtuNet, target netns.NsHandle, fn func()) {
	t.Helper()
	if err := netns.Set(target); err != nil {
		t.Fatalf("enter a namespace of the line: %v", err)
	}
	defer func() {
		if err := netns.Set(n.base); err != nil {
			t.Errorf("return to the base namespace: %v", err)
		}
	}()
	fn()
}

// pmtuVeth joins two namespaces with a veth of the given MTU and brings both ends up.
func pmtuVeth(t *testing.T, n *pmtuNet, left netns.NsHandle, leftName string, right netns.NsHandle, rightName string, mtu int) {
	t.Helper()
	pmtuIn(t, n, left, func() {
		veth := &netlink.Veth{
			Name:          leftName,
			MTU:           mtu,
			PeerName:      rightName,
			PeerMTU:       uint32(mtu),
			PeerNamespace: netlink.NsFd(int(right)),
		}
		if err := netlink.LinkAdd(veth); err != nil {
			t.Fatalf("veth %s-%s: %v", leftName, rightName, err)
		}
		pmtuUp(t, leftName)
	})
	pmtuIn(t, n, right, func() { pmtuUp(t, rightName) })
}

func pmtuUp(t *testing.T, name string) {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("find %s: %v", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("%s up: %v", name, err)
	}
}

// pmtuAddrs puts one IPv4 address in a /24 and one IPv6 address in a /64 on a
// link. crossFamilyAddr skips DAD on the IPv6 one, so a tentative address cannot
// hold a probe back.
func pmtuAddrs(t *testing.T, name string, v4, v6 net.IP) {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("find %s: %v", name, err)
	}
	crossFamilyAddr(t, link, v4, 24)
	crossFamilyAddr(t, link, v6, 64)
}

// pmtuRoute adds a route in the thread's namespace: a default route when dst is
// nil, through gw when gw is set, else out of the link at linkIndex.
func pmtuRoute(t *testing.T, dst *net.IPNet, gw net.IP, linkIndex int) {
	t.Helper()
	route := &netlink.Route{Dst: dst, Gw: gw, LinkIndex: linkIndex}
	if gw == nil {
		route.Scope = netlink.SCOPE_LINK
	}
	if dst == nil {
		bits := crossFamilyHostBits(gw)
		route.Dst = &net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, bits)}
		if bits == 32 {
			route.Dst.IP = net.IPv4zero.To4()
		}
	}
	if err := netlink.RouteAdd(route); err != nil {
		t.Fatalf("route %v via %v dev %d: %v", route.Dst, gw, linkIndex, err)
	}
}

func pmtuNetOf(cidr string) *net.IPNet {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		panic("BUG: pmtu test prefix " + cidr + ": " + err.Error())
	}
	return ipnet
}

// pmtuRouter turns on forwarding for both families in the thread's namespace.
// The ICMP rate limits go to zero so a second error to the same peer inside a
// second is not held back; a namespace that refuses that write only slows the
// test, so the refusal is logged rather than failed.
func pmtuRouter(t *testing.T) {
	t.Helper()
	pmtuSysctlHere(t, "net/ipv4/ip_forward", "1")
	pmtuSysctlHere(t, "net/ipv6/conf/all/forwarding", "1")
	for _, key := range []string{"net/ipv4/icmp_ratelimit", "net/ipv6/icmp/ratelimit"} {
		if err := os.WriteFile("/proc/sys/"+key, []byte("0"), 0o644); err != nil { //nolint:gosec // fixed /proc/sys path in the test's own namespace
			t.Logf("leave %s at its default, the write was refused: %v", key, err)
		}
	}
}

// pmtuSysctl writes a sysctl of the target namespace.
func pmtuSysctl(t *testing.T, n *pmtuNet, target netns.NsHandle, key, value string) {
	t.Helper()
	pmtuIn(t, n, target, func() { pmtuSysctlHere(t, key, value) })
}

// pmtuSysctlHere writes a sysctl of the namespace the thread is in and reads it
// back. /proc/sys/net resolves through the opening thread's namespace, and the
// thread is locked.
func pmtuSysctlHere(t *testing.T, key, value string) {
	t.Helper()
	path := "/proc/sys/" + key
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil { //nolint:gosec // fixed /proc/sys path in the test's own namespace
		t.Fatalf("write %s=%s: %v", key, value, err)
	}
	raw, err := os.ReadFile(path) //nolint:gosec // fixed /proc/sys path in the test's own namespace
	if err != nil {
		t.Fatalf("read back %s: %v", key, err)
	}
	if got := strings.TrimSpace(string(raw)); got != value {
		t.Fatalf("%s reads %q after writing %q", key, got, value)
	}
}

// pmtuXFRMInterface adds an XFRM interface on the Child SA's if_id over the
// underlay, brings it up, and returns its index.
func pmtuXFRMInterface(t *testing.T, underlayName string) int {
	t.Helper()
	underlay, err := netlink.LinkByName(underlayName)
	if err != nil {
		t.Fatalf("find %s: %v", underlayName, err)
	}
	name := underlayName + "x"
	if err := netlink.LinkAdd(&netlink.Xfrmi{Name: name, ParentIndex: underlay.Attrs().Index, Ifid: pmtuIfID}); err != nil {
		t.Fatalf("xfrm interface on %s: %v", underlayName, err)
	}
	pmtuUp(t, name)
	tunnel, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("find %s: %v", name, err)
	}
	return tunnel.Attrs().Index
}

// pmtuInstall installs one side's half of the Child SA through the XFRM backend:
// the outbound and inbound tunnel-mode ESP states with no state selector, and a
// policy of each inner family in each direction. A route-based pair binds
// catch-all policies to the if_id, as a peer with vti bind does. A policy-based
// pair sits in the node-wide SPD, where a catch-all would capture the ESP and
// ICMP traffic itself, so its policies carry the host LAN and the inner prefixes,
// as a negotiated traffic selector pair does.
func pmtuInstall(t *testing.T, n *pmtuNet, local, remote net.IP, outSPI, inSPI uint32, side pmtuSide) {
	t.Helper()
	b := &xfrmBackend{}
	type selectorPair struct{ mine, theirs *net.IPNet }
	pairs := []selectorPair{
		{pmtuNetOf("0.0.0.0/0"), pmtuNetOf("0.0.0.0/0")},
		{pmtuNetOf("::/0"), pmtuNetOf("::/0")},
	}
	if n.ifID == 0 {
		lan := []*net.IPNet{pmtuNetOf("192.0.2.0/24"), pmtuNetOf("2001:db8:1::/64")}
		inner := []*net.IPNet{pmtuNetOf("10.99.0.0/24"), pmtuNetOf("fd00:99::/64")}
		pairs = []selectorPair{{lan[0], inner[0]}, {lan[1], inner[1]}}
		if side == pmtuSideB {
			pairs = []selectorPair{{inner[0], lan[0]}, {inner[1], lan[1]}}
		}
	}
	states := []SAParams{
		{SPI: outSPI, Src: local, Dst: remote, Dir: SADirOut},
		{SPI: inSPI, Src: remote, Dst: local, Dir: SADirIn},
	}
	for i := range states {
		p := &states[i]
		p.Proto, p.Mode = ProtoESP, ModeTunnel
		p.IfID, p.ReqID, p.ReplayWin = n.ifID, pmtuReqID, 32
		p.EncAlgo, p.EncKey, p.IsAEAD = "aes256gcm", encapTestAEADKey, true
		if err := b.InstallSA(*p); err != nil {
			t.Fatalf("install SA spi=%#x %s -> %s: %v", p.SPI, p.Src, p.Dst, err)
		}
	}
	for _, pair := range pairs {
		policies := []SPParams{
			{Dir: SADirOut, TunnelSrc: local, TunnelDst: remote},
			{Dir: SADirIn, TunnelSrc: remote, TunnelDst: local},
		}
		for i := range policies {
			p := &policies[i]
			p.Src, p.Dst = pair.mine, pair.theirs
			if p.Dir == SADirIn {
				p.Src, p.Dst = pair.theirs, pair.mine
			}
			p.Proto, p.Mode, p.IfID, p.ReqID = ProtoESP, ModeTunnel, n.ifID, pmtuReqID
			p.Owner, p.Priority = "pmtu", PriorityChildSA
			if err := b.InstallPolicy(*p); err != nil {
				t.Fatalf("install policy %s -> %s dir=%d: %v", p.Src, p.Dst, p.Dir, err)
			}
		}
	}
}

// pmtuListen opens a UDP socket on an inner address in b.
func pmtuListen(t *testing.T, n *pmtuNet, ns netns.NsHandle, inner net.IP) *net.UDPConn {
	t.Helper()
	var conn *net.UDPConn
	pmtuIn(t, n, ns, func() {
		var err error
		conn, err = net.ListenUDP("udp", &net.UDPAddr{IP: inner, Port: pmtuPort})
		if err != nil {
			t.Fatalf("listen on b's inner address %s: %v", inner, err)
		}
	})
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// pmtuCurrent answers a handle on the thread's current namespace, closed at cleanup.
func pmtuCurrent(t *testing.T) netns.NsHandle {
	t.Helper()
	ns, err := netns.Get()
	if err != nil {
		t.Fatalf("handle on the current namespace: %v", err)
	}
	t.Cleanup(func() { _ = ns.Close() })
	return ns
}

// pmtuReceived reports whether a probe reached the sink within two seconds.
func pmtuReceived(t *testing.T, sink *net.UDPConn) bool {
	t.Helper()
	if err := sink.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var buf [2048]byte
	_, _, err := sink.ReadFromUDP(buf[:])
	if err == nil {
		return true
	}
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return false
	}
	t.Fatalf("read the sink: %v", err)
	return false
}

// pmtuSend sends one probe of pmtuProbeOctets from the host to an inner address,
// with the DF bit set (IPv4) or path MTU discovery on (IPv6) when df is true, and
// the DF bit clear when it is false.
func pmtuSend(t *testing.T, n *pmtuNet, inner net.IP, df bool) error {
	t.Helper()
	var sendErr error
	pmtuIn(t, n, n.host, func() {
		conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: inner, Port: pmtuPort})
		if err != nil {
			t.Fatalf("dial %s from the host: %v", inner, err)
		}
		defer conn.Close() //nolint:errcheck // a probe socket closed after one write; nothing reads the result.
		pmtuDiscover(t, conn, inner, df)
		header := 20 + 8
		if inner.To4() == nil {
			header = 40 + 8
		}
		_, sendErr = conn.Write(make([]byte, pmtuProbeOctets-header))
	})
	return sendErr
}

// pmtuDiscover sets the socket's path MTU discovery mode: DO sets the IPv4 DF bit
// and stops the host fragmenting, DONT leaves DF clear.
func pmtuDiscover(t *testing.T, conn *net.UDPConn, inner net.IP, df bool) {
	t.Helper()
	level, option, value := unix.IPPROTO_IP, unix.IP_MTU_DISCOVER, unix.IP_PMTUDISC_DO
	if !df {
		value = unix.IP_PMTUDISC_DONT
	}
	if inner.To4() == nil {
		level, option, value = unix.IPPROTO_IPV6, unix.IPV6_MTU_DISCOVER, unix.IPV6_PMTUDISC_DO
	}
	raw, err := conn.SyscallConn()
	if err != nil {
		t.Fatalf("raw probe socket: %v", err)
	}
	var setErr error
	if err := raw.Control(func(fd uintptr) { setErr = unix.SetsockoptInt(int(fd), level, option, value) }); err != nil {
		t.Fatalf("control the probe socket: %v", err)
	}
	if setErr != nil {
		t.Fatalf("set path MTU discovery on the probe socket: %v", setErr)
	}
}

// pmtuRouteMTU answers the PMTU the target namespace holds for dst: the RTAX_MTU
// an RTM_GETROUTE reports, which the kernel fills from a learned, unexpired PMTU
// exception (rt_fill_info, rt6_fill_node), and 0 when it holds none.
func pmtuRouteMTU(t *testing.T, n *pmtuNet, target netns.NsHandle, dst net.IP) int {
	t.Helper()
	var mtu int
	pmtuIn(t, n, target, func() {
		routes, err := netlink.RouteGet(dst)
		if err != nil {
			t.Fatalf("route get %s: %v", dst, err)
		}
		if len(routes) == 0 {
			t.Fatalf("route get %s answered no route", dst)
		}
		mtu = routes[0].MTU
	})
	return mtu
}

// pmtuLinkMTUOf answers the MTU of the link the target namespace routes dst out of.
func pmtuLinkMTUOf(t *testing.T, n *pmtuNet, target netns.NsHandle, dst net.IP) int {
	t.Helper()
	var mtu int
	pmtuIn(t, n, target, func() {
		routes, err := netlink.RouteGet(dst)
		if err != nil {
			t.Fatalf("route get %s: %v", dst, err)
		}
		if len(routes) == 0 {
			t.Fatalf("route get %s answered no route", dst)
		}
		link, err := netlink.LinkByIndex(routes[0].LinkIndex)
		if err != nil {
			t.Fatalf("link %d toward %s: %v", routes[0].LinkIndex, dst, err)
		}
		mtu = link.Attrs().MTU
	})
	return mtu
}

// pmtuWaitRouteMTU waits up to five seconds for the target namespace to hold
// exactly want as the PMTU toward dst, and fails naming what it held.
func pmtuWaitRouteMTU(t *testing.T, n *pmtuNet, target netns.NsHandle, dst net.IP, want int, what string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := pmtuRouteMTU(t, n, target, dst)
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: the PMTU toward %s is %d, want %d (0 means no PMTU was learned)", what, dst, got, want)
		}
		time.Sleep(50 * time.Millisecond) // sleep(poll-interval): the loop ends when RTM_GETROUTE reports the PMTU this step waits for
	}
}

// pmtuIPCounter reads one Ip counter of the target namespace's /proc/net/snmp.
// It reads /proc/thread-self for the reason encapStat states.
func pmtuIPCounter(t *testing.T, n *pmtuNet, target netns.NsHandle, name string) int {
	t.Helper()
	var value int
	pmtuIn(t, n, target, func() {
		raw, err := os.ReadFile("/proc/thread-self/net/snmp")
		if err != nil {
			t.Fatalf("read snmp: %v", err)
		}
		var names []string
		for line := range strings.SplitSeq(string(raw), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 0 || fields[0] != "Ip:" {
				continue
			}
			if names == nil {
				names = fields
				continue
			}
			for i := 1; i < len(fields) && i < len(names); i++ {
				if names[i] != name {
					continue
				}
				v, convErr := strconv.Atoi(fields[i])
				if convErr != nil {
					t.Fatalf("snmp Ip %s: %v", name, convErr)
				}
				value = v
				return
			}
		}
		t.Fatalf("snmp holds no Ip counter %s", name)
	})
	return value
}
