// Design: docs/architecture/testing/interop.md -- observe useful traffic after a peer form change.
// Related: frame.go -- portable reply boundary, independent of packet capture.
package main

import (
	"encoding/binary"
	"testing"
)

// TestParseEchoReplyRequiresTheProbeIdentity accepts the peer's matching reply only.
func TestParseEchoReplyRequiresTheProbeIdentity(t *testing.T) {
	want := command{action: actionProbe, key: fixtureKey(), echoID: 71, echoSequence: 1}
	if err := parseEchoReply(replyFixture(), want); err != nil {
		t.Fatalf("matching echo reply: %v", err)
	}
}

// TestParseEchoReplyRefusesUnrelatedOrTruncatedInput prevents incidental ICMP proving delivery.
func TestParseEchoReplyRefusesUnrelatedOrTruncatedInput(t *testing.T) {
	want := command{action: actionProbe, key: fixtureKey(), echoID: 71, echoSequence: 1}
	for _, test := range []struct {
		name string
		edit func([]byte) []byte
	}{
		{"type", func(packet []byte) []byte { packet[20] = 8; return packet }},
		{"source", func(packet []byte) []byte { packet[15]++; return packet }},
		{"target", func(packet []byte) []byte { packet[19]++; return packet }},
		{"id", func(packet []byte) []byte { packet[25]++; return packet }},
		{"sequence", func(packet []byte) []byte { packet[27]++; return packet }},
		{"payload", func(packet []byte) []byte { packet[28]++; return packet }},
		{"truncated", func(packet []byte) []byte { return packet[:27] }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := parseEchoReply(test.edit(replyFixture()), want); err == nil {
				t.Fatal("input other than the matching probe reply was accepted")
			}
		})
	}
}

func replyFixture() []byte {
	packet := make([]byte, 28+len(echoPayload))
	packet[0], packet[8], packet[9] = 0x45, 64, 1
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	source, target := fixtureKey().target.As4(), fixtureKey().source.As4()
	copy(packet[12:16], source[:])
	copy(packet[16:20], target[:])
	binary.BigEndian.PutUint16(packet[24:26], 71)
	binary.BigEndian.PutUint16(packet[26:28], 1)
	copy(packet[28:], echoPayload)
	return packet
}
