// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc4271.md — error codes and subcodes (Section 6)
// Overview: message.go — Message interface and writeHeader

package message

import "errors"

// Wire format errors.
var (
	// ErrShortRead indicates insufficient data for parsing.
	ErrShortRead = errors.New("short read: insufficient data")

	// ErrInvalidMarker indicates the 16-byte marker is not all 0xFF.
	ErrInvalidMarker = errors.New("invalid marker: expected 16 bytes of 0xFF")

	// ErrInvalidLength indicates the message length is out of valid range.
	ErrInvalidLength = errors.New("invalid length: must be >= 19")
)
