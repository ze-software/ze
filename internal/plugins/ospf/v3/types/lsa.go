// Design: docs/architecture/ospf/ospfv3-1-types.md -- LSType (with embedded flooding scope) + LSAKey.
// RFC: rfc/short/rfc5340.md (§A.4.2.1 LS type, U/S2/S1 bits and function codes),
// rfc/short/rfc7770.md (§2.2 Router Information LSA, function code 12, U-bit set)
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

// LS Type field masks (RFC 5340 §A.4.2.1): the U-bit is the most significant bit and the
// low 13 bits are the LSA function code; the two scope bits sit between them.
const (
	lsTypeUBit         LSType = 0x8000
	lsTypeFunctionMask LSType = 0x1FFF
)

// Scope selectors: the S2/S1 bits (RFC 5340 §A.4.2.1) that pick a flooding scope.
const (
	scopeBitsLinkLocal LSType = 0x0000 // S2S1 = 00
	scopeBitsArea      LSType = 0x2000 // S2S1 = 01
	scopeBitsAS        LSType = 0x4000 // S2S1 = 10
)

// Known OSPFv3 base LSA types (RFC 5340 §4.4).
const (
	LSTypeRouter          LSType = 0x2001 // Router-LSA (area)
	LSTypeNetwork         LSType = 0x2002 // Network-LSA (area)
	LSTypeInterAreaPrefix LSType = 0x2003 // Inter-Area-Prefix-LSA (area)
	LSTypeInterAreaRouter LSType = 0x2004 // Inter-Area-Router-LSA (area)
	LSTypeASExternal      LSType = 0x4005 // AS-External-LSA (AS)
	LSTypeNSSA            LSType = 0x2007 // NSSA-LSA (area)
	LSTypeLink            LSType = 0x0008 // Link-LSA (link-local)
	LSTypeGrace           LSType = 0x000B // Grace-LSA, LSA function code 11, link-local scope (U=0/S2=0/S1=0), RFC 5187 sec 2.1.
	LSTypeIntraAreaPrefix LSType = 0x2009 // Intra-Area-Prefix-LSA (area)
)

// RFC 8362 Extended-LSA types (spec-ospf-ext-5): the SR-relevant subset the OSPFv3
// Segment Routing extension (RFC 8666) rides on. Scope falls out of the high two bits
// exactly as for the base types, so the shared LSDB stores and floods them by scope
// with no new store. The U-bit is 0: an Extended LSA a router does not understand is
// flooded only within its scope, per RFC 8362 §3.
const (
	LSTypeERouter          LSType = 0x2021 // E-Router-LSA (area)
	LSTypeENetwork         LSType = 0x2022 // E-Network-LSA (area)
	LSTypeEInterAreaPrefix LSType = 0x2023 // E-Inter-Area-Prefix-LSA (area)
	LSTypeEASExternal      LSType = 0x4025 // E-AS-External-LSA (AS)
	LSTypeEType7           LSType = 0x2027 // E-Type-7-LSA (NSSA area)
	LSTypeELink            LSType = 0x0028 // E-Link-LSA (link-local)
	LSTypeEIntraAreaPrefix LSType = 0x2029 // E-Intra-Area-Prefix-LSA (area)
)

// RIFunctionCode is the RFC 7770 §2.2 / §5.2 OSPFv3 LSA function code for the Router
// Information LSA. The full 16-bit LS Type is U | S2 | S1 | function code, so the wire type
// depends on the flooding scope (see RouterInformationLSType); the function code alone
// identifies an RI LSA regardless of the U-bit or scope bits (IsRouterInformation).
const RIFunctionCode LSType = 0x000C

// Router Information LSA wire types (RFC 7770 §2.2): U-bit SET (flood even if not
// understood) plus the scope bits plus function code 12. RFC 5340 §4.4.1 requires U=1 so a
// non-supporting router still floods the area/AS-scope LSA rather than confining it to
// link-local scope; RFC 7770 §2.2 mandates it.
const (
	LSTypeRouterInformationLink LSType = lsTypeUBit | scopeBitsLinkLocal | RIFunctionCode // 0x800C
	LSTypeRouterInformationArea LSType = lsTypeUBit | scopeBitsArea | RIFunctionCode      // 0xA00C
	LSTypeRouterInformationAS   LSType = lsTypeUBit | scopeBitsAS | RIFunctionCode        // 0xC00C
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

// Known reports whether the type is a recognized OSPFv3 LSA type: an RFC 5340 base type or
// the RFC 7770 Router Information LSA (any scope). RI is recognized by its function code (the
// low 13 bits) regardless of the U-bit and scope bits, so a peer's RI LSA at link/area/AS
// scope, encoded with or without the U-bit, is still recognized as RI (RFC 7770 §5.2 assigns
// function code 12 exclusively to the RI LSA).
func (t LSType) Known() bool {
	switch t {
	case LSTypeRouter, LSTypeNetwork, LSTypeInterAreaPrefix, LSTypeInterAreaRouter,
		LSTypeASExternal, LSTypeNSSA, LSTypeLink, LSTypeGrace, LSTypeIntraAreaPrefix:
		return true
	case LSTypeERouter, LSTypeENetwork, LSTypeEInterAreaPrefix, LSTypeEASExternal,
		LSTypeEType7, LSTypeELink, LSTypeEIntraAreaPrefix:
		// RFC 8362 Extended LSAs (spec-ospf-ext-5): recognized so the LSDB stores and
		// floods them by scope; the SR consumer decodes the bodies it understands and
		// refloods unknown TLVs verbatim.
		return true
	default:
		return t&lsTypeFunctionMask == RIFunctionCode
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
