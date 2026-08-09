// Design: docs/architecture/isis/isis-1-types.md -- SequenceNumber (32-bit, reserved-zero semantics)

package types

// SequenceNumber bounds and constants.
const (
	// SequenceNumberLen is the LSP sequence number width on the wire (4 octets).
	SequenceNumberLen = 4
	// MaxSequenceNumber is the maximum 32-bit sequence value.
	MaxSequenceNumber = 1<<32 - 1 // 0xFFFFFFFF
	// FirstSequenceNumber is the first valid originated LSP version.
	//
	// ISO/IEC 10589 section 7.3: an LSP's Sequence Number is a 32-bit value that
	// increases monotonically. 0 is RESERVED and never a valid originated
	// version; origination starts at 1. (A purge is signaled by Remaining
	// Lifetime 0 at runtime, isis-6 -- NOT by sequence 0.)
	FirstSequenceNumber SequenceNumber = 1
)

// SequenceNumber is an LSP version, a 32-bit monotonically increasing value.
// The value 0 is reserved and never a valid originated version; the type
// represents 0 distinctly (IsReserved) and never silently coerces it. Sequence
// wraparound and re-origination are runtime concerns (isis-6); this type only
// models the value and the reserved-zero rule.
type SequenceNumber uint32

// SequenceNumberFromBytes decodes a 4-octet big-endian sequence number. A
// length other than 4 returns ErrWrongLength. The reserved value 0 is decoded
// as-is (not rejected here): receiving and recognizing a reserved value is a
// runtime concern; the type only reports it via IsReserved.
func SequenceNumberFromBytes(b []byte) (SequenceNumber, error) {
	if len(b) != SequenceNumberLen {
		return 0, ErrWrongLength
	}
	v := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	return SequenceNumber(v), nil
}

// IsReserved reports whether this is the reserved value 0 (never a valid
// originated LSP version). ISO/IEC 10589 section 7.3.
func (s SequenceNumber) IsReserved() bool { return s == 0 }

// Next returns the next sequence number, skipping the reserved 0: the successor
// of 0 (or of the 32-bit maximum, which wraps) is FirstSequenceNumber (1). Use
// NextChecked when the caller must distinguish a normal increment from a
// wraparound that requires purge-then-re-originate handling (isis-6).
func (s SequenceNumber) Next() SequenceNumber {
	next, _ := s.NextChecked()
	return next
}

// NextChecked returns the next sequence number and whether the increment
// wrapped past the 32-bit maximum. On wrap, and when incrementing from the
// reserved 0, it returns FirstSequenceNumber (1) so the result is never the
// reserved 0. The wrapped flag lets the runtime (isis-6) perform the required
// purge + ZeroAge wait before re-originating; the type itself never silently
// produces 0.
func (s SequenceNumber) NextChecked() (next SequenceNumber, wrapped bool) {
	if s == MaxSequenceNumber {
		return FirstSequenceNumber, true
	}
	// s+1 cannot be the reserved 0 here: the only s with s+1 == 0 is
	// MaxSequenceNumber, handled above. Incrementing the reserved 0 yields 1
	// (FirstSequenceNumber) directly.
	return s + 1, false
}

// WriteTo writes the 4 big-endian octets into buf at off; returns
// SequenceNumberLen. Buffer-first, no allocation.
func (s SequenceNumber) WriteTo(buf []byte, off int) int {
	v := uint32(s)
	buf[off] = byte(v >> 24)
	buf[off+1] = byte(v >> 16)
	buf[off+2] = byte(v >> 8)
	buf[off+3] = byte(v)
	return SequenceNumberLen
}
