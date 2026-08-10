package wire

import (
	"encoding/binary"
	"errors"
	"testing"
)

func TestDecodeUnknownPayloadSkip(t *testing.T) {
	// Build a message with an unknown payload type (200), critical=false
	buf := make([]byte, 512)
	var h Header
	h.MajorVersion = 2
	h.ExchangeType = ExchangeIKESAInit
	h.NextPayload = 200 // unknown type

	// Write header
	h.WriteTo(buf, 0)

	// Write generic header for unknown payload: next=Nonce(40), critical=false, len=12
	buf[28] = PayloadTypeNonce               // next payload
	buf[29] = 0x00                           // not critical
	binary.BigEndian.PutUint16(buf[30:], 12) // 4 header + 8 body

	// 8 bytes of body
	for i := range 8 {
		buf[32+i] = byte(i)
	}

	// Write nonce payload: next=0, critical=false, len=4+32=36
	off := 40
	buf[off] = 0   // no next
	buf[off+1] = 0 // not critical
	binary.BigEndian.PutUint16(buf[off+2:], 36)

	// 32 bytes of nonce data
	for i := range 32 {
		buf[off+4+i] = byte(i + 100)
	}

	// Set total length
	totalLen := uint32(off + 4 + 32)
	binary.BigEndian.PutUint32(buf[24:], totalLen)

	var msg Message
	if err := msg.ReadFrom(buf[:totalLen]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if len(msg.Payloads) != 2 {
		t.Fatalf("got %d payloads, want 2", len(msg.Payloads))
	}

	// First payload should be raw (unknown skipped)
	if _, ok := msg.Payloads[0].Payload.(*payloadRaw); !ok {
		t.Errorf("payload[0] type = %T, want *PayloadRaw", msg.Payloads[0].Payload)
	}
	// Second should be nonce
	if _, ok := msg.Payloads[1].Payload.(*PayloadNonce); !ok {
		t.Errorf("payload[1] type = %T, want *PayloadNonce", msg.Payloads[1].Payload)
	}
}

func TestDecodeUnknownPayloadCritical(t *testing.T) {
	buf := make([]byte, 512)
	var h Header
	h.MajorVersion = 2
	h.ExchangeType = ExchangeIKESAInit
	h.NextPayload = 200 // unknown type
	h.WriteTo(buf, 0)

	// Generic header: next=0, critical=true, len=8
	buf[28] = 0
	buf[29] = 0x80 // critical bit set
	binary.BigEndian.PutUint16(buf[30:], 8)
	// 4 bytes body
	totalLen := uint32(28 + 8)
	binary.BigEndian.PutUint32(buf[24:], totalLen)

	var msg Message
	err := msg.ReadFrom(buf[:totalLen])
	if !errors.Is(err, ErrUnsupportedCrit) {
		t.Errorf("ReadFrom = %v, want ErrUnsupportedCrit", err)
	}
}

func TestDecodeTruncatedPayload(t *testing.T) {
	buf := make([]byte, 512)
	var h Header
	h.MajorVersion = 2
	h.ExchangeType = ExchangeIKESAInit
	h.NextPayload = PayloadTypeNonce
	h.WriteTo(buf, 0)

	// Generic header claims 100 bytes but message is only 36
	buf[28] = 0
	buf[29] = 0
	binary.BigEndian.PutUint16(buf[30:], 100) // payload says 100 bytes
	totalLen := uint32(36)                    // but only 8 bytes of payload space
	binary.BigEndian.PutUint32(buf[24:], totalLen)

	var msg Message
	err := msg.ReadFrom(buf[:totalLen])
	if err == nil {
		t.Error("expected error for truncated payload")
	}
}
