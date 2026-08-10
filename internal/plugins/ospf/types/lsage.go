// Design: docs/architecture/ospf/ospf-1-types.md -- LSAge value and aging helpers
// Related: sequence.go -- sequence and age jointly feed freshness decisions

package types

import "strconv"

// LSAgeLen is the LS Age wire width.
const LSAgeLen = 2

const (
	// MaxAge is the RFC 2328 purge age in seconds.
	MaxAge uint16 = 3600
	// LSRefreshTime is the self-originated LSA refresh cadence in seconds.
	LSRefreshTime uint16 = 1800
	// MaxAgeDiff is the age-difference threshold in RFC 2328 Section 13.1.
	MaxAgeDiff uint16 = 900
	// DoNotAgeBit marks a frozen age value; low 15 bits still carry age.
	DoNotAgeBit uint16 = 0x8000
)

// LSAge is the 16-bit LS Age field, including the DoNotAge flag bit.
//
// RFC 2328 Appendix B defines MaxAge as 3600. The DoNotAge high bit is exposed
// separately so runtime code does not mistake a frozen LSA for a very old one.
type LSAge uint16

// LSAgeFromBytes decodes a two-octet big-endian LS Age field.
func LSAgeFromBytes(b []byte) (LSAge, error) {
	if len(b) != LSAgeLen {
		return 0, ErrWrongLength
	}
	return lSAgeFromRaw(uint16(b[0])<<8 | uint16(b[1]))
}

// lSAgeFromRaw validates and preserves a raw 16-bit LS Age field.
func lSAgeFromRaw(raw uint16) (LSAge, error) {
	if raw&^DoNotAgeBit > MaxAge {
		return 0, ErrOutOfRange
	}
	return LSAge(raw), nil
}

// Age returns the low 15-bit age in seconds, excluding DoNotAgeBit.
func (a LSAge) Age() uint16 { return uint16(a) &^ DoNotAgeBit }

// DoNotAge reports whether the high DoNotAge bit is set.
func (a LSAge) DoNotAge() bool { return uint16(a)&DoNotAgeBit != 0 }

// IsMaxAge reports whether the masked age is MaxAge.
func (a LSAge) IsMaxAge() bool { return a.Age() == MaxAge }

// Add increments age by seconds, saturating at MaxAge and preserving DoNotAge.
func (a LSAge) Add(seconds uint16) LSAge {
	if a.DoNotAge() {
		return a
	}
	age := a.Age()
	if seconds >= MaxAge || age > MaxAge-seconds {
		return LSAge(MaxAge)
	}
	return LSAge(age + seconds)
}

// WriteTo writes the two big-endian LS Age octets into buf at off.
func (a LSAge) WriteTo(buf []byte, off int) int {
	return writeUint16(buf, off, uint16(a))
}

// String returns the masked age in decimal seconds.
func (a LSAge) String() string {
	return strconv.FormatUint(uint64(a.Age()), 10)
}
