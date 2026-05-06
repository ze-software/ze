//go:build linux

package l2tp

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

func TestSockaddrPPPoL2TP(t *testing.T) {
	peer := netip.MustParseAddrPort("192.0.2.1:1701")
	buf, err := buildSockaddrPPPoL2TP(42, peer, 100, 1001, 200, 2001)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if len(buf) != sockaddrPPPoL2TPSize {
		t.Fatalf("length: want %d (packed), got %d", sockaddrPPPoL2TPSize, len(buf))
	}

	// Verify every field at its packed offset.
	if v := binary.LittleEndian.Uint16(buf[0:2]); v != afPPPOX {
		t.Fatalf("[0:2] sa_family: want %d (AF_PPPOX), got %d", afPPPOX, v)
	}
	if v := binary.LittleEndian.Uint32(buf[2:6]); v != pxProtoOL2TP {
		t.Fatalf("[2:6] sa_protocol: want %d (PX_PROTO_OL2TP), got %d", pxProtoOL2TP, v)
	}
	if v := binary.LittleEndian.Uint32(buf[6:10]); v != 0 {
		t.Fatalf("[6:10] pid: want 0, got %d", v)
	}
	if v := binary.LittleEndian.Uint32(buf[10:14]); v != 42 {
		t.Fatalf("[10:14] fd: want 42, got %d", v)
	}
	if v := binary.LittleEndian.Uint16(buf[14:16]); v != 2 {
		t.Fatalf("[14:16] sin_family: want 2 (AF_INET), got %d", v)
	}
	if v := binary.BigEndian.Uint16(buf[16:18]); v != 1701 {
		t.Fatalf("[16:18] sin_port: want 1701, got %d", v)
	}
	wantIP := [4]byte{192, 0, 2, 1}
	if got := [4]byte(buf[18:22]); got != wantIP {
		t.Fatalf("[18:22] sin_addr: want %v, got %v", wantIP, got)
	}
	for i := 22; i < 30; i++ {
		if buf[i] != 0 {
			t.Fatalf("[22:30] sin_zero: byte %d not zero: %02x", i, buf[i])
		}
	}
	if v := binary.LittleEndian.Uint16(buf[30:32]); v != 100 {
		t.Fatalf("[30:32] s_tunnel: want 100, got %d", v)
	}
	if v := binary.LittleEndian.Uint16(buf[32:34]); v != 1001 {
		t.Fatalf("[32:34] s_session: want 1001, got %d", v)
	}
	if v := binary.LittleEndian.Uint16(buf[34:36]); v != 200 {
		t.Fatalf("[34:36] d_tunnel: want 200, got %d", v)
	}
	if v := binary.LittleEndian.Uint16(buf[36:38]); v != 2001 {
		t.Fatalf("[36:38] d_session: want 2001, got %d", v)
	}
}

func TestSockaddrPPPoL2TPRejectsIPv6(t *testing.T) {
	peer := netip.MustParseAddrPort("[2001:db8::1]:1701")
	_, err := buildSockaddrPPPoL2TP(0, peer, 1, 2, 3, 4)
	if err == nil {
		t.Fatal("expected error for IPv6 peer, got nil")
	}
}
