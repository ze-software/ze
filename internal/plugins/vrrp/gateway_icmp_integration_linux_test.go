//go:build integration && linux

// Design: docs/architecture/diagnostics/active-probes.md
// Linux is Ze's IPv4 forwarding and ICMP-error producer. These tests enter an
// isolated namespace, configure links through Ze's netlink backend, inject
// Ethernet frames, and inspect both the return path and the forwarding path.
package vrrp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/rtproto"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink" // register the product backend
)

type gatewayWire struct {
	backend iface.Backend
	parent  netlink.Link
	peer    netlink.Link
	fd      int
}

// newGatewayWire MUST be paired with testing cleanup, which closes sockets and
// the backend before restoring the calling thread's namespace.
func newGatewayWire(t *testing.T) gatewayWire {
	t.Helper()
	runtime.LockOSThread()
	original, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("get namespace: %v", err)
	}
	ns, err := netns.New()
	if err != nil {
		_ = original.Close()
		runtime.UnlockOSThread()
		if errors.Is(err, unix.EPERM) {
			t.Skipf("requires CAP_NET_ADMIN: %v", err)
		}
		t.Fatalf("create namespace: %v", err)
	}
	t.Cleanup(func() {
		if err := netns.Set(original); err != nil {
			t.Errorf("restore namespace: %v", err)
		}
		_ = ns.Close()
		_ = original.Close()
		runtime.UnlockOSThread()
	})
	if err := iface.LoadBackend("netlink"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = iface.CloseBackend() })
	b := iface.GetBackend()
	gatewaySysctl(t, "ipv4/ip_forward", "1")
	gatewaySysctl(t, "ipv4/conf/all/rp_filter", "0")
	gatewaySysctl(t, "ipv4/conf/default/rp_filter", "0")
	gatewaySysctl(t, "ipv4/icmp_ratemask", "0")
	if err := b.CreateVeth("gw0", "host0"); err != nil {
		t.Fatal(err)
	}
	if err := b.AddAddress("gw0", "192.0.2.254/24"); err != nil {
		t.Fatal(err)
	}
	gatewaySysctl(t, "ipv4/conf/host0/forwarding", "0")
	parent, peer := gatewayLink(t, "gw0"), gatewayLink(t, "host0")
	return gatewayWire{backend: b, parent: parent, peer: peer, fd: gatewaySocket(t, peer)}
}

func gatewaySysctl(t *testing.T, path, value string) {
	t.Helper()
	if err := osSysctlWrite(filepath.Join(procNetRoot, path), value); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func gatewayLink(t *testing.T, name string) netlink.Link {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatal(err)
	}
	return link
}

func gatewaySocket(t *testing.T, link netlink.Link) int {
	t.Helper()
	protocol := uint16(unix.ETH_P_IP)
	protocol = protocol<<8 | protocol>>8
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, int(protocol))
	if err != nil {
		t.Fatalf("packet socket: %v", err)
	}
	t.Cleanup(func() { _ = unix.Close(fd) })
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Ifindex: link.Attrs().Index, Protocol: protocol}); err != nil {
		t.Fatal(err)
	}
	return fd
}

func gatewayNeighbor(t *testing.T, link netlink.Link, address netip.Addr, mac net.HardwareAddr) {
	t.Helper()
	if err := netlink.NeighSet(&netlink.Neigh{LinkIndex: link.Attrs().Index, IP: net.IP(address.AsSlice()), HardwareAddr: mac, State: netlink.NUD_PERMANENT}); err != nil {
		t.Fatal(err)
	}
}

