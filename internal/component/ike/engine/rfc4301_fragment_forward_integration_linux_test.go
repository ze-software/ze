//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- protected-host forwarding and SPD dispositions.
// Related: fragment_boundary_integration_linux_test.go -- production Child SA creator and ordinary fragment sender.
// Related: child_forward_policy_linux_test.go -- forwarding setup and exact kernel-counter waits.
// Related: mh_selector_boundary_integration_linux_test.go -- IPv6 addresses installed without DAD.
// Packet-level conformance probe; its forward-bypass phase and its IPv6 counterpart
// (TestRFC4301ForwardedIPv6FragmentsFollowAPortScopedBypass) need Ze's kernel (K3 fragment guard).

package engine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/net/ipv4"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	fragmentHostLink      = "zefragh0"
	fragmentRouterLink    = "zefragi0"
	fragmentHostAddress   = "10.2.0.2"
	fragmentRouterAddress = "10.2.0.254"
	fragmentCaptureWait   = 3 * time.Second
	// fragmentUnnamedPort is a UDP port no operator entry and no Child SA names, so under
	// unmatched discard its forwarded traffic meets only the catch-all.
	fragmentUnnamedPort = 7100
)

// fragmentForwardPeers adds one protected host to the existing isolated Ze/peer
// lab. The sender owns 10.2.0.2, the outside peer owns 10.1.0.5, and Ze routes
// between them. Both veth queues have one queue, so the later ordinary BYPASS
// datagram is an observation barrier for the preceding cleartext frames.
// The function MUST return the calling locked thread to Ze's namespace.
func fragmentForwardPeers(t *testing.T, lab *espPolLab) (*ipv4.RawConn, int) {
	t.Helper()
	defer lab.enter(t, lab.ze)
	// The router runs IPv4 alone. Its callers count XfrmOutPolBlock for one packet,
	// and IPv6 on its links would send neighbor discovery and MLD through the IPv6
	// catch-all under `unmatched discard`, each send moving that counter as well.
	// The IPv6 counterpart builds its own lab (fragmentForwardIPv6Peers).
	for _, setting := range []struct{ path, value string }{
		{"/proc/sys/net/ipv6/conf/all/disable_ipv6", "1"},
		{"/proc/sys/net/ipv6/conf/default/disable_ipv6", "1"},
		{"/proc/sys/net/ipv4/ip_forward", "1"},
		{"/proc/sys/net/ipv4/conf/all/rp_filter", "0"},
		{"/proc/sys/net/ipv4/conf/default/rp_filter", "0"},
		{"/proc/sys/net/ipv4/conf/lo/rp_filter", "0"},
		{"/proc/sys/net/ipv4/conf/" + espPolZeLink + "/rp_filter", "0"},
	} {
		if err := os.WriteFile(setting.path, []byte(setting.value), 0o600); err != nil {
			t.Fatalf("set the private forwarding router %s: %v", setting.path, err)
		}
	}
	host, err := netns.New()
	if err != nil {
		t.Fatalf("make the private protected-host namespace: %v", err)
	}
	t.Cleanup(func() {
		if err := host.Close(); err != nil {
			t.Errorf("close the protected-host namespace handle: %v", err)
		}
	})
	lab.enter(t, lab.ze)
	link := &netlink.Veth{
		Name: fragmentRouterLink, PeerName: fragmentHostLink,
		NumTxQueues: 1, NumRxQueues: 1,
		PeerNumTxQueues: 1, PeerNumRxQueues: 1,
		PeerNamespace: netlink.NsFd(int(host)),
	}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatalf("join the private protected host to Ze: %v", err)
	}
	espPolAddr(t, fragmentRouterLink, fragmentRouterAddress+"/24")
	espPolUp(t, fragmentRouterLink)
	routerMAC := espPolLink(t, fragmentRouterLink).Attrs().HardwareAddr

	lab.enter(t, lab.peer)
	espPolAddr(t, "lo", sadSelInnerRemote+"/32")
	peerMAC := espPolLink(t, espPolPeerLink).Attrs().HardwareAddr
	capture := fragmentCaptureSocket(t, espPolLink(t, espPolPeerLink).Attrs().Index)
	lab.enter(t, lab.ze)
	if err := netlink.NeighAdd(&netlink.Neigh{
		LinkIndex: espPolLink(t, espPolZeLink).Attrs().Index, Family: unix.AF_INET,
		State: netlink.NUD_PERMANENT, IP: net.ParseIP(espPolPeerOuter), HardwareAddr: peerMAC,
	}); err != nil {
		t.Fatalf("give Ze the outside peer's permanent neighbor: %v", err)
	}

	lab.enter(t, host)
	espPolUp(t, "lo")
	espPolAddr(t, fragmentHostLink, fragmentHostAddress+"/24")
	espPolUp(t, fragmentHostLink)
	hostLink := espPolLink(t, fragmentHostLink)
	if err := netlink.RouteAdd(&netlink.Route{
		LinkIndex: hostLink.Attrs().Index,
		Dst:       mustCIDRNet(t, "10.1.0.0/16"), Gw: net.ParseIP(fragmentRouterAddress),
	}); err != nil {
		t.Fatalf("route the protected host's ordinary UDP through Ze: %v", err)
	}
	if err := netlink.NeighAdd(&netlink.Neigh{
		LinkIndex: hostLink.Attrs().Index, Family: unix.AF_INET,
		State: netlink.NUD_PERMANENT, IP: net.ParseIP(fragmentRouterAddress), HardwareAddr: routerMAC,
	}); err != nil {
		t.Fatalf("give the protected host Ze's permanent neighbor: %v", err)
	}
	return fragmentRawSender(t, fragmentHostAddress), capture
}

