//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the forwarding entries of a policy-based Child SA
// Related: rfc4301_forward_icmp_integration_linux_test.go -- icmpForwardHostCapture, icmpForwardChild, the protected host and Child SA
// Related: rfc4301_espform_policy_integration_linux_test.go -- newEspPolSender, the peer's ESP-in-UDP sender
// Related: child.go -- appendChildDeliveryPolicies, the producer

package engine

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/net/ipv4"
	"golang.org/x/sys/unix"
)

// transitDeliveryStatTmplMismatch is the kernel counter the Child SA's FWD PROTECT
// policy raises for a cleartext packet that matches it without the SA it requires.
const transitDeliveryStatTmplMismatch = "XfrmInTmplMismatch"

// transitDeliveryPacket is UDP from the outside peer's protected address (10.1.0.5,
// port 40000) to the protected host (10.2.0.2, port 5000), inside the Child SA's
// inbound selectors, tagged with id.
func transitDeliveryPacket(id uint16) []byte {
	packet := sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(sadSelPort))
	copy(packet[16:20], net.ParseIP(icmpForwardHost).To4())
	fragmentPacketID(packet, id)
	return packet
}

// transitDeliveryArrived reports whether the protected host's ingress saw the UDP
// packet tagged id from the peer's protected address within fragmentCaptureWait. At
// most 64 frames are read.
func transitDeliveryArrived(t *testing.T, fd int, id uint16) bool {
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
			t.Fatalf("wait on the protected host's capture: %v", err)
		}
		if ready == 0 {
			return false
		}
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			t.Fatalf("read the protected host's ingress: %v", err)
		}
		header, err := ipv4.ParseHeader(buf[:n])
		if err != nil {
			t.Fatalf("parse a protected-host IPv4 header: %v", err)
		}
		if header.Protocol == sadSelProtoUDP && header.ID == int(id) &&
			header.Src.Equal(net.ParseIP(sadSelInnerRemote)) && header.Dst.Equal(net.ParseIP(icmpForwardHost)) {
			return true
		}
	}
	return false
}

// TestRFC4301DecryptedTransitReachesProtectedHostUnderDiscard proves that a policy-based
// gateway under `unmatched discard`, the default, delivers a packet inbound processing
// accepted to the protected host behind it, and still refuses the same packet in clear.
//
// RFC 4301 Section 5.2: "Then match the packet against the inbound selectors identified
// by the SAD entry to verify that the received packet is appropriate for the SA via
// which it was received." Section 5.1 applies the outbound steps to "traffic entering
// the implementation via a protected interface and exiting via an unprotected
// interface", which a packet leaving towards a protected host is not.
//
// Method: the netns lab with the protected host 10.2.0.2 behind Ze, the catch-all
// `unmatched discard` (espPolSPD), and a Child SA 10.2.0.0/24 port 5000 to 10.1.0.0/24
// built by the production creator, with no operator entry. Positive: the peer sends
// 10.1.0.5 to 10.2.0.2 inside ESP-in-UDP on the Child SA; the protected host must
// receive it. Without the delivery entry Linux's OUT lookup on the forwarded packet
// meets only the catch-all BLOCK and drops it (XfrmOutPolBlock). Negative: the peer
// sends the same packet in clear; the Child SA's FWD PROTECT policy must refuse it
// (XfrmInTmplMismatch rises by one) and the host must not receive it, so the delivery
// entry admits nothing inbound processing did not.
//
//ze:guest-root
func TestRFC4301DecryptedTransitReachesProtectedHostUnderDiscard(t *testing.T) {
	lab := newEspPolLab(t)
	capture := icmpForwardHostCapture(t, lab)
	espPolSPD(t)
	child := icmpForwardChild(t, 0, true)
	sender := newEspPolSender(t, lab, child)

	lab.enter(t, lab.peer)
	espPolAddr(t, "lo", sadSelInnerRemote+"/32")
	if err := netlink.RouteAdd(&netlink.Route{
		LinkIndex: espPolLink(t, espPolPeerLink).Attrs().Index,
		Dst:       &net.IPNet{IP: net.ParseIP("10.2.0.0").To4(), Mask: net.CIDRMask(24, 32)},
		Gw:        net.ParseIP(espPolZeOuter),
	}); err != nil {
		t.Fatalf("route the peer's cleartext towards the protected host through Ze: %v", err)
	}
	clear := fragmentRawSender(t, sadSelInnerRemote)
	lab.enter(t, lab.ze)

	const protectedID, clearID = 0x7b01, 0x7b02
	sender.sendEncap(t, transitDeliveryPacket(protectedID))
	if !transitDeliveryArrived(t, capture, protectedID) {
		t.Fatalf("a packet inbound processing accepted on the Child SA (10.1.0.5 to 10.2.0.2) never reached the protected host "+
			"(XfrmOutPolBlock=%d, XfrmInNoPols=%d)", sadSelStat(t, "XfrmOutPolBlock"), sadSelStat(t, sadSelStatNoPols))
	}

	before := sadSelStat(t, transitDeliveryStatTmplMismatch)
	fragmentSend(t, clear, transitDeliveryPacket(clearID))
	awaitForwardKernelCounter(t, "cleartext refused by the Child SA's FWD PROTECT policy", uint64(before+1),
		func(t *testing.T) uint64 { return uint64(sadSelStat(t, transitDeliveryStatTmplMismatch)) })
	if transitDeliveryArrived(t, capture, clearID) {
		t.Fatal("cleartext from the unprotected side with the remote source reached the protected host")
	}
}
