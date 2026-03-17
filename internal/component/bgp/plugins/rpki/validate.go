// Design: docs/architecture/plugin/rib-storage-design.md — RFC 6811 origin validation
// Overview: rpki.go — plugin using validation for route decisions
// Related: roa_cache.go — ROA cache providing VRP lookups
package rpki

import (
	"encoding/binary"
	"encoding/hex"
	"net"
)

// Validation state constants (must match adj_rib_in values).
const (
	ValidationNotValidated uint8 = 0
	ValidationValid        uint8 = 1
	ValidationNotFound     uint8 = 2
	ValidationInvalid      uint8 = 3
)

// OriginNone is a sentinel value for AS_SET or empty AS_PATH.
const OriginNone uint32 = 0xFFFFFFFF

// Validate performs RFC 6811 origin validation for a prefix and origin AS.
// Returns ValidationValid, ValidationInvalid, or ValidationNotFound.
func (c *ROACache) Validate(prefix string, originAS uint32) uint8 {
	covering := c.FindCovering(prefix)
	if len(covering) == 0 {
		return ValidationNotFound
	}

	// Parse prefix to get the prefix length.
	_, ipnet, err := net.ParseCIDR(prefix)
	if err != nil {
		return ValidationNotFound
	}
	prefixLen, _ := ipnet.Mask.Size()

	// If origin is NONE (AS_SET), it can never match any VRP.
	if originAS == OriginNone {
		return ValidationInvalid
	}

	// RFC 6811: Valid if ANY covering VRP matches (ASN + maxLength).
	for _, entry := range covering {
		if uint8(prefixLen) <= entry.MaxLength && entry.ASN == originAS && entry.ASN != 0 {
			return ValidationValid
		}
	}

	// Covering VRPs exist but none matched.
	return ValidationInvalid
}

// extractOriginAS extracts the origin AS from raw path attributes hex.
// The origin AS is the rightmost AS in the final AS_SEQUENCE segment.
// Returns OriginNone for AS_SET, empty AS_PATH, or parse errors.
func extractOriginAS(rawAttrHex string) uint32 {
	if rawAttrHex == "" {
		return OriginNone
	}

	data, err := hex.DecodeString(rawAttrHex)
	if err != nil {
		return OriginNone
	}

	// Walk path attributes looking for AS_PATH (type code 2).
	offset := 0
	for offset < len(data) {
		if offset+3 > len(data) {
			break
		}
		flags := data[offset]
		typeCode := data[offset+1]
		extended := flags&0x10 != 0

		var attrLen int
		if extended {
			if offset+4 > len(data) {
				break
			}
			attrLen = int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
			offset += 4
		} else {
			attrLen = int(data[offset+2])
			offset += 3
		}

		if offset+attrLen > len(data) {
			break
		}

		if typeCode == 2 { // AS_PATH
			return originASFromASPath(data[offset : offset+attrLen])
		}

		offset += attrLen
	}

	return OriginNone
}

// originASFromASPath extracts the origin AS from AS_PATH attribute value.
// Assumes 4-byte ASN encoding (ASN4).
func originASFromASPath(data []byte) uint32 {
	if len(data) == 0 {
		return OriginNone
	}

	// Walk segments to find the last one.
	offset := 0
	var lastSegType uint8
	var lastSegASNs []uint32

	for offset < len(data) {
		if offset+2 > len(data) {
			break
		}
		segType := data[offset]
		segLen := int(data[offset+1])
		offset += 2

		segSize := segLen * 4 // 4 bytes per ASN (ASN4)
		if offset+segSize > len(data) {
			break
		}

		asns := make([]uint32, segLen)
		for i := range segLen {
			asns[i] = binary.BigEndian.Uint32(data[offset+i*4 : offset+i*4+4])
		}

		lastSegType = segType
		lastSegASNs = asns
		offset += segSize
	}

	if len(lastSegASNs) == 0 {
		return OriginNone
	}

	// AS_SEQUENCE (type 2): origin is rightmost AS.
	// AS_SET (type 1): origin is NONE per RFC 6811.
	if lastSegType == 2 {
		return lastSegASNs[len(lastSegASNs)-1]
	}

	return OriginNone
}
