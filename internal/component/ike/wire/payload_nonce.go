// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — Nonce payload (Section 3.9)
package wire

// RFC 7296 Section 2.10: nonce must be 16-256 bytes.
const (
	NonceMinLen = 16
	NonceMaxLen = 256
)

// PayloadNonce is the Nonce payload (type 40).
type PayloadNonce struct {
	NonceData []byte
}

func (p *PayloadNonce) Type() uint8 { return PayloadTypeNonce }

func (p *PayloadNonce) WriteTo(buf []byte, off int) int {
	copy(buf[off:], p.NonceData)
	return len(p.NonceData)
}

func (p *PayloadNonce) Len() int { return len(p.NonceData) }

func (p *PayloadNonce) ReadFrom(data []byte) error {
	if len(data) < NonceMinLen {
		return ErrNonceTooShort
	}
	if len(data) > NonceMaxLen {
		return ErrNonceTooLong
	}
	p.NonceData = make([]byte, len(data))
	copy(p.NonceData, data)
	return nil
}
