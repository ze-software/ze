package fixture

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // RADIUS wire protocol requires MD5
	"encoding/binary"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestTunnelL2TPSCCRQWireShape(t *testing.T) {
	challenge := []byte{1, 2, 3, 4}
	packet := tunnelL2TPSCCRQ(0x1234, "wire-peer", challenge)
	if got := binary.BigEndian.Uint16(packet[:2]); got != 0xc802 {
		t.Fatalf("flags = %#x, want 0xc802", got)
	}
	if got := int(binary.BigEndian.Uint16(packet[2:4])); got != len(packet) {
		t.Fatalf("length = %d, datagram = %d", got, len(packet))
	}
	if got := binary.BigEndian.Uint16(packet[16:18]); got != 0 {
		t.Fatalf("first AVP type = %d, want Message Type", got)
	}
	avps, err := tunnelL2TPParseAVPs(packet)
	if err != nil {
		t.Fatal(err)
	}
	if tunnelL2TPMessageType(avps) != 1 || binary.BigEndian.Uint16(avps[9]) != 0x1234 || !bytes.Equal(avps[11], challenge) {
		t.Fatalf("decoded SCCRQ fields = %#v", avps)
	}
}

// VALIDATES: the SCCRQ exchange's socket deadlines end with that exchange.
// PREVENTS: a completed CHAP/IPCP session losing every later echo reply to the
// expired handshake write deadline, before accounting can reach its Interim.
func TestTunnelL2TPAccountingEchoAfterHandshakeDeadline(t *testing.T) {
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close() //nolint:errcheck // fixture teardown
	target, ok := server.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("udp4 listener address is %T", server.LocalAddr())
	}
	conn, _, err := tunnelL2TPDial(target.Port)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() //nolint:errcheck // fixture teardown

	handshake := make(chan error, 1)
	// The responder MUST publish its result; the caller joins before closing UDP.
	go func() {
		if err := server.SetDeadline(time.Now().Add(time.Second)); err != nil {
			handshake <- err
			return
		}
		var buffer [64]byte
		n, address, err := server.ReadFromUDP(buffer[:])
		if err == nil {
			_, err = server.WriteToUDP(buffer[:n], address)
		}
		handshake <- err
	}()
	const exchangeTimeout = 250 * time.Millisecond
	request := tunnelL2TPSCCRQ(0x0321, "echo-peer", nil)
	_, _, exchangeErr := tunnelL2TPExchange(context.Background(), conn, target, request, 1, exchangeTimeout)
	// The caller MUST join the responder before a failure can start teardown.
	if err := <-handshake; err != nil {
		t.Fatal(err)
	}
	if exchangeErr != nil {
		t.Fatal(exchangeErr)
	}
	// Wait for the specific deadline installed by the exchange to expire.
	// This is not readiness slack: the regression needs a post-deadline write.
	<-time.After(exchangeTimeout)

	peer := tunnelAccountingPeer{
		conn: conn, target: target, localTID: 0x0123, zeSID: 0x0234,
	}
	// Linux PPPoL2TP transmits Address/Control followed by the PPP protocol.
	echo := []byte{0, 2, 3, 0x21, 2, 0xbc, 0xff, 3, 0xc0, 0x21, 9, 7, 0, 8, 0x55, 0x66, 0x77, 0x88}
	if err := peer.handleAccountingPacket(echo); err != nil {
		t.Fatal(err)
	}
	if err := server.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var buffer [64]byte
	n, _, err := server.ReadFromUDP(buffer[:])
	if err != nil {
		t.Fatalf("active peer failed to deliver its post-handshake Echo-Reply: %v", err)
	}
	want := []byte{0, 2, 1, 0x23, 2, 0x34, 0xc0, 0x21, 10, 7, 0, 8, 0x11, 0x22, 0x33, 0x44}
	if !bytes.Equal(buffer[:n], want) {
		t.Fatalf("Echo-Reply = %x, want %x", buffer[:n], want)
	}

	// The explicit post-Interim silence remains the only phase that drops
	// probes; clearing a handshake deadline must not remove the Stop trigger.
	peer.silent = true
	if err := peer.handleAccountingPacket(echo); err != nil {
		t.Fatal(err)
	}
	if err := server.SetReadDeadline(time.Now().Add(25 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if n, _, err := server.ReadFromUDP(buffer[:]); err == nil {
		t.Fatalf("silent peer emitted %x", buffer[:n])
	} else if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatalf("silent peer read failed without a timeout: %v", err)
	}
}

