//go:build integration && linux

// Design: docs/architecture/testing/interop.md -- independent SRv6 wire oracle.
// Related: vpp_srv6_probe_integration_linux_test.go -- live VPP assertions.
// RFC 9252 Sections 3.1 and 5.3 -- see rfc/short/rfc9252.md.
package testdeployment

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// vppSRv6Open uses a deliberately independent, bounded wire speaker.
// RFC 4271 Section 4.2: "After a TCP connection is established, the first
// message sent by each side is an OPEN message.".
func vppSRv6Open(t *testing.T, peer net.Conn) {
	t.Helper()
	// OPEN: version[0], AS[1:3], hold[3:5], router ID[5:9], optlen[9].
	// Capabilities: MP IPv4/unicast, extended next-hop IPv4/unicast -> IPv6,
	// four-octet ASN. ASN 1 matches the existing vppFIBConfig iBGP peer.
	open := []byte{4, 0, 1, 0, 180, 192, 0, 2, 2, 22,
		2, 20, 1, 4, 0, 1, 0, 1, 5, 6, 0, 1, 0, 1, 0, 2, 65, 4, 0, 0, 0, 1}
	vppSRv6Send(t, peer, 1, open)
	if err := peer.SetReadDeadline(time.Now().Add(20 * time.Second)); err != nil {
		t.Fatal(err)
	}
	kind, body := vppSRv6Read(t, peer)
	if kind != 1 {
		t.Fatalf("expected OPEN, got type %d: %x", kind, body)
	}
	if len(body) < 10 {
		t.Fatalf("short OPEN: %x", body)
	}
	if !bytes.Contains(body[10:], []byte{5, 6, 0, 1, 0, 1, 0, 2}) {
		t.Fatalf("Ze did not negotiate IPv4/unicast with IPv6 next hop: %x", body)
	}
	vppSRv6Send(t, peer, 4, nil)
	kind, body = vppSRv6Read(t, peer)
	if kind != 4 {
		t.Fatalf("expected KEEPALIVE, got type %d: %x", kind, body)
	}
}

// vppSRv6Update emits one global IPv4 route without using Ze's route encoder.
// RFC 9252 Section 5.3: "SRv6 Service SID is encoded as part of the SRv6 L3
// Service TLV." "The SRv6 Endpoint Behavior SHOULD be one of these: End.DX4,
// End.DT4, or End.DT46.".
func vppSRv6Update(t *testing.T, peer net.Conn, prefix netip.Prefix, sid netip.Addr) {
	t.Helper()
	var body [128]byte
	// UPDATE [0:2] withdrawn length, [2:4] attributes length.
	off := 4
	// ORIGIN IGP, empty iBGP AS_PATH, LOCAL_PREF 100.
	off += copy(body[off:], []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 5, 4, 0, 0, 0, 100})
	// MP_REACH: AFI 1, SAFI 1, NH length 16, IPv6 NH, reserved, /24 NLRI.
	off += copy(body[off:], []byte{0x80, 14, 25, 0, 1, 1, 16})
	nextHop := netip.MustParseAddr(vppSRv6NextHop).As16()
	off += copy(body[off:], nextHop[:])
	body[off] = 0
	off++
	off += vppSRv6NLRI(t, body[off:], prefix)
	// Prefix-SID: L3 TLV [type=5,len=25,reserved=0], SID-info sub-TLV
	// [type=1,len=21,reserved=0,SID(16),flags=0,behavior=0x0013,reserved=0].
	off += copy(body[off:], []byte{0xc0, 40, 28, 5, 0, 25, 0, 1, 0, 21, 0})
	segment := sid.As16()
	off += copy(body[off:], segment[:])
	off += copy(body[off:], []byte{0, 0, 0x13, 0})
	binary.BigEndian.PutUint16(body[2:4], uint16(off-4))
	vppSRv6Send(t, peer, 2, body[:off])
	t.Logf("announced %s with received SID %s", prefix, sid)
}

// RFC 4760 Section 4: "An UPDATE message that contains the MP_UNREACH_NLRI is
// not required to carry any other path attributes.".
func vppSRv6Withdraw(t *testing.T, peer net.Conn, prefix netip.Prefix) {
	t.Helper()
	var body [14]byte
	copy(body[:], []byte{0, 0, 0, 10, 0x80, 15, 7, 0, 1, 1})
	vppSRv6NLRI(t, body[10:], prefix)
	vppSRv6Send(t, peer, 2, body[:])
	t.Logf("withdrew %s", prefix)
}

// RFC 4271 Section 4.3: "The Length field indicates the length in bits of the IP
// address prefix.".
func vppSRv6NLRI(t *testing.T, dst []byte, prefix netip.Prefix) int {
	t.Helper()
	if prefix.Bits() != 24 {
		t.Fatal("this bounded fixture carries only /24 routes")
	}
	addr := prefix.Addr().As4()
	dst[0] = 24
	copy(dst[1:4], addr[:3])
	return 4
}

