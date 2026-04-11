// Design: docs/architecture/core-design.md — community filter config parsing
// Overview: filter_community.go — plugin entry point
// Related: filter.go — ingress filter logic
// Related: egress.go — egress filter logic

package filter_community

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// Community type constants.
const (
	communityTypeStandard = iota
	communityTypeLarge
	communityTypeExtended
)

// communityDef holds a named community definition with pre-parsed wire bytes.
type communityDef struct {
	typ        int      // communityTypeStandard/Large/Extended
	wireValues [][]byte // Pre-built wire bytes per value (4/12/8 bytes each)
}

// communityDefs maps community names to their definitions.
type communityDefs map[string]*communityDef

// filterConfig holds per-peer community filter configuration.
type filterConfig struct {
	ingressTag   []string
	ingressStrip []string
	egressTag    []string
	egressStrip  []string
}

// parseCommunityDefinitions extracts named community definitions from the
// bgp-level config map. Returns a map of name to definition with pre-parsed
// wire bytes ready for filter operations.
func parseCommunityDefinitions(bgpCfg map[string]any) (communityDefs, error) {
	defs := make(communityDefs)

	communityBlock, ok := bgpCfg["community"].(map[string]any)
	if !ok {
		return defs, nil
	}

	// Parse each community type.
	for _, entry := range []struct {
		key     string
		typ     int
		parseFn func(string) ([]byte, error)
	}{
		{"standard", communityTypeStandard, parseStandardWire},
		{"large", communityTypeLarge, parseLargeWire},
		{"extended", communityTypeExtended, parseExtendedWire},
	} {
		typeBlock, ok := communityBlock[entry.key].(map[string]any)
		if !ok {
			continue
		}
		for name, v := range typeBlock {
			namedBlock, ok := v.(map[string]any)
			if !ok {
				continue
			}
			// `value` is a leaf-list in YANG but the config loader may pass
			// either []any (JSON round-trip), []string (ToMap() multi-value),
			// or a bare string (ToMap() single-value). Normalise via the
			// same helper used for leaf-list fields elsewhere in this file.
			valueStrs := anySliceToStrings(namedBlock["value"])
			if len(valueStrs) == 0 {
				return nil, fmt.Errorf("community %s %q: no values defined", entry.key, name)
			}
			def := &communityDef{typ: entry.typ}
			for _, s := range valueStrs {
				wire, err := entry.parseFn(s)
				if err != nil {
					return nil, fmt.Errorf("community %s %q value %q: %w", entry.key, name, s, err)
				}
				def.wireValues = append(def.wireValues, wire)
			}
			if _, exists := defs[name]; exists {
				return nil, fmt.Errorf("community name %q defined in multiple type blocks", name)
			}
			defs[name] = def
		}
	}

	return defs, nil
}

// validateCommunityRefs checks that all referenced community names exist in defs.
func validateCommunityRefs(defs communityDefs, refs []string) error {
	for _, name := range refs {
		if _, ok := defs[name]; !ok {
			return fmt.Errorf("undefined community name %q", name)
		}
	}
	return nil
}

// parseFilterConfig extracts the filter tag/strip lists from a peer config map.
func parseFilterConfig(peerCfg map[string]any) filterConfig {
	var fc filterConfig

	filterBlock, ok := peerCfg["filter"].(map[string]any)
	if !ok {
		return fc
	}

	if ingress, ok := filterBlock["ingress"].(map[string]any); ok {
		if community, ok := ingress["community"].(map[string]any); ok {
			fc.ingressTag = anySliceToStrings(community["tag"])
			fc.ingressStrip = anySliceToStrings(community["strip"])
		}
	}

	if egress, ok := filterBlock["egress"].(map[string]any); ok {
		if community, ok := egress["community"].(map[string]any); ok {
			fc.egressTag = anySliceToStrings(community["tag"])
			fc.egressStrip = anySliceToStrings(community["strip"])
		}
	}

	return fc
}

