// Design: docs/architecture/ospf/ospf-1-types.md -- OSPFv2 LSA type discriminator
// Related: lsakey.go -- LSType is the first field of LSAKey
// RFC: rfc/short/rfc2328.md (OSPFv2 LSA types), rfc/short/rfc5340.md (OSPFv3 scope-typed
// LS Type, sec A.4.2.1), rfc/short/rfc7770.md (AS-scope Router Information LSA vs AS-External)

package types

// LSType is the OSPF LSA type discriminator. It is 16-bit in memory so one shared type
// holds both the OSPFv2 8-bit type (RFC 2328) and the OSPFv3 16-bit scope-typed LS Type
// (RFC 5340 sec A.4.2.1). The wire width is the codec's concern: the OSPFv2 codec writes
// and reads a single octet (WriteTo / LSTypeFromByte); the OSPFv3 codec uses two octets.
type LSType uint16

const (
	// LSTypeRouter is Type 1, Router-LSA.
	LSTypeRouter LSType = 1
	// LSTypeNetwork is Type 2, Network-LSA.
	LSTypeNetwork LSType = 2
	// LSTypeSummaryNetwork is Type 3, Summary-LSA for a network.
	LSTypeSummaryNetwork LSType = 3
	// LSTypeSummaryASBR is Type 4, Summary-LSA for an ASBR.
	LSTypeSummaryASBR LSType = 4
	// LSTypeASExternal is Type 5, AS-External-LSA.
	LSTypeASExternal LSType = 5
	// LSTypeNSSA is Type 7, NSSA-LSA.
	LSTypeNSSA LSType = 7
	// LSTypeLink is OSPFv3 Type 8, Link-LSA (link-local scope). OSPFv2 has no
	// implemented Type 8 LSA, so this remains out of InScope for the v2 codec.
	LSTypeLink LSType = 8
	// LSTypeOpaqueLink is Type 9, opaque link-local scope, recognized but out of v1 scope.
	LSTypeOpaqueLink LSType = 9
	// LSTypeOpaqueArea is Type 10, opaque area scope, recognized but out of v1 scope.
	LSTypeOpaqueArea LSType = 10
	// LSTypeOpaqueAS is Type 11, opaque AS scope, recognized but out of v1 scope.
	LSTypeOpaqueAS LSType = 11
	// LSTypeGraceV6 is the AF-neutral LSDB key type for an OSPFv3 Grace-LSA (RFC 5187
	// sec 2.1, wire LS Type 0x000B, function code 11, link-local scope). The on-wire
	// OSPFv3 value 0x000B numerically equals the OSPFv2 Opaque-AS Type 11, so the OSPFv3
	// codec maps the Grace-LSA to this DISTINCT sentinel (the otherwise-unused U-bit set,
	// 0x8000|0x000B) for LSDB keying / link-scope routing so it never collides with the
	// OSPFv2 Type-11 Opaque-AS store. This value is INTERNAL only: it is never written to
	// the wire (the OSPFv3 codec emits 0x000B). See codec_v6.go v6LSAHeaderToNeutral and
	// its encoder inverse, and lsdb.isLinkLSAType.
	LSTypeGraceV6 LSType = 0x800B
)

// LSTypeFromByte returns the LSA type represented by b.
func LSTypeFromByte(b byte) LSType { return LSType(b) }

// asScopeBits / asScope are the OSPFv3 LS Type scope field (RFC 5340 sec A.4.2.1, the S1/S2
// bits): a scope value of 0b10 selects AS-wide flooding. areaScope is the area-wide scope
// value (0b01) carried by the OSPFv3 NSSA-LSA.
const (
	asScopeBits LSType = 0x6000
	asScope     LSType = 0x4000
	areaScope   LSType = 0x2000
)

// ASExternal reports whether the LS type is specifically an AS-External LSA (LSA function
// code 5): the OSPFv2 Type 5 (RFC 2328) or the OSPFv3 AS-scope AS-External-LSA 0x4005 (RFC
// 5340 sec A.4.7). This is the precise "carries an AS-External route / makes the router an
// ASBR" test used by SPF external computation and ASBR detection. It is deliberately NARROWER
// than ASWide: another AS-scope native LSA (e.g. the RFC 7770 Router Information LSA function
// code 12, wire type 0xC00C) is AS-wide but is NOT an AS-External and must never yield a route
// or set the E-bit.
func (t LSType) ASExternal() bool {
	return t == LSTypeASExternal || t == asScope|LSTypeASExternal
}

