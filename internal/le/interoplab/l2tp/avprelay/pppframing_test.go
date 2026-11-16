// Design: docs/labs/l2tp-interop.md -- independent framing capture and malformed input.
// Related: pppframing.go -- ordinal producer and compressed Echo constructor.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

// TestPPPCaptureStartsAtOne checks that capture precedes every injection
// marker, preserves datagrams, and counts both directions consecutively.
// The explicit flag preserves the exact marker sample supplied by relayData.
func TestPPPCaptureStartsAtOne(t *testing.T) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	var capture framingCapture
	packet := []byte{0, 2, 0, 7, 0, 9, 0xff, 3, 0, 0x21}
	if err := capture.observe(encoder, packet, directionLAC, false); err != nil {
		t.Fatal(err)
	}
	if err := capture.observe(encoder, packet, "ze", true); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	for want := 1; want <= 2; want++ {
		var event normalDataObservation
		if err := decoder.Decode(&event); err != nil {
			t.Fatal(err)
		}
		if event.DataOrdinal != want {
			t.Fatalf("ordinal %d, want %d", event.DataOrdinal, want)
		}
		if event.CodeRejectArmed != (want == 2) {
			t.Fatal("marker phase sample changed")
		}
		if event.Original != event.Forwarded {
			t.Fatal("capture changed forwarded bytes")
		}
	}
}

// TestCompressedEchoFrame keeps a live header and its Length while omitting
// only PPP Address and Control and carrying the requested peer Magic-Number.
func TestCompressedEchoFrame(t *testing.T) {
	header := []byte{0x40, 2, 0, 0, 0x12, 0x34, 0x56, 0x78}
	frame := compressedEchoFrame(header, 0x11223344)
	if !bytes.Equal(frame[4:8], header[4:8]) {
		t.Fatal("compressed injection changed tunnel/session")
	}
	if int(binary.BigEndian.Uint16(frame[2:])) != len(frame) {
		t.Fatal("incorrect L2TP Length")
	}
	if binary.BigEndian.Uint16(frame[8:]) != protocolLCP {
		t.Fatal("compressed injection does not start at PPP Protocol")
	}
	if frame[10] != 9 || frame[11] != compressedIdentifier {
		t.Fatal("incorrect Echo Code/Identifier")
	}
	if binary.BigEndian.Uint32(frame[14:]) != 0x11223344 {
		t.Fatal("incorrect peer Magic-Number")
	}
	if string(frame[18:]) != compressedData {
		t.Fatal("incorrect correlated Echo Data")
	}
}
