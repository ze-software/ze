// Design: docs/architecture/ospf/ospf-2-wire.md -- common header tests

package packet

import (
	"errors"
	"testing"
)

// VALIDATES: AC-1 - OSPFv2 version and packet type constants are pinned.
// PREVENTS: wire interop breakage from accidental renumbering.
func TestOSPFPacketTypeConstants(t *testing.T) {
	if Version != 2 || CommonHeaderLen != 24 {
		t.Fatalf("version/header constants = %d/%d, want 2/24", Version, CommonHeaderLen)
	}
	cases := map[PacketType]byte{
		PacketTypeHello:    1,
		PacketTypeDBDesc:   2,
		PacketTypeLSReq:    3,
		PacketTypeLSUpdate: 4,
		PacketTypeLSAck:    5,
	}
	for typ, want := range cases {
		if byte(typ) != want || !typ.known() {
			t.Fatalf("packet type %v = %d known=%v, want %d known", typ, typ, typ.known(), want)
		}
	}
}

// VALIDATES: AC-1 - PacketType.String renders a stable lowercase token for each of the five
// OSPF packet types and "unknown" for any other value.
// PREVENTS: a diagnostic/JSON view labeling a known packet type as "unknown" or renaming one.
func TestPacketTypeString(t *testing.T) {
	cases := map[PacketType]string{
		PacketTypeHello:    "hello",
		PacketTypeDBDesc:   "dbdesc",
		PacketTypeLSReq:    "ls-request",
		PacketTypeLSUpdate: "ls-update",
		PacketTypeLSAck:    "ls-ack",
		PacketType(0):      "unknown",
		PacketType(6):      "unknown",
		PacketType(255):    "unknown",
	}
	for typ, want := range cases {
		if got := typ.String(); got != want {
			t.Errorf("PacketType(%d).String() = %q, want %q", typ, got, want)
		}
	}
}

// VALIDATES: AC-1 - DecodeHeader rejects a packet type outside 1..5 with ErrUnknownType
// before any body parser runs.
// PREVENTS: dispatching an unknown packet type as if it were a known body.
func TestDecodeHeaderRejectsUnknownType(t *testing.T) {
	for _, bad := range []byte{0, 6, 200} {
		h := sampleHeader(t, PacketTypeHello)
		h.Length = CommonHeaderLen
		buf := make([]byte, CommonHeaderLen)
		h.WriteTo(buf, 0)
		buf[offType] = bad
		if _, _, err := DecodeHeader(buf); !errors.Is(err, ErrUnknownType) {
			t.Errorf("DecodeHeader(type=%d) err = %v, want ErrUnknownType", bad, err)
		}
	}
}

