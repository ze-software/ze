// Design: docs/architecture/route-types.md — labeled unicast route parsing
// Overview: route.go — core route types and attribute parsing

//nolint:goconst // Many string literals are intentional for BGP protocol keywords
package route

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
)

// ErrMissingLabel is returned when label is required but not provided.
var ErrMissingLabel = errors.New("missing label")

// ErrInvalidLabel is returned when label value is out of range.
var ErrInvalidLabel = errors.New("invalid label")

// MaxMPLSLabel is the maximum valid MPLS label value (20 bits).
const MaxMPLSLabel = 1048575

// validateLabel validates MPLS label value (20-bit, 0-1048575).
func validateLabel(label uint32) error {
	if label > MaxMPLSLabel {
		return fmt.Errorf("%w: must be 0-%d, got %d", ErrInvalidLabel, MaxMPLSLabel, label)
	}
	return nil
}

// parseLabels parses MPLS label(s) from args.
// Supports single value or bracketed list: "100" or "[100 200 300]" or "[100,200]".
func parseLabels(args []string) ([]uint32, int, error) {
	if len(args) == 0 {
		return nil, 0, ErrMissingLabel
	}

	tokens, consumed := attribute.ParseBracketedList(args)
	if len(tokens) == 0 {
		return nil, consumed, ErrMissingLabel
	}

	labels := make([]uint32, 0, len(tokens))
	for _, tok := range tokens {
		val, err := strconv.ParseUint(tok, 10, 32)
		if err != nil {
			return nil, consumed, fmt.Errorf("%w: '%s'", ErrInvalidLabel, tok)
		}
		label := uint32(val)
		if err := validateLabel(label); err != nil {
			return nil, consumed, err
		}
		labels = append(labels, label)
	}

	return labels, consumed, nil
}

// parseLabeledUnicastAttributes parses MPLS labeled unicast route attributes.
// Args format: <prefix> [keyword value]...
// Supports MPLSKeywords: label plus all unicast keywords (no RD/RT).
func parseLabeledUnicastAttributes(args []string) (bgptypes.LabeledUnicastRoute, error) {
	if len(args) < 1 {
		return bgptypes.LabeledUnicastRoute{}, ErrMissingPrefix
	}

	// Parse prefix (first arg)
	prefix, err := netip.ParsePrefix(args[0])
	if err != nil {
		return bgptypes.LabeledUnicastRoute{}, fmt.Errorf("%w: %s", ErrInvalidPrefix, args[0])
	}

	route := bgptypes.LabeledUnicastRoute{
		Prefix: prefix,
	}

	// Use wire-first Builder for attribute parsing
	builder := attribute.NewBuilder()

	// Parse remaining args as key-value pairs
	for i := 1; i < len(args); i++ {
		key := strings.ToLower(args[i])

		// Validate keyword against MPLS keywords (not VPN - no RD/RT)
		if !MPLSKeywords[key] {
			return bgptypes.LabeledUnicastRoute{}, fmt.Errorf("%w: '%s' not valid for labeled-unicast", ErrInvalidKeyword, key)
		}

		// Try common attribute parsing with Builder (wire-first)
		consumed, err := parseCommonAttributeBuilder(key, args, i, builder)
		if err != nil {
			return bgptypes.LabeledUnicastRoute{}, err
		}
		if consumed > 0 {
			i += consumed
			continue
		}

		// Handle MPLS-specific keywords
		switch key {
		case "label":
			if i+1 >= len(args) {
				return bgptypes.LabeledUnicastRoute{}, ErrMissingLabel
			}
			labels, consumed, err := parseLabels(args[i+1:])
			if err != nil {
				return bgptypes.LabeledUnicastRoute{}, err
			}
			route.Labels = labels
			i += consumed

		case "next-hop":
			if i+1 >= len(args) {
				return bgptypes.LabeledUnicastRoute{}, ErrMissingNextHop
			}
			nh, err := netip.ParseAddr(args[i+1])
			if err != nil {
				return bgptypes.LabeledUnicastRoute{}, fmt.Errorf("%w: %s", ErrInvalidNextHop, args[i+1])
			}
			route.NextHop = nh
			i++

		case "split":
			// Just skip - split is handled by caller (announceLabeledUnicastImpl)
			if i+1 < len(args) {
				i++
			}

		case "path-id":
			// RFC 7911 ADD-PATH identifier
			if i+1 >= len(args) {
				return bgptypes.LabeledUnicastRoute{}, fmt.Errorf("missing path-id value")
			}
			var pathID uint64
			pathID, err = strconv.ParseUint(args[i+1], 10, 32)
			if err != nil {
				return bgptypes.LabeledUnicastRoute{}, fmt.Errorf("invalid path-id: %s", args[i+1])
			}
			route.PathID = uint32(pathID)
			i++
		}
	}

	// Build wire-format attributes
	wireBytes := builder.Build()
	if len(wireBytes) > 0 {
		route.Wire = attribute.NewAttributesWire(wireBytes, context.APIContextID)
	}

	return route, nil
}

// ParseLabeledUnicastAttributes parses labeled unicast (nlri-mpls) command arguments.
// Exported for use by encode command.
// Format: <prefix> next-hop <addr> label <label> [attributes...].
func ParseLabeledUnicastAttributes(args []string) (bgptypes.LabeledUnicastRoute, error) {
	return parseLabeledUnicastAttributes(args)
}