// ASWide reports whether the LS type floods AS-wide and lives in the LSDB's AS-wide store:
// the OSPFv2 Type 5 (RFC 2328) or ANY OSPFv3 AS-scope LS Type (S2/S1 = 0b10, RFC 5340 sec
// A.4.2.1), which includes both the AS-External-LSA 0x4005 and the RFC 7770 AS-scope Router
// Information LSA 0xC00C. This is the store-routing / flooding-scope / stub-area-suppression
// test. The OSPFv2 8-bit types never set the scope bits, so for OSPFv2 this is exactly Type 5
// (Opaque-AS Type 11 has no scope bits and is routed by its own explicit branch), preserving
// OSPFv2 store routing.
func (t LSType) ASWide() bool {
	return t == LSTypeASExternal || t&asScopeBits == asScope
}

// NSSA reports whether the LS type is an NSSA-LSA: the OSPFv2 Type 7 (RFC 2328 / RFC 3101)
// or the OSPFv3 area-scoped NSSA-LSA 0x2007 (RFC 5340 sec A.4.8, area scope + functional
// code 7). The OSPFv2 8-bit types never reach 0x2007, so for OSPFv2 this is exactly Type 7,
// preserving its behavior.
func (t LSType) NSSA() bool {
	return t == LSTypeNSSA || t == areaScope|LSTypeNSSA
}

// InterAreaRouter reports whether the LS type summarizes an ASBR's reachability into an
// area: the OSPFv2 Type 4 Summary-ASBR-LSA (RFC 2328) or the OSPFv3 area-scoped
// Inter-Area-Router-LSA 0x2004 (RFC 5340 sec A.4.6, area scope + functional code 4). Like an
// AS-External, it is suppressed from stub/NSSA areas. The OSPFv2 8-bit types never reach
// 0x2004, so for OSPFv2 this is exactly Type 4, preserving its behavior.
func (t LSType) InterAreaRouter() bool {
	return t == LSTypeSummaryASBR || t == areaScope|LSTypeSummaryASBR
}

// Known reports whether t is a known OSPFv2 LSA type value.
func (t LSType) Known() bool { return t.inScope() || t.IsOpaque() }

// inScope reports whether t is implemented by the first OSPFv2 pass.
func (t LSType) inScope() bool {
	switch t {
	case LSTypeRouter, LSTypeNetwork, LSTypeSummaryNetwork, LSTypeSummaryASBR, LSTypeASExternal, LSTypeNSSA:
		return true
	default:
		return false
	}
}

// IsOpaque reports whether t is one of the RFC 5250 opaque LSA types recognized but out of scope.
func (t LSType) IsOpaque() bool {
	switch t {
	case LSTypeOpaqueLink, LSTypeOpaqueArea, LSTypeOpaqueAS:
		return true
	default:
		return false
	}
}

// WriteTo writes the one-octet LSA type into buf at off and returns 1.
func (t LSType) WriteTo(buf []byte, off int) int {
	buf[off] = byte(t)
	return 1
}

// String returns a stable lowercase name for known LSA types. OSPFv3 carries the
// flooding scope in the high bits of the LS Type; render those scope-typed values
// with the same semantic names as their OSPFv2 counterparts where they share a
// function code.
func (t LSType) String() string {
	switch t {
	case LSTypeRouter, areaScope | LSTypeRouter:
		return "router"
	case LSTypeNetwork, areaScope | LSTypeNetwork:
		return "network"
	case LSTypeSummaryNetwork, areaScope | LSTypeSummaryNetwork:
		return "summary-network"
	case LSTypeSummaryASBR, areaScope | LSTypeSummaryASBR:
		return "summary-asbr"
	case LSTypeASExternal, asScope | LSTypeASExternal:
		return "as-external"
	case LSTypeNSSA, areaScope | LSTypeNSSA:
		return "nssa"
	case LSTypeLink:
		return "link"
	case LSTypeGraceV6:
		return "grace"
	case areaScope | LSTypeOpaqueLink:
		return "intra-area-prefix"
	case LSTypeOpaqueLink:
		return "opaque-link"
	case LSTypeOpaqueArea:
		return "opaque-area"
	case LSTypeOpaqueAS:
		return "opaque-as"
	default:
		return "unknown"
	}
}