// RFC 4271 Section 4.1: "The value of the Length field MUST always be at least
// 19 and no greater than 4096, and MAY be further constrained, depending on the
// message type.".
func vppSRv6Send(t *testing.T, peer net.Conn, kind byte, body []byte) {
	t.Helper()
	var frame [4096]byte
	if len(body) > len(frame)-19 {
		t.Fatal("fixture BGP message exceeds 4096 bytes")
	}
	for i := range 16 {
		frame[i] = 0xff
	}
	binary.BigEndian.PutUint16(frame[16:18], uint16(19+len(body)))
	frame[18] = kind
	copy(frame[19:], body)
	if err := peer.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(peer, bytes.NewReader(frame[:19+len(body)])); err != nil {
		t.Fatal(err)
	}
}

// RFC 4271 Section 4.1: "The value of the Length field MUST always be at least
// 19 and no greater than 4096, and MAY be further constrained, depending on the
// message type.".
func vppSRv6Read(t *testing.T, peer net.Conn) (byte, []byte) {
	t.Helper()
	var header [19]byte
	if _, err := io.ReadFull(peer, header[:]); err != nil {
		t.Fatal(err)
	}
	length := int(binary.BigEndian.Uint16(header[16:18]))
	if length < len(header) {
		t.Fatalf("invalid BGP length %d", length)
	}
	if length > 4096 {
		t.Fatalf("invalid BGP length %d", length)
	}
	body := make([]byte, length-len(header))
	if _, err := io.ReadFull(peer, body); err != nil {
		t.Fatal(err)
	}
	return header[18], body
}

func vppSRv6PacketSocket(t *testing.T) (int, int) {
	t.Helper()
	iface, err := net.InterfaceByName(vppSRv6Host)
	if err != nil {
		t.Fatal(err)
	}
	// ETH_P_ALL in network byte order, on the little-endian lab targets.
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, 0x0300)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	})
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Protocol: 0x0300, Ifindex: iface.Index}); err != nil {
		t.Fatal(err)
	}
	return fd, iface.Index
}

// vppSRv6Packet injects a unique datagram and rejects wrong or stale forwarding.
// An invalid want denotes withdrawal: no IPv6 or plain IPv4 forwarding of that
// datagram may be observed during the bounded capture. Adjacent positive probes
// ensure that a dead or disconnected capture cannot satisfy the negative case.
// RFC 9252 Section 1: "The ingress PE encapsulates the payload in an outer IPv6
// header where the destination address is the SRv6 Service SID provided by the
// egress PE.".
func vppSRv6Packet(t *testing.T, fd, index int, destination, want netip.Addr, sequence byte) {
	t.Helper()
	var frame [74]byte
	vppSRv6Frame(frame[:], destination, sequence)
	ip := frame[14:]
	if err := unix.Sendto(fd, frame[:], 0, &unix.SockaddrLinklayer{Ifindex: index, Halen: 6,
		Addr: [8]byte{2, 0, 0, 0x94, 0, 1}}); err != nil {
		t.Fatal(err)
	}
	t.Logf("injected real Ethernet datagram: ifindex=%d sequence=%d frame=%x", index, sequence, frame)
	deadline := time.Now().Add(time.Second)
	if want.IsValid() {
		deadline = time.Now().Add(3 * time.Second)
	}
	var received [2048]byte
	seenFrames := 0
	for time.Now().Before(deadline) {
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		if _, err := unix.Poll(poll, 100); err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			t.Fatal(err)
		}
		n, from, err := unix.Recvfrom(fd, received[:], 0)
		if errors.Is(err, unix.EAGAIN) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		link, ok := from.(*unix.SockaddrLinklayer)
		if !ok {
			t.Fatalf("unexpected packet socket address %T", from)
		}
		if seenFrames < 8 {
			t.Logf("packet observation sequence=%d ifindex=%d type=%d length=%d first-bytes=%x",
				sequence, link.Ifindex, link.Pkttype, n, received[:min(n, 160)])
			seenFrames++
		}
		if link.Pkttype == unix.PACKET_OUTGOING {
			continue
		}
		packet := received[:n]
		if !bytes.Contains(packet, ip[28:]) {
			continue
		}
		// ICMP errors quote the offending datagram; they are not forwarding
		// of the service payload and cannot satisfy either packet assertion.
		if n >= 34 && binary.BigEndian.Uint16(packet[12:14]) == 0x0800 && packet[23] == 1 {
			continue
		}
		if n >= 54 && binary.BigEndian.Uint16(packet[12:14]) == 0x86dd && packet[20] == 58 {
			continue
		}
		if !want.IsValid() {
			t.Fatalf("withdrawn %s still forwarded sequence %d: %x", destination, sequence, packet)
		}
		// RFC 9252 Section 5: single-SID H.Encaps.Red reaches the received SID.
		if err := vppSRv6PacketMatches(packet, ip, want); err != nil {
			vppSRv6Diagnostics(t)
			t.Fatalf("packet sequence %d: %v: %x", sequence, err, packet)
		}
		t.Logf("captured sequence=%d inner=%s outer=%s frame=%x", sequence, destination, want, packet)
		return
	}
	if want.IsValid() {
		vppSRv6Diagnostics(t)
		t.Fatalf("no encapsulated packet sequence=%d inner=%s outer=%s", sequence, destination, want)
	}
	t.Logf("no forwarded packet after withdrawal: sequence=%d inner=%s", sequence, destination)
}

