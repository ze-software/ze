package ppp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// VALIDATES: ParseLCPOptions walks consecutive options and returns
//
//	them in order, with Data sub-slicing into the input.
func TestParseLCPOptionsList(t *testing.T) {
	// Three options: MRU=1500, Magic=0xDEADBEEF, PFC.
	mru := []byte{LCPOptMRU, 0x04, 0x05, 0xDC}
	magic := []byte{LCPOptMagic, 0x06, 0xDE, 0xAD, 0xBE, 0xEF}
	pfc := []byte{LCPOptPFC, 0x02}
	buf := append(append(mru, magic...), pfc...)

	opts, err := ParseLCPOptions(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts) != 3 {
		t.Fatalf("got %d opts, want 3", len(opts))
	}
	if opts[0].Type != LCPOptMRU || !bytes.Equal(opts[0].Data, []byte{0x05, 0xDC}) {
		t.Errorf("opt 0 wrong: %+v", opts[0])
	}
	if opts[1].Type != LCPOptMagic || binary.BigEndian.Uint32(opts[1].Data) != 0xDEADBEEF {
		t.Errorf("opt 1 wrong: %+v", opts[1])
	}
	if opts[2].Type != LCPOptPFC || len(opts[2].Data) != 0 {
		t.Errorf("opt 2 wrong: %+v", opts[2])
	}
}

