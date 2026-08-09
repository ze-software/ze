// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- NAT detection tests

package transport

import (
	"net"
	"testing"
)

func TestNATDetectionHash(t *testing.T) {
	spiI := [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	spiR := [8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}
	ip := net.ParseIP("192.168.1.1")
	port := uint16(500)

	hash := NATDetectionHash(spiI, spiR, ip, port)
	if len(hash) != 20 {
		t.Fatalf("hash length: got %d, want 20", len(hash))
	}

	// Same inputs produce same hash.
	hash2 := NATDetectionHash(spiI, spiR, ip, port)
	for i := range hash {
		if hash[i] != hash2[i] {
			t.Fatal("same inputs produced different hashes")
		}
	}

	// Different IP produces different hash.
	hash3 := NATDetectionHash(spiI, spiR, net.ParseIP("10.0.0.1"), port)
	same := true
	for i := range hash {
		if hash[i] != hash3[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different IPs produced same hash")
	}
}

// RFC requirement: RFC7296-2.23-1 positive -- NAT is detected automatically by comparing the
// SHA-1(SPIi|SPIr|IP|Port) hash a peer sent against the locally computed value: DetectNAT
// (nat.go:40) reports NAT present when the address the hash was computed over differs.
func TestNATDetectionPresent(t *testing.T) {
	spiI := [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	spiR := [8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}
	realIP := net.ParseIP("192.168.1.1")
	nattedIP := net.ParseIP("203.0.113.5")
	port := uint16(500)

	// Compute hash with the real (pre-NAT) IP.
	received := NATDetectionHash(spiI, spiR, realIP, port)

	// Detection with the NATted IP should detect NAT.
	natPresent := DetectNAT(received, spiI, spiR, nattedIP, port)
	if !natPresent {
		t.Fatal("should detect NAT when IPs differ")
	}
}

// RFC requirement: RFC7296-2.23-1 negative -- when the received hash matches the locally
// computed SHA-1 over the same address/port, DetectNAT (nat.go:40) reports no NAT, so the
// automatic comparison does not raise a false positive on an un-NATted path.
func TestNATDetectionAbsent(t *testing.T) {
	spiI := [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	spiR := [8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}
	ip := net.ParseIP("192.168.1.1")
	port := uint16(500)

	// Hash computed with same IP: no NAT.
	received := NATDetectionHash(spiI, spiR, ip, port)
	natPresent := DetectNAT(received, spiI, spiR, ip, port)
	if natPresent {
		t.Fatal("should not detect NAT when IPs match")
	}
}

// RFC requirement: RFC7296-2.23-3 positive -- an IKE packet on port 4500 is prefixed with the
// 4 zero bytes (the Non-ESP marker): AddNonESPMarker (nat.go:58) prepends them and
// StripNonESPMarker (nat.go:67,76-77) recovers the IKE bytes from a marked packet.
// RFC requirement: RFC3948-2.2-1 positive -- AddNonESPMarker prepends the 4-byte zero
// Non-ESP Marker to an IKE message on port 4500 (nat.go:58), and StripNonESPMarker recovers
// the original IKE bytes from a marked packet (nat.go:67,76-77).
// RFC requirement: RFC3948-2.1-3 positive -- demultiplexing recognizes a leading 4-zero-byte
// marker as IKE: StripNonESPMarker returns the IKE payload with ok=true (nat.go:67,76-77).
func TestNonESPMarker(t *testing.T) {
	ikeMsg := []byte{0x01, 0x02, 0x03, 0x04}
	marked := AddNonESPMarker(ikeMsg)

	if len(marked) != NonESPMarkerLen+len(ikeMsg) {
		t.Fatalf("length: got %d, want %d", len(marked), NonESPMarkerLen+len(ikeMsg))
	}
	for i := range NonESPMarkerLen {
		if marked[i] != 0 {
			t.Fatalf("marker byte %d: got %d, want 0", i, marked[i])
		}
	}

	stripped, ok := StripNonESPMarker(marked)
	if !ok {
		t.Fatal("StripNonESPMarker returned false for valid IKE packet")
	}
	for i := range ikeMsg {
		if stripped[i] != ikeMsg[i] {
			t.Fatalf("stripped[%d]: got %d, want %d", i, stripped[i], ikeMsg[i])
		}
	}
}

// RFC requirement: RFC7296-2.23-3 negative -- a packet on port 4500 whose leading bytes are a
// non-zero ESP SPI (no 4-zero Non-ESP marker) is not treated as an IKE packet: StripNonESPMarker
// returns ok=false (nat.go:79-80), so only IKE packets carry the marker.
// RFC requirement: RFC3948-2.2-1 negative -- a packet whose first bytes are a non-zero ESP
// SPI is NOT treated as a marked IKE packet: StripNonESPMarker returns ok=false (nat.go:79-80),
// so the marker is never falsely stripped off ESP payload.
// RFC requirement: RFC3948-2.1-3 negative -- demultiplexing does not misclassify ESP as IKE:
// non-zero leading bytes yield ok=false, so the packet is routed as ESP, not IKE (nat.go:79-80).
func TestNonESPMarkerESPPacket(t *testing.T) {
	espData := []byte{0x00, 0x00, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab}
	_, ok := StripNonESPMarker(espData)
	if ok {
		t.Fatal("ESP packet should not be recognized as IKE")
	}
}

// RFC requirement: RFC3948-2.1-3 positive -- demultiplexing classifies a single 0xFF byte as
// a NAT-keepalive (IsNATKeepalive true), the third packet type sharing port 4500 (nat.go:85).
func TestIsNATKeepalive(t *testing.T) {
	if !IsNATKeepalive([]byte{0xFF}) {
		t.Fatal("single 0xFF should be keepalive")
	}
	// RFC requirement: RFC3948-2.1-3 negative -- demultiplexing does not over-classify as a
	// keepalive: a 2-byte payload or a single 0x00 byte is NOT a keepalive (nat.go:86), so only
	// the exact 1-byte 0xFF form is consumed as one.
	if IsNATKeepalive([]byte{0xFF, 0x00}) {
		t.Fatal("two bytes should not be keepalive")
	}
	if IsNATKeepalive([]byte{0x00}) {
		t.Fatal("single 0x00 should not be keepalive")
	}
}
