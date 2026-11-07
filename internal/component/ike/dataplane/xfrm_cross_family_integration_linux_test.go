// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- tunnel states serve either inner family
// Related: xfrm_linux.go -- xfrmStateFromParams sets XFRM_STATE_AF_UNSPEC
//
// VALIDATES: a tunnel-mode ESP SA pair installed through the XFRM backend, with no
// state selector, carries inner traffic of the other address family across the
// kernel datapath: IPv6 inside an IPv4 outer pair, IPv4 inside an IPv6 outer pair,
// and (no regression) IPv4 inside IPv4.
// PREVENTS: the defect RFC 4301 Section 5.1.2 rules out: "IPsec allows the IP
// version of the encapsulating header to be different from that of the inner
// header." Without XFRM_STATE_AF_UNSPEC the kernel's copy_from_user_state gives the
// state the outer family, xfrm_state_look_at skips it for a flow of the other
// family, and the sender drops every such packet (XfrmOutNoStates) while the SA
// reports up.
// METHOD: two network namespaces joined by a veth carrying the outer pair. Each side
// has an XFRM interface bound to the Child SA's if_id with the inner address on it,
// and its own XFRM backend installing its half of the SA pair and catch-all policies
// of the inner family in each direction, as a route-based tunnel does. A UDP
// datagram crosses from one inner address to the other, the receiver answers, and
// the sender's outbound state must have counted the packet. A failure prints every
// xfrm_stat counter that moved on either side.
//
// Each subtest runs in a user and network namespace of the test's own
// (userns.Enter), so it needs no host privilege.

//go:build linux

package dataplane

import (
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
	crossFamilyIfID  = 0x2a
	crossFamilyReqID = 0x2a2a
	crossFamilySPIAB = 0x2a2a0001 // carries side a's packets to side b
	crossFamilySPIBA = 0x2a2a0002 // carries side b's packets to side a
	crossFamilyPort  = 47020
)

// crossFamilyCase is one outer pair and one inner pair. The outer addresses sit on
// the veth with outerBits; the inner addresses are host addresses on the XFRM
// interfaces.
type crossFamilyCase struct {
	name           string
	outerA, outerB string
	outerBits      int
	innerA, innerB string
}

func TestXFRMTunnelCarriesTheOtherInnerFamily(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	cases := []crossFamilyCase{
		{"ipv6 inside ipv4", "198.51.100.1", "198.51.100.2", 24, "fd00:2a::1", "fd00:2a::2"},
		{"ipv4 inside ipv6", "2001:db8:2a::1", "2001:db8:2a::2", 64, "10.42.0.1", "10.42.0.2"},
		{"ipv4 inside ipv4", "198.51.100.1", "198.51.100.2", 24, "10.42.0.1", "10.42.0.2"},
	}
	for _, tc := range cases {
		// Each subtest runs on its own goroutine, so encapNetns locks that goroutine's
		// thread into a fresh namespace and the cases share no kernel state.
		t.Run(tc.name, func(t *testing.T) { crossFamilyExchange(t, tc) })
	}
}

