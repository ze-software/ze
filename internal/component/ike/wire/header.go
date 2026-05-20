// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — IKEv2 message header (Section 3.1)
package wire

import "encoding/binary"

// RFC 7296 Section 3.1: IKE header is 28 bytes fixed.
const HeaderLen = 28

// Exchange types (RFC 7296 Section 3.1).
const (
	ExchangeIKESAInit     uint8 = 34
	ExchangeIKEAuth       uint8 = 35
	ExchangeCreateChildSA uint8 = 36
	ExchangeInformational uint8 = 37
)

// Header flags (RFC 7296 Section 3.1).
const (
	FlagInitiator uint8 = 0x08
	FlagVersion   uint8 = 0x10
	FlagResponse  uint8 = 0x20
)

// Header is the 28-byte IKEv2 message header.
type Header struct {
	InitiatorSPI [8]byte
	ResponderSPI [8]byte
	NextPayload  uint8
	MajorVersion uint8
	MinorVersion uint8
	ExchangeType uint8
	Flags        uint8
	MessageID    uint32
	Length       uint32
}

// WriteTo writes the 28-byte header into buf at off. Returns 28.
func (h *Header) WriteTo(buf []byte, off int) int {
	copy(buf[off:], h.InitiatorSPI[:])
	copy(buf[off+8:], h.ResponderSPI[:])
	buf[off+16] = h.NextPayload
	// RFC 7296 Section 3.1: version = (major << 4) | minor
	buf[off+17] = (h.MajorVersion << 4) | (h.MinorVersion & 0x0f)
	buf[off+18] = h.ExchangeType
	buf[off+19] = h.Flags
	binary.BigEndian.PutUint32(buf[off+20:], h.MessageID)
	binary.BigEndian.PutUint32(buf[off+24:], h.Length)
	return HeaderLen
}

// ReadFrom parses a 28-byte header from data.
func (h *Header) ReadFrom(data []byte) error {
	if len(data) < HeaderLen {
		return ErrTruncated
	}
	copy(h.InitiatorSPI[:], data[0:8])
	copy(h.ResponderSPI[:], data[8:16])
	h.NextPayload = data[16]
	h.MajorVersion = data[17] >> 4
	h.MinorVersion = data[17] & 0x0f
	h.ExchangeType = data[18]
	h.Flags = data[19]
	h.MessageID = binary.BigEndian.Uint32(data[20:24])
	h.Length = binary.BigEndian.Uint32(data[24:28])
	return nil
}
