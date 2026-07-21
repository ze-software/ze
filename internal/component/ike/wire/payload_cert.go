// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — Certificate payloads (Sections 3.6, 3.7)
package wire

// Certificate encoding types (RFC 7296 Section 3.6).
const (
	CertEncodingX509Sig uint8 = 4
	CertEncodingHashURL uint8 = 12
)

// PayloadCERT is the Certificate payload (type 37).
type PayloadCERT struct {
	CertEncoding uint8
	CertData     []byte
}

func (p *PayloadCERT) Type() uint8 { return PayloadTypeCERT }

func (p *PayloadCERT) WriteTo(buf []byte, off int) int {
	buf[off] = p.CertEncoding
	copy(buf[off+1:], p.CertData)
	return 1 + len(p.CertData)
}

func (p *PayloadCERT) Len() int { return 1 + len(p.CertData) }

func (p *PayloadCERT) ReadFrom(data []byte) error {
	if len(data) < 1 {
		return ErrTruncated
	}
	p.CertEncoding = data[0]
	p.CertData = make([]byte, len(data)-1)
	copy(p.CertData, data[1:])
	return nil
}

// PayloadCERTREQ is the Certificate Request payload (type 38).
type PayloadCERTREQ struct {
	CertEncoding  uint8
	CertAuthority []byte
}

func (p *PayloadCERTREQ) Type() uint8 { return PayloadTypeCERTREQ }

func (p *PayloadCERTREQ) WriteTo(buf []byte, off int) int {
	buf[off] = p.CertEncoding
	copy(buf[off+1:], p.CertAuthority)
	return 1 + len(p.CertAuthority)
}

func (p *PayloadCERTREQ) Len() int { return 1 + len(p.CertAuthority) }

func (p *PayloadCERTREQ) ReadFrom(data []byte) error {
	if len(data) < 1 {
		return ErrTruncated
	}
	p.CertEncoding = data[0]
	p.CertAuthority = make([]byte, len(data)-1)
	copy(p.CertAuthority, data[1:])
	return nil
}
