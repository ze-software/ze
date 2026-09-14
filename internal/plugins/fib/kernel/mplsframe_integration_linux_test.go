//go:build integration && linux

// Design: docs/architecture/mpls/mpls-kernel.md -- the labeled frame an AF_MPLS proof forwards
// Overview: mplsentry_integration_linux_test.go -- the swap and pop proofs this harness carries
//
// A read-back of an AF_MPLS route says what the backend WROTE. It cannot say
// whether the kernel forwards a labeled frame on it: the kernel accepts a
// labeled frame only on an interface whose net.mpls.conf.<iface>.input sysctl
// is set, so a table of correct entries forwards nothing on a router that never
// set it, and the read-back looks the same either way. So each proof injects a
// labeled frame on the peer end of a veth pair, the kernel receives it on ze's
// end, and the proof reads what the kernel sends back to the peer end. That is
// the shape internal/component/gtsm uses for its ICMP proofs.
//
// Every test that uses this harness runs inside withNetNS, on a locked OS
// thread, because a network namespace is a property of the thread.

package fibkernel

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// The two ends of the pair. Ze's end carries the address, the label space and
// the input sysctl; the peer end is where the test injects and captures.
const (
	mplsZeLink   = "zempls0"
	mplsPeerLink = "zempls1"
	mplsInput    = "/proc/sys/net/mpls/conf/" + mplsZeLink + "/input"
)

// The neighbors the proofs forward to. 10.0.0.2 is the swap and pop next hop
// every entry names; 10.0.0.5 is the bypass next hop the facility-backup swap
// moves to. Each has its own permanent neighbor entry, so the kernel never
// waits for ARP, and its own hardware address, so the destination of a
// forwarded frame says which next hop the kernel chose.
var (
	mplsNextHop   = netip.MustParseAddr("10.0.0.2")
	mplsBypassHop = netip.MustParseAddr("10.0.0.5")
	mplsNextMAC   = net.HardwareAddr{0x02, 0x00, 0x00, 0x00, 0x00, 0x02}
	mplsBypassMAC = net.HardwareAddr{0x02, 0x00, 0x00, 0x00, 0x00, 0x05}
)

// mplsForwardWait bounds every wait for a forwarded frame. A kernel that
// forwards nothing is the failure the test reports, and the bound is several
// times what a veth takes on an idle namespace.
const mplsForwardWait = 3 * time.Second

// mplsSilenceWait is how long a control observation listens before it accepts
// that the kernel forwarded nothing. The same frame is forwarded within
// milliseconds when the kernel accepts it, so the window is generous.
const mplsSilenceWait = 300 * time.Millisecond

// mplsTTL is the TTL every injected label carries. The kernel drops a label
// whose TTL is 1 or less before it looks the entry up (net/mpls/af_mpls.c,
// mpls_forward), and it writes the decremented value into what it forwards.
const mplsTTL = 64

type mplsTestbed struct {
	t         *testing.T
	zeMAC     net.HardwareAddr
	peerIndex int
	injectFD  int
	captureFD int
}

// newMPLSTestbed builds the veth pair in the namespace the caller already
// entered, addresses ze's end with the /24 the swap and pop next hops sit on,
// pins both neighbors, and opens the packet sockets on the peer end.
//
// It SKIPS when a packet socket cannot be opened. The file runs privileged in
// the QEMU and container runs, and a missing capability is not a broken
// product.
func newMPLSTestbed(t *testing.T, h *netlink.Handle) *mplsTestbed {
	t.Helper()

	// The veth ends would otherwise send IPv6 multicast listener reports and
	// neighbor solicitations of their own, and the control observation counts
	// every frame that reaches the peer end.
	disableIPv6(t)

	lo, err := h.LinkByName("lo")
	require.NoError(t, err)
	require.NoError(t, h.LinkSetUp(lo))

	veth := &netlink.Veth{PeerName: mplsPeerLink}
	veth.Name = mplsZeLink
	require.NoError(t, h.LinkAdd(veth), "create the veth pair")
	zeLink, err := h.LinkByName(mplsZeLink)
	require.NoError(t, err)
	peerLink, err := h.LinkByName(mplsPeerLink)
	require.NoError(t, err)
	require.NoError(t, h.LinkSetUp(zeLink))
	require.NoError(t, h.LinkSetUp(peerLink))

	addr, err := netlink.ParseAddr("10.0.0.1/24")
	require.NoError(t, err)
	require.NoError(t, h.AddrAdd(zeLink, addr))
	addPermanentNeighbor(t, h, zeLink, mplsNextHop, mplsNextMAC)
	addPermanentNeighbor(t, h, zeLink, mplsBypassHop, mplsBypassMAC)

	bed := &mplsTestbed{
		t:         t,
		zeMAC:     zeLink.Attrs().HardwareAddr,
		peerIndex: peerLink.Attrs().Index,
	}
	bed.injectFD = bed.openPacketSocket()
	bed.captureFD = bed.openPacketSocket()
	return bed
}

