// VALIDATES: the DHCPv4 client keeps reading after a frame whose IPv4 header
// declares fewer payload bytes than a UDP header needs. The client sends its
// DISCOVER, the lab writes the short frame, and the OFFER that follows still
// drives ze to send a REQUEST.
// PREVENTS: a vendored nclient4 that computes a negative DHCP length from such
// a frame. uio's Consume panics on the negative slice bound, and nclient4 runs
// its receive loop on a goroutine ze does not own, so safeRunV4's recover never
// sees it and the whole process dies. An on-link station can send that frame.

//go:build integration && linux

package ifacedhcp

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/mdlayher/packet"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink" // register the netlink backend so iface.Resolve works
)

const (
	// sfIPv4HeaderLen is the header of every frame this lab writes: no options.
	sfIPv4HeaderLen = 20
	// sfUDPHeaderLen is what the vendored guard compares the IPv4 payload
	// length against.
	sfUDPHeaderLen = 8
	// sfShortPayload declares four IPv4 payload bytes. The pre-fix subtraction
	// is 4 - 8, which reaches uio's Consume as a negative slice bound.
	sfShortPayload = 4
	// sfFrameFloor pads the short frame past the bytes its own header claims,
	// so the receiver holds a full UDP header to read and reaches the
	// subtraction. An Ethernet segment pads a runt to 60 bytes on the wire; the
	// lab writes the padding itself rather than depending on a veth doing it.
	sfFrameFloor = 46
)

var (
	sfServerIP = net.IP{192, 0, 2, 1}
	sfClientIP = net.IP{192, 0, 2, 50}
)

// sfLabSeq keeps two labs in one run from naming the same link.
var sfLabSeq atomic.Int32

// TestDHCPv4SurvivesShortIPv4Payload drives the real client over a veth pair.
// The guard on the vendored file is asserted separately, and cheaply, by
// TestNclient4ShortIPv4PayloadGuard (internal/le/qemu/guestlabs_test.go).
func TestDHCPv4SurvivesShortIPv4Payload(t *testing.T) {
	zeSide, peerConn := sfSetupLab(t)

	bus := &recordingBus{}
	client, err := newDHCPClient(zeSide, "0", bus, true, false, dHCPConfig{})
	if err != nil {
		t.Fatalf("newDHCPClient(%s): %v", zeSide, err)
	}
	if err := client.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(client.Stop)

	// The DISCOVER proves the receive loop is up, and it carries the
	// transaction id and hardware address the OFFER must echo.
	discover := sfReadDHCP(t, peerConn, dhcpv4.MessageTypeDiscover, 10*time.Second)

	// The frame that panicked the loop. It is written between the DISCOVER and
	// the OFFER, so a reply that arrives after it can only be read by a loop
	// that survived it.
	if err := sfWrite(peerConn, sfShortFrame()); err != nil {
		t.Fatalf("write short frame: %v", err)
	}

	offer, err := dhcpv4.NewReplyFromRequest(discover,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
		dhcpv4.WithServerIP(sfServerIP),
		dhcpv4.WithYourIP(sfClientIP),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(sfServerIP)),
		dhcpv4.WithOption(dhcpv4.OptSubnetMask(net.CIDRMask(24, 32))),
		dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(10*time.Minute)),
	)
	if err != nil {
		t.Fatalf("build offer: %v", err)
	}
	if err := sfWrite(peerConn, sfUDPFrame(offer.ToBytes())); err != nil {
		t.Fatalf("write offer: %v", err)
	}

	request := sfReadDHCP(t, peerConn, dhcpv4.MessageTypeRequest, 10*time.Second)
	if request.TransactionID != discover.TransactionID {
		t.Fatalf("REQUEST xid = %v, want the DISCOVER's %v",
			request.TransactionID, discover.TransactionID)
	}
	if got := request.RequestedIPAddress(); !got.Equal(sfClientIP) {
		t.Fatalf("REQUEST asks for %v, want the offered %v", got, sfClientIP)
	}
}

