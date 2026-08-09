// Design: docs/architecture/ospf/ospf-1-types.md -- LSSequenceNumber signed freshness value
// Related: lsakey.go -- sequence is excluded from LSAKey identity

package types

import "strconv"

// LSSequenceNumberLen is the LS sequence number wire width.
const LSSequenceNumberLen = 4

// LSSequenceNumber is the signed 32-bit LSA version field.
//
// RFC 2328 Section 12.1.6: InitialSequenceNumber is 0x80000001,
// MaxSequenceNumber is 0x7fffffff, and 0x80000000 is reserved.
type LSSequenceNumber uint32

const (
	// InitialSequenceNumber is the first originated sequence number.
	InitialSequenceNumber LSSequenceNumber = 0x80000001
	// MaxSequenceNumber is the highest usable sequence number.
	MaxSequenceNumber LSSequenceNumber = 0x7fffffff
	// ReservedSequenceNumber is never used on the wire by an originator.
	ReservedSequenceNumber LSSequenceNumber = 0x80000000
)

// LSSequenceNumberFromBytes decodes a 4-octet big-endian LS sequence number.
func LSSequenceNumberFromBytes(b []byte) (LSSequenceNumber, error) {
	if len(b) != LSSequenceNumberLen {
		return 0, ErrWrongLength
	}
	v := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	return LSSequenceNumber(v), nil
}

// IsReserved reports whether s is the reserved 0x80000000 value.
func (s LSSequenceNumber) IsReserved() bool { return s == ReservedSequenceNumber }

// IsMax reports whether s is MaxSequenceNumber, the wrap boundary.
func (s LSSequenceNumber) IsMax() bool { return s == MaxSequenceNumber }

// NewerThan reports whether s is newer than other by RFC 2328 Section 13.1 sequence ordering.
func (s LSSequenceNumber) NewerThan(other LSSequenceNumber) bool {
	return int32(s) > int32(other)
}

// Next returns the next sequence number, never ReservedSequenceNumber.
func (s LSSequenceNumber) Next() LSSequenceNumber {
	next, _ := s.NextChecked()
	return next
}

// NextChecked returns the next sequence and whether the caller crossed the wrap boundary.
func (s LSSequenceNumber) NextChecked() (LSSequenceNumber, bool) {
	if s == MaxSequenceNumber || s == ReservedSequenceNumber {
		return InitialSequenceNumber, true
	}
	next := s + 1
	if next == ReservedSequenceNumber {
		return InitialSequenceNumber, true
	}
	return next, false
}

// WriteTo writes the four big-endian sequence octets into buf at off.
func (s LSSequenceNumber) WriteTo(buf []byte, off int) int {
	return writeUint32(buf, off, uint32(s))
}

// String returns the signed decimal sequence number.
func (s LSSequenceNumber) String() string {
	return strconv.FormatInt(int64(int32(s)), 10)
}