// disableIPv6 turns IPv6 off for every present and future device in the
// namespace. A kernel built without IPv6 has no knob to write and nothing to
// silence.
func disableIPv6(t *testing.T) {
	t.Helper()
	for _, knob := range []string{
		"/proc/sys/net/ipv6/conf/all/disable_ipv6",
		"/proc/sys/net/ipv6/conf/default/disable_ipv6",
	} {
		err := os.WriteFile(knob, []byte("1"), 0o644)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		require.NoError(t, err, "write %s", knob)
	}
}

func addPermanentNeighbor(t *testing.T, h *netlink.Handle, link netlink.Link, addr netip.Addr, mac net.HardwareAddr) {
	t.Helper()
	err := h.NeighSet(&netlink.Neigh{
		LinkIndex:    link.Attrs().Index,
		Family:       unix.AF_INET,
		State:        netlink.NUD_PERMANENT,
		IP:           net.IP(addr.AsSlice()),
		HardwareAddr: mac,
	})
	require.NoError(t, err, "neighbor %s", addr)
}

// enableInput sets net.mpls.conf.zempls0.input, the sysctl without which the
// kernel discards every labeled frame that arrives on the interface.
//
// The product sets this key from the interface's `mpls { enable true }` leaf:
// applySysctl in internal/component/iface/config_sysctl.go emits it on the
// EventBus and the sysctl plugin writes it. Neither is reachable from this
// package, so the proof writes the same key the same way the plugin does, and
// the assertion is about what the kernel does once the key holds 1.
func (b *mplsTestbed) enableInput() {
	b.t.Helper()
	require.NoError(b.t, os.WriteFile(mplsInput, []byte("1"), 0o644), "write %s", mplsInput)
}

// openPacketSocket opens a raw packet socket bound to the peer end, seeing
// every frame in both directions. The read timeout is what turns "the kernel
// never answered" into a test failure rather than a hung run.
func (b *mplsTestbed) openPacketSocket() int {
	b.t.Helper()

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ALL)))
	if err != nil {
		b.t.Skipf("needs CAP_NET_RAW to open a packet socket: %v", err)
	}
	b.t.Cleanup(func() { unix.Close(fd) }) //nolint:errcheck // best-effort cleanup

	addr := &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ALL),
		Ifindex:  b.peerIndex,
	}
	require.NoError(b.t, unix.Bind(fd, addr), "bind packet socket to %s", mplsPeerLink)
	timeout := unix.NsecToTimeval(int64(100 * time.Millisecond))
	require.NoError(b.t, unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &timeout))
	return fd
}

// htons is the byte order a packet socket's protocol field is given in.
func htons(v uint16) uint16 { return v<<8 | v>>8 }

// inject writes one labeled frame out of the peer end, so the kernel receives
// it on ze's end. The frame is addressed to ze's own hardware address: the
// kernel forwards a labeled frame only when it arrived as PACKET_HOST.
func (b *mplsTestbed) inject(labels []uint32, payload []byte) {
	b.t.Helper()

	frame := mplsFrame(b.zeMAC, labels, payload)
	addr := &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ALL),
		Ifindex:  b.peerIndex,
		Halen:    6,
	}
	copy(addr.Addr[:], b.zeMAC)
	require.NoError(b.t, unix.Sendto(b.injectFD, frame, 0, addr), "inject frame")
}

