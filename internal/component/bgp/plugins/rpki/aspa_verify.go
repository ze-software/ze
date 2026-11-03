// Design: docs/architecture/plugin/rib-storage-design.md -- ASPA upstream path verification
// Overview: rpki.go -- plugin calling verification on received UPDATEs
// Related: aspa_cache.go -- ASPA cache providing check_pair lookups
package rpki

import (
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// ASPA validation states.
// draft-ietf-sidrops-aspa-verification-28 Section 5.
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

// normalizeASPath builds COMPRESSED_AS_PATH in wire order (neighbor to origin).
// Section 5.2 removes consecutive duplicates only. Sets cannot be ordered, and
// confederation segments must not escape onto the external sessions verified here.
func normalizeASPath(segments []attribute.ASPathSegment) ([]uint32, bool) {
	var hops []uint32

	for _, seg := range segments {
		switch seg.Type {
		case attribute.ASSet, attribute.ASConfedSet:
			return nil, true
		case attribute.ASConfedSequence:
			return nil, true
		case attribute.ASSequence:
			for _, asn := range seg.ASNs {
				if len(hops) == 0 || hops[len(hops)-1] != asn {
					hops = append(hops, asn)
				}
			}
		default:
			// Segment types are open input; retain the existing skip for unknown types.
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
// draft-ietf-sidrops-aspa-verification-28 Section 5.5.
//
// Input: normalized unique-hop list [neighbor, ..., origin] and ASPA cache.
// Output: ASPAValid, ASPAInvalid, or ASPAUnknown.
//
// Walk from neighbor toward origin. For each adjacent pair (path[i], path[i+1]),
// path[i+1] is the customer, path[i] is the provider candidate.
func verifyASPA(cache *aSPACache, path []uint32) uint8 {
	// Section 5.5 step 1: "If the AS_PATH is empty, then the procedure halts
	// with the outcome 'Invalid'."
	if len(path) == 0 {
		return ASPAInvalid
	}
	if len(path) <= 1 {
		return ASPAValid
	}

	hasUnknown := false

	for i := range len(path) - 1 {
		providerCandidate := path[i]
		customerAS := path[i+1]

		switch cache.checkPair(providerCandidate, customerAS) {
		case HopProviderPlus:
			// Authorized, continue.
		case HopNotProviderPlus:
			return ASPAInvalid
		case HopNoAttestation:
			hasUnknown = true
		default:
			panic("BUG: invalid ASPA hop result")
		}
	}

	if hasUnknown {
		return ASPAUnknown
	}
	return ASPAValid
}

// aspaAppliesTo reports whether ASPA verification runs on routes of fam.
//
// draft-ietf-sidrops-aspa-verification Section 6.2: "The verification procedures
// described in this document MUST be applied to BGP routes with {AFI, SAFI}
// combinations {AFI 1 (IPv4), SAFI 1} and {AFI 2 (IPv6), SAFI 1}" and "The
// procedures MUST NOT be applied to other address families by default." So a
// route of any other family carries aspaStateNone: it is neither verified nor
// tracked for re-validation, and no ASPA policy action can exclude it.
func aspaAppliesTo(fam family.Family) bool {
	if fam.SAFI != family.SAFIUnicast {
		return false
	}
	return fam.AFI == family.AFIIPv4 || fam.AFI == family.AFIIPv6
}

// aspaMode is the algorithm selected from our configured BGP role.
type aspaMode uint8

const (
	aspaModeUnspecified aspaMode = iota
	aspaUpstream
	aspaDownstream
)

// aspaNeighbor carries the neighbor-AS step of Sections 5.5 and 5.6 into a
// verification. The receive session performs that step for every other eBGP
// peer, on the reconstructed path (RFC 4271 Section 6.3 with RFC 7606
// handling), so a route that reaches ASPA from one has passed it. A peer whose
// AS is AS_TRANS is the exception: on a two-octet session the session compares
// AS_TRANS with the AS_PATH as received (RFC 6793 Section 4.2.2), which says
// nothing about the reconstructed path ASPA verifies, because the peer chose
// that path's first AS in its own AS4_PATH. Section 5.1: "If a BGP speaker
// applies ASPA procedures to an AS_PATH without first enforcing these checks,
// any route that fails either condition (i.e., it contains an AS_SET or has a
// mismatched neighbor ASN) is evaluated as Invalid by the ASPA verification
// procedures (Section 5.5 and Section 5.6)." So for that peer ASPA performs the
// step itself.
type aspaNeighbor struct {
	as uint32
	// check is false where the receive session already confirmed the
	// neighbor, and for a local RS-client, which Section 5.5 step 2 exempts.
	check bool
}

// newASPANeighbor names the neighbor AS a route was received from and whether
// ASPA owes the neighbor-AS step for it. localRSClient is the configured local
// role; the receive session exempts the same role.
func newASPANeighbor(peerAS uint32, localRSClient bool) aspaNeighbor {
	if localRSClient {
		// Section 5.5 step 2: "If the receiving AS is not an RS-client and the
		// most recently added AS in the AS_PATH does not match the neighbor AS,
		// then the procedure halts with the outcome "Invalid"."
		return aspaNeighbor{as: peerAS}
	}
	return aspaNeighbor{as: peerAS, check: peerAS == attribute.ASTrans}
}

// verify applies the neighbor-AS step to state, the outcome of the remaining
// steps over the compressed path. Every step that fails halts with Invalid, so
// the order of the steps does not change the outcome.
func (n aspaNeighbor) verify(state uint8, path []uint32) uint8 {
	if !n.check {
		return state
	}
	// Sections 5.5 and 5.6 step 1: "If the AS_PATH is empty, then the
	// procedure halts with the outcome "Invalid"."
	if len(path) == 0 {
		return ASPAInvalid
	}
	// Section 5.6 step 2: "If the most recently added AS in the AS_PATH does
	// not match the neighbor AS, then the procedure halts with the outcome
	// "Invalid"."
	if path[0] != n.as {
		return ASPAInvalid
	}
	return state
}

// aspaStateForPath applies Section 5's structural checks before measuring ramps.
// The neighbor-AS step is the caller's, through aspaNeighbor.verify.
func aspaStateForPath(cache *aSPACache, segments []attribute.ASPathSegment, mode aspaMode) (uint8, []uint32) {
	normalizedPath, hasASSet := normalizeASPath(segments)
	// Sections 5.5 and 5.6 step 3: "If the AS_PATH has an AS_SET, then the
	// procedure halts with the outcome 'Invalid'."
	if hasASSet {
		return ASPAInvalid, normalizedPath
	}
	return verifyASPAPath(cache, normalizedPath, mode), normalizedPath
}

// verifyASPAPath selects the relationship-specific algorithm. An unspecified
// relationship supplies no basis for choosing an upstream or downstream ramp.
func verifyASPAPath(cache *aSPACache, path []uint32, mode aspaMode) uint8 {
	if len(path) == 0 {
		return ASPAInvalid
	}
	switch mode {
	case aspaUpstream:
		return verifyASPA(cache, path)
	case aspaDownstream:
		return verifyASPADownstream(cache, path)
	case aspaModeUnspecified:
		return ASPAUnknown
	default:
		panic("BUG: invalid ASPA verification mode")
	}
}

// verifyASPADownstream implements Sections 5.4 and 5.6. Each ramp starts with
// one AS, even when its first hop is denied. The two ramps may meet at a single
// unverified apex hop. Only the first denial from each end bounds its ramp.
func verifyASPADownstream(cache *aSPACache, path []uint32) uint8 {
	if len(path) == 0 {
		return ASPAInvalid
	}
	upMin, upMax := len(path), len(path)
	for i := len(path) - 1; i > 0; i-- {
		hop := cache.checkPair(path[i-1], path[i])
		if hop != HopProviderPlus && upMin == len(path) {
			upMin = len(path) - i
		}
		if hop == HopNotProviderPlus {
			upMax = len(path) - i
			break
		}
	}
	downMin, downMax := len(path), len(path)
	for i := 0; i+1 < len(path); i++ {
		hop := cache.checkPair(path[i+1], path[i])
		if hop != HopProviderPlus && downMin == len(path) {
			downMin = i + 1
		}
		if hop == HopNotProviderPlus {
			downMax = i + 1
			break
		}
	}
	if upMax+downMax < len(path) {
		return ASPAInvalid
	}
	if upMin+downMin < len(path) {
		return ASPAUnknown
	}
	return ASPAValid
}
