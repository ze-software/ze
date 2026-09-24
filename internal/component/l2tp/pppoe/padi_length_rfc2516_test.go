// Design: docs/architecture/l2tp/bng-5-pppoe.md -- RFC 2516 conformance coverage
//
// Proves RFC 2516 Section 5.1 and Appendix A on the PADI length: its
// Ethernet payload cannot exceed 1484 octets, leaving 16 octets for a
// Relay-Session-Id tag with a 12-octet value.

package pppoe

import (
	"strings"
	"testing"
)

// TestPADINeverExceeds1484Octets builds a PADI at the Ethernet payload
// boundary and one octet above it, then checks the available relay-tag room.
//
// RFC requirement: RFC2516-5.1-5 positive -- BuildPADI accepts an Ethernet payload of exactly 1484 octets, including the PPPoE header, in a 1498-octet frame.
// RFC requirement: RFC2516-5.1-5 negative -- BuildPADI returns nil for a 1485-octet Ethernet payload, even when the caller's buffer can hold it.
// RFC requirement: RFC2516-x-10 positive -- the largest PADI leaves exactly 16 octets under the 1500-octet Ethernet payload limit for a Relay-Session-Id tag with a 12-octet value.
// RFC requirement: RFC2516-x-10 negative -- BuildPADI refuses a PADI that would leave only 15 octets for that relay tag.
func TestPADINeverExceeds1484Octets(t *testing.T) {
	t.Parallel()

	var buf [EthMaxLen]byte
	src := [EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	hostUniq := []byte{1, 2, 3, 4}

	// Two tags: Service-Name (4 + n) and Host-Uniq (4 + 4).
	fill := PADIMaxLen - PPPoEHdrLen - TagHdrLen - (TagHdrLen + len(hostUniq))
	exact := BuildPADI(buf[:], src, strings.Repeat("s", fill), hostUniq)
	if exact == nil {
		t.Fatal("BuildPADI refused a PADI of exactly 1484 octets")
	}
	payloadLen := len(exact) - EthHdrLen
	if payloadLen != PADIMaxLen {
		t.Fatalf("PADI payload is %d octets, want %d", payloadLen, PADIMaxLen)
	}
	const relayTagLen = TagHdrLen + 12
	if EthMaxData-payloadLen != relayTagLen {
		t.Fatalf("PADI leaves %d octets for a relay tag, want %d", EthMaxData-payloadLen, relayTagLen)
	}

	over := BuildPADI(buf[:], src, strings.Repeat("s", fill+1), hostUniq)
	if over != nil {
		t.Fatalf("BuildPADI emitted a %d-octet payload; Section 5.1 caps it at %d", len(over)-EthHdrLen, PADIMaxLen)
	}
}
