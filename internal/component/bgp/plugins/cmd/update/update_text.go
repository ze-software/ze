// Design: docs/architecture/api/commands.md — text UPDATE parsing handlers
// Overview: doc.go — bgp-cmd-update plugin registration
// Detail: update_text_nlri.go — NLRI section parsing
// Detail: update_text_evpn.go — EVPN NLRI parsing
// Detail: update_text_flowspec.go — FlowSpec NLRI parsing
// Detail: update_text_vpls.go — VPLS NLRI parsing
//
// update_text.go provides the update text parser for the "update text" command format.
//
// Grammar (flat — no set/add/del on attributes):
//
//	<update-text>  := <attribute>* <nlri-section>+
//	<attribute>    := <attr-name> <value>
//	<attr-name>    := origin | med | local-preference | as-path | community |
//	                  large-community | extended-community | nhop | path-information | rd | label
//	<nlri-section> := nlri <family> <nlri-op>+
//	<nlri-op>      := add <prefix>+ [watchdog <name>] | del <prefix>+
//
// Attributes are flat declarations (keyword + value). No set/add/del on attributes.
// add/del are NLRI-only keywords (MP_REACH vs MP_UNREACH).
// Attributes must precede all nlri sections (no interleaving).
//
// Note: rd and label are ignored for families that don't support them.
package update

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/route"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/textparse"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
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
	kwNhop     = "nhop"             // Top-level next-hop keyword.
	kwPathInfo = "path-information" // ADD-PATH path-id keyword.
)

