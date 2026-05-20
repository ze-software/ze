// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — Encrypted payload (Section 3.14)
package wire

// PayloadSK is the Encrypted (SK) payload (type 46).
// Stores raw ciphertext; decryption is handled by the crypto layer (ipsec-6).
type PayloadSK struct {
	CipherText []byte
}

func (p *PayloadSK) Type() uint8 { return PayloadTypeSK }

func (p *PayloadSK) WriteTo(buf []byte, off int) int {
	copy(buf[off:], p.CipherText)
	return len(p.CipherText)
}

func (p *PayloadSK) ReadFrom(data []byte) error {
	p.CipherText = make([]byte, len(data))
	copy(p.CipherText, data)
	return nil
}
