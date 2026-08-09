// Design: docs/architecture/isis/isis-3-l2-transport.md -- 802.3 + LLC frame codec tests

package transport

import (
	"bytes"
	"testing"
)

func samplePDU() []byte {
	// A 10-byte stand-in for an IS-IS PDU. The transport treats the PDU as
	// opaque bytes; it never parses or pads it.
	return []byte{0x83, 0x1b, 0x01, 0x00, 0x11, 0x01, 0x00, 0x00, 0x05, 0xd9}
}

func TestBuildFrame(t *testing.T) {
	// VALIDATES: AC-2 frame = dst(6)+src(6)+802.3 len(2)+LLC(0xFE 0xFE 0x03)+PDU.
	// PREVENTS: emitting an Ethernet-II ethertype frame FRR cannot parse (R-1).
	src := [MACLen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}
	dst := AllL2ISs
	pdu := samplePDU()

	buf := make([]byte, FrameHeaderLen+len(pdu))
	n, err := BuildFrame(buf, dst, src, pdu)
	if err != nil {
		t.Fatalf("BuildFrame: %v", err)
	}
	if n != FrameHeaderLen+len(pdu) {
		t.Fatalf("n = %d, want %d", n, FrameHeaderLen+len(pdu))
	}
	frame := buf[:n]

	if !bytes.Equal(frame[0:6], dst[:]) {
		t.Errorf("dst MAC = %x, want %x", frame[0:6], dst)
	}
	if !bytes.Equal(frame[6:12], src[:]) {
		t.Errorf("src MAC = %x, want %x", frame[6:12], src)
	}
	// 802.3 length field = LLC(3) + PDU.
	wantLen := uint16(LLCHeaderLen + len(pdu))
	gotLen := uint16(frame[12])<<8 | uint16(frame[13])
	if gotLen != wantLen {
		t.Errorf("802.3 length = %d, want %d", gotLen, wantLen)
	}
	// The classic-error guard: the length field MUST be below the ethertype
	// threshold so receivers parse it as 802.3, not Ethernet II.
	if gotLen >= EthertypeThreshold {
		t.Errorf("802.3 length %d >= ethertype threshold %d (would parse as Ethernet II)", gotLen, EthertypeThreshold)
	}
	// LLC header: DSAP 0xFE, SSAP 0xFE, control 0x03.
	if frame[14] != LLCSAP || frame[15] != LLCSAP || frame[16] != LLCControl {
		t.Errorf("LLC = %x %x %x, want 0xFE 0xFE 0x03", frame[14], frame[15], frame[16])
	}
	// PDU follows the LLC header byte-for-byte.
	if !bytes.Equal(frame[17:], pdu) {
		t.Errorf("PDU = %x, want %x", frame[17:], pdu)
	}
}

func TestBuildFrameBufferTooShort(t *testing.T) {
	// VALIDATES: building into an undersized buffer errors, never panics.
	pdu := samplePDU()
	buf := make([]byte, FrameHeaderLen) // no room for the PDU
	if _, err := BuildFrame(buf, AllL1ISs, [MACLen]byte{}, pdu); err == nil {
		t.Fatal("expected error for short buffer, got nil")
	}
}

func TestBuildFramePDUTooLarge(t *testing.T) {
	// VALIDATES: AC-2 boundary -- a PDU whose LLC+PDU length would reach the
	// ethertype threshold is rejected (it could not be expressed as an 802.3
	// length without colliding with Ethernet II).
	pdu := make([]byte, EthertypeThreshold) // LLC(3)+this >= 0x0600
	buf := make([]byte, FrameHeaderLen+len(pdu))
	if _, err := BuildFrame(buf, AllL1ISs, [MACLen]byte{}, pdu); err == nil {
		t.Fatal("expected error for oversized PDU, got nil")
	}
}

func TestParseFrame(t *testing.T) {
	// VALIDATES: AC-8 strip 802.3+LLC, return a zero-copy PDU view.
	src := [MACLen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x02}
	pdu := samplePDU()
	buf := make([]byte, FrameHeaderLen+len(pdu))
	n, err := BuildFrame(buf, AllL1ISs, src, pdu)
	if err != nil {
		t.Fatalf("BuildFrame: %v", err)
	}

	got, err := ParseFrame(buf[:n])
	if err != nil {
		t.Fatalf("ParseFrame: %v", err)
	}
	if !bytes.Equal(got.PDU, pdu) {
		t.Errorf("PDU = %x, want %x", got.PDU, pdu)
	}
	if got.DstMAC != AllL1ISs {
		t.Errorf("dst = %x, want %x", got.DstMAC, AllL1ISs)
	}
	if got.SrcMAC != src {
		t.Errorf("src = %x, want %x", got.SrcMAC, src)
	}
	// Zero-copy: the PDU view must alias the input buffer, not be a copy.
	if len(got.PDU) > 0 && &got.PDU[0] != &buf[FrameHeaderLen] {
		t.Error("ParseFrame PDU is not a zero-copy view of the input buffer")
	}
}