// UpdateText action keywords (NLRI-only: add=MP_REACH, del=MP_UNREACH).
const (
	kwAdd = "add"
	kwDel = "del"
	kwSet = "set" // Rejected with migration hint — kept for detection only.
	kwEOR = "eor" // End-of-RIB marker (RFC 4724).
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

// Structure keywords for NLRI modifiers.
const (
	kwRD    = "rd"    // Route Distinguisher for VPN families.
	kwLabel = "label" // MPLS label for VPN/labeled families.
)

// isAttributeKeyword returns true if token is a per-attribute keyword.
func isAttributeKeyword(token string) bool {
	switch token {
	case kwOrigin, kwMED, kwLocalPref, kwASPath,
		kwCommunity, kwLargeCommunity, kwExtendedCommunity:
		return true
	}
	return false
}

// isBoundaryKeyword returns true if token starts a new top-level section.
// Used by NLRI sub-parsers (EVPN, FlowSpec, VPLS) to detect section boundaries.
// Tokens are already alias-resolved (nhop→next-hop, path-id→path-information).
func isBoundaryKeyword(token string) bool {
	switch token {
	case kwAttr, kwNLRI, kwWatchdog, textparse.KWNextHop, kwPathInfo, kwRD, kwLabel:
		return true
	}
	return isAttributeKeyword(token)
}

// parsedAttrs collects attribute declarations during flat parsing.
// Includes next-hop which is NOT part of path attributes.
// Path-id moved to per-NLRI-section modifier (in nlriAccum).
type parsedAttrs struct {
	NextHop     netip.Addr
	NextHopSelf bool

	// Path attributes (wire-first: build directly to wire format).
	Origin              *uint8
	LocalPreference     *uint32
	MED                 *uint32
	ASPath              []uint32
	Communities         []uint32
	LargeCommunities    []bgptypes.LargeCommunity
	ExtendedCommunities []attribute.ExtendedCommunity

	// VPN/labeled NLRI modifiers.
	RD     nlri.RouteDistinguisher // Route Distinguisher for VPN families.
	Labels []uint32                // MPLS labels for VPN/labeled families.
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
	return wire, nh, nlriAccum{RD: a.RD, Labels: labels}
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

// errAttrsAfterNLRI is returned when attributes appear after the first nlri section.
var errAttrsAfterNLRI = errors.New("attributes must precede all nlri sections")

// resolveAliases returns a copy of args with all keyword aliases resolved to canonical forms.
// Uses textparse.ResolveAlias so "next"→"next-hop", "pref"→"local-preference", etc.
func resolveAliases(args []string) []string {
	resolved := make([]string, len(args))
	for i, token := range args {
		resolved[i] = textparse.ResolveAlias(token)
	}
	return resolved
}

// ParseUpdateText parses the "update text" command format.
// Flat grammar: attributes are keyword-value pairs (no set/add/del).
// Attributes must all precede the nlri sections.
// All keyword aliases are resolved before parsing (next→next-hop, pref→local-preference, etc.).
func ParseUpdateText(args []string) (*bgptypes.UpdateTextResult, error) {
	args = resolveAliases(args)

	var attrs parsedAttrs
	var groups []bgptypes.NLRIGroup
	var eorFamilies []nlri.Family
	var watchdog string
	seenNLRI := false
	i := 0

	for i < len(args) {
		token := args[i] //nolint:gosec // G602 false positive: loop condition guards access

		switch token {
		case kwNLRI:
			seenNLRI = true
			wire, nh, nlriAcc := attrs.snapshot()
			result, err := parseNLRISection(args[i:], nlriAcc)
			if err != nil {
				return nil, err
			}

			// RFC 4724: EOR is signaled by valid family with empty announce/withdraw lists.
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
				if result.Watchdog != "" {
					watchdog = result.Watchdog
				}
			}
			i += result.Consumed

		case kwWatchdog:
			if i+1 >= len(args) {
				return nil, errors.New("missing watchdog name")
			}
			watchdog = args[i+1]
			i += 2

		case textparse.KWNextHop:
			if seenNLRI {
				return nil, errAttrsAfterNLRI
			}
			consumed, err := parseNhopFlat(args[i:], &attrs)
			if err != nil {
				return nil, err
			}
			i += consumed

		case kwRD:
			if seenNLRI {
				return nil, errAttrsAfterNLRI
			}
			consumed, err := parseRDFlat(args[i:], &attrs)
			if err != nil {
				return nil, err
			}
			i += consumed

		case kwLabel:
			if seenNLRI {
				return nil, errAttrsAfterNLRI
			}
			consumed, err := parseLabelFlat(args[i:], &attrs)
			if err != nil {
				return nil, err
			}
			i += consumed

		default: // attribute keywords and unknown tokens
			if isAttributeKeyword(token) {
				if seenNLRI {
					return nil, errAttrsAfterNLRI
				}
				// Reject old set/add/del syntax with migration hint.
				if i+1 < len(args) && (args[i+1] == kwSet || args[i+1] == kwAdd || args[i+1] == kwDel) {
					return nil, fmt.Errorf("%s keyword removed for attributes; use: %s <value>", args[i+1], token)
				}
				// Flat attribute parsing: keyword + value(s).
				extra, err := parseCommonAttributeText(token, args, i, &attrs)
				if err != nil {
					return nil, err
				}
				if extra == 0 {
					return nil, fmt.Errorf("missing value for %s", token)
				}
				i += 1 + extra
				continue
			}
			return nil, fmt.Errorf("unexpected token '%s'; valid: origin, med, local-preference (pref), as-path (path), community (s-com), large-community (l-com), extended-community (x-com), next-hop (next), path-information (info), rd, label, nlri, watchdog", token)
		}
	}

	return &bgptypes.UpdateTextResult{Groups: groups, WatchdogName: watchdog, EORFamilies: eorFamilies}, nil
}

// parseNhopFlat parses next-hop <address|self> (flat, no set/del).
func parseNhopFlat(args []string, accum *parsedAttrs) (int, error) {
	// args[0] = "next-hop" (alias-resolved)
	if len(args) < 2 {
		return 0, errors.New("next-hop requires a value (address or self)")
	}
	// Reject old set/del syntax.
	if args[1] == kwSet || args[1] == kwDel {
		return 0, fmt.Errorf("set/del keywords removed; use: next-hop <address|self>")
	}
	value := args[1]
	if value == kwSelf {
		accum.NextHopSelf = true
		accum.NextHop = netip.Addr{}
		return 2, nil
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return 0, fmt.Errorf("invalid next-hop: %w", err)
	}
	accum.NextHop = addr
	accum.NextHopSelf = false
	return 2, nil
}

// parseRDFlat parses rd <value> (flat, no set/del).
// RD format: ASN:NN or IP:NN (e.g., "65000:100" or "192.0.2.1:100").
func parseRDFlat(args []string, accum *parsedAttrs) (int, error) {
	// args[0] = "rd"
	if len(args) < 2 {
		return 0, errors.New("rd requires a value (ASN:NN or IP:NN)")
	}
	if args[1] == kwSet || args[1] == kwDel {
		return 0, fmt.Errorf("set/del keywords removed; use: rd <value>")
	}
	rd, err := nlri.ParseRDString(args[1])
	if err != nil {
		return 0, fmt.Errorf("invalid rd: %w", err)
	}
	accum.RD = rd
	return 2, nil
}

// parseLabelFlat parses label <value> (flat, no set/del).
// Label is a single MPLS label value (0-1048575).
func parseLabelFlat(args []string, accum *parsedAttrs) (int, error) {
	// args[0] = "label"
	if len(args) < 2 {
		return 0, errors.New("label requires a value (0-1048575)")
	}
	if args[1] == kwSet || args[1] == kwDel {
		return 0, fmt.Errorf("set/del keywords removed; use: label <value>")
	}
	label, err := strconv.ParseUint(args[1], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid label: %w", err)
	}
	if label > 0xFFFFF { // 20-bit max
		return 0, fmt.Errorf("label out of range (max 1048575): %d", label)
	}
	accum.Labels = []uint32{uint32(label)} //nolint:gosec // G115: bounded by check above
	return 2, nil
}

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-update", Handler: handleUpdate, RequiresSelector: true},
	)
}

// handleUpdate dispatches update subcommands by encoding.
// Syntax: peer <addr> update <encoding> ...
func handleUpdate(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
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
func handleUpdateText(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
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
func DispatchNLRIGroups(ctx *pluginserver.CommandContext, groups []bgptypes.NLRIGroup) (*plugin.Response, error) {
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
