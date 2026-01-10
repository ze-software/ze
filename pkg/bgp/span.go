// Package bgp provides core BGP protocol types and utilities.
package bgp

// Span represents a section of a byte buffer.
// Used for zero-copy access to message sections without allocation.
//
// Example:
//
//	span := Span{Start: 10, Len: 20}
//	section := span.Slice(buf)  // buf[10:30]
type Span struct {
	Start int
	Len   int
}

// Slice extracts the section from buf.
// Returns buf[Start:Start+Len].
// Panics if span exceeds buffer bounds.
func (s Span) Slice(buf []byte) []byte {
	return buf[s.Start : s.Start+s.Len]
}

// End returns the end offset (Start + Len).
func (s Span) End() int {
	return s.Start + s.Len
}

// IsEmpty returns true if the span has zero length.
func (s Span) IsEmpty() bool {
	return s.Len == 0
}

// Valid returns true if the span is within buffer bounds.
func (s Span) Valid(bufLen int) bool {
	return s.Start >= 0 && s.Len >= 0 && s.Start+s.Len <= bufLen
}
