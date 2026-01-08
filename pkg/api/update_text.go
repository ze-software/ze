// Package api provides the update text parser for the "update text" command format.
//
// Grammar:
//
//	<update-text> := <section>*
//	<section>     := <scalar-attr> | <list-attr> | <nlri-section> | <wire-attr>
//
//	<scalar-attr> := <scalar-name> (set <value> | del [<value>])
//	<scalar-name> := origin | med | local-preference | nhop | path-information | rd | label
//
//	<list-attr>   := <list-name> (set <list> | add <list> | del [<list>])
//	<list-name>   := as-path | community | large-community | extended-community
//
//	<nlri-section> := nlri <family> <nlri-op>+
//	<nlri-op>      := add <prefix>+ [watchdog set <name>] | del <prefix>+
//
//	<wire-attr>    := attr (set <bytes> | del [<bytes>])   // hex/b64 mode only
//
// Scalar del [<value>]: unconditional if no value, conditional if value (must match current).
// List attributes support set/add/del. Attributes accumulate; each nlri captures a snapshot.
//
// Standalone watchdog commands: watchdog announce <name>, watchdog withdraw <name>
//
// Note: rd and label are ignored for families that don't support them.
package api

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
)

// UpdateText command keywords.
const (
	kwAttr     = "attr"
	kwNLRI     = "nlri"
	kwWatchdog = "watchdog"
	kwNhop     = "nhop"             // New: top-level next-hop accumulator
	kwPathInfo = "path-information" // New: ADD-PATH path-id accumulator
)

// UpdateText action keywords.
const (
	kwAdd = "add"
	kwDel = "del"
	kwSet = "set"
)

// Attribute keywords for per-attribute syntax.
const (
	kwOrigin            = "origin"
	kwMED               = "med"
	kwLocalPref         = "local-preference"
	kwASPath            = "as-path"
	kwCommunity         = "community"
	kwLargeCommunity    = "large-community"
	kwExtendedCommunity = "extended-community"
)

// VPN/labeled NLRI accumulator keywords.
const (
	kwRD    = "rd"    // Route Distinguisher for VPN families
	kwLabel = "label" // MPLS label for VPN/labeled families
)

// strNil is the string representation of nil for error messages.
const strNil = "nil"

// isAttributeKeyword returns true if token is a per-attribute keyword.
func isAttributeKeyword(token string) bool {
	switch token {
	case kwOrigin, kwMED, kwLocalPref, kwASPath,
		kwCommunity, kwLargeCommunity, kwExtendedCommunity:
		return true
	}
	return false
}

// isScalarAttribute returns true if the attribute is scalar (set/del only, no add).
func isScalarAttribute(token string) bool {
	switch token {
	case kwOrigin, kwMED, kwLocalPref, kwRD, kwLabel:
		return true
	}
	return false
}

// isBoundaryKeyword returns true if token starts a new section.
func isBoundaryKeyword(token string) bool {
	return token == kwAttr || token == kwNLRI || token == kwWatchdog ||
		token == kwNhop || token == kwPathInfo || token == kwRD || token == kwLabel ||
		isAttributeKeyword(token)
}

// parsedAttrs tracks attribute state during parsing.
// Includes next-hop and path-id which are NOT part of PathAttributes.
// Clear* fields signal "del without value" to remove the attribute entirely.
// Del*Expected fields signal "del <value>" conditional delete (must match current).
type parsedAttrs struct {
	NextHop     netip.Addr
	NextHopSelf bool
	PathID      uint32 // ADD-PATH path identifier (0 = not set)
	PathAttributes

	// VPN/labeled NLRI accumulators
	RD     nlri.RouteDistinguisher // Route Distinguisher for VPN families
	Labels []uint32                // MPLS labels for VPN/labeled families

	// Clear flags for "del without value" - remove entire attribute
	ClearOrigin              bool
	ClearMED                 bool
	ClearLocalPref           bool
	ClearASPath              bool
	ClearCommunities         bool
	ClearLargeCommunities    bool
	ClearExtendedCommunities bool
	ClearRD                  bool
	ClearLabels              bool

	// Conditional delete expected values - only clear if current matches
	DelOriginExpected    *uint8
	DelMEDExpected       *uint32
	DelLocalPrefExpected *uint32
}

// applySet merges other into a, overwriting only fields that are set in other.
func (a *parsedAttrs) applySet(other parsedAttrs) {
	if other.NextHop.IsValid() {
		a.NextHop = other.NextHop
	}
	if other.NextHopSelf {
		a.NextHopSelf = true
	}
	if other.Origin != nil {
		a.Origin = other.Origin
	}
	if other.LocalPreference != nil {
		a.LocalPreference = other.LocalPreference
	}
	if other.MED != nil {
		a.MED = other.MED
	}
	if other.ASPath != nil {
		a.ASPath = other.ASPath
	}
	if other.Communities != nil {
		a.Communities = other.Communities
	}
	if other.LargeCommunities != nil {
		a.LargeCommunities = other.LargeCommunities
	}
	if other.ExtendedCommunities != nil {
		a.ExtendedCommunities = other.ExtendedCommunities
	}
	// VPN/labeled NLRI accumulators
	if other.RD.Type != 0 || other.RD.Value != [6]byte{} {
		a.RD = other.RD
	}
	if other.Labels != nil {
		a.Labels = other.Labels
	}
}

