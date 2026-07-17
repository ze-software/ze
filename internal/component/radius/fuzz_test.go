package radius

import (
	"testing"
)

// FuzzDecodeRADIUSVSA feeds arbitrary bytes into DecodeVSA, the vendor-specific
// attribute decoder the L2TP RADIUS client (authradius) runs over vendor
// attributes extracted from RADIUS server replies. Input shape is the
// attribute value AFTER the outer Type+Length, matching the real callers
// (they pass data[2:]); seeds use EncodeVSA(...)[2:] for that reason.
//
// The decoder must never panic, and on success the returned value must be
// exactly vendorLen-2 bytes and lie within the input (4+vendorLen <= len),
// so a forged vendor length can never make the value escape the buffer.
//
// VALIDATES: DecodeVSA bounds under adversarial input (AC-2).
// PREVENTS: regression where a crafted vendor length panics or returns a
// value slice that over-reads the attribute.
func FuzzDecodeRADIUSVSA(f *testing.F) {
	for _, seed := range radiusVSASeeds() {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, value, err := DecodeVSA(data)
		if err != nil {
			return
		}
		// len(data) >= 6 is guaranteed once err == nil, so data[5] is safe.
		vendorLen := int(data[5])
		if 4+vendorLen > len(data) {
			t.Fatalf("accepted VSA: 4+vendorLen=%d exceeds data=%d", 4+vendorLen, len(data))
		}
		if len(value) != vendorLen-2 {
			t.Fatalf("accepted VSA: value len %d != vendorLen-2 %d", len(value), vendorLen-2)
		}
	})
}

// radiusVSASeeds returns valid and malformed vendor-specific attribute values
// (post outer Type+Length): a well-formed VSA with a payload, a well-formed
// VSA with an empty payload, zero-length, a too-short buffer, a vendorLen
// below the 2-byte minimum, and a vendorLen overrunning the buffer.
func radiusVSASeeds() [][]byte {
	valid := func(vendorID uint32, vendorType uint8, val []byte) []byte {
		b, err := EncodeVSA(vendorID, vendorType, val)
		if err != nil {
			return nil
		}
		return b[2:] // strip the outer Type(1)+Length(1)
	}

	// valid() never returns nil for these small values (EncodeVSA only errors
	// when 2+len(value) > 255); a nil seed would be a harmless empty input anyway.
	return [][]byte{
		valid(311, 25, []byte("hello")),      // vendor Microsoft, non-empty value
		valid(9, 1, []byte{}),                // vendor Cisco, empty value (vendorLen==2)
		{},                                   // zero-length
		{0x00, 0x00, 0x01},                   // too short (<6)
		{0x00, 0x00, 0x01, 0x2b, 0x01, 0x01}, // vendorLen=1 (<2)
		{0x00, 0x00, 0x01, 0x2b, 0x01, 0xff}, // vendorLen=255 overruns 6-byte buffer
	}
}
