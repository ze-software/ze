// Design: plan/learned/956-ospf-2-wire.md -- common header tests

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

// VALIDATES: AC-1 - common header encode/decode for all packet types.
// PREVENTS: packet body dispatch running on malformed common headers.
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