func (w gatewayWire) routedExit(t *testing.T) int {
	t.Helper()
	if err := w.backend.CreateVeth("gw1", "far0"); err != nil {
		t.Fatal(err)
	}
	if err := w.backend.AddAddress("gw1", "198.51.100.254/24"); err != nil {
		t.Fatal(err)
	}
	out, far := gatewayLink(t, "gw1"), gatewayLink(t, "far0")
	if err := netlink.LinkSetMTU(out, 576); err != nil {
		t.Fatal(err)
	}
	gatewaySysctl(t, "ipv4/conf/far0/forwarding", "0")
	gatewayNeighbor(t, out, netip.MustParseAddr("198.51.100.2"), far.Attrs().HardwareAddr)
	gatewayNeighbor(t, w.parent, netip.MustParseAddr("192.0.2.20"), w.peer.Attrs().HardwareAddr)
	if err := w.backend.AddRoute("gw1", "203.0.113.0/24", "198.51.100.2", 0, rtproto.Static); err != nil {
		t.Fatal(err)
	}
	return gatewaySocket(t, far)
}

func gatewayChecksum(b []byte) uint16 {
	var sum uint32
	for len(b) >= 2 {
		sum += uint32(binary.BigEndian.Uint16(b))
		b = b[2:]
	}
	if len(b) != 0 {
		sum += uint32(b[0]) << 8
	}
	for sum > 0xffff {
		sum = sum>>16 + sum&0xffff
	}
	return ^uint16(sum)
}

func gatewayDatagram(id uint16, size int, ttl byte, flags uint16, options []byte) []byte {
	header := 20 + len(options)
	ip := make([]byte, size)
	ip[0], ip[8], ip[9] = 0x40|byte(header/4), ttl, unix.IPPROTO_UDP
	binary.BigEndian.PutUint16(ip[2:], uint16(size))
	binary.BigEndian.PutUint16(ip[4:], id)
	binary.BigEndian.PutUint16(ip[6:], flags)
	copy(ip[12:16], netip.MustParseAddr("192.0.2.20").AsSlice())
	copy(ip[16:20], netip.MustParseAddr("203.0.113.9").AsSlice())
	copy(ip[20:header], options)
	binary.BigEndian.PutUint16(ip[header:], 31000)
	binary.BigEndian.PutUint16(ip[header+2:], 31001)
	binary.BigEndian.PutUint16(ip[header+4:], uint16(size-header))
	for i := header + 8; i < len(ip); i++ {
		ip[i] = byte(i ^ int(id))
	}
	binary.BigEndian.PutUint16(ip[10:], gatewayChecksum(ip[:header]))
	return ip
}

func (w gatewayWire) send(t *testing.T, mac net.HardwareAddr, ip []byte) {
	t.Helper()
	frame := make([]byte, 14+len(ip))
	copy(frame[:6], mac)
	copy(frame[6:12], w.peer.Attrs().HardwareAddr)
	binary.BigEndian.PutUint16(frame[12:], unix.ETH_P_IP)
	copy(frame[14:], ip)
	if err := unix.Sendto(w.fd, frame, 0, &unix.SockaddrLinklayer{Ifindex: w.peer.Attrs().Index}); err != nil {
		t.Fatal(err)
	}
}

type gatewayCapture struct {
	fd int
	ip []byte
}

// gatewayCapturePackets observes a full bounded window because absence on one
// path is part of the assertion. Every discard test sends a valid control over
// the same path in that window; an empty capture can never pass it.
func gatewayCapturePackets(t *testing.T, fds ...int) []gatewayCapture {
	t.Helper()
	poll := make([]unix.PollFd, len(fds))
	for i, fd := range fds {
		poll[i] = unix.PollFd{Fd: int32(fd), Events: unix.POLLIN}
	}
	var captured []gatewayCapture
	var buf [2048]byte
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := unix.Poll(poll, 50); err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			t.Fatal(err)
		}
		for _, ready := range poll {
			if ready.Revents&unix.POLLIN == 0 {
				continue
			}
			n, from, err := unix.Recvfrom(int(ready.Fd), buf[:], 0)
			if errors.Is(err, unix.EAGAIN) {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			if sa, ok := from.(*unix.SockaddrLinklayer); ok && sa.Pkttype == unix.PACKET_OUTGOING {
				continue
			}
			if n < 34 || binary.BigEndian.Uint16(buf[12:14]) != unix.ETH_P_IP {
				continue
			}
			ip := buf[14:n]
			length := int(binary.BigEndian.Uint16(ip[2:4]))
			header := int(ip[0]&15) * 4
			if ip[0]>>4 != 4 || header < 20 || length < header || length > len(ip) {
				t.Fatalf("invalid captured IPv4 datagram: %x", ip)
			}
			if gatewayChecksum(ip[:header]) != 0 {
				t.Fatalf("invalid captured IPv4 checksum: %x", ip[:header])
			}
			captured = append(captured, gatewayCapture{fd: int(ready.Fd), ip: bytes.Clone(ip[:length])})
		}
	}
	return captured
}

