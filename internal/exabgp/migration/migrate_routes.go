// Design: docs/architecture/core-design.md — ExaBGP route conversion to Ze update blocks
// Overview: migrate.go — migration orchestration and neighbor conversion
// Related: migrate_family.go — family syntax conversion helpers
// Related: migrate_serialize.go — tree serialization

package migration

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/exabgp/bridge"
)

const (
	routeAttributeNextHop           = "next-hop"
	routeAttributeLocalPreference   = "local-preference"
	routeAttributeMED               = "med"
	routeAttributeExtendedCommunity = "extended-community"
	routeAttributeLargeCommunity    = "large-community"
	routeAttributeCommunity         = "community"
	routeAttributeOriginatorID      = "originator-id"
	routeAttributeASPath            = "as-path"
	routeAttributeClusterList       = "cluster-list"
)

// flexAttrKeywords lists path attribute keywords that go in the attribute block
// when converting flex container entries (mcast-vpn, mup) to update blocks.
// Everything else is treated as NLRI fields.
var flexAttrKeywords = map[string]bool{
	routeAttributeNextHop:           true,
	"origin":                        true,
	routeAttributeLocalPreference:   true,
	routeAttributeMED:               true,
	routeAttributeExtendedCommunity: true,
	routeAttributeLargeCommunity:    true,
	routeAttributeCommunity:         true,
	routeAttributeOriginatorID:      true,
	routeAttributeClusterList:       true,
	routeAttributeASPath:            true,
	"bgp-prefix-sid-srv6":           true,
}

// convertAnnounceToUpdate converts ExaBGP announce blocks to Ze update blocks.
// ExaBGP: announce { ipv4 { unicast PREFIX next-hop NH; } }.
// Ze: update { attribute { next-hop NH; } nlri { ipv4/unicast PREFIX; } }.
//
// It reports an attribute ExaBGP itself refuses (checkExtCommunityAttr).
func convertAnnounceToUpdate(announce, dst *config.Tree) error {
	// Process each AFI (ipv4, ipv6, l2vpn)
	for _, afi := range []string{"ipv4", "ipv6", "l2vpn"} {
		afiBlock := announce.GetContainer(afi)
		if afiBlock == nil {
			continue
		}

		// Process each SAFI (unicast, multicast, nlri-mpls, mpls-vpn, flow, vpls, evpn)
		safis := []string{"unicast", "multicast", safiNLRIMPLS, "mpls-vpn", "flow", safiVPLS, "evpn"}
		for _, safi := range safis {
			// With ze:syntax "inline-list", routes are stored as list entries, not container values.
			// Each list entry has: Key=prefix, Value=Tree containing attributes.
			routeList := afiBlock.GetListOrdered(safi)
			if len(routeList) == 0 {
				continue
			}

			// Convert fam name
			var tb textbuf.Buffer
			fam := tb.Str(afi).Byte('/').Str(safi).String()

			// Process each route entry
			for _, routeEntry := range routeList {
				prefix := config.StripListKeySuffix(routeEntry.Key)
				attrTree := routeEntry.Value
				if err := checkExtCommunityAttr(attrTree); err != nil {
					return fmt.Errorf("route %s: %w", prefix, err)
				}

				update := config.NewTree()

				// Build attribute block from route's attribute tree
				attrBlock := config.NewTree()

				// Default origin to igp if not specified
				if origin, ok := attrTree.Get("origin"); ok {
					attrBlock.Set("origin", origin)
				} else {
					attrBlock.Set("origin", "igp")
				}

				// Copy common attributes from the route's tree
				attrFields := []string{routeAttributeNextHop, routeAttributeLocalPreference, routeAttributeMED, routeAttributeASPath, routeAttributeCommunity,
					routeAttributeExtendedCommunity, routeAttributeLargeCommunity, "aggregator", routeAttributeOriginatorID, routeAttributeClusterList,
					"rd", "label", "labels", "path-information"}
				for _, field := range attrFields {
					if v, ok := attrTree.Get(field); ok {
						attrBlock.Set(field, v)
					}
				}
				mapIPv6RouteIPv4NextHop(prefix, attrTree, attrBlock)

				update.SetContainer("attribute", attrBlock)

				// Build nlri list entry
				nlriEntry := config.NewTree()
				nlriEntry.Set("content", "add "+prefix)
				update.AddListEntry("nlri", fam, nlriEntry)

				// Add update to dst as list entry
				dst.AddListEntry("update", "", update)
			}
		}

		// Handle flex SAFIs (mcast-vpn, mup, vpls) which store values via AppendValue.
		// These use ze:syntax "flex" in the ExaBGP YANG schema, so GetListOrdered returns nothing.
		flexSafis := []string{"mcast-vpn", "mup", safiVPLS, "sr-policy"}
		for _, safi := range flexSafis {
			values := afiBlock.GetMultiValues(safi)
			if len(values) == 0 {
				continue
			}
			if err := convertFlexToUpdate(afi, safi, values, dst); err != nil {
				return err
			}
		}
	}

	// Handle announce { route PREFIX ... } (generic route syntax)
	// This uses inline-list as well, so use GetListOrdered.
	routeList := announce.GetListOrdered("route")
	for _, routeEntry := range routeList {
		if err := convertRouteToUpdate(routeEntry.Key, routeEntry.Value, dst); err != nil {
			return err
		}
	}
	return nil
}

