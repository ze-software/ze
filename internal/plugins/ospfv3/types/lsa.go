// Design: plan/spec-ospfv3-1-types.md -- LSType (with embedded flooding scope) + LSAKey.
// RFC: rfc/short/rfc5340.md (§A.4.2.1 LS type, U/S2/S1 bits and function codes)
//
// OSPFv3 widens the LS Type to 16 bits and embeds the flooding scope: the top bit is the
// U-bit (unknown-LSA handling) and the next two bits (S2,S1) select link-local / area /
// AS / reserved scope. The low 13 bits are the function code. Keeping scope decoding on
// the type means the LSDB and flooding never re-derive scope ad hoc.

package types

// LSType is an OSPFv3 16-bit LSA type (U-bit | S2 | S1 | 13-bit function code).
type LSType uint16

// floodScope is the flooding scope encoded by the LS Type S2/S1 bits.
type floodScope uint8

// LS Type bit masks.
const lsTypeScopeMask LSType = 0x6000

const lsTypeScopeShift = 13

// Known OSPFv3 base LSA types (RFC 5340 §4.4).
const (
	LSTypeRouter          LSType = 0x2001 // Router-LSA (area)
	LSTypeNetwork         LSType = 0x2002 // Network-LSA (area)
	LSTypeInterAreaPrefix LSType = 0x2003 // Inter-Area-Prefix-LSA (area)
	LSTypeInterAreaRouter LSType = 0x2004 // Inter-Area-Router-LSA (area)
	LSTypeASExternal      LSType = 0x4005 // AS-External-LSA (AS)
	LSTypeNSSA            LSType = 0x2007 // NSSA-LSA (area)
	LSTypeLink            LSType = 0x0008 // Link-LSA (link-local)
	LSTypeIntraAreaPrefix LSType = 0x2009 // Intra-Area-Prefix-LSA (area)
)

// LSTypeFromBytes reads a 16-bit big-endian LS Type from b[off:].
func LSTypeFromBytes(b []byte, off int) (LSType, error) {
	if off < 0 || off+2 > len(b) {
		return 0, ErrWrongLength
	}
	return LSType(uint16(b[off])<<8 | uint16(b[off+1])), nil
}

// WriteTo writes the 2 big-endian octets into buf at off and returns 2.
func (t LSType) WriteTo(buf []byte, off int) int { return writeUint16(buf, off, uint16(t)) }

// Scope returns the flooding scope from the S2/S1 bits.
func (t LSType) Scope() floodScope { return floodScope((t & lsTypeScopeMask) >> lsTypeScopeShift) }

// Known reports whether the type is one of the RFC 5340 base LSA types.
func (t LSType) Known() bool {
	switch t {
	case LSTypeRouter, LSTypeNetwork, LSTypeInterAreaPrefix, LSTypeInterAreaRouter,
		LSTypeASExternal, LSTypeNSSA, LSTypeLink, LSTypeIntraAreaPrefix:
		return true
	default:
		return false
	}
}

// LSAKey is the comparable LSDB identity of an LSA instance. RFC 5340 §A.4.2.1: the LSA
// is identified by (LS Type, Link State ID, Advertising Router); age, sequence number,
// checksum, and length are NOT part of identity. The LS Type already carries scope, so the
// key needs no separate scope field.
type LSAKey struct {
	Type              LSType
	LinkStateID       LinkStateID
	AdvertisingRouter RouterID
}

// Compare orders keys by LS Type, then Link State ID, then Advertising Router.
func (k LSAKey) Compare(other LSAKey) int {
	if k.Type != other.Type {
		if k.Type < other.Type {
			return -1
		}
		return 1
	}
	if c := compare4(k.LinkStateID, other.LinkStateID); c != 0 {
		return c
	}
	return compare4(k.AdvertisingRouter, other.AdvertisingRouter)
}
