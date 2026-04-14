// Design: docs/architecture/wire/l2tp.md — L2TP v2 header parse and encode
// RFC: rfc/short/rfc2661.md — RFC 2661 Section 3 (Control Message Header)
// Related: avp.go — AVP stream parsed from the header payload
// Related: errors.go — ErrShortBuffer, ErrUnsupportedVersion, ErrMalformedControl
// Related: pool.go — pooled buffers used for encoding

package l2tp

import "encoding/binary"

// Flag bits on the leading 16-bit word of the L2TP header.
// RFC 2661 Section 3.1: bit 0 is the MSB of the first octet.
const (
	flagT = 0x8000 // Type: 1=control, 0=data
	flagL = 0x4000 // Length field present
	flagS = 0x0800 // Sequence (Ns/Nr) present
	flagO = 0x0200 // Offset Size present
	flagP = 0x0100 // Priority (data messages only)

	verMask = 0x000F // Version field
	verL2TP = 2      // L2TPv2
)

// Fixed control-header prefix: T=1,L=1,S=1,O=0,P=0,Ver=2.
// RFC 2661 Section 3.1: "Control messages always have T=1, L=1, S=1, O=0, P=0, Ver=2".
const controlFlagsFixed = 0xC802

// ControlHeaderLen is the fixed size of an L2TPv2 control header in octets.
const ControlHeaderLen = 12

// MessageHeader is a parsed L2TP v2 header (control or data).
//
// Field validity depends on flag bits:
//   - Length is valid iff HasLength.
//   - Ns, Nr are valid iff HasSequence.
//   - OffsetSize is valid iff HasOffset.
//
// PayloadOff is the offset into the byte slice passed to ParseMessageHeader
// where the payload (AVPs for control messages, PPP frame for data messages)
// begins.
type MessageHeader struct {
	IsControl   bool
	HasLength   bool
	HasSequence bool
	HasOffset   bool
	Priority    bool
	Version     uint8
	Length      uint16
	TunnelID    uint16
	SessionID   uint16
	Ns          uint16
	Nr          uint16
	OffsetSize  uint16
	PayloadOff  int
}

// ParseMessageHeader decodes the L2TP v2 header at the start of b.
//
// Version is inspected first, so short L2TPv3 / L2F frames return
// ErrUnsupportedVersion rather than ErrShortBuffer. This matters: phase
// 3 distinguishes the two conditions to send StopCCN Result Code 5 for
// L2TPv3 and silently discard L2F, and cannot do so if a truncated
// L2TPv3 frame is indistinguishable from a truncated L2TPv2 frame.
//
// Errors:
//   - ErrShortBuffer: b is too short to read the flag/version word, or
//     too short for the header layout indicated by the flags.
//   - ErrUnsupportedVersion: the Ver field is not 2.
//   - ErrMalformedControl: a control message (T=1) has L=0 or S=0.
func ParseMessageHeader(b []byte) (MessageHeader, error) {
	if len(b) < 2 {
		return MessageHeader{}, ErrShortBuffer
	}
	word := binary.BigEndian.Uint16(b[:2])
	h := MessageHeader{
		IsControl:   word&flagT != 0,
		HasLength:   word&flagL != 0,
		HasSequence: word&flagS != 0,
		HasOffset:   word&flagO != 0,
		Priority:    word&flagP != 0,
		Version:     uint8(word & verMask),
	}
	// RFC 2661 Section 3.2: "MUST be 2 for L2TPv2".
	if h.Version != verL2TP {
		return h, ErrUnsupportedVersion
	}
	// Minimum L2TPv2 header is 6 octets (flags + TunnelID + SessionID) for
	// a data message with no optional fields. Control messages require L=1
	// and S=1 per Section 3.1, enforced below after length parsing.
	if len(b) < 6 {
		return h, ErrShortBuffer
	}
	// RFC 2661 Section 3.1: control messages MUST set L=1 and S=1.
	if h.IsControl && (!h.HasLength || !h.HasSequence) {
		return h, ErrMalformedControl
	}
	off := 2
	if h.HasLength {
		if len(b) < off+2 {
			return h, ErrShortBuffer
		}
		h.Length = binary.BigEndian.Uint16(b[off:])
		off += 2
		if int(h.Length) < off || int(h.Length) > len(b) {
			return h, ErrShortBuffer
		}
	}
	if len(b) < off+4 {
		return h, ErrShortBuffer
	}
	h.TunnelID = binary.BigEndian.Uint16(b[off:])
	h.SessionID = binary.BigEndian.Uint16(b[off+2:])
	off += 4
	if h.HasSequence {
		if len(b) < off+4 {
			return h, ErrShortBuffer
		}
		h.Ns = binary.BigEndian.Uint16(b[off:])
		h.Nr = binary.BigEndian.Uint16(b[off+2:])
		off += 4
	}
	if h.HasOffset {
		if len(b) < off+2 {
			return h, ErrShortBuffer
		}
		h.OffsetSize = binary.BigEndian.Uint16(b[off:])
		off += 2
		if len(b) < off+int(h.OffsetSize) {
			return h, ErrShortBuffer
		}
		off += int(h.OffsetSize)
	}
	h.PayloadOff = off
	// RFC 2661 Section 3.1: "Length is the total length of the message in
	// octets." Must cover at least the header we just parsed.
	if h.HasLength && int(h.Length) < off {
		return h, ErrShortBuffer
	}
	return h, nil
}

