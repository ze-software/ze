// Design: docs/architecture/ospf/ospfv3-1-types.md -- LSSequenceNumber signed 32-bit LSA version.
// RFC: rfc/short/rfc5340.md (§A.4.2.1 LS sequence number; RFC 2328 §12.1.6 semantics)
//
// OSPFv3 keeps the OSPFv2 signed-32-bit LS sequence number space: InitialSequenceNumber
// 0x80000001, MaxSequenceNumber 0x7fffffff, and the reserved 0x80000000 that an originator
// must never use. A larger signed value is the more recent LSA instance.

package types

// LSSequenceNumber is the signed 32-bit LSA version.
type LSSequenceNumber int32

// LS sequence number boundary values (RFC 2328 §12.1.6, carried into RFC 5340).
const (
	// InitialSequenceNumber (0x80000001) is the first instance an originator floods.
	InitialSequenceNumber LSSequenceNumber = -0x7fffffff
	// MaxSequenceNumber (0x7fffffff) is the last value before a flush-and-restart.
	MaxSequenceNumber LSSequenceNumber = 0x7fffffff
	// reservedSequenceNumber (0x80000000) must never be originated.
	reservedSequenceNumber LSSequenceNumber = -0x80000000
)

// IsMax reports whether this is the maximum sequence number.
func (s LSSequenceNumber) IsMax() bool { return s == MaxSequenceNumber }

// IsReserved reports whether this is the reserved (never-originated) value.
func (s LSSequenceNumber) IsReserved() bool { return s == reservedSequenceNumber }

// Newer reports whether s is a more recent LSA instance than other (signed comparison).
func (s LSSequenceNumber) Newer(other LSSequenceNumber) bool { return s > other }

// Next returns the next sequence number for a re-origination (caller flushes at Max).
func (s LSSequenceNumber) Next() LSSequenceNumber { return s + 1 }
