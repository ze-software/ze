//go:build integration && linux

// Design: docs/config-reference.md -- TTL security on a configured TCP egress.
package network

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// TestGTSMConfiguredEgressDoesNotDecrement observes the receiving veth in a
// different namespace, not a local delivery or a value read back from a socket.
// RFC 5082 Section 3: "On some architectures, the TTL of control plane originated
// traffic is under some configurations decremented in the forwarding plane.
// The TTL of GTSM-enabled sessions MUST NOT be decremented."
// RFC requirement: RFC5082-3-3 positive -- RealDialer configured with OutTTL 255 sends its SYN and data over the configured IPv4 veth egress with TTL 255 at the adjacent peer; the local output path does not decrement them.
func TestGTSMConfiguredEgressDoesNotDecrement(t *testing.T) {
	path := newTCPEgress(t, "")
	dialer := RealDialer{LocalAddr: &net.TCPAddr{IP: net.IPv4(192, 0, 2, 1)}, OutTTL: 255}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// RFC 5082 Section 3: use the production dialer, including pre-SYN TTL setup.
	conn, err := dialer.DialContext(ctx, "tcp4", path.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer closeOrLog(t, conn)
	accepted, err := path.listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer closeOrLog(t, accepted)
	payload := []byte("gtsm-egress")
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}
	seenSYN, seenData := false, false
	for range 32 {
		packet := path.receive(t)
		if !bytes.Equal(packet[12:16], []byte{192, 0, 2, 1}) {
			continue
		}
		if packet[8] != 255 {
			t.Fatalf("configured local egress decremented TTL: %d", packet[8])
		}
		tcp := packet[int(packet[0]&15)*4:]
		seenSYN = seenSYN || tcp[13]&2 != 0
		seenData = seenData || bytes.Equal(tcp[int(tcp[12]>>4)*4:], payload)
		if seenSYN && seenData {
			return
		}
	}
	t.Fatalf("egress capture incomplete: SYN=%v data=%v", seenSYN, seenData)
}

// tcpEgress owns two isolated namespaces and a packet socket on the peer link.
// The caller stays on its locked sender thread until test cleanup restores it.
type tcpEgress struct {
	listener net.Listener
	capture  int
	peerLink int
}

// newTCPEgress configures a /30 directly connected link with MTU 1500. Every
// socket is opened in its endpoint's namespace, so local routing cannot shortcut
// the observed link. Cleanup MUST restore the original namespace before unlock.
func newTCPEgress(t *testing.T, key string) *tcpEgress {
	t.Helper()
	runtime.LockOSThread()
	original, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := netns.Set(original); err != nil {
			t.Fatalf("restore namespace: %v", err)
		}
		if err := original.Close(); err != nil {
			t.Error(err)
		}
		runtime.UnlockOSThread()
	})
	sender, err := netns.New()
	if err != nil {
		t.Skipf("network namespaces need CAP_SYS_ADMIN: %v", err)
	}
	t.Cleanup(func() { _ = sender.Close() })
	sendHandle, err := netlink.NewHandle()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sendHandle.Close)
	peer, err := netns.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = peer.Close() })
	pair := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: "gtsm-peer", MTU: 1500}, PeerName: "gtsm-send", PeerNamespace: netlink.NsFd(int(sender))}
	if err := netlink.LinkAdd(pair); err != nil {
		t.Fatal(err)
	}
	peerLink, err := netlink.LinkByName("gtsm-peer")
	if err != nil {
		t.Fatal(err)
	}
	peerAddr, err := netlink.ParseAddr("192.0.2.2/30")
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.AddrAdd(peerLink, peerAddr); err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(peerLink); err != nil {
		t.Fatal(err)
	}
	// AF_PACKET/SOCK_DGRAM returns the IPv4 header, without an Ethernet header.
	protocol := int(binary.NativeEndian.Uint16([]byte{0x08, 0x00}))
	capture, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM, protocol)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Close(capture) })
	if err := unix.Bind(capture, &unix.SockaddrLinklayer{Protocol: uint16(protocol), Ifindex: peerLink.Attrs().Index}); err != nil {
		t.Fatal(err)
	}
	if err := unix.SetsockoptTimeval(capture, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 5}); err != nil {
		t.Fatal(err)
	}
	factory := RealListenerFactory{}
	if key != "" {
		factory.MD5Peers = []MD5Peer{{Addr: net.IPv4(192, 0, 2, 1), Key: key}}
	}
	listener, err := factory.Listen(context.Background(), "tcp4", "192.0.2.2:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeOrLog(t, listener) })
	if err := netns.Set(sender); err != nil {
		t.Fatal(err)
	}
	link, err := sendHandle.LinkByName("gtsm-send")
	if err != nil {
		t.Fatal(err)
	}
	address, err := netlink.ParseAddr("192.0.2.1/30")
	if err != nil {
		t.Fatal(err)
	}
	if err := sendHandle.AddrAdd(link, address); err != nil {
		t.Fatal(err)
	}
	if err := sendHandle.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	routes, err := sendHandle.RouteGet(net.IPv4(192, 0, 2, 2))
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0].LinkIndex != link.Attrs().Index {
		t.Fatalf("peer route does not use configured egress: %v", routes)
	}
	return &tcpEgress{listener: listener, capture: capture, peerLink: peerLink.Attrs().Index}
}

// receive returns one complete IPv4 TCP packet captured on the peer's link.
func (p *tcpEgress) receive(t *testing.T) []byte {
	t.Helper()
	var buffer [1600]byte
	for range 64 {
		packet, err := p.receiveFrame(t, buffer[:], 0)
		if err != nil {
			t.Fatalf("capture configured egress: %v", err)
		}
		if packet != nil {
			return bytes.Clone(packet)
		}
	}
	t.Fatal("no TCP packet on configured egress")
	return nil
}

// receiveFrame consumes one frame; nil without error means non-IPv4/TCP traffic.
// A returned packet borrows buffer. Deadline owners pass MSG_DONTWAIT and check
// their deadline between frames, including frames this filter discards.
func (p *tcpEgress) receiveFrame(t *testing.T, buffer []byte, flags int) ([]byte, error) {
	t.Helper()
	n, source, err := unix.Recvfrom(p.capture, buffer, flags)
	if err != nil {
		return nil, err
	}
	link, ok := source.(*unix.SockaddrLinklayer)
	if !ok || link.Ifindex != p.peerLink {
		t.Fatal("packet did not arrive on the configured peer link")
	}
	if n < 40 || buffer[0]>>4 != 4 || buffer[9] != 6 {
		return nil, nil
	}
	size := int(binary.BigEndian.Uint16(buffer[2:4]))
	ipHeader := int(buffer[0]&15) * 4
	if size > n || ipHeader < 20 || size < ipHeader+20 {
		t.Fatal("truncated IPv4/TCP capture")
	}
	tcpHeader := int(buffer[ipHeader+12]>>4) * 4
	if tcpHeader < 20 || size < ipHeader+tcpHeader {
		t.Fatal("truncated TCP options")
	}
	return buffer[:size], nil
}
