package ppp

import (
	"bytes"
	"errors"
	"testing"
)

// VALIDATES: ParseLCPPacket decodes Code, Identifier, Length and
//
//	returns Data as a sub-slice of the input.
//
// PREVENTS: regressions where Data is allocated or Length is treated
//
//	as the data length instead of the total packet length.
func TestLCPPacketParse(t *testing.T) {
	// Configure-Request, id=0x42, length=8, data=[0x01 0x02 0x03 0x04]
	buf := []byte{0x01, 0x42, 0x00, 0x08, 0x01, 0x02, 0x03, 0x04}
	pkt, err := ParseLCPPacket(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkt.Code != LCPConfigureRequest {
		t.Errorf("Code = %d, want %d", pkt.Code, LCPConfigureRequest)
	}
	if pkt.Identifier != 0x42 {
		t.Errorf("Identifier = 0x%02x, want 0x42", pkt.Identifier)
	}
	if !bytes.Equal(pkt.Data, []byte{0x01, 0x02, 0x03, 0x04}) {
		t.Errorf("Data = %x, want 01020304", pkt.Data)
	}
	if &pkt.Data[0] != &buf[lcpHeaderLen] {
		t.Errorf("Data should sub-slice into buf; got fresh allocation")
	}
}

// VALIDATES: ParseLCPPacket honors Length and ignores trailing padding.
// PREVENTS: parser consuming bytes beyond Length, which would corrupt
//
//	the next packet in a batched read.
func TestLCPPacketParseIgnoresPadding(t *testing.T) {
	// Length=4 (header only). Trailing bytes are padding.
	buf := []byte{0x01, 0x01, 0x00, 0x04, 0xFF, 0xFF, 0xFF}
	pkt, err := ParseLCPPacket(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkt.Data) != 0 {
		t.Errorf("Data = %x, want empty (Length=4 means header only)", pkt.Data)
	}
}

// VALIDATES: ParseLCPPacket rejects buffers shorter than the header.
func TestLCPPacketParseTooShort(t *testing.T) {
	for _, n := range []int{0, 1, 2, 3} {
		_, err := ParseLCPPacket(make([]byte, n))
		if !errors.Is(err, errLCPTooShort) {
			t.Errorf("len=%d: err = %v, want errLCPTooShort", n, err)
		}
	}
}

// VALIDATES: ParseLCPPacket rejects Length field below 4, above buf
//
//	length, or above MaxFrameLen-2.
func TestLCPPacketParseInvalidLength(t *testing.T) {
	cases := []struct {
		name string
		buf  []byte
	}{
		{"length 3 (below header)", []byte{0x01, 0x00, 0x00, 0x03, 0xAA}},
		{"length exceeds buffer", []byte{0x01, 0x00, 0x00, 0x10, 0xAA, 0xBB}},
		{"length exceeds frame max", append([]byte{0x01, 0x00, 0xFA, 0x00}, bytes.Repeat([]byte{0xAA}, 64000)...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseLCPPacket(tc.buf)
			if !errors.Is(err, errLCPLengthMismatch) {
				t.Errorf("err = %v, want errLCPLengthMismatch", err)
			}
		})
	}
}

// VALIDATES: WriteLCPPacket backfills the Length field with the total
//
//	packet length and writes Code/Identifier/Data correctly.
//
// PREVENTS: regressions where Length is set to the data length instead
//
//	of total length, or where Length is left zeroed.
func TestLCPPacketWriteTo(t *testing.T) {
	data := []byte{0x05, 0x06, 0x00, 0x00}
	buf := make([]byte, 16)
	n := WriteLCPPacket(buf, 0, LCPConfigureAck, 0x37, data)
	if n != 8 {
		t.Errorf("n = %d, want 8", n)
	}
	want := []byte{0x02, 0x37, 0x00, 0x08, 0x05, 0x06, 0x00, 0x00}
	if !bytes.Equal(buf[:n], want) {
		t.Errorf("buf = %x, want %x", buf[:n], want)
	}
}

// VALIDATES: WriteLCPPacket writes at the requested offset, not 0.
func TestLCPPacketWriteToOffset(t *testing.T) {
	data := []byte{0xAA}
	buf := []byte{0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	n := WriteLCPPacket(buf, 2, LCPEchoReply, 0x01, data)
	if n != 5 {
		t.Errorf("n = %d, want 5", n)
	}
	if buf[0] != 0xFF || buf[1] != 0xFF {
		t.Errorf("prefix overwritten: %x", buf[:2])
	}
	if buf[2] != LCPEchoReply || buf[3] != 0x01 || buf[4] != 0x00 || buf[5] != 0x05 {
		t.Errorf("packet bytes wrong: %x", buf[2:6])
	}
}

// VALIDATES: WriteLCPPacket / ParseLCPPacket round-trip.
func TestLCPPacketRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		code uint8
		id   uint8
		data []byte
	}{
		{"echo-request empty", LCPEchoRequest, 1, nil},
		{"configure-request typical", LCPConfigureRequest, 1, []byte{0x01, 0x04, 0x05, 0xDC, 0x05, 0x06, 0x00, 0x01, 0x02, 0x03}},
		{"large", LCPConfigureRequest, 0, bytes.Repeat([]byte{0x55}, 256)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, MaxFrameLen)
			n := WriteLCPPacket(buf, 0, tc.code, tc.id, tc.data)
			pkt, err := ParseLCPPacket(buf[:n])
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if pkt.Code != tc.code {
				t.Errorf("code mismatch")
			}
			if pkt.Identifier != tc.id {
				t.Errorf("id mismatch")
			}
			if !bytes.Equal(pkt.Data, tc.data) {
				t.Errorf("data mismatch")
			}
		})
	}
}

// VALIDATES: LCPCodeName returns lowercase well-known names and a
//
//	"code-N" fallback for unknown codes.
func TestLCPCodeName(t *testing.T) {
	cases := []struct {
		code uint8
		want string
	}{
		{LCPConfigureRequest, "configure-request"},
		{LCPEchoReply, "echo-reply"},
		{LCPCodeReject, "code-reject"},
		{0, "code-0"},
		{255, "code-255"},
		{99, "code-99"},
	}
	for _, tc := range cases {
		got := LCPCodeName(tc.code)
		if got != tc.want {
			t.Errorf("LCPCodeName(%d) = %q, want %q", tc.code, got, tc.want)
		}
	}
}
