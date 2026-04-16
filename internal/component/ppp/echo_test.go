package ppp

import (
	"bytes"
	"errors"
	"testing"
)

// VALIDATES: WriteLCPEcho produces a valid LCP packet with code,
//
//	identifier, and Magic-Number, parseable round-trip.
func TestLCPEchoRoundTrip(t *testing.T) {
	buf := make([]byte, 32)
	const magic uint32 = 0xDEADBEEF
	n := WriteLCPEcho(buf, 0, LCPEchoRequest, 0x42, magic, nil)
	if n != 8 {
		t.Errorf("n = %d, want 8 (4 LCP header + 4 magic)", n)
	}
	pkt, err := ParseLCPPacket(buf[:n])
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if pkt.Code != LCPEchoRequest {
		t.Errorf("Code = %d, want %d", pkt.Code, LCPEchoRequest)
	}
	if pkt.Identifier != 0x42 {
		t.Errorf("Identifier = 0x%02x, want 0x42", pkt.Identifier)
	}
	gotMagic, err := ParseLCPEchoMagic(pkt.Data)
	if err != nil {
		t.Fatalf("magic parse error: %v", err)
	}
	if gotMagic != magic {
		t.Errorf("magic = 0x%08x, want 0x%08x", gotMagic, magic)
	}
}

// VALIDATES: BuildLCPEchoReply echoes the request's Identifier and
//
//	uses the LOCAL Magic-Number (NOT the peer's).
//
// PREVENTS: regression where the reply mirrors the peer's magic --
//
//	that would prevent the peer from detecting a loopback.
func TestLCPBuildEchoReplyEchoesIDLocalMagic(t *testing.T) {
	const localMagic uint32 = 0xCAFEBABE
	const peerMagic uint32 = 0x12345678
	const reqID uint8 = 0x55

	// Pretend we received an Echo-Request with peerMagic and reqID
	// and no extra Data bytes. Build the reply.
	requestData := []byte{0x12, 0x34, 0x56, 0x78} // peerMagic only, no extra bytes
	replyBuf := make([]byte, 16)
	n := BuildLCPEchoReply(replyBuf, 0, reqID, localMagic, requestData)
	pkt, err := ParseLCPPacket(replyBuf[:n])
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pkt.Code != LCPEchoReply {
		t.Errorf("Code = %d, want %d (Echo-Reply)", pkt.Code, LCPEchoReply)
	}
	if pkt.Identifier != reqID {
		t.Errorf("Identifier = 0x%02x, want 0x%02x", pkt.Identifier, reqID)
	}
	gotMagic, _ := ParseLCPEchoMagic(pkt.Data)
	if gotMagic != localMagic {
		t.Errorf("magic = 0x%08x, want LOCAL 0x%08x (not peer 0x%08x)", gotMagic, localMagic, peerMagic)
	}
}

// VALIDATES: ParseLCPEchoMagic rejects payloads shorter than 4 bytes.
func TestLCPEchoMagicTooShort(t *testing.T) {
	for _, n := range []int{0, 1, 2, 3} {
		_, err := ParseLCPEchoMagic(make([]byte, n))
		if !errors.Is(err, errLCPEchoTooShort) {
			t.Errorf("len=%d: err = %v, want errLCPEchoTooShort", n, err)
		}
	}
}

// VALIDATES: ParseLCPEchoMagic ignores trailing payload bytes.
func TestLCPEchoMagicIgnoresTrailing(t *testing.T) {
	payload := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xFF, 0xEE, 0xDD}
	got, err := ParseLCPEchoMagic(payload)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != 0xAABBCCDD {
		t.Errorf("magic = 0x%08x, want 0xAABBCCDD", got)
	}
}

// VALIDATES: IsLCPLoopback returns true when the payload's magic
//
//	matches ours and false otherwise.
func TestIsLCPLoopback(t *testing.T) {
	const ours uint32 = 0xCAFEBABE
	cases := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{"matches local magic", []byte{0xCA, 0xFE, 0xBA, 0xBE}, true},
		{"different magic", []byte{0xDE, 0xAD, 0xBE, 0xEF}, false},
		{"too short", []byte{0xCA, 0xFE}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsLCPLoopback(tc.payload, ours)
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// VALIDATES: BuildLCPEchoReply mirrors any post-Magic-Number Data
//
//	bytes from the request into the reply, per RFC 1661 §5.8.
//
// PREVENTS: regression where peer-supplied Data is silently dropped,
//
//	breaking peers that use the Data field for diagnostics.
func TestLCPEchoReplyMirrorsRequestData(t *testing.T) {
	const localMagic uint32 = 0x11223344
	// Request Data: 4-byte peer magic + 6 extra bytes the peer wants
	// echoed back.
	requestData := []byte{
		0xAA, 0xBB, 0xCC, 0xDD, // peer magic
		'h', 'e', 'l', 'l', 'o', '!', // extra Data field
	}
	replyBuf := make([]byte, 32)
	n := BuildLCPEchoReply(replyBuf, 0, 0x77, localMagic, requestData)
	pkt, err := ParseLCPPacket(replyBuf[:n])
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// First 4 bytes of reply Data: ze's local magic, NOT peer's.
	gotMagic, _ := ParseLCPEchoMagic(pkt.Data)
	if gotMagic != localMagic {
		t.Errorf("magic = 0x%08x, want local 0x%08x", gotMagic, localMagic)
	}
	// Remaining bytes: mirrored verbatim.
	if !bytes.Equal(pkt.Data[4:], []byte("hello!")) {
		t.Errorf("extra Data = %q, want \"hello!\"", pkt.Data[4:])
	}
}

// VALIDATES: BuildLCPEchoReply with no extra Data produces the
//
//	bare 8-byte form.
func TestLCPEchoReplyNoExtraData(t *testing.T) {
	const localMagic uint32 = 0xCAFEBABE
	requestData := []byte{0x12, 0x34, 0x56, 0x78} // peer magic only
	replyBuf := make([]byte, 32)
	n := BuildLCPEchoReply(replyBuf, 0, 1, localMagic, requestData)
	if n != 8 {
		t.Errorf("n = %d, want 8 (no extra data)", n)
	}
}

// VALIDATES: BuildLCPEchoReply with truncated request Data (< 4 bytes,
//
//	violation of RFC) still produces a valid reply with no extra
//	bytes.
func TestLCPEchoReplyShortRequest(t *testing.T) {
	const localMagic uint32 = 0xCAFEBABE
	replyBuf := make([]byte, 32)
	n := BuildLCPEchoReply(replyBuf, 0, 1, localMagic, []byte{0xAA, 0xBB})
	if n != 8 {
		t.Errorf("n = %d, want 8 (short request data, no extra)", n)
	}
}

// VALIDATES: WriteLCPEcho writes consistent bytes for the same magic.
func TestLCPEchoBytesStable(t *testing.T) {
	a := make([]byte, 16)
	b := make([]byte, 16)
	WriteLCPEcho(a, 0, LCPEchoRequest, 1, 0x11223344, nil)
	WriteLCPEcho(b, 0, LCPEchoRequest, 1, 0x11223344, nil)
	if !bytes.Equal(a[:8], b[:8]) {
		t.Errorf("non-deterministic encoding")
	}
}