// convertStaticToUpdate converts ExaBGP static blocks to Ze update blocks.
// ExaBGP: static { route PREFIX next-hop NH; }.
// Ze: update { attribute { next-hop NH; } nlri { ipv4/unicast PREFIX; } }.
func convertStaticToUpdate(static, dst *config.Tree) error {
	// With ze:syntax "inline-list", routes are stored as list entries.
	routeList := static.GetListOrdered("route")
	for _, routeEntry := range routeList {
		if err := convertRouteToUpdate(routeEntry.Key, routeEntry.Value, dst); err != nil {
			return err
		}
	}
	return nil
}

// convertFlowToUpdate converts ExaBGP flow blocks to Ze update blocks.
// ExaBGP: flow { route NAME { match { criteria; } scope { interface-set [ ... ]; } then { actions; } } }
// Ze: update { attribute { extended-community ...; } nlri { ipv4/flow criterion value ...; } }.
//
// It reports the error of a scope block ze cannot express, so a filter is never
// announced with a wider reach than the ExaBGP config gave it.
func convertFlowToUpdate(flow, dst *config.Tree) error {
	for _, entry := range flow.GetListOrdered("route") {
		route := entry.Value

		rd, _ := route.Get("rd")
		nextHop, _ := route.Get(routeAttributeNextHop)

		// Parse match criteria and detect IPv6.
		isIPv6 := false
		var nlriCriteria []string
		if match := route.GetContainer("match"); match != nil {
			for _, key := range match.Values() {
				val, _ := match.Get(key)
				criterion, value := parseFlowMatchEntry(key, val)
				if criterion == "" {
					continue
				}
				// Detect IPv6 by address content.
				if (criterion == "source" || criterion == "destination") && strings.Contains(value, ":") {
					isIPv6 = true
				}
				// Add AFI suffix to source/destination.
				nlriCriteria = append(nlriCriteria, flowCriterionWithValues(criterion, value, isIPv6))
			}
		}

		// Parse scope block → interface-set extended communities. They are
		// written BEFORE the then block's traffic actions because ExaBGP sorts
		// its extended communities by value before it packs them
		// (bgp/message/update/attribute/community/extended/communities.py,
		// pack), and an interface set's type octet (0x07 or 0x47) sorts before
		// a traffic action's (0x80). Writing them first reproduces that order.
		var extComms []string
		if scope := route.GetContainer("scope"); scope != nil {
			scopeComms, err := flowScopeExtCommunities(entry.Key, scope)
			if err != nil {
				return err
			}
			extComms = append(extComms, scopeComms...)
		}

		// Parse then block → attributes.
		var community string
		var rawAttr string
		if then := route.GetContainer("then"); then != nil {
			for _, key := range then.Values() {
				val, _ := then.Get(key)
				action, value := parseFlowMatchEntry(key, val)
				switch action {
				case "discard":
					extComms = append(extComms, "rate-limit:0")
				case "rate-limit":
					rate, unit, hasUnit := strings.Cut(value, " ")
					if hasUnit {
						switch unit {
						case "packets":
							extComms = append(extComms, "rate-limit:"+rate+":packets")
						case "bytes":
							extComms = append(extComms, "rate-limit:"+rate)
						default:
							extComms = append(extComms, "rate-limit:"+value)
						}
					} else {
						extComms = append(extComms, "rate-limit:"+value)
					}
				case "rate-limit-packets":
					extComms = append(extComms, "rate-limit:"+value+":packets")
				case exaRedirect, exaRedirectSimpson, exaCopy, exaCopySimpson,
					exaRedirectToNextHop, exaRedirectToNextHopIETF, exaRedirectToNextHopSimpson:
					redirect, err := flowRedirectAction(action, value)
					if err != nil {
						return fmt.Errorf("flow route %s: %w", entry.Key, err)
					}
					if redirect.nextHop != "" {
						nextHop = redirect.nextHop
					}
					extComms = append(extComms, redirect.extCommunity)
				case "mark":
					extComms = append(extComms, "mark "+value)
				case "action":
					extComms = append(extComms, "action "+value)
				case routeAttributeCommunity:
					community = strings.Trim(value, "[] ")
				case routeAttributeExtendedCommunity:
					// Inline extended communities: "[ origin:... origin:... ]"
					if err := bridge.CheckExtendedCommunityList(value); err != nil {
						return fmt.Errorf("flow route %s: %w", entry.Key, err)
					}
					inner := strings.Trim(value, "[] ")
					extComms = append(extComms, strings.Fields(inner)...)
				case "attribute":
					rawAttr = value
				}
			}
		}

		// Determine family.
		fam := "ipv4/flow"
		if isIPv6 {
			fam = "ipv6/flow"
		}
		if rd != "" {
			if isIPv6 {
				fam = "ipv6/flow-vpn"
			} else {
				fam = "ipv4/flow-vpn"
			}
		}

		// Build NLRI content: [rd <rd>] add <criteria...>
		// FlowSpec-VPN parser expects rd before operation keyword.
		var nlriContent textbuf.Buffer
		if rd != "" {
			nlriContent.Str("rd ").Str(rd).Byte(' ')
		}
		nlriContent.Str("add")
		for _, c := range nlriCriteria {
			nlriContent.Byte(' ').Str(c)
		}

		// Build update block.
		update := config.NewTree()

		attrBlock := config.NewTree()
		if nextHop != "" {
			attrBlock.Set(routeAttributeNextHop, nextHop)
		}
		if len(extComms) > 0 {
			var tb textbuf.Buffer
			attrBlock.Set(routeAttributeExtendedCommunity, tb.Byte('[').Join(extComms, " ").Byte(']').String())
		}
		if community != "" {
			var tb textbuf.Buffer
			attrBlock.Set(routeAttributeCommunity, tb.Byte('[').Str(community).Byte(']').String())
		}
		if rawAttr != "" {
			attrBlock.Set("attribute", rawAttr)
		}
		update.SetContainer("attribute", attrBlock)

		nlriEntry := config.NewTree()
		nlriEntry.Set("content", nlriContent.String())
		update.AddListEntry("nlri", fam, nlriEntry)

		dst.AddListEntry("update", "", update)
	}

	return nil
}