// fragmentCaptureSocket observes IP packets received on the outside peer's veth.
// SOCK_DGRAM removes only the link header; the ordinary IPv4 fragment header
// remains observable before the outside host reassembles it. The caller MUST be
// in the outside peer's namespace when this socket is created.
func fragmentCaptureSocket(t *testing.T, ifindex int) int {
	t.Helper()
	return fragmentCaptureSocketFor(t, ifindex, unix.ETH_P_IP)
}

// fragmentCaptureSocketFor is fragmentCaptureSocket for one EtherType: ETH_P_IP or
// ETH_P_IPV6. For IPv6 the Fragment extension header stays observable the same way.
// The caller MUST be in the namespace that owns ifindex.
func fragmentCaptureSocketFor(t *testing.T, ifindex int, ethernetProtocol uint16) int {
	t.Helper()
	var etherType [2]byte
	binary.BigEndian.PutUint16(etherType[:], ethernetProtocol)
	protocol := binary.NativeEndian.Uint16(etherType[:])
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, int(protocol))
	if err != nil {
		t.Fatalf("open the outside peer's packet observation socket: %v", err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Errorf("close the outside peer's packet observation socket: %v", err)
		}
	})
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Ifindex: ifindex, Protocol: protocol}); err != nil {
		t.Fatalf("bind the outside peer's packet observer: %v", err)
	}
	if err := unix.SetsockoptInt(fd, unix.SOL_PACKET, unix.PACKET_IGNORE_OUTGOING, 1); err != nil {
		t.Fatalf("observe only packets arriving from Ze: %v", err)
	}
	return fd
}

// fragmentWire counts only the current test datagram at the peer's actual ingress.
type fragmentWire struct {
	whole, initial, nonInitial, other, protected int
}

// fragmentObserve stops after the subsequent ordinary BYPASS datagram arrives.
// A whole-PROTECT control also waits for its exact ESP SPI, because asynchronous
// encryption may finish after that cleartext barrier. At most 64 received frames
// are considered, within one bounded receive budget.
func fragmentObserve(t *testing.T, fd int, packetID, barrierID uint16, spi uint32, requireESP bool) fragmentWire {
	t.Helper()
	var seen fragmentWire
	barrier := false
	deadline := time.Now().Add(fragmentCaptureWait)
	buf := make([]byte, 65535)
	for range 64 {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("outside observation timed out: barrier=%v, packets=%+v", barrier, seen)
		}
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		ready, err := unix.Poll(poll, max(1, int(remaining/time.Millisecond)))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Fatalf("wait for the private peer's packet observation: %v", err)
		}
		if ready == 0 {
			t.Fatalf("outside observation has no barrier/control: barrier=%v, packets=%+v", barrier, seen)
		}
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			t.Fatalf("read the private peer's ingress packet: %v", err)
		}
		h, err := ipv4.ParseHeader(buf[:n])
		if err != nil {
			t.Fatalf("read the received ordinary IPv4 header: %v", err)
		}
		if h.Protocol == protoESP && n >= h.Len+4 {
			if binary.BigEndian.Uint32(buf[h.Len:h.Len+4]) == spi {
				seen.protected++
			}
		}
		if h.Protocol == sadSelProtoUDP && h.Src.Equal(net.ParseIP(fragmentHostAddress)) && h.Dst.Equal(net.ParseIP(sadSelInnerRemote)) {
			if h.ID == int(barrierID) {
				barrier = true
			}
			if h.ID == int(packetID) {
				switch {
				case h.FragOff == 0 && h.Flags&ipv4.MoreFragments == 0:
					seen.whole++
				case h.FragOff == 0 && h.Flags&ipv4.MoreFragments != 0:
					seen.initial++
				case h.FragOff == fragFirstOctets/8 && h.Flags&ipv4.MoreFragments == 0:
					seen.nonInitial++
				default:
					seen.other++
				}
			}
		}
		if !barrier {
			continue
		}
		if !requireESP {
			return seen
		}
		if seen.protected == 1 {
			return seen
		}
	}
	t.Fatalf("outside observation exceeded its 64-frame bound: barrier=%v, packets=%+v", barrier, seen)
	return seen
}

// fragmentForwardPacket uses the existing ordinary UDP builder with the protected
// host and outside peer as source/destination. Each row gets its own datagram ID.
func fragmentForwardPacket(t *testing.T, port, id uint16) []byte {
	t.Helper()
	packet := sadSelIPv4(fragmentHostAddress, sadSelProtoUDP, sadSelUDP(port))
	copy(packet[16:20], net.ParseIP(sadSelInnerRemote).To4())
	fragmentPacketID(packet, id)
	return packet
}

func fragmentPacketID(packet []byte, id uint16) {
	binary.BigEndian.PutUint16(packet[4:6], id)
	packet[10], packet[11] = 0, 0
	binary.BigEndian.PutUint16(packet[10:12], sadSelChecksum(packet[:20]))
}

