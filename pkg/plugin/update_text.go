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
package plugin

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"
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
// Includes next-hop and path-id which are NOT part of path attributes.
// Clear* fields signal "del without value" to remove the attribute entirely.
// Del*Expected fields signal "del <value>" conditional delete (must match current).
type parsedAttrs struct {
	NextHop     netip.Addr
	NextHopSelf bool
	PathID      uint32 // ADD-PATH path identifier (0 = not set)

	// Path attributes (wire-first: build directly to wire format)
	Origin              *uint8
	LocalPreference     *uint32
	MED                 *uint32
	ASPath              []uint32
	Communities         []uint32
	LargeCommunities    []LargeCommunity
	ExtendedCommunities []attribute.ExtendedCommunity

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

// snapshot returns a wire-format snapshot of the current attribute state.
// Builds attributes using Builder for wire-first encoding.
// Also returns the current NLRI accumulators (pathID, RD, labels).
func (a *parsedAttrs) snapshot() (*attribute.AttributesWire, RouteNextHop, nlriAccum) {
	// Build wire-format attributes.
	// Note: ORIGIN is not forced here; reactor adds mandatory attributes if missing.
	b := attribute.NewBuilder()

	if a.Origin != nil {
		b.SetOrigin(*a.Origin)
	}
	// AS_PATH: always set (even if empty) so Builder outputs the attribute.
	// Empty AS_PATH is valid for locally originated routes (iBGP).
	b.SetASPath(a.ASPath)
	if a.LocalPreference != nil {
		b.SetLocalPref(*a.LocalPreference)
	}
	if a.MED != nil {
		b.SetMED(*a.MED)
	}
	for _, c := range a.Communities {
		b.AddCommunityValue(c)
	}
	for _, lc := range a.LargeCommunities {
		b.AddLargeCommunity(lc.GlobalAdmin, lc.LocalData1, lc.LocalData2)
	}
	for _, ec := range a.ExtendedCommunities {
		b.AddExtendedCommunity(ec)
	}

	// Build wire bytes and wrap
	wireBytes := b.Build()
	var wire *attribute.AttributesWire
	if len(wireBytes) > 0 {
		wire = attribute.NewAttributesWire(wireBytes, context.APIContextID)
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
	return wire, nh, nlriAccum{PathID: a.PathID, RD: a.RD, Labels: labels}
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

// parseCommonAttributeText parses a common BGP attribute by keyword into parsedAttrs.
// Returns the number of args consumed (0 if keyword not handled), or error.
func parseCommonAttributeText(key string, args []string, idx int, attrs *parsedAttrs) (int, error) {
	switch key {
	case "origin":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing origin value")
		}
		origin, err := parseOriginText(args[idx+1])
		if err != nil {
			return 0, err
		}
		attrs.Origin = &origin
		return 1, nil

	case "local-preference":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing local-preference value")
		}
		lp, err := strconv.ParseUint(args[idx+1], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid local-preference: %w", err)
		}
		lpVal := uint32(lp)
		attrs.LocalPreference = &lpVal
		return 1, nil

	case "med":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing med value")
		}
		med, err := strconv.ParseUint(args[idx+1], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid med: %w", err)
		}
		medVal := uint32(med)
		attrs.MED = &medVal
		return 1, nil

	case "as-path":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing as-path value")
		}
		tokens, consumed := parseBracketedListText(args[idx+1:])
		asPath := make([]uint32, 0, len(tokens))
		for _, tok := range tokens {
			asn, err := strconv.ParseUint(tok, 10, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid ASN in as-path: %s", tok)
			}
			asPath = append(asPath, uint32(asn))
		}
		attrs.ASPath = asPath
		return consumed, nil

	case "community":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing community value")
		}
		tokens, consumed := parseBracketedListText(args[idx+1:])
		communities := make([]uint32, 0, len(tokens))
		for _, tok := range tokens {
			c, err := parseCommunityText(tok)
			if err != nil {
				return 0, err
			}
			communities = append(communities, c)
		}
		attrs.Communities = communities
		return consumed, nil

	case "large-community":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing large-community value")
		}
		tokens, consumed := parseBracketedListText(args[idx+1:])
		lcs := make([]LargeCommunity, 0, len(tokens))
		for _, tok := range tokens {
			lc, err := parseLargeCommunityText(tok)
			if err != nil {
				return 0, err
			}
			lcs = append(lcs, lc)
		}
		attrs.LargeCommunities = lcs
		return consumed, nil

	case "extended-community":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing extended-community value")
		}
		// Use parseExtendedCommunities which handles both function syntax
		// (traffic-rate, discard, redirect, traffic-marking) and list syntax.
		ecs, consumed, err := parseExtendedCommunities(args[idx+1:])
		if err != nil {
			return 0, err
		}
		attrs.ExtendedCommunities = ecs
		return consumed, nil
	}

	return 0, nil
}