// validateListOp checks if other contains only list attributes.
// Returns error if scalar attrs are set. AS-PATH is treated as list.
func (a *parsedAttrs) validateListOp(other parsedAttrs, scalarErr error) error {
	if other.Origin != nil {
		return fmt.Errorf("origin: %w", scalarErr)
	}
	if other.MED != nil {
		return fmt.Errorf("med: %w", scalarErr)
	}
	if other.LocalPreference != nil {
		return fmt.Errorf("local-preference: %w", scalarErr)
	}
	if other.NextHop.IsValid() {
		return fmt.Errorf("next-hop: %w", scalarErr)
	}
	if other.NextHopSelf {
		return fmt.Errorf("next-hop-self: %w", scalarErr)
	}
	return nil
}

// applyAdd prepends list attributes from other into a.
// Returns error if non-list attributes are set in other.
func (a *parsedAttrs) applyAdd(other parsedAttrs) error {
	if err := a.validateListOp(other, ErrAddOnScalar); err != nil {
		return err
	}
	if other.ASPath != nil {
		a.ASPath = append(other.ASPath, a.ASPath...)
	}
	if other.Communities != nil {
		a.Communities = append(other.Communities, a.Communities...)
	}
	if other.LargeCommunities != nil {
		a.LargeCommunities = append(other.LargeCommunities, a.LargeCommunities...)
	}
	if other.ExtendedCommunities != nil {
		a.ExtendedCommunities = append(other.ExtendedCommunities, a.ExtendedCommunities...)
	}
	return nil
}

// applyDel removes attributes from a.
// Clear* flags remove entire attribute unconditionally.
// Del*Expected fields remove only if current value matches (conditional delete).
// List values remove specific items. Returns error if any value to delete is not found.
func (a *parsedAttrs) applyDel(other parsedAttrs) error {
	// Handle conditional scalar deletes first (del <value>)
	if other.DelOriginExpected != nil {
		if a.Origin == nil || *a.Origin != *other.DelOriginExpected {
			currentVal := strNil
			if a.Origin != nil {
				currentVal = originToString(*a.Origin)
			}
			return fmt.Errorf("origin del: current value is %s, not %s", currentVal, originToString(*other.DelOriginExpected))
		}
		a.Origin = nil
	}
	if other.DelMEDExpected != nil {
		if a.MED == nil || *a.MED != *other.DelMEDExpected {
			currentVal := strNil
			if a.MED != nil {
				currentVal = fmt.Sprintf("%d", *a.MED)
			}
			return fmt.Errorf("med del: current value is %s, not %d", currentVal, *other.DelMEDExpected)
		}
		a.MED = nil
	}
	if other.DelLocalPrefExpected != nil {
		if a.LocalPreference == nil || *a.LocalPreference != *other.DelLocalPrefExpected {
			currentVal := strNil
			if a.LocalPreference != nil {
				currentVal = fmt.Sprintf("%d", *a.LocalPreference)
			}
			return fmt.Errorf("local-preference del: current value is %s, not %d", currentVal, *other.DelLocalPrefExpected)
		}
		a.LocalPreference = nil
	}

	// Handle clear flags (del without value - unconditional)
	if other.ClearOrigin {
		a.Origin = nil
	}
	if other.ClearMED {
		a.MED = nil
	}
	if other.ClearLocalPref {
		a.LocalPreference = nil
	}
	if other.ClearASPath {
		a.ASPath = nil
	}
	if other.ClearCommunities {
		a.Communities = nil
	}
	if other.ClearLargeCommunities {
		a.LargeCommunities = nil
	}
	if other.ClearExtendedCommunities {
		a.ExtendedCommunities = nil
	}
	if other.ClearRD {
		a.RD = nlri.RouteDistinguisher{}
	}
	if other.ClearLabels {
		a.Labels = nil
	}

	// Handle del with specific values (list attributes only)
	if other.ASPath != nil {
		result, notFound := removeFromSliceStrict(a.ASPath, other.ASPath)
		if len(notFound) > 0 {
			return fmt.Errorf("as-path ASN %d not found in current path", notFound[0])
		}
		a.ASPath = result
	}
	if other.Communities != nil {
		result, notFound := removeFromSliceStrict(a.Communities, other.Communities)
		if len(notFound) > 0 {
			return fmt.Errorf("community %s not found in current list", formatCommunity(notFound[0]))
		}
		a.Communities = result
	}
	if other.LargeCommunities != nil {
		result, notFound := removeFromSliceStrict(a.LargeCommunities, other.LargeCommunities)
		if len(notFound) > 0 {
			return fmt.Errorf("large-community %s not found in current list", formatLargeCommunity(notFound[0]))
		}
		a.LargeCommunities = result
	}
	if other.ExtendedCommunities != nil {
		result, notFound := removeFromSliceStrict(a.ExtendedCommunities, other.ExtendedCommunities)
		if len(notFound) > 0 {
			return fmt.Errorf("extended-community not found in current list")
		}
		a.ExtendedCommunities = result
	}
	return nil
}

// nlriAccum holds VPN/labeled NLRI accumulator values for snapshot.
type nlriAccum struct {
	PathID uint32
	RD     nlri.RouteDistinguisher
	Labels []uint32
}