// TestRFC4301ForwardedFragmentsRespectOperatorDisposition observes actual egress
// between three private namespaces under unmatched BYPASS. Whole protected UDP
// leaves as ESP, BYPASS UDP is delivered in the clear, and DISCARD UDP is blocked.
// Fragments of protected or discarded traffic must not fall through to cleartext;
// reassembly before protection or exact refusal are permitted for the protected row.
// It then turns unmatched to DISCARD, where only the port-scoped bypass's forward
// entry admits its traffic, and requires the fragments of a bypassed datagram to cross
// and the fragments of no permitted datagram to stay behind
// (fragmentForwardedPortScopedBypass).
// RFC 4301 Section 7.3: "If an implementation does not successfully negotiate
// transmission of non-initial fragments for such an SA, it MUST NOT send such
// fragments over the SA."
// RFC 4301 Section 7.4: "All implementations MUST support DISCARDing of fragments
// using the normal SPD packet classification mechanisms."
//
// RFC requirement: RFC4301-7.4-2 positive -- on Ze's kernel, the kernel holds the
// forward BYPASS entry Ze projects for an operator bypass on one UDP port, and under
// unmatched discard a whole and a fragmented forwarded datagram to that port reach the
// outside peer, while a lone non-initial fragment to that port and a fragmented datagram
// to a port no entry names do not.
//
//ze:guest-root
func TestRFC4301ForwardedFragmentsRespectOperatorDisposition(t *testing.T) {
	lab := newEspPolLab(t)
	sender, capture := fragmentForwardPeers(t, lab)
	espPolSPD(t)
	child := fragmentChildren(t, fragmentPorts{
		local: ipsec.AnyPort(), remote: ipsec.PortSelector{Form: ipsec.PortSingle, Port: sadSelPort},
	})[0]
	// RFC 4301 Sections 4.4.1 and 7.4: use the configured fallback, not a test-only allow.
	if err := installUnmatched(dataplane.Get(), dataplane.SPActionBypass, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("select the production unmatched-BYPASS disposition: %v", err)
	}
	entries := map[string]ipsec.SPDPolicy{
		"forward-fragment-bypass": {
			Name: "forward-fragment-bypass", Action: dataplane.SPActionBypass, Order: 1000,
			Direction: ipsec.SPDDirOut, Protocol: sadSelProtoUDP,
			LocalPrefix: mustCIDRNet(t, "10.2.0.0/16"), LocalPort: ipsec.AnyPort(),
			RemotePrefix: mustCIDRNet(t, "10.1.0.0/16"),
			RemotePort:   ipsec.PortSelector{Form: ipsec.PortSingle, Port: fragmentBypassPort},
		},
		"forward-fragment-discard": {
			Name: "forward-fragment-discard", Action: dataplane.SPActionDiscard, Order: 1001,
			Direction: ipsec.SPDDirOut, Protocol: sadSelProtoUDP,
			LocalPrefix: mustCIDRNet(t, "10.2.0.0/16"), LocalPort: ipsec.AnyPort(),
			RemotePrefix: mustCIDRNet(t, "10.1.0.0/16"),
			RemotePort:   ipsec.PortSelector{Form: ipsec.PortSingle, Port: fragmentDiscardPort},
		},
	}
	// RFC 4301 Sections 4.4.1 and 7.4.
	if err := installSPDPolicies(dataplane.Get(), nil, entries, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("install the operator SPD entries: %v", err)
	}
	t.Cleanup(func() { removeSPDPolicies(dataplane.Get(), entries, slogutil.DiscardLogger()) })
	fragmentWantOperatorPolicies(t, entries)
	lab.enter(t, lab.peer)
	bypassed := sadSelListen(t, sadSelInnerRemote, fragmentBypassPort)
	lab.enter(t, lab.ze)

	for index, tc := range []struct {
		name                   string
		port                   uint16
		split, protect, bypass bool
	}{
		{"whole PROTECT", sadSelPort, false, true, false},
		{"whole BYPASS", fragmentBypassPort, false, false, true},
		{"fragmented BYPASS", fragmentBypassPort, true, false, true},
		{"whole DISCARD", fragmentDiscardPort, false, false, false},
		{"fragmented DISCARD", fragmentDiscardPort, true, false, false},
		{"fragmented PROTECT", sadSelPort, true, true, false},
	} {
		id := uint16(0x6a00 + index*2)
		packet := fragmentForwardPacket(t, tc.port, id)
		blocked := sadSelStat(t, outStepStatPolBlock)
		if tc.split {
			initial, nonInitial := fragSplit(packet)
			fragmentPacketID(initial, id)
			fragmentPacketID(nonInitial, id)
			fragmentSend(t, sender, initial)
			fragmentSend(t, sender, nonInitial)
		} else {
			fragmentSend(t, sender, packet)
		}
		fragmentSend(t, sender, fragmentForwardPacket(t, fragmentBypassPort, id+1))
		seen := fragmentObserve(t, capture, id, id+1, child.OutboundSPI, tc.protect && !tc.split)
		if !sadSelDelivered(t, bypassed) {
			t.Fatalf("%s: the subsequent ordinary BYPASS barrier was not delivered", tc.name)
		}
		if tc.bypass {
			if !sadSelDelivered(t, bypassed) {
				t.Errorf("%s: only one of the ordinary datagram and its barrier was delivered", tc.name)
			}
			whole := seen.whole == 1 && seen.initial == 0 && seen.nonInitial == 0 && seen.other == 0
			fragments := seen.whole == 0 && seen.initial == 1 && seen.nonInitial == 1 && seen.other == 0
			if !whole && !fragments {
				t.Errorf("%s: outside peer saw %+v, want one whole datagram or the complete ordinary fragment pair", tc.name, seen)
			}
			continue
		}
		if seen.whole+seen.initial+seen.nonInitial+seen.other != 0 {
			t.Errorf("%s: traffic that must not leave in clear reached the outside peer: %+v", tc.name, seen)
		}
		if !tc.protect && seen.protected != 0 {
			t.Errorf("%s: discarded traffic unexpectedly left on ESP: %+v", tc.name, seen)
		}
		if !tc.protect && !tc.split {
			if got := sadSelStat(t, outStepStatPolBlock) - blocked; got != 1 {
				t.Errorf("%s: XfrmOutPolBlock moved by %d, want 1", tc.name, got)
			}
		}
	}

	fragmentForwardedPortScopedBypass(t, sender, capture, child.OutboundSPI, bypassed)
}