func gatewayForwarded(t *testing.T, captured []gatewayCapture, fd int, sent []byte, want bool) {
	t.Helper()
	id := binary.BigEndian.Uint16(sent[4:6])
	header := int(sent[0]&15) * 4
	payload := make([]byte, len(sent)-header)
	seen := make([]bool, len(payload))
	found := false
	for _, frame := range captured {
		ip := frame.ip
		if frame.fd != fd || ip[9] != sent[9] || binary.BigEndian.Uint16(ip[4:6]) != id {
			continue
		}
		found = true
		if !want {
			t.Fatalf("discarded datagram %d appeared on forwarding path: %x", id, ip)
		}
		if ip[8] != sent[8]-1 || !bytes.Equal(ip[12:20], sent[12:20]) {
			t.Fatalf("forwarded datagram changed addresses or has wrong TTL: %x", ip)
		}
		fragment := binary.BigEndian.Uint16(ip[6:8])
		offset := int(fragment&0x1fff) * 8
		data := ip[int(ip[0]&15)*4:]
		if offset+len(data) > len(payload) {
			t.Fatalf("fragment extends beyond original datagram: %x", ip)
		}
		copy(payload[offset:], data)
		for i := offset; i < offset+len(data); i++ {
			if seen[i] {
				t.Fatalf("overlapping fragment at %d", i)
			}
			seen[i] = true
		}
	}
	if !want {
		return
	}
	if !found {
		t.Fatalf("valid control %d never forwarded", id)
	}
	for i, present := range seen {
		if !present {
			t.Fatalf("forwarded datagram %d missing payload byte %d", id, i)
		}
	}
	if !bytes.Equal(payload, sent[header:]) {
		t.Fatalf("forwarded payload differs: %x != %x", payload, sent[header:])
	}
}

func gatewayICMP(t *testing.T, captured []gatewayCapture, fd int, sent []byte, typ, code byte) []byte {
	t.Helper()
	for _, frame := range captured {
		ip := frame.ip
		if frame.fd != fd || ip[9] != unix.IPPROTO_ICMP {
			continue
		}
		icmp := ip[int(ip[0]&15)*4:]
		if len(icmp) < 36 || !bytes.Equal(icmp[12:14], sent[4:6]) {
			continue
		}
		if icmp[0] != typ || icmp[1] != code || gatewayChecksum(icmp) != 0 {
			t.Fatalf("wrong ICMP error for %d: %x", binary.BigEndian.Uint16(sent[4:6]), icmp)
		}
		if !bytes.Equal(ip[16:20], sent[12:16]) || !bytes.Equal(icmp[20:28], sent[12:20]) {
			t.Fatalf("ICMP reply/quoted addresses differ: %x", ip)
		}
		return ip
	}
	t.Fatalf("no ICMP type %d code %d for packet %d", typ, code, binary.BigEndian.Uint16(sent[4:6]))
	return nil
}

