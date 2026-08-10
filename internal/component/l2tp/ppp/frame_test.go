package ppp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// VALIDATES: ParseFrame extracts a two-byte protocol field and returns
//
//	the payload sub-slice without copying.
//
// PREVENTS: regression where the parser allocates or mis-aligns the
//
//	protocol field.
func TestPPPFrameParseTwoByteProtocol(t *testing.T) {
	// Frame: [0xC0 0x21] [0x01 0x02 0x03] -- LCP with 3-byte payload.
	buf := []byte{0xC0, 0x21, 0x01, 0x02, 0x03}
	proto, payload, hlen, err := ParseFrame(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proto != ProtoLCP {
		t.Errorf("proto = 0x%04x, want 0x%04x", proto, ProtoLCP)
	}
	if hlen != 2 {
		t.Errorf("hlen = %d, want 2", hlen)
	}
	if !bytes.Equal(payload, buf[2:]) {
		t.Errorf("payload not a sub-slice of buf")
	}
	if &payload[0] != &buf[2] {
		t.Errorf("payload allocated; expected sub-slice")
	}
}

// VALIDATES: ParseFrame rejects one-byte (PFC-compressed) frames.
//
// PREVENTS: regression where PFC parsing is re-introduced and silently
//
//	accepts protocols ze cannot meaningfully handle (control plane is
//	never PFC-eligible; data plane never reaches userspace).
func TestPPPFrameRejectsOneByteForm(t *testing.T) {
	// Single byte 0x21 -- previously parsed as PFC IPv4. Now rejected.
	buf := []byte{0x21}
	_, _, _, err := ParseFrame(buf)
	if !errors.Is(err, errFrameTooShort) {
		t.Errorf("err = %v, want errFrameTooShort", err)
	}
}

// VALIDATES: ParseFrame rejects empty and one-byte buffers.
func TestPPPFrameParseTooShort(t *testing.T) {
	for _, n := range []int{0, 1} {
		_, _, _, err := ParseFrame(make([]byte, n))
		if !errors.Is(err, errFrameTooShort) {
			t.Errorf("len=%d: err = %v, want errFrameTooShort", n, err)
		}
	}
}

// VALIDATES: ParseFrame rejects buffers above MaxFrameLen.
func TestPPPFrameParseTooLong(t *testing.T) {
	buf := make([]byte, MaxFrameLen+1)
	_, _, _, err := ParseFrame(buf)
	if !errors.Is(err, errFrameTooLong) {
		t.Errorf("err = %v, want errFrameTooLong", err)
	}
}

// VALIDATES: WriteFrame produces a two-byte protocol field followed by
//
//	the payload bytes, without allocating.
//
// PREVENTS: regressions where WriteFrame uses append or returns a
//
//	freshly allocated buffer.
func TestPPPFrameWriteTo(t *testing.T) {
	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	buf := make([]byte, 16)
	n := WriteFrame(buf, 0, ProtoLCP, payload)
	if n != 6 {
		t.Errorf("n = %d, want 6", n)
	}
	want := []byte{0xC0, 0x21, 0xDE, 0xAD, 0xBE, 0xEF}
	if !bytes.Equal(buf[:n], want) {
		t.Errorf("buf = %x, want %x", buf[:n], want)
	}
}

// VALIDATES: WriteFrame writes at the requested offset without
//
//	disturbing prior bytes.
func TestPPPFrameWriteToOffset(t *testing.T) {
	payload := []byte{0x01}
	buf := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	n := WriteFrame(buf, 2, ProtoLCP, payload)
	if n != 3 {
		t.Errorf("n = %d, want 3", n)
	}
	// First two bytes preserved.
	if buf[0] != 0xFF || buf[1] != 0xFF {
		t.Errorf("prefix overwritten: %x", buf[:2])
	}
	// Frame at offset 2.
	if buf[2] != 0xC0 || buf[3] != 0x21 || buf[4] != 0x01 {
		t.Errorf("frame bytes wrong: %x", buf[2:5])
	}
}

// VALIDATES: WriteFrame is round-trip with ParseFrame.
func TestPPPFrameRoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		proto   uint16
		payload []byte
	}{
		{"lcp empty", ProtoLCP, nil},
		{"lcp one byte", ProtoLCP, []byte{0xAA}},
		{"chap large", ProtoCHAP, bytes.Repeat([]byte{0x55}, 256)},
		{"ipcp typical", ProtoIPCP, []byte{0x01, 0x02, 0x00, 0x0A}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, MaxFrameLen)
			n := WriteFrame(buf, 0, tc.proto, tc.payload)
			gotProto, gotPayload, hlen, err := ParseFrame(buf[:n])
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if hlen != 2 {
				t.Errorf("hlen = %d, want 2", hlen)
			}
			if gotProto != tc.proto {
				t.Errorf("proto = 0x%04x, want 0x%04x", gotProto, tc.proto)
			}
			if !bytes.Equal(gotPayload, tc.payload) {
				t.Errorf("payload mismatch")
			}
		})
	}
}

