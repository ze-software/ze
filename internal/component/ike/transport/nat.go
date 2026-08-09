// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- NAT detection and port 4500 handling
// RFC: rfc/short/rfc7296.md -- NAT detection via hash notify payloads (Section 2.23)
// RFC: rfc/short/rfc3948.md -- Non-ESP marker, UDP encapsulation on port 4500

package transport

import (
	"crypto/sha1" //nolint:gosec // RFC 7296 Section 2.23 mandates SHA-1 for NAT detection
	"encoding/binary"
	"net"
)

const (
	NATTPort = 4500

	// NonESPMarkerLen is the 4-byte zero prefix for IKE packets on port 4500.
	// RFC 3948 Section 2.2: distinguishes IKE from ESP on the shared port.
	NonESPMarkerLen = 4
)

// NATDetectionHash computes SHA-1(SPIi || SPIr || IP || Port) per RFC 7296 Section 2.23.
func NATDetectionHash(spiI, spiR [8]byte, ip net.IP, port uint16) []byte {
	h := sha1.New() //nolint:gosec // mandated by RFC, not a security hash here
	h.Write(spiI[:])
	h.Write(spiR[:])
	if ip4 := ip.To4(); ip4 != nil {
		h.Write(ip4)
	} else {
		h.Write(ip.To16())
	}
	var portBuf [2]byte
	binary.BigEndian.PutUint16(portBuf[:], port)
	h.Write(portBuf[:])
	return h.Sum(nil)
}

// DetectNAT compares the received NAT detection hash against the expected value.
// Returns true if NAT is present (hashes do not match).
// The received hash is the 20-byte SHA-1 from the notify payload.
func DetectNAT(received []byte, spiI, spiR [8]byte, localIP net.IP, localPort uint16) bool {
	expected := NATDetectionHash(spiI, spiR, localIP, localPort)
	return !constantTimeEqual(received, expected)
}

func constantTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// AddNonESPMarker prepends the 4-byte non-ESP marker to an IKE message.
// RFC 3948 Section 2.2: IKE on port 4500 must have 4 zero bytes before the header.
func AddNonESPMarker(ikeMsg []byte) []byte {
	out := make([]byte, NonESPMarkerLen+len(ikeMsg))
	copy(out[NonESPMarkerLen:], ikeMsg)
	return out
}

// StripNonESPMarker removes the 4-byte non-ESP marker from data received on port 4500.
// Returns the IKE message bytes and true if the marker was present (IKE packet),
// or nil and false if this is an ESP or keepalive packet.
func StripNonESPMarker(data []byte) ([]byte, bool) {
	if len(data) < NonESPMarkerLen+1 {
		return nil, false
	}
	// NAT keepalive: single 0xFF byte.
	if len(data) == 1 && data[0] == 0xFF {
		return nil, false
	}
	// Non-ESP marker: first 4 bytes are zero.
	if data[0] == 0 && data[1] == 0 && data[2] == 0 && data[3] == 0 {
		return data[NonESPMarkerLen:], true
	}
	// Non-zero first 4 bytes: ESP packet.
	return nil, false
}

// IsNATKeepalive returns true if the data is a NAT keepalive packet (single 0xFF byte).
// RFC 3948 Section 2.3.
func IsNATKeepalive(data []byte) bool {
	return len(data) == 1 && data[0] == 0xFF
}
