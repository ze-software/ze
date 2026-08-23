// Design: docs/architecture/core-design.md — community filter config parsing
// RFC: rfc/short/rfc8195.md — Section 3.2, the relation function number leaf
// RFC: rfc/short/rfc7454.md — Section 11, the scrub and its keep-list leaves
// RFC: rfc/short/rfc7999.md — Section 3.2, the blackhole propagation leaf
// Overview: filter_community.go — plugin entry point
// Related: filter.go — ingress filter logic
// Related: egress.go — egress filter logic
// Related: scrub.go — the keep-list consumer
// Related: blackhole.go — the propagation guard consumer

package filter_community

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/configvalue"
)

// Community type constants.
const (
	communityTypeStandard = iota
	communityTypeLarge
	communityTypeExtended
)

// communityDef holds a named community definition with pre-parsed wire
// bytes.
type communityDef struct {
	typ        int      // communityTypeStandard/Large/Extended
	wireValues [][]byte // Pre-built wire bytes per value (4/12/8 bytes each)
}

// communityDefs maps community names to their definitions.
type communityDefs map[string]*communityDef

// filterConfig holds per-peer community filter configuration.
//
// The named-set lists are ze:cumulative, so a more specific level APPENDS
// to a less specific one (mergeFilterConfigs). The scalars below are
// ordinary leaves. So a more specific level REPLACES a less specific one --
// and only when it is set, which is why each is a pointer. A plain zero
// value cannot express "this level said nothing". Reading one as "the
// operator turned it off" would let a group-level setting be silently
// canceled by every peer under it.
type filterConfig struct {
	ingressTag   []string
	ingressStrip []string
	egressTag    []string
	egressStrip  []string

	// RFC 8195 Section 3.2, the relation-to-origin tag written on ingress.
	relationTag      *bool
	relationFunction *uint32

	// RFC 7454 Section 11, the own-Global-Administrator scrub.
	scrubOwnGA     *bool
	scrubKeepFuncs []uint32 // ze:cumulative

	// RFC 7999 Section 3.2, the blackhole propagation guard.
	blackholePropagation *string
}

// defaultRelationFunction is the number RFC 8195 Section 3.2 gives as its
// example, and the YANG default. RFC 8195 leaves the number to each AS. It
// is therefore configuration rather than a constant. A constant would state
// a convention on the operator's behalf.
const defaultRelationFunction uint32 = 3

// relationTagEnabled reports whether the relation tag is on.
func (fc filterConfig) relationTagEnabled() bool {
	return fc.relationTag != nil && *fc.relationTag
}

// relationFunctionNumber returns the configured function number, or the
// default.
func (fc filterConfig) relationFunctionNumber() uint32 {
	if fc.relationFunction != nil {
		return *fc.relationFunction
	}
	return defaultRelationFunction
}

// scrubEnabled reports whether the RFC 7454 Section 11 scrub is on.
func (fc filterConfig) scrubEnabled() bool {
	return fc.scrubOwnGA != nil && *fc.scrubOwnGA
}

// scrubKeepSet builds the keep-list lookup, or nil when the operator listed
// nothing. Nil keeps nothing: see scrubOwnGACommunities on why an empty
// keep-list is the closed state rather than the open one.
func (fc filterConfig) scrubKeepSet() map[uint32]bool {
	if len(fc.scrubKeepFuncs) == 0 {
		return nil
	}
	set := make(map[uint32]bool, len(fc.scrubKeepFuncs))
	for _, fn := range fc.scrubKeepFuncs {
		set[fn] = true
	}
	return set
}

// blackholeGuardToken returns the configured propagation community, or
// "none".
func (fc filterConfig) blackholeGuardToken() string {
	if fc.blackholePropagation != nil {
		return *fc.blackholePropagation
	}
	return blackholeGuardNone
}

// scrubRelationFunction is the function number the scrub must never keep.
// It is the relation function when relation tagging is on for this peer,
// and 0 otherwise: with the tag off there is no Ze-written relation value
// to protect. Reserving the number anyway would scrub an operator's own use
// of it.
func (fc filterConfig) scrubRelationFunction() uint32 {
	if !fc.relationTagEnabled() {
		return 0
	}
	return fc.relationFunctionNumber()
}

