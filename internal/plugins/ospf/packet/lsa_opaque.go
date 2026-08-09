// Design: docs/architecture/ospf/ospf-2-wire.md -- opaque/unknown LSA passthrough
// RFC 5250 opaque types 9/10/11 are retained verbatim in v1.

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/types"

// OpaqueLSA is a raw body for OSPF opaque LSA types 9, 10, and 11.
type OpaqueLSA struct {
	Type types.LSType
	Data []byte
}

// RFC 5250 Section 3 / Appendix A.2: the 32-bit Link State ID of an opaque LSA is
// split into an 8-bit Opaque Type (the high byte, an IANA registry selector) and a
// 24-bit Opaque ID (the low three bytes, an application-defined instance identifier).
// The split lives in the codec layer, NOT in the types.LinkStateID identity, so the
// LSDB key stays a plain 4-byte Link State ID.

// OpaqueTypeOf returns the Opaque Type (high 8 bits) of an opaque LSA's Link State ID.
func OpaqueTypeOf(id types.LinkStateID) uint8 { return id[0] }

// OpaqueIDOf returns the Opaque ID (low 24 bits) of an opaque LSA's Link State ID.
func OpaqueIDOf(id types.LinkStateID) uint32 {
	return uint32(id[1])<<16 | uint32(id[2])<<8 | uint32(id[3])
}

// OpaqueLinkStateID composes a Link State ID from an Opaque Type and Opaque ID. The
// Opaque ID is masked to its 24-bit namespace (RFC 5250 Appendix A.2).
func OpaqueLinkStateID(opaqueType uint8, opaqueID uint32) types.LinkStateID {
	return types.LinkStateID{opaqueType, byte(opaqueID >> 16), byte(opaqueID >> 8), byte(opaqueID)}
}

// OpaqueType returns the Opaque Type (high 8 bits of the Link State ID) of this LSA.
// Meaningful only for opaque LSAs (types 9/10/11).
func (l LSA) OpaqueType() uint8 { return OpaqueTypeOf(l.Header.LinkStateID) }

// OpaqueID returns the Opaque ID (low 24 bits of the Link State ID) of this LSA.
// Meaningful only for opaque LSAs (types 9/10/11).
func (l LSA) OpaqueID() uint32 { return OpaqueIDOf(l.Header.LinkStateID) }
