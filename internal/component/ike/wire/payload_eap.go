// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — EAP payload (Section 3.16)
package wire

import (
	"encoding/binary"
	"errors"
)

// ErrEAPLengthExceedsData reports an EAP message whose Length field claims more
// octets than the EAP payload carries. It is not ErrTruncated, because the two
// draw different answers: a truncated IKE payload is an IKE error, while this is
// an EAP-layer error that RFC 3748 Section 4 makes the receiver discard in silence.
// The engine drops the IKE message that carries it and keeps the EAP exchange.
var ErrEAPLengthExceedsData = errors.New("ike: EAP length exceeds the received octets")

// EAP codes.
const (
	EAPCodeRequest  uint8 = 1
	EAPCodeResponse uint8 = 2
	EAPCodeSuccess  uint8 = 3
	EAPCodeFailure  uint8 = 4
)

// PayloadEAP is the Extensible Authentication Protocol payload (type 48).
type PayloadEAP struct {
	Code       uint8
	Identifier uint8
	EAPData    []byte
}

func (p *PayloadEAP) Type() uint8 { return PayloadTypeEAP }

func (p *PayloadEAP) WriteTo(buf []byte, off int) int {
	buf[off] = p.Code
	buf[off+1] = p.Identifier
	totalLen := uint16(4 + len(p.EAPData))
	binary.BigEndian.PutUint16(buf[off+2:], totalLen)
	copy(buf[off+4:], p.EAPData)
	return 4 + len(p.EAPData)
}

func (p *PayloadEAP) Len() int { return 4 + len(p.EAPData) }

func (p *PayloadEAP) ReadFrom(data []byte) error {
	if len(data) < 4 {
		return ErrTruncated
	}
	p.Code = data[0]
	p.Identifier = data[1]
	eapLen := int(binary.BigEndian.Uint16(data[2:4]))
	if eapLen < 4 {
		return ErrTruncated
	}
	// RFC 3748 Section 4: "A message with the Length field set to a value
	// larger than the number of received octets MUST be silently discarded."
	// The distinct error lets the engine discard it rather than end the IKE SA.
	if eapLen > len(data) {
		return ErrEAPLengthExceedsData
	}
	if eapLen > 4 {
		p.EAPData = make([]byte, eapLen-4)
		copy(p.EAPData, data[4:eapLen])
	}
	return nil
}
