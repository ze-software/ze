//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions, transit traffic
// Related: spd_policy.go -- spdEntryPolicies and spdTransitPolicies, the producer
// Related: rfc4301_fragment_forward_integration_linux_test.go -- fragmentForwardPeers, the protected host and outside peer
// Related: spd_transit_test.go -- the unit test of the same projection

package engine

import (
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/net/ipv4"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	// transitPassPort is the destination port of the flows the operator's bypass carries.
	transitPassPort = 5100
	// transitDropPort is the destination port of the flows the operator's discards name.
	transitDropPort = 5101
)

// transitLinkProtocol is etherType as an AF_PACKET address carries it: in network byte
// order, held in a native uint16.
func transitLinkProtocol(etherType uint16) uint16 {
	var wire [2]byte
	binary.BigEndian.PutUint16(wire[:], etherType)
	return binary.NativeEndian.Uint16(wire[:])
}

// transitEgressCapture observes the frames on one of Ze's own links in both directions,
// so a packet Ze forwards out of it is seen as Ze sent it. It binds ETH_P_ALL because
// the kernel hands a transmitted frame only to the ETH_P_ALL taps (dev_queue_xmit_nit
// walks ptype_all and the device's ptype_all), so an ETH_P_IP binding sees ingress
// alone and never the packet Ze forwards. The socket therefore also reads Ze's ARP for
// the protected host, which transitEntryArrived skips. The caller MUST be in Ze's
// namespace.
func transitEgressCapture(t *testing.T, ifindex int) int {
	t.Helper()
	protocol := transitLinkProtocol(unix.ETH_P_ALL)
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, int(protocol))
	if err != nil {
		t.Fatalf("open Ze's egress observation socket: %v", err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Errorf("close Ze's egress observation socket: %v", err)
		}
	})
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Ifindex: ifindex, Protocol: protocol}); err != nil {
		t.Fatalf("bind Ze's egress observer: %v", err)
	}
	return fd
}

// transitEntryPacket is UDP from source to target, destination port port, tagged id.
func transitEntryPacket(source, target string, port, id uint16) []byte {
	packet := sadSelIPv4(source, sadSelProtoUDP, sadSelUDP(port))
	copy(packet[16:20], net.ParseIP(target).To4())
	fragmentPacketID(packet, id)
	return packet
}

// transitEntryArrived reports whether fd saw the UDP packet tagged id from source to
// target within fragmentCaptureWait.
func transitEntryArrived(t *testing.T, fd int, source, target string, id uint16) bool {
	t.Helper()
	return transitArrived(t, fd, sadSelProtoUDP, source, target, id)
}

// transitArrived reports whether fd saw the IPv4 packet of protocol tagged id from
// source to target within fragmentCaptureWait. At most 64 frames are read, and a frame
// that is not IPv4 (the egress capture's ARP) is read and skipped.
func transitArrived(t *testing.T, fd, protocol int, source, target string, id uint16) bool {
	t.Helper()
	deadline := time.Now().Add(fragmentCaptureWait)
	buf := make([]byte, 65535)
	for range 64 {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
		}
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		ready, err := unix.Poll(poll, max(1, int(remaining/time.Millisecond)))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Fatalf("wait on the transit observation: %v", err)
		}
		if ready == 0 {
			return false
		}
		n, from, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			t.Fatalf("read the transit observation: %v", err)
		}
		link, ok := from.(*unix.SockaddrLinklayer)
		if !ok {
			t.Fatalf("the transit observation came from a %T, not a link-layer address", from)
		}
		if link.Protocol != transitLinkProtocol(unix.ETH_P_IP) {
			continue
		}
		header, err := ipv4.ParseHeader(buf[:n])
		if err != nil {
			t.Fatalf("parse an observed IPv4 header: %v", err)
		}
		if header.Protocol == protocol && header.ID == int(id) &&
			header.Src.Equal(net.ParseIP(source)) && header.Dst.Equal(net.ParseIP(target)) {
			return true
		}
	}
	return false
}

