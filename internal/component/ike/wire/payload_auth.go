// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — Authentication payload (Section 3.8)
// RFC: rfc/short/rfc7427.md — Digital Signature AUTH method 14
// RFC: rfc/short/rfc4754.md — ECDSA AUTH methods 9, 10 and 11
package wire

// Auth methods (RFC 7296 Section 3.8, RFC 4754 Section 8, RFC 7427).
const (
	AuthMethodRSASig     uint8 = 1
	AuthMethodPSK        uint8 = 2
	AuthMethodDSSSig     uint8 = 3
	AuthMethodECDSA256   uint8 = 9  // RFC 4754 Section 8.1: "IANA assigned the ID value 9 to ECDSA-256."
	AuthMethodECDSA384   uint8 = 10 // RFC 4754 Section 8.2: "IANA assigned the ID value 10 to ECDSA-384."
	AuthMethodECDSA521   uint8 = 11 // RFC 4754 Section 8.3: "IANA assigned the ID value 11 to ECDSA-521."
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
