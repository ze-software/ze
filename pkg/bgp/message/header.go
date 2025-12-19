// Package message provides BGP message types and parsing.
package message

import (
	"encoding/binary"
	"fmt"
)

// BGP message header constants (RFC 4271)
const (
	MarkerLen = 16 // All 0xFF bytes
	HeaderLen = 19 // Marker(16) + Length(2) + Type(1)
	MaxMsgLen = 4096
	ExtMsgLen = 65535 // RFC 8654 Extended Message
)

// Marker is the 16-byte sync marker (all 0xFF).
var Marker = [MarkerLen]byte{
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
}

// MessageType represents the BGP message type.
type MessageType uint8

// BGP message types (RFC 4271)
const (
	TypeOPEN         MessageType = 1
	TypeUPDATE       MessageType = 2
	TypeNOTIFICATION MessageType = 3
	TypeKEEPALIVE    MessageType = 4
	TypeROUTEREFRESH MessageType = 5
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

// Header represents a BGP message header (RFC 4271 Section 4.1).
type Header struct {
	Length uint16      // Total message length including header
	Type   MessageType // Message type
}

// ParseHeader parses a BGP message header from wire format.
// Returns ErrShortRead if data is less than 19 bytes.
// Returns ErrInvalidMarker if the 16-byte marker is not all 0xFF.
// Returns ErrInvalidLength if length is less than 19.
func ParseHeader(data []byte) (Header, error) {
	if len(data) < HeaderLen {
		return Header{}, ErrShortRead
	}

	// Validate marker (must be 16 bytes of 0xFF)
	for i := 0; i < MarkerLen; i++ {
		if data[i] != 0xFF {
			return Header{}, ErrInvalidMarker
		}
	}

	length := binary.BigEndian.Uint16(data[16:18])
	if length < HeaderLen {
		return Header{}, ErrInvalidLength
	}

	return Header{
		Length: length,
		Type:   MessageType(data[18]),
	}, nil
}

// Pack serializes the header to wire format.
// Returns a 19-byte slice with marker, length, and type.
func (h Header) Pack() []byte {
	data := make([]byte, HeaderLen)

	// Marker
	copy(data[:MarkerLen], Marker[:])

	// Length
	binary.BigEndian.PutUint16(data[16:18], h.Length)

	// Type
	data[18] = byte(h.Type)

	return data
}