// RFC requirement: RFC1332-2-1 positive -- exactly one IPCP packet is encapsulated per
// PPP frame with the Protocol field 0x8021: WriteFrame + WriteLCPPacket
// (internal/component/l2tp/ppp/frame.go, lcp.go) build one IPCP Configure-Request; the
// two-byte Protocol field is 0x8021 (ProtoIPCP), and the LCP Length field spans the whole
// Information field, so the frame carries a single IPCP packet with no trailing second one.
func TestIPCPFrameCarriesExactlyOnePacket(t *testing.T) {
	// One IPCP Configure-Request carrying an IP-Address option (type 3, 10.0.0.1).
	data := []byte{IPCPOptIPAddress, 6, 10, 0, 0, 1}
	buf := make([]byte, MaxFrameLen)
	off := WriteFrame(buf, 0, ProtoIPCP, nil)
	off += WriteLCPPacket(buf, off, LCPConfigureRequest, 0x01, data)
	frame := buf[:off]

	// Protocol field is hex 8021, both as the two raw octets and via ParseFrame.
	if frame[0] != 0x80 || frame[1] != 0x21 {
		t.Fatalf("protocol octets = %02x %02x, want 80 21", frame[0], frame[1])
	}
	proto, payload, _, err := ParseFrame(frame)
	if err != nil {
		t.Fatalf("ParseFrame: %v", err)
	}
	if proto != ProtoIPCP {
		t.Fatalf("proto = 0x%04x, want 0x%04x (ProtoIPCP)", proto, ProtoIPCP)
	}

	// Exactly one IPCP packet: the LCP Length field equals the Information field
	// length, so no bytes remain for a second packet.
	pkt, err := ParseLCPPacket(payload)
	if err != nil {
		t.Fatalf("ParseLCPPacket: %v", err)
	}
	if pkt.Code != LCPConfigureRequest {
		t.Errorf("code = %d, want Configure-Request", pkt.Code)
	}
	length := int(binary.BigEndian.Uint16(payload[2:4]))
	if length != len(payload) {
		t.Errorf("LCP Length = %d, Information field = %d; frame is not exactly one IPCP packet", length, len(payload))
	}
	if lcpHeaderLen+len(pkt.Data) != len(payload) {
		t.Errorf("parsed packet spans %d bytes but Information field is %d; trailing bytes present", lcpHeaderLen+len(pkt.Data), len(payload))
	}
}

// RFC requirement: RFC1332-2-1 negative -- a Protocol-0x8021 frame whose IPCP Length field
// does not delimit exactly one packet is rejected: ParseLCPPacket
// (internal/component/l2tp/ppp/lcp.go) returns errLCPLengthMismatch when the Length is
// below the 4-byte header or exceeds the Information field, so a frame that is not exactly
// one well-formed IPCP packet cannot be processed (handleFrame drops it,
// internal/component/l2tp/ppp/session_run.go).
func TestIPCPFrameRejectsNonSinglePacket(t *testing.T) {
	cases := []struct {
		name string
		body []byte
	}{
		{"length below header", []byte{LCPConfigureRequest, 0x01, 0x00, 0x03, 0xAA}},
		{"length exceeds information field", []byte{LCPConfigureRequest, 0x01, 0x00, 0x20, 0xAA, 0xBB}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, MaxFrameLen)
			off := WriteFrame(buf, 0, ProtoIPCP, tc.body)
			proto, payload, _, err := ParseFrame(buf[:off])
			if err != nil {
				t.Fatalf("ParseFrame: %v", err)
			}
			if proto != ProtoIPCP {
				t.Fatalf("proto = 0x%04x, want 0x%04x (ProtoIPCP)", proto, ProtoIPCP)
			}
			if _, err := ParseLCPPacket(payload); !errors.Is(err, errLCPLengthMismatch) {
				t.Errorf("err = %v, want errLCPLengthMismatch", err)
			}
		})
	}
}

// VALIDATES: FrameLen reports the correct wire size for two-byte form.
func TestFrameLen(t *testing.T) {
	cases := []struct {
		payloadLen int
		want       int
	}{
		{0, 2},
		{1, 3},
		{1498, 1500},
	}
	for _, tc := range cases {
		got := frameLen(tc.payloadLen)
		if got != tc.want {
			t.Errorf("FrameLen(%d) = %d, want %d", tc.payloadLen, got, tc.want)
		}
	}
}