func crossFamilyExchange(t *testing.T, tc crossFamilyCase) {
	encapNetns(t)
	encapNetnsUsable(t)
	home, err := netns.Get()
	if err != nil {
		t.Fatalf("handle on side a's namespace: %v", err)
	}
	t.Cleanup(func() { _ = home.Close() })
	peer, err := netns.New()
	if err != nil {
		t.Fatalf("create side b's namespace: %v", err)
	}
	t.Cleanup(func() { _ = peer.Close() })
	if err := netns.Set(home); err != nil {
		t.Fatalf("return to side a's namespace: %v", err)
	}

	veth := &netlink.Veth{Name: "xfam0", PeerName: "xfam1", PeerNamespace: netlink.NsFd(int(peer))}
	if err := netlink.LinkAdd(veth); err != nil {
		t.Fatalf("veth: %v", err)
	}
	outerA, outerB := net.ParseIP(tc.outerA), net.ParseIP(tc.outerB)
	innerA, innerB := net.ParseIP(tc.innerA), net.ParseIP(tc.innerB)

	crossFamilySide(t, "xfam0", tc.outerA, tc.outerBits, innerA, innerB)
	crossFamilyInstall(t, outerA, outerB, innerA, crossFamilySPIAB, crossFamilySPIBA)

	var listener *net.UDPConn
	crossFamilyIn(t, peer, home, func() {
		crossFamilySide(t, "xfam1", tc.outerB, tc.outerBits, innerB, innerA)
		crossFamilyInstall(t, outerB, outerA, innerB, crossFamilySPIBA, crossFamilySPIAB)
		listener, err = net.ListenUDP("udp", &net.UDPAddr{IP: innerB, Port: crossFamilyPort})
		if err != nil {
			t.Fatalf("listen on side b's inner address %s: %v", innerB, err)
		}
	})
	t.Cleanup(func() { _ = listener.Close() })

	conn, err := net.DialUDP("udp", &net.UDPAddr{IP: innerA}, &net.UDPAddr{IP: innerB, Port: crossFamilyPort})
	if err != nil {
		t.Fatalf("dial side b's inner address from %s: %v", innerA, err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	beforeA := crossFamilyStats(t)
	var beforeB map[string]int
	crossFamilyIn(t, peer, home, func() { beforeB = crossFamilyStats(t) })
	moved := func() string {
		var afterB map[string]int
		crossFamilyIn(t, peer, home, func() { afterB = crossFamilyStats(t) })
		return "side a: " + crossFamilyDelta(beforeA, crossFamilyStats(t)) +
			"; side b: " + crossFamilyDelta(beforeB, afterB)
	}

	request := []byte("ze cross-family request")
	if _, err := conn.Write(request); err != nil {
		t.Fatalf("send %s -> %s: %v; xfrm_stat moved: %s", innerA, innerB, err, moved())
	}
	var buf [256]byte
	if err := listener.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, from, err := listener.ReadFromUDP(buf[:])
	if err != nil {
		t.Fatalf("side b received no request from %s through the %s-outer SA: %v; xfrm_stat moved: %s",
			innerA, crossFamilyFamily(outerA), err, moved())
	}
	if string(buf[:n]) != string(request) || !from.IP.Equal(innerA) {
		t.Fatalf("side b received %q from %v, want %q from %s", buf[:n], from, request, innerA)
	}
	reply := []byte("ze cross-family reply")
	if _, err := listener.WriteToUDP(reply, from); err != nil {
		t.Fatalf("answer %s -> %s: %v; xfrm_stat moved: %s", innerB, innerA, err, moved())
	}
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, err = conn.Read(buf[:])
	if err != nil {
		t.Fatalf("side a received no reply from %s through the %s-outer SA: %v; xfrm_stat moved: %s",
			innerB, crossFamilyFamily(outerA), err, moved())
	}
	if string(buf[:n]) != string(reply) {
		t.Fatalf("side a received %q, want %q", buf[:n], reply)
	}

	// Delivery alone could come from a path around the tunnel. The routes make that
	// impossible, and the outbound state counting the packet proves it.
	out, err := readXFRMMigrationState(&netlink.XfrmState{Spi: crossFamilySPIAB, Dst: outerB, Proto: netlink.XFRM_PROTO_ESP, Ifid: crossFamilyIfID})
	if err != nil {
		t.Fatalf("read side a's outbound state: %v", err)
	}
	if out.info.Curlft.Packets == 0 {
		t.Fatalf("the request crossed but side a's outbound SA counted no packet; it bypassed the tunnel")
	}
	if delta := moved(); strings.Contains(delta, "XfrmOutNoStates") {
		t.Errorf("an outbound packet found no state on the way: %s", delta)
	}
}

// crossFamilySide sets up one side in the current namespace: the outer address on
// the veth end, an XFRM interface on the Child SA's if_id carrying the inner
// address, and a host route to the other side's inner address through it. IPv6
// addresses skip DAD, so a tentative address cannot hold the first datagram back.
func crossFamilySide(t *testing.T, underlayName, outer string, outerBits int, inner, peerInner net.IP) {
	t.Helper()
	underlay, err := netlink.LinkByName(underlayName)
	if err != nil {
		t.Fatalf("find %s: %v", underlayName, err)
	}
	crossFamilyAddr(t, underlay, net.ParseIP(outer), outerBits)
	if err := netlink.LinkSetUp(underlay); err != nil {
		t.Fatalf("%s up: %v", underlayName, err)
	}
	xfrmi := &netlink.Xfrmi{Name: underlayName + "x", ParentIndex: underlay.Attrs().Index, Ifid: crossFamilyIfID}
	if err := netlink.LinkAdd(xfrmi); err != nil {
		t.Fatalf("xfrm interface on %s: %v", underlayName, err)
	}
	tunnel, err := netlink.LinkByName(underlayName + "x")
	if err != nil {
		t.Fatalf("find the xfrm interface on %s: %v", underlayName, err)
	}
	crossFamilyAddr(t, tunnel, inner, crossFamilyHostBits(inner))
	if err := netlink.LinkSetUp(tunnel); err != nil {
		t.Fatalf("xfrm interface up: %v", err)
	}
	bits := crossFamilyHostBits(peerInner)
	route := &netlink.Route{
		LinkIndex: tunnel.Attrs().Index,
		Dst:       &net.IPNet{IP: peerInner, Mask: net.CIDRMask(bits, bits)},
		Scope:     netlink.SCOPE_LINK,
	}
	if err := netlink.RouteAdd(route); err != nil {
		t.Fatalf("route %s through the xfrm interface: %v", peerInner, err)
	}
}

func crossFamilyAddr(t *testing.T, link netlink.Link, ip net.IP, bits int) {
	t.Helper()
	addr := &netlink.Addr{IPNet: &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, crossFamilyHostBits(ip))}}
	if ip.To4() == nil {
		addr.Flags = unix.IFA_F_NODAD
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		t.Fatalf("address %s/%d on %s: %v", ip, bits, link.Attrs().Name, err)
	}
}

