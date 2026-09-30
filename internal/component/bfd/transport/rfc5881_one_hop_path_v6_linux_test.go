//go:build linux

// The IPv6 half of the RFC 5881 Section 6 one-hop path tests. It reuses the
// veth pairs of rfc5881_one_hop_path_linux_test.go (p0-p1 the protected link,
// o0-o1 another link) and gives them IPv6 subnets of their own.
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

// The IPv6 one-hop path topology: the peer sits on p0's /64, o0 has a /64 of
// its own, and a /128 route through o0 is the input pushed toward the
// violation.
var (
	onePathLocal6     = netip.MustParsePrefix("fd00:5881:1::1/64")
	onePathOther6     = netip.MustParsePrefix("fd00:5881:2::1/64")
	onePathPeer6      = netip.MustParseAddr("fd00:5881:1::2")
	onePathEtherIPv6  = uint16(0x86dd)
	onePathIPv6Header = 40
)

// onePathLinks6 creates the veth pairs, addresses p0 and o0 with IPv6 (no
// duplicate address detection, so the address is usable at once), gives the
// peer a permanent neighbor entry on both, and installs a /128 route to the
// peer through o0, more specific than p0's connected /64.
func onePathLinks6(t *testing.T) {
	t.Helper()
	onePathLinks(t)
	for name, prefix := range map[string]netip.Prefix{"p0": onePathLocal6, "o0": onePathOther6} {
		link := onePathLink(t, name)
		addr := &netlink.Addr{IPNet: &net.IPNet{IP: prefix.Addr().AsSlice(), Mask: net.CIDRMask(prefix.Bits(), 128)}, Flags: unix.IFA_F_NODAD}
		if err := netlink.AddrAdd(link, addr); err != nil {
			t.Fatalf("address %s on %s: %v", prefix, name, err)
		}
		neigh := &netlink.Neigh{LinkIndex: link.Attrs().Index, Family: unix.AF_INET6, State: netlink.NUD_PERMANENT, IP: onePathPeer6.AsSlice(), HardwareAddr: onePathPeerMAC}
		if err := netlink.NeighAdd(neigh); err != nil {
			t.Fatalf("neighbor %s on %s: %v", onePathPeer6, name, err)
		}
	}
	route := &netlink.Route{
		LinkIndex: onePathLink(t, "o0").Attrs().Index,
		Dst:       &net.IPNet{IP: onePathPeer6.AsSlice(), Mask: net.CIDRMask(128, 128)},
		Scope:     netlink.SCOPE_LINK,
	}
	if err := netlink.RouteAdd(route); err != nil {
		t.Fatalf("route %s via o0: %v", onePathPeer6, err)
	}
}

// onePathSaw6 reports whether an IPv6 UDP frame to the IPv6 peer at the
// single-hop Control port arrived on the capture within 500 ms.
func onePathSaw6(t *testing.T, fd int) bool {
	t.Helper()
	buf := make([]byte, 2048)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			continue
		}
		if onePathIsControl6(buf[:n]) {
			return true
		}
	}
	return false
}

// onePathIsControl6 matches an Ethernet frame carrying IPv6 UDP, with no
// extension header, to the IPv6 peer at the single-hop Control port.
func onePathIsControl6(frame []byte) bool {
	const ethLen = 14
	if len(frame) < ethLen+onePathIPv6Header+8 {
		return false
	}
	if binary.BigEndian.Uint16(frame[12:14]) != onePathEtherIPv6 {
		return false
	}
	ip := frame[ethLen:]
	if ip[6] != unix.IPPROTO_UDP {
		return false
	}
	if netip.AddrFrom16([16]byte(ip[24:40])) != onePathPeer6 {
		return false
	}
	return binary.BigEndian.Uint16(ip[onePathIPv6Header+2:onePathIPv6Header+4]) == UDPPortSingleHopControl
}

// RFC requirement: RFC5881-6-1 negative -- over IPv6, with the single-hop
// transport bound to [::] and no device and a /128 route sending the peer's
// traffic over o0, a Control packet for the session on p0 is never observed on
// the far end of o0 and is observed on the far end of p0.
//
// PREVENTS: an unbound IPv6 socket following the /128 route and sending the
// session's Control packet over o0. Send pins each single-hop packet to
// Outbound.Interface with an IPV6_PKTINFO ifindex control message.
func TestRFC5881IPv6ControlNeverFollowsARouteOffTheSessionLink(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	onePathLinks6(t)
	protected := onePathCapture(t, "p1")
	other := onePathCapture(t, "o1")
	u := &UDP{Bind: netip.AddrPortFrom(netip.IPv6Unspecified(), UDPPortSingleHopControl), Mode: api.SingleHop}
	if err := u.Start(); err != nil {
		t.Fatalf("start IPv6 transport: %v", err)
	}
	t.Cleanup(func() { u.Stop() }) //nolint:errcheck // Test cleanup; the transport is discarded.
	out := Outbound{To: onePathPeer6, Interface: "p0", Mode: api.SingleHop, Bytes: make([]byte, 24)}
	if err := u.Send(out); err != nil {
		t.Fatalf("send to %s: %v", onePathPeer6, err)
	}
	if onePathSaw6(t, other) {
		t.Fatalf("a Control packet to %s for a session on p0 left over o0, off the session's one-hop path", onePathPeer6)
	}
	if !onePathSaw6(t, protected) {
		t.Fatalf("no Control packet to %s left over p0, the session's one-hop path", onePathPeer6)
	}
}