// snapshot returns a deep copy of the current attribute state.
// MUST deep copy slices AND pointers to isolate each group from later modifications.
// Also returns the current NLRI accumulators (pathID, RD, labels).
func (a *parsedAttrs) snapshot() (PathAttributes, RouteNextHop, nlriAccum) {
	var pa PathAttributes
	// Deep copy pointer fields
	if a.Origin != nil {
		v := *a.Origin
		pa.Origin = &v
	}
	if a.LocalPreference != nil {
		v := *a.LocalPreference
		pa.LocalPreference = &v
	}
	if a.MED != nil {
		v := *a.MED
		pa.MED = &v
	}
	if a.ASPath != nil {
		pa.ASPath = make([]uint32, len(a.ASPath))
		copy(pa.ASPath, a.ASPath)
	}
	if a.Communities != nil {
		pa.Communities = make([]uint32, len(a.Communities))
		copy(pa.Communities, a.Communities)
	}
	if a.LargeCommunities != nil {
		pa.LargeCommunities = make([]LargeCommunity, len(a.LargeCommunities))
		copy(pa.LargeCommunities, a.LargeCommunities)
	}
	if a.ExtendedCommunities != nil {
		pa.ExtendedCommunities = make([]attribute.ExtendedCommunity, len(a.ExtendedCommunities))
		copy(pa.ExtendedCommunities, a.ExtendedCommunities)
	}
	// Convert to RouteNextHop: Self takes precedence if set
	var nh RouteNextHop
	if a.NextHopSelf {
		nh = NewNextHopSelf()
	} else if a.NextHop.IsValid() {
		nh = NewNextHopExplicit(a.NextHop)
	}
	// Deep copy labels slice
	var labels []uint32
	if a.Labels != nil {
		labels = make([]uint32, len(a.Labels))
		copy(labels, a.Labels)
	}
	return pa, nh, nlriAccum{PathID: a.PathID, RD: a.RD, Labels: labels}
}

