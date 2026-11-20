//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- protected-host forwarding and SPD dispositions.
// Related: fragment_boundary_integration_linux_test.go -- production Child SA creator and ordinary fragment sender.
// Related: child_forward_policy_linux_test.go -- forwarding setup and exact kernel-counter waits.
// Packet-level conformance probe; its forward-bypass phase needs Ze's kernel (K3 fragment guard).

package engine

import (
	"encoding/binary"
	"errors"
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
	var etherType [2]byte
	binary.BigEndian.PutUint16(etherType[:], unix.ETH_P_IP)
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
