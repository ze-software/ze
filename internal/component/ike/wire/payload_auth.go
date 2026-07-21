// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — Authentication payload (Section 3.8)
// RFC: rfc/short/rfc7427.md — Digital Signature AUTH method 14
package wire

// Auth methods (RFC 7296 Section 3.8, RFC 7427).
const (
	AuthMethodRSASig     uint8 = 1
	AuthMethodPSK        uint8 = 2
	AuthMethodDSSSig     uint8 = 3
	AuthMethodDigitalSig uint8 = 14
)

// PayloadAUTH is the Authentication payload (type 39).
type PayloadAUTH struct {
	AuthMethod uint8
	AuthData   []byte
}

func (p *PayloadAUTH) Type() uint8 { return PayloadTypeAUTH }

func (p *PayloadAUTH) WriteTo(buf []byte, off int) int {
	buf[off] = p.AuthMethod
	buf[off+1] = 0 // reserved
	buf[off+2] = 0
	buf[off+3] = 0
	copy(buf[off+4:], p.AuthData)
	return 4 + len(p.AuthData)
}

func (p *PayloadAUTH) Len() int { return 4 + len(p.AuthData) }

func (p *PayloadAUTH) ReadFrom(data []byte) error {
	if len(data) < 4 {
		return ErrTruncated
	}
	p.AuthMethod = data[0]
	p.AuthData = make([]byte, len(data)-4)
	copy(p.AuthData, data[4:])
	return nil
}