// transitWantInstalled fails the test unless the kernel holds every policy
// spdEntryPolicies hands the XFRM backend for each entry, forward entries included,
// with its selector, action and rank, read back through ListPolicies, the listing
// `show vpn ipsec dataplane policy` prints.
func transitWantInstalled(t *testing.T, entries map[string]ipsec.SPDPolicy) {
	t.Helper()
	actual, err := dataplane.Get().ListPolicies()
	if err != nil {
		t.Fatalf("read back the operator policies: %v", err)
	}
	forward := 0
	for name, entry := range entries {
		wanted := spdEntryPolicies(dataplane.Get(), entry)
		for i := range wanted {
			want := &wanted[i]
			if want.Dir == dataplane.SADirFwd {
				forward++
			}
			found := false
			for j := range actual {
				got := &actual[j]
				if got.Role == dataplane.PolicyRoleOperator && got.Dir == want.Dir &&
					got.Action == want.Action && got.SrcPort == want.SrcPort && got.DstPort == want.DstPort &&
					got.UpperProto == want.UpperProto && got.Priority == want.Priority &&
					sameIPNet(got.Src, want.Src) && sameIPNet(got.Dst, want.Dst) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("operator entry %s: the kernel holds no %s policy %v to %v at rank %d",
					name, dirLabel(want.Dir), want.Src, want.Dst, want.Priority)
			}
		}
	}
	if forward == 0 {
		t.Fatal("the entries were handed no forward policy, so the packet rows below would not test the projection")
	}
}

