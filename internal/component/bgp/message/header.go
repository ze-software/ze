// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc4271.md — message header format (Section 4.1)
// Overview: message.go — Message interface and writeHeader
//
// Package message provides BGP message types and parsing.
package message

import (
	"encoding/binary"
	"fmt"
)

// RFC 4271 Section 4.1 - BGP message header constants.
// The header consists of Marker (16 octets) + Length (2 octets) + Type (1 octet).
const (
	// RFC 4271 Section 4.1 - Marker is 16 octets, MUST be set to all ones.
	MarkerLen = 16
	// RFC 4271 Section 4.1 - Fixed header size: Marker(16) + Length(2) + Type(1) = 19 octets.
	HeaderLen = 19
	// RFC 4271 Section 4.1 - Length field MUST be at least 19 and no greater than 4096.
	MaxMsgLen = 4096
	// RFC 8654 - Extended Message capability allows messages up to 65535 octets.
	ExtMsgLen = 65535

	// Minimum message lengths per RFC 4271.
	MinOpenLen         = 29 // RFC 4271 Section 4.2: "The minimum length of the OPEN message is 29 octets."
	MinUpdateLen       = 23 // RFC 4271 Section 4.3: "The minimum length of the UPDATE message is 23 octets."
	MinNotificationLen = 21 // RFC 4271 Section 4.5: "The minimum length of the NOTIFICATION message is 21 octets."
	KeepaliveLen       = 19 // RFC 4271 Section 4.4: KEEPALIVE is header-only (19 octets).
	// RFC 2918: ROUTE-REFRESH has AFI(2) + Reserved(1) + SAFI(1) = 4 bytes after header.
	MinRouteRefreshLen = 23
)

// RFC 4271 Section 4.1 - Marker field MUST be set to all ones for compatibility.
var Marker = [MarkerLen]byte{
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
}

// RFC 4271 Section 4.1 - Type is a 1-octet unsigned integer indicating message type.
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
		return fmt.Sprintf("UNKNOWN(%d)", t)
	}
}

// RFC 4271 Section 4.1 - BGP message header structure.
// Note: Marker field is not stored as it is always all ones.
type Header struct {
	// RFC 4271 Section 4.1 - 2-octet Length indicates total message length including header.
	Length uint16
	// RFC 4271 Section 4.1 - 1-octet Type indicates the message type code.
	Type MessageType
}

// ParseHeader parses a BGP message header from wire format.
// RFC 4271 Section 4.1 - Validates marker, length, and extracts type.
// RFC 4271 Section 6.1 - Error handling for message header errors.
//
// Returns ErrShortRead if data is less than 19 bytes.
// Returns ErrInvalidMarker if the 16-byte marker is not all 0xFF.
// Returns ErrInvalidLength if length is less than 19.
func ParseHeader(data []byte) (Header, error) {
	if len(data) < HeaderLen {
		return Header{}, ErrShortRead
	}

	// RFC 4271 Section 4.1 - Marker MUST be set to all ones.
	// RFC 4271 Section 6.1 - If Marker is not as expected, Connection Not Synchronized error.
	for i := range MarkerLen {
		if data[i] != 0xFF {
			return Header{}, ErrInvalidMarker
		}
	}

	// RFC 4271 Section 4.1 - Length is 2-octet unsigned integer at offset 16.
	length := binary.BigEndian.Uint16(data[16:18])
	// RFC 4271 Section 4.1 - Length MUST be at least 19 and no greater than 4096.
	// RFC 4271 Section 6.1 - Bad Message Length error if less than 19 or greater than 4096.
	if length < HeaderLen {
		return Header{}, ErrInvalidLength
	}

	// RFC 4271 Section 4.1 - Type is 1-octet unsigned integer at offset 18.
	return Header{
		Length: length,
		Type:   MessageType(data[18]),
	}, nil
}

