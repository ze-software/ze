// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP receive boundaries.
package rsvpte

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/netip"
	"testing"
)

// TestDecodeMessageChecksum verifies transmitted checksums, including a corrupt
// object body, and accepts the RFC 2205 zero-checksum form.
func TestDecodeMessageChecksum(t *testing.T) {
	raw := buildPath(samplePSB(), netip.MustParseAddr("10.0.0.1"), 64)
	msg, err := DecodeMessage(raw)
	if err != nil || msg.Session.TunnelID != 42 {
		t.Fatalf("valid message = %+v, error = %v", msg, err)
	}
	raw[4] ^= 1
	if _, err := DecodeMessage(raw); !errors.Is(err, errBadChecksum) {
		t.Fatalf("corrupt message error = %v", err)
	}
	clear(raw[2:4])
	msg, err = DecodeMessage(raw)
	if err != nil || msg.Session.TunnelID != 42 {
		t.Fatalf("zero-checksum message = %+v, error = %v", msg, err)
	}
}

// TestDecodeMessageLengths bounds object reads to the declared RSVP message,
// enforces four-byte object alignment, and leaves unrelated trailing bytes out
// of checksum verification.
func TestDecodeMessageLengths(t *testing.T) {
	hop := netip.MustParseAddr("10.0.0.1")
	raw := buildPath(samplePSB(), hop, 64)
	padded := append(bytes.Clone(raw), 0xde, 0xad, 0xbe, 0xef)
	if _, err := DecodeMessage(padded); err != nil {
		t.Fatalf("bytes after declared length affected checksum: %v", err)
	}
	for _, length := range []uint16{0, 4, 7} {
		malformed := bytes.Clone(raw)
		clear(malformed[2:4])
		binary.BigEndian.PutUint16(malformed[6:8], length)
		if _, err := DecodeMessage(malformed); err == nil {
			t.Fatalf("header length %d accepted", length)
		}
	}
	unaligned := pathWithObject(samplePSB(), hop, ClassNull, 1, []byte{1})
	if _, err := DecodeMessage(unaligned); !errors.Is(err, errBadObjLen) {
		t.Fatalf("unaligned object error = %v", err)
	}
	aligned := pathWithObject(samplePSB(), hop, ClassNull, 1, []byte{1, 2, 3, 4})
	if _, err := DecodeMessage(aligned); err != nil {
		t.Fatalf("aligned padding refused: %v", err)
	}
}

// TestForwardOpaqueObjects verifies that a transit builder retains an unknown
// class with 11 high bits, but omits a class with 10 high bits.
func TestForwardOpaqueObjects(t *testing.T) {
	hop := netip.MustParseAddr("10.0.0.1")
	psb := samplePSB()
	raw := pathWithObject(psb, hop, 0xc8, 7, []byte{1, 2, 3, 4})
	msg, err := DecodeMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.ForwardObjects) != 1 {
		t.Fatalf("forwarded objects = %d", len(msg.ForwardObjects))
	}
	psb.ForwardObjects = msg.ForwardObjects
	forwarded, err := DecodeMessage(buildPath(psb, hop, 63))
	if err != nil {
		t.Fatal(err)
	}
	if len(forwarded.ForwardObjects) != 1 || !bytes.Equal(forwarded.ForwardObjects[0], msg.ForwardObjects[0]) {
		t.Fatalf("opaque object not preserved: %x", forwarded.ForwardObjects)
	}
	psb.ForwardObjects = nil
	ignored, err := DecodeMessage(pathWithObject(psb, hop, 0xa0, 1, []byte{1, 2, 3, 4}))
	if err != nil || len(ignored.ForwardObjects) != 0 {
		t.Fatalf("10bbbbbb class retained: %+v, error = %v", ignored, err)
	}
}

// TestIntservMalformedLengths exercises all three nested length boundaries and
// rejects a token-bucket layout that pretends to carry a Null Service TSpec.
func TestIntservMalformedLengths(t *testing.T) {
	cases := []struct {
		name   string
		change func([]byte)
	}{
		{"message", func(b []byte) { b[3]-- }},
		{"service", func(b []byte) { b[7]++ }},
		{"parameter", func(b []byte) { b[11]-- }},
		{"null-layout", func(b []byte) { b[4] = serviceNull }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b [36]byte
			encodeFlowSpec(b[:], ClassSenderTSpec, samplePSB().SenderTSpec)
			tc.change(b[4:])
			if _, err := decodeFlowSpec(b[4:]); err == nil {
				t.Fatal("malformed IntServ object accepted")
			}
		})
	}
}

// TestRSVPEncodingCapacity rejects an object that cannot fit the native carrier
// without a partial copy or a wrapped sixteen-bit RSVP length.
func TestRSVPEncodingCapacity(t *testing.T) {
	raw := make([]byte, maxRSVPPacket)
	encodeObjectHeader(raw, objectHeader{Length: maxRSVPPacket, ClassNum: 0xc8, CType: 1})
	psb := samplePSB()
	psb.ForwardObjects = [][]byte{raw}
	if got := buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64); got != nil {
		t.Fatalf("oversized PATH encoded %d bytes", len(got))
	}
	psb.ForwardObjects = nil
	hop := netip.MustParseAddr("10.0.0.1")
	base := buildPath(psb, hop, 64)
	fitting := raw[:maxRSVPPacket-len(base)]
	encodeObjectHeader(fitting, objectHeader{Length: uint16(len(fitting)), ClassNum: 0xc8, CType: 1})
	psb.ForwardObjects = [][]byte{fitting}
	atLimit := buildPath(psb, hop, 64)
	if len(atLimit) != maxRSVPPacket {
		t.Fatalf("maximum valid PATH length = %d, want %d", len(atLimit), maxRSVPPacket)
	}
	if _, err := DecodeMessage(atLimit); err != nil {
		t.Fatalf("maximum valid PATH refused: %v", err)
	}
	var short [4]byte
	if n := encodeAdspec(short[:], 1500, serviceControlledLoad); n != -1 {
		t.Fatalf("short ADSPEC buffer returned %d", n)
	}
}

// FuzzRSVPMessageBounds exercises the shared receive boundary, including nested
// IntServ lengths. The decoder must never panic on peer-controlled bytes.
func FuzzRSVPMessageBounds(f *testing.F) {
	psb := samplePSB()
	psb.Adspec = make([]byte, adspecSize)
	encodeAdspec(psb.Adspec, 1500, serviceControlledLoad)
	raw := buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64)
	f.Add(raw)
	withoutChecksum := bytes.Clone(raw)
	clear(withoutChecksum[2:4])
	f.Add(withoutChecksum)
	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := DecodeMessage(data)
		if err != nil {
			return
		}
	})
}
