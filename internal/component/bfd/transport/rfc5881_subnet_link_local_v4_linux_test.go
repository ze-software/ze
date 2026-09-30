//go:build linux

// The IPv4 link-local case of the RFC 5881 Section 6 destination clause
// (RFC5881-6-2). It shares the veth topology helpers of
// rfc5881_one_hop_path_linux_test.go and rfc5881_echo_link_linux_test.go.
package transport

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/test/userns"
)

// RFC requirement: RFC5881-6-2 negative -- a single-hop session on the
// multiaccess p0 link, which holds no 169.254.0.0/16 address, whose peer is
// the IPv4 link-local 169.254.5.2 is refused by Send with errUDPOffSubnet, and
// no Control packet addressed to that peer is transmitted on p0, even though a
// default route through a gateway on p0 gives the packet a path out of p0.
//
// VALIDATES: an IPv4 link-local peer gets no exemption from the subnet check;
// only IPv6 fe80::/10 is scoped to the link by its address.
// PREVENTS: the kernel forwarding a single-hop Control packet to a gateway on
// p0 with a 169.254 destination outside p0's subnet.
func TestRFC5881ControlNeverAddressedToAnIPv4LinkLocalOffTheSubnet(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	onePathLinks(t)
	linkLocal := netip.MustParseAddr("169.254.5.2")
	gateway := netip.MustParseAddr("10.58.1.254")
	p0 := onePathLink(t, "p0").Attrs().Index
	neigh := &netlink.Neigh{LinkIndex: p0, Family: unix.AF_INET, State: netlink.NUD_PERMANENT, IP: gateway.AsSlice(), HardwareAddr: onePathPeerMAC}
	if err := netlink.NeighAdd(neigh); err != nil {
		t.Fatalf("neighbor %s on p0: %v", gateway, err)
	}
	if err := netlink.RouteAdd(&netlink.Route{LinkIndex: p0, Gw: gateway.AsSlice()}); err != nil {
		t.Fatalf("default route via %s: %v", gateway, err)
	}
	captured := onePathCapture(t, "p1")
	u := &UDP{Bind: netip.AddrPortFrom(netip.IPv4Unspecified(), UDPPortSingleHopControl), Mode: api.SingleHop}
	if err := u.Start(); err != nil {
		t.Fatalf("start transport: %v", err)
	}
	t.Cleanup(func() { u.Stop() }) //nolint:errcheck // Test cleanup; the transport is discarded.
	out := Outbound{To: linkLocal, Interface: "p0", Mode: api.SingleHop, Bytes: make([]byte, 24)}
	err := u.Send(out)
	if !errors.Is(err, errUDPOffSubnet) {
		t.Errorf("Send to %s on p0 (subnet %s): got %v, want errUDPOffSubnet", linkLocal, onePathSubnet, err)
	}
	buf := make([]byte, 2048)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		n, _, rerr := unix.Recvfrom(captured, buf, 0)
		if rerr != nil {
			continue
		}
		frame, ok := onePathParse(buf[:n])
		if ok && frame.target == linkLocal && frame.port == UDPPortSingleHopControl {
			t.Fatalf("a Control packet to %s, outside the link's subnet %s, was transmitted on p0", linkLocal, onePathSubnet)
		}
	}
}