// parseOriginText parses origin string to value.
func parseOriginText(s string) (uint8, error) {
	switch strings.ToLower(s) {
	case "igp":
		return 0, nil
	case "egp":
		return 1, nil
	case "incomplete":
		return 2, nil
	default:
		return 0, fmt.Errorf("invalid origin: %s (valid: igp, egp, incomplete)", s)
	}
}

// parseBracketedListText parses [ v1 v2 ] or v1,v2 or [ v1, v2 ] style lists.
// Returns tokens and consumed arg count.
func parseBracketedListText(args []string) ([]string, int) {
	if len(args) == 0 {
		return nil, 0
	}

	first := args[0]

	// Case 1: "[" as separate token
	if first == "[" {
		var tokens []string
		consumed := 1
		for i := 1; i < len(args); i++ {
			if args[i] == "]" {
				return tokens, i + 1
			}
			// Split by comma if present
			for _, tok := range strings.Split(args[i], ",") {
				tok = strings.TrimSpace(tok)
				if tok != "" {
					tokens = append(tokens, tok)
				}
			}
			consumed = i + 1
		}
		return tokens, consumed
	}

	// Case 2: "[value]" as single token (entire list in one arg)
	if strings.HasPrefix(first, "[") && strings.HasSuffix(first, "]") {
		inner := first[1 : len(first)-1]
		var tokens []string
		for _, tok := range strings.Split(inner, " ") {
			tok = strings.TrimSpace(tok)
			if tok != "" {
				tokens = append(tokens, tok)
			}
		}
		return tokens, 1
	}

	// Case 3: "[value" followed by more tokens then "value]" (brackets attached)
	if strings.HasPrefix(first, "[") {
		var tokens []string
		// First token without leading bracket
		firstVal := strings.TrimPrefix(first, "[")
		for _, tok := range strings.Split(firstVal, ",") {
			tok = strings.TrimSpace(tok)
			if tok != "" {
				tokens = append(tokens, tok)
			}
		}
		consumed := 1

		// Continue until we find closing bracket
		for i := 1; i < len(args); i++ {
			consumed = i + 1
			arg := args[i]
			if strings.HasSuffix(arg, "]") {
				// Last token - strip trailing bracket
				lastVal := strings.TrimSuffix(arg, "]")
				for _, tok := range strings.Split(lastVal, ",") {
					tok = strings.TrimSpace(tok)
					if tok != "" {
						tokens = append(tokens, tok)
					}
				}
				return tokens, consumed
			}
			// Middle tokens
			for _, tok := range strings.Split(arg, ",") {
				tok = strings.TrimSpace(tok)
				if tok != "" {
					tokens = append(tokens, tok)
				}
			}
		}
		return tokens, consumed
	}

	// Case 4: Single value or comma-separated list without brackets
	var tokens []string
	for _, tok := range strings.Split(first, ",") {
		tok = strings.TrimSpace(tok)
		if tok != "" {
			tokens = append(tokens, tok)
		}
	}
	return tokens, 1
}

// parseCommunityText parses community in ASN:value or well-known format.
func parseCommunityText(s string) (uint32, error) {
	// Well-known communities
	switch strings.ToLower(s) {
	case "no-export":
		return 0xFFFFFF01, nil
	case "no-advertise":
		return 0xFFFFFF02, nil
	case "no-export-subconfed":
		return 0xFFFFFF03, nil
	}

	// ASN:value format
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid community format: %s (expected ASN:value)", s)
	}
	high, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid community ASN: %s", parts[0])
	}
	low, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid community value: %s", parts[1])
	}
	return uint32(high)<<16 | uint32(low), nil
}

