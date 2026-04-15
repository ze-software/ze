//go:build linux

package l2tp

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

func TestSockaddrPPPoL2TP(t *testing.T) {
	// VALIDATES: AC-8 -- sockaddr_pppol2tp binary layout: family,
	// protocol, pid, fd, peer sockaddr_in, tunnel/session IDs.
	// PREVENTS: kernel connect() rejects due to wrong sockaddr layout.
	peer := netip.MustParseAddrPort("192.0.2.1:1701")
	buf, err := buildSockaddrPPPoL2TP(42, peer, 100, 1001, 200, 2001)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// Decode the buffer back into a struct view to verify every field.
	if uintptr(len(buf)) < unsafe.Sizeof(sockaddrPPPoL2TP{}) {
		t.Fatalf("sockaddr length %d less than struct size %d",
			len(buf), unsafe.Sizeof(sockaddrPPPoL2TP{}))
	}
	sa := *(*sockaddrPPPoL2TP)(unsafe.Pointer(&buf[0]))

	if sa.Family != afPPPOX {
		t.Fatalf("Family: want %d (AF_PPPOX), got %d", afPPPOX, sa.Family)
	}
	if sa.Protocol != pxProtoOL2TP {
		t.Fatalf("Protocol: want %d (PX_PROTO_OL2TP), got %d", pxProtoOL2TP, sa.Protocol)
	}
	if sa.PID != 0 {
		t.Fatalf("PID: want 0 (current process), got %d", sa.PID)
	}
	if sa.FD != 42 {
		t.Fatalf("FD: want 42, got %d", sa.FD)
	}
	if sa.Addr.Family != unix.AF_INET {
		t.Fatalf("peer Family: want AF_INET, got %d", sa.Addr.Family)
	}
	if sa.STunnel != 100 {
		t.Fatalf("STunnel: want 100, got %d", sa.STunnel)
	}
	if sa.SSession != 1001 {
		t.Fatalf("SSession: want 1001, got %d", sa.SSession)
	}
	if sa.DTunnel != 200 {
		t.Fatalf("DTunnel: want 200, got %d", sa.DTunnel)
	}
	if sa.DSession != 2001 {
		t.Fatalf("DSession: want 2001, got %d", sa.DSession)
	}
	want := [4]byte{192, 0, 2, 1}
	if sa.Addr.Addr != want {
		t.Fatalf("peer Addr: want %v, got %v", want, sa.Addr.Addr)
	}
}

func TestSockaddrPPPoL2TPRejectsIPv6(t *testing.T) {
	// VALIDATES: builder rejects IPv6 peers (phase 5 is IPv4-only).
	// PREVENTS: silent truncation of IPv6 address into IPv4 sockaddr.
	peer := netip.MustParseAddrPort("[2001:db8::1]:1701")
	_, err := buildSockaddrPPPoL2TP(0, peer, 1, 2, 3, 4)
	if err == nil {
		t.Fatal("expected error for IPv6 peer, got nil")
	}
}

func TestHtons(t *testing.T) {
	// VALIDATES: htons stores network-order bytes in the uint16's memory.
	// PREVENTS: peer port set in host byte order, which the kernel
	// interprets as the wrong port number.
	// 1701 = 0x06A5; in the struct field, memory must hold 06 then A5.
	got := htons(1701)
	var mem [2]byte
	// Write got using native byte order; on little-endian amd64/arm64
	// that yields the low byte first. htons constructed the uint16 so
	// that low-byte-first-in-memory equals the network-order MSB.
	*(*uint16)(unsafe.Pointer(&mem[0])) = got
	if mem[0] != 0x06 || mem[1] != 0xA5 {
		t.Fatalf("htons(1701): want bytes 06 A5 in native memory, got %02x %02x",
			mem[0], mem[1])
	}
	// Also verify decoding as big-endian gives back the original value.
	if v := binary.BigEndian.Uint16(mem[:]); v != 1701 {
		t.Fatalf("re-decoded BE: want 1701, got %d", v)
	}
}
