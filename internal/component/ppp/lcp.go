// Design: docs/research/l2tpv2-implementation-guide.md -- LCP packet format
// Related: ppp_fsm.go -- FSM that drives Configure/Terminate exchanges
// Related: lcp_options.go -- option codec used inside Configure-* packets
// Related: echo.go -- Echo-Request/Reply built on top of WriteLCPPacket

package ppp

import (
	"encoding/binary"
	"errors"
	"strconv"
)

// LCP code values from RFC 1661 Section 5. NCPs (IPCP, IPv6CP) reuse
// codes 1-7 with the same semantics; codes 8-11 are LCP-specific.
const (
	LCPConfigureRequest uint8 = 1
	LCPConfigureAck     uint8 = 2
	LCPConfigureNak     uint8 = 3
	LCPConfigureReject  uint8 = 4
	LCPTerminateRequest uint8 = 5
	LCPTerminateAck     uint8 = 6
	LCPCodeReject       uint8 = 7
	LCPProtocolReject   uint8 = 8  // LCP only
	LCPEchoRequest      uint8 = 9  // LCP only
	LCPEchoReply        uint8 = 10 // LCP only
	LCPDiscardRequest   uint8 = 11 // LCP only
)

// lcpHeaderLen is the fixed header size: Code + Identifier + Length.
// RFC 1661 Section 5 defines Length as two octets indicating the entire
// LCP packet length, including the Code, Identifier, Length and Data
// fields.
const lcpHeaderLen = 4

// errLCPTooShort is returned when a buffer is smaller than the LCP
// header.
var errLCPTooShort = errors.New("ppp: LCP packet shorter than 4-byte header")

// errLCPLengthMismatch is returned when the Length field does not
// match the buffer length, OR is below the header minimum, OR exceeds
// MaxFrameLen-2 (PPP frame max minus protocol field).
var errLCPLengthMismatch = errors.New("ppp: LCP Length field does not match buffer")

// LCPPacket is a parsed LCP packet. Data is a sub-slice of the input
// buffer; callers MUST NOT retain the slice past the next read into
// that buffer.
type LCPPacket struct {
	Code       uint8
	Identifier uint8
	Data       []byte // code-specific payload, length = total - 4
}

// ParseLCPPacket decodes the 4-byte LCP header and validates the
// Length field against the input. Returns the parsed packet with
// Data sub-slicing into buf.
//
// RFC 1661 Section 5:
//
//	Length MUST be greater than or equal to 4.
//	The Length field is the entire packet length; bytes beyond
//	Length in the source frame MUST be ignored as padding.
func ParseLCPPacket(buf []byte) (LCPPacket, error) {
	if len(buf) < lcpHeaderLen {
		return LCPPacket{}, errLCPTooShort
	}
	length := int(binary.BigEndian.Uint16(buf[2:4]))
	if length < lcpHeaderLen {
		return LCPPacket{}, errLCPLengthMismatch
	}
	if length > len(buf) {
		return LCPPacket{}, errLCPLengthMismatch
	}
	if length > MaxFrameLen-2 {
		return LCPPacket{}, errLCPLengthMismatch
	}
	return LCPPacket{
		Code:       buf[0],
		Identifier: buf[1],
		Data:       buf[lcpHeaderLen:length],
	}, nil
}

// WriteLCPPacket encodes an LCP packet into buf at offset off using
// skip-and-backfill for the Length field. Returns total bytes written
// (4 + len(data)).
//
// The caller MUST ensure buf[off:] has cap >= 4 + len(data). No
// allocation; pure offset writes per .claude/rules/buffer-first.md.
func WriteLCPPacket(buf []byte, off int, code, identifier uint8, data []byte) int {
	buf[off] = code
	buf[off+1] = identifier
	// Skip the Length field; backfill below.
	n := copy(buf[off+lcpHeaderLen:], data)
	total := lcpHeaderLen + n
	binary.BigEndian.PutUint16(buf[off+2:off+4], uint16(total))
	return total
}

// LCPCodeName returns the lowercase name of an LCP code, or "code-N"
// for unknown values. Used in log fields and FSM debug output.
func LCPCodeName(code uint8) string {
	switch code {
	case LCPConfigureRequest:
		return "configure-request"
	case LCPConfigureAck:
		return "configure-ack"
	case LCPConfigureNak:
		return "configure-nak"
	case LCPConfigureReject:
		return "configure-reject"
	case LCPTerminateRequest:
		return "terminate-request"
	case LCPTerminateAck:
		return "terminate-ack"
	case LCPCodeReject:
		return "code-reject"
	case LCPProtocolReject:
		return "protocol-reject"
	case LCPEchoRequest:
		return "echo-request"
	case LCPEchoReply:
		return "echo-reply"
	case LCPDiscardRequest:
		return "discard-request"
	}
	return "code-" + strconv.Itoa(int(code))
}
