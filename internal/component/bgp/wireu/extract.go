// Design: docs/architecture/wire/messages.md — wire UPDATE lazy parsing
// RFC: rfc/short/rfc4271.md — path attribute extraction from UPDATE
// Related: community.go — zero-copy community extraction from UPDATE wire bytes

package wireu

import (
	"github.com/ze-software/ze/internal/core/family"
)

// extractRawAttributes returns the raw path attribute bytes from an UPDATE.
// Returns the attribute bytes without the length prefix.
// Returns nil for empty attributes, error for malformed payload.
//
// RFC 4271 Section 4.3: UPDATE message format.
func extractRawAttributes(wu *WireUpdate) ([]byte, error) {
	attrs, err := wu.Attrs()
	if err != nil {
		return nil, err
	}
	if attrs == nil {
		return nil, nil
	}
	return attrs.Packed(), nil
}

// extractRawNLRI returns raw NLRI bytes for the specified family.
// For IPv4 unicast, returns NLRI from message body.
// For other families, extracts from MP_REACH_NLRI attribute.
// Returns nil if family not present, error if malformed.
//
// RFC 4271 Section 4.3: IPv4 unicast NLRI in message body.
// RFC 4760 Section 3: Other families in MP_REACH_NLRI.
func extractRawNLRI(wu *WireUpdate, fam family.Family, _ bool) ([]byte, error) {
	// IPv4 unicast uses message body NLRI field
	if fam == (family.IPv4Unicast) {
		return wu.NLRI()
	}

	// Other families use MP_REACH_NLRI attribute
	mpReach, err := wu.MPReach()
	if err != nil {
		return nil, err
	}
	if mpReach == nil {
		return nil, nil
	}

	// Check if MP_REACH matches requested family
	if mpReach.AFI() != uint16(fam.AFI) || mpReach.SAFI() != uint8(fam.SAFI) {
		return nil, nil
	}

	// Return just the NLRI portion (after next-hop and reserved byte)
	return mpReach.NLRIBytes(), nil
}

// extractRawWithdrawn returns raw withdrawn NLRI bytes for the specified family.
// For IPv4 unicast, returns withdrawn routes from message body.
// For other families, extracts from MP_UNREACH_NLRI attribute.
// Returns nil if family not present, error if malformed.
//
// RFC 4271 Section 4.3: IPv4 unicast withdrawn in message body.
// RFC 4760 Section 4: Other families in MP_UNREACH_NLRI.
func extractRawWithdrawn(wu *WireUpdate, fam family.Family, _ bool) ([]byte, error) {
	// IPv4 unicast uses message body withdrawn field
	if fam == (family.IPv4Unicast) {
		return wu.Withdrawn()
	}

	// Other families use MP_UNREACH_NLRI attribute
	mpUnreach, err := wu.MPUnreach()
	if err != nil {
		return nil, err
	}
	if mpUnreach == nil {
		return nil, nil
	}

	// Check if MP_UNREACH matches requested family
	if mpUnreach.AFI() != uint16(fam.AFI) || mpUnreach.SAFI() != uint8(fam.SAFI) {
		return nil, nil
	}

	// Return just the withdrawn NLRI portion (after AFI/SAFI)
	return mpUnreach.WithdrawnBytes(), nil
}

// ExtractAllRawNLRI extracts raw NLRI bytes for all families in the UPDATE.
// Returns a map of family -> raw NLRI bytes.
// Used for including raw-nlri in JSON output.
//
// RFC 4271/4760: Extracts from both body NLRI and MP_REACH_NLRI.
func ExtractAllRawNLRI(wu *WireUpdate) (map[family.Family][]byte, error) {
	result := make(map[family.Family][]byte)

	// Check body NLRI (IPv4 unicast)
	bodyNLRI, err := wu.NLRI()
	if err != nil {
		return nil, err
	}
	if len(bodyNLRI) > 0 {
		result[family.IPv4Unicast] = bodyNLRI
	}

	// Check MP_REACH_NLRI for other families
	mpReach, err := wu.MPReach()
	if err != nil {
		return nil, err
	}
	if mpReach != nil {
		fam := family.Family{
			AFI:  family.AFI(mpReach.AFI()),
			SAFI: family.SAFI(mpReach.SAFI()),
		}
		if nlriBytes := mpReach.NLRIBytes(); len(nlriBytes) > 0 {
			result[fam] = nlriBytes
		}
	}

	return result, nil
}
