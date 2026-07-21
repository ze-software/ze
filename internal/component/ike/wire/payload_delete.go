// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — Delete payload (Section 3.11)
package wire

import "encoding/binary"

// PayloadDelete is the Delete payload (type 42).
type PayloadDelete struct {
	ProtocolID uint8
	SPISize    uint8
	NumSPIs    uint16
	SPIs       []byte
}

func (p *PayloadDelete) Type() uint8 { return PayloadTypeDelete }

func (p *PayloadDelete) WriteTo(buf []byte, off int) int {
	buf[off] = p.ProtocolID
	buf[off+1] = p.SPISize
	binary.BigEndian.PutUint16(buf[off+2:], p.NumSPIs)
	copy(buf[off+4:], p.SPIs)
	return 4 + len(p.SPIs)
}

func (p *PayloadDelete) Len() int { return 4 + len(p.SPIs) }

func (p *PayloadDelete) ReadFrom(data []byte) error {
	if len(data) < 4 {
		return ErrTruncated
	}
	p.ProtocolID = data[0]
	p.SPISize = data[1]
	p.NumSPIs = binary.BigEndian.Uint16(data[2:4])
	// Both values are small (uint16 * uint8), product fits in int.
	expected := int(p.NumSPIs) * int(p.SPISize)
	if expected < 0 || len(data)-4 < expected {
		return ErrTruncated
	}
	if expected > 0 {
		p.SPIs = make([]byte, expected)
		copy(p.SPIs, data[4:4+expected])
	}
	return nil
}