// convertL2VPNToUpdate converts neighbor-level l2vpn blocks to Ze update blocks.
// Handles both inline VPLS routes (flex multi-values) and named VPLS blocks (list entries).
func convertL2VPNToUpdate(l2vpn, dst *config.Tree) error {
	// Handle inline VPLS routes (stored via AppendValue by the flex parser).
	vplsValues := l2vpn.GetMultiValues(safiVPLS)
	if len(vplsValues) > 0 {
		if err := convertFlexToUpdate("l2vpn", safiVPLS, vplsValues, dst); err != nil {
			return err
		}
	}

	// Handle named VPLS routes (stored via AddListEntry by the flex parser).
	for _, entry := range l2vpn.GetListOrdered(safiVPLS) {
		if err := convertNamedVPLSToUpdate(entry.Value, dst); err != nil {
			return err
		}
	}
	return nil
}

// convertNamedVPLSToUpdate converts a named VPLS block (e.g., vpls site5 { ... })
// to a Ze update block. The block's Tree has parsed fields like endpoint, base, rd, etc.
func convertNamedVPLSToUpdate(vpls, dst *config.Tree) error {
	if err := checkExtCommunityAttr(vpls); err != nil {
		return fmt.Errorf("vpls: %w", err)
	}
	update := config.NewTree()

	attrBlock := config.NewTree()

	// Extract path attributes from the VPLS block.
	// Simple values (single word) are stored as-is.
	simpleFields := []string{routeAttributeNextHop, "origin", routeAttributeLocalPreference, routeAttributeMED, routeAttributeOriginatorID}
	for _, field := range simpleFields {
		if v, ok := vpls.Get(field); ok {
			attrBlock.Set(field, v)
		}
	}
	// Array fields: value-or-array strips brackets, so re-wrap multi-word values.
	arrayFields := []string{routeAttributeASPath, routeAttributeCommunity, routeAttributeExtendedCommunity, routeAttributeLargeCommunity, routeAttributeClusterList}
	for _, field := range arrayFields {
		if v, ok := vpls.Get(field); ok {
			if strings.Contains(v, " ") && !strings.HasPrefix(v, "[") {
				var tb textbuf.Buffer
				v = tb.Byte('[').Str(v).Byte(']').String()
			}
			attrBlock.Set(field, v)
		}
	}
	if _, ok := attrBlock.Get("origin"); !ok {
		attrBlock.Set("origin", "igp")
	}
	update.SetContainer("attribute", attrBlock)

	// Build NLRI line from VPLS-specific fields.
	// Format: "l2vpn/vpls [rd X] add endpoint Y base Z offset A size B"
	// VPLS parser expects rd before operation keyword.
	var nlriParts []string
	if v, ok := vpls.Get("rd"); ok {
		nlriParts = append(nlriParts, "rd", v)
	}
	nlriParts = append(nlriParts, "add")
	for _, field := range []string{"endpoint", "ve-id", "base", "offset", "ve-block-offset", "size", "ve-block-size"} {
		if v, ok := vpls.Get(field); ok {
			nlriParts = append(nlriParts, field, v)
		}
	}

	nlriEntry := config.NewTree()
	nlriEntry.Set("content", textbuf.Join(nlriParts, " "))
	update.AddListEntry("nlri", "l2vpn/vpls", nlriEntry)

	dst.AddListEntry("update", "", update)
	return nil
}

