// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — Key Exchange payload (Section 3.4)
package wire

import "encoding/binary"

// PayloadKE is the Key Exchange payload (type 34).
type PayloadKE struct {
	DHGroup         uint16
	KeyExchangeData []byte
}

func (p *PayloadKE) Type() uint8 { return PayloadTypeKE }

func (p *PayloadKE) WriteTo(buf []byte, off int) int {
	binary.BigEndian.PutUint16(buf[off:], p.DHGroup)
	binary.BigEndian.PutUint16(buf[off+2:], 0) // reserved
	copy(buf[off+4:], p.KeyExchangeData)
	return 4 + len(p.KeyExchangeData)
}

func (p *PayloadKE) Len() int { return 4 + len(p.KeyExchangeData) }

func (p *PayloadKE) ReadFrom(data []byte) error {
	if len(data) < 4 {
		return ErrTruncated
	}
	p.DHGroup = binary.BigEndian.Uint16(data[0:2])
	p.KeyExchangeData = make([]byte, len(data)-4)
	copy(p.KeyExchangeData, data[4:])
	return nil
}