// WriteControlHeader writes a 12-byte L2TPv2 control-message header into
// buf at off. Returns ControlHeaderLen.
//
// The caller typically writes the header LAST, after AVPs have been assembled
// starting at buf[off+ControlHeaderLen:], so that the total length is known.
// Callers MUST ensure buf has at least ControlHeaderLen bytes available at off.
func WriteControlHeader(buf []byte, off int, length, tunnelID, sessionID, ns, nr uint16) int {
	binary.BigEndian.PutUint16(buf[off:], controlFlagsFixed)
	binary.BigEndian.PutUint16(buf[off+2:], length)
	binary.BigEndian.PutUint16(buf[off+4:], tunnelID)
	binary.BigEndian.PutUint16(buf[off+6:], sessionID)
	binary.BigEndian.PutUint16(buf[off+8:], ns)
	binary.BigEndian.PutUint16(buf[off+10:], nr)
	return ControlHeaderLen
}

// WriteDataHeader writes a variable-length L2TPv2 data-message header into
// buf at off, shaped by h's flag fields.
//
// Only these fields of h are read:
//   - HasLength, HasSequence, HasOffset, Priority (flag bits)
//   - Length (written when HasLength)
//   - TunnelID, SessionID (always written)
//   - Ns, Nr (written when HasSequence)
//   - OffsetSize (written when HasOffset, plus OffsetSize bytes of pad reserved)
//
// Version is always set to 2 and the T bit is cleared; h.IsControl is
// IGNORED. To serialize a control-shaped header use WriteControlHeader
// instead. Offset-pad bytes are NOT initialized; the caller MUST populate
// them after this call. Returns the number of bytes written.
func WriteDataHeader(buf []byte, off int, h MessageHeader) int {
	start := off
	var word uint16 = verL2TP
	if h.HasLength {
		word |= flagL
	}
	if h.HasSequence {
		word |= flagS
	}
	if h.HasOffset {
		word |= flagO
	}
	if h.Priority {
		word |= flagP
	}
	binary.BigEndian.PutUint16(buf[off:], word)
	off += 2
	if h.HasLength {
		binary.BigEndian.PutUint16(buf[off:], h.Length)
		off += 2
	}
	binary.BigEndian.PutUint16(buf[off:], h.TunnelID)
	binary.BigEndian.PutUint16(buf[off+2:], h.SessionID)
	off += 4
	if h.HasSequence {
		binary.BigEndian.PutUint16(buf[off:], h.Ns)
		binary.BigEndian.PutUint16(buf[off+2:], h.Nr)
		off += 4
	}
	if h.HasOffset {
		binary.BigEndian.PutUint16(buf[off:], h.OffsetSize)
		off += 2
		off += int(h.OffsetSize) // reserve pad bytes; caller populates
	}
	return off - start
}