// fragmentForwardedPortScopedBypass is the forward half of RFC 4301 Section 7.4 for a
// port-scoped bypass. RFC 4301 Section 7.4: "All implementations MUST support stateful
// fragment checking to accommodate BYPASS traffic for which a non-trivial port range is
// specified."
//
// Under unmatched DISCARD the bypass entry's port-scoped FWD gate is the only forward
// policy that admits its traffic, so a fragment of it crosses only when the kernel
// reassembled the datagram before ip_forward's FWD check read its ports: K3's fragment
// guard (capability bit 3), on which spdEntryHasTransit projects the entry. The negatives
// are the fragments that belong to no permitted datagram: a lone non-initial fragment of
// the bypass port, which never completes, and a whole fragment pair of a datagram to a
// port no entry names. The caller MUST be in Ze's namespace.
func fragmentForwardedPortScopedBypass(t *testing.T, sender *ipv4.RawConn, capture int, spi uint32, bypassed *net.UDPConn) {
	t.Helper()
	fragmentWantForwardBypassGate(t)
	// RFC 4301 Sections 4.4.1 and 7.4: use the configured fallback, not a test-only drop.
	if err := installUnmatched(dataplane.Get(), dataplane.SPActionDiscard, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("select the production unmatched-DISCARD disposition: %v", err)
	}

	for index, tc := range []struct {
		name          string
		port          uint16
		initial, tail bool
		bypass        bool
	}{
		{"whole BYPASS under discard", fragmentBypassPort, false, false, true},
		{"fragmented BYPASS under discard", fragmentBypassPort, true, true, true},
		{"lone BYPASS tail under discard", fragmentBypassPort, false, true, false},
		{"fragmented unnamed port under discard", fragmentUnnamedPort, true, true, false},
	} {
		id := uint16(0x6b00 + index*2)
		packet := fragmentForwardPacket(t, tc.port, id)
		initial, nonInitial := fragSplit(packet)
		fragmentPacketID(initial, id)
		fragmentPacketID(nonInitial, id)
		switch {
		case !tc.initial && !tc.tail:
			fragmentSend(t, sender, packet)
		case tc.initial:
			fragmentSend(t, sender, initial)
			fragmentSend(t, sender, nonInitial)
		default:
			fragmentSend(t, sender, nonInitial)
		}
		// The barrier is a whole datagram on the bypass port: under discard it crosses
		// only through the forward entry the projection installed.
		fragmentSend(t, sender, fragmentForwardPacket(t, fragmentBypassPort, id+1))
		seen := fragmentObserve(t, capture, id, id+1, spi, false)
		if !sadSelDelivered(t, bypassed) {
			t.Fatalf("%s: the subsequent whole BYPASS barrier was not delivered", tc.name)
		}
		if tc.bypass {
			if !sadSelDelivered(t, bypassed) {
				t.Errorf("%s: only one of the datagram and its barrier was delivered", tc.name)
			}
			whole := seen.whole == 1 && seen.initial == 0 && seen.nonInitial == 0 && seen.other == 0
			fragments := seen.whole == 0 && seen.initial == 1 && seen.nonInitial == 1 && seen.other == 0
			if !whole && !fragments {
				t.Errorf("%s: outside peer saw %+v, want one whole datagram or the complete fragment pair", tc.name, seen)
			}
			continue
		}
		if seen.whole+seen.initial+seen.nonInitial+seen.other+seen.protected != 0 {
			t.Errorf("%s: a fragment of no permitted datagram reached the outside peer: %+v", tc.name, seen)
		}
	}
}

// fragmentWantForwardBypassGate requires the kernel to hold the forward entry the
// engine projects for the operator bypass on fragmentBypassPort. The engine projects
// it only on a kernel whose capability receipt carries bit 3 (K3's fragment
// gathering), so without it a forward-path fragment test would measure the stock
// kernel's FWD check, which reads the ports of each fragment alone.
func fragmentWantForwardBypassGate(t *testing.T) {
	t.Helper()
	policies, err := dataplane.Get().ListPolicies()
	if err != nil {
		t.Fatalf("read back the forward projection: %v", err)
	}
	for i := range policies {
		p := &policies[i]
		if p.Role == dataplane.PolicyRoleOperator && p.Dir == dataplane.SADirFwd &&
			p.Action == dataplane.SPActionBypass && p.DstPort == dataplane.ExactPortMatch(fragmentBypassPort) {
			return
		}
	}
	t.Fatalf("the kernel holds no forward entry for the bypass on port %d; the engine projects one only on a "+
		"kernel whose capability receipt carries bit 3 (fragment gathering)", fragmentBypassPort)
}

const (
	// The IPv6 counterpart's addresses: the protected host and Ze's side of its link,
	// then Ze's side of the outside link and the outside peer.
	fragmentIPv6HostAddress   = "2001:db8:46::2"
	fragmentIPv6RouterAddress = "2001:db8:46::1"
	fragmentIPv6ZeAddress     = "2001:db8:45::1"
	fragmentIPv6PeerAddress   = "2001:db8:45::5"
	fragmentIPv6HostPrefix    = "2001:db8:46::/64"
	fragmentIPv6PeerPrefix    = "2001:db8:45::/64"
	// fragmentIPv6BodyOctets is every IPv6 row's UDP body. With its 8-octet header the
	// datagram is 40 octets, which fragmentIPv6Split cuts at fragmentIPv6InitialOctets.
	fragmentIPv6BodyOctets = 32
	// fragmentIPv6InitialOctets is the UDP part of the initial fragment: the whole UDP
	// header and 16 body octets, a multiple of eight, so the tail carries no port.
	fragmentIPv6InitialOctets = 24
)