// ValidateLength checks if the message length is valid for the message type.
// RFC 4271 Section 6.1 - Returns Message Header Error / Bad Message Length if invalid.
//
// Per-type requirements:
// - OPEN: >= 29 octets (RFC 4271 Section 4.2)
// - UPDATE: >= 23 octets (RFC 4271 Section 4.3)
// - NOTIFICATION: >= 21 octets (RFC 4271 Section 4.5)
// - KEEPALIVE: == 19 octets exactly (RFC 4271 Section 4.4)
// - ROUTE-REFRESH: >= 23 octets (RFC 2918)
//
// Note: For upper bound validation, use ValidateLengthWithMax after capability negotiation.
func (h Header) ValidateLength() error {
	var minLen uint16
	var exactLen bool

	switch h.Type {
	case TypeOPEN:
		minLen = MinOpenLen
	case TypeUPDATE:
		minLen = MinUpdateLen
	case TypeNOTIFICATION:
		minLen = MinNotificationLen
	case TypeKEEPALIVE:
		minLen = KeepaliveLen
		exactLen = true // KEEPALIVE must be exactly 19 octets
	case TypeROUTEREFRESH:
		minLen = MinRouteRefreshLen
	default:
		// Unknown message type - only basic length check (>= 19)
		minLen = HeaderLen
	}

	// RFC 4271 Section 6.1 - Check per-type length requirements
	if exactLen {
		if h.Length != minLen {
			return &Notification{
				ErrorCode:    NotifyMessageHeader,
				ErrorSubcode: NotifyHeaderBadLength,
				Data:         []byte{byte(h.Length >> 8), byte(h.Length)},
			}
		}
	} else {
		if h.Length < minLen {
			return &Notification{
				ErrorCode:    NotifyMessageHeader,
				ErrorSubcode: NotifyHeaderBadLength,
				Data:         []byte{byte(h.Length >> 8), byte(h.Length)},
			}
		}
	}

	return nil
}

// ValidateLengthWithMax checks message length against per-type minimums AND upper bound.
// RFC 4271 Section 6.1 - Returns Message Header Error / Bad Message Length if invalid.
// RFC 8654 - Extended Message capability changes upper bound for UPDATE, NOTIFICATION, ROUTE-REFRESH.
//
// Parameters:
//   - extendedMessage: true if Extended Message capability was negotiated
//
// Upper bounds per RFC 4271 + RFC 8654:
//   - OPEN: always 4096 (RFC 8654 Section 4)
//   - KEEPALIVE: always 19 (no extension)
//   - UPDATE, NOTIFICATION, ROUTE-REFRESH: 4096 or 65535 depending on Extended Message
func (h Header) ValidateLengthWithMax(extendedMessage bool) error {
	// First check per-type minimums
	if err := h.ValidateLength(); err != nil {
		return err
	}

	// RFC 8654 Section 4: "The BGP Extended Message Capability applies to all
	// messages except for OPEN and KEEPALIVE messages."
	maxLen := uint16(MaxMsgLen)
	switch h.Type {
	case TypeOPEN, TypeKEEPALIVE:
		// Always 4096 max for OPEN and KEEPALIVE (RFC 8654 Section 4)
		maxLen = MaxMsgLen
	case TypeUPDATE, TypeNOTIFICATION, TypeROUTEREFRESH:
		// UPDATE, NOTIFICATION, ROUTE-REFRESH: extended if negotiated
		if extendedMessage {
			maxLen = ExtMsgLen
		}
	}

	if h.Length > maxLen {
		return &Notification{
			ErrorCode:    NotifyMessageHeader,
			ErrorSubcode: NotifyHeaderBadLength,
			Data:         []byte{byte(h.Length >> 8), byte(h.Length)},
		}
	}

	return nil
}

// WriteTo writes a BGP message header into buf at offset.
// RFC 4271 Section 4.1: Marker(16) + Length(2) + Type(1) = 19 octets.
// Returns HeaderLen (19).
func (h Header) WriteTo(buf []byte, off int) int {
	copy(buf[off:], Marker[:])
	binary.BigEndian.PutUint16(buf[off+16:], h.Length)
	buf[off+18] = byte(h.Type)
	return HeaderLen
}

// MaxMessageLength returns the maximum message length for a given type.
// RFC 4271: Default max is 4096.
// RFC 8654: Extended Message capability raises limit to 65535 for UPDATE, NOTIFICATION, ROUTE-REFRESH.
// OPEN and KEEPALIVE are always limited to 4096 (RFC 8654 Section 4).
func MaxMessageLength(msgType MessageType, extendedMessage bool) uint16 {
	switch msgType {
	case TypeOPEN, TypeKEEPALIVE:
		return MaxMsgLen
	case TypeUPDATE, TypeNOTIFICATION, TypeROUTEREFRESH:
		if extendedMessage {
			return ExtMsgLen
		}
		return MaxMsgLen
	default:
		return MaxMsgLen
	}
}
