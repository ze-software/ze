// VALIDATES: spec-ospf-af-unify Phase 2 -- the v4 Codec adapter satisfies the engine
// Codec interface and projects the OSPFv2 common header / verifies the checksum
// identically to calling ospf/packet directly (the seam is behavior-preserving).
// PREVENTS: a future v6 adapter or a refactor silently changing v2 header decode or
// checksum semantics when the engine is routed through the Codec interface.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestOSPFCodecInterfaceV4Adapter(t *testing.T) {
	// A valid OSPFv2 common header on the wire (RouterID 1.1.1.1, backbone area).
	src := packet.Header{
		Type:     packet.PacketTypeHello,
		Length:   packet.CommonHeaderLen,
		RouterID: types.RouterID{1, 1, 1, 1},
		AreaID:   types.AreaID{0, 0, 0, 0},
	}
	buf := make([]byte, packet.CommonHeaderLen)
	src.WriteTo(buf, 0)

	var c Codec = v4Codec{}

	got, err := c.DecodeHeader(buf)
	if err != nil {
		t.Fatalf("adapter DecodeHeader: %v", err)
	}
	want, _, err := packet.DecodeHeader(buf)
	if err != nil {
		t.Fatalf("packet.DecodeHeader: %v", err)
	}
	if got.Type != PacketType(want.Type) || got.Length != want.Length ||
		got.RouterID != want.RouterID || got.AreaID != want.AreaID || got.Checksum != want.Checksum {
		t.Fatalf("adapter header %+v does not match packet header %+v", got, want)
	}
	if got.InstanceID != 0 {
		t.Errorf("v4 InstanceID = %d, want 0 (OSPFv2 has no Instance ID)", got.InstanceID)
	}

	// VerifyChecksum must delegate to packet.VerifyPacketChecksum (src/dst ignored for v2).
	if c.VerifyChecksum(buf, netip.Addr{}, netip.Addr{}) != packet.VerifyPacketChecksum(buf) {
		t.Fatalf("adapter VerifyChecksum disagrees with packet.VerifyPacketChecksum")
	}

	// The decoded type must map to the neutral Hello constant (1..5 identical across versions).
	if got.Type != PacketTypeHello {
		t.Errorf("decoded type = %d, want PacketTypeHello (%d)", got.Type, PacketTypeHello)
	}
}

// TestV4CodecSurfacesInstanceID proves AC-3 / A-3 (RFC 6549): the v4 codec projects the
// OSPFv2 header Instance ID (offset 14) into the neutral Header.InstanceID instead of the
// old hard-coded 0, so the shared dispatcher demux can act on it for the IPv4 family.
func TestV4CodecSurfacesInstanceID(t *testing.T) {
	for _, id := range []uint8{0, 5, 255} {
		src := packet.Header{
			Type:       packet.PacketTypeHello,
			Length:     packet.CommonHeaderLen,
			RouterID:   types.RouterID{1, 1, 1, 1},
			AreaID:     types.AreaID{0, 0, 0, 0},
			InstanceID: id,
		}
		buf := make([]byte, packet.CommonHeaderLen)
		src.WriteTo(buf, 0)

		got, err := v4Codec{}.DecodeHeader(buf)
		if err != nil {
			t.Fatalf("id %d: DecodeHeader: %v", id, err)
		}
		if got.InstanceID != id {
			t.Fatalf("id %d: neutral Header.InstanceID = %d, want %d", id, got.InstanceID, id)
		}
	}
}

func TestOSPFCodecDecodeHelloV4(t *testing.T) {
	hello := packet.Hello{
		NetworkMask:   [4]byte{255, 255, 255, 0},
		HelloInterval: 10,
		Options:       types.OptionE,
		Priority:      1,
		DeadInterval:  40,
		DR:            [4]byte{10, 0, 0, 1},
		Neighbors:     []types.RouterID{{2, 2, 2, 2}},
	}
	p := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello, RouterID: types.RouterID{1, 1, 1, 1}}, Hello: &hello}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)

	got, err := v4Codec{}.DecodeHello(buf)
	if err != nil {
		t.Fatalf("DecodeHello: %v", err)
	}
	if got.NetworkMask != ([4]byte{255, 255, 255, 0}) || got.HelloInterval != 10 || got.DeadInterval != 40 || got.Priority != 1 {
		t.Fatalf("decoded v4 Hello = %+v", got)
	}
	if !got.Options.Has(types.OptionE) {
		t.Errorf("E-bit lost in decode")
	}
	if got.DR != ([4]byte{10, 0, 0, 1}) {
		t.Errorf("DR = %v, want 10.0.0.1", got.DR)
	}
	if len(got.Neighbors) != 1 || got.Neighbors[0] != (types.RouterID{2, 2, 2, 2}) {
		t.Errorf("neighbors = %v, want [2.2.2.2]", got.Neighbors)
	}
	if got.InterfaceID != 0 {
		t.Errorf("v4 InterfaceID = %d, want 0 (OSPFv2 has no Interface ID)", got.InterfaceID)
	}
}