// TestRFC4301ForwardedIPv6FragmentsFollowAPortScopedBypass is the IPv6 counterpart of
// fragmentForwardedPortScopedBypass. K3 gathers IPv6 fragments at PRE_ROUTING as well
// (xfrm6_fragment_prerouting, nf_ct_frag6_gather), so the forward check of an IPv6
// datagram carried in Fragment extension headers reads the ports of the whole datagram.
//
// Method: three private namespaces with IPv6 forwarding in Ze's, every neighbor pinned,
// `unmatched discard`, and one operator bypass for UDP to fragmentBypassPort between the
// protected host's prefix and the outside peer's. The protected host sends each row
// through a raw socket that keeps its Fragment headers, then a whole datagram on the
// bypass port as the barrier. The outside peer's ingress is observed below its own
// reassembly, and its UDP socket on the bypass port reads what was delivered. A whole and
// a fragmented datagram to the bypass port cross, the fragmented one as fragments, and
// the socket receives its payload once before the barrier. A lone non-initial fragment
// to that port and a fragmented datagram to a port no entry names leave nothing at the
// outside peer. No counter is asserted: router solicitations, neighbor discovery and MLD
// from Ze's links meet the IPv6 catch-all too.
//
// RFC 4301 Section 7.4: "All implementations MUST support DISCARDing of fragments using
// the normal SPD packet classification mechanisms.  All implementations MUST support
// stateful fragment checking to accommodate BYPASS traffic for which a non-trivial port
// range is specified."
//
// RFC requirement: RFC4301-7.4-2 positive -- on Ze's kernel, the kernel holds the IPv6
// forward BYPASS entry Ze projects for an operator bypass on one UDP port, and under
// unmatched discard a whole and a fragmented forwarded IPv6 datagram to that port reach
// the outside peer, the fragmented one as fragments whose payload the peer's socket
// receives once, while a lone non-initial IPv6 fragment to that port and a fragmented
// IPv6 datagram to a port no entry names do not.
//
//ze:guest-root
func TestRFC4301ForwardedIPv6FragmentsFollowAPortScopedBypass(t *testing.T) {
	lab := newEspPolLab(t)
	sender, capture, bypassed := fragmentForwardIPv6Peers(t, lab)
	// RFC 4301 Sections 4.4.1 and 7.4: the configured fallback is `unmatched discard`.
	espPolSPD(t)
	entries := map[string]ipsec.SPDPolicy{
		"forward-ipv6-fragment-bypass": {
			Name: "forward-ipv6-fragment-bypass", Action: dataplane.SPActionBypass, Order: 1000,
			Direction: ipsec.SPDDirOut, Protocol: sadSelProtoUDP,
			LocalPrefix: mustCIDRNet(t, fragmentIPv6HostPrefix), LocalPort: ipsec.AnyPort(),
			RemotePrefix: mustCIDRNet(t, fragmentIPv6PeerPrefix),
			RemotePort:   ipsec.PortSelector{Form: ipsec.PortSingle, Port: fragmentBypassPort},
		},
	}
	// RFC 4301 Sections 4.4.1 and 7.4.
	if err := installSPDPolicies(dataplane.Get(), nil, entries, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("install the operator IPv6 SPD entry: %v", err)
	}
	t.Cleanup(func() { removeSPDPolicies(dataplane.Get(), entries, slogutil.DiscardLogger()) })
	fragmentWantOperatorPolicies(t, entries)
	fragmentWantIPv6ForwardBypassGate(t)

	for index, tc := range []struct {
		name          string
		port          uint16
		initial, tail bool
		bypass        bool
	}{
		{"whole IPv6 BYPASS under discard", fragmentBypassPort, false, false, true},
		{"fragmented IPv6 BYPASS under discard", fragmentBypassPort, true, true, true},
		{"lone IPv6 BYPASS tail under discard", fragmentBypassPort, false, true, false},
		{"fragmented IPv6 unnamed port under discard", fragmentUnnamedPort, true, true, false},
	} {
		payload := fragmentIPv6Body(t, "payload", index)
		barrier := fragmentIPv6Body(t, "barrier", index)
		datagram := fragmentIPv6UDP(tc.port, payload)
		initial, tail := fragmentIPv6Split(datagram, 0x6c000000+uint32(index))
		switch {
		case !tc.initial && !tc.tail:
			fragmentIPv6Send(t, sender, fragmentIPv6Packet(unix.IPPROTO_UDP, datagram))
		case tc.initial:
			fragmentIPv6Send(t, sender, initial)
			fragmentIPv6Send(t, sender, tail)
		default:
			fragmentIPv6Send(t, sender, tail)
		}
		// The barrier is a whole datagram on the bypass port: under discard it crosses
		// only through the forward entry the projection installed.
		fragmentIPv6Send(t, sender, fragmentIPv6Packet(unix.IPPROTO_UDP, fragmentIPv6UDP(fragmentBypassPort, barrier)))
		seen := fragmentIPv6Observe(t, capture, payload, barrier)

		// Both veths have one queue, so a delivered row reaches the socket before its
		// barrier, and a second copy of it would be read where the barrier is wanted.
		received := fragmentIPv6Receive(t, bypassed)
		if tc.bypass {
			if !bytes.Equal(received, payload) {
				t.Fatalf("%s: the outside peer's socket read %q first, want the row's payload %q", tc.name, received, payload)
			}
			received = fragmentIPv6Receive(t, bypassed)
		}
		if !bytes.Equal(received, barrier) {
			t.Fatalf("%s: the outside peer's socket read %q, want the barrier %q", tc.name, received, barrier)
		}

		if !tc.bypass {
			if seen.whole+seen.initial+seen.nonInitial+seen.single+seen.other != 0 {
				t.Errorf("%s: a fragment of no permitted datagram reached the outside peer: %+v", tc.name, seen)
			}
			continue
		}
		if !tc.initial {
			if seen != (fragmentIPv6Wire{whole: 1}) {
				t.Errorf("%s: outside peer saw %+v, want the one whole datagram", tc.name, seen)
			}
			continue
		}
		// An IPv6 router never sends a datagram larger than the fragments it gathered
		// it from: ip6_fragment re-emits it within IP6CB(skb)->frag_max_size, keeping
		// the gathered fragments when they still chain as received, or as one fragment
		// when reassembly coalesced them (its slow path raises the size to IPV6_MIN_MTU).
		pair := seen == fragmentIPv6Wire{initial: 1, nonInitial: 1}
		single := seen == fragmentIPv6Wire{single: 1}
		if !pair && !single {
			t.Errorf("%s: outside peer saw %+v, want the fragment pair or one fragment holding the datagram", tc.name, seen)
		}
	}
}