// hasAnyRule reports whether this peer has any community filter work at
// all. A peer with no rule is skipped before any wire scan.
func (fc filterConfig) hasAnyRule() bool {
	return len(fc.ingressTag) > 0 || len(fc.ingressStrip) > 0 ||
		len(fc.egressTag) > 0 || len(fc.egressStrip) > 0 ||
		fc.relationTagEnabled() || fc.scrubEnabled() ||
		fc.blackholeGuardToken() != blackholeGuardNone
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
			valueStrs := configvalue.LeafList(namedBlock["value"])
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

// validateScrubKeepList refuses a keep-list that names the relation
// function while relation tagging is on for the same peer.
//
// The code already refuses to keep that function (scrubKeepsFunction), so
// this changes no behavior. It changes what the operator is told. A config
// that lists the relation function states a belief: a peer CAN send this
// value. The implementation contradicts that belief in silence. A line that
// looks in force and is ignored is how a security control comes to be
// trusted wrongly.
//
// It is checked here rather than through a ze:validate extension because
// the check spans two leaves. A ze:validate function receives one path and
// one value (internal/component/config/yang/validator_registry.go,
// CustomValidator) and can see no sibling. Registering one would put this
// plugin's spelling in the central validator table, which
// ai/rules/plugins.md forbids.
func validateScrubKeepList(fc filterConfig) error {
	if !fc.relationTagEnabled() {
		return nil
	}
	fn := fc.relationFunctionNumber()
	if slices.Contains(fc.scrubKeepFuncs, fn) {
		return fmt.Errorf(
			"scrub-keep-function %d is the relation-function, which a peer must never send: "+
				"remove it from the keep-list, or move relation-function to another number", fn)
	}
	return nil
}

// validateCommunityRefs checks that all referenced community names exist in
// defs.
func validateCommunityRefs(defs communityDefs, refs []string) error {
	for _, name := range refs {
		if _, ok := defs[name]; !ok {
			return fmt.Errorf("undefined community name %q", name)
		}
	}
	return nil
}

// parseFilterConfig extracts the filter tag/strip lists from a peer config
// map.
func parseFilterConfig(peerCfg map[string]any) filterConfig {
	var fc filterConfig

	filterBlock, ok := peerCfg["filter"].(map[string]any)
	if !ok {
		return fc
	}

	if ingress, ok := filterBlock["ingress"].(map[string]any); ok {
		if community, ok := ingress["community"].(map[string]any); ok {
			fc.ingressTag = configvalue.LeafList(community["tag"])
			fc.ingressStrip = configvalue.LeafList(community["strip"])
			fc.relationTag = optBool(community["relation-tag"])
			fc.relationFunction = optUint32(community["relation-function"])
			fc.scrubOwnGA = optBool(community["scrub-own-ga"])
			fc.scrubKeepFuncs = uint32List(community["scrub-keep-function"])
			fc.blackholePropagation = optString(community["blackhole-propagation"])
		}
	}

	if egress, ok := filterBlock["egress"].(map[string]any); ok {
		if community, ok := egress["community"].(map[string]any); ok {
			fc.egressTag = configvalue.LeafList(community["tag"])
			fc.egressStrip = configvalue.LeafList(community["strip"])
		}
	}

	return fc
}

// mergeFilterConfigs accumulates filter tag/strip lists from a
// more-specific config level into the base. Mirrors ze:cumulative
// semantics: lists are appended, not replaced.
//
// The scalar leaves take the opposite rule, which is the ordinary YANG one:
// the more specific level wins WHEN IT IS SET, and a level that said
// nothing leaves the inherited value standing. Treating an unset leaf as a
// value is how a group-level `scrub-own-ga true` would be canceled by every
// peer under it.
func mergeFilterConfigs(base, overlay filterConfig) filterConfig {
	return filterConfig{
		ingressTag:   appendUnique(base.ingressTag, overlay.ingressTag),
		ingressStrip: appendUnique(base.ingressStrip, overlay.ingressStrip),
		egressTag:    appendUnique(base.egressTag, overlay.egressTag),
		egressStrip:  appendUnique(base.egressStrip, overlay.egressStrip),

		relationTag:          overrideBool(base.relationTag, overlay.relationTag),
		relationFunction:     overrideUint32(base.relationFunction, overlay.relationFunction),
		scrubOwnGA:           overrideBool(base.scrubOwnGA, overlay.scrubOwnGA),
		scrubKeepFuncs:       appendUniqueUint32(base.scrubKeepFuncs, overlay.scrubKeepFuncs),
		blackholePropagation: overrideString(base.blackholePropagation, overlay.blackholePropagation),
	}
}

func overrideBool(base, overlay *bool) *bool {
	if overlay != nil {
		return overlay
	}
	return base
}

func overrideUint32(base, overlay *uint32) *uint32 {
	if overlay != nil {
		return overlay
	}
	return base
}

func overrideString(base, overlay *string) *string {
	if overlay != nil {
		return overlay
	}
	return base
}

// appendUniqueUint32 is appendUnique for the ze:cumulative keep-list.
func appendUniqueUint32(a, b []uint32) []uint32 {
	if len(b) == 0 {
		return a
	}
	seen := make(map[uint32]bool, len(a))
	for _, v := range a {
		seen[v] = true
	}
	result := append([]uint32{}, a...)
	for _, v := range b {
		if !seen[v] {
			result = append(result, v)
			seen[v] = true
		}
	}
	return result
}

// optBool reads a YANG boolean leaf that may be absent. The config
// framework delivers leaf values as strings, and a JSON round-trip delivers
// them as real booleans, so both forms are read. An unparseable value reads
// as ABSENT rather than false: absent leaves the inherited value standing,
// while false would silently override it (ai/rules/evidence.md).
func optBool(v any) *bool {
	switch b := v.(type) {
	case bool:
		return &b
	case string:
		switch b {
		case "true":
			t := true
			return &t
		case "false":
			f := false
			return &f
		}
	}
	return nil
}

// optString reads a YANG enumeration or string leaf that may be absent.
func optString(v any) *string {
	if s, ok := v.(string); ok && s != "" {
		return &s
	}
	return nil
}

// optUint32 reads a YANG uint32 leaf that may be absent, across the numeric
// shapes the config framework and a JSON round-trip both produce.
func optUint32(v any) *uint32 {
	n, ok := readUint32(v)
	if !ok {
		return nil
	}
	return &n
}

// uint32List reads a uint32 leaf-list. A value that does not parse is
// DROPPED rather than defaulted to zero: a keep-list is a security control,
// and 0 is a real function number that would then be kept without anyone
// asking for it.
func uint32List(v any) []uint32 {
	strs := configvalue.LeafList(v)
	if len(strs) > 0 {
		out := make([]uint32, 0, len(strs))
		for _, s := range strs {
			if n, err := strconv.ParseUint(s, 10, 32); err == nil {
				out = append(out, uint32(n))
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]uint32, 0, len(items))
	for _, item := range items {
		if n, ok := readUint32(item); ok {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// readUint32 converts the numeric shapes a config value can arrive in. A
// value outside the uint32 range is refused rather than truncated.
func readUint32(v any) (uint32, bool) {
	switch n := v.(type) {
	case string:
		parsed, err := strconv.ParseUint(n, 10, 32)
		if err != nil {
			return 0, false
		}
		return uint32(parsed), true
	case float64:
		if n < 0 || n > 4294967295 {
			return 0, false
		}
		return uint32(n), true //nolint:gosec // G115: bounds-checked above
	case int:
		if n < 0 || n > 4294967295 {
			return 0, false
		}
		return uint32(n), true //nolint:gosec // G115: bounds-checked above
	case int64:
		if n < 0 || n > 4294967295 {
			return 0, false
		}
		return uint32(n), true //nolint:gosec // G115: bounds-checked above
	case uint64:
		if n > 4294967295 {
			return 0, false
		}
		return uint32(n), true //nolint:gosec // G115: bounds-checked above
	case uint32:
		return n, true
	}
	return 0, false
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

// parseStandardWire parses a standard community string (ASN:value) to
// 4-byte wire format. Format: "ASN:value" where ASN is upper 16 bits and
// value is lower 16 bits.
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

// parseLargeWire parses a large community string (GA:LD1:LD2) to 12-byte
// wire format.
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

// parseExtendedWire parses an extended community string to 8-byte wire
// format. Supports hex format (16 hex chars) and target:ASN:NN /
// origin:ASN:NN.
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
