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

func TestTunnelRadiusAccessAcceptWireShape(t *testing.T) {
	probe, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	port := probe.LocalAddr().(*net.UDPAddr).Port
	_ = probe.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- tunnelRadiusDriver(port, 2, tunnelRadiusUint32Attr(27, 60), false)(ctx, []string{strconv.Itoa(port)})
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
	defer client.Close()
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
	packet := tunnelPPPoEPacket(tunnelPPPoEPADR, cookie, []byte{0x42, 0x42})
	if packet[0] != 0x11 || packet[1] != tunnelPPPoEPADR || int(binary.BigEndian.Uint16(packet[4:6])) != len(packet)-6 {
		t.Fatalf("PPPoE discovery header = %x", packet[:6])
	}
	tags := tunnelPPPoEParseTags(packet[6:])
	if !bytes.Equal(tags[tunnelPPPoEACCookie], cookie) || !bytes.Equal(tags[tunnelPPPoEHostUniq], []byte{0x42, 0x42}) {
		t.Fatalf("PPPoE tags = %#v", tags)
	}
}