func TestGatewayICMPDFDiscard(t *testing.T) {
	// RFC requirement: RFC792-Unreachable-1 positive -- a DF datagram above the egress MTU never appears on that link, and Linux returns Fragmentation Needed.
	// RFC requirement: RFC792-Unreachable-1 negative -- a DF datagram exactly at the MTU forwards; the oversized datagram with DF clear forwards in fragments with its entire payload intact.
	// RFC requirement: RFC792-Format-1 positive -- Fragmentation Needed leaves the reserved octet zero while carrying the RFC1191 next-hop MTU in its assigned field.
	w := newGatewayWire(t)
	out := w.routedExit(t)
	bad := gatewayDatagram(1, 900, 64, 0x4000, nil)
	fit := gatewayDatagram(2, 576, 64, 0x4000, nil)
	fragment := gatewayDatagram(3, 900, 64, 0, nil)
	for _, ip := range [][]byte{bad, fit, fragment} {
		w.send(t, w.parent.Attrs().HardwareAddr, ip)
	}
	captured := gatewayCapturePackets(t, w.fd, out)
	gatewayForwarded(t, captured, out, bad, false)
	gatewayForwarded(t, captured, out, fit, true)
	gatewayForwarded(t, captured, out, fragment, true)
	reply := gatewayICMP(t, captured, w.fd, bad, 3, 4)
	icmp := reply[int(reply[0]&15)*4:]
	// RFC4884 assigns byte 5 to quote length; RFC1191 assigns bytes 6:8 to MTU.
	if icmp[4] != 0 || binary.BigEndian.Uint16(icmp[6:8]) != 576 {
		t.Fatalf("unused/next-hop MTU fields: %x", icmp[:8])
	}
}

func TestGatewayICMPTTLDiscard(t *testing.T) {
	// RFC requirement: RFC792-TimeExceeded-1 positive -- TTL zero and TTL one are discarded and answered with Time Exceeded.
	// RFC requirement: RFC792-TimeExceeded-1 negative -- TTL two traverses the same route with TTL one and unchanged payload.
	// RFC requirement: RFC792-Format-1 positive -- emitted Time Exceeded reserved fields are zero; the RFC4884 length octet is not mistaken for unused space.
	w := newGatewayWire(t)
	out := w.routedExit(t)
	zero := gatewayDatagram(10, 64, 0, 0, nil)
	one := gatewayDatagram(11, 64, 1, 0, nil)
	two := gatewayDatagram(12, 64, 2, 0, nil)
	for _, ip := range [][]byte{zero, one, two} {
		w.send(t, w.parent.Attrs().HardwareAddr, ip)
	}
	captured := gatewayCapturePackets(t, w.fd, out)
	gatewayForwarded(t, captured, out, two, true)
	for _, ip := range [][]byte{zero, one} {
		gatewayForwarded(t, captured, out, ip, false)
		reply := gatewayICMP(t, captured, w.fd, ip, 11, 0)
		icmp := reply[int(reply[0]&15)*4:]
		if icmp[4] != 0 || icmp[6] != 0 || icmp[7] != 0 {
			t.Fatalf("Time Exceeded unused fields: %x", icmp[:8])
		}
	}
}

func TestGatewayICMPHeaderDiscard(t *testing.T) {
	// RFC requirement: RFC792-ParamProblem-1 positive -- an invalid Record Route pointer and a corrupt IPv4 checksum are absent on egress; the invalid option elicits Parameter Problem.
	// RFC requirement: RFC792-ParamProblem-1 negative -- the otherwise identical Record Route option with a valid pointer forwards, as does the uncorrupted fixed header.
	// RFC requirement: RFC792-Format-1 positive -- Parameter Problem carries its nonzero pointer while the remaining unused bytes are zero (RFC4884 allocates byte 5 to length).
	w := newGatewayWire(t)
	out := w.routedExit(t)
	badOption := gatewayDatagram(20, 68, 64, 0, []byte{7, 3, 3, 0})
	goodOption := gatewayDatagram(21, 68, 64, 0, []byte{7, 3, 4, 0})
	badChecksum := gatewayDatagram(22, 64, 64, 0, nil)
	badChecksum[10] ^= 1
	goodHeader := gatewayDatagram(23, 64, 64, 0, nil)
	for _, ip := range [][]byte{badOption, goodOption, badChecksum, goodHeader} {
		w.send(t, w.parent.Attrs().HardwareAddr, ip)
	}
	captured := gatewayCapturePackets(t, w.fd, out)
	gatewayForwarded(t, captured, out, badOption, false)
	gatewayForwarded(t, captured, out, badChecksum, false)
	gatewayForwarded(t, captured, out, goodOption, true)
	gatewayForwarded(t, captured, out, goodHeader, true)
	reply := gatewayICMP(t, captured, w.fd, badOption, 12, 0)
	icmp := reply[int(reply[0]&15)*4:]
	if icmp[4] != 22 || icmp[6] != 0 || icmp[7] != 0 {
		t.Fatalf("Parameter Problem pointer/unused fields: %x", icmp[:8])
	}
}