// convertRouteToUpdate converts a single ExaBGP route to a Ze update block.
// prefix is the NLRI prefix (e.g., "10.0.0.0/24").
// attrTree contains the route attributes from ExaBGP.
// dst is the peer block where the update will be added.
//
// Family detection: rd present → mpls-vpn, label present (no rd) → nlri-mpls, else → unicast.
// RD and label go inline in the NLRI (config loader expects them there, not in attribute block).
func convertRouteToUpdate(prefix string, attrTree, dst *config.Tree) error {
	prefix = config.StripListKeySuffix(prefix)
	if err := checkExtCommunityAttr(attrTree); err != nil {
		return fmt.Errorf("route %s: %w", prefix, err)
	}
	update := config.NewTree()

	attrBlock := config.NewTree()
	attrBlock.Set("origin", "igp")

	// Path attributes that go in the attribute block.
	// Note: rd and label are NOT here — they go inline in the NLRI line.
	attrFields := []string{routeAttributeNextHop, routeAttributeLocalPreference, routeAttributeMED, routeAttributeASPath, routeAttributeCommunity,
		routeAttributeExtendedCommunity, routeAttributeLargeCommunity, "aggregator", routeAttributeOriginatorID, routeAttributeClusterList,
		"path-information", "labels", "split"}
	for _, field := range attrFields {
		if v, ok := attrTree.Get(field); ok {
			attrBlock.Set(field, v)
		}
	}
	mapIPv6RouteIPv4NextHop(prefix, attrTree, attrBlock)

	// BGP Prefix-SID (RFC 8669) — inline-list parser stores bracketed values via Set().
	if v, ok := attrTree.Get("bgp-prefix-sid"); ok {
		attrBlock.Set("bgp-prefix-sid", v)
	}

	// Handle atomic-aggregate (flag attribute stored as "true")
	if v, ok := attrTree.Get("atomic-aggregate"); ok && v == configTrue {
		attrBlock.Set("atomic-aggregate", configTrue)
	}

	// Handle raw attribute bytes: "[0x20 0xc0 0x...]" → "0x20 0xc0 0x..."
	if v, ok := attrTree.Get("attribute"); ok {
		// Strip brackets from array syntax
		v = strings.TrimPrefix(v, "[")
		v = strings.TrimSuffix(v, "]")
		attrBlock.Set("attribute", v)
	}

	update.SetContainer("attribute", attrBlock)

	// Detect fam from prefix format and route attributes.
	isIPv6 := strings.Contains(prefix, ":")
	rdVal, hasRD := attrTree.Get("rd")
	labelVal, hasLabel := attrTree.Get("label")

	fam := detectRouteFamily(isIPv6, hasRD, hasLabel)

	// Build NLRI value with inline rd/label (config loader parses these from NLRI line).
	// Format: add [rd VALUE] [label VALUE] prefix
	var tb textbuf.Buffer
	tb.Str("add ")
	if hasRD {
		tb.Str("rd ").Str(rdVal).Byte(' ')
	}
	if hasLabel {
		tb.Str("label ").Str(labelVal).Byte(' ')
	}
	tb.Str(prefix)
	nlriValue := tb.String()

	nlriEntry := config.NewTree()
	nlriEntry.Set("content", nlriValue)
	update.AddListEntry("nlri", fam, nlriEntry)

	// Handle watchdog attribute.
	// ExaBGP: "route PREFIX ... watchdog NAME withdraw;" stores watchdog=NAME, withdraw=true.
	// Ze: "update { watchdog { name NAME; } ... }"
	// Note: we omit "withdraw true" because in ExaBGP the watchdog process announces
	// before session establishment, so the route is effectively announced at session start.
	// The exabgp wrapper handles the process lifecycle via the API socket.
	if wdName, ok := attrTree.Get("watchdog"); ok {
		wdBlock := config.NewTree()
		wdBlock.Set("name", wdName)
		update.SetContainer("watchdog", wdBlock)
	}

	dst.AddListEntry("update", "", update)
	return nil
}

