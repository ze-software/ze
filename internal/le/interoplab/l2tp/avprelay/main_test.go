// Design: docs/architecture/testing/interop.md -- transparent L2TP relay input integrity.
// Related: main.go -- inject preserves the real peer's packet except length and appended AVP.
package main

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestInjectOnlySelectedControlMessages checks both polarities, retransmissions,
// malformed headers and an unrelated control message without starting a socket.
func TestInjectOnlySelectedControlMessages(t *testing.T) {
	for _, tc := range []struct {
		name      string
		message   uint16
		armed     bool
		flags     byte
		appendAVP bool
	}{
		{name: "optional SCCCN", message: 3, appendAVP: true},
		{name: "SCCCN retransmission remains optional", message: 3, armed: true, appendAVP: true},
		{name: "unarmed HELLO", message: 6},
		{name: "armed HELLO", message: 6, armed: true, flags: 0x80, appendAVP: true},
		{name: "unrelated ICRQ", message: 10, armed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			packet := relayControl(tc.message)
			original := bytes.Clone(packet)
			got := inject(packet, tc.armed)
			if !tc.appendAVP {
				if !bytes.Equal(got, original) {
					t.Fatalf("unselected packet changed: %x", got)
				}
				return
			}
			if len(got) != len(original)+6 {
				t.Fatalf("length %d, want %d", len(got), len(original)+6)
			}
			if int(binary.BigEndian.Uint16(got[2:4])) != len(got) {
				t.Fatal("declared length does not include appended AVP")
			}
			if !bytes.Equal(got[:2], original[:2]) || !bytes.Equal(got[4:len(original)], original[4:]) {
				t.Fatal("relay changed original flags, sequence, IDs or payload")
			}
			if !bytes.Equal(got[len(original):], []byte{tc.flags, 6, 0, 0, 0xff, 0xfe}) {
				t.Fatalf("wrong appended AVP: %x", got[len(original):])
			}
		})
	}
	for _, malformed := range [][]byte{
		nil, {0xc8, 2}, {0xc8, 2, 0, 20},
	} {
		original := bytes.Clone(malformed)
		if got := inject(malformed, true); !bytes.Equal(got, original) {
			t.Fatalf("malformed input changed: %x", got)
		}
	}
	for _, offset := range []int{0, 2, 12, 14} {
		packet := relayControl(6)
		packet[offset] ^= 1
		original := bytes.Clone(packet)
		if got := inject(packet, true); !bytes.Equal(got, original) {
			t.Fatalf("malformed header at %d changed: %x", offset, got)
		}
	}
}

func relayControl(message uint16) []byte {
	packet := make([]byte, 20, 26)
	for i, value := range []uint16{0xc802, 20, 19, 0, 7, 11, 0x8008, 0, 0, message} {
		binary.BigEndian.PutUint16(packet[i*2:], value)
	}
	return packet
}
