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

	// Future fields for other capabilities:
	// ASN4 bool           // RFC 6793: 4-byte AS numbers in AS_PATH
	// ExtendedNextHop AFI // RFC 8950: Extended next-hop encoding
}
