// Design: docs/architecture/wire/isis.md -- fixture pin for test/isis-wire/isis-pdu-1.ci
package packet

import (
	"encoding/hex"
	"testing"
)

// ciFixtureHex is the exact LAN L1 IIH used by test/isis-wire/isis-pdu-1.ci. It
// is pinned here so the Go codec and the functional fixture cannot drift: if a
// codec change alters the bytes, this test fails alongside the .ci. The PDU is
// a captured-style Hello carrying TLV 1 (area 49.0001), TLV 129 (IPv4 NLPID),
// and TLV 6 (one neighbor SNPA).
const ciFixtureHex = "831b01060f01000003000000000001001e002c40000000000002010104034900018101cc0606001122334455"

// VALIDATES: Story 1 wiring -- the captured IS-IS Hello fixture used by the
// functional test decodes through DecodePDU into the expected LAN L1 IIH with
// its three TLVs. The .ci runs the same bytes through `ze isis decode`; this
// keeps the fixture and the codec in lock-step.
// PREVENTS: the functional fixture silently diverging from codec output.
func TestISISCIFixtureDecodes(t *testing.T) {
	wire, err := hex.DecodeString(ciFixtureHex)
	if err != nil {
		t.Fatalf("fixture hex: %v", err)
	}
	pdu, err := DecodePDU(wire)
	if err != nil {
		t.Fatalf("DecodePDU(fixture): %v", err)
	}
	if pdu.Header.PDUType != PDUTypeL1LANHello {
		t.Fatalf("PDU type = %v, want l1-lan-hello", pdu.Header.PDUType)
	}
	h := pdu.LANHello
	if h == nil {
		t.Fatal("LANHello nil")
	}
	if h.CircuitType != CircuitL1L2 {
		t.Errorf("circuit type = %d, want %d", h.CircuitType, CircuitL1L2)
	}
	if got := h.SystemID.String(); got != "0000.0000.0001" {
		t.Errorf("system-id = %q, want 0000.0000.0001", got)
	}
	if h.HoldingTime.Seconds() != 30 {
		t.Errorf("holding-time = %d, want 30", h.HoldingTime.Seconds())
	}
	if h.Priority != 64 {
		t.Errorf("priority = %d, want 64", h.Priority)
	}
	if got := h.LANID.String(); got != "0000.0000.0002.01" {
		t.Errorf("lan-id = %q, want 0000.0000.0002.01", got)
	}
	wantTLVTypes := []uint8{TLVAreaAddresses, TLVProtocolsSupported, TLVISNeighbors}
	if len(h.TLVs) != len(wantTLVTypes) {
		t.Fatalf("got %d TLVs, want %d", len(h.TLVs), len(wantTLVTypes))
	}
	for i, want := range wantTLVTypes {
		if h.TLVs[i].Type != want {
			t.Errorf("tlv[%d].Type = %d, want %d", i, h.TLVs[i].Type, want)
		}
	}
	// The TLV 1 value must carry the area 49.0001 (1-octet length 3 + 49 00 01).
	area, err := DecodeAreaAddressesTLV(h.TLVs[0].Value)
	if err != nil {
		t.Fatalf("DecodeAreaAddressesTLV: %v", err)
	}
	if len(area.Areas) != 1 || area.Areas[0].String() != "49.0001" {
		t.Errorf("area = %v, want [49.0001]", area.Areas)
	}
}
