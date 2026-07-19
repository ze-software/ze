package wire

import (
	"bytes"
	"errors"
	"testing"
)

// RFC requirement: RFC7296-2.6-1 positive -- the 28-byte header (header.go:8) carries the
// 8-byte InitiatorSPI and 8-byte ResponderSPI pair; WriteTo/ReadFrom round-trip both SPIs
// byte-for-byte, so the (SPIi, SPIr) pair that identifies the IKE SA is present in every header.
func TestHeaderRoundtrip(t *testing.T) {
	h := Header{
		InitiatorSPI: [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		ResponderSPI: [8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18},
		NextPayload:  PayloadTypeSA,
		MajorVersion: 2,
		MinorVersion: 0,
		ExchangeType: ExchangeIKESAInit,
		Flags:        FlagInitiator,
		MessageID:    1,
		Length:       28,
	}

	buf := make([]byte, 128)
	n := h.WriteTo(buf, 0)
	if n != HeaderLen {
		t.Fatalf("WriteTo returned %d, want %d", n, HeaderLen)
	}

	var got Header
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if got.InitiatorSPI != h.InitiatorSPI {
		t.Errorf("InitiatorSPI = %x, want %x", got.InitiatorSPI, h.InitiatorSPI)
	}
	if got.ResponderSPI != h.ResponderSPI {
		t.Errorf("ResponderSPI = %x, want %x", got.ResponderSPI, h.ResponderSPI)
	}
	if got.NextPayload != h.NextPayload {
		t.Errorf("NextPayload = %d, want %d", got.NextPayload, h.NextPayload)
	}
	if got.MajorVersion != h.MajorVersion {
		t.Errorf("MajorVersion = %d, want %d", got.MajorVersion, h.MajorVersion)
	}
	if got.MinorVersion != h.MinorVersion {
		t.Errorf("MinorVersion = %d, want %d", got.MinorVersion, h.MinorVersion)
	}
	if got.ExchangeType != h.ExchangeType {
		t.Errorf("ExchangeType = %d, want %d", got.ExchangeType, h.ExchangeType)
	}
	if got.Flags != h.Flags {
		t.Errorf("Flags = %d, want %d", got.Flags, h.Flags)
	}
	if got.MessageID != h.MessageID {
		t.Errorf("MessageID = %d, want %d", got.MessageID, h.MessageID)
	}
	if got.Length != h.Length {
		t.Errorf("Length = %d, want %d", got.Length, h.Length)
	}
}

func TestHeaderVersionEncoding(t *testing.T) {
	h := Header{MajorVersion: 2, MinorVersion: 0}
	buf := make([]byte, HeaderLen)
	h.WriteTo(buf, 0)
	// Version byte should be 0x20 (major=2 << 4 | minor=0)
	if buf[17] != 0x20 {
		t.Errorf("version byte = 0x%02x, want 0x20", buf[17])
	}
}

// RFC requirement: RFC7296-2.6-1 negative -- a header shorter than the fixed 28 bytes cannot
// carry the full (SPIi, SPIr) pair, so ReadFrom (header.go:54) rejects it with ErrTruncated
// rather than accepting a header with a partial or absent SPI pair.
func TestDecodeTruncatedHeader(t *testing.T) {
	var h Header
	for _, size := range []int{0, 1, 15, 27} {
		err := h.ReadFrom(make([]byte, size))
		if !errors.Is(err, ErrTruncated) {
			t.Errorf("ReadFrom(%d bytes) = %v, want ErrTruncated", size, err)
		}
	}
}

func TestHeaderOffset(t *testing.T) {
	h := Header{
		InitiatorSPI: [8]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x00, 0x11},
		MajorVersion: 2,
		ExchangeType: ExchangeIKEAuth,
		MessageID:    42,
		Length:       100,
	}
	buf := make([]byte, 256)
	offset := 10
	n := h.WriteTo(buf, offset)
	if n != HeaderLen {
		t.Fatalf("WriteTo returned %d", n)
	}
	var got Header
	if err := got.ReadFrom(buf[offset : offset+n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if !bytes.Equal(got.InitiatorSPI[:], h.InitiatorSPI[:]) {
		t.Errorf("SPI mismatch at offset %d", offset)
	}
	if got.MessageID != 42 {
		t.Errorf("MessageID = %d, want 42", got.MessageID)
	}
}