// readForwarded reads one frame the kernel sent to the peer end, or returns
// nil when the read timed out. A frame the test itself injected passes the
// same socket on its way out and is dropped here by its packet type, so what
// comes back is only what ze's end transmitted.
func (b *mplsTestbed) readForwarded(buf []byte) []byte {
	b.t.Helper()

	n, from, err := unix.Recvfrom(b.captureFD, buf, 0)
	if err != nil {
		return nil // the read timeout, or a signal; the caller holds the deadline
	}
	link, ok := from.(*unix.SockaddrLinklayer)
	require.True(b.t, ok, "packet socket answered a %T address", from)
	if link.Pkttype == unix.PACKET_OUTGOING {
		return nil
	}
	if n <= ethernetHeaderLen {
		return nil
	}
	return buf[:n]
}

// awaitForwarded reads frames off the peer end until one satisfies want, and
// returns it whole, ethernet header included, so a caller can read which
// neighbor the kernel sent it to.
func (b *mplsTestbed) awaitForwarded(what string, want func(frame []byte) bool) []byte {
	b.t.Helper()

	buf := make([]byte, 2048)
	deadline := time.Now().Add(mplsForwardWait)
	for time.Now().Before(deadline) {
		frame := b.readForwarded(buf)
		if frame == nil {
			continue
		}
		if want(frame) {
			out := make([]byte, len(frame))
			copy(out, frame)
			return out
		}
	}
	b.t.Fatalf("the kernel forwarded no %s to %s within %s", what, mplsPeerLink, mplsForwardWait)
	return nil
}

// requireNothingForwarded asserts the kernel sent nothing to the peer end
// inside the silence window. On its own this is an absence, and an absence is
// what a dead veth also answers, so every caller pairs it with an
// awaitForwarded on the same entry in the same test.
func (b *mplsTestbed) requireNothingForwarded(why string) {
	b.t.Helper()

	buf := make([]byte, 2048)
	deadline := time.Now().Add(mplsSilenceWait)
	for time.Now().Before(deadline) {
		frame := b.readForwarded(buf)
		if frame == nil {
			continue
		}
		b.t.Fatalf("%s, yet the kernel forwarded a frame of ethertype %#04x to %s",
			why, binary.BigEndian.Uint16(frame[12:14]), mplsPeerLink)
	}
}

const (
	ethernetHeaderLen = 14
	ipv4HeaderLen     = 20
	udpHeaderLen      = 8
	labelEntryLen     = 4
)

// mplsFrame wraps payload in the label stack, outermost label first, and an
// ethernet header addressed to destination. Each entry is the four octets of
// RFC 3032 Section 2.1, with the label in the top 20 bits, a zero traffic
// class, the bottom-of-stack bit on the last entry, and mplsTTL:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                Label                  | TC  |S|       TTL     |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
func mplsFrame(destination net.HardwareAddr, labels []uint32, payload []byte) []byte {
	frame := make([]byte, ethernetHeaderLen+labelEntryLen*len(labels)+len(payload))
	copy(frame[0:6], destination)
	copy(frame[6:12], mplsNextMAC)
	binary.BigEndian.PutUint16(frame[12:14], unix.ETH_P_MPLS_UC)
	off := ethernetHeaderLen
	for i, label := range labels {
		entry := label<<12 | mplsTTL
		if i == len(labels)-1 {
			entry |= 1 << 8
		}
		binary.BigEndian.PutUint32(frame[off:off+labelEntryLen], entry)
		off += labelEntryLen
	}
	copy(frame[off:], payload)
	return frame
}