// fragmentForwardIPv6Peers builds the IPv6 forwarding lab on newEspPolLab's two
// namespaces and a third for the protected host. The protected host owns
// fragmentIPv6HostAddress and routes the outside prefix through Ze, Ze forwards, and
// the outside peer owns fragmentIPv6PeerAddress on its veth. Every neighbor a row
// needs is permanent, because Ze's neighbor discovery meets `unmatched discard`, and
// every address skips DAD. The host link has one queue on each side, as the outside
// veth already has, so the barrier datagram orders each row.
//
// It answers the protected host's raw IPv6 sender, the outside peer's ingress capture
// and its UDP socket on fragmentBypassPort, and MUST return the calling locked thread
// to Ze's namespace.
func fragmentForwardIPv6Peers(t *testing.T, lab *espPolLab) (sender, capture int, bypassed *net.UDPConn) {
	t.Helper()
	defer lab.enter(t, lab.ze)
	fragmentIPv6Enable(t, espPolZeLink)
	if err := os.WriteFile("/proc/sys/net/ipv6/conf/all/forwarding", []byte("1"), 0o600); err != nil {
		t.Fatalf("make Ze's namespace an IPv6 router: %v", err)
	}
	host, err := netns.New()
	if err != nil {
		t.Fatalf("make the private IPv6 protected-host namespace: %v", err)
	}
	t.Cleanup(func() {
		if err := host.Close(); err != nil {
			t.Errorf("close the IPv6 protected-host namespace handle: %v", err)
		}
	})
	lab.enter(t, lab.ze)
	if err := netlink.LinkAdd(&netlink.Veth{
		Name: fragmentRouterLink, PeerName: fragmentHostLink,
		NumTxQueues: 1, NumRxQueues: 1,
		PeerNumTxQueues: 1, PeerNumRxQueues: 1,
		PeerNamespace: netlink.NsFd(int(host)),
	}); err != nil {
		t.Fatalf("join the private IPv6 protected host to Ze: %v", err)
	}
	fragmentIPv6Enable(t, fragmentRouterLink)
	mhBoundaryAddress(t, fragmentRouterLink, fragmentIPv6RouterAddress+"/64")
	espPolUp(t, fragmentRouterLink)
	mhBoundaryAddress(t, espPolZeLink, fragmentIPv6ZeAddress+"/64")
	routerMAC := espPolLink(t, fragmentRouterLink).Attrs().HardwareAddr

	lab.enter(t, lab.peer)
	fragmentIPv6Enable(t, espPolPeerLink)
	mhBoundaryAddress(t, espPolPeerLink, fragmentIPv6PeerAddress+"/64")
	peerLink := espPolLink(t, espPolPeerLink)
	capture = fragmentCaptureSocketFor(t, peerLink.Attrs().Index, unix.ETH_P_IPV6)
	bypassed, err = net.ListenUDP("udp6", &net.UDPAddr{IP: net.ParseIP(fragmentIPv6PeerAddress), Port: fragmentBypassPort})
	if err != nil {
		t.Fatalf("open the outside peer's IPv6 bypass-port socket: %v", err)
	}
	t.Cleanup(func() {
		if err := bypassed.Close(); err != nil {
			t.Errorf("close the outside peer's IPv6 bypass-port socket: %v", err)
		}
	})
	lab.enter(t, lab.ze)
	if err := netlink.NeighAdd(&netlink.Neigh{
		LinkIndex: espPolLink(t, espPolZeLink).Attrs().Index, Family: unix.AF_INET6,
		State: netlink.NUD_PERMANENT, IP: net.ParseIP(fragmentIPv6PeerAddress), HardwareAddr: peerLink.Attrs().HardwareAddr,
	}); err != nil {
		t.Fatalf("give Ze the outside peer's permanent IPv6 neighbor: %v", err)
	}

	lab.enter(t, host)
	espPolUp(t, "lo")
	fragmentIPv6Enable(t, fragmentHostLink)
	mhBoundaryAddress(t, fragmentHostLink, fragmentIPv6HostAddress+"/64")
	espPolUp(t, fragmentHostLink)
	hostLink := espPolLink(t, fragmentHostLink)
	if err := netlink.RouteAdd(&netlink.Route{
		LinkIndex: hostLink.Attrs().Index,
		Dst:       mustCIDRNet(t, fragmentIPv6PeerPrefix), Gw: net.ParseIP(fragmentIPv6RouterAddress),
	}); err != nil {
		t.Fatalf("route the protected host's IPv6 UDP through Ze: %v", err)
	}
	if err := netlink.NeighAdd(&netlink.Neigh{
		LinkIndex: hostLink.Attrs().Index, Family: unix.AF_INET6,
		State: netlink.NUD_PERMANENT, IP: net.ParseIP(fragmentIPv6RouterAddress), HardwareAddr: routerMAC,
	}); err != nil {
		t.Fatalf("give the protected host Ze's permanent IPv6 neighbor: %v", err)
	}
	return fragmentIPv6RawSender(t), capture, bypassed
}

