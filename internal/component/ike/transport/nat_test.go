// Design: plan/spec-ipsec-9-ikev2-eap-nat.md -- NAT detection tests

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

func TestNonESPMarkerESPPacket(t *testing.T) {
	espData := []byte{0x00, 0x00, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab}
	_, ok := StripNonESPMarker(espData)
	if ok {
		t.Fatal("ESP packet should not be recognized as IKE")
	}
}

func TestIsNATKeepalive(t *testing.T) {
	if !IsNATKeepalive([]byte{0xFF}) {
		t.Fatal("single 0xFF should be keepalive")
	}
	if IsNATKeepalive([]byte{0xFF, 0x00}) {
		t.Fatal("two bytes should not be keepalive")
	}
	if IsNATKeepalive([]byte{0x00}) {
		t.Fatal("single 0x00 should not be keepalive")
	}
}
