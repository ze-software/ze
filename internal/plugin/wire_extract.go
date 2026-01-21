package plugin

import (
	"encoding/binary"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/bgp/nlri"
)

// ExtractRawAttributes returns the raw path attribute bytes from an UPDATE.
// Returns the attribute bytes without the length prefix.
// Returns nil for empty attributes, error for malformed payload.
//
// RFC 4271 Section 4.3: UPDATE message format.
func ExtractRawAttributes(wu *WireUpdate) ([]byte, error) {
	attrs, err := wu.Attrs()
	if err != nil {
		return nil, err
	}
	if attrs == nil {
		return nil, nil
	}
	return attrs.Packed(), nil
}

// ExtractRawNLRI returns raw NLRI bytes for the specified family.
// For IPv4 unicast, returns NLRI from message body.
// For other families, extracts from MP_REACH_NLRI attribute.
// Returns nil if family not present, error if malformed.
//
// RFC 4271 Section 4.3: IPv4 unicast NLRI in message body.
// RFC 4760 Section 3: Other families in MP_REACH_NLRI.
func ExtractRawNLRI(wu *WireUpdate, family nlri.Family, _ bool) ([]byte, error) {
	// IPv4 unicast uses message body NLRI field
	if family == nlri.IPv4Unicast {
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
	if mpReach.AFI() != uint16(family.AFI) || mpReach.SAFI() != uint8(family.SAFI) {
		return nil, nil
	}

	// Return just the NLRI portion (after next-hop and reserved byte)
	return mpReach.NLRIBytes(), nil
}

// ExtractRawWithdrawn returns raw withdrawn NLRI bytes for the specified family.
// For IPv4 unicast, returns withdrawn routes from message body.
// For other families, extracts from MP_UNREACH_NLRI attribute.
// Returns nil if family not present, error if malformed.
//
// RFC 4271 Section 4.3: IPv4 unicast withdrawn in message body.
// RFC 4760 Section 4: Other families in MP_UNREACH_NLRI.
func ExtractRawWithdrawn(wu *WireUpdate, family nlri.Family, _ bool) ([]byte, error) {
	// IPv4 unicast uses message body withdrawn field
	if family == nlri.IPv4Unicast {
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
	if mpUnreach.AFI() != uint16(family.AFI) || mpUnreach.SAFI() != uint8(family.SAFI) {
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
func ExtractAllRawNLRI(wu *WireUpdate) (map[nlri.Family][]byte, error) {
	result := make(map[nlri.Family][]byte)

	// Check body NLRI (IPv4 unicast)
	bodyNLRI, err := wu.NLRI()
	if err != nil {
		return nil, err
	}
	if len(bodyNLRI) > 0 {
		result[nlri.IPv4Unicast] = bodyNLRI
	}

	// Check MP_REACH_NLRI for other families
	mpReach, err := wu.MPReach()
	if err != nil {
		return nil, err
	}
	if mpReach != nil {
		family := nlri.Family{
			AFI:  nlri.AFI(mpReach.AFI()),
			SAFI: nlri.SAFI(mpReach.SAFI()),
		}
		if nlriBytes := mpReach.NLRIBytes(); len(nlriBytes) > 0 {
			result[family] = nlriBytes
		}
	}

	return result, nil
}

// ExtractAllRawWithdrawn extracts raw withdrawn NLRI bytes for all families.
// Returns a map of family -> raw withdrawn NLRI bytes.
//
// RFC 4271/4760: Extracts from both body withdrawn and MP_UNREACH_NLRI.
func ExtractAllRawWithdrawn(wu *WireUpdate) (map[nlri.Family][]byte, error) {
	result := make(map[nlri.Family][]byte)

	// Check body withdrawn (IPv4 unicast)
	bodyWithdrawn, err := wu.Withdrawn()
	if err != nil {
		return nil, err
	}
	if len(bodyWithdrawn) > 0 {
		result[nlri.IPv4Unicast] = bodyWithdrawn
	}

	// Check MP_UNREACH_NLRI for other families
	mpUnreach, err := wu.MPUnreach()
	if err != nil {
		return nil, err
	}
	if mpUnreach != nil {
		family := nlri.Family{
			AFI:  nlri.AFI(mpUnreach.AFI()),
			SAFI: nlri.SAFI(mpUnreach.SAFI()),
		}
		if wdBytes := mpUnreach.WithdrawnBytes(); len(wdBytes) > 0 {
			result[family] = wdBytes
		}
	}

	return result, nil
}

// RawUpdateComponents holds extracted wire bytes from an UPDATE.
// Used for efficient pool-based storage.
type RawUpdateComponents struct {
	// Attributes is the raw path attributes (without MP_REACH/UNREACH).
	// Pool-friendly: same bytes = same handle.
	Attributes []byte

	// NLRI per family (includes IPv4 body NLRI and MP_REACH families).
	NLRI map[nlri.Family][]byte

	// Withdrawn per family (includes IPv4 body withdrawn and MP_UNREACH families).
	Withdrawn map[nlri.Family][]byte
}

// ExtractRawComponents extracts all wire components from an UPDATE.
// Attributes exclude MP_REACH/MP_UNREACH (those are in NLRI/Withdrawn maps).
// This is the preferred method for pool-based RIB storage.
func ExtractRawComponents(wu *WireUpdate) (*RawUpdateComponents, error) {
	result := &RawUpdateComponents{
		NLRI:      make(map[nlri.Family][]byte),
		Withdrawn: make(map[nlri.Family][]byte),
	}

	// Extract attributes (excluding MP_REACH/MP_UNREACH for separate handling)
	attrs, err := wu.Attrs()
	if err != nil {
		return nil, fmt.Errorf("extract attrs: %w", err)
	}
	if attrs != nil {
		// Filter out MP_REACH and MP_UNREACH - they're in NLRI/Withdrawn maps
		result.Attributes = filterMPAttributes(attrs)
	}

	// Extract all NLRI
	nlriMap, err := ExtractAllRawNLRI(wu)
	if err != nil {
		return nil, fmt.Errorf("extract nlri: %w", err)
	}
	result.NLRI = nlriMap

	// Extract all withdrawn
	wdMap, err := ExtractAllRawWithdrawn(wu)
	if err != nil {
		return nil, fmt.Errorf("extract withdrawn: %w", err)
	}
	result.Withdrawn = wdMap

	return result, nil
}

// filterMPAttributes returns attributes without MP_REACH_NLRI and MP_UNREACH_NLRI.
// These are excluded because NLRI is stored separately per-family.
func filterMPAttributes(attrs *attribute.AttributesWire) []byte {
	packed := attrs.Packed()
	if len(packed) == 0 {
		return nil
	}

	// Scan for MP_REACH (14) and MP_UNREACH (15) to check if filtering needed
	hasMPAttrs := false
	offset := 0
	for offset < len(packed) {
		if offset+2 > len(packed) {
			break
		}
		flags := packed[offset]
		code := packed[offset+1]
		if code == byte(attribute.AttrMPReachNLRI) || code == byte(attribute.AttrMPUnreachNLRI) {
			hasMPAttrs = true
			break
		}

		// Skip this attribute
		lenOffset := offset + 2
		if flags&0x10 != 0 { // Extended length
			if lenOffset+2 > len(packed) {
				break
			}
			attrLen := int(binary.BigEndian.Uint16(packed[lenOffset:]))
			offset = lenOffset + 2 + attrLen
		} else {
			if lenOffset+1 > len(packed) {
				break
			}
			attrLen := int(packed[lenOffset])
			offset = lenOffset + 1 + attrLen
		}
	}

	// No MP attributes - return original
	if !hasMPAttrs {
		return packed
	}

	// Build new attribute bytes excluding MP_REACH/MP_UNREACH
	result := make([]byte, 0, len(packed))
	offset = 0
	for offset < len(packed) {
		if offset+2 > len(packed) {
			break
		}
		flags := packed[offset]
		code := packed[offset+1]

		// Calculate attribute end
		lenOffset := offset + 2
		var attrLen int
		var attrEnd int
		if flags&0x10 != 0 { // Extended length
			if lenOffset+2 > len(packed) {
				break
			}
			attrLen = int(binary.BigEndian.Uint16(packed[lenOffset:]))
			attrEnd = lenOffset + 2 + attrLen
		} else {
			if lenOffset+1 > len(packed) {
				break
			}
			attrLen = int(packed[lenOffset])
			attrEnd = lenOffset + 1 + attrLen
		}

		// Include if not MP_REACH or MP_UNREACH
		if code != byte(attribute.AttrMPReachNLRI) && code != byte(attribute.AttrMPUnreachNLRI) {
			result = append(result, packed[offset:attrEnd]...)
		}

		offset = attrEnd
	}

	return result
}
