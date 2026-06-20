package sampling

import (
	"encoding/binary"
	"testing"
)

func TestParsePsampleMessage(t *testing.T) {
	// Build a synthetic netlink attribute payload matching psample format.
	// Netlink attributes: 4-byte header (len u16, type u16) + data, padded to 4 bytes.
	var buf []byte

	// PSAMPLE_ATTR_IIFINDEX (type=1, uint32)
	buf = appendNLA(buf, psampleAttrIIfIndex, uint32ToBytes(42))

	// PSAMPLE_ATTR_ORIGSIZE (type=3, uint32)
	buf = appendNLA(buf, psampleAttrOrigSize, uint32ToBytes(1500))

	// PSAMPLE_ATTR_SAMPLE_RATE (type=6, uint32)
	buf = appendNLA(buf, psampleAttrSampleRate, uint32ToBytes(2048))

	// PSAMPLE_ATTR_DATA (type=7, variable bytes)
	header := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE}
	buf = appendNLA(buf, psampleAttrData, header)

	pkt, err := parsePsampleMessage(buf)
	if err != nil {
		t.Fatalf("parsePsampleMessage: %v", err)
	}

	if pkt.IfIndex != 42 {
		t.Errorf("IfIndex = %d, want 42", pkt.IfIndex)
	}
	if pkt.OrigSize != 1500 {
		t.Errorf("OrigSize = %d, want 1500", pkt.OrigSize)
	}
	if pkt.Rate != 2048 {
		t.Errorf("Rate = %d, want 2048", pkt.Rate)
	}
	if len(pkt.Header) != len(header) {
		t.Fatalf("Header length = %d, want %d", len(pkt.Header), len(header))
	}
	for i := range header {
		if pkt.Header[i] != header[i] {
			t.Errorf("Header[%d] = %02x, want %02x", i, pkt.Header[i], header[i])
		}
	}
}

func TestParsePsampleMessageEmpty(t *testing.T) {
	pkt, err := parsePsampleMessage(nil)
	if err != nil {
		t.Fatalf("parsePsampleMessage(nil): %v", err)
	}
	if pkt.IfIndex != 0 || pkt.Rate != 0 || pkt.OrigSize != 0 || pkt.Header != nil {
		t.Errorf("expected zero SampledPacket, got %+v", pkt)
	}
}

func TestParsePsampleHeaderIsCopy(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04}
	buf := appendNLA(nil, psampleAttrData, data)

	pkt, err := parsePsampleMessage(buf)
	if err != nil {
		t.Fatalf("parsePsampleMessage: %v", err)
	}

	// Mutate original; parsed copy must not change.
	data[0] = 0xFF
	if pkt.Header[0] == 0xFF {
		t.Error("Header shares backing array with source data")
	}
}

// appendNLA appends a netlink attribute (NLA header + data + padding).
func appendNLA(buf []byte, attrType uint16, data []byte) []byte {
	nlaLen := 4 + len(data)
	padLen := (4 - nlaLen%4) % 4

	var hdr [4]byte
	binary.NativeEndian.PutUint16(hdr[0:], uint16(nlaLen))
	binary.NativeEndian.PutUint16(hdr[2:], attrType)

	buf = append(buf, hdr[:]...)
	buf = append(buf, data...)
	for range padLen {
		buf = append(buf, 0)
	}
	return buf
}

func uint32ToBytes(v uint32) []byte {
	var b [4]byte
	binary.NativeEndian.PutUint32(b[:], v)
	return b[:]
}