// removeFromSliceStrict removes first instance of each element in remove from slice.
// Returns the result slice and any elements that were not found in slice.
// Empty remove list returns original slice with no errors.
// If remove contains duplicates, each removes one more instance from slice.
func removeFromSliceStrict[T comparable](slice, remove []T) ([]T, []T) {
	if len(remove) == 0 {
		return slice, nil // empty remove = no-op, no error
	}
	if len(slice) == 0 {
		return slice, remove // all items not found
	}

	// Work with a copy to track which indices are removed
	result := make([]T, len(slice))
	copy(result, slice)
	var notFound []T

	for _, r := range remove {
		found := false
		for i, v := range result {
			if v == r {
				// Remove first occurrence by shifting
				result = append(result[:i], result[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			notFound = append(notFound, r)
		}
	}

	if len(notFound) > 0 {
		return slice, notFound // return original on error
	}
	return result, nil
}

// formatCommunity formats a community uint32 as "ASN:value".
func formatCommunity(c uint32) string {
	return fmt.Sprintf("%d:%d", c>>16, c&0xFFFF)
}

// formatLargeCommunity formats a LargeCommunity as "GA:LD1:LD2".
func formatLargeCommunity(lc LargeCommunity) string {
	return fmt.Sprintf("%d:%d:%d", lc.GlobalAdmin, lc.LocalData1, lc.LocalData2)
}

// originToString converts origin value to string.
func originToString(o uint8) string {
	switch o {
	case 0:
		return "igp"
	case 1:
		return "egp"
	case 2:
		return "incomplete"
	default:
		return fmt.Sprintf("%d", o)
	}
}

// ParseUpdateText parses the "update text" command format.
// Returns the parsed result or an error.
func ParseUpdateText(args []string) (*UpdateTextResult, error) {
	var accum parsedAttrs
	var groups []NLRIGroup
	var watchdog string
	i := 0

	for i < len(args) {
		switch args[i] { //nolint:gosec // G602 false positive: loop condition guards access
		case kwAttr:
			mode, attrs, consumed, err := parseAttrSection(args[i:])
			if err != nil {
				return nil, err
			}

			switch mode {
			case kwSet:
				accum.applySet(attrs)
			case kwAdd:
				if err := accum.applyAdd(attrs); err != nil {
					return nil, err
				}
			case kwDel:
				if err := accum.applyDel(attrs); err != nil {
					return nil, err
				}
			}
			i += consumed

		case kwNhop:
			consumed, err := parseNhopSection(args[i:], &accum)
			if err != nil {
				return nil, err
			}
			i += consumed

		case kwPathInfo:
			consumed, err := parsePathInfoSection(args[i:], &accum)
			if err != nil {
				return nil, err
			}
			i += consumed

		case kwRD:
			consumed, err := parseRDSection(args[i:], &accum)
			if err != nil {
				return nil, err
			}
			i += consumed

		case kwLabel:
			consumed, err := parseLabelSection(args[i:], &accum)
			if err != nil {
				return nil, err
			}
			i += consumed

		case kwNLRI:
			attrs, nh, nlriAcc := accum.snapshot()
			family, announce, withdraw, nlriWatchdog, consumed, err := parseNLRISection(args[i:], nlriAcc)
			if err != nil {
				return nil, err
			}

			groups = append(groups, NLRIGroup{
				Family:       family,
				Announce:     announce,
				Withdraw:     withdraw,
				Attrs:        attrs,
				NextHop:      nh,
				WatchdogName: nlriWatchdog,
			})
			// Also set global watchdog if specified in nlri section (for backward compat)
			if nlriWatchdog != "" {
				watchdog = nlriWatchdog
			}
			i += consumed

		case kwWatchdog:
			// Legacy standalone watchdog - still supported but deprecated
			// New syntax: nlri ... add ... watchdog set <name>
			if i+1 >= len(args) {
				return nil, errors.New("missing watchdog name")
			}
			watchdog = args[i+1]
			i += 2

		default:
			// Check for per-attribute keywords (origin, med, community, etc.)
			if isAttributeKeyword(args[i]) { //nolint:gosec // G602 false positive: loop guards access
				mode, attrs, consumed, err := parsePerAttributeSection(args[i:])
				if err != nil {
					return nil, err
				}

				switch mode {
				case kwSet:
					accum.applySet(attrs)
				case kwAdd:
					if err := accum.applyAdd(attrs); err != nil {
						return nil, err
					}
				case kwDel:
					if err := accum.applyDel(attrs); err != nil {
						return nil, err
					}
				}
				i += consumed
				continue
			}
			return nil, fmt.Errorf("unexpected token '%s'; valid: origin, med, local-preference, as-path, community, large-community, extended-community, nhop, nlri, watchdog", args[i]) //nolint:gosec // G602 false positive: loop guards access
		}
	}

	return &UpdateTextResult{Groups: groups, WatchdogName: watchdog}, nil
}

// parseNhopSection parses nhop <set <addr>|del> section.
// Returns consumed token count and error.
func parseNhopSection(args []string, accum *parsedAttrs) (int, error) {
	// args[0] = "nhop"
	if len(args) < 2 {
		return 0, errors.New("nhop requires set or del")
	}

	switch args[1] {
	case kwSet:
		if len(args) < 3 {
			return 0, errors.New("nhop set requires data")
		}
		value := args[2]
		if value == kwSelf {
			accum.NextHopSelf = true
			accum.NextHop = netip.Addr{} // Clear any explicit address
			return 3, nil
		}
		addr, err := netip.ParseAddr(value)
		if err != nil {
			return 0, fmt.Errorf("invalid next-hop: %w", err)
		}
		accum.NextHop = addr
		accum.NextHopSelf = false
		return 3, nil

	case kwDel:
		// del without value: clear unconditionally
		// del with value: clear only if matches current (conditional delete)
		if len(args) > 2 && !isBoundaryKeyword(args[2]) {
			value := args[2]
			if value == kwSelf {
				// Conditional delete of "self"
				if !accum.NextHopSelf {
					return 0, errors.New("nhop del: current value is not self")
				}
			} else {
				// Conditional delete of specific address
				addr, err := netip.ParseAddr(value)
				if err != nil {
					return 0, fmt.Errorf("invalid next-hop: %w", err)
				}
				if accum.NextHop != addr {
					return 0, fmt.Errorf("nhop del: current value is %s, not %s", accum.NextHop, addr)
				}
			}
			accum.NextHop = netip.Addr{}
			accum.NextHopSelf = false
			return 3, nil
		}
		// Unconditional delete
		accum.NextHop = netip.Addr{}
		accum.NextHopSelf = false
		return 2, nil

	default:
		return 0, fmt.Errorf("nhop requires set or del, got: %s", args[1])
	}
}

// parsePathInfoSection parses path-information (set <id> | del [<id>]) section.
// Returns consumed token count and error.
func parsePathInfoSection(args []string, accum *parsedAttrs) (int, error) {
	// args[0] = "path-information"
	if len(args) < 2 {
		return 0, errors.New("path-information requires set or del")
	}

	switch args[1] {
	case kwSet:
		if len(args) < 3 {
			return 0, errors.New("path-information set requires id")
		}
		id, err := strconv.ParseUint(args[2], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid path-information: %w", err)
		}
		accum.PathID = uint32(id) //nolint:gosec // G115: bounded by ParseUint 32-bit
		return 3, nil

	case kwDel:
		// del without value: clear unconditionally
		// del with value: clear only if matches current (conditional delete)
		if len(args) > 2 && !isBoundaryKeyword(args[2]) {
			// Conditional delete - check if value matches
			id, err := strconv.ParseUint(args[2], 10, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid path-information: %w", err)
			}
			if accum.PathID != uint32(id) { //nolint:gosec // G115: bounded by ParseUint 32-bit
				return 0, fmt.Errorf("path-information del: current value is %d, not %d", accum.PathID, id)
			}
			accum.PathID = 0
			return 3, nil
		}
		// Unconditional delete
		accum.PathID = 0
		return 2, nil

	default:
		return 0, fmt.Errorf("path-information requires set or del, got: %s", args[1])
	}
}

// parseRDSection parses rd (set <value> | del [<value>]) section.
// RD format: ASN:NN or IP:NN (e.g., "65000:100" or "192.0.2.1:100").
// Returns consumed token count and error.
func parseRDSection(args []string, accum *parsedAttrs) (int, error) {
	// args[0] = "rd"
	if len(args) < 2 {
		return 0, errors.New("rd requires set or del")
	}

	switch args[1] {
	case kwSet:
		if len(args) < 3 {
			return 0, errors.New("rd set requires value (ASN:NN or IP:NN)")
		}
		rd, err := nlri.ParseRDString(args[2])
		if err != nil {
			return 0, fmt.Errorf("invalid rd: %w", err)
		}
		accum.RD = rd
		return 3, nil

	case kwDel:
		// del without value: clear unconditionally
		// del with value: clear only if matches current (conditional delete)
		if len(args) > 2 && !isBoundaryKeyword(args[2]) {
			// Conditional delete - check if value matches
			rd, err := nlri.ParseRDString(args[2])
			if err != nil {
				return 0, fmt.Errorf("invalid rd: %w", err)
			}
			if accum.RD != rd {
				return 0, fmt.Errorf("rd del: current value is %s, not %s", accum.RD, rd)
			}
			accum.RD = nlri.RouteDistinguisher{}
			return 3, nil
		}
		// Unconditional delete
		accum.RD = nlri.RouteDistinguisher{}
		return 2, nil

	default:
		return 0, fmt.Errorf("rd requires set or del, got: %s", args[1])
	}
}

// parseLabelSection parses label (set <value> | del [<value>]) section.
// Label is a single MPLS label value (0-1048575).
// Returns consumed token count and error.
func parseLabelSection(args []string, accum *parsedAttrs) (int, error) {
	// args[0] = "label"
	if len(args) < 2 {
		return 0, errors.New("label requires set or del")
	}

	switch args[1] {
	case kwSet:
		if len(args) < 3 {
			return 0, errors.New("label set requires value (0-1048575)")
		}
		label, err := strconv.ParseUint(args[2], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid label: %w", err)
		}
		if label > 0xFFFFF { // 20-bit max
			return 0, fmt.Errorf("label out of range (max 1048575): %d", label)
		}
		accum.Labels = []uint32{uint32(label)} //nolint:gosec // G115: bounded by check above
		return 3, nil

	case kwDel:
		// del without value: clear unconditionally
		// del with value: clear only if matches current (conditional delete)
		if len(args) > 2 && !isBoundaryKeyword(args[2]) {
			// Conditional delete - check if value matches
			label, err := strconv.ParseUint(args[2], 10, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid label: %w", err)
			}
			if len(accum.Labels) != 1 || accum.Labels[0] != uint32(label) { //nolint:gosec // G115: bounded by ParseUint
				currentStr := "[]"
				if len(accum.Labels) > 0 {
					currentStr = fmt.Sprintf("[%d]", accum.Labels[0])
				}
				return 0, fmt.Errorf("label del: current value is %s, not [%d]", currentStr, label)
			}
			accum.Labels = nil
			return 3, nil
		}
		// Unconditional delete
		accum.Labels = nil
		return 2, nil

	default:
		return 0, fmt.Errorf("label requires set or del, got: %s", args[1])
	}
}

// parseAttrSection parses attr <mode> <key> <value>... until boundary keyword.
// Returns mode, parsed attrs, consumed token count, and any error.
func parseAttrSection(args []string) (string, parsedAttrs, int, error) {
	// args[0] = "attr"
	if len(args) < 2 {
		return "", parsedAttrs{}, 0, ErrMissingAttrMode
	}
	mode := args[1]
	if mode != kwSet && mode != kwAdd && mode != kwDel {
		return "", parsedAttrs{}, 0, ErrInvalidAttrMode
	}

	consumed := 2 // "attr" + mode
	i := 2
	var attrs parsedAttrs

	for i < len(args) {
		key := args[i] //nolint:gosec // G602 false positive: loop condition guards access

		// Boundary keywords end this section
		if isBoundaryKeyword(key) {
			// attr set/add/del is for wire encoding only, not attribute keywords
			if i == 2 && isAttributeKeyword(key) {
				return "", parsedAttrs{}, 0, fmt.Errorf(
					"'attr' is for hex/b64 wire encoding; for text mode use: %s %s <value>",
					key, mode)
			}
			break
		}

		// Try parseCommonAttribute for standard attrs
		extra, err := parseCommonAttribute(key, args, i, &attrs.PathAttributes)
		if err != nil {
			return "", parsedAttrs{}, 0, err
		}
		if extra > 0 {
			i += 1 + extra // key + extra args consumed
			consumed += 1 + extra
			continue
		}

		// Unknown attribute - list valid options
		return "", parsedAttrs{}, 0, fmt.Errorf("unknown attribute '%s'; valid: origin, med, local-preference, as-path, community, large-community, extended-community", key)
	}

	return mode, attrs, consumed, nil
}

// parsePerAttributeSection parses per-attribute syntax: <attr-name> <set|add|del> [<value>]
// Returns mode, parsed attrs, consumed token count, and any error.
func parsePerAttributeSection(args []string) (string, parsedAttrs, int, error) {
	// args[0] = attribute name (origin, med, etc.)
	if len(args) < 1 {
		return "", parsedAttrs{}, 0, errors.New("missing attribute name")
	}
	if len(args) < 2 {
		return "", parsedAttrs{}, 0, fmt.Errorf("missing operation for %s", args[0])
	}

	attrName := args[0]
	mode := args[1]

	// Validate mode
	if mode != kwSet && mode != kwAdd && mode != kwDel {
		return "", parsedAttrs{}, 0, fmt.Errorf("invalid operation '%s' for %s: use set, add, or del", mode, attrName)
	}

	// Validate scalar vs list operations
	if isScalarAttribute(attrName) && mode == kwAdd {
		return "", parsedAttrs{}, 0, fmt.Errorf("%s: %w", attrName, ErrAddOnScalar)
	}

	// AS-PATH supports set/add/del:
	// - set: replace entire path
	// - add: prepend ASN(s) to path
	// - del: clear entire path (no value) or remove specific ASN (with value)

	consumed := 2 // attr-name + mode
	var attrs parsedAttrs

	// For del, check if there's a value following
	if mode == kwDel {
		hasValue := len(args) > 2 && !isBoundaryKeyword(args[2])

		if !hasValue {
			// del without value - set clear flag for this attribute (unconditional)
			switch attrName {
			case kwOrigin:
				attrs.ClearOrigin = true
			case kwMED:
				attrs.ClearMED = true
			case kwLocalPref:
				attrs.ClearLocalPref = true
			case kwASPath:
				attrs.ClearASPath = true
			case kwCommunity:
				attrs.ClearCommunities = true
			case kwLargeCommunity:
				attrs.ClearLargeCommunities = true
			case kwExtendedCommunity:
				attrs.ClearExtendedCommunities = true
			}
			return mode, attrs, consumed, nil
		}
		// hasValue is true - for scalars this is conditional delete
		if isScalarAttribute(attrName) {
			// Parse the value and set Del*Expected field
			valueArgs := append([]string{attrName}, args[2:]...)
			var tempAttrs PathAttributes
			extra, err := parseCommonAttribute(attrName, valueArgs, 0, &tempAttrs)
			if err != nil {
				return "", parsedAttrs{}, 0, err
			}
			if extra == 0 {
				return "", parsedAttrs{}, 0, fmt.Errorf("missing value for %s del", attrName)
			}
			// Copy parsed value to Del*Expected field
			switch attrName {
			case kwOrigin:
				attrs.DelOriginExpected = tempAttrs.Origin
			case kwMED:
				attrs.DelMEDExpected = tempAttrs.MED
			case kwLocalPref:
				attrs.DelLocalPrefExpected = tempAttrs.LocalPreference
			}
			return mode, attrs, consumed + extra, nil
		}
		// For list attrs, fall through to regular parsing
	}

	// Parse the value using parseCommonAttribute
	// Build args slice: [attrName, value1, value2, ...] (skip mode keyword)
	// parseCommonAttribute expects: args[idx]=attrName, args[idx+1]=value
	valueArgs := append([]string{attrName}, args[2:]...)
	extra, err := parseCommonAttribute(attrName, valueArgs, 0, &attrs.PathAttributes)
	if err != nil {
		return "", parsedAttrs{}, 0, err
	}
	if extra == 0 {
		return "", parsedAttrs{}, 0, fmt.Errorf("missing value for %s %s", attrName, mode)
	}

	consumed += extra // value tokens consumed
	return mode, attrs, consumed, nil
}

// parseNLRISection parses nlri <family> [rd <value>] [label <value>] <nlri-op>+
// <nlri-op> := add <prefix>+ [watchdog set <name>] | del <prefix>+
// accum contains NLRI accumulators: pathID, RD, labels.
// In-NLRI modifiers (rd/label without 'set') override accumulated values.
// Returns family, announce list, withdraw list, watchdog name, consumed token count, and any error.
func parseNLRISection(args []string, accum nlriAccum) (nlri.Family, []nlri.NLRI, []nlri.NLRI, string, int, error) {
	// args[0] = "nlri"
	if len(args) < 2 {
		return nlri.Family{}, nil, nil, "", 0, ErrInvalidFamily
	}

	family, ok := nlri.ParseFamily(args[1])
	if !ok {
		return nlri.Family{}, nil, nil, "", 0, fmt.Errorf("%w: %s", ErrInvalidFamily, args[1])
	}

	// Check if family is supported
	if !isSupportedFamily(family) {
		return nlri.Family{}, nil, nil, "", 0, fmt.Errorf("%w: %s", ErrFamilyNotSupported, args[1])
	}

	consumed := 2 // "nlri" + family
	i := 2

	// Parse in-NLRI modifiers: rd <value>, label <value> (without 'set')
	// These override accumulated values for this nlri section only
	for i < len(args) {
		token := args[i] //nolint:gosec // G602 false positive: loop condition guards access

		if token == kwRD {
			// rd <value> (in-NLRI modifier, no 'set')
			if i+1 >= len(args) {
				return nlri.Family{}, nil, nil, "", 0, errors.New("rd requires value (ASN:NN or IP:NN)")
			}
			next := args[i+1]
			// If next token is 'set', this is accumulator syntax - don't handle here
			if next == kwSet {
				break
			}
			rd, err := nlri.ParseRDString(next)
			if err != nil {
				return nlri.Family{}, nil, nil, "", 0, fmt.Errorf("invalid rd: %w", err)
			}
			accum.RD = rd
			i += 2
			consumed += 2
			continue
		}

		if token == kwLabel {
			// label <value> (in-NLRI modifier, no 'set')
			if i+1 >= len(args) {
				return nlri.Family{}, nil, nil, "", 0, errors.New("label requires value (0-1048575)")
			}
			next := args[i+1]
			// If next token is 'set', this is accumulator syntax - don't handle here
			if next == kwSet {
				break
			}
			label, err := strconv.ParseUint(next, 10, 32)
			if err != nil {
				return nlri.Family{}, nil, nil, "", 0, fmt.Errorf("invalid label: %w", err)
			}
			if label > 0xFFFFF { // 20-bit max
				return nlri.Family{}, nil, nil, "", 0, fmt.Errorf("label out of range (max 1048575): %d", label)
			}
			accum.Labels = []uint32{uint32(label)} //nolint:gosec // G115: bounded by check above
			i += 2
			consumed += 2
			continue
		}

		// Not an in-NLRI modifier, proceed to add/del parsing
		break
	}

	mode := "" // "", "add", or "del"
	var announce, withdraw []nlri.NLRI
	var watchdog string

	for i < len(args) {
		token := args[i] //nolint:gosec // G602 false positive: loop condition guards access

		// Boundary keywords end this section (except watchdog which is handled specially)
		if isBoundaryKeyword(token) && token != kwWatchdog {
			break
		}

		// Watchdog inside nlri section: watchdog set <name>
		if token == kwWatchdog {
			if mode != kwAdd {
				return nlri.Family{}, nil, nil, "", 0, errors.New("watchdog only valid after 'add' in nlri section")
			}
			if i+2 >= len(args) {
				return nlri.Family{}, nil, nil, "", 0, errors.New("watchdog requires 'set <name>'")
			}
			if args[i+1] != kwSet {
				return nlri.Family{}, nil, nil, "", 0, fmt.Errorf("watchdog requires 'set', got: %s", args[i+1])
			}
			watchdog = args[i+2]
			i += 3
			consumed += 3
			continue
		}

		// Mode switches
		if token == kwAdd {
			mode = kwAdd
			i++
			consumed++
			continue
		}
		if token == kwDel {
			mode = kwDel
			i++
			consumed++
			continue
		}

		// Must have a mode before prefixes
		if mode == "" {
			return nlri.Family{}, nil, nil, "", 0, fmt.Errorf("%w: got %q", ErrMissingAddDel, token)
		}

		// Parse prefix based on family
		n, extra, err := parseNLRI(token, family, accum)
		if err != nil {
			return nlri.Family{}, nil, nil, "", 0, err
		}

		if mode == kwAdd {
			announce = append(announce, n)
		} else {
			withdraw = append(withdraw, n)
		}
		i += 1 + extra
		consumed += 1 + extra
	}

	// Must have at least one prefix
	if len(announce) == 0 && len(withdraw) == 0 {
		return nlri.Family{}, nil, nil, "", 0, ErrEmptyNLRISection
	}

	return family, announce, withdraw, watchdog, consumed, nil
}

// parseINETNLRI parses a single prefix for unicast/multicast families.
// pathID is the ADD-PATH path identifier (0 = not set).
// Returns the NLRI, extra args consumed (always 0 for INET), and any error.
// The second return value exists for future family parsers (labeled, VPN)
// that consume additional arguments.
//
//nolint:unparam // int return value reserved for future families
func parseINETNLRI(token string, family nlri.Family, pathID uint32) (nlri.NLRI, int, error) {
	prefix, err := netip.ParsePrefix(token)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %s", ErrInvalidPrefix, token)
	}

	// Validate prefix matches family AFI
	isIPv4 := prefix.Addr().Is4()
	if isIPv4 && family.AFI != nlri.AFIIPv4 {
		return nil, 0, fmt.Errorf("%w: IPv4 prefix for %s", ErrFamilyMismatch, family)
	}
	if !isIPv4 && family.AFI != nlri.AFIIPv6 {
		return nil, 0, fmt.Errorf("%w: IPv6 prefix for %s", ErrFamilyMismatch, family)
	}

	return nlri.NewINET(family, prefix, pathID), 0, nil // 0 extra args consumed
}

// parseNLRI dispatches to the appropriate NLRI parser based on family.
// Returns NLRI, extra args consumed, and any error.
func parseNLRI(token string, family nlri.Family, accum nlriAccum) (nlri.NLRI, int, error) {
	switch family.SAFI { //nolint:exhaustive // Other SAFIs use INET parser via default
	case nlri.SAFIVPN: // SAFI 128 - MPLS VPN
		return parseVPNNLRI(token, family, accum)
	case nlri.SAFIMPLSLabel: // SAFI 4 - Labeled Unicast
		return parseLabeledNLRI(token, family, accum)
	default:
		return parseINETNLRI(token, family, accum.PathID)
	}
}

// parseVPNNLRI parses a prefix for VPN families (SAFI 128).
// Requires RD and labels from accum.
// RFC 4364 Section 4.3.4: VPN NLRI = labels + RD + prefix.
func parseVPNNLRI(token string, family nlri.Family, accum nlriAccum) (nlri.NLRI, int, error) {
	prefix, err := netip.ParsePrefix(token)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %s", ErrInvalidPrefix, token)
	}

	// Validate prefix matches family AFI
	isIPv4 := prefix.Addr().Is4()
	if isIPv4 && family.AFI != nlri.AFIIPv4 {
		return nil, 0, fmt.Errorf("%w: IPv4 prefix for %s", ErrFamilyMismatch, family)
	}
	if !isIPv4 && family.AFI != nlri.AFIIPv6 {
		return nil, 0, fmt.Errorf("%w: IPv6 prefix for %s", ErrFamilyMismatch, family)
	}

	// Require RD for VPN families
	if accum.RD.Type == 0 && accum.RD.Value == [6]byte{} {
		return nil, 0, fmt.Errorf("%w: rd required for %s", ErrMissingRD, family)
	}

	// Require at least one label for VPN families
	if len(accum.Labels) == 0 {
		return nil, 0, fmt.Errorf("%w: label required for %s", ErrMissingLabel, family)
	}

	return nlri.NewIPVPN(family, accum.RD, accum.Labels, prefix, accum.PathID), 0, nil
}

// parseLabeledNLRI parses a prefix for labeled unicast families (SAFI 4).
// Requires labels from accum.
// RFC 8277: Labeled Unicast NLRI = labels + prefix.
func parseLabeledNLRI(token string, family nlri.Family, accum nlriAccum) (nlri.NLRI, int, error) {
	prefix, err := netip.ParsePrefix(token)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %s", ErrInvalidPrefix, token)
	}

	// Validate prefix matches family AFI
	isIPv4 := prefix.Addr().Is4()
	if isIPv4 && family.AFI != nlri.AFIIPv4 {
		return nil, 0, fmt.Errorf("%w: IPv4 prefix for %s", ErrFamilyMismatch, family)
	}
	if !isIPv4 && family.AFI != nlri.AFIIPv6 {
		return nil, 0, fmt.Errorf("%w: IPv6 prefix for %s", ErrFamilyMismatch, family)
	}

	// Require at least one label for labeled unicast
	if len(accum.Labels) == 0 {
		return nil, 0, fmt.Errorf("%w: label required for %s", ErrMissingLabel, family)
	}

	return nlri.NewLabeledUnicast(family, prefix, accum.Labels, accum.PathID), 0, nil
}

