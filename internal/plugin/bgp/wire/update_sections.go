// Package wire provides zero-allocation buffer writing for BGP messages.
package wire

import (
	"errors"
)

// ErrUpdateTruncated indicates the UPDATE payload is shorter than declared lengths.
var ErrUpdateTruncated = errors.New("UPDATE payload truncated")

// UpdateSections holds parsed offsets into an UPDATE message body.
// This is a value type (not pointer) to avoid allocation.
// The struct stores only offsets, not data - call accessors with original payload.
//
// RFC 4271 Section 4.3 - UPDATE message body format:
//
//	+-----------------------------------------------------+
//	|   Withdrawn Routes Length (2 octets)                |
//	+-----------------------------------------------------+
//	|   Withdrawn Routes (variable)                       |
//	+-----------------------------------------------------+
//	|   Total Path Attribute Length (2 octets)            |
//	+-----------------------------------------------------+
//	|   Path Attributes (variable)                        |
//	+-----------------------------------------------------+
//	|   Network Layer Reachability Information (variable) |
//	+-----------------------------------------------------+
//
// Thread-safety: This type holds only immutable offset data.
// Multiple goroutines can safely call accessor methods concurrently.
//
// Fields use int (not uint32) because Go slice operations use int for indices,
// wire protocol uses uint16 for lengths (max 65535) which always fits in int,
// and this eliminates int-to-uint32 conversions when comparing with len().
type UpdateSections struct {
	wdLen     int  // Withdrawn routes length (bytes)
	attrStart int  // Offset where attrs begin (after wdLen field + withdrawn)
	attrLen   int  // Attributes length (bytes)
	nlriStart int  // Offset where NLRI begins
	valid     bool // Distinguishes zero-value from "parsed, all empty"
}

// ParseUpdateSections parses an UPDATE message body and returns section offsets.
// Returns error if payload is truncated or malformed.
//
// RFC 4271 Section 4.3 - Minimum UPDATE body is 4 octets:
// 2 octets Withdrawn Routes Length + 2 octets Total Path Attribute Length.
//
// RFC 8654 - Extended Message allows UPDATE body up to 65516 octets
// (65535 max message - 19 byte header).
//
// The returned UpdateSections can be used with the original payload to
// extract section slices via Withdrawn(), Attrs(), NLRI() methods.
func ParseUpdateSections(data []byte) (UpdateSections, error) {
	// RFC 4271 Section 4.3 - Minimum UPDATE body is 4 octets
	if len(data) < 4 {
		return UpdateSections{}, ErrUpdateTruncated
	}

	// RFC 4271 Section 4.3 - Withdrawn Routes Length: 2-octet unsigned integer
	// Read as uint16 from wire, convert to int for slice operations
	wdLen := int(data[0])<<8 | int(data[1])

	// Verify withdrawn routes fit in payload
	// Need: 2 (wdLen field) + wdLen + 2 (attrLen field)
	if len(data) < 2+wdLen+2 {
		return UpdateSections{}, ErrUpdateTruncated
	}

	// RFC 4271 Section 4.3 - Total Path Attribute Length: 2-octet unsigned integer
	attrLenOffset := 2 + wdLen
	attrLen := int(data[attrLenOffset])<<8 | int(data[attrLenOffset+1])

	// Verify attributes fit in payload
	attrStart := attrLenOffset + 2
	if len(data) < attrStart+attrLen {
		return UpdateSections{}, ErrUpdateTruncated
	}

	// RFC 4271 Section 4.3 - NLRI length is not encoded explicitly but calculated as:
	// remaining bytes after attributes section
	nlriStart := attrStart + attrLen

	return UpdateSections{
		wdLen:     wdLen,
		attrStart: attrStart,
		attrLen:   attrLen,
		nlriStart: nlriStart,
		valid:     true,
	}, nil
}

// Valid returns true if this UpdateSections was successfully parsed.
// A zero-value UpdateSections is not valid.
func (s UpdateSections) Valid() bool {
	return s.valid
}

// WithdrawnLen returns the length of the withdrawn routes section in bytes.
func (s UpdateSections) WithdrawnLen() int {
	return s.wdLen
}

// AttrsLen returns the length of the path attributes section in bytes.
func (s UpdateSections) AttrsLen() int {
	return s.attrLen
}

// NLRILen returns the length of the NLRI section in bytes.
// Requires the original payload to compute (NLRI length is implicit).
func (s UpdateSections) NLRILen(data []byte) int {
	if !s.valid || s.nlriStart > len(data) {
		return 0
	}
	return len(data) - s.nlriStart
}

// Withdrawn returns the withdrawn routes section as a slice into data.
// Returns nil if not valid, withdrawn length is 0, or data is too short.
// Zero-copy: returns a slice sharing the underlying array with data.
func (s UpdateSections) Withdrawn(data []byte) []byte {
	if !s.valid || s.wdLen == 0 {
		return nil
	}
	end := 2 + s.wdLen
	if end > len(data) {
		return nil
	}
	return data[2:end]
}

// Attrs returns the path attributes section as a slice into data.
// Returns nil if not valid, attributes length is 0, or data is too short.
// Zero-copy: returns a slice sharing the underlying array with data.
func (s UpdateSections) Attrs(data []byte) []byte {
	if !s.valid || s.attrLen == 0 {
		return nil
	}
	end := s.attrStart + s.attrLen
	if end > len(data) {
		return nil
	}
	return data[s.attrStart:end]
}

// NLRI returns the NLRI section as a slice into data.
// Returns nil if not valid or no NLRI present (nlriStart >= len(data)).
// Zero-copy: returns a slice sharing the underlying array with data.
func (s UpdateSections) NLRI(data []byte) []byte {
	if !s.valid || s.nlriStart >= len(data) {
		return nil
	}
	return data[s.nlriStart:]
}
