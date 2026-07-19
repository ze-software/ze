// Design: docs/architecture/plugin/rib-storage-design.md -- ASPA upstream path verification
// Overview: rpki.go -- plugin calling verification on received UPDATEs
// Related: aspa_cache.go -- ASPA cache providing check_pair lookups
package rpki

import "codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"

// ASPA validation states.
// draft-ietf-sidrops-aspa-verification Section 6.
const (
	ASPAValid     uint8 = 0
	ASPAInvalid   uint8 = 1
	ASPAUnknown   uint8 = 2
	aspaStateNone uint8 = 255 // sentinel: ASPA not active, omit from event JSON
)

// aspaStateString returns the JSON string for an ASPA validation state.
func aspaStateString(state uint8) string {
	switch state {
	case ASPAValid:
		return "valid"
	case ASPAInvalid:
		return "invalid"
	default:
		return "unknown"
	}
}

// normalizeASPath extracts a unique hop list from AS_PATH segments for ASPA verification.
// Returns the normalized path and whether AS_SET/AS_CONFED_SET was encountered.
// draft-ietf-sidrops-aspa-verification Section 6, Step 0:
//   - Remove consecutive duplicate ASNs (prepending artifacts)
//   - Strip AS_CONFED_SEQUENCE segments (confederation-internal)
//   - Flag AS_SET or AS_CONFED_SET as unverifiable
func normalizeASPath(segments []attribute.ASPathSegment) ([]uint32, bool) {
	var hops []uint32

	for _, seg := range segments {
		switch seg.Type {
		case attribute.ASSet, attribute.ASConfedSet:
			return nil, true
		case attribute.ASConfedSequence:
			continue
		case attribute.ASSequence:
			for _, asn := range seg.ASNs {
				if len(hops) == 0 || hops[len(hops)-1] != asn {
					hops = append(hops, asn)
				}
			}
		}
	}

	return hops, false
}

// deduplicateASPath removes consecutive duplicate ASNs from a flat AS_PATH.
// Used by the JSON fallback path where segment types are unavailable.
func deduplicateASPath(path []uint32) []uint32 {
	if len(path) == 0 {
		return nil
	}
	result := make([]uint32, 0, len(path))
	result = append(result, path[0])
	for i := 1; i < len(path); i++ {
		if path[i] != path[i-1] {
			result = append(result, path[i])
		}
	}
	return result
}

// verifyASPA runs the upstream path verification algorithm.
// draft-ietf-sidrops-aspa-verification Section 6.
//
// Input: normalized unique-hop list [neighbor, ..., origin] and ASPA cache.
// Output: ASPAValid, ASPAInvalid, or ASPAUnknown.
//
// Walk from neighbor toward origin. For each adjacent pair (path[i], path[i+1]),
// path[i+1] is the customer, path[i] is the provider candidate.
func verifyASPA(cache *ASPACache, path []uint32) uint8 {
	if len(path) <= 1 {
		return ASPAValid
	}

	hasUnknown := false

	for i := range len(path) - 1 {
		providerCandidate := path[i]
		customerAS := path[i+1]

		switch cache.CheckPair(providerCandidate, customerAS) {
		case HopProviderPlus:
			// Authorized, continue.
		case HopNotProviderPlus:
			return ASPAInvalid
		case HopNoAttestation:
			hasUnknown = true
		}
	}

	if hasUnknown {
		return ASPAUnknown
	}
	return ASPAValid
}

// aspaStateForPath maps a received route's AS_PATH segments to an ASPA validation state.
// draft-ietf-sidrops-aspa-verification Section 6: an AS_SET (or AS_CONFED_SET) makes the
// path unverifiable and yields Unknown; otherwise the normalized unique-hop list is run
// through the upstream verification algorithm. This is the entry point handleStructuredUpdate
// uses to verify received customer and lateral-peer routes. Returns the state and the
// normalized path (retained by the caller for re-validation tracking).
func aspaStateForPath(cache *ASPACache, segments []attribute.ASPathSegment) (uint8, []uint32) {
	normalizedPath, hasASSet := normalizeASPath(segments)
	if hasASSet {
		return ASPAUnknown, normalizedPath
	}
	return verifyASPA(cache, normalizedPath), normalizedPath
}
