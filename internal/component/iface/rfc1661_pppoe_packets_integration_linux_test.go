//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- real IPv4 discard proof.
// Related: pppoe_kernel_integration_linux_test.go -- exact installed-state observer.

package iface_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestDialRenegotiatesLCPInNetworkPhase measures both IPv4 directions through
// real PPP and runSession. A held Configure-Ack, PAP Ack and IPCP Ack keep each
// negative window open. Successful traffic before and after rejects a dead path.
// Operator config reaches runSession through registered plugin config delivery.
// MUTATION: replace restart's NPModeDrop with NPModePass to expose IPv4 leakage.
// RFC requirement: RFC1661-3.6-2 positive -- over a real PPPoE session through the kernel PPP channel, IPv4 packets sent in both directions while LCP, PAP and IPCP renegotiate (each Ack held back) are not delivered, while IPv4 in both directions is delivered before the renegotiation starts and after IPCP reopens.
func TestDialRenegotiatesLCPInNetworkPhase(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	peer := pppoeac.OpenVeth(t, "zec0", "zea0", 44)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	startPPPoEConfig(t, pppoeOperatorConfig)
	// RFC 2516 Section 5.
	session, err := peer.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// RFC 1661 Section 5.1, RFC 1994 Section 4.1.
	options := []byte{1, 4, 5, 170, 3, 5, 0xc2, 0x23, 5, 5, 6, 0x10, 0x20, 0x30, 0x40}
	if err := session.LCP(ctx, 70, options, false); err != nil {
		t.Fatal(err)
	}
	if err := session.CHAP(ctx, 80, []byte{1, 3, 5, 7}, "subscriber", "secret"); err != nil {
		t.Fatal(err)
	}
	local, remote := netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("192.0.2.1")
	// RFC 1332 Section 3.3.
	if err := session.IPCP(ctx, 90, remote, local); err != nil {
		t.Fatal(err)
	}
	before := waitPPPoEKernelState(t, ctx, local, remote, 1450)
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	})
	packetProbe(t, ctx, session, conn, local, remote, "before", true)

	// RFC 1661 Sections 4.1 and 5.1. Switch to PAP so its client request is an
	// observable authentication barrier, before we send the authentication Ack.
	options = []byte{1, 4, 5, 120, 3, 4, 0xc0, 0x23, 5, 6, 0x50, 0x60, 0x70, 0x80}
	if err := session.SendControl(ppp.ProtoLCP, ppp.LCPConfigureRequest, 71, options); err != nil {
		t.Fatal(err)
	}
	request := packetControl(t, ctx, session, ppp.ProtoLCP, ppp.LCPConfigureRequest)
	ack := packetControl(t, ctx, session, ppp.ProtoLCP, ppp.LCPConfigureAck)
	if ack.Identifier != 71 {
		t.Fatalf("restart Ack identifier = %d, want 71", ack.Identifier)
	}
	if !bytes.Equal(ack.Data, options) {
		t.Fatalf("restart Ack changed options: %x", ack.Data)
	}
	packetProbe(t, ctx, session, conn, local, remote, "lcp", false)
	// RFC 1661 Section 5.2.
	if err := session.SendControl(ppp.ProtoLCP, ppp.LCPConfigureAck, request.Identifier, request.Data); err != nil {
		t.Fatal(err)
	}
	// RFC 1334 Section 2.2.1. Check both configured credentials on the wire.
	auth := packetControl(t, ctx, session, ppp.ProtoPAP, 1)
	if !bytes.Equal(auth.Data, []byte("\x0asubscriber\x06secret")) {
		t.Fatalf("PAP request differs from configured credentials: %x", auth.Data)
	}
	packetProbe(t, ctx, session, conn, local, remote, "auth", false)
	// RFC 1334 Section 2.2.2. Empty Message is encoded with a zero length.
	if err := session.SendControl(ppp.ProtoPAP, 2, auth.Identifier, []byte{0}); err != nil {
		t.Fatal(err)
	}
	ipcp := packetControl(t, ctx, session, ppp.ProtoIPCP, ppp.LCPConfigureRequest)
	packetProbe(t, ctx, session, conn, local, remote, "ipcp", false)
	local, remote = netip.MustParseAddr("198.51.100.2"), netip.MustParseAddr("198.51.100.1")
	// RFC 1332 Section 3.3. Nak the held request, then let the shared IPCP
	// exchange verify the fresh request and the peer's exact Configure-Ack.
	if err := session.SendControl(ppp.ProtoIPCP, ppp.LCPConfigureNak, ipcp.Identifier, []byte{3, 6, 198, 51, 100, 2}); err != nil {
		t.Fatal(err)
	}
	if err := session.IPCP(ctx, 91, remote, local); err != nil {
		t.Fatal(err)
	}
	after := waitPPPoEKernelState(t, ctx, local, remote, 1400)
	if after.Attrs().Index != before.Attrs().Index {
		t.Fatal("renegotiation replaced the PPP ifindex")
	}
	if after.Attrs().Name != before.Attrs().Name {
		t.Fatal("renegotiation replaced the PPP name")
	}
	packetProbe(t, ctx, session, conn, local, remote, "after", true)
	t.Log("SID 44 and PPP ifindex retained; both IPv4 directions pass before/after and drop during LCP, PAP and IPCP")
}

