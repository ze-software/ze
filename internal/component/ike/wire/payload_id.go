// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — Identification payloads (Section 3.5)
package wire

// ID types (RFC 7296 Section 3.5).
const (
	IDTypeIPv4Addr   uint8 = 1
	IDTypeFQDN       uint8 = 2
	IDTypeRFC822Addr uint8 = 3
	IDTypeIPv6Addr   uint8 = 5
	IDTypeDERASN1DN  uint8 = 9
	IDTypeDERASN1GN  uint8 = 10
	IDTypeKeyID      uint8 = 11
)

// PayloadID is the Identification payload (types 35 IDi, 36 IDr).
type PayloadID struct {
	IDPayloadType uint8
	IDType        uint8
	IDData        []byte
}

func (p *PayloadID) Type() uint8 { return p.IDPayloadType }

func (p *PayloadID) WriteTo(buf []byte, off int) int {
	buf[off] = p.IDType
	buf[off+1] = 0 // reserved
	buf[off+2] = 0
	buf[off+3] = 0
	copy(buf[off+4:], p.IDData)
	return 4 + len(p.IDData)
}

func (p *PayloadID) Len() int { return 4 + len(p.IDData) }

func (p *PayloadID) ReadFrom(data []byte) error {
	if len(data) < 4 {
		return ErrTruncated
	}
	p.IDType = data[0]
	p.IDData = make([]byte, len(data)-4)
	copy(p.IDData, data[4:])
	return nil
}