// checkExtCommunityAttr refuses an extended-community attribute holding an
// item ExaBGP refuses there (bridge.CheckExtendedCommunityList), so the
// migration stops rather than writing a route ExaBGP would never have sent.
func checkExtCommunityAttr(attrTree *config.Tree) error {
	value, ok := attrTree.Get(routeAttributeExtendedCommunity)
	if !ok {
		return nil
	}
	return bridge.CheckExtendedCommunityList(value)
}

// mapIPv6RouteIPv4NextHop rewrites the IPv4 next hop of an IPv6 route without
// a route distinguisher as its IPv4-mapped IPv6 address, as ExaBGP 6.0 sends it
// (685f772e4, qa/encoding/extended-nexthop.ci: `next-hop 170.170.170.170` on an
// IPv6 route goes out as ::ffff:170.170.170.170). ze refuses a 4-octet next hop
// for IPv6 NLRI, so without this the route would never be sent.
// RFC 2545 Section 3: the Length of Next Hop Network Address field "shall be
// set to 16, when only a global address is present, or 32 if a link-local
// address is also included in the Next Hop field". RFC 4798 Section 2: "The
// IPv4 address of the egress 6PE router MUST be encoded as an IPv4-mapped IPv6
// address in the BGP Next Hop field."
// A VPN route (rd set) is left alone: RFC 4659 carries its own next-hop form.
func mapIPv6RouteIPv4NextHop(prefix string, route, attrBlock *config.Tree) {
	if _, hasRD := route.Get("rd"); hasRD {
		return
	}
	// ExaBGP takes a bare address as a host route (the upstream
	// extended-nexthop.conf writes `route DEAD:BEEF:...:BEEF next-hop ...`),
	// so only the address part decides the family.
	address, _, _ := strings.Cut(prefix, "/")
	routeAddr, err := netip.ParseAddr(address)
	if err != nil {
		return // A malformed prefix is the config loader's to refuse.
	}
	if !routeAddr.Is6() {
		return
	}
	value, ok := attrBlock.Get(routeAttributeNextHop)
	if !ok {
		return
	}
	nextHop, err := netip.ParseAddr(value)
	if err != nil {
		return // "self" and other keywords are not addresses.
	}
	if !nextHop.Is4() {
		return
	}
	attrBlock.Set(routeAttributeNextHop, netip.AddrFrom16(nextHop.As16()).String())
}

