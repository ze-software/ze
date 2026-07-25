// VALIDATES: spec-ospfv3-2-wire AC-3 -- the Hello body round-trips Interface ID,
// 24-bit Options, priority, the 2-octet RouterDeadInterval, DR/BDR Router IDs,
// and the neighbor list, and there is no network mask field.
// PREVENTS: re-introducing the OSPFv2 network mask or a 4-octet DeadInterval.

package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func sampleHello(t *testing.T) Hello {
	t.Helper()
	return Hello{
		InterfaceID:        types.InterfaceID(0x01020304),
		Priority:           1,
		Options:            mustOptions(t, uint32(types.OptV6|types.OptE|types.OptR)),
		HelloInterval:      10,
		RouterDeadInterval: 40,
		DR:                 mustRouterID(t, "10.0.0.1"),
		BDR:                mustRouterID(t, "10.0.0.2"),
		Neighbors:          []types.RouterID{mustRouterID(t, "10.0.0.2"), mustRouterID(t, "10.0.0.3")},
	}
}

func TestOSPFv3HelloRoundTrip(t *testing.T) {
	want := sampleHello(t)
	p := Packet{Header: sampleHeader(t, PacketTypeHello), Hello: &want}
	wire := encodePacket(t, p)

	got, err := DecodePacket(wire)
	if err != nil {
		t.Fatalf("DecodePacket hello: %v", err)
	}
	if got.Hello == nil {
		t.Fatalf("decoded packet has no Hello body")
	}
	h := got.Hello
	if h.InterfaceID != want.InterfaceID {
		t.Fatalf("InterfaceID = %d, want %d", h.InterfaceID, want.InterfaceID)
	}
	if h.Options != want.Options {
		t.Fatalf("Options = %#06x, want %#06x", uint32(h.Options), uint32(want.Options))
	}
	if h.Priority != want.Priority || h.HelloInterval != want.HelloInterval || h.RouterDeadInterval != want.RouterDeadInterval {
		t.Fatalf("scalar mismatch: got %+v want %+v", h, want)
	}
	if h.DR != want.DR || h.BDR != want.BDR {
		t.Fatalf("DR/BDR mismatch: got %v/%v want %v/%v", h.DR, h.BDR, want.DR, want.BDR)
	}
	if len(h.Neighbors) != len(want.Neighbors) {
		t.Fatalf("neighbor count = %d, want %d", len(h.Neighbors), len(want.Neighbors))
	}
	for i := range want.Neighbors {
		if h.Neighbors[i] != want.Neighbors[i] {
			t.Fatalf("neighbor[%d] = %v, want %v", i, h.Neighbors[i], want.Neighbors[i])
		}
	}
	// The body has a 20-octet fixed prefix (no 4-octet network mask): a packet
	// with N neighbors is exactly header + 20 + 4N octets.
	wantLen := CommonHeaderLen + helloFixedLen + len(want.Neighbors)*types.IDLen
	if len(wire) != wantLen {
		t.Fatalf("encoded length = %d, want %d (no network mask)", len(wire), wantLen)
	}
}

func TestOSPFv3HelloRejectsMisalignedNeighbors(t *testing.T) {
	want := sampleHello(t)
	p := Packet{Header: sampleHeader(t, PacketTypeHello), Hello: &want}
	wire := encodePacket(t, p)
	// Append one stray octet to the body and fix the Packet Length so DecodeHeader
	// passes but the neighbor list no longer aligns to 4 octets.
	bad := append(append([]byte(nil), wire...), 0x00)
	writeUint16(bad, offLength, uint16(len(bad)))
	if _, err := DecodePacket(bad); err == nil {
		t.Fatalf("DecodePacket accepted a misaligned neighbor list")
	}
}