// mergeFilterConfigs accumulates filter tag/strip lists from a more-specific
// config level into the base. Mirrors ze:cumulative semantics: lists are
// appended, not replaced.
func mergeFilterConfigs(base, overlay filterConfig) filterConfig {
	return filterConfig{
		ingressTag:   appendUnique(base.ingressTag, overlay.ingressTag),
		ingressStrip: appendUnique(base.ingressStrip, overlay.ingressStrip),
		egressTag:    appendUnique(base.egressTag, overlay.egressTag),
		egressStrip:  appendUnique(base.egressStrip, overlay.egressStrip),
	}
}

// appendUnique appends items from b to a, skipping duplicates.
func appendUnique(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	seen := make(map[string]bool, len(a))
	for _, s := range a {
		seen[s] = true
	}
	result := append([]string{}, a...)
	for _, s := range b {
		if !seen[s] {
			result = append(result, s)
			seen[s] = true
		}
	}
	return result
}

// anySliceToStrings converts a value to []string.
// Handles: []any (from JSON round-trip), []string (from ToMap() multi-values),
// and bare string (from ToMap() single-value).
func anySliceToStrings(v any) []string {
	switch s := v.(type) {
	case []any:
		result := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	case []string:
		if len(s) == 0 {
			return nil
		}
		return s
	case string:
		if s == "" {
			return nil
		}
		return []string{s}
	}
	return nil
}

// parseStandardWire parses a standard community string (ASN:value) to 4-byte wire format.
// Format: "ASN:value" where ASN is upper 16 bits and value is lower 16 bits.
func parseStandardWire(s string) ([]byte, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid standard community %q (expected ASN:value)", s)
	}
	asn, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid ASN in community %q: %w", s, err)
	}
	val, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid value in community %q: %w", s, err)
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(asn)<<16|uint32(val))
	return buf, nil
}

// parseLargeWire parses a large community string (GA:LD1:LD2) to 12-byte wire format.
func parseLargeWire(s string) ([]byte, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid large community %q (expected GA:LD1:LD2)", s)
	}
	buf := make([]byte, 12)
	for i, part := range parts {
		v, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid field %d in large community %q: %w", i, s, err)
		}
		binary.BigEndian.PutUint32(buf[i*4:], uint32(v))
	}
	return buf, nil
}

// parseExtendedWire parses an extended community string to 8-byte wire format.
// Supports hex format (16 hex chars) and target:ASN:NN / origin:ASN:NN.
func parseExtendedWire(s string) ([]byte, error) {
	// Try hex format first (16 hex digits).
	if len(s) == 16 {
		if b, err := hex.DecodeString(s); err == nil && len(b) == 8 {
			return b, nil
		}
	}
	// Try 0x prefix hex.
	if strings.HasPrefix(s, "0x") && len(s) == 18 {
		if b, err := hex.DecodeString(s[2:]); err == nil && len(b) == 8 {
			return b, nil
		}
	}
	// Try target:ASN:NN or origin:ASN:NN.
	parts := strings.SplitN(s, ":", 3)
	if len(parts) == 3 && (parts[0] == "target" || parts[0] == "origin") {
		var subtype byte
		if parts[0] == "target" {
			subtype = 0x02
		} else {
			subtype = 0x03
		}
		asn, err := strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid ASN in ext community %q: %w", s, err)
		}
		val, err := strconv.ParseUint(parts[2], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid value in ext community %q: %w", s, err)
		}
		buf := make([]byte, 8)
		buf[0] = 0x00 // Transitive 2-byte AS
		buf[1] = subtype
		binary.BigEndian.PutUint16(buf[2:4], uint16(asn))
		binary.BigEndian.PutUint32(buf[4:8], uint32(val))
		return buf, nil
	}
	return nil, fmt.Errorf("unsupported extended community format %q", s)
}