// isSupportedFamily returns true if the family is supported in text mode.
func isSupportedFamily(f nlri.Family) bool {
	switch f {
	case nlri.IPv4Unicast, nlri.IPv6Unicast, nlri.IPv4Multicast, nlri.IPv6Multicast:
		return true
	case nlri.IPv4VPN, nlri.IPv6VPN: // VPN families (SAFI 128)
		return true
	case nlri.IPv4LabeledUnicast, nlri.IPv6LabeledUnicast: // Labeled unicast (SAFI 4)
		return true
	default:
		return false
	}
}

// handleUpdate dispatches update subcommands by encoding.
// Syntax: peer <addr> update <encoding> ...
func handleUpdate(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("usage: peer <addr> update <text|hex|b64>")
	}

	encoding := strings.ToLower(args[0])
	switch encoding {
	case "text":
		return handleUpdateText(ctx, args[1:])
	case "hex":
		return handleUpdateHex(ctx, args[1:])
	case "b64":
		return handleUpdateB64(ctx, args[1:])
	default:
		return nil, fmt.Errorf("unknown encoding: %s", encoding)
	}
}

// handleUpdateText handles: peer <addr> update text ...
// Parses the update text format and dispatches to reactor batch methods.
// RFC 4271 Section 4.3: UPDATE Message Format.
func handleUpdateText(ctx *CommandContext, args []string) (*Response, error) {
	result, err := ParseUpdateText(args)
	if err != nil {
		return &Response{Status: statusError, Data: err.Error()}, err
	}

	if result.WatchdogName != "" {
		errMsg := "watchdog not yet implemented for update text"
		return &Response{Status: statusError, Data: errMsg}, errors.New(errMsg)
	}

	return dispatchNLRIGroups(ctx, result.Groups)
}

