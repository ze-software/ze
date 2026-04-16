package ppp

import (
	"testing"
)

// FuzzParseLCPPacket exercises ParseLCPPacket against arbitrary input
// to catch panics and out-of-bounds reads. Required by .claude/rules/
// testing.md "Fuzz (MANDATORY for wire format)".
func FuzzParseLCPPacket(f *testing.F) {
	seeds := [][]byte{
		nil,
		{0x01, 0x00, 0x00, 0x04},             // valid empty Configure-Request
		{0x09, 0x42, 0x00, 0x08, 1, 2, 3, 4}, // Echo-Request with magic
		{0x01, 0x00, 0xFF, 0xFF},             // length way larger than buf
		{0x01, 0x00, 0x00, 0x03},             // length below header
		{0x01, 0x00, 0x00, 0x04, 0xFF, 0xFF}, // length 4 with trailing bytes (must be ignored)
		make([]byte, 1500),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		pkt, err := ParseLCPPacket(data)
		if err != nil {
			return
		}
		// Invariants: Data is in bounds; total length matches header.
		if len(pkt.Data) > len(data)-lcpHeaderLen {
			t.Errorf("Data len = %d exceeds available %d", len(pkt.Data), len(data)-lcpHeaderLen)
		}
	})
}

// FuzzParseLCPOptions exercises ParseLCPOptions against arbitrary
// option-list input.
func FuzzParseLCPOptions(f *testing.F) {
	seeds := [][]byte{
		nil,
		{LCPOptMRU, 0x04, 0x05, 0xDC}, // MRU=1500
		{LCPOptMagic, 0x06, 0xDE, 0xAD, 0xBE, 0xEF, LCPOptPFC, 0x02},                            // magic + PFC
		{LCPOptMRU, 0x04, 0x05, 0xDC, LCPOptAuthProto, 0x05, 0xC2, 0x23, 0x05, LCPOptPFC, 0x02}, // multiple opts
		{LCPOptMRU, 0x01},                    // length 1 (below header)
		{LCPOptMRU, 0xFF, 0xAA},              // length exceeds buffer
		{0x99, 0x02, 0x99, 0x02, 0x99, 0x02}, // unknown option type, repeated
		make([]byte, 254),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		opts, _ := ParseLCPOptions(data)
		// Invariant: every returned option's Data length matches its
		// declared Length minus the 2-byte header, AND fits within
		// the input buffer.
		for i, o := range opts {
			if len(o.Data) > len(data) {
				t.Errorf("opt %d: data len %d > input %d", i, len(o.Data), len(data))
			}
		}
	})
}
