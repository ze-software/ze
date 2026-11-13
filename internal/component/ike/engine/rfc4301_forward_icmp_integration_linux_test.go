//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- ICMP error messages carried by a Child SA.
// Related: rfc4301_forward_default_integration_linux_test.go -- final-disposition lifecycle.
// Related: fragment_forward_integration_linux_test.go -- shared ordinary AF_PACKET observer.
// RFC naming: untagged -- packet-level conformance probe awaiting guest observation.

package engine

import (
	"bytes"
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
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	icmpForwardHostLink   = "zicmph0"
	icmpForwardRouterLink = "zicmpri0"
	icmpForwardHost       = "10.2.0.2"
	icmpForwardRouter     = "10.2.0.254"
	icmpForwardForged     = "10.9.9.9"
	icmpForwardUnmatched  = "10.7.0.5"
	icmpForwardIfID       = 43
	icmpForwardXFRMLink   = "zicmpx0"
	icmpForwardWait       = 3 * time.Second
)

// icmpForwardHostCapture adds the protected host to the existing private ESP lab.
// Capture runs on the host's ingress, after Ze actually forwarded the inner packet.
// It MUST return the calling locked thread to Ze's namespace.
func icmpForwardHostCapture(t *testing.T, lab *espPolLab) int {
	t.Helper()
	defer lab.enter(t, lab.ze)
	for _, setting := range []struct{ path, value string }{
		{"/proc/sys/net/ipv4/ip_forward", "1"},
		{"/proc/sys/net/ipv4/conf/all/rp_filter", "0"},
		{"/proc/sys/net/ipv4/conf/default/rp_filter", "0"},
		{"/proc/sys/net/ipv4/conf/" + espPolZeLink + "/rp_filter", "0"},
	} {
		if err := os.WriteFile(setting.path, []byte(setting.value), 0o600); err != nil {
			t.Fatalf("prepare the private ICMP gateway %s: %v", setting.path, err)
		}
	}
	host, err := netns.New()
	if err != nil {
		t.Fatalf("create the protected ICMP recipient: %v", err)
	}
	t.Cleanup(func() {
		if err := host.Close(); err != nil {
			t.Errorf("close the ICMP recipient namespace: %v", err)
		}
	})
	lab.enter(t, lab.ze)
	if err := netlink.LinkAdd(&netlink.Veth{
		Name: icmpForwardRouterLink, PeerName: icmpForwardHostLink,
		NumTxQueues: 1, NumRxQueues: 1, PeerNumTxQueues: 1, PeerNumRxQueues: 1,
		PeerNamespace: netlink.NsFd(int(host)),
	}); err != nil {
		t.Fatalf("connect the protected ICMP recipient to Ze: %v", err)
	}
	espPolAddr(t, icmpForwardRouterLink, icmpForwardRouter+"/24")
	espPolUp(t, icmpForwardRouterLink)
	routerMAC := espPolLink(t, icmpForwardRouterLink).Attrs().HardwareAddr
	lab.enter(t, host)
	espPolUp(t, "lo")
	espPolAddr(t, icmpForwardHostLink, icmpForwardHost+"/24")
	espPolUp(t, icmpForwardHostLink)
	hostLink := espPolLink(t, icmpForwardHostLink)
	hostMAC := hostLink.Attrs().HardwareAddr
	if err := netlink.NeighAdd(&netlink.Neigh{
		LinkIndex: hostLink.Attrs().Index, Family: unix.AF_INET,
		State: netlink.NUD_PERMANENT, IP: net.ParseIP(icmpForwardRouter), HardwareAddr: routerMAC,
	}); err != nil {
		t.Fatalf("give the recipient its permanent router neighbor: %v", err)
	}
	capture := fragmentCaptureSocket(t, hostLink.Attrs().Index)
	lab.enter(t, lab.ze)
	if err := netlink.NeighAdd(&netlink.Neigh{
		LinkIndex: espPolLink(t, icmpForwardRouterLink).Attrs().Index, Family: unix.AF_INET,
		State: netlink.NUD_PERMANENT, IP: net.ParseIP(icmpForwardHost), HardwareAddr: hostMAC,
	}); err != nil {
		t.Fatalf("give Ze the ICMP recipient's permanent neighbor: %v", err)
	}
	return capture
}

