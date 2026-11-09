// VALIDATES: on a real Linux stack, the tunnel-source guard (source_guard.go) keeps IPv6
// from a tunnel address off any interface but its XFRM interface, through its two
// halves, each red alone on its own path (AC-5, AC-6, A-2, A-3):
//
//   - bound source (the socket chose the tunnel address): the `from` rule steers the
//     packet into the guard table, so it goes into the XFRM interface when a tunnel
//     route covers it and is refused as unreachable otherwise. With the rule alone,
//     the WAN never transmits it.
//   - kernel-chosen source (a route's preferred source picks the tunnel address after
//     routing out of the WAN): no source rule is consulted, so the rule alone lets it
//     leave; the nft postrouting drop stops it.
//
// The instrument is the WAN device's transmit counter: a dummy device transmits every
// packet routed to it without neighbor discovery, and nft postrouting runs before.
// PREVENTS: the residual the IPv6 source BYPASS left, an IPv6 packet from a tunnel
// address leaving another interface in the clear.

//go:build linux

package engine

import (
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/test/userns"

	_ "github.com/ze-software/ze/internal/plugins/firewall/nft" // registers the nft firewall backend
)

const (
	guardTestWAN       = "zegwan0"
	guardTestWANCIDR   = "2001:db8:50::1/64"
	guardTestXFRMI     = "zegxfi0"
	guardTestIfID      = 11
	guardTestTunnel    = "2001:db8:60::1"
	guardTestTunnelNet = "2001:db8:60::1/64"
	guardTestCovered   = "2001:db8:98::1"   // a tunnel route covers it
	guardTestUncovered = "2001:db8:97::1"   // routed out of the WAN only
	guardTestAutoDst   = "2001:db8:99::1"   // the WAN route picks the tunnel address as source
	guardTestCoveredTS = "2001:db8:98::/64" // the Child SA remote selector
)

func guardTestLink(t *testing.T, name string) netlink.Link {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return link
}