// parseLargeCommunityText parses large community in GA:LD1:LD2 format.
func parseLargeCommunityText(s string) (LargeCommunity, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return LargeCommunity{}, fmt.Errorf("invalid large-community format: %s (expected GA:LD1:LD2)", s)
	}
	ga, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return LargeCommunity{}, fmt.Errorf("invalid large-community global-admin: %s", parts[0])
	}
	ld1, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return LargeCommunity{}, fmt.Errorf("invalid large-community local-data-1: %s", parts[1])
	}
	ld2, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return LargeCommunity{}, fmt.Errorf("invalid large-community local-data-2: %s", parts[2])
	}
	return LargeCommunity{GlobalAdmin: uint32(ga), LocalData1: uint32(ld1), LocalData2: uint32(ld2)}, nil
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
			wire, nh, nlriAcc := accum.snapshot()
			family, announce, withdraw, nlriWatchdog, consumed, err := parseNLRISection(args[i:], nlriAcc)
			if err != nil {
				return nil, err
			}

			groups = append(groups, NLRIGroup{
				Family:       family,
				Announce:     announce,
				Withdraw:     withdraw,
				Wire:         wire,
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

		// Try parseCommonAttributeText for standard attrs
		extra, err := parseCommonAttributeText(key, args, i, &attrs)
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
			var tempAttrs parsedAttrs
			extra, err := parseCommonAttributeText(attrName, valueArgs, 0, &tempAttrs)
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

	// Parse the value using parseCommonAttributeText
	// Build args slice: [attrName, value1, value2, ...] (skip mode keyword)
	// parseCommonAttributeText expects: args[idx]=attrName, args[idx+1]=value
	valueArgs := append([]string{attrName}, args[2:]...)
	extra, err := parseCommonAttributeText(attrName, valueArgs, 0, &attrs)
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

	// FlowSpec families use different parsing (components instead of prefixes)
	// RFC 8955 Section 4: FlowSpec NLRI = ordered list of match components
	if family.SAFI == nlri.SAFIFlowSpec || family.SAFI == nlri.SAFIFlowSpecVPN {
		return parseFlowSpecSection(args, family, accum)
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
// For FlowSpec families, this function is not used - see parseFlowSpecSection.
func parseNLRI(token string, family nlri.Family, accum nlriAccum) (nlri.NLRI, int, error) {
	switch family.SAFI { //nolint:exhaustive // Other SAFIs use INET parser via default
	case nlri.SAFIVPN: // SAFI 128 - MPLS VPN
		return parseVPNNLRI(token, family, accum)
	case nlri.SAFIMPLSLabel: // SAFI 4 - Labeled Unicast
		return parseLabeledNLRI(token, family, accum)
	case nlri.SAFIFlowSpec, nlri.SAFIFlowSpecVPN:
		// FlowSpec uses special parsing - should not reach here
		return nil, 0, fmt.Errorf("flowspec parsing requires parseFlowSpecSection")
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

// ============================================================================
// FlowSpec Parsing (RFC 8955)
// ============================================================================

// FlowSpec component keywords per RFC 8955 Section 4.2.2.
// Each component type has a specific type code in the wire format:
//
//	Type 1:  Destination Prefix (RFC 8955 §4.2.2.1)
//	Type 2:  Source Prefix (RFC 8955 §4.2.2.2)
//	Type 3:  IP Protocol (RFC 8955 §4.2.2.3)
//	Type 4:  Port (RFC 8955 §4.2.2.4)
//	Type 5:  Destination Port (RFC 8955 §4.2.2.5)
//	Type 6:  Source Port (RFC 8955 §4.2.2.6)
//	Type 7:  ICMP Type (RFC 8955 §4.2.2.7)
//	Type 8:  ICMP Code (RFC 8955 §4.2.2.8)
//	Type 9:  TCP Flags (RFC 8955 §4.2.2.9) - uses bitmask_op
//	Type 10: Packet Length (RFC 8955 §4.2.2.10)
//	Type 11: DSCP (RFC 8955 §4.2.2.11)
//	Type 12: Fragment (RFC 8955 §4.2.2.12) - uses bitmask_op
const (
	kwFSDestination  = "destination"      // Type 1
	kwFSSource       = "source"           // Type 2
	kwFSProtocol     = "protocol"         // Type 3
	kwFSPort         = "port"             // Type 4
	kwFSDestPort     = "destination-port" // Type 5
	kwFSSourcePort   = "source-port"      // Type 6
	kwFSICMPType     = "icmp-type"        // Type 7
	kwFSICMPCode     = "icmp-code"        // Type 8
	kwFSTCPFlags     = "tcp-flags"        // Type 9
	kwFSPacketLength = "packet-length"    // Type 10
	kwFSDSCP         = "dscp"             // Type 11
	kwFSFragment     = "fragment"         // Type 12
	kwFSNot          = "not"              // Modifier for bitmask_op (RFC 8955 §4.2.1.2)
)

// isFlowSpecComponentKeyword returns true if token is a FlowSpec component keyword.
func isFlowSpecComponentKeyword(token string) bool {
	switch token {
	case kwFSDestination, kwFSSource, kwFSProtocol, kwFSPort,
		kwFSDestPort, kwFSSourcePort, kwFSICMPType, kwFSICMPCode,
		kwFSTCPFlags, kwFSPacketLength, kwFSDSCP, kwFSFragment:
		return true
	}
	return false
}

// parseFlowSpecSection parses nlri <flowspec-family> [rd <value>] add <components>+ | del <components>+
// RFC 8955 Section 4: FlowSpec NLRI = ordered list of match components.
// Returns family, announce list, withdraw list, watchdog name, consumed tokens, and error.
func parseFlowSpecSection(args []string, family nlri.Family, accum nlriAccum) (nlri.Family, []nlri.NLRI, []nlri.NLRI, string, int, error) {
	// args[0] = "nlri", args[1] = family string
	consumed := 2
	i := 2

	// Parse in-NLRI modifiers: rd <value> (for flowspec-vpn)
	for i < len(args) {
		token := args[i] //nolint:gosec // G602 false positive: loop condition guards access

		if token == kwRD {
			if i+1 >= len(args) {
				return nlri.Family{}, nil, nil, "", 0, errors.New("rd requires value (ASN:NN or IP:NN)")
			}
			next := args[i+1]
			if next == kwSet {
				break // Accumulator syntax, not in-NLRI modifier
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
		break
	}

	// Parse add/del + components
	mode := "" // "", "add", or "del"
	var announce, withdraw []nlri.NLRI

	for i < len(args) {
		token := args[i] //nolint:gosec // G602 false positive: loop condition guards access

		// Boundary keywords end this section
		if isBoundaryKeyword(token) {
			break
		}

		// Mode switches
		if token == kwAdd {
			// Create a new FlowSpec for this add block
			// (consecutive add tokens are implicit continuation)
			mode = kwAdd
			i++
			consumed++
			continue
		}
		if token == kwDel {
			// (consecutive del tokens are implicit continuation)
			mode = kwDel
			i++
			consumed++
			continue
		}

		// Must have a mode before components
		if mode == "" {
			return nlri.Family{}, nil, nil, "", 0, fmt.Errorf("%w: got %q", ErrMissingAddDel, token)
		}

		// Parse FlowSpec components for this rule
		fs, extra, err := parseFlowSpecComponents(args[i:], family, accum)
		if err != nil {
			return nlri.Family{}, nil, nil, "", 0, err
		}

		if mode == kwAdd {
			announce = append(announce, fs)
		} else {
			withdraw = append(withdraw, fs)
		}
		i += extra
		consumed += extra
	}

	if len(announce) == 0 && len(withdraw) == 0 {
		return nlri.Family{}, nil, nil, "", 0, ErrEmptyNLRISection
	}

	return family, announce, withdraw, "", consumed, nil
}

// parseFlowSpecComponents parses FlowSpec components until boundary or mode switch.
// One add/del = one FlowSpec rule with all its components.
// RFC 8955: Components are ANDed together.
func parseFlowSpecComponents(args []string, family nlri.Family, accum nlriAccum) (nlri.NLRI, int, error) {
	var fs *nlri.FlowSpec
	var fsv *nlri.FlowSpecVPN

	// Create appropriate FlowSpec type
	if family.SAFI == nlri.SAFIFlowSpecVPN {
		// Require RD for VPN families
		if accum.RD.Type == 0 && accum.RD.Value == [6]byte{} {
			return nil, 0, fmt.Errorf("%w: rd required for %s", ErrMissingRD, family)
		}
		fsv = nlri.NewFlowSpecVPN(family, accum.RD)
	} else {
		fs = nlri.NewFlowSpec(family)
	}

	addComponent := func(c nlri.FlowComponent) {
		if fsv != nil {
			fsv.AddComponent(c)
		} else {
			fs.AddComponent(c)
		}
	}

	consumed := 0
	i := 0

	for i < len(args) {
		token := args[i] //nolint:gosec // G602 false positive: loop condition guards access

		// Stop at boundary keywords or mode switches
		if isBoundaryKeyword(token) || token == kwAdd || token == kwDel {
			break
		}

		// Parse component by type
		comp, extra, err := parseFlowSpecComponent(args[i:], family)
		if err != nil {
			return nil, 0, err
		}

		addComponent(comp)
		i += extra
		consumed += extra
	}

	if consumed == 0 {
		return nil, 0, errors.New("flowspec requires at least one component")
	}

	if fsv != nil {
		return fsv, consumed, nil
	}
	return fs, consumed, nil
}

// parseFlowSpecComponent parses a single FlowSpec component.
// Returns the component, tokens consumed, and any error.
func parseFlowSpecComponent(args []string, family nlri.Family) (nlri.FlowComponent, int, error) {
	if len(args) == 0 {
		return nil, 0, errors.New("expected flowspec component")
	}

	compType := args[0]

	switch compType {
	case kwFSDestination:
		if len(args) < 2 {
			return nil, 0, errors.New("destination requires prefix")
		}
		prefix, err := netip.ParsePrefix(args[1])
		if err != nil {
			return nil, 0, fmt.Errorf("invalid destination prefix: %w", err)
		}
		// Validate AFI match
		if prefix.Addr().Is4() && family.AFI != nlri.AFIIPv4 {
			return nil, 0, fmt.Errorf("IPv4 prefix for IPv6 flowspec")
		}
		if prefix.Addr().Is6() && family.AFI != nlri.AFIIPv6 {
			return nil, 0, fmt.Errorf("IPv6 prefix for IPv4 flowspec")
		}
		return nlri.NewFlowDestPrefixComponent(prefix), 2, nil

	case kwFSSource:
		if len(args) < 2 {
			return nil, 0, errors.New("source requires prefix")
		}
		prefix, err := netip.ParsePrefix(args[1])
		if err != nil {
			return nil, 0, fmt.Errorf("invalid source prefix: %w", err)
		}
		if prefix.Addr().Is4() && family.AFI != nlri.AFIIPv4 {
			return nil, 0, fmt.Errorf("IPv4 prefix for IPv6 flowspec")
		}
		if prefix.Addr().Is6() && family.AFI != nlri.AFIIPv6 {
			return nil, 0, fmt.Errorf("IPv6 prefix for IPv4 flowspec")
		}
		return nlri.NewFlowSourcePrefixComponent(prefix), 2, nil

	case kwFSProtocol:
		return parseFlowSpecProtocol(args[1:])

	case kwFSPort:
		return parseFlowSpecNumeric(args[1:], nlri.FlowPort)

	case kwFSDestPort:
		return parseFlowSpecNumeric(args[1:], nlri.FlowDestPort)

	case kwFSSourcePort:
		return parseFlowSpecNumeric(args[1:], nlri.FlowSourcePort)

	case kwFSICMPType:
		return parseFlowSpecNumeric(args[1:], nlri.FlowICMPType)

	case kwFSICMPCode:
		return parseFlowSpecNumeric(args[1:], nlri.FlowICMPCode)

	case kwFSTCPFlags:
		return parseFlowSpecTCPFlags(args[1:])

	case kwFSPacketLength:
		return parseFlowSpecNumeric(args[1:], nlri.FlowPacketLength)

	case kwFSDSCP:
		return parseFlowSpecNumeric(args[1:], nlri.FlowDSCP)

	case kwFSFragment:
		return parseFlowSpecFragment(args[1:])

	default:
		return nil, 0, fmt.Errorf("unknown flowspec component: %s", compType)
	}
}

// Protocol name to number mapping.
// IANA Protocol Numbers: https://www.iana.org/assignments/protocol-numbers
// RFC 8955 Section 4.2.2.3: IP Protocol component uses these values.
var protocolNameToNumber = map[string]uint8{
	"icmp":   1,   // RFC 792
	"igmp":   2,   // RFC 1112
	"tcp":    6,   // RFC 793
	"udp":    17,  // RFC 768
	"gre":    47,  // RFC 2784
	"icmpv6": 58,  // RFC 4443
	"ospf":   89,  // RFC 2328
	"sctp":   132, // RFC 4960
}

// parseFlowSpecProtocol parses protocol component values.
// RFC 8955 Section 4.2.2.3: IP Protocol (Type 3) uses numeric_op format.
// Accepts: tcp, udp, icmp, gre, or numeric (0-255).
func parseFlowSpecProtocol(args []string) (nlri.FlowComponent, int, error) {
	if len(args) == 0 {
		return nil, 0, errors.New("protocol requires value")
	}

	var protocols []uint8
	consumed := 0

	for i := 0; i < len(args); i++ {
		token := args[i]

		// Stop at boundary/mode keywords or next component
		if isBoundaryKeyword(token) || token == kwAdd || token == kwDel || isFlowSpecComponentKeyword(token) {
			break
		}

		// Try protocol name first
		if num, ok := protocolNameToNumber[strings.ToLower(token)]; ok {
			protocols = append(protocols, num)
			consumed++
			continue
		}

		// Try numeric
		num, err := strconv.ParseUint(token, 10, 8)
		if err != nil {
			if consumed == 0 {
				return nil, 0, fmt.Errorf("invalid protocol: %s", token)
			}
			break // End of protocol values
		}
		protocols = append(protocols, uint8(num)) //nolint:gosec // G115: bounded by ParseUint
		consumed++
	}

	if len(protocols) == 0 {
		return nil, 0, errors.New("protocol requires at least one value")
	}

	return nlri.NewFlowIPProtocolComponent(protocols...), consumed + 1, nil // +1 for "protocol" keyword
}

// flowSpecComponentMaxValue returns the maximum valid value for a component type.
// RFC 8955 Section 4.2.2 defines value sizes per component type:
//
//	Type 3 (Protocol):      8-bit (0-255)    - RFC 8955 §4.2.2.3
//	Type 4-6 (Ports):       16-bit (0-65535) - RFC 8955 §4.2.2.4-6
//	Type 7-8 (ICMP):        8-bit (0-255)    - RFC 8955 §4.2.2.7-8
//	Type 10 (Packet Len):   16-bit (0-65535) - RFC 8955 §4.2.2.10
//	Type 11 (DSCP):         6-bit (0-63)     - RFC 8955 §4.2.2.11, RFC 2474
func flowSpecComponentMaxValue(compType nlri.FlowComponentType) uint64 {
	switch compType { //nolint:exhaustive // Prefix/bitmask types handled separately
	case nlri.FlowIPProtocol, nlri.FlowICMPType, nlri.FlowICMPCode:
		return 255 // 8-bit per RFC 8955
	case nlri.FlowPort, nlri.FlowDestPort, nlri.FlowSourcePort, nlri.FlowPacketLength:
		return 65535 // 16-bit per RFC 8955
	case nlri.FlowDSCP:
		return 63 // 6-bit per RFC 2474 (DSCP uses 6 bits of TOS byte)
	default:
		return 0xFFFFFFFF // 32-bit default
	}
}

// parseFlowSpecNumeric parses numeric component values with operators.
// RFC 8955 Section 4.2.1.1: Numeric operator format (numeric_op).
// Syntax: <op><value>+ where op is optional (=, >, <, >=, <=).
// Multiple values are ANDed together for range matching.
func parseFlowSpecNumeric(args []string, compType nlri.FlowComponentType) (nlri.FlowComponent, int, error) {
	if len(args) == 0 {
		return nil, 0, fmt.Errorf("%s requires value", compType)
	}

	maxValue := flowSpecComponentMaxValue(compType)
	var matches []nlri.FlowMatch
	consumed := 0

	for i := 0; i < len(args); i++ {
		token := args[i]

		// Stop at boundary/mode keywords or next component
		if isBoundaryKeyword(token) || token == kwAdd || token == kwDel || isFlowSpecComponentKeyword(token) {
			break
		}

		// Parse operator and value
		op, value, err := parseNumericOperator(token)
		if err != nil {
			if consumed == 0 {
				return nil, 0, fmt.Errorf("invalid %s value: %w", compType, err)
			}
			break // End of values
		}

		// Validate value range
		if value > maxValue {
			return nil, 0, fmt.Errorf("%s value %d exceeds maximum %d", compType, value, maxValue)
		}

		// Second and subsequent values have AND logic for range matching
		match := nlri.FlowMatch{Op: op, Value: value, And: consumed > 0}
		matches = append(matches, match)
		consumed++
	}

	if len(matches) == 0 {
		return nil, 0, fmt.Errorf("%s requires at least one value", compType)
	}

	return nlri.NewFlowNumericComponent(compType, matches), consumed + 1, nil // +1 for component keyword
}

// parseNumericOperator parses <op><value> syntax.
// RFC 8955 Section 4.2.1.1 Table 1: Numeric operator bit combinations:
//
//	lt=0 gt=0 eq=1: == (equal)      -> 0x01
//	lt=0 gt=1 eq=0: >  (greater)    -> 0x02
//	lt=0 gt=1 eq=1: >= (greater-eq) -> 0x03
//	lt=1 gt=0 eq=0: <  (less)       -> 0x04
//	lt=1 gt=0 eq=1: <= (less-eq)    -> 0x05
//
// Returns operator, value, and error.
func parseNumericOperator(token string) (nlri.FlowOperator, uint64, error) {
	op := nlri.FlowOpEqual // default
	valueStr := token

	// Check for operator prefix (order matters: >= before >, <= before <)
	//nolint:gocritic // ifElseChain: order matters here, can't use switch
	if strings.HasPrefix(token, ">=") {
		op = nlri.FlowOpGreater | nlri.FlowOpEqual
		valueStr = token[2:]
	} else if strings.HasPrefix(token, "<=") {
		op = nlri.FlowOpLess | nlri.FlowOpEqual
		valueStr = token[2:]
	} else if strings.HasPrefix(token, ">") {
		op = nlri.FlowOpGreater
		valueStr = token[1:]
	} else if strings.HasPrefix(token, "<") {
		op = nlri.FlowOpLess
		valueStr = token[1:]
	} else if strings.HasPrefix(token, "=") {
		op = nlri.FlowOpEqual
		valueStr = token[1:]
	}
	// No operator = default equal

	value, err := strconv.ParseUint(valueStr, 10, 64)
	if err != nil {
		return 0, 0, err
	}

	return op, value, nil
}

// TCP flag name to bitmask mapping.
// RFC 8955 Section 4.2.2.9: TCP flags (Type 9) uses bitmask_op format.
// RFC 793 Section 3.1 defines the TCP header flags field:
//
//	Bit 0: FIN  - No more data from sender
//	Bit 1: SYN  - Synchronize sequence numbers
//	Bit 2: RST  - Reset the connection
//	Bit 3: PSH  - Push function
//	Bit 4: ACK  - Acknowledgment field significant
//	Bit 5: URG  - Urgent pointer field significant
//	Bit 6: ECE  - ECN-Echo (RFC 3168)
//	Bit 7: CWR  - Congestion Window Reduced (RFC 3168)
var tcpFlagNameToValue = map[string]uint8{
	"fin":  0x01, // RFC 793
	"syn":  0x02, // RFC 793
	"rst":  0x04, // RFC 793
	"psh":  0x08, // RFC 793
	"push": 0x08, // ExaBGP compatibility alias for psh
	"ack":  0x10, // RFC 793
	"urg":  0x20, // RFC 793
	"ece":  0x40, // RFC 3168 ECN-Echo
	"cwr":  0x80, // RFC 3168 Congestion Window Reduced
}

// parseFlowSpecTCPFlags parses tcp-flags component.
// RFC 8955 Section 4.2.1.2: Bitmask operator format.
//
// Syntax: tcp-flags <match>+ where match is [!][=]<flag>[&<flag>...]
// Operators:
//   - (none): match if ANY of the flags are set in the packet
//   - =     : match if EXACTLY these flags are set (match bit)
//   - !     : negate the match (NOT operator)
//   - &     : AND with previous match (vs OR)
//
// Examples:
//   - tcp-flags syn           -> match if SYN is set
//   - tcp-flags =syn          -> match if ONLY SYN is set
//   - tcp-flags !rst          -> match if RST is NOT set
//   - tcp-flags syn &ack      -> match if SYN OR (previous AND ACK)
//   - tcp-flags =syn&ack      -> match if exactly SYN+ACK
func parseFlowSpecTCPFlags(args []string) (nlri.FlowComponent, int, error) {
	if len(args) == 0 {
		return nil, 0, errors.New("tcp-flags requires flag names")
	}

	var matches []nlri.FlowMatch
	consumed := 0

	for i := 0; i < len(args); i++ {
		token := strings.ToLower(args[i])

		// Stop at boundary/mode keywords or next component
		if isBoundaryKeyword(token) || token == kwAdd || token == kwDel || isFlowSpecComponentKeyword(token) {
			break
		}

		match, err := parseBitmaskMatch(token, tcpFlagNameToValue)
		if err != nil {
			if consumed == 0 {
				return nil, 0, fmt.Errorf("invalid tcp-flags: %w", err)
			}
			break // End of flags
		}

		// RFC 8955: AND bit is N/A for first operand
		if consumed == 0 {
			match.And = false
		}

		matches = append(matches, match)
		consumed++
	}

	if consumed == 0 {
		return nil, 0, errors.New("tcp-flags requires at least one flag")
	}

	return nlri.NewFlowTCPFlagsMatchComponent(matches), consumed + 1, nil // +1 for "tcp-flags" keyword
}

// parseBitmaskMatch parses a single bitmask match expression.
// Format: [&][!][=]<flag>[&<flag>...]
// Returns FlowMatch with Op set to bitmask operator bits (NOT, Match).
func parseBitmaskMatch[T ~uint8](token string, flagMap map[string]T) (nlri.FlowMatch, error) {
	var op nlri.FlowOperator
	var and bool
	s := token

	// Parse leading & (AND with previous)
	if strings.HasPrefix(s, "&") {
		and = true
		s = s[1:]
	}

	// Parse ! (NOT operator)
	if strings.HasPrefix(s, "!") {
		op |= nlri.FlowOpNot
		s = s[1:]
		// Handle != (NOT + Match)
		if strings.HasPrefix(s, "=") {
			op |= nlri.FlowOpMatch
			s = s[1:]
		}
	} else if strings.HasPrefix(s, "=") {
		// Parse = (Match operator - exact match)
		op |= nlri.FlowOpMatch
		s = s[1:]
	}
	// No prefix = INCLUDE (any bit set)

	// Parse flags joined by &
	var value uint8
	flagParts := strings.Split(s, "&")
	for _, part := range flagParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if v, ok := flagMap[part]; ok {
			value |= uint8(v)
		} else {
			return nlri.FlowMatch{}, fmt.Errorf("unknown flag: %s", part)
		}
	}

	if value == 0 {
		return nlri.FlowMatch{}, errors.New("no valid flags specified")
	}

	return nlri.FlowMatch{Op: op, Value: uint64(value), And: and}, nil
}

// Fragment flag name to value mapping.
// RFC 8955 Section 4.2.2.12: Fragment (Type 12) uses bitmask_op format.
// The bitmask operand encodes IP fragmentation status:
//
//	Bit 0: DF  - Don't Fragment (IP header flags bit 1)
//	Bit 1: IsF - Is a Fragment (fragment offset != 0)
//	Bit 2: FF  - First Fragment (fragment offset == 0, MF == 1)
//	Bit 3: LF  - Last Fragment (fragment offset != 0, MF == 0)
var fragmentNameToValue = map[string]nlri.FlowFragmentFlag{
	"dont-fragment":  nlri.FlowFragDontFragment,  // DF bit (RFC 791)
	"is-fragment":    nlri.FlowFragIsFragment,    // Fragment offset != 0
	"first-fragment": nlri.FlowFragFirstFragment, // First of fragmented packet
	"last-fragment":  nlri.FlowFragLastFragment,  // Last of fragmented packet
}

// parseFlowSpecFragment parses fragment component.
// RFC 8955 Section 4.2.1.2: Bitmask operator format.
//
// Syntax: fragment <match>+ where match is [!][=]<type>[&<type>...]
// Operators same as tcp-flags (!, =, &).
//
// Examples:
//   - fragment dont-fragment        -> match if DF is set
//   - fragment !is-fragment         -> match if NOT a fragment
//   - fragment =first-fragment      -> match if EXACTLY first-fragment
func parseFlowSpecFragment(args []string) (nlri.FlowComponent, int, error) {
	if len(args) == 0 {
		return nil, 0, errors.New("fragment requires type")
	}

	var matches []nlri.FlowMatch
	consumed := 0

	for i := 0; i < len(args); i++ {
		token := strings.ToLower(args[i])

		// Stop at boundary/mode keywords or next component
		if isBoundaryKeyword(token) || token == kwAdd || token == kwDel || isFlowSpecComponentKeyword(token) {
			break
		}

		match, err := parseBitmaskMatch(token, fragmentNameToValue)
		if err != nil {
			if consumed == 0 {
				return nil, 0, fmt.Errorf("invalid fragment: %w", err)
			}
			break
		}

		// RFC 8955: AND bit is N/A for first operand
		if consumed == 0 {
			match.And = false
		}

		matches = append(matches, match)
		consumed++
	}

	if len(matches) == 0 {
		return nil, 0, errors.New("fragment requires at least one type")
	}

	return nlri.NewFlowFragmentMatchComponent(matches), consumed + 1, nil // +1 for "fragment" keyword
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
	case nlri.IPv4FlowSpec, nlri.IPv6FlowSpec: // FlowSpec (SAFI 133) - RFC 8955
		return true
	case nlri.IPv4FlowSpecVPN, nlri.IPv6FlowSpecVPN: // FlowSpec VPN (SAFI 134) - RFC 8955
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
				Wire:    group.Wire,
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
