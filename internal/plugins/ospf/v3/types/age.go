// Design: docs/architecture/ospf/ospfv3-1-types.md -- LSAge 16-bit LSA age in seconds.
// RFC: rfc/short/rfc5340.md (§A.4.2.1 LS age; RFC 2328 §12.1.1 MaxAge semantics)
//
// OSPFv3 keeps the OSPFv2 16-bit LS age in seconds and the MaxAge flush boundary at 3600.

package types

// LSAge is an LSA age in seconds.
type LSAge uint16

// MaxAge (3600s) is the age at which an LSA is flushed from the LSDB.
const MaxAge LSAge = 3600

// IsMaxAge reports whether the age has reached MaxAge.
func (a LSAge) IsMaxAge() bool { return a >= MaxAge }

// WriteTo writes the 2 big-endian octets into buf at off and returns 2.
func (a LSAge) WriteTo(buf []byte, off int) int { return writeUint16(buf, off, uint16(a)) }
