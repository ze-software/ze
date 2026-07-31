// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — IKEv2 payload generic header (Section 3.2)
package wire

import (
	"errors"
	"fmt"
)

// RFC 7296 Section 3.2: generic payload header is 4 bytes.
const GenericHeaderLen = 4

// Maximum number of payloads in a chain to prevent infinite loops.
const MaxPayloads = 128

// Maximum nesting depth for proposal/transform parsing.
const MaxNestingDepth = 64

var (
	ErrTruncated         = errors.New("ike: data truncated")
	ErrPayloadTooShort   = errors.New("ike: payload length less than generic header")
	ErrUnsupportedCrit   = errors.New("ike: unsupported critical payload")
	ErrTooManyPayloads   = errors.New("ike: payload chain exceeds maximum count")
	ErrNonceTooShort     = errors.New("ike: nonce shorter than 16 bytes")
	ErrNonceTooLong      = errors.New("ike: nonce longer than 256 bytes")
	ErrInvalidSPISize    = errors.New("ike: invalid SPI size")
	ErrNotifyProtocolID  = errors.New("ike: notify carrying an SPI must name AH or ESP")
	ErrNoProposals       = errors.New("ike: SA contains no proposals")
	ErrNoTrafficSelector = errors.New("ike: TS payload contains no selectors")
	ErrHeaderTooShort    = errors.New("ike: message shorter than header")
	ErrLengthMismatch    = errors.New("ike: header length exceeds data")
	ErrUnknownPayload    = errors.New("ike: unknown payload type")
)

// Payload type values (RFC 7296 Section 3.2).
const (
	PayloadTypeSA       uint8 = 33
	PayloadTypeKE       uint8 = 34
	PayloadTypeIDi      uint8 = 35
	PayloadTypeIDr      uint8 = 36
	PayloadTypeCERT     uint8 = 37
	PayloadTypeCERTREQ  uint8 = 38
	PayloadTypeAUTH     uint8 = 39
	PayloadTypeNonce    uint8 = 40
	PayloadTypeNotify   uint8 = 41
	PayloadTypeDelete   uint8 = 42
	PayloadTypeVendorID uint8 = 43
	PayloadTypeTSi      uint8 = 44
	PayloadTypeTSr      uint8 = 45
	PayloadTypeSK       uint8 = 46
	PayloadTypeCP       uint8 = 47
	PayloadTypeEAP      uint8 = 48
)

// Payload is the interface for all IKEv2 payload types.
type Payload interface {
	Type() uint8
	WriteTo(buf []byte, off int) int
	// Len reports the number of bytes WriteTo will write for the current
	// payload contents. It must equal WriteTo's return value exactly, so
	// Message.CheckedWriteTo can size a fixed buffer before any index write.
	Len() int
	ReadFrom(data []byte) error
}

// GenericHeader is the 4-byte header prefixing every payload.
type GenericHeader struct {
	NextPayload uint8
	Critical    bool
	Length      uint16
}

func (g *GenericHeader) WriteTo(buf []byte, off int) int {
	buf[off] = g.NextPayload
	buf[off+1] = 0
	if g.Critical {
		buf[off+1] = 0x80
	}
	buf[off+2] = byte(g.Length >> 8)
	buf[off+3] = byte(g.Length)
	return GenericHeaderLen
}

func (g *GenericHeader) ReadFrom(data []byte) error {
	if len(data) < GenericHeaderLen {
		return ErrTruncated
	}
	g.NextPayload = data[0]
	// RFC 7296 Section 3.2: critical bit is the high bit of octet 1
	g.Critical = data[1]&0x80 != 0
	g.Length = uint16(data[2])<<8 | uint16(data[3])
	return nil
}

// PayloadRaw holds an unknown or unparsed payload (skipped by type).
type PayloadRaw struct {
	PayloadType uint8
	Data        []byte
}

func (p *PayloadRaw) Type() uint8 { return p.PayloadType }

func (p *PayloadRaw) WriteTo(buf []byte, off int) int {
	copy(buf[off:], p.Data)
	return len(p.Data)
}

func (p *PayloadRaw) Len() int { return len(p.Data) }

func (p *PayloadRaw) ReadFrom(data []byte) error {
	p.Data = data
	return nil
}

// decodePayload parses a single typed payload from its body (after generic header).
func decodePayload(ptype uint8, data []byte) (Payload, error) {
	var p Payload
	switch ptype {
	case PayloadTypeSA:
		p = &PayloadSA{}
	case PayloadTypeKE:
		p = &PayloadKE{}
	case PayloadTypeIDi:
		p = &PayloadID{IDPayloadType: PayloadTypeIDi}
	case PayloadTypeIDr:
		p = &PayloadID{IDPayloadType: PayloadTypeIDr}
	case PayloadTypeCERT:
		p = &PayloadCERT{}
	case PayloadTypeCERTREQ:
		p = &PayloadCERTREQ{}
	case PayloadTypeAUTH:
		p = &PayloadAUTH{}
	case PayloadTypeNonce:
		p = &PayloadNonce{}
	case PayloadTypeNotify:
		p = &PayloadNotify{}
	case PayloadTypeDelete:
		p = &PayloadDelete{}
	case PayloadTypeVendorID:
		p = &PayloadVendorID{}
	case PayloadTypeTSi:
		p = &PayloadTS{TSPayloadType: PayloadTypeTSi}
	case PayloadTypeTSr:
		p = &PayloadTS{TSPayloadType: PayloadTypeTSr}
	case PayloadTypeSK:
		p = &PayloadSK{}
	case PayloadTypeCP:
		p = &PayloadCP{}
	case PayloadTypeEAP:
		p = &PayloadEAP{}
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnknownPayload, ptype)
	}
	if err := p.ReadFrom(data); err != nil {
		if isItemRejected(err) {
			// One proposal or transform was dropped and the payload holds the rest
			// (RFC 7296 Section 3.3.6). The caller receives what survived beside the
			// reason, so it can keep the payload.
			return p, err
		}
		return nil, err
	}
	return p, nil
}