// dispatchNLRIGroups sends NLRI groups to the reactor for announce/withdraw.
// Returns response with counts and any warnings, or error response.
func dispatchNLRIGroups(ctx *CommandContext, groups []NLRIGroup) (*Response, error) {
	peerSelector := ctx.PeerSelector()
	var announced, withdrawn int
	var warnings []string

	for _, group := range groups {
		if len(group.Announce) > 0 {
			batch := NLRIBatch{
				Family:  group.Family,
				NLRIs:   group.Announce,
				NextHop: group.NextHop,
				Attrs:   group.Attrs,
			}
			if err := ctx.Reactor.AnnounceNLRIBatch(peerSelector, batch); err != nil {
				if errors.Is(err, ErrNoPeersAcceptedFamily) {
					warnings = append(warnings, fmt.Sprintf("announce %v: %s", group.Family, err))
					continue
				}
				return &Response{Status: statusError, Data: err.Error()}, err
			}
			announced += len(group.Announce)
		}
		if len(group.Withdraw) > 0 {
			batch := NLRIBatch{
				Family: group.Family,
				NLRIs:  group.Withdraw,
			}
			if err := ctx.Reactor.WithdrawNLRIBatch(peerSelector, batch); err != nil {
				if errors.Is(err, ErrNoPeersAcceptedFamily) {
					warnings = append(warnings, fmt.Sprintf("withdraw %v: %s", group.Family, err))
					continue
				}
				return &Response{Status: statusError, Data: err.Error()}, err
			}
			withdrawn += len(group.Withdraw)
		}
	}

	if announced == 0 && withdrawn == 0 {
		msg := "no routes to announce or withdraw"
		if len(warnings) > 0 {
			msg = strings.Join(warnings, "; ")
		}
		return &Response{
			Status: "warning",
			Data:   msg,
		}, nil
	}

	respData := map[string]any{
		"announced": announced,
		"withdrawn": withdrawn,
	}
	if len(warnings) > 0 {
		respData["warnings"] = warnings
	}

	return &Response{
		Status: statusDone,
		Data:   respData,
	}, nil
}