// sfSetupLab builds a veth pair and returns the name of the link the client
// binds to plus a raw socket on the other end. Every frame written to that
// socket arrives on the client's link.
//
// The lab stays in the namespace the test runs in, unlike its siblings under
// internal/plugins/iface/netlink. A namespace is a property of a THREAD, and
// the client under test hands its work to goroutines the runtime schedules on
// any thread, so a namespace this function entered would hold the veth while
// the DHCP worker looked for it somewhere else.
func sfSetupLab(t *testing.T) (string, *packet.Conn) {
	t.Helper()

	if err := iface.LoadBackend("netlink"); err != nil {
		t.Fatalf("load iface backend: %v", err)
	}
	t.Cleanup(func() { _ = iface.CloseBackend() })

	// Unique per lab: the pair is deleted on cleanup, and a name reused inside
	// one run would collide with a pair whose cleanup has not run yet.
	suffix := (os.Getpid() % 1000) + int(sfLabSeq.Add(1))*1000
	zeSide := fmt.Sprintf("zedhcpz%d", suffix)
	peerSide := fmt.Sprintf("zedhcpp%d", suffix)

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: zeSide},
		PeerName:  peerSide,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		t.Skipf("add veth (needs CAP_NET_ADMIN): %v", err)
	}
	// Deleting one end deletes the pair.
	t.Cleanup(func() { _ = netlink.LinkDel(veth) })
	for _, name := range []string{zeSide, peerSide} {
		link, lerr := netlink.LinkByName(name)
		if lerr != nil {
			t.Fatalf("LinkByName(%s): %v", name, lerr)
		}
		if serr := netlink.LinkSetUp(link); serr != nil {
			t.Fatalf("LinkSetUp(%s): %v", name, serr)
		}
	}

	peerIfc, err := net.InterfaceByName(peerSide)
	if err != nil {
		t.Fatalf("InterfaceByName(%s): %v", peerSide, err)
	}
	// Datagram matches what nclient4 opens on the other end: the kernel writes
	// the Ethernet header, so this lab reads and writes IP packets.
	conn, err := packet.Listen(peerIfc, packet.Datagram, unix.ETH_P_IP, nil)
	if err != nil {
		t.Skipf("raw socket on %s (needs CAP_NET_RAW): %v", peerSide, err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return zeSide, conn
}

// sfReadDHCP returns the first DHCP message of the wanted type the client sent.
// Frames of any other kind are skipped: the lab shares the namespace it runs
// in, so IPv4 carries more than this exchange.
func sfReadDHCP(t *testing.T, conn *packet.Conn, want dhcpv4.MessageType, within time.Duration) *dhcpv4.DHCPv4 {
	t.Helper()

	deadline := time.Now().Add(within)
	if err := conn.SetReadDeadline(deadline); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	buf := make([]byte, 2048)
	for time.Now().Before(deadline) {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			t.Fatalf("waiting for %s: %v", want, err)
		}
		msg, ok := sfParseDHCP(buf[:n])
		if !ok || msg.MessageType() != want {
			continue
		}
		return msg
	}
	t.Fatalf("no %s within %s", want, within)
	return nil
}

// sfParseDHCP reads an IPv4 frame as a DHCP message sent to the server port.
func sfParseDHCP(frame []byte) (*dhcpv4.DHCPv4, bool) {
	if len(frame) < sfIPv4HeaderLen {
		return nil, false
	}
	hlen := int(frame[0]&0x0f) * 4
	if hlen < sfIPv4HeaderLen || len(frame) < hlen+sfUDPHeaderLen {
		return nil, false
	}
	if frame[9] != unix.IPPROTO_UDP {
		return nil, false
	}
	udp := frame[hlen:]
	if binary.BigEndian.Uint16(udp[2:4]) != dhcpv4.ServerPort {
		return nil, false
	}
	msg, err := dhcpv4.FromBytes(udp[sfUDPHeaderLen:])
	if err != nil {
		return nil, false
	}
	return msg, true
}

// sfShortFrame is the frame under test: an IPv4 header declaring four payload
// bytes, eight bytes that read as a UDP header addressed to the client port,
// and padding so the receiver holds more bytes than the header claims.
func sfShortFrame() []byte {
	frame := sfIPv4Header(sfShortPayload)
	frame = append(frame, sfUDPHeader(0)...)
	for len(frame) < sfFrameFloor {
		frame = append(frame, 0)
	}
	return frame
}

// sfUDPFrame wraps a DHCP message in the UDP and IPv4 headers a server sends.
func sfUDPFrame(payload []byte) []byte {
	frame := sfIPv4Header(sfUDPHeaderLen + len(payload))
	frame = append(frame, sfUDPHeader(len(payload))...)
	return append(frame, payload...)
}

// sfIPv4Header writes a header whose total length counts payloadLen bytes after
// it. The short frame declares a payload no UDP header fits in, which is the
// point of the test, so this never derives the length from what follows.
func sfIPv4Header(payloadLen int) []byte {
	h := make([]byte, sfIPv4HeaderLen)
	h[0] = 0x45 // IPv4, 20-byte header
	binary.BigEndian.PutUint16(h[2:4], uint16(sfIPv4HeaderLen+payloadLen))
	h[8] = 64 // TTL
	h[9] = unix.IPPROTO_UDP
	copy(h[12:16], sfServerIP.To4())
	copy(h[16:20], net.IPv4bcast.To4())
	binary.BigEndian.PutUint16(h[10:12], sfChecksum(h))
	return h
}

// sfUDPHeader addresses the client port, which is what BroadcastRawUDPConn is
// bound to: a frame for any other port is dropped before the subtraction. The
// checksum stays zero, which IPv4 allows and the receiver never reads.
func sfUDPHeader(payloadLen int) []byte {
	h := make([]byte, sfUDPHeaderLen)
	binary.BigEndian.PutUint16(h[0:2], dhcpv4.ServerPort)
	binary.BigEndian.PutUint16(h[2:4], dhcpv4.ClientPort)
	binary.BigEndian.PutUint16(h[4:6], uint16(sfUDPHeaderLen+payloadLen))
	return h
}

// sfChecksum is the one's complement sum every IPv4 header carries.
func sfChecksum(header []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(header); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(header[i : i+2]))
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

// sfWrite sends one IPv4 frame to the broadcast address, which is where a DHCP
// server answers a client that holds no address yet.
func sfWrite(conn *packet.Conn, frame []byte) error {
	_, err := conn.WriteTo(frame, &packet.Addr{HardwareAddr: net.HardwareAddr{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	}})
	return err
}
