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
