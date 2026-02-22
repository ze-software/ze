// Design: docs/architecture/api/commands.md — API command handlers
//
// update_text.go provides the update text parser for the "update text" command format.
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
package handler

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/format"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/route"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// YANG schema paths for attribute validation.
const (
	yangPathOrigin    = "bgp.peer.update.attribute.origin"
	yangPathMED       = "bgp.peer.update.attribute.med"
	yangPathLocalPref = "bgp.peer.update.attribute.local-preference"
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
	kwEOR = "eor" // End-of-RIB marker (RFC 4724)
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
	LargeCommunities    []bgptypes.LargeCommunity
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
	if err := a.validateListOp(other, route.ErrAddOnScalar); err != nil {
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

// nlriParseResult holds the return values from NLRI section parsing.
type nlriParseResult struct {
	Family   nlri.Family
	Announce []nlri.NLRI
	Withdraw []nlri.NLRI
	Watchdog string
	Consumed int
}

// snapshot returns a wire-format snapshot of the current attribute state.
// Builds attributes using Builder for wire-first encoding.
// Also returns the current NLRI accumulators (pathID, RD, labels).
func (a *parsedAttrs) snapshot() (*attribute.AttributesWire, bgptypes.RouteNextHop, nlriAccum) {
	// Build wire-format attributes.
	// Note: ORIGIN and AS_PATH are not forced here; reactor adds mandatory
	// attributes if missing (with correct iBGP/eBGP AS_PATH handling).
	b := attribute.NewBuilder()

	if a.Origin != nil {
		b.SetOrigin(*a.Origin)
	}
	if len(a.ASPath) > 0 {
		b.SetASPath(a.ASPath)
	}
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

	// Convert to bgptypes.RouteNextHop: Self takes precedence if set
	var nh bgptypes.RouteNextHop
	if a.NextHopSelf {
		nh = bgptypes.NewNextHopSelf()
	} else if a.NextHop.IsValid() {
		nh = bgptypes.NewNextHopExplicit(a.NextHop)
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

// formatLargeCommunity formats a bgptypes.LargeCommunity as "GA:LD1:LD2".
func formatLargeCommunity(lc bgptypes.LargeCommunity) string {
	return fmt.Sprintf("%d:%d:%d", lc.GlobalAdmin, lc.LocalData1, lc.LocalData2)
}

// originToString converts origin value to string.
func originToString(o uint8) string {
	switch o {
	case 0:
		return format.OriginIGP
	case 1:
		return format.OriginEGP
	case 2:
		return format.OriginIncomplete
	default:
		return fmt.Sprintf("%d", o)
	}
}

// parseCommonAttributeText parses a common BGP attribute by keyword into parsedAttrs.
// Returns the number of args consumed (0 if keyword not handled), or error.
func parseCommonAttributeText(key string, args []string, idx int, attrs *parsedAttrs) (int, error) {
	switch key {
	case kwOrigin:
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing origin value")
		}
		// YANG validation for origin enum (single source of truth)
		if plugin.YANGValidator() != nil {
			if err := plugin.YANGValidator().Validate(yangPathOrigin, args[idx+1]); err != nil {
				return 0, fmt.Errorf("invalid origin: %w", err)
			}
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
		// YANG validation for local-preference uint32 (single source of truth)
		if plugin.YANGValidator() != nil {
			if err := plugin.YANGValidator().Validate(yangPathLocalPref, lpVal); err != nil {
				return 0, fmt.Errorf("invalid local-preference: %w", err)
			}
		}
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
		// YANG validation for MED uint32 (single source of truth)
		if plugin.YANGValidator() != nil {
			if err := plugin.YANGValidator().Validate(yangPathMED, medVal); err != nil {
				return 0, fmt.Errorf("invalid med: %w", err)
			}
		}
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

	case kwCommunity:
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

	case kwLargeCommunity:
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing large-community value")
		}
		tokens, consumed := parseBracketedListText(args[idx+1:])
		lcs := make([]bgptypes.LargeCommunity, 0, len(tokens))
		for _, tok := range tokens {
			lc, err := attribute.ParseLargeCommunity(tok)
			if err != nil {
				return 0, err
			}
			lcs = append(lcs, lc)
		}
		attrs.LargeCommunities = lcs
		return consumed, nil

	case kwExtendedCommunity:
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing extended-community value")
		}
		// Use route.ParseExtendedCommunities which handles both function syntax
		// (traffic-rate, discard, redirect, traffic-marking) and list syntax.
		ecs, consumed, err := route.ParseExtendedCommunities(args[idx+1:])
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
			for tok := range strings.SplitSeq(args[i], ",") {
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
		for tok := range strings.SplitSeq(inner, " ") {
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
		for tok := range strings.SplitSeq(firstVal, ",") {
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
			if before, ok := strings.CutSuffix(arg, "]"); ok {
				// Last token - strip trailing bracket
				lastVal := before
				for tok := range strings.SplitSeq(lastVal, ",") {
					tok = strings.TrimSpace(tok)
					if tok != "" {
						tokens = append(tokens, tok)
					}
				}
				return tokens, consumed
			}
			// Middle tokens
			for tok := range strings.SplitSeq(arg, ",") {
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
	for tok := range strings.SplitSeq(first, ",") {
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

// ParseUpdateText parses the "update text" command format.
// Returns the parsed result or an error.
func ParseUpdateText(args []string) (*bgptypes.UpdateTextResult, error) {
	var accum parsedAttrs
	var groups []bgptypes.NLRIGroup
	var eorFamilies []nlri.Family
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
			result, err := parseNLRISection(args[i:], nlriAcc)
			if err != nil {
				return nil, err
			}

			// RFC 4724: EOR is signaled by valid family with empty announce/withdraw lists
			if len(result.Announce) == 0 && len(result.Withdraw) == 0 && result.Family.AFI != 0 {
				eorFamilies = append(eorFamilies, result.Family)
			} else {
				groups = append(groups, bgptypes.NLRIGroup{
					Family:       result.Family,
					Announce:     result.Announce,
					Withdraw:     result.Withdraw,
					Wire:         wire,
					NextHop:      nh,
					WatchdogName: result.Watchdog,
				})
				// Also set global watchdog if specified in nlri section (for backward compat)
				if result.Watchdog != "" {
					watchdog = result.Watchdog
				}
			}
			i += result.Consumed

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

	return &bgptypes.UpdateTextResult{Groups: groups, WatchdogName: watchdog, EORFamilies: eorFamilies}, nil
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
			if uint64(accum.PathID) != id { //nolint:gosec // G115: bounded by ParseUint 32-bit
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
			if len(accum.Labels) != 1 || uint64(accum.Labels[0]) != label { //nolint:gosec // G115: bounded by ParseUint
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
		return "", parsedAttrs{}, 0, route.ErrMissingAttrMode
	}
	mode := args[1]
	if mode != kwSet && mode != kwAdd && mode != kwDel {
		return "", parsedAttrs{}, 0, route.ErrInvalidAttrMode
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
		return "", parsedAttrs{}, 0, fmt.Errorf("%s: %w", attrName, route.ErrAddOnScalar)
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

// updateRPCs returns RPC registrations for handlers defined in this file.
// Part of the ze-bgp module — aggregated by BgpHandlerRPCs().
func UpdateRPCs() []plugin.RPCRegistration {
	return []plugin.RPCRegistration{
		{WireMethod: "ze-bgp:peer-update", CLICommand: "bgp peer update", Handler: handleUpdate, Help: "Batch UPDATE with text/hex/b64 encoding"},
	}
}

// handleUpdate dispatches update subcommands by encoding.
// Syntax: peer <addr> update <encoding> ...
func handleUpdate(ctx *plugin.CommandContext, args []string) (*plugin.Response, error) {
	_, errResp, err := plugin.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

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
// RFC 4724 Section 2: End-of-RIB marker.
func handleUpdateText(ctx *plugin.CommandContext, args []string) (*plugin.Response, error) {
	result, err := ParseUpdateText(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, err
	}

	if result.WatchdogName != "" {
		errMsg := "watchdog not yet implemented for update text"
		return &plugin.Response{Status: plugin.StatusError, Data: errMsg}, errors.New(errMsg)
	}

	// BGP-specific operations: EOR, announce, withdraw
	bgpReactor, errResp, bgpErr := requireBGPReactor(ctx)
	if bgpErr != nil {
		return errResp, bgpErr
	}

	// Handle EOR markers (RFC 4724)
	peerSelector := ctx.PeerSelector()
	var eorSent int
	for _, family := range result.EORFamilies {
		if err := bgpReactor.AnnounceEOR(peerSelector, uint16(family.AFI), uint8(family.SAFI)); err != nil {
			return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, err
		}
		eorSent++
	}

	// If only EOR (no NLRI groups), return early
	if len(result.Groups) == 0 {
		if eorSent > 0 {
			return &plugin.Response{
				Status: plugin.StatusDone,
				Data: map[string]any{
					"eor": eorSent,
				},
			}, nil
		}
		return &plugin.Response{
			Status: "warning",
			Data:   "no routes or EOR markers to send",
		}, nil
	}

	resp, err := DispatchNLRIGroups(ctx, result.Groups)
	if err != nil {
		return resp, err
	}

	// Add EOR count to response if both were sent
	if eorSent > 0 {
		if respData, ok := resp.Data.(map[string]any); ok {
			respData["eor"] = eorSent
		}
	}

	return resp, nil
}

// DispatchNLRIGroups sends NLRI groups to the reactor for announce/withdraw.
// Returns response with counts and any warnings, or error response.
func DispatchNLRIGroups(ctx *plugin.CommandContext, groups []bgptypes.NLRIGroup) (*plugin.Response, error) {
	bgpReactor, errResp, bgpErr := requireBGPReactor(ctx)
	if bgpErr != nil {
		return errResp, bgpErr
	}

	peerSelector := ctx.PeerSelector()
	var announced, withdrawn int
	var warnings []string

	for _, group := range groups {
		if len(group.Announce) > 0 {
			batch := bgptypes.NLRIBatch{
				Family:  group.Family,
				NLRIs:   group.Announce,
				NextHop: group.NextHop,
				Wire:    group.Wire,
			}
			if err := bgpReactor.AnnounceNLRIBatch(peerSelector, batch); err != nil {
				if errors.Is(err, route.ErrNoPeersAcceptedFamily) {
					warnings = append(warnings, fmt.Sprintf("announce %v: %s", group.Family, err))
					continue
				}
				return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, err
			}
			announced += len(group.Announce)
		}
		if len(group.Withdraw) > 0 {
			batch := bgptypes.NLRIBatch{
				Family: group.Family,
				NLRIs:  group.Withdraw,
			}
			if err := bgpReactor.WithdrawNLRIBatch(peerSelector, batch); err != nil {
				if errors.Is(err, route.ErrNoPeersAcceptedFamily) {
					warnings = append(warnings, fmt.Sprintf("withdraw %v: %s", group.Family, err))
					continue
				}
				return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, err
			}
			withdrawn += len(group.Withdraw)
		}
	}

	if announced == 0 && withdrawn == 0 {
		msg := "no routes to announce or withdraw"
		if len(warnings) > 0 {
			msg = strings.Join(warnings, "; ")
		}
		return &plugin.Response{
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

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   respData,
	}, nil
}