func icmpForwardChild(t *testing.T, ifID uint32, check bool) *ChildSA {
	t.Helper()
	natt := mbTransport(t, net.JoinHostPort(espPolZeOuter, "4500"), true)
	if err := transport.EnableESPInUDP(natt.Conn()); err != nil {
		t.Fatalf("enable the gateway's ESP-in-UDP receiver: %v", err)
	}
	sa := testSA()
	sa.IsInitiator = true
	sa.ChildInboundSPI, sa.ChildOutboundSPI = 0x6f020001, 0x6f020002
	sa.PeerCfg.SkipICMPErrorCheck = !check
	sa.bindSockets(nil, natt)
	sa.floatToNATTPort()
	sa.NATDetected = false
	sa.peerEndpoint = &net.UDPAddr{IP: net.ParseIP(espPolPeerOuter), Port: transport.NATTPort}
	sa.NegotiatedTSi = mustCIDRNet(t, "10.2.0.0/24")
	sa.NegotiatedTSr = mustCIDRNet(t, "10.1.0.0/24")
	sa.NegotiatedPairs = []tsPair{{
		I: tsSelector{Net: sa.NegotiatedTSi, Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: sadSelPort}, Proto: sadSelProtoUDP},
		R: tsSelector{Net: sa.NegotiatedTSr, Port: ipsec.AnyPort(), Proto: sadSelProtoUDP},
	}}
	group := ipsec.ESPGroup{Name: "forward-icmp-gcm", Lifetime: 3600, PFS: ipsec.PFSDisable,
		Proposals: []ipsec.ESPProposal{{Number: 1, Encryption: ipsec.EncryptionAES256GCM}}}
	child, err := createFirstChildSA(sa, group, espPolZeOuter, espPolPeerOuter,
		ifID, dataplane.Get(), slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("create the real ICMP-test Child SA: %v", err)
	}
	t.Cleanup(child.Clear)
	t.Cleanup(func() { removeChildSA(child, dataplane.Get(), slogutil.DiscardLogger()) })
	if !child.ESPInstalled {
		t.Fatal("the ICMP-test Child SA was not installed")
	}
	state, err := netlink.XfrmStateGet(&netlink.XfrmState{
		Dst: net.ParseIP(espPolZeOuter), Spi: int(child.InboundSPI), Proto: netlink.XFRM_PROTO_ESP,
	})
	if err != nil {
		t.Fatalf("read the real ICMP-test inbound state: %v", err)
	}
	if state.ICMP != check {
		t.Fatalf("inbound XFRM_STATE_ICMP=%v, want the configured %v", state.ICMP, check)
	}
	return child
}

// icmpForwardPacket is an ICMP Destination Unreachable about a UDP datagram the
// protected host sent. Reversing its quoted headers must select remote->local UDP
// and the local destination port sadSelPort. Its outer source matches no Child policy.
func icmpForwardPacket(quotedDestination string, id uint16) []byte {
	quoted := sadSelIPv4(icmpForwardHost, sadSelProtoUDP, sadSelUDP(sadSelSourcePort))
	copy(quoted[16:20], net.ParseIP(quotedDestination).To4())
	binary.BigEndian.PutUint16(quoted[20:22], sadSelPort)
	fragmentPacketID(quoted, id)
	message := make([]byte, 8, 36)
	message[0], message[1] = 3, 1
	message = append(message, quoted[:28]...)
	binary.BigEndian.PutUint16(message[2:4], sadSelChecksum(message))
	packet := sadSelIPv4(icmpForwardForged, sadSelProtoICMP, message)
	copy(packet[16:20], net.ParseIP(icmpForwardHost).To4())
	fragmentPacketID(packet, id)
	return packet
}

func icmpForwardControl(id uint16) []byte {
	packet := sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(sadSelPort))
	copy(packet[16:20], net.ParseIP(icmpForwardHost).To4())
	fragmentPacketID(packet, id)
	return packet
}

// icmpForwardObserve requires the positive UDP egress control. When the error is
// expected too, it waits for both, independently of crypto completion order. A rejected
// error is observed only after its kernel rejection counter moved, before control send.
func icmpForwardObserve(t *testing.T, fd int, packet []byte, controlID uint16, wantError bool) bool {
	t.Helper()
	errorSeen, controlSeen := false, false
	deadline := time.Now().Add(icmpForwardWait)
	buf := make([]byte, 65535)
	for range 64 {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		ready, err := unix.Poll(poll, max(1, int(remaining/time.Millisecond)))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Fatalf("wait for actual protected-host ICMP ingress: %v", err)
		}
		if ready == 0 {
			break
		}
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			t.Fatalf("read the protected host's ingress: %v", err)
		}
		header, err := ipv4.ParseHeader(buf[:n])
		if err != nil {
			t.Fatalf("parse the protected host's received IPv4 packet: %v", err)
		}
		if !header.Dst.Equal(net.ParseIP(icmpForwardHost)) {
			continue
		}
		if header.Protocol == sadSelProtoUDP && header.ID == int(controlID) &&
			header.Src.Equal(net.ParseIP(sadSelInnerRemote)) {
			controlSeen = true
		}
		if header.Protocol == sadSelProtoICMP && header.ID == int(binary.BigEndian.Uint16(packet[4:6])) &&
			header.Src.Equal(net.ParseIP(icmpForwardForged)) {
			if !bytes.Equal(buf[header.Len:n], packet[20:]) {
				t.Fatalf("forwarding changed the ICMP error or quoted packet: got %x, want %x", buf[header.Len:n], packet[20:])
			}
			errorSeen = true
		}
		if controlSeen && (!wantError || errorSeen) {
			return errorSeen
		}
	}
	if !controlSeen {
		t.Fatal("the authenticated UDP forwarding control did not reach the real protected host")
	}
	return errorSeen
}

