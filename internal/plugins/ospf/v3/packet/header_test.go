// VALIDATES: spec-ospfv3-2-wire AC-1, AC-2, AC-18 -- the 16-octet OSPFv3 common
// header round-trips and DecodeHeader rejects a wrong version, a type outside
// 1..5, and a Packet Length below the header or past the datagram.
// PREVENTS: carrying the OSPFv2 24-octet header (with AuType) into OSPFv3, and
// slicing past a hostile short or oversized buffer.

package packet

import (
	"errors"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv3PeekInstanceID(t *testing.T) {
	h := Header{Type: PacketTypeHello, InstanceID: types.InstanceID(42), Length: CommonHeaderLen}
	buf := make([]byte, CommonHeaderLen)
	h.WriteTo(buf, 0)

	got, ok := PeekInstanceID(buf)
	if !ok || got != types.InstanceID(42) {
		t.Fatalf("PeekInstanceID = (%d, %v), want (42, true)", got, ok)
	}
	// A sub-header buffer is rejected without a panic.
	if _, ok := PeekInstanceID(buf[:CommonHeaderLen-1]); ok {
		t.Fatalf("PeekInstanceID accepted a %d-byte buffer", CommonHeaderLen-1)
	}
}

// RFC requirement: RFC5340-4.2.2-5 positive -- the version number field specifies protocol
// version 3: Header.WriteTo stamps 3 into the Version octet and DecodeHeader accepts it,
// returning the body offset (Header.WriteTo header.go:163-174, DecodeHeader header.go:107-140).
func TestOSPFv3HeaderRoundTrip(t *testing.T) {
	h := Header{
		Type:       PacketTypeHello,
		RouterID:   mustRouterID(t, "192.0.2.1"),
		AreaID:     mustAreaID(t, "0.0.0.7"),
		InstanceID: types.InstanceID(42),
	}
	buf := make([]byte, CommonHeaderLen)
	// Encode a header alone (Length is backfilled by Packet.WriteTo in practice).
	h.Length = CommonHeaderLen
	if n := h.WriteTo(buf, 0); n != CommonHeaderLen {
		t.Fatalf("Header.WriteTo wrote %d, want %d", n, CommonHeaderLen)
	}
	if buf[offVersion] != Version {
		t.Fatalf("version octet = %d, want %d", buf[offVersion], Version)
	}
	if buf[offReserved] != 0 {
		t.Fatalf("reserved octet = %d, want 0", buf[offReserved])
	}
	got, n, err := DecodeHeader(buf)
	if err != nil {
		t.Fatalf("DecodeHeader: %v", err)
	}
	if n != CommonHeaderLen {
		t.Fatalf("DecodeHeader body offset = %d, want %d", n, CommonHeaderLen)
	}
	if got.Type != h.Type || got.RouterID != h.RouterID || got.AreaID != h.AreaID ||
		got.InstanceID != h.InstanceID || got.Length != CommonHeaderLen {
		t.Fatalf("DecodeHeader = %+v, want %+v", got, h)
	}
}

func TestOSPFv3DecodeHeaderBounds(t *testing.T) {
	valid := func() []byte {
		h := Header{Type: PacketTypeHello, RouterID: mustRouterID(t, "10.0.0.1"), AreaID: mustAreaID(t, "0"), Length: CommonHeaderLen}
		buf := make([]byte, CommonHeaderLen)
		h.WriteTo(buf, 0)
		return buf
	}

	tests := []struct {
		name    string
		mutate  func([]byte) []byte
		wantErr error
	}{
		{"short buffer", func(b []byte) []byte { return b[:CommonHeaderLen-1] }, ErrShortBuffer},
		// RFC requirement: RFC5340-4.2.2-5 negative -- a header whose Version octet is not 3
		// (here the OSPFv2 value 2) is rejected with ErrBadVersion before any body parser runs,
		// so a non-version-3 packet is never processed (DecodeHeader, header.go:113-115).
		{"bad version", func(b []byte) []byte { b[offVersion] = 2; return b }, ErrBadVersion},
		{"type zero", func(b []byte) []byte { b[offType] = 0; return b }, ErrUnknownType},
		{"type six", func(b []byte) []byte { b[offType] = 6; return b }, ErrUnknownType},
		{"length below header", func(b []byte) []byte { writeUint16(b, offLength, CommonHeaderLen-1); return b }, ErrLength},
		{"length past datagram", func(b []byte) []byte { writeUint16(b, offLength, CommonHeaderLen+1); return b }, ErrTruncated},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := tc.mutate(valid())
			_, _, err := DecodeHeader(buf)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("DecodeHeader err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// VALIDATES: spec-ospfv3-2-wire -- PacketType.String renders a stable lowercase token for the
// five OSPFv3 packet types and "unknown" for any other value.
// PREVENTS: a diagnostic view mislabeling a known OSPFv3 packet type.
func TestOSPFv3PacketTypeString(t *testing.T) {
	cases := map[PacketType]string{
		PacketTypeHello:    "hello",
		PacketTypeDBDesc:   "dbdesc",
		PacketTypeLSReq:    "ls-request",
		PacketTypeLSUpdate: "ls-update",
		PacketTypeLSAck:    "ls-ack",
		PacketType(0):      "unknown",
		PacketType(6):      "unknown",
	}
	for typ, want := range cases {
		if got := typ.String(); got != want {
			t.Errorf("PacketType(%d).String() = %q, want %q", typ, got, want)
		}
	}
}

// VALIDATES: spec-ospfv3-2-wire -- packetType prefers an explicit Header.Type and otherwise
// derives the type from whichever body pointer is set, returning 0 for an empty packet.
// PREVENTS: WriteTo stamping the wrong type octet when Header.Type is left unset.
func TestOSPFv3PacketTypeDerivation(t *testing.T) {
	cases := []struct {
		name string
		p    Packet
		want PacketType
	}{
		{"explicit-header-type-wins", Packet{Header: Header{Type: PacketTypeLSAck}, Hello: &Hello{}}, PacketTypeLSAck},
		{"derive-hello", Packet{Hello: &Hello{}}, PacketTypeHello},
		{"derive-dbdesc", Packet{DBDesc: &DBDesc{}}, PacketTypeDBDesc},
		{"derive-lsreq", Packet{LSReq: &LSReq{}}, PacketTypeLSReq},
		{"derive-lsupdate", Packet{LSUpdate: &LSUpdate{}}, PacketTypeLSUpdate},
		{"derive-lsack", Packet{LSAck: &LSAck{}}, PacketTypeLSAck},
		{"empty-packet", Packet{}, PacketType(0)},
	}
	for _, tc := range cases {
		if got := tc.p.packetType(); got != tc.want {
			t.Errorf("%s: packetType() = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// TestOSPFv3HeaderHasNoAuType asserts the OSPFv2 AuType/Authentication octets are
// gone: a 16-octet header is the entire header, and the reclaimed octets are
// Instance ID + Reserved.
func TestOSPFv3HeaderHasNoAuType(t *testing.T) {
	if CommonHeaderLen != 16 {
		t.Fatalf("CommonHeaderLen = %d, want 16 (OSPFv3 has no 8-octet AuType field)", CommonHeaderLen)
	}
	if offInstanceID != 14 || offReserved != 15 {
		t.Fatalf("Instance ID / Reserved offsets = %d/%d, want 14/15", offInstanceID, offReserved)
	}
}
