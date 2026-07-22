// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc4271.md — message header format (Section 4.1)

// Package msgtype owns the BGP message-type code (the 1-octet Type field of
// the RFC 4271 header) and its RFC-defined values.
//
// It lives in internal/core because always-on consumers outside the BGP engine
// classify raw BGP messages by type -- the MRT recorder writes a BGP4MP record
// per message and only distinguishes UPDATE from everything else -- and must
// keep compiling when the engine is compiled out (//go:build ze_bgp). The BGP
// codec (internal/component/bgp/message) owns everything else about the header;
// only the type vocabulary is shared.
package msgtype

import "codeberg.org/thomas-mangin/ze/internal/core/textbuf"

// MessageType is the RFC 4271 Section 4.1 Type field: a 1-octet unsigned
// integer indicating the message type.
type MessageType uint8

// RFC 4271 Section 4.1 - Message type codes.
// Types 1-4 are defined in RFC 4271, type 5 (ROUTE-REFRESH) in RFC 2918.
const (
	TypeOPEN         MessageType = 1 // RFC 4271 Section 4.1
	TypeUPDATE       MessageType = 2 // RFC 4271 Section 4.1
	TypeNOTIFICATION MessageType = 3 // RFC 4271 Section 4.1
	TypeKEEPALIVE    MessageType = 4 // RFC 4271 Section 4.1
	TypeROUTEREFRESH MessageType = 5 // RFC 2918
)

// String returns a human-readable name for the message type.
func (t MessageType) String() string {
	switch t {
	case TypeOPEN:
		return "OPEN"
	case TypeUPDATE:
		return "UPDATE"
	case TypeNOTIFICATION:
		return "NOTIFICATION"
	case TypeKEEPALIVE:
		return "KEEPALIVE"
	case TypeROUTEREFRESH:
		return "ROUTE-REFRESH"
	default:
		var b textbuf.Buffer
		return b.Reset().Str("UNKNOWN(").Int(int64(t)).Byte(')').String()
	}
}
