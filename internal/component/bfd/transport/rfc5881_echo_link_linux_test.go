//go:build linux

// VALIDATES: RFC 5881 Section 4 and Section 6 on the wire, over the veth
// topology of rfc5881_one_hop_path_linux_test.go: the datalink address an Echo
// packet is sent to, and the addresses a Control packet carries on the
// multiaccess link the session protects. The capture on p1 reads the whole
// Ethernet frame the transport put on p0.
// PREVENTS: an Echo packet the remote system never receives, and a Control
// packet whose source address belongs to another link's subnet.
//
// Each test creates links and routes, so it runs in a user and network
// namespace of its own (userns.Enter).
package transport

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/test/userns"
)

// onePathSubnet is the p0-p1 link's subnet, the multiaccess network the
// session protects.
var onePathSubnet = onePathLocal.Masked()

// onePathFrame is one captured IPv4 UDP frame to the peer: its Ethernet
// destination, its IP source and destination, and its UDP destination port.
type onePathFrame struct {
	mac    [6]byte
	source netip.Addr
	target netip.Addr
	port   uint16
}

// onePathSendPort starts a single-hop transport bound to device (empty for
// none) on port, and sends one datagram to the peer for a session on p0.
func onePathSendPort(t *testing.T, device string, port uint16) {
	t.Helper()
	u := &UDP{Bind: netip.AddrPortFrom(netip.IPv4Unspecified(), port), Mode: api.SingleHop, Device: device}
	if err := u.Start(); err != nil {
		t.Fatalf("start transport on %d (device %q): %v", port, device, err)
	}
	t.Cleanup(func() { u.Stop() }) //nolint:errcheck // Test cleanup; the transport is discarded.
	out := Outbound{To: onePathPeer, Interface: "p0", Mode: api.SingleHop, Bytes: make([]byte, 24)}
	if err := u.Send(out); err != nil {
		t.Fatalf("send to %s:%d: %v", onePathPeer, port, err)
	}
}

// onePathCatch answers the first IPv4 UDP frame to the peer at port seen on
// the capture within 500 ms, and whether one arrived.
func onePathCatch(t *testing.T, fd int, port uint16) (onePathFrame, bool) {
	t.Helper()
	buf := make([]byte, 2048)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			continue
		}
		frame, ok := onePathParse(buf[:n])
		if !ok {
			continue
		}
		if frame.target != onePathPeer {
			continue
		}
		if frame.port == port {
			return frame, true
		}
	}
	return onePathFrame{}, false
}

// onePathParse reads an Ethernet frame carrying IPv4 UDP.
func onePathParse(raw []byte) (onePathFrame, bool) {
	const ethLen = 14
	if len(raw) < ethLen+20+8 {
		return onePathFrame{}, false
	}
	if binary.BigEndian.Uint16(raw[12:14]) != onePathEtherIPv4 {
		return onePathFrame{}, false
	}
	ip := raw[ethLen:]
	ihl := int(ip[0]&0x0f) * 4
	if ip[9] != unix.IPPROTO_UDP || len(ip) < ihl+8 {
		return onePathFrame{}, false
	}
	return onePathFrame{
		mac:    [6]byte(raw[0:6]),
		source: netip.AddrFrom4([4]byte(ip[12:16])),
		target: netip.AddrFrom4([4]byte(ip[16:20])),
		port:   binary.BigEndian.Uint16(ip[ihl+2 : ihl+4]),
	}, true
}

// RFC requirement: RFC5881-4-8 positive -- an Echo packet the single-hop echo
// transport (port 3785) sends for a session on p0 is observed on the far end
// of p0 in an Ethernet frame whose destination address is the remote system's
// link address and whose IP destination is the remote system.
func TestRFC5881EchoFrameAddressedToTheRemoteSystem(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	onePathLinks(t)
	protected := onePathCapture(t, "p1")
	onePathSendPort(t, "p0", UDPPortEcho)
	frame, ok := onePathCatch(t, protected, UDPPortEcho)
	if !ok {
		t.Fatalf("no Echo packet to %s:%d left over p0", onePathPeer, UDPPortEcho)
	}
	if !bytes.Equal(frame.mac[:], onePathPeerMAC) {
		t.Fatalf("Echo frame destination %x, want the remote system's %s", frame.mac, onePathPeerMAC)
	}
}

// RFC requirement: RFC5881-4-8 negative -- with the echo transport bound to no
// device and a /32 route sending the peer's traffic over o0, a link the remote
// system is not on, an Echo packet for the session on p0 is never transmitted
// over o0, and leaves over p0 addressed to the remote system's link address.
func TestRFC5881EchoNeverSentWhereTheRemoteCannotReceiveIt(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	onePathLinks(t)
	onePathDetour(t)
	protected := onePathCapture(t, "p1")
	other := onePathCapture(t, "o1")
	onePathSendPort(t, "", UDPPortEcho)
	if _, ok := onePathCatch(t, other, UDPPortEcho); ok {
		t.Fatalf("an Echo packet to %s for a session on p0 left over o0, where the remote system is not", onePathPeer)
	}
	frame, ok := onePathCatch(t, protected, UDPPortEcho)
	if !ok {
		t.Fatalf("no Echo packet to %s left over p0", onePathPeer)
	}
	if !bytes.Equal(frame.mac[:], onePathPeerMAC) {
		t.Fatalf("Echo frame destination %x, want the remote system's %s", frame.mac, onePathPeerMAC)
	}
}

// RFC requirement: RFC5881-6-2 positive -- on the multiaccess p0 link, a
// Control packet for the session on p0 leaves with an IP source address and an
// IP destination address that are both in p0's subnet.
func TestRFC5881ControlAddressedFromAndToTheSubnet(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	onePathLinks(t)
	protected := onePathCapture(t, "p1")
	onePathSendPort(t, "p0", UDPPortSingleHopControl)
	frame, ok := onePathCatch(t, protected, UDPPortSingleHopControl)
	if !ok {
		t.Fatalf("no Control packet to %s left over p0", onePathPeer)
	}
	if !onePathSubnet.Contains(frame.source) {
		t.Fatalf("Control source %s is not in the link's subnet %s", frame.source, onePathSubnet)
	}
	if !onePathSubnet.Contains(frame.target) {
		t.Fatalf("Control destination %s is not in the link's subnet %s", frame.target, onePathSubnet)
	}
}

// RFC requirement: RFC5881-6-2 negative -- with the transport bound to no
// device and a /32 route through o0, which would give the packet o0's address
// (another subnet) as its source, a Control packet for the session on p0
// never carries a source address outside p0's subnet, on either link.
func TestRFC5881ControlNeverSourcedOffTheSubnet(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	onePathLinks(t)
	onePathDetour(t)
	captures := map[string]int{"p1": onePathCapture(t, "p1"), "o1": onePathCapture(t, "o1")}
	onePathSendPort(t, "", UDPPortSingleHopControl)
	seen := 0
	for link, fd := range captures {
		frame, ok := onePathCatch(t, fd, UDPPortSingleHopControl)
		if !ok {
			continue
		}
		seen++
		if !onePathSubnet.Contains(frame.source) {
			t.Fatalf("Control source %s (captured on %s) is off the session link's subnet %s", frame.source, link, onePathSubnet)
		}
	}
	if seen == 0 {
		t.Fatalf("no Control packet to %s was transmitted", onePathPeer)
	}
}