// fragmentIPv6Enable turns IPv6 on in the calling thread's namespace, for new links
// and for link, which already exists there.
func fragmentIPv6Enable(t *testing.T, link string) {
	t.Helper()
	for _, conf := range []string{"all", "default", link} {
		path := "/proc/sys/net/ipv6/conf/" + conf + "/disable_ipv6"
		if err := os.WriteFile(path, []byte("0"), 0o600); err != nil {
			t.Fatalf("enable IPv6 through %s: %v", path, err)
		}
	}
}

// fragmentIPv6RawSender opens a raw IPv6 socket whose packets keep the headers the
// test wrote, Fragment headers included. The caller MUST be in the protected host's
// namespace; the socket keeps it when the thread returns to Ze's.
func fragmentIPv6RawSender(t *testing.T) int {
	t.Helper()
	fd, err := unix.Socket(unix.AF_INET6, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.IPPROTO_RAW)
	if err != nil {
		t.Fatalf("open the protected host's raw IPv6 sender: %v", err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Errorf("close the protected host's raw IPv6 sender: %v", err)
		}
	})
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_HDRINCL, 1); err != nil {
		t.Fatalf("keep the protected host's IPv6 headers: %v", err)
	}
	return fd
}

func fragmentIPv6Send(t *testing.T, fd int, packet []byte) {
	t.Helper()
	target := &unix.SockaddrInet6{}
	copy(target.Addr[:], net.ParseIP(fragmentIPv6PeerAddress).To16())
	if err := unix.Sendto(fd, packet, 0, target); err != nil {
		t.Fatalf("send the protected host's IPv6 packet: %v", err)
	}
}

// fragmentIPv6Body is one row's UDP body, distinct per kind and row, so the capture
// and the socket tell the row's datagram from its barrier.
func fragmentIPv6Body(t *testing.T, kind string, row int) []byte {
	t.Helper()
	body := []byte(fmt.Sprintf("ze rfc4301 7.4 ipv6 %s %04x", kind, row))
	if len(body) != fragmentIPv6BodyOctets {
		t.Fatalf("IPv6 row body %q is %d octets, want %d", body, len(body), fragmentIPv6BodyOctets)
	}
	return body
}

// fragmentIPv6UDP returns a UDP datagram from the protected host's sadSelSourcePort to
// port on the outside peer. IPv6 has no checksum-free UDP, so it carries the checksum
// over the RFC 8200 Section 8.1 pseudo-header:
//
//	 0      16     32     36  39
//	+------+------+------+---+-+
//	| src  | dst  | len  | 0 |17|  then the UDP header and body
//	+------+------+------+---+-+
func fragmentIPv6UDP(port uint16, body []byte) []byte {
	segment := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint16(segment[0:2], sadSelSourcePort)
	binary.BigEndian.PutUint16(segment[2:4], port)
	binary.BigEndian.PutUint16(segment[4:6], uint16(8+len(body)))
	segment = append(segment, body...)
	pseudo := make([]byte, 40, 40+len(segment)+1)
	copy(pseudo[0:16], net.ParseIP(fragmentIPv6HostAddress).To16())
	copy(pseudo[16:32], net.ParseIP(fragmentIPv6PeerAddress).To16())
	binary.BigEndian.PutUint32(pseudo[32:36], uint32(len(segment)))
	pseudo[39] = unix.IPPROTO_UDP
	pseudo = append(pseudo, segment...)
	if len(pseudo)%2 != 0 {
		pseudo = append(pseudo, 0)
	}
	checksum := sadSelChecksum(pseudo)
	if checksum == 0 {
		checksum = 0xffff
	}
	binary.BigEndian.PutUint16(segment[6:8], checksum)
	return segment
}

// fragmentIPv6Packet returns an IPv6 packet from the protected host to the outside peer:
// version 6 at octet 0, payload length at 4, next header at 6, hop limit at 7, then
// source at 8 and destination at 24.
func fragmentIPv6Packet(next byte, payload []byte) []byte {
	header := make([]byte, 40, 40+len(payload))
	header[0], header[6], header[7] = 0x60, next, 64
	binary.BigEndian.PutUint16(header[4:6], uint16(len(payload)))
	copy(header[8:24], net.ParseIP(fragmentIPv6HostAddress).To16())
	copy(header[24:40], net.ParseIP(fragmentIPv6PeerAddress).To16())
	return append(header, payload...)
}