// detectRouteFamily determines the BGP address family from route characteristics.
// rd present → mpls-vpn, label present (no rd) → mpls-label, else → unicast.
// Family names match Ze's canonical SAFI strings registered via family.MustRegister.
func detectRouteFamily(isIPv6, hasRD, hasLabel bool) string {
	if hasRD && isIPv6 {
		return "ipv6/mpls-vpn"
	}
	if hasRD {
		return "ipv4/mpls-vpn"
	}
	if hasLabel && isIPv6 {
		return "ipv6/mpls-label"
	}
	if hasLabel {
		return "ipv4/mpls-label"
	}
	if isIPv6 {
		return familyIPv6Unicast
	}
	return familyIPv4Unicast
}

// parseFlowMatchEntry splits a freeform entry into criterion and value.
// Freeform stores "keyword value" as key→"true", or "keyword" as key→"[ values ]".
func parseFlowMatchEntry(key, val string) (string, string) {
	if val == "true" || val == "" {
		parts := strings.SplitN(key, " ", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
		return key, ""
	}
	return key, val
}

// flowCriterionWithValues formats a FlowSpec criterion for the NLRI line.
// Adds -ipv4/-ipv6 suffix to source/destination, normalizes operators.
func flowCriterionWithValues(criterion, value string, isIPv6 bool) string {
	var tb textbuf.Buffer
	switch criterion {
	case "source":
		if isIPv6 {
			return tb.Str("source-ipv6 ").Str(value).String()
		}
		return tb.Str("source-ipv4 ").Str(value).String()
	case "destination":
		if isIPv6 {
			return tb.Str("destination-ipv6 ").Str(value).String()
		}
		return tb.Str("destination-ipv4 ").Str(value).String()
	default: // pass through other criteria unchanged
		if value == "" {
			return criterion
		}
		// Unwrap single-element bracket lists: "[ !=0 ]" → "!=0"
		if strings.HasPrefix(value, "[ ") && strings.HasSuffix(value, " ]") {
			inner := strings.Fields(value[2 : len(value)-2])
			if len(inner) == 1 {
				return tb.Str(criterion).Byte(' ').Str(inner[0]).String()
			}
		}
		return tb.Reset().Str(criterion).Byte(' ').Str(value).String()
	}
}

// convertFlexToUpdate converts flex container entries to update blocks.
// Flex SAFIs (mcast-vpn, mup) store entire route lines as strings via AppendValue().
// Each string contains both NLRI fields and path attributes intermixed.
// This function separates them into attribute { } and nlri { } blocks.
func convertFlexToUpdate(afi, safi string, values []string, dst *config.Tree) error {
	// Translate ExaBGP SAFI to Ze canonical SAFI for the family name in the
	// emitted nlri block. The flex value tokens themselves stay as-is; only
	// the family key needs to match what Ze's family registry registered.
	var tb textbuf.Buffer
	fam := tb.Str(afi).Byte('/').Str(canonicalSAFI(safi)).String()

	for _, value := range values {
		attrs, nlriParts := splitFlexAttrs(value)
		if err := bridge.CheckExtendedCommunityList(attrs[routeAttributeExtendedCommunity]); err != nil {
			return fmt.Errorf("%s %s: %w", afi, safi, err)
		}

		update := config.NewTree()

		// Build attribute block
		attrBlock := config.NewTree()
		if _, ok := attrs["origin"]; !ok {
			attrBlock.Set("origin", "igp")
		}
		for k, v := range attrs {
			attrBlock.Set(k, v)
		}
		update.SetContainer("attribute", attrBlock)

		// Build nlri block with add operation keyword.
		// VPLS parser expects rd before operation keyword; others expect add first.
		nlriEntry := config.NewTree()
		nlriEntry.Set("content", flexNLRIContent(safi, nlriParts))
		update.AddListEntry("nlri", fam, nlriEntry)

		dst.AddListEntry("update", "", update)
	}
	return nil
}

// flexNLRIContent builds NLRI content with the add operation keyword.
// VPLS parser expects rd before the operation keyword; MVPN/MUP expect add first.
func flexNLRIContent(safi string, nlriParts []string) string {
	if safi == safiVPLS {
		// Extract rd (if present) and place before add.
		var parts, rest []string
		for i := 0; i < len(nlriParts); i++ {
			if nlriParts[i] == "rd" && i+1 < len(nlriParts) {
				parts = append(parts, "rd", nlriParts[i+1])
				i++
			} else {
				rest = append(rest, nlriParts[i])
			}
		}
		parts = append(parts, "add")
		parts = append(parts, rest...)
		return textbuf.Join(parts, " ")
	}
	var tb textbuf.Buffer
	return tb.Str("add ").Join(nlriParts, " ").String()
}

// splitFlexAttrs separates a flex value string into path attributes and NLRI fields.
// Returns a map of attribute key→value and a slice of NLRI tokens.
// Attribute values with brackets [..] or parens (..) have the delimiters stripped.
func splitFlexAttrs(value string) (map[string]string, []string) {
	tokens := tokenizeFlexValue(value)
	attrs := make(map[string]string)
	var nlriParts []string

	i := 0
	for i < len(tokens) {
		token := tokens[i]
		// Skip backslash continuations — artifacts from multiline ExaBGP config parsing
		if token == `\` {
			i++
			continue
		}
		if flexAttrKeywords[token] {
			// Path attribute — next token is its value
			i++
			if i < len(tokens) {
				attrValue := tokens[i]
				// Bracket values [val1 val2]: keep brackets for multi-value (Ze value-or-array syntax),
				// strip for single-value (no spaces inside brackets).
				if strings.HasPrefix(attrValue, "[") && strings.HasSuffix(attrValue, "]") {
					inner := attrValue[1 : len(attrValue)-1]
					if !strings.Contains(inner, " ") {
						// Single value inside brackets — strip them
						attrValue = inner
					}
					// Multi-value: keep brackets as-is for Ze "[ val1 val2 ]" syntax
				}
				// Paren values (val1 val2): ExaBGP grouping — strip parens, content is the value.
				// Flex syntax handles "key value key value" natively without outer brackets.
				if strings.HasPrefix(attrValue, "(") && strings.HasSuffix(attrValue, ")") {
					attrValue = attrValue[1 : len(attrValue)-1]
				}
				attrs[token] = strings.TrimSpace(attrValue)
			}
			i++
		} else {
			nlriParts = append(nlriParts, token)
			i++
		}
	}
	return attrs, nlriParts
}

// tokenizeFlexValue splits a flex value string into tokens, grouping bracketed [...]
// and parenthesized (...) sequences as single atomic tokens.
// The flex parser (parseFlex value mode) joins brackets/parens with their content,
// so "[target:10:10 mup:10:10]" may split into ["[target:10:10", "mup:10:10]"]
// after strings.Fields. This function regroups them.
func tokenizeFlexValue(s string) []string {
	fields := strings.Fields(s)
	var tokens []string

	i := 0
	for i < len(fields) {
		f := fields[i]

		switch {
		case strings.HasPrefix(f, "["):
			// Collect until matching ]
			var parts []string
			for i < len(fields) {
				parts = append(parts, fields[i])
				if strings.HasSuffix(fields[i], "]") {
					i++
					break
				}
				i++
			}
			tokens = append(tokens, textbuf.Join(parts, " "))
		case strings.HasPrefix(f, "("):
			// Collect until paren depth returns to 0
			var parts []string
			depth := 0
			for i < len(fields) {
				part := fields[i]
				parts = append(parts, part)
				depth += strings.Count(part, "(") - strings.Count(part, ")")
				i++
				if depth <= 0 {
					break
				}
			}
			tokens = append(tokens, textbuf.Join(parts, " "))
		default: // plain token — no grouping
			tokens = append(tokens, f)
			i++
		}
	}
	return tokens
}