// packetControl retains the owned payload while a later probe reads the socket.
func packetControl(t *testing.T, ctx context.Context, session *pppoeac.Session, wantProto uint16, wantCode uint8) ppp.LCPPacket {
	t.Helper()
	proto, packet, err := session.ReadExpectedControl(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if proto != wantProto {
		t.Fatalf("control protocol = %x, want %x", proto, wantProto)
	}
	if packet.Code != wantCode {
		t.Fatalf("control code = %d, want %d", packet.Code, wantCode)
	}
	return packet
}

// packetProbe sends distinct UDP payloads in both directions. Negative windows
// are bounded observations while the peer withholds the phase-completing reply,
// not sleeps to guess when a transition occurred. Later positive probes also
// refuse delayed delivery of any earlier token, distinguishing DROP from QUEUE.
func packetProbe(t *testing.T, ctx context.Context, session *pppoeac.Session, conn *net.UDPConn, local, remote netip.Addr, stage string, pass bool) {
	t.Helper()
	outbound := []byte("out-" + stage)
	if _, err := conn.WriteToUDPAddrPort(outbound, netip.AddrPortFrom(remote, 32123)); err != nil {
		t.Fatal(err)
	}
	window := 250 * time.Millisecond
	if pass {
		window = 2 * time.Second
	}
	readCtx, cancel := context.WithTimeout(ctx, window)
	defer cancel()
	proto, payload, err := session.ReadExpectedPacket(readCtx)
	if pass {
		if err != nil {
			t.Fatalf("%s outbound positive control: %v", stage, err)
		}
		if proto != ppp.ProtoIPv4 {
			t.Fatalf("%s outbound protocol = %x", stage, proto)
		}
		packetUDPMatch(t, payload, local, remote, outbound)
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("%s outbound DROP failed: protocol=%x payload=%x error=%v", stage, proto, payload, err)
	}
	inbound := []byte("in-" + stage)
	address, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("UDP socket returned local address type %T", conn.LocalAddr())
	}
	port := address.AddrPort().Port()
	var datagram [128]byte
	n := packetUDP(datagram[:], remote, local, port, inbound)
	if err := session.SendPacket(ppp.ProtoIPv4, datagram[:n]); err != nil {
		t.Fatal(err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(window)); err != nil {
		t.Fatal(err)
	}
	n, source, err := conn.ReadFromUDPAddrPort(datagram[:])
	if pass {
		if err != nil {
			t.Fatalf("%s inbound positive control: %v", stage, err)
		}
		if source != netip.AddrPortFrom(remote, 32123) {
			t.Fatalf("%s inbound source = %v", stage, source)
		}
		if !bytes.Equal(datagram[:n], inbound) {
			t.Fatalf("%s inbound payload = %x, want %x", stage, datagram[:n], inbound)
		}
	} else if !os.IsTimeout(err) {
		t.Fatalf("%s inbound DROP failed: payload=%x source=%v error=%v", stage, datagram[:n], source, err)
	}
	t.Logf("%s: IPv4 outbound and inbound pass=%t", stage, pass)
}

// packetUDP writes an IPv4/UDP probe. UDP checksum zero is permitted for IPv4.
// Wire: IPv4 0..19, UDP 20..27, probe token 28..end (RFC 791, RFC 768).
func packetUDP(buf []byte, source, target netip.Addr, port uint16, data []byte) int {
	clear(buf)
	buf[0], buf[8], buf[9] = 0x45, 64, 17
	binary.BigEndian.PutUint16(buf[2:4], uint16(28+len(data)))
	src, dst := source.As4(), target.As4()
	copy(buf[12:16], src[:])
	copy(buf[16:20], dst[:])
	var sum uint32
	for off := 0; off < 20; off += 2 {
		sum += uint32(binary.BigEndian.Uint16(buf[off : off+2]))
	}
	sum = (sum & 0xffff) + (sum >> 16)
	sum = (sum & 0xffff) + (sum >> 16)
	binary.BigEndian.PutUint16(buf[10:12], ^uint16(sum))
	binary.BigEndian.PutUint16(buf[20:22], 32123)
	binary.BigEndian.PutUint16(buf[22:24], port)
	binary.BigEndian.PutUint16(buf[24:26], uint16(8+len(data)))
	return 28 + copy(buf[28:], data)
}

// packetUDPMatch checks the actual PPP payload rather than an interface counter.
func packetUDPMatch(t *testing.T, packet []byte, source, target netip.Addr, data []byte) {
	t.Helper()
	if len(packet) < 28 {
		t.Fatalf("short IPv4 UDP packet: %x", packet)
	}
	if packet[0] != 0x45 {
		t.Fatalf("unexpected IPv4 header: %x", packet)
	}
	if packet[9] != 17 {
		t.Fatalf("unexpected IP protocol: %x", packet)
	}
	if int(binary.BigEndian.Uint16(packet[2:4])) != len(packet) {
		t.Fatalf("IPv4 total length differs from received payload: %x", packet)
	}
	if binary.BigEndian.Uint16(packet[6:8])&0x3fff != 0 {
		t.Fatalf("outbound UDP probe is fragmented: %x", packet)
	}
	if int(binary.BigEndian.Uint16(packet[24:26])) != len(packet)-20 {
		t.Fatalf("UDP length differs from received payload: %x", packet)
	}
	src, dst := source.As4(), target.As4()
	if !bytes.Equal(packet[12:16], src[:]) {
		t.Fatalf("outbound source differs: %x", packet)
	}
	if !bytes.Equal(packet[16:20], dst[:]) {
		t.Fatalf("outbound destination differs: %x", packet)
	}
	if binary.BigEndian.Uint16(packet[22:24]) != 32123 {
		t.Fatalf("outbound UDP destination differs: %x", packet)
	}
	if !bytes.Equal(packet[28:], data) {
		t.Fatalf("outbound payload = %x, want %x", packet[28:], data)
	}
}
