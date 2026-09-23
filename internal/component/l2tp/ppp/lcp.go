// Design: docs/research/l2tpv2-implementation-guide.md -- LCP packet format
// Related: ppp_fsm.go -- FSM that drives Configure/Terminate exchanges
// Related: lcp_options.go -- option codec used inside Configure-* packets
// Related: echo.go -- Echo-Request/Reply built on top of WriteLCPPacket

package ppp

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/ze-software/ze/internal/core/textbuf"
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
// MaxFrameLen (the maximum Information field length).
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
	if length > MaxFrameLen {
		return LCPPacket{}, errLCPLengthMismatch
	}
	return LCPPacket{
		Code:       buf[0],
		Identifier: buf[1],
		Data:       buf[lcpHeaderLen:length],
	}, nil
}

// ValidateLCPReply checks a reply against the last transmitted Configure-Request.
// RFC 1661 Section 5.2: "On reception of a Configure-Ack, the Identifier
// field MUST match that of the last transmitted Configure-Request.
// Additionally, the Configuration Options in a Configure-Ack MUST exactly
// match those of the last transmitted Configure-Request. Invalid packets
// are silently discarded." Sections 5.3 and 5.4 impose the same Identifier
// check on Nak and Reject; Reject preserves an ordered, unmodified subset
// of the request (including the entire set, per Errata 543).
func ValidateLCPReply(reply LCPPacket, requestID uint8, requestData []byte) bool {
	if reply.Identifier != requestID {
		return false
	}
	if reply.Code == LCPConfigureAck {
		return bytes.Equal(reply.Data, requestData)
	}
	if reply.Code != LCPConfigureNak && reply.Code != LCPConfigureReject {
		return false
	}
	var seen [4]uint64
	nextRequested := 0
	appended := false
	for data := reply.Data; len(data) != 0; {
		if len(data) < 2 || int(data[1]) < 2 || int(data[1]) > len(data) {
			return false
		}
		typ, length := data[0], int(data[1])
		mask := uint64(1) << (typ % 64)
		if seen[typ/64]&mask != 0 {
			return false
		}
		seen[typ/64] |= mask
		if reply.Code == LCPConfigureNak {
			// A Nak may change a variable-length option (Section 5.3),
			// but it must still carry that option's required value fields.
			switch typ {
			case LCPOptMRU:
				if length != 4 {
					return false
				}
			case LCPOptACCM, LCPOptMagic:
				if length != 6 {
					return false
				}
			case LCPOptAuthProto:
				if length < 4 {
					return false
				}
			case LCPOptPFC, LCPOptACFC:
				return false
			}
		}
		found := -1
		var requested []byte
		for off := 0; off < len(requestData); {
			if len(requestData)-off < 2 {
				return false
			}
			n := int(requestData[off+1])
			if n < 2 || n > len(requestData)-off {
				return false
			}
			if requestData[off] == typ {
				found, requested = off, requestData[off:off+n]
				break
			}
			off += n
		}
		if found < 0 {
			if reply.Code == LCPConfigureReject {
				return false
			}
			// Section 5.3 permits additional desired options only at the end.
			appended = true
		} else {
			if appended || found < nextRequested {
				return false
			}
			if reply.Code == LCPConfigureReject && !bytes.Equal(data[:length], requested) {
				return false
			}
			nextRequested = found + len(requested)
		}
		data = data[length:]
	}
	return true
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
	return textbuf.StrInt("code-", int64(code))
}