// VALIDATES: AC-1 - packetType prefers an explicit Header.Type but otherwise derives the type
// from whichever body pointer is set, and returns 0 for an empty packet.
// PREVENTS: WriteTo stamping the wrong type octet when the caller leaves Header.Type unset.
func TestPacketTypeDerivation(t *testing.T) {
	hello := sampleHello(t)
	dd := &DBDesc{}
	lsreq := &LSReq{}
	lsupd := &LSUpdate{}
	lsack := &LSAck{}
	cases := []struct {
		name string
		p    Packet
		want PacketType
	}{
		{"explicit-header-type-wins", Packet{Header: Header{Type: PacketTypeLSAck}, Hello: &hello}, PacketTypeLSAck},
		{"derive-hello", Packet{Hello: &hello}, PacketTypeHello},
		{"derive-dbdesc", Packet{DBDesc: dd}, PacketTypeDBDesc},
		{"derive-lsreq", Packet{LSReq: lsreq}, PacketTypeLSReq},
		{"derive-lsupdate", Packet{LSUpdate: lsupd}, PacketTypeLSUpdate},
		{"derive-lsack", Packet{LSAck: lsack}, PacketTypeLSAck},
		{"empty-packet", Packet{}, PacketType(0)},
	}
	for _, tc := range cases {
		if got := tc.p.packetType(); got != tc.want {
			t.Errorf("%s: packetType() = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// VALIDATES: AC-1 - VerifyChecksum recomputes the Fletcher/one's-complement packet checksum on
// decoded RawBytes: a freshly encoded-then-decoded packet verifies, a single flipped body byte
// fails, and an empty RawBytes (never decoded) is rejected rather than panicking.
// PREVENTS: accepting a corrupted OSPF packet, or dereferencing an empty raw slice.
// RFC requirement: RFC2328-A.3.1-2 negative -- corrupting a covered octet makes the header checksum verification fail, so a packet whose checksum does not cover the wire bytes is rejected (VerifyPacketChecksum, checksum.go:23-33).
func TestPacketVerifyChecksum(t *testing.T) {
	hello := sampleHello(t)
	buf := encodePacket(t, Packet{Header: sampleHeader(t, PacketTypeHello), Hello: &hello})
	p, err := DecodePacket(buf)
	if err != nil {
		t.Fatalf("DecodePacket: %v", err)
	}
	if !p.VerifyChecksum() {
		t.Fatalf("VerifyChecksum() = false on a freshly encoded packet, want true")
	}

	// Corrupt a body octet in the decoded RawBytes: the checksum must now fail.
	corrupt := append([]byte(nil), buf...)
	corrupt[CommonHeaderLen] ^= 0xff
	bad, err := DecodePacket(corrupt)
	if err != nil {
		t.Fatalf("DecodePacket(corrupt): %v", err)
	}
	if bad.VerifyChecksum() {
		t.Fatalf("VerifyChecksum() = true after corrupting a body byte, want false")
	}

	// A packet with no RawBytes (constructed, not decoded) is rejected, not a panic.
	if (Packet{}).VerifyChecksum() {
		t.Fatalf("VerifyChecksum() = true on empty RawBytes, want false")
	}
}

// VALIDATES: AC-1 - common header encode/decode for all packet types.
// PREVENTS: packet body dispatch running on malformed common headers.
// RFC requirement: RFC2328-A.3.1-1 positive -- every OSPF packet type round-trips through the standard 24-octet common header: WriteTo emits Version 2 and DecodeHeader accepts it, returning the body offset CommonHeaderLen (Header.WriteTo header.go:179-193, DecodeHeader header.go:132-175).
func TestOSPFHeaderRoundTrip(t *testing.T) {
	for _, typ := range []PacketType{PacketTypeHello, PacketTypeDBDesc, PacketTypeLSReq, PacketTypeLSUpdate, PacketTypeLSAck} {
		h := sampleHeader(t, typ)
		h.Length = CommonHeaderLen
		h.Checksum = 0x1234
		copy(h.Auth[:], "password")
		buf := make([]byte, CommonHeaderLen)
		n := h.WriteTo(buf, 0)
		if n != CommonHeaderLen {
			t.Fatalf("Header.WriteTo wrote %d", n)
		}
		got, off, err := DecodeHeader(buf)
		if err != nil {
			t.Fatalf("DecodeHeader(%v): %v", typ, err)
		}
		if off != CommonHeaderLen || got != h {
			t.Fatalf("DecodeHeader(%v) = %+v off=%d, want %+v off=%d", typ, got, off, h, CommonHeaderLen)
		}
	}
}

// RFC requirement: RFC2328-A.3.1-1 negative -- a header whose Version octet is not 2 is rejected with ErrBadVersion before any body parser runs, and a Packet Length shorter than the 24-octet header is rejected with ErrLength (DecodeHeader header.go:136-146).
func TestOSPFHeaderRejectsBadVersionAndLength(t *testing.T) {
	h := sampleHeader(t, PacketTypeHello)
	h.Length = CommonHeaderLen
	buf := make([]byte, CommonHeaderLen)
	h.WriteTo(buf, 0)
	buf[0] = 3
	if _, _, err := DecodeHeader(buf); !errors.Is(err, ErrBadVersion) {
		t.Fatalf("bad version err = %v, want %v", err, ErrBadVersion)
	}
	buf[0] = Version
	writeUint16(buf, offLength, CommonHeaderLen-1)
	if _, _, err := DecodeHeader(buf); !errors.Is(err, ErrLength) {
		t.Fatalf("short packet length err = %v, want %v", err, ErrLength)
	}
	writeUint16(buf, offLength, CommonHeaderLen+1)
	if _, _, err := DecodeHeader(buf); !errors.Is(err, ErrTruncated) {
		t.Fatalf("truncated packet err = %v, want %v", err, ErrTruncated)
	}
}

// VALIDATES: AC-4 - AuType 2 field framing is exposed without doing crypto.
// PREVENTS: auth spec receiving a mangled key id / auth length / sequence number.
func TestOSPFHeaderAuTypeField(t *testing.T) {
	h := sampleHeader(t, PacketTypeHello)
	h.AuType = AuTypeCryptographic
	h.Length = CommonHeaderLen
	h.Auth = AuthField{0, 0, 7, 16, 0x01, 0x02, 0x03, 0x04}
	buf := make([]byte, CommonHeaderLen)
	h.WriteTo(buf, 0)
	got, _, err := DecodeHeader(buf)
	if err != nil {
		t.Fatalf("DecodeHeader: %v", err)
	}
	if got.AuType != AuTypeCryptographic || got.Auth[2] != 7 || got.Auth[3] != 16 || readUint32(got.Auth[:], 4) != 0x01020304 {
		t.Fatalf("AuType field decoded wrong: %+v", got)
	}
}

// VALIDATES: AC-1, A-1, R-1 - RFC 6549 Section 2 splits the former 16-bit AuType into an
// 8-bit Instance ID at offset 14 and an 8-bit AuType at offset 15, in both directions.
// PREVENTS: writing the Instance ID to the wrong octet (silent no-adjacency on any peer).
func TestHeaderInstanceIDSplit(t *testing.T) {
	h := sampleHeader(t, PacketTypeHello)
	h.Length = CommonHeaderLen
	h.InstanceID = 0xAB
	h.AuType = AuTypeCryptographic // 0x02
	buf := make([]byte, CommonHeaderLen)
	h.WriteTo(buf, 0)

	// RFC 6549 Section 2 byte layout: offset 14 = Instance ID, offset 15 = AuType.
	if buf[14] != 0xAB {
		t.Fatalf("wire offset 14 (Instance ID) = 0x%02x, want 0xAB", buf[14])
	}
	if buf[15] != 0x02 {
		t.Fatalf("wire offset 15 (AuType) = 0x%02x, want 0x02", buf[15])
	}
	got, _, err := DecodeHeader(buf)
	if err != nil {
		t.Fatalf("DecodeHeader: %v", err)
	}
	if got.InstanceID != 0xAB {
		t.Fatalf("decoded InstanceID = 0x%02x, want 0xAB", got.InstanceID)
	}
	if got.AuType != AuTypeCryptographic {
		t.Fatalf("decoded AuType = %d, want %d", got.AuType, AuTypeCryptographic)
	}
}

// VALIDATES: AC-1 - the Instance ID field spans the full 0..255 range at offset 14.
func TestHeaderInstanceIDBoundary(t *testing.T) {
	for _, id := range []uint8{0, 1, 2, 3, 127, 128, 255} {
		h := sampleHeader(t, PacketTypeHello)
		h.Length = CommonHeaderLen
		h.InstanceID = id
		buf := make([]byte, CommonHeaderLen)
		h.WriteTo(buf, 0)
		if buf[14] != id {
			t.Fatalf("id %d: offset 14 = %d", id, buf[14])
		}
		got, _, err := DecodeHeader(buf)
		if err != nil {
			t.Fatalf("id %d: DecodeHeader: %v", id, err)
		}
		if got.InstanceID != id {
			t.Fatalf("id %d: decoded InstanceID = %d", id, got.InstanceID)
		}
	}
}

// VALIDATES: AC-2, A-2, R-2 - Instance ID 0 round-trips a golden today-shaped header
// byte-for-byte (offset 14 stays 0), so legacy OSPFv2 interop is untouched.
// PREVENTS: the AuType/Instance-ID split silently changing the default-instance wire bytes.
func TestHeaderInstanceZeroUnchanged(t *testing.T) {
	// A golden header as base OSPFv2 (pre-6549) would encode it: AuType 2 in the low
	// octet, high octet (offset 14) zero, an 8-octet auth field.
	golden := []byte{
		Version, byte(PacketTypeHello), 0x00, CommonHeaderLen, // version, type, length
		0x0a, 0x00, 0x00, 0x01, // router id 10.0.0.1
		0x00, 0x00, 0x00, 0x00, // area 0
		0x12, 0x34, // checksum
		0x00, 0x02, // offset 14 = 0 (no instance), offset 15 = AuType 2
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, // auth
	}
	got, _, err := DecodeHeader(golden)
	if err != nil {
		t.Fatalf("DecodeHeader(golden): %v", err)
	}
	if got.InstanceID != 0 {
		t.Fatalf("golden InstanceID = %d, want 0", got.InstanceID)
	}
	if got.AuType != AuTypeCryptographic {
		t.Fatalf("golden AuType = %d, want %d", got.AuType, AuTypeCryptographic)
	}
	// Re-encode the decoded header and confirm the 24 octets are bit-for-bit identical.
	out := make([]byte, CommonHeaderLen)
	got.WriteTo(out, 0)
	for i := range golden {
		if out[i] != golden[i] {
			t.Fatalf("re-encoded octet %d = 0x%02x, want 0x%02x (byte-for-byte compatibility broken)", i, out[i], golden[i])
		}
	}
}

// VALIDATES: AC-12, R-8 - a buffer shorter than the 24-octet common header returns
// ErrShortBuffer without panicking; the Instance-ID split adds no out-of-bounds read.
func TestDecodeHeaderTruncated(t *testing.T) {
	full := make([]byte, CommonHeaderLen)
	sampleHeader(t, PacketTypeHello).WriteTo(full, 0)
	for n := range CommonHeaderLen {
		if _, _, err := DecodeHeader(full[:n]); !errors.Is(err, ErrShortBuffer) {
			t.Fatalf("DecodeHeader(len %d) err = %v, want ErrShortBuffer", n, err)
		}
	}
}