// TestRFC4301OperatorEntryDecidesTransitTraffic proves that an operator `vpn ipsec
// policy` entry decides traffic a gateway forwards between a protected host and the
// outside, in both forwarding directions, under `unmatched discard`, the default.
//
// RFC 4301 Section 6: "Disposition of non-error, ICMP messages (that are not addressed
// to the IPsec implementation itself) MUST be explicitly accounted for using SPD
// entries." Linux checks a forwarded packet in FWD and then in OUT, never in IN, so an
// entry with in and out policies alone decides no forwarded packet from the remote side.
//
// Method: the netns lab with the protected host 10.2.0.2 behind Ze and the outside peer
// 10.1.0.5, the catch-all `unmatched discard` (espPolSPD), no Child SA, and the operator
// entries installed through the reconciler config apply uses: a both-direction UDP
// bypass between 10.2.0.0/24 and 10.1.0.0/24 at order 1000, and two port-scoped UDP
// discards at order 900, one in for local port 5101 and one out for remote port 5101.
// Every policy the XFRM backend is handed must be in the kernel. Then, in each direction,
// cleartext UDP to port 5100 is the positive control: only the operator bypass can carry
// it, since no Child SA covers it and the catch-all drops it, and it must be seen leaving
// Ze towards its destination. UDP to port 5101 is the discarded flow: it must not leave
// Ze, and the drop counter of the policy that decides it rises by one: XfrmInPolBlock
// for the remote-to-local discard, which only its forward entry can stop, and
// XfrmOutPolBlock for the local-to-remote one, which its out policy stops.
//
//ze:guest-root
func TestRFC4301OperatorEntryDecidesTransitTraffic(t *testing.T) {
	lab := newEspPolLab(t)
	hostSender, peerCapture := fragmentForwardPeers(t, lab)
	espPolSPD(t)

	lab.enter(t, lab.peer)
	if err := netlink.RouteAdd(&netlink.Route{
		LinkIndex: espPolLink(t, espPolPeerLink).Attrs().Index,
		Dst:       mustCIDRNet(t, "10.2.0.0/24"),
		Gw:        net.ParseIP(espPolZeOuter),
	}); err != nil {
		t.Fatalf("route the peer's cleartext towards the protected host through Ze: %v", err)
	}
	peerSender := fragmentRawSender(t, sadSelInnerRemote)
	lab.enter(t, lab.ze)
	hostEgress := transitEgressCapture(t, espPolLink(t, fragmentRouterLink).Attrs().Index)

	entries := map[string]ipsec.SPDPolicy{
		"transit-pass": {
			Name: "transit-pass", Action: dataplane.SPActionBypass, Order: 1000,
			Direction: ipsec.SPDDirBoth, Protocol: sadSelProtoUDP,
			LocalPrefix: mustCIDRNet(t, "10.2.0.0/24"), LocalPort: ipsec.AnyPort(),
			RemotePrefix: mustCIDRNet(t, "10.1.0.0/24"), RemotePort: ipsec.AnyPort(),
		},
		"transit-drop-in": {
			Name: "transit-drop-in", Action: dataplane.SPActionDiscard, Order: 900,
			Direction: ipsec.SPDDirIn, Protocol: sadSelProtoUDP,
			LocalPrefix:  mustCIDRNet(t, "10.2.0.0/24"),
			LocalPort:    ipsec.PortSelector{Form: ipsec.PortSingle, Port: transitDropPort},
			RemotePrefix: mustCIDRNet(t, "10.1.0.0/24"), RemotePort: ipsec.AnyPort(),
		},
		"transit-drop-out": {
			Name: "transit-drop-out", Action: dataplane.SPActionDiscard, Order: 900,
			Direction: ipsec.SPDDirOut, Protocol: sadSelProtoUDP,
			LocalPrefix: mustCIDRNet(t, "10.2.0.0/24"), LocalPort: ipsec.AnyPort(),
			RemotePrefix: mustCIDRNet(t, "10.1.0.0/24"),
			RemotePort:   ipsec.PortSelector{Form: ipsec.PortSingle, Port: transitDropPort},
		},
	}
	log := slogutil.DiscardLogger()
	// RFC 4301 Section 6.
	if err := installSPDPolicies(dataplane.Get(), nil, entries, log); err != nil {
		t.Fatalf("install the operator SPD entries: %v", err)
	}
	t.Cleanup(func() { removeSPDPolicies(dataplane.Get(), entries, log) })
	transitWantInstalled(t, entries)

	for _, tc := range []struct {
		name           string
		sender         *ipv4.RawConn
		capture        int
		source, target string
		blockCounter   string
	}{
		{"remote to local", peerSender, hostEgress, sadSelInnerRemote, fragmentHostAddress, inStepStatPolBlock},
		{"local to remote", hostSender, peerCapture, fragmentHostAddress, sadSelInnerRemote, outStepStatPolBlock},
	} {
		passID, dropID := uint16(0x7c01), uint16(0x7c02)
		if tc.sender == hostSender {
			passID, dropID = 0x7c11, 0x7c12
		}
		fragmentSend(t, tc.sender, transitEntryPacket(tc.source, tc.target, transitPassPort, passID))
		if !transitEntryArrived(t, tc.capture, tc.source, tc.target, passID) {
			t.Fatalf("%s: the flow the operator bypass names (%s to %s, port %d) was not forwarded "+
				"(XfrmInPolBlock=%d, XfrmInNoPols=%d, XfrmOutPolBlock=%d)", tc.name, tc.source, tc.target, transitPassPort,
				sadSelStat(t, inStepStatPolBlock), sadSelStat(t, "XfrmInNoPols"), sadSelStat(t, outStepStatPolBlock))
		}

		before := sadSelStat(t, tc.blockCounter)
		fragmentSend(t, tc.sender, transitEntryPacket(tc.source, tc.target, transitDropPort, dropID))
		awaitForwardKernelCounter(t, tc.name+": the flow the operator discard names refused by its policy", uint64(before+1),
			func(t *testing.T) uint64 { return uint64(sadSelStat(t, tc.blockCounter)) })
		if transitEntryArrived(t, tc.capture, tc.source, tc.target, dropID) {
			t.Errorf("%s: the flow the operator discard names (%s to %s, port %d) was forwarded",
				tc.name, tc.source, tc.target, transitDropPort)
		}
	}
}
