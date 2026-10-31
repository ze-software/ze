// Design: docs/architecture/testing/interop.md -- Code-Reject PPP frame injection.
// Related: pppinject.go -- the header reader and the probe builder under test.
package main

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestDataHeaderLength reads each optional L2TP data header field and refuses
// every header the relay cannot inject behind without changing the session.
func TestDataHeaderLength(t *testing.T) {
	ppp := []byte{0xff, 0x03, 0x00, 0x21}
	for _, tc := range []struct {
		name   string
		header []byte
		want   int
		refuse bool
	}{
		{name: "bare", header: []byte{0x00, 0x02, 0, 7, 0, 9}, want: 6},
		{name: "length", header: []byte{0x40, 0x02, 0, 0, 0, 7, 0, 9}, want: 8},
		{name: "offset", header: []byte{0x02, 0x02, 0, 7, 0, 9, 0, 2, 0, 0}, want: 10},
		{name: "control", header: []byte{0xc8, 0x02, 0, 7, 0, 9}, refuse: true},
		{name: "version 3", header: []byte{0x00, 0x03, 0, 7, 0, 9}, refuse: true},
		{name: "sequenced", header: []byte{0x08, 0x02, 0, 7, 0, 9, 0, 1, 0, 1}, refuse: true},
		{name: "short", header: []byte{0x00, 0x02, 0, 7}, refuse: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			packet := append(append([]byte(nil), tc.header...), ppp...)
			got, err := dataHeaderLength(packet)
			if tc.refuse {
				if err == nil {
					t.Fatalf("accepted %x at %d", packet, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("header length %d, want %d", got, tc.want)
			}
		})
	}
	for _, frame := range [][]byte{{0xfe, 0x03, 0, 0x21}, {0xff, 0x02, 0, 0x21}, {0xff, 0x03, 0}} {
		packet := append([]byte{0x00, 0x02, 0, 7, 0, 9}, frame...)
		if _, err := dataHeaderLength(packet); err == nil {
			t.Fatalf("accepted a PPP frame without Address and Control: %x", packet)
		}
	}
}

// TestCodeRejectFramesKeepTheSession builds the probes behind a trigger with a
// Length field and checks the tunnel and session IDs survive, each Length is
// the frame's own, and each probe is the packet the checker expects.
func TestCodeRejectFramesKeepTheSession(t *testing.T) {
	trigger := []byte{0x40, 0x02, 0, 12, 0x12, 0x34, 0x56, 0x78, 0xff, 0x03, 0x00, 0x21}
	frames, err := codeRejectFrames(trigger)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != len(codeRejectProbes) {
		t.Fatalf("%d frames, want %d", len(frames), len(codeRejectProbes))
	}
	for index, frame := range frames {
		if !bytes.Equal(frame[4:8], trigger[4:8]) {
			t.Fatalf("frame %d changed the tunnel or session: %x", index, frame[:8])
		}
		if int(binary.BigEndian.Uint16(frame[2:])) != len(frame) {
			t.Fatalf("frame %d Length %d, want %d", index, binary.BigEndian.Uint16(frame[2:]), len(frame))
		}
		packet := frame[12:]
		want := codeRejectProbes[index]
		if binary.BigEndian.Uint16(frame[10:]) != want.protocol {
			t.Fatalf("frame %d protocol %04x, want %04x", index, binary.BigEndian.Uint16(frame[10:]), want.protocol)
		}
		if packet[0] != want.code {
			t.Fatalf("frame %d code %d, want %d", index, packet[0], want.code)
		}
		if packet[1] != probeIdentifier {
			t.Fatalf("frame %d Identifier %d", index, packet[1])
		}
		if int(binary.BigEndian.Uint16(packet[2:])) != 4+probeDataOctets {
			t.Fatalf("frame %d packet Length %d", index, binary.BigEndian.Uint16(packet[2:]))
		}
		if len(packet) != 4+probeDataOctets {
			t.Fatalf("frame %d carries %d packet octets", index, len(packet))
		}
	}
	if _, err := codeRejectFrames([]byte{0x08, 0x02, 0, 7, 0, 9, 0, 1, 0, 1, 0xff, 0x03, 0, 0x21}); err == nil {
		t.Fatal("built probes behind a sequenced data message")
	}
}

// TestTerminateFrameRidesTheSession builds the IPCP Terminate-Request behind a
// data header with a Length field and checks it keeps the tunnel and session
// IDs, carries its own Length, and is IPCP Code 5 with terminateIdentifier and
// no Data.
func TestTerminateFrameRidesTheSession(t *testing.T) {
	trigger := []byte{0x40, 0x02, 0, 12, 0x12, 0x34, 0x56, 0x78, 0xff, 0x03, 0x00, 0x21}
	start, err := dataHeaderLength(trigger)
	if err != nil {
		t.Fatal(err)
	}
	frame := pppFrameBehind(trigger[:start], protocolIPCP, codeTerminateRequest, terminateIdentifier, nil)
	want := []byte{0x40, 0x02, 0, 16, 0x12, 0x34, 0x56, 0x78, 0xff, 0x03, 0x80, 0x21, 5, terminateIdentifier, 0, 4}
	if !bytes.Equal(frame, want) {
		t.Fatalf("frame % x, want % x", frame, want)
	}
}