func TestTunnelRadiusAccessAcceptWireShape(t *testing.T) {
	probe, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	local, ok := probe.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("a udp4 socket answered %T, want *net.UDPAddr", probe.LocalAddr())
	}
	port := local.Port
	_ = probe.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- tunnelRadiusDriver(port, 2, tunnelRadiusUint32Attr(27, 60))(ctx, []string{strconv.Itoa(port)})
	}()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("RADIUS driver: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("RADIUS driver did not stop")
		}
	}()

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close() //nolint:errcheck // fixture teardown
	request := make([]byte, 20)
	request[0], request[1] = 1, 9
	binary.BigEndian.PutUint16(request[2:4], 20)
	for index := range request[4:20] {
		request[4+index] = byte(index + 1)
	}
	target := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port}
	var response []byte
	if !Poll(ctx, 20, 25*time.Millisecond, func() bool {
		_ = client.SetDeadline(time.Now().Add(25 * time.Millisecond))
		_, _ = client.WriteToUDP(request, target)
		buffer := make([]byte, 256)
		n, _, readErr := client.ReadFromUDP(buffer)
		if readErr == nil {
			response = append([]byte(nil), buffer[:n]...)
			return true
		}
		return false
	}) {
		t.Fatal("RADIUS driver did not answer Access-Request")
	}
	if len(response) != 26 || response[0] != 2 || response[1] != 9 || binary.BigEndian.Uint32(response[22:26]) != 60 {
		t.Fatalf("Access-Accept = %x", response)
	}
	hash := md5.New() //nolint:gosec // RADIUS response authenticator
	hash.Write(response[:4])
	hash.Write(request[4:20])
	hash.Write(response[20:])
	hash.Write([]byte("testing123"))
	if !bytes.Equal(response[4:20], hash.Sum(nil)) {
		t.Fatalf("response authenticator did not verify: %x", response[4:20])
	}
}

func TestTunnelIPsecIKEHeaderWireShape(t *testing.T) {
	ispi := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	rspi := []byte{9, 10, 11, 12, 13, 14, 15, 16}
	packet := tunnelIPsecIKEHeader(ispi, rspi, 37, 0x20, 11)
	if len(packet) != 28 || !bytes.Equal(packet[:8], ispi) || !bytes.Equal(packet[8:16], rspi) {
		t.Fatalf("IKE header identity fields = %x", packet)
	}
	if packet[17] != 0x20 || packet[18] != 37 || packet[19] != 0x20 || binary.BigEndian.Uint32(packet[20:24]) != 11 || binary.BigEndian.Uint32(packet[24:28]) != 28 {
		t.Fatalf("IKE header fields = %x", packet)
	}
}

func TestTunnelPPPoEDiscoveryPacketTags(t *testing.T) {
	cookie := []byte{1, 2, 3, 4}
	packet := tunnelPPPoEPacket(tunnelPPPoEPADR, cookie, []byte{0x42, 0x42}, "")
	if packet[0] != 0x11 || packet[1] != tunnelPPPoEPADR || int(binary.BigEndian.Uint16(packet[4:6])) != len(packet)-6 {
		t.Fatalf("PPPoE discovery header = %x", packet[:6])
	}
	tags := tunnelPPPoEParseTags(packet[6:])
	if !bytes.Equal(tags[tunnelPPPoEACCookie], cookie) || !bytes.Equal(tags[tunnelPPPoEHostUniq], []byte{0x42, 0x42}) {
		t.Fatalf("PPPoE tags = %#v", tags)
	}
	if _, ok := tags[tunnelPPPoEService]; !ok || len(tags[tunnelPPPoEService]) != 0 {
		t.Fatalf("PPPoE Service-Name tag = %#v, want a present zero-length tag", tags[tunnelPPPoEService])
	}
}

func TestTunnelPPPoEDiscoveryPacketNamedService(t *testing.T) {
	packet := tunnelPPPoEPacket(tunnelPPPoEPADI, nil, []byte{0x50, 0x50}, "internet")
	tags := tunnelPPPoEParseTags(packet[6:])
	if string(tags[tunnelPPPoEService]) != "internet" {
		t.Fatalf("PPPoE Service-Name tag = %q, want %q", tags[tunnelPPPoEService], "internet")
	}
}
