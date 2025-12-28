// Package nlri provides capability-aware NLRI packing.
package nlri

// PackContext holds capability-dependent packing options.
// Used to adapt wire format based on negotiated session parameters.
//
// This type enables the Pack(ctx *PackContext) pattern where NLRI types
// can adapt their wire encoding based on negotiated capabilities.
type PackContext struct {
	// AddPath indicates ADD-PATH is negotiated for this family.
	// RFC 7911 Section 3: When true, NLRI includes 4-byte Path Identifier.
	AddPath bool

	// ASN4 indicates 4-byte AS number capability is negotiated.
	// RFC 6793 Section 4.1: When true, AS_PATH uses 4-byte AS numbers.
	// When false, AS_PATH uses 2-byte with AS_TRANS (23456) for large ASNs.
	ASN4 bool

	// Future fields for other capabilities:
	// ExtendedNextHop AFI // RFC 8950: Extended next-hop encoding
}