func icmpForwardRejects(t *testing.T) uint64 {
	t.Helper()
	return uint64(sadSelStat(t, "XfrmInPolBlock") + sadSelStat(t, "XfrmInNoPols") +
		sadSelStat(t, "XfrmInStateMismatch") + sadSelStat(t, "XfrmInTmplMismatch"))
}

func runICMPForwarding(t *testing.T, ifID uint32, check bool) {
	t.Helper()
	lab := newEspPolLab(t)
	capture := icmpForwardHostCapture(t, lab)
	espPolSPD(t)
	// Output to the protected network is explicitly permitted by the ordinary SPD-O
	// producer; otherwise its unrelated OUT discard would mask the FWD check's result.
	entries := map[string]ipsec.SPDPolicy{"icmp-protected-side-output": {
		Name: "icmp-protected-side-output", Action: dataplane.SPActionBypass, Order: 1000,
		Direction: ipsec.SPDDirOut, LocalPrefix: mustCIDRNet(t, "0.0.0.0/0"), LocalPort: ipsec.AnyPort(),
		RemotePrefix: mustCIDRNet(t, "10.2.0.0/24"), RemotePort: ipsec.AnyPort(),
	}}
	installSPDPolicies(dataplane.Get(), nil, entries, slogutil.DiscardLogger())
	t.Cleanup(func() { removeSPDPolicies(dataplane.Get(), entries, slogutil.DiscardLogger()) })
	if ifID != 0 {
		link := &netlink.Xfrmi{Name: icmpForwardXFRMLink, Ifid: ifID}
		if err := netlink.LinkAdd(link); err != nil {
			t.Fatalf("create the route-based receiving interface: %v", err)
		}
		espPolUp(t, icmpForwardXFRMLink)
		state := &ikeEngineState{log: slogutil.DiscardLogger()}
		plan := xfrmInterfacePlan{entries: make(map[spdSelector]*xfrmInterfaceEntry)}
		for _, policy := range interfaceSPDCatchAll(ifID) {
			plan.entries[selectorOf(&policy)] = &xfrmInterfaceEntry{
				iface: icmpForwardXFRMLink, role: dataplane.PolicyRoleInterfaceCatchAll, params: policy,
			}
		}
		if err := state.installXFRMInterfaceEntries(dataplane.Get(), plan); err != nil {
			t.Fatalf("install the route-based interface's production final entries: %v", err)
		}
		t.Cleanup(func() { state.pruneXFRMInterfaceEntries(dataplane.Get(), xfrmInterfacePlan{}) })
	}
	child := icmpForwardChild(t, ifID, check)
	sender := newEspPolSender(t, lab, child)
	// Establish the same real Child FWD/route/egress path before any ICMP conclusion.
	control := icmpForwardControl(0x7100)
	sender.sendEncap(t, control)
	_ = icmpForwardObserve(t, capture, icmpForwardPacket(sadSelInnerRemote, 0x7101), 0x7100, false)
	for index, destination := range []string{sadSelInnerRemote, icmpForwardUnmatched} {
		id := uint16(0x7110 + index*2)
		packet := icmpForwardPacket(destination, id)
		before := icmpForwardRejects(t)
		sender.sendEncap(t, packet)
		consistent := index == 0
		wantDelivery := consistent && check
		if !wantDelivery {
			awaitForwardKernelCounter(t, "ordinary selector or inconsistent quoted-header refusal", before+1, icmpForwardRejects)
		}
		sender.sendEncap(t, icmpForwardControl(id+1))
		got := icmpForwardObserve(t, capture, packet, id+1, wantDelivery)
		if got != wantDelivery {
			t.Errorf("if_id=%d check=%v quoted destination=%s: error reached the protected host=%v, want %v (reject counter %d -> %d)",
				ifID, check, destination, got, wantDelivery, before, icmpForwardRejects(t))
		}
	}
}

// XFRM_STATE_ICMP gates the immutable state-selector fallback before the FWD policy
// lookup. With checking disabled, this UDP Child SA refuses the error's own ICMP
// headers, regardless of its quote. No explicit receive entry authorizes those headers.

//ze:guest-root
func TestRFC4301ForwardedICMPUsesPayloadPolicy(t *testing.T) {
	runICMPForwarding(t, 0, true)
}

//ze:guest-root
func TestRFC4301ForwardedICMPUsesPayloadPolicyOnXFRMInterface(t *testing.T) {
	runICMPForwarding(t, icmpForwardIfID, true)
}

//ze:guest-root
func TestRFC4301ForwardedICMPWithoutStateCheckRequiresOwnHeadersToMatch(t *testing.T) {
	runICMPForwarding(t, 0, false)
}

//ze:guest-root
func TestRFC4301ForwardedICMPWithoutStateCheckRequiresOwnHeadersToMatchOnXFRMInterface(t *testing.T) {
	runICMPForwarding(t, icmpForwardIfID, false)
}
