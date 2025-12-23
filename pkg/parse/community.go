// Package parse provides shared parsing functions for BGP attributes.
// Used by both api and config packages to avoid duplication.
package parse

import (
	"fmt"
	"strconv"
	"strings"
)

// Well-known community values per RFC 1997 and RFC 7999.
const (
	CommunityNoExport          uint32 = 0xFFFFFF01 // RFC 1997
	CommunityNoAdvertise       uint32 = 0xFFFFFF02 // RFC 1997
	CommunityNoExportSubconfed uint32 = 0xFFFFFF03 // RFC 1997
	CommunityNoPeer            uint32 = 0xFFFFFF04 // RFC 3765
	CommunityBlackhole         uint32 = 0xFFFF029A // RFC 7999
)

// Community parses a single standard community string to uint32.
// RFC 1997: COMMUNITIES attribute.
//
// Supports:
//   - ASN:VAL format per RFC 1997
//   - Well-known names: no-export, no-advertise, no-export-subconfed, nopeer, blackhole
func Community(s string) (uint32, error) {
	// Check for well-known community names (case-insensitive)
	switch strings.ToLower(s) {
	case "no-export", "no_export":
		return CommunityNoExport, nil
	case "no-advertise", "no_advertise":
		return CommunityNoAdvertise, nil
	case "no-export-subconfed", "no_export_subconfed":
		return CommunityNoExportSubconfed, nil
	case "nopeer", "no-peer", "no_peer":
		return CommunityNoPeer, nil
	case "blackhole":
		return CommunityBlackhole, nil
	}

	// Parse ASN:Value format
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid community %q: expected ASN:Value format or well-known name", s)
	}

	asn, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid community ASN %q", parts[0])
	}
	val, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid community value %q", parts[1])
	}

	return uint32(asn)<<16 | uint32(val), nil
}

// LargeCommunity parses a single large community GA:LD1:LD2 to [3]uint32.
// RFC 8092: LARGE_COMMUNITIES attribute.
func LargeCommunity(s string) ([3]uint32, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return [3]uint32{}, fmt.Errorf("invalid large-community %q: expected GA:LD1:LD2 format", s)
	}

	ga, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return [3]uint32{}, fmt.Errorf("invalid large-community global-admin %q", parts[0])
	}
	ld1, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return [3]uint32{}, fmt.Errorf("invalid large-community local-data1 %q", parts[1])
	}
	ld2, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return [3]uint32{}, fmt.Errorf("invalid large-community local-data2 %q", parts[2])
	}

	return [3]uint32{uint32(ga), uint32(ld1), uint32(ld2)}, nil
}