func TestParseLCPOptionsTruncated(t *testing.T) {
	cases := []struct {
		name string
		buf  []byte
		want error
	}{
		{"only one byte", []byte{0x01}, errOptionTooShort},
		{"length 1 (below header)", []byte{0x01, 0x01}, errOptionLengthMismatch},
		{"length exceeds buf", []byte{0x01, 0x06, 0xAA, 0xBB}, errOptionLengthMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseLCPOptions(tc.buf)
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// VALIDATES: WriteLCPOption emits Type, Length (total), then Data.
func TestWriteLCPOption(t *testing.T) {
	buf := make([]byte, 16)
	n := WriteLCPOption(buf, 0, LCPOptMagic, []byte{0xDE, 0xAD, 0xBE, 0xEF})
	if n != 6 {
		t.Errorf("n = %d, want 6", n)
	}
	want := []byte{LCPOptMagic, 0x06, 0xDE, 0xAD, 0xBE, 0xEF}
	if !bytes.Equal(buf[:n], want) {
		t.Errorf("buf = %x, want %x", buf[:n], want)
	}
}

// VALIDATES: Round-trip: BuildLocalConfigRequest -> WriteLCPOptions ->
//
//	ParseLCPOptions yields the same logical options.
func TestLCPOptionsRoundTrip(t *testing.T) {
	o := LCPOptions{
		MRU:     1500,
		Magic:   0xCAFEBABE,
		HasACCM: true,
		ACCM:    0xFFFFFFFF,
		PFC:     true,
		ACFC:    true,
	}
	opts := BuildLocalConfigRequest(o)
	buf := make([]byte, 64)
	n := WriteLCPOptions(buf, 0, opts)
	parsed, err := ParseLCPOptions(buf[:n])
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(parsed) != len(opts) {
		t.Fatalf("got %d, want %d options", len(parsed), len(opts))
	}
	for i := range opts {
		if parsed[i].Type != opts[i].Type {
			t.Errorf("opt %d type mismatch", i)
		}
		if !bytes.Equal(parsed[i].Data, opts[i].Data) {
			t.Errorf("opt %d data mismatch", i)
		}
	}
}

// VALIDATES: NegotiatePeerOptions clamps an over-large peer MRU
//
//	(peer asks 2000, MaxMRU=1500) to a NAK with 1500.
func TestNegotiatePeerMRUTooLarge(t *testing.T) {
	mruData := make([]byte, 2)
	binary.BigEndian.PutUint16(mruData, 2000)
	opts := []LCPOption{{Type: LCPOptMRU, Data: mruData}}
	policy := LCPNegPolicy{MaxMRU: 1500}

	acks, naks, rejects := NegotiatePeerOptions(opts, policy)
	if len(acks) != 0 {
		t.Errorf("expected no acks, got %d", len(acks))
	}
	if len(rejects) != 0 {
		t.Errorf("expected no rejects, got %d", len(rejects))
	}
	if len(naks) != 1 {
		t.Fatalf("expected 1 nak, got %d", len(naks))
	}
	if binary.BigEndian.Uint16(naks[0].Data) != 1500 {
		t.Errorf("nak suggest = %d, want 1500", binary.BigEndian.Uint16(naks[0].Data))
	}
}

// VALIDATES: NegotiatePeerOptions accepts an in-range peer MRU.
func TestNegotiatePeerMRUOK(t *testing.T) {
	mruData := make([]byte, 2)
	binary.BigEndian.PutUint16(mruData, 1460)
	opts := []LCPOption{{Type: LCPOptMRU, Data: mruData}}
	policy := LCPNegPolicy{MaxMRU: 1500}

	acks, naks, rejects := NegotiatePeerOptions(opts, policy)
	if len(naks) != 0 || len(rejects) != 0 || len(acks) != 1 {
		t.Errorf("ack=%d nak=%d rej=%d, want 1/0/0", len(acks), len(naks), len(rejects))
	}
}

// VALIDATES: Boundary -- peer MRU below 64 (RFC 1661 §6.1 floor) is
//
//	NAKd up to 64.
func TestNegotiatePeerMRUTooSmall(t *testing.T) {
	mruData := make([]byte, 2)
	binary.BigEndian.PutUint16(mruData, 32)
	opts := []LCPOption{{Type: LCPOptMRU, Data: mruData}}
	policy := LCPNegPolicy{MaxMRU: 1500}
	_, naks, _ := NegotiatePeerOptions(opts, policy)
	if len(naks) != 1 {
		t.Fatalf("want 1 nak, got %d", len(naks))
	}
	if binary.BigEndian.Uint16(naks[0].Data) != 64 {
		t.Errorf("nak suggest = %d, want 64", binary.BigEndian.Uint16(naks[0].Data))
	}
}

// VALIDATES: Wrong-length MRU option is rejected.
func TestNegotiatePeerMRUWrongLength(t *testing.T) {
	opts := []LCPOption{{Type: LCPOptMRU, Data: []byte{0x05}}}
	_, _, rejects := NegotiatePeerOptions(opts, LCPNegPolicy{})
	if len(rejects) != 1 {
		t.Errorf("expected 1 reject, got %d", len(rejects))
	}
}

// VALIDATES: 6a default rejects peer-proposed Auth-Protocol.
func TestNegotiatePeerAuthProtoRejected(t *testing.T) {
	opts := []LCPOption{{Type: LCPOptAuthProto, Data: []byte{0xC0, 0x23}}} // PAP
	policy := LCPNegPolicy{AcceptAuthProto: false}
	_, _, rejects := NegotiatePeerOptions(opts, policy)
	if len(rejects) != 1 {
		t.Errorf("expected reject when AcceptAuthProto=false, got %d rejects", len(rejects))
	}
}

// VALIDATES: When AcceptAuthProto=true, peer auth is acked (handed to
//
//	6b for real handling).
func TestNegotiatePeerAuthProtoAccepted(t *testing.T) {
	opts := []LCPOption{{Type: LCPOptAuthProto, Data: []byte{0xC0, 0x23}}}
	policy := LCPNegPolicy{AcceptAuthProto: true}
	acks, _, _ := NegotiatePeerOptions(opts, policy)
	if len(acks) != 1 {
		t.Errorf("expected 1 ack, got %d", len(acks))
	}
}

// VALIDATES: Magic-Number = 0 is rejected (reserved per RFC 1661 §6.4).
func TestNegotiatePeerMagicZeroRejected(t *testing.T) {
	opts := []LCPOption{{Type: LCPOptMagic, Data: []byte{0, 0, 0, 0}}}
	_, _, rejects := NegotiatePeerOptions(opts, LCPNegPolicy{})
	if len(rejects) != 1 {
		t.Errorf("expected reject for zero magic, got %d", len(rejects))
	}
}

// VALIDATES: Unknown option types are rejected.
func TestNegotiatePeerUnknownRejected(t *testing.T) {
	opts := []LCPOption{{Type: 99, Data: []byte{0xAA}}}
	_, _, rejects := NegotiatePeerOptions(opts, LCPNegPolicy{})
	if len(rejects) != 1 {
		t.Errorf("expected reject for unknown type, got %d", len(rejects))
	}
}

// VALIDATES: PFC and ACFC have zero-length data; non-zero rejected.
func TestNegotiatePeerPFCBadLength(t *testing.T) {
	opts := []LCPOption{
		{Type: LCPOptPFC, Data: []byte{0xAA}},
		{Type: LCPOptACFC, Data: []byte{0xBB, 0xCC}},
	}
	_, _, rejects := NegotiatePeerOptions(opts, LCPNegPolicy{})
	if len(rejects) != 2 {
		t.Errorf("expected 2 rejects for non-empty PFC/ACFC, got %d", len(rejects))
	}
}