func TestGatewayICMPEchoReply(t *testing.T) {
	// RFC requirement: RFC792-Echo-5 positive -- Linux returns the complete odd-length request data unchanged.
	// RFC requirement: RFC792-Echo-5 negative -- a corrupt-checksum request is never reflected, alongside the valid request control.
	// RFC requirement: RFC792-Echo-6 positive -- the observed reply reverses addresses, changes Type 8 to 0, and carries a recomputed valid checksum.
	// RFC requirement: RFC792-Echo-6 negative -- a corrupt-checksum request produces no reply.
	w := newGatewayWire(t)
	gatewaySysctl(t, "ipv4/icmp_echo_ignore_all", "0")
	gatewayNeighbor(t, w.parent, netip.MustParseAddr("192.0.2.20"), w.peer.Attrs().HardwareAddr)
	request := gatewayDatagram(50, 61, 64, 0, nil)
	request[9] = unix.IPPROTO_ICMP
	copy(request[16:20], netip.MustParseAddr("192.0.2.254").AsSlice())
	request[10], request[11] = 0, 0
	binary.BigEndian.PutUint16(request[10:], gatewayChecksum(request[:20]))
	icmp := request[20:]
	icmp[0], icmp[1], icmp[2], icmp[3] = 8, 0, 0, 0
	binary.BigEndian.PutUint16(icmp[4:], 0x1234)
	binary.BigEndian.PutUint16(icmp[6:], 1)
	binary.BigEndian.PutUint16(icmp[2:], gatewayChecksum(icmp))
	corrupt := bytes.Clone(request)
	binary.BigEndian.PutUint16(corrupt[26:], 2)
	corrupt[22], corrupt[23] = 0, 0
	binary.BigEndian.PutUint16(corrupt[22:], gatewayChecksum(corrupt[20:])^1)
	w.send(t, w.parent.Attrs().HardwareAddr, corrupt)
	w.send(t, w.parent.Attrs().HardwareAddr, request)
	captured := gatewayCapturePackets(t, w.fd)
	replied := false
	for _, frame := range captured {
		ip := frame.ip
		if ip[9] != unix.IPPROTO_ICMP {
			continue
		}
		answer := ip[int(ip[0]&15)*4:]
		if len(answer) < 8 || binary.BigEndian.Uint16(answer[4:6]) != 0x1234 {
			continue
		}
		if binary.BigEndian.Uint16(answer[6:8]) == 2 {
			t.Fatal("corrupt Echo Request was reflected")
		}
		if answer[0] != 0 || answer[1] != 0 || gatewayChecksum(answer) != 0 {
			t.Fatalf("invalid Echo Reply header: %x", answer)
		}
		if !bytes.Equal(answer[4:], icmp[4:]) {
			t.Fatalf("Echo Reply changed identifier, sequence or data: %x != %x", answer[4:], icmp[4:])
		}
		if !bytes.Equal(ip[12:16], request[16:20]) || !bytes.Equal(ip[16:20], request[12:16]) {
			t.Fatalf("Echo Reply did not reverse addresses: %x", ip[:20])
		}
		replied = true
	}
	if !replied {
		t.Fatal("valid Echo Request received no reply")
	}
}