func TestParseFrameRejectEthertype(t *testing.T) {
	// VALIDATES: AC-8 / R-1 a frame whose 2-byte field is an ethertype
	// (>= 0x0600) is rejected, NOT parsed as if it were an 802.3 length.
	frame := make([]byte, FrameHeaderLen+4)
	copy(frame[0:6], AllL1ISs[:])
	// length field = 0x0800 (IPv4 ethertype): must be rejected.
	frame[12] = 0x08
	frame[13] = 0x00
	frame[14] = LLCSAP
	frame[15] = LLCSAP
	frame[16] = LLCControl
	if _, err := ParseFrame(frame); err == nil {
		t.Fatal("expected ethertype frame to be rejected, got nil error")
	}
}

func TestParseFrameRejectBadSAP(t *testing.T) {
	// VALIDATES: AC-8 LLC DSAP/SSAP other than 0xFE is rejected.
	pdu := samplePDU()
	buf := make([]byte, FrameHeaderLen+len(pdu))
	n, _ := BuildFrame(buf, AllL1ISs, [MACLen]byte{}, pdu)
	cases := []struct {
		name string
		idx  int
		val  byte
	}{
		{"dsap", 14, 0xAA},
		{"ssap", 15, 0xAA},
		{"control", 16, 0x00},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bad := append([]byte(nil), buf[:n]...)
			bad[tc.idx] = tc.val
			if _, err := ParseFrame(bad); err == nil {
				t.Fatalf("expected rejection for bad %s, got nil", tc.name)
			}
		})
	}
}

func TestParseFrameRejectShort(t *testing.T) {
	// VALIDATES: AC-8 boundary -- a frame shorter than the minimum header is
	// rejected before any slice into the PDU (crafted-frame safety, R-1).
	for _, l := range []int{0, 1, FrameHeaderLen - 1} {
		if _, err := ParseFrame(make([]byte, l)); err == nil {
			t.Errorf("ParseFrame(len=%d) = nil error, want rejection", l)
		}
	}
}

func TestParseFrameRejectLengthOverrun(t *testing.T) {
	// VALIDATES: AC-8 a declared 802.3 length longer than the captured bytes is
	// rejected (must not over-read the buffer; crafted-frame safety).
	pdu := samplePDU()
	buf := make([]byte, FrameHeaderLen+len(pdu))
	n, _ := BuildFrame(buf, AllL1ISs, [MACLen]byte{}, pdu)
	bad := append([]byte(nil), buf[:n]...)
	// Inflate the declared length well past the real LLC+PDU bytes present.
	over := uint16(LLCHeaderLen+len(pdu)) + 100
	bad[12] = byte(over >> 8)
	bad[13] = byte(over)
	if _, err := ParseFrame(bad); err == nil {
		t.Fatal("expected rejection for length overrun, got nil")
	}
}

func TestParseFrameRejectLengthTooSmall(t *testing.T) {
	// VALIDATES: boundary -- an 802.3 length below LLC(3) leaves no room for a
	// valid LLC header and is rejected.
	frame := make([]byte, FrameHeaderLen)
	copy(frame[0:6], AllL1ISs[:])
	frame[12] = 0x00
	frame[13] = 0x02 // < LLCHeaderLen (3)
	frame[14] = LLCSAP
	frame[15] = LLCSAP
	frame[16] = LLCControl
	if _, err := ParseFrame(frame); err == nil {
		t.Fatal("expected rejection for length < LLC header, got nil")
	}
}

func TestSendDoesNotAlterPDU(t *testing.T) {
	// VALIDATES: AC-3 the transport adds only 802.3+LLC framing and never pads
	// or alters the PDU. Build then parse must round-trip the PDU byte-for-byte.
	pdu := samplePDU()
	original := append([]byte(nil), pdu...)
	buf := make([]byte, FrameHeaderLen+len(pdu))
	n, err := BuildFrame(buf, AllL2ISs, [MACLen]byte{0x02}, pdu)
	if err != nil {
		t.Fatalf("BuildFrame: %v", err)
	}
	if !bytes.Equal(pdu, original) {
		t.Fatal("BuildFrame mutated the caller's PDU")
	}
	got, err := ParseFrame(buf[:n])
	if err != nil {
		t.Fatalf("ParseFrame: %v", err)
	}
	if !bytes.Equal(got.PDU, original) {
		t.Errorf("round-trip PDU = %x, want %x (transport must not pad/alter)", got.PDU, original)
	}
	// No trailing padding: the parsed PDU length equals the original exactly.
	if len(got.PDU) != len(original) {
		t.Errorf("PDU length = %d, want %d (transport must not pad)", len(got.PDU), len(original))
	}
}

func TestLLCConstantsExact(t *testing.T) {
	// VALIDATES: AC-2 LLC SAP/control constants per ISO/IEC 10589.
	if LLCSAP != 0xFE {
		t.Errorf("LLCSAP = %#x, want 0xFE", LLCSAP)
	}
	if LLCControl != 0x03 {
		t.Errorf("LLCControl = %#x, want 0x03", LLCControl)
	}
	if FrameHeaderLen != 2*MACLen+2+LLCHeaderLen {
		t.Errorf("FrameHeaderLen = %d, want %d", FrameHeaderLen, 2*MACLen+2+LLCHeaderLen)
	}
}
