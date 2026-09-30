//go:build linux

// VALIDATES: RFC 5881 Section 6 on the wire: the link a single-hop Control
// packet leaves on. Two veth pairs stand for two links, p0-p1 (the one-hop path
// the session protects, where the peer is connected) and o0-o1 (another path).
// A packet socket on the far end of each pair observes which link carried the
// frame. The peer's /32 route through o0 is the input pushed toward the
// violation: an unpinned socket follows it and leaves over o0.
// PREVENTS: a single-hop Control packet the kernel routes over another link,
// which keeps a session Up while the protected link is gone.
//
// The test creates links and routes, so it runs in a user and network
// namespace of its own (userns.Enter), where it is root over the namespace
// and needs no privilege on the host.
package transport

import (
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/test/userns"
)

// The one-hop path topology. The peer sits on the p0 subnet; the other link
// has a subnet of its own and, when onePathDetour runs, a more specific route
// to the peer.
var (
	onePathLocal     = netip.MustParsePrefix("10.58.1.1/24")
	onePathOther     = netip.MustParsePrefix("10.58.2.1/24")
	onePathPeer      = netip.MustParseAddr("10.58.1.2")
	onePathPeerMAC   = net.HardwareAddr{0x02, 0x00, 0x00, 0x00, 0x58, 0x81}
	onePathEtherIPv4 = uint16(0x0800)
)

// onePathLinks creates the two veth pairs, brings them up, addresses p0 and
// o0, and gives the peer a permanent neighbor entry on both, so a frame is
// transmitted on whichever link the route picks without waiting for ARP.
func onePathLinks(t *testing.T) {
	t.Helper()
	for _, pair := range [][2]string{{"p0", "p1"}, {"o0", "o1"}} {
		veth := &netlink.Veth{Name: pair[0], PeerName: pair[1]}
		if err := netlink.LinkAdd(veth); err != nil {
			t.Fatalf("add veth %s-%s: %v", pair[0], pair[1], err)
		}
		for _, name := range pair {
			link := onePathLink(t, name)
			if err := netlink.LinkSetUp(link); err != nil {
				t.Fatalf("%s up: %v", name, err)
			}
		}
	}
	for name, prefix := range map[string]netip.Prefix{"p0": onePathLocal, "o0": onePathOther} {
		link := onePathLink(t, name)
		addr := &netlink.Addr{IPNet: &net.IPNet{IP: prefix.Addr().AsSlice(), Mask: net.CIDRMask(prefix.Bits(), 32)}}
		if err := netlink.AddrAdd(link, addr); err != nil {
			t.Fatalf("address %s on %s: %v", prefix, name, err)
		}
		neigh := &netlink.Neigh{
			LinkIndex:    link.Attrs().Index,
			Family:       unix.AF_INET,
			State:        netlink.NUD_PERMANENT,
			IP:           onePathPeer.AsSlice(),
			HardwareAddr: onePathPeerMAC,
		}
		if err := netlink.NeighAdd(neigh); err != nil {
			t.Fatalf("neighbor %s on %s: %v", onePathPeer, name, err)
		}
	}
}

// onePathDetour installs a /32 route to the peer through o0, more specific
// than p0's connected /24, so the routing table sends the peer's traffic over
// the other link.
func onePathDetour(t *testing.T) {
	t.Helper()
	route := &netlink.Route{
		LinkIndex: onePathLink(t, "o0").Attrs().Index,
		Dst:       &net.IPNet{IP: onePathPeer.AsSlice(), Mask: net.CIDRMask(32, 32)},
		Scope:     netlink.SCOPE_LINK,
	}
	if err := netlink.RouteAdd(route); err != nil {
		t.Fatalf("route %s via o0: %v", onePathPeer, err)
	}
}

// onePathLink looks a link up by name.
func onePathLink(t *testing.T, name string) netlink.Link {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("link %s: %v", name, err)
	}
	return link
}

// onePathCapture opens a packet socket on the named link, which sees every
// frame the other end of its veth pair transmits.
func onePathCapture(t *testing.T, name string) int {
	t.Helper()
	proto := int(onePathHtons(unix.ETH_P_ALL))
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, proto)
	if err != nil {
		t.Fatalf("packet socket for %s: %v", name, err)
	}
	t.Cleanup(func() { unix.Close(fd) }) //nolint:errcheck // Test cleanup; the socket is discarded.
	sa := &unix.SockaddrLinklayer{Protocol: onePathHtons(unix.ETH_P_ALL), Ifindex: onePathLink(t, name).Attrs().Index}
	if err := unix.Bind(fd, sa); err != nil {
		t.Fatalf("bind packet socket to %s: %v", name, err)
	}
	tv := unix.NsecToTimeval((50 * time.Millisecond).Nanoseconds())
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		t.Fatalf("receive timeout on %s: %v", name, err)
	}
	return fd
}