// labelStack decodes the label stack of a forwarded MPLS frame, outermost
// first, reading entries until the bottom-of-stack bit.
func labelStack(frame []byte) []uint32 {
	var labels []uint32
	for off := ethernetHeaderLen; off+labelEntryLen <= len(frame); off += labelEntryLen {
		entry := binary.BigEndian.Uint32(frame[off : off+labelEntryLen])
		labels = append(labels, entry>>12)
		if entry&(1<<8) != 0 {
			break
		}
	}
	return labels
}

// ipv4UDP builds an IPv4 datagram carrying one UDP payload, the inner packet
// every labeled frame here carries. The IPv4 header is RFC 791 Section 3.1
// with no options, so its checksum covers a fixed 20 octets; the UDP checksum
// stays zero, which RFC 768 permits over IPv4.
func ipv4UDP(source, destination netip.Addr, port uint16, payload []byte) []byte {
	packet := make([]byte, ipv4HeaderLen+udpHeaderLen+len(payload))
	packet[0] = 0x45
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	packet[8] = mplsTTL
	packet[9] = unix.IPPROTO_UDP
	copy(packet[12:16], source.AsSlice())
	copy(packet[16:20], destination.AsSlice())
	binary.BigEndian.PutUint16(packet[10:12], internetChecksum(packet[:ipv4HeaderLen]))
	udp := packet[ipv4HeaderLen:]
	binary.BigEndian.PutUint16(udp[0:2], port)
	binary.BigEndian.PutUint16(udp[2:4], port)
	binary.BigEndian.PutUint16(udp[4:6], uint16(udpHeaderLen+len(payload)))
	copy(udp[udpHeaderLen:], payload)
	return packet
}

// internetChecksum is the one's complement of the one's complement sum of the
// 16-bit words, which RFC 1071 defines and the IPv4 header uses.
func internetChecksum(data []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(data); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(data[i : i+2]))
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	return ^uint16(sum)
}

// isMPLS reports whether a forwarded frame carries a label stack.
func isMPLS(frame []byte) bool {
	return binary.BigEndian.Uint16(frame[12:14]) == unix.ETH_P_MPLS_UC
}

// isIPv4 reports whether a forwarded frame carries a bare IPv4 packet, which
// is what a pop leaves behind.
func isIPv4(frame []byte) bool {
	return binary.BigEndian.Uint16(frame[12:14]) == unix.ETH_P_IP
}

// listenUDP opens a UDP socket on ze's address inside the namespace and hands
// back the port the kernel chose, so an egress-pop proof can inject an inner
// packet addressed to it and read the datagram the kernel delivered. The
// socket is created on the locked thread, so it lives in the test namespace.
func listenUDP(t *testing.T) (net.PacketConn, uint16) {
	t.Helper()
	var config net.ListenConfig
	conn, err := config.ListenPacket(context.Background(), "udp4", "10.0.0.1:0")
	require.NoError(t, err, "listen on ze's address inside the namespace")
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // best-effort cleanup
	port, err := netip.ParseAddrPort(conn.LocalAddr().String())
	require.NoError(t, err)
	return conn, port.Port()
}

// awaitDatagram reads one datagram off conn within the forward wait and
// returns its payload, or fails the test when nothing was delivered.
func awaitDatagram(t *testing.T, conn net.PacketConn) []byte {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(mplsForwardWait)))
	buf := make([]byte, 2048)
	n, _, err := conn.ReadFrom(buf)
	require.NoError(t, err, "the kernel delivered no popped datagram within %s", mplsForwardWait)
	return buf[:n]
}

// requireNoDatagram asserts nothing reached conn inside the silence window.
// An absence on its own is what a closed socket also answers, so every caller
// pairs it with an awaitDatagram on the same socket in the same test.
func requireNoDatagram(t *testing.T, conn net.PacketConn, why string) {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(mplsSilenceWait)))
	buf := make([]byte, 2048)
	n, _, err := conn.ReadFrom(buf)
	if err == nil {
		t.Fatalf("%s, yet the kernel delivered a %d-octet datagram", why, n)
	}
	var netErr net.Error
	require.ErrorAs(t, err, &netErr)
	require.True(t, netErr.Timeout(), "read failed for another reason than the silence window: %v", err)
}