// vppSRv6PacketMatches compares the independent received bytes, allowing only
// the IPv4 hop-limit/checksum changes made by forwarding.
// RFC 9252 Section 1: "The ingress PE encapsulates the payload in an outer IPv6
// header where the destination address is the SRv6 Service SID provided by the
// egress PE.".
func vppSRv6PacketMatches(frame, inner []byte, sid netip.Addr) error {
	if len(frame) < 54+len(inner) {
		return errors.New("short encapsulated Ethernet frame")
	}
	if binary.BigEndian.Uint16(frame[12:14]) != 0x86dd {
		return errors.New("service packet was not encapsulated in IPv6")
	}
	outer := frame[14:54]
	if outer[0]>>4 != 6 {
		return errors.New("outer header is not IPv6")
	}
	if outer[6] != 4 {
		return errors.New("single-SID reduced encapsulation must carry IPv4 directly, without an SRH")
	}
	if int(binary.BigEndian.Uint16(outer[4:6])) != len(inner) {
		return errors.New("outer payload length does not match the injected packet")
	}
	source := netip.MustParseAddr(vppSRv6Source).As16()
	if !bytes.Equal(outer[8:24], source[:]) {
		return errors.New("outer source is not the configured encapsulation source")
	}
	segment := sid.As16()
	if !bytes.Equal(outer[24:40], segment[:]) {
		return errors.New("outer destination is not the received SID")
	}
	got := frame[54 : 54+len(inner)]
	if !bytes.Equal(got[:8], inner[:8]) {
		return errors.New("inner IPv4 header was corrupted")
	}
	if got[8] != inner[8]-1 {
		return errors.New("inner IPv4 TTL was not decremented once")
	}
	if got[9] != inner[9] {
		return errors.New("inner IPv4 protocol changed")
	}
	if !bytes.Equal(got[12:], inner[12:]) {
		return errors.New("inner addresses or payload differ from the injected datagram")
	}
	if vppSRv6Checksum(got[:20]) != 0 {
		return errors.New("forwarded inner IPv4 checksum is invalid")
	}
	return nil
}

// RFC 791 Section 3.1: "The checksum field is the 16 bit one's complement of
// the one's complement sum of all 16 bit words in the header.".
func vppSRv6Checksum(header []byte) uint16 {
	var sum uint32
	for i := 0; i < len(header); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(header[i : i+2]))
	}
	for sum > 0xffff {
		sum = sum&0xffff + sum>>16
	}
	return ^uint16(sum)
}

// vppSRv6Frame writes the common datagram used by raw-socket and VPP PG ingress.
// RFC 791 Section 3.1: "The checksum field is the 16 bit one's complement of
// the one's complement sum of all 16 bit words in the header.".
func vppSRv6Frame(frame []byte, destination netip.Addr, sequence byte) {
	copy(frame[:12], []byte{2, 0, 0, 0x94, 0, 1, 2, 0, 0, 0x94, 0, 2})
	binary.BigEndian.PutUint16(frame[12:14], 0x0800)
	ip := frame[14:]
	ip[0] = 0x45
	binary.BigEndian.PutUint16(ip[2:4], uint16(len(ip)))
	ip[8], ip[9] = 64, 17
	copy(ip[12:16], []byte{192, 0, 2, 2})
	dst := destination.As4()
	copy(ip[16:20], dst[:])
	binary.BigEndian.PutUint16(ip[20:22], 19400)
	binary.BigEndian.PutUint16(ip[22:24], 19401)
	binary.BigEndian.PutUint16(ip[24:26], uint16(len(ip)-20))
	copy(ip[28:], "ze-srv6-forwarding-packet-oracle")
	ip[len(ip)-1] = sequence
	binary.BigEndian.PutUint16(ip[10:12], vppSRv6Checksum(ip[:20]))
}