func guardTestAddr(t *testing.T, link netlink.Link, cidr string) {
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

func guardTestRoute(t *testing.T, route *netlink.Route) {
	t.Helper()
	if err := netlink.RouteAdd(route); err != nil {
		t.Fatalf("add route %v: %v", route, err)
	}
}

// guardTestTopology builds the WAN dummy and the XFRM interface, and routes the three
// destinations out of the WAN: the auto-source one with the tunnel address as its
// preferred source. It answers the XFRM interface's index.
func guardTestTopology(t *testing.T) int {
	t.Helper()
	if err := netlink.LinkAdd(&netlink.Dummy{Name: guardTestWAN}); err != nil {
		t.Fatalf("add %s: %v", guardTestWAN, err)
	}
	if err := netlink.LinkAdd(&netlink.Xfrmi{Name: guardTestXFRMI, Ifid: guardTestIfID}); err != nil {
		t.Fatalf("add %s: %v", guardTestXFRMI, err)
	}
	wan, xfrmi := guardTestLink(t, guardTestWAN), guardTestLink(t, guardTestXFRMI)
	for _, link := range []netlink.Link{wan, xfrmi} {
		if err := netlink.LinkSetUp(link); err != nil {
			t.Fatalf("set %s up: %v", link.Attrs().Name, err)
		}
	}
	guardTestAddr(t, wan, guardTestWANCIDR)
	guardTestAddr(t, xfrmi, guardTestTunnelNet)
	for _, dst := range []string{guardTestCovered, guardTestUncovered} {
		guardTestRoute(t, &netlink.Route{LinkIndex: wan.Attrs().Index, Dst: &net.IPNet{IP: net.ParseIP(dst), Mask: net.CIDRMask(64, 128)}})
	}
	guardTestRoute(t, &netlink.Route{
		LinkIndex: wan.Attrs().Index,
		Dst:       &net.IPNet{IP: net.ParseIP(guardTestAutoDst), Mask: net.CIDRMask(64, 128)},
		Src:       net.ParseIP(guardTestTunnel),
	})
	return xfrmi.Attrs().Index
}

// wanSend writes one datagram from source (nil: the kernel picks it) to target and
// answers how many packets the WAN transmitted, and the send's error.
func wanSend(t *testing.T, source net.IP, target string) (uint64, error) {
	t.Helper()
	before := guardTestLink(t, guardTestWAN).Attrs().Statistics.TxPackets
	conn, err := net.ListenUDP("udp6", &net.UDPAddr{IP: source})
	if err != nil {
		t.Fatalf("open a socket from %v: %v", source, err)
	}
	defer func() { _ = conn.Close() }()
	_, sendErr := conn.WriteToUDP([]byte("ze-guard-probe"), &net.UDPAddr{IP: net.ParseIP(target), Port: 9})
	return guardTestLink(t, guardTestWAN).Attrs().Statistics.TxPackets - before, sendErr
}

// guardOnly is a guardKernel that installs the rules and routes for real but publishes
// no nft table, for the leg that proves the rule alone.
type guardOnly struct{ kernelGuard }

func (guardOnly) publish([]firewall.Table) error { return nil }

func TestTunnelSourceGuardInKernel(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	xfrmiIndex := guardTestTopology(t)
	tunnel := net.ParseIP(guardTestTunnel)
	log := slog.Default()

	// VACUITY: with no guard, both kinds of send leave through the WAN.
	if tx, err := wanSend(t, tunnel, guardTestUncovered); tx == 0 {
		t.Fatalf("a bound-source send did not leave the WAN with no guard (err %v); the test cannot see the guard", err)
	}
	if tx, err := wanSend(t, nil, guardTestAutoDst); tx == 0 {
		t.Fatalf("a kernel-chosen-source send did not leave the WAN with no guard (err %v); the test cannot see the guard", err)
	}

	spec := map[string]sourceGuardSpec{guardTestXFRMI: {
		iface: guardTestXFRMI, device: guardTestXFRMI, ifIndex: xfrmiIndex, ifID: guardTestIfID,
		sources: []netip.Addr{netip.MustParseAddr(guardTestTunnel)},
	}}
	ts := netip.MustParsePrefix(guardTestCoveredTS)

	// LEG 1, the rule alone (A-2, AC-5).
	ruleOnly := newSourceGuards(guardOnly{})
	ruleOnly.tunnelRoute(true, "peer", guardRouteKey{ifID: guardTestIfID, prefix: ts}, log)
	if guarded := ruleOnly.install(spec, log); !guarded[netip.MustParseAddr(guardTestTunnel)] {
		t.Fatalf("the rule-only guard did not hold %s", guardTestTunnel)
	}
	if tx, err := wanSend(t, tunnel, guardTestCovered); tx != 0 {
		t.Errorf("a bound-source send to a tunnel-covered destination left the WAN (+%d, err %v); the rule did not steer it into %s", tx, err, guardTestXFRMI)
	}
	tx, err := wanSend(t, tunnel, guardTestUncovered)
	if tx != 0 {
		t.Errorf("a bound-source send to a WAN-only destination left the WAN (+%d)", tx)
	}
	if !errors.Is(err, syscall.ENETUNREACH) && !errors.Is(err, syscall.EHOSTUNREACH) {
		t.Errorf("a bound-source send to a WAN-only destination answered %v, want unreachable from the guard table", err)
	}
	// A-3: the rule alone does not see a source the kernel picked after routing.
	if tx, _ := wanSend(t, nil, guardTestAutoDst); tx == 0 {
		t.Error("with the rule alone a kernel-chosen-source send did not leave the WAN; the nft leg below would prove nothing")
	}
	ruleOnly.prune(nil, log)

	// LEG 2, the whole guard through the firewall registry (AC-6).
	if err := firewall.LoadBackend("nft"); err != nil {
		t.Fatalf("load the nft firewall backend: %v", err)
	}
	whole := newSourceGuards(kernelGuard{})
	whole.tunnelRoute(true, "peer", guardRouteKey{ifID: guardTestIfID, prefix: ts}, log)
	t.Cleanup(func() { whole.prune(nil, log) })
	if guarded := whole.install(spec, log); !guarded[netip.MustParseAddr(guardTestTunnel)] {
		t.Fatalf("the guard did not hold %s", guardTestTunnel)
	}
	if tx, err := wanSend(t, nil, guardTestAutoDst); tx != 0 {
		t.Errorf("a kernel-chosen-source send from the tunnel address left the WAN in the clear (+%d, err %v)", tx, err)
	}
	if tx, err := wanSend(t, tunnel, guardTestCovered); tx != 0 {
		t.Errorf("a bound-source send left the WAN with the whole guard (+%d, err %v)", tx, err)
	}
	// The WAN's own address is not guarded.
	if tx, err := wanSend(t, nil, guardTestUncovered); tx == 0 {
		t.Errorf("a send from the WAN's own address no longer leaves the WAN (err %v); the guard is wider than the tunnel address", err)
	}
}
