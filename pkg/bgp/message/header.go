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
	for i := 0; i < MarkerLen; i++ {
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

// Pack serializes the header to wire format.
// RFC 4271 Section 4.1 - Returns a 19-byte slice with marker, length, and type.
func (h Header) Pack() []byte {
	data := make([]byte, HeaderLen)

	// RFC 4271 Section 4.1 - Marker (16 octets) MUST be set to all ones.
	copy(data[:MarkerLen], Marker[:])

	// RFC 4271 Section 4.1 - Length (2 octets) at offset 16.
	binary.BigEndian.PutUint16(data[16:18], h.Length)

	// RFC 4271 Section 4.1 - Type (1 octet) at offset 18.
	data[18] = byte(h.Type)

	return data
}