// fragmentIPv6Split cuts a UDP datagram into its two fragments under identifier. The
// Fragment header (RFC 8200 Section 4.5) is next header at octet 0, the offset in
// 8-octet units in the top 13 bits of octets 2-3 with the M flag in the lowest bit,
// and the identification at 4-7. The initial fragment holds the UDP header and the
// first 16 body octets, with M set; the tail holds the rest at offset 24.
func fragmentIPv6Split(datagram []byte, identifier uint32) (initial, tail []byte) {
	fragment := func(part []byte, offsetAndMore uint16) []byte {
		header := make([]byte, 8, 8+len(part))
		header[0] = unix.IPPROTO_UDP
		binary.BigEndian.PutUint16(header[2:4], offsetAndMore)
		binary.BigEndian.PutUint32(header[4:8], identifier)
		return fragmentIPv6Packet(unix.IPPROTO_FRAGMENT, append(header, part...))
	}
	// An offset in octets that is a multiple of eight is already the 13-bit field
	// shifted into place, so the more-fragments flag is the only bit added.
	return fragment(datagram[:fragmentIPv6InitialOctets], 1),
		fragment(datagram[fragmentIPv6InitialOctets:], fragmentIPv6InitialOctets)
}

// fragmentIPv6Wire counts the current row's packets from the protected host at the
// outside peer's ingress. single is a Fragment header at offset 0 with M clear: one
// fragment holding the whole datagram.
type fragmentIPv6Wire struct {
	whole, initial, nonInitial, single, other int
}

// fragmentIPv6Observe stops at the row's barrier. Fragments carry no ports and the
// router picks a new identification when it re-emits a gathered datagram, so every
// fragment from the protected host to the outside peer before the barrier is the
// row's: the single-queue veths keep that order. Packets with other addresses (Ze's
// own neighbor discovery and MLD) are not the row's. At most 64 received frames are
// considered, within one bounded receive budget.
func fragmentIPv6Observe(t *testing.T, fd int, payload, barrier []byte) fragmentIPv6Wire {
	t.Helper()
	source := net.ParseIP(fragmentIPv6HostAddress).To16()
	target := net.ParseIP(fragmentIPv6PeerAddress).To16()
	var seen fragmentIPv6Wire
	deadline := time.Now().Add(fragmentCaptureWait)
	buf := make([]byte, 65535)
	for range 64 {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("outside IPv6 observation timed out before the barrier: packets=%+v", seen)
		}
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		ready, err := unix.Poll(poll, max(1, int(remaining/time.Millisecond)))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Fatalf("wait for the outside peer's IPv6 packet observation: %v", err)
		}
		if ready == 0 {
			t.Fatalf("outside IPv6 observation has no barrier: packets=%+v", seen)
		}
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			t.Fatalf("read the outside peer's IPv6 ingress packet: %v", err)
		}
		if n < 48 {
			continue
		}
		if !bytes.Equal(buf[8:24], source) {
			continue
		}
		if !bytes.Equal(buf[24:40], target) {
			continue
		}
		switch buf[6] {
		case unix.IPPROTO_UDP:
			body := buf[48:n]
			switch {
			case bytes.Equal(body, barrier):
				return seen
			case bytes.Equal(body, payload):
				seen.whole++
			default:
				seen.other++
			}
		case unix.IPPROTO_FRAGMENT:
			field := binary.BigEndian.Uint16(buf[42:44])
			offset, more := field&^7, field&1 != 0
			switch {
			case offset == 0 && more:
				seen.initial++
			case offset == 0:
				seen.single++
			case offset == fragmentIPv6InitialOctets && !more:
				seen.nonInitial++
			default:
				seen.other++
			}
		default:
			seen.other++
		}
	}
	t.Fatalf("outside IPv6 observation exceeded its 64-frame bound: packets=%+v", seen)
	return seen
}

// fragmentIPv6Receive reads the next datagram the outside peer's bypass-port socket was
// given, and requires the protected host's address and port as its source.
func fragmentIPv6Receive(t *testing.T, conn *net.UDPConn) []byte {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(fragmentCaptureWait)); err != nil {
		t.Fatalf("bound the outside peer's IPv6 socket read: %v", err)
	}
	buf := make([]byte, 256)
	n, from, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read the outside peer's IPv6 bypass-port socket: %v", err)
	}
	if !from.IP.Equal(net.ParseIP(fragmentIPv6HostAddress)) {
		t.Fatalf("the outside peer's IPv6 socket read a datagram from %s, want %s", from.IP, fragmentIPv6HostAddress)
	}
	if from.Port != sadSelSourcePort {
		t.Fatalf("the outside peer's IPv6 socket read a datagram from port %d, want %d", from.Port, sadSelSourcePort)
	}
	return buf[:n]
}

// fragmentWantIPv6ForwardBypassGate requires the kernel to hold the IPv6 forward entry
// the engine projects for the operator bypass on fragmentBypassPort, which it does only
// on a kernel whose capability receipt carries bit 3 (fragmentWantForwardBypassGate).
func fragmentWantIPv6ForwardBypassGate(t *testing.T) {
	t.Helper()
	policies, err := dataplane.Get().ListPolicies()
	if err != nil {
		t.Fatalf("read back the IPv6 forward projection: %v", err)
	}
	for i := range policies {
		p := &policies[i]
		if p.Dst == nil || p.Dst.IP.To4() != nil {
			continue
		}
		if p.Role == dataplane.PolicyRoleOperator && p.Dir == dataplane.SADirFwd &&
			p.Action == dataplane.SPActionBypass && p.DstPort == dataplane.ExactPortMatch(fragmentBypassPort) {
			return
		}
	}
	t.Fatalf("the kernel holds no IPv6 forward entry for the bypass on port %d; the engine projects one only on a "+
		"kernel whose capability receipt carries bit 3 (fragment gathering)", fragmentBypassPort)
}