// crossFamilyInstall installs one side's half of the Child SA through the XFRM
// backend, as the IKE engine does for a route-based tunnel: the outbound and
// inbound tunnel-mode states with no state selector, and a catch-all policy of the
// inner family in each direction, bound to the if_id.
func crossFamilyInstall(t *testing.T, local, remote, inner net.IP, outSPI, inSPI uint32) {
	t.Helper()
	b := &xfrmBackend{}
	for _, p := range []SAParams{
		{SPI: outSPI, Src: local, Dst: remote},
		{SPI: inSPI, Src: remote, Dst: local},
	} {
		p.Proto, p.Mode = ProtoESP, ModeTunnel
		p.IfID, p.ReqID, p.ReplayWin = crossFamilyIfID, crossFamilyReqID, 32
		p.EncAlgo, p.EncKey, p.IsAEAD = "aes256gcm", encapTestAEADKey, true
		if err := b.InstallSA(p); err != nil {
			t.Fatalf("install SA spi=%#x %s -> %s: %v", p.SPI, p.Src, p.Dst, err)
		}
	}
	bits := crossFamilyHostBits(inner)
	anyInner := &net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, bits)}
	if bits == 32 {
		anyInner.IP = net.IPv4zero.To4()
	}
	for _, p := range []SPParams{
		{Dir: SADirOut, TunnelSrc: local, TunnelDst: remote},
		{Dir: SADirIn, TunnelSrc: remote, TunnelDst: local},
	} {
		p.Src, p.Dst = anyInner, anyInner
		p.Proto, p.Mode, p.IfID, p.ReqID = ProtoESP, ModeTunnel, crossFamilyIfID, crossFamilyReqID
		p.Owner, p.Priority = "cross-family", PriorityChildSA
		if err := b.InstallPolicy(p); err != nil {
			t.Fatalf("install %s policy dir=%d: %v", crossFamilyFamily(inner), p.Dir, err)
		}
	}
}

// crossFamilyIn runs fn with this goroutine's locked thread in target, then
// returns it to home. Sockets fn opens stay in target.
func crossFamilyIn(t *testing.T, target, home netns.NsHandle, fn func()) {
	t.Helper()
	if err := netns.Set(target); err != nil {
		t.Fatalf("enter side b's namespace: %v", err)
	}
	defer func() {
		if err := netns.Set(home); err != nil {
			t.Errorf("return to side a's namespace: %v", err)
		}
	}()
	fn()
}

// crossFamilyStats reads every xfrm_stat counter of the namespace this thread is
// in. It reads /proc/thread-self for the reason encapStat states.
func crossFamilyStats(t *testing.T) map[string]int {
	t.Helper()
	raw, err := os.ReadFile("/proc/thread-self/net/xfrm_stat")
	if err != nil {
		t.Fatalf("read xfrm_stat: %v", err)
	}
	stats := make(map[string]int)
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		v, err := strconv.Atoi(fields[1])
		if err != nil {
			t.Fatalf("xfrm_stat %s: %v", fields[0], err)
		}
		stats[fields[0]] = v
	}
	if len(stats) == 0 {
		t.Fatalf("xfrm_stat holds no counter; a failure here could not name what moved")
	}
	return stats
}

// crossFamilyDelta names each counter that changed between two snapshots.
func crossFamilyDelta(before, after map[string]int) string {
	var b strings.Builder
	for name, v := range after {
		if d := v - before[name]; d != 0 {
			if b.Len() > 0 {
				b.WriteString(", ")
			}
			b.WriteString(name + " +" + strconv.Itoa(d))
		}
	}
	if b.Len() == 0 {
		return "no counter moved"
	}
	return b.String()
}

func crossFamilyHostBits(ip net.IP) int {
	if ip.To4() != nil {
		return 32
	}
	return 128
}

func crossFamilyFamily(ip net.IP) string {
	if ip.To4() != nil {
		return "IPv4"
	}
	return "IPv6"
}