// onePathHtons converts a 16-bit value to network byte order for AF_PACKET.
func onePathHtons(v uint16) uint16 { return v<<8 | v>>8 }

// onePathSaw reports whether an IPv4 UDP frame to the peer at port 3784
// arrived on the capture within 500 ms.
func onePathSaw(t *testing.T, fd int) bool {
	t.Helper()
	buf := make([]byte, 2048)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			continue
		}
		if onePathIsControl(buf[:n]) {
			return true
		}
	}
	return false
}

// onePathIsControl matches an Ethernet frame carrying IPv4 UDP to the peer at
// the single-hop Control port.
func onePathIsControl(frame []byte) bool {
	const ethLen = 14
	if len(frame) < ethLen+20+8 {
		return false
	}
	if binary.BigEndian.Uint16(frame[12:14]) != onePathEtherIPv4 {
		return false
	}
	ip := frame[ethLen:]
	ihl := int(ip[0]&0x0f) * 4
	if ip[9] != unix.IPPROTO_UDP || len(ip) < ihl+8 {
		return false
	}
	if netip.AddrFrom4([4]byte(ip[16:20])) != onePathPeer {
		return false
	}
	return binary.BigEndian.Uint16(ip[ihl+2:ihl+4]) == UDPPortSingleHopControl
}

// onePathSend starts a single-hop transport bound to device (empty for none)
// and sends one Control-sized datagram to the peer for a session on p0.
func onePathSend(t *testing.T, device string) {
	t.Helper()
	u := &UDP{Bind: netip.AddrPortFrom(netip.IPv4Unspecified(), UDPPortSingleHopControl), Mode: api.SingleHop, Device: device}
	if err := u.Start(); err != nil {
		t.Fatalf("start transport (device %q): %v", device, err)
	}
	t.Cleanup(func() { u.Stop() }) //nolint:errcheck // Test cleanup; the transport is discarded.
	out := Outbound{To: onePathPeer, Interface: "p0", Mode: api.SingleHop, Bytes: make([]byte, 24)}
	if err := u.Send(out); err != nil {
		t.Fatalf("send to %s: %v", onePathPeer, err)
	}
}

// RFC requirement: RFC5881-6-1 positive -- a Control packet for a session on
// p0, sent through the single-hop transport the loop binds to p0, is observed
// on the far end of p0, the one-hop path the session protects.
func TestRFC5881ControlLeavesOnTheProtectedLink(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	onePathLinks(t)
	protected := onePathCapture(t, "p1")
	onePathSend(t, "p0")
	if !onePathSaw(t, protected) {
		t.Fatalf("no Control packet to %s left over p0, the session's one-hop path", onePathPeer)
	}
}

// RFC requirement: RFC5881-6-1 negative -- a /32 route sends the peer's
// traffic over o0, another link, and a Control packet for the session on p0,
// sent through the transport the loop binds to p0, still leaves over p0 and
// never over o0.
func TestRFC5881ControlNeverFollowsARouteOffTheProtectedLink(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	onePathLinks(t)
	onePathDetour(t)
	protected := onePathCapture(t, "p1")
	other := onePathCapture(t, "o1")
	onePathSend(t, "p0")
	if onePathSaw(t, other) {
		t.Fatalf("a Control packet to %s left over o0, off the session's one-hop path", onePathPeer)
	}
	if !onePathSaw(t, protected) {
		t.Fatalf("no Control packet to %s left over p0, the session's one-hop path", onePathPeer)
	}
}

// RFC requirement: RFC5881-6-1 negative -- with the single-hop transport
// bound to no device, as resolveLoopDevices (bfd.go) leaves a loop whose
// default-VRF sessions name more than one interface, a /32 route sends the
// peer's traffic over o0, and a Control packet for the session on p0 still
// leaves over p0 and never over o0.
//
// PREVENTS: an unbound socket following the /32 route and sending the
// session's Control packet over o0. Send pins each single-hop packet to
// Outbound.Interface with an IP_PKTINFO ifindex control message.
func TestRFC5881UnboundLoopControlLeavesOnTheSessionLink(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	onePathLinks(t)
	onePathDetour(t)
	protected := onePathCapture(t, "p1")
	other := onePathCapture(t, "o1")
	onePathSend(t, "")
	if onePathSaw(t, other) {
		t.Fatalf("a Control packet to %s for a session on p0 left over o0: an unbound loop does not pin the session's link", onePathPeer)
	}
	if !onePathSaw(t, protected) {
		t.Fatalf("no Control packet to %s left over p0, the session's one-hop path", onePathPeer)
	}
}
