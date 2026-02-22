// Design: docs/architecture/core-design.md — ExaBGP route conversion to Ze update blocks
// Related: migrate.go — migration orchestration and neighbor conversion
// Related: migrate_family.go — family syntax conversion helpers
// Related: migrate_serialize.go — tree serialization

package exabgp

import (
	"encoding/hex"
	"net"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config"
)

// flexAttrKeywords lists path attribute keywords that go in the attribute block
// when converting flex container entries (mcast-vpn, mup) to update blocks.
// Everything else is treated as NLRI fields.
var flexAttrKeywords = map[string]bool{
	"next-hop":            true,
	"origin":              true,
	"local-preference":    true,
	"med":                 true,
	"extended-community":  true,
	"large-community":     true,
	"community":           true,
	"originator-id":       true,
	"cluster-list":        true,
	"as-path":             true,
	"bgp-prefix-sid-srv6": true,
}

// convertAnnounceToUpdate converts ExaBGP announce blocks to Ze update blocks.
// ExaBGP: announce { ipv4 { unicast PREFIX next-hop NH; } }.
// Ze: update { attribute { next-hop NH; } nlri { ipv4/unicast PREFIX; } }.
func convertAnnounceToUpdate(announce, dst *config.Tree) {
	// Process each AFI (ipv4, ipv6, l2vpn)
	for _, afi := range []string{"ipv4", "ipv6", "l2vpn"} {
		afiBlock := announce.GetContainer(afi)
		if afiBlock == nil {
			continue
		}

		// Process each SAFI (unicast, multicast, nlri-mpls, mpls-vpn, flow, vpls, evpn)
		safis := []string{"unicast", "multicast", "nlri-mpls", "mpls-vpn", "flow", "vpls", "evpn"}
		for _, safi := range safis {
			// With ze:syntax "inline-list", routes are stored as list entries, not container values.
			// Each list entry has: Key=prefix, Value=Tree containing attributes.
			routeList := afiBlock.GetListOrdered(safi)
			if len(routeList) == 0 {
				continue
			}

			// Convert family name
			family := afi + "/" + safi

			// Process each route entry
			for _, routeEntry := range routeList {
				prefix := routeEntry.Key
				attrTree := routeEntry.Value

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
				attrFields := []string{"next-hop", "local-preference", "med", "as-path", "community",
					"extended-community", "large-community", "aggregator", "originator-id", "cluster-list",
					"rd", "label", "labels", "path-information"}
				for _, field := range attrFields {
					if v, ok := attrTree.Get(field); ok {
						attrBlock.Set(field, v)
					}
				}

				update.SetContainer("attribute", attrBlock)

				// Build nlri list entry
				nlriEntry := config.NewTree()
				nlriEntry.Set("content", prefix)
				update.AddListEntry("nlri", family, nlriEntry)

				// Add update to dst as list entry
				dst.AddListEntry("update", "", update)
			}
		}

		// Handle flex SAFIs (mcast-vpn, mup, vpls) which store values via AppendValue.
		// These use ze:syntax "flex" in the ExaBGP YANG schema, so GetListOrdered returns nothing.
		flexSafis := []string{"mcast-vpn", "mup", "vpls"}
		for _, safi := range flexSafis {
			values := afiBlock.GetMultiValues(safi)
			if len(values) == 0 {
				continue
			}
			convertFlexToUpdate(afi, safi, values, dst)
		}
	}

	// Handle announce { route PREFIX ... } (generic route syntax)
	// This uses inline-list as well, so use GetListOrdered.
	routeList := announce.GetListOrdered("route")
	for _, routeEntry := range routeList {
		convertRouteToUpdate(routeEntry.Key, routeEntry.Value, dst)
	}
}

// convertStaticToUpdate converts ExaBGP static blocks to Ze update blocks.
// ExaBGP: static { route PREFIX next-hop NH; }.
// Ze: update { attribute { next-hop NH; } nlri { ipv4/unicast PREFIX; } }.
func convertStaticToUpdate(static, dst *config.Tree) {
	// With ze:syntax "inline-list", routes are stored as list entries.
	routeList := static.GetListOrdered("route")
	for _, routeEntry := range routeList {
		convertRouteToUpdate(routeEntry.Key, routeEntry.Value, dst)
	}
}

// convertFlowToUpdate converts ExaBGP flow blocks to Ze update blocks.
// ExaBGP: flow { route NAME { rd RD; next-hop NH; match { criteria; } then { actions; } } }
// Ze: update { attribute { extended-community ...; } nlri { ipv4/flow criterion value ...; } }.
func convertFlowToUpdate(flow, dst *config.Tree) {
	for _, entry := range flow.GetListOrdered("route") {
		route := entry.Value

		rd, _ := route.Get("rd")
		nextHop, _ := route.Get("next-hop")

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

		// Parse then block → attributes.
		var extComms []string
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
					extComms = append(extComms, "rate-limit:"+value)
				case "redirect":
					// redirect <IP> = set next-hop + redirect-to-nexthop-draft
					// redirect <AS:VAL> = redirect extended community
					if net.ParseIP(value) != nil {
						nextHop = value
						extComms = append(extComms, "redirect-to-nexthop-draft")
					} else {
						extComms = append(extComms, "redirect:"+value)
					}
				case "redirect-to-nexthop":
					extComms = append(extComms, "redirect-to-nexthop-draft")
				case "copy":
					// copy <IP> = set next-hop + copy-to-nexthop
					if net.ParseIP(value) != nil {
						nextHop = value
					}
					extComms = append(extComms, "copy-to-nexthop")
				case "mark":
					extComms = append(extComms, "mark "+value)
				case "action":
					extComms = append(extComms, "action "+value)
				case "community":
					community = strings.Trim(value, "[] ")
				case "extended-community":
					// Inline extended communities: "[ origin:... origin:... ]"
					inner := strings.Trim(value, "[] ")
					extComms = append(extComms, strings.Fields(inner)...)
				case "redirect-to-nexthop-ietf":
					// ExaBGP name → Ze canonical name (RFC 7674)
					ip := net.ParseIP(value)
					if ip != nil && ip.To4() != nil {
						// IPv4: extended-community
						extComms = append(extComms, "redirect-to-nexthop "+value)
					} else if ip != nil {
						// IPv6: raw attribute (type 25=IPv6 ExtComm, flags 0xC0)
						// Format: sub-type 0x000c + IPv6 (16 bytes) + local-admin 0x0000
						ipHex := hex.EncodeToString(ip.To16())
						rawAttr = "[0x19 0xc0 0x000c" + ipHex + "0000]"
					}
				case "attribute":
					rawAttr = value
				}
			}
		}

		// Determine family.
		family := "ipv4/flow"
		if isIPv6 {
			family = "ipv6/flow"
		}
		if rd != "" {
			if isIPv6 {
				family = "ipv6/flow-vpn"
			} else {
				family = "ipv4/flow-vpn"
			}
		}

		// Build NLRI content: [rd <rd>] <criteria...>
		var nlriContent strings.Builder
		if rd != "" {
			nlriContent.WriteString("rd " + rd)
		}
		for _, c := range nlriCriteria {
			if nlriContent.Len() > 0 {
				nlriContent.WriteString(" ")
			}
			nlriContent.WriteString(c)
		}

		// Build update block.
		update := config.NewTree()

		attrBlock := config.NewTree()
		if nextHop != "" {
			attrBlock.Set("next-hop", nextHop)
		}
		if len(extComms) > 0 {
			attrBlock.Set("extended-community", "["+strings.Join(extComms, " ")+"]")
		}
		if community != "" {
			attrBlock.Set("community", "["+community+"]")
		}
		if rawAttr != "" {
			attrBlock.Set("attribute", rawAttr)
		}
		update.SetContainer("attribute", attrBlock)

		nlriEntry := config.NewTree()
		nlriEntry.Set("content", nlriContent.String())
		update.AddListEntry("nlri", family, nlriEntry)

		dst.AddListEntry("update", "", update)
	}
}

// convertL2VPNToUpdate converts neighbor-level l2vpn blocks to Ze update blocks.
// Handles both inline VPLS routes (flex multi-values) and named VPLS blocks (list entries).
func convertL2VPNToUpdate(l2vpn, dst *config.Tree) {
	// Handle inline VPLS routes (stored via AppendValue by the flex parser).
	vplsValues := l2vpn.GetMultiValues("vpls")
	if len(vplsValues) > 0 {
		convertFlexToUpdate("l2vpn", "vpls", vplsValues, dst)
	}

	// Handle named VPLS routes (stored via AddListEntry by the flex parser).
	for _, entry := range l2vpn.GetListOrdered("vpls") {
		convertNamedVPLSToUpdate(entry.Value, dst)
	}
}

// convertNamedVPLSToUpdate converts a named VPLS block (e.g., vpls site5 { ... })
// to a Ze update block. The block's Tree has parsed fields like endpoint, base, rd, etc.
func convertNamedVPLSToUpdate(vpls, dst *config.Tree) {
	update := config.NewTree()

	attrBlock := config.NewTree()

	// Extract path attributes from the VPLS block.
	// Simple values (single word) are stored as-is.
	simpleFields := []string{"next-hop", "origin", "local-preference", "med", "originator-id"}
	for _, field := range simpleFields {
		if v, ok := vpls.Get(field); ok {
			attrBlock.Set(field, v)
		}
	}
	// Array fields: value-or-array strips brackets, so re-wrap multi-word values.
	arrayFields := []string{"as-path", "community", "extended-community", "large-community", "cluster-list"}
	for _, field := range arrayFields {
		if v, ok := vpls.Get(field); ok {
			if strings.Contains(v, " ") && !strings.HasPrefix(v, "[") {
				v = "[" + v + "]"
			}
			attrBlock.Set(field, v)
		}
	}
	if _, ok := attrBlock.Get("origin"); !ok {
		attrBlock.Set("origin", "igp")
	}
	update.SetContainer("attribute", attrBlock)

	// Build NLRI line from VPLS-specific fields.
	// Format: "l2vpn/vpls rd X endpoint Y base Z offset A size B"
	nlriFields := []string{"rd", "endpoint", "ve-id", "base", "offset", "ve-block-offset", "size", "ve-block-size"}
	var nlriParts []string
	for _, field := range nlriFields {
		if v, ok := vpls.Get(field); ok {
			nlriParts = append(nlriParts, field, v)
		}
	}

	nlriEntry := config.NewTree()
	nlriEntry.Set("content", strings.Join(nlriParts, " "))
	update.AddListEntry("nlri", "l2vpn/vpls", nlriEntry)

	dst.AddListEntry("update", "", update)
}

// convertRouteToUpdate converts a single ExaBGP route to a Ze update block.
// prefix is the NLRI prefix (e.g., "10.0.0.0/24").
// attrTree contains the route attributes from ExaBGP.
// dst is the peer block where the update will be added.
//
// Family detection: rd present → mpls-vpn, label present (no rd) → nlri-mpls, else → unicast.
// RD and label go inline in the NLRI (config loader expects them there, not in attribute block).
func convertRouteToUpdate(prefix string, attrTree, dst *config.Tree) {
	update := config.NewTree()

	attrBlock := config.NewTree()
	attrBlock.Set("origin", "igp")

	// Path attributes that go in the attribute block.
	// Note: rd and label are NOT here — they go inline in the NLRI line.
	attrFields := []string{"next-hop", "local-preference", "med", "as-path", "community",
		"extended-community", "large-community", "aggregator", "originator-id", "cluster-list",
		"path-information", "labels", "split"}
	for _, field := range attrFields {
		if v, ok := attrTree.Get(field); ok {
			attrBlock.Set(field, v)
		}
	}

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

	// Detect family from prefix format and route attributes.
	isIPv6 := strings.Contains(prefix, ":")
	rdVal, hasRD := attrTree.Get("rd")
	labelVal, hasLabel := attrTree.Get("label")

	family := detectRouteFamily(isIPv6, hasRD, hasLabel)

	// Build NLRI value with inline rd/label (config loader parses these from NLRI line).
	nlriValue := prefix
	if hasLabel {
		nlriValue = "label " + labelVal + " " + nlriValue
	}
	if hasRD {
		nlriValue = "rd " + rdVal + " " + nlriValue
	}

	nlriEntry := config.NewTree()
	nlriEntry.Set("content", nlriValue)
	update.AddListEntry("nlri", family, nlriEntry)

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
}

// detectRouteFamily determines the BGP address family from route characteristics.
// rd present → mpls-vpn, label present (no rd) → nlri-mpls, else → unicast.
func detectRouteFamily(isIPv6, hasRD, hasLabel bool) string {
	if hasRD && isIPv6 {
		return "ipv6/mpls-vpn"
	}
	if hasRD {
		return "ipv4/mpls-vpn"
	}
	if hasLabel && isIPv6 {
		return "ipv6/mpls"
	}
	if hasLabel {
		return "ipv4/mpls"
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
	switch criterion {
	case "source":
		if isIPv6 {
			return "source-ipv6 " + value
		}
		return "source-ipv4 " + value
	case "destination":
		if isIPv6 {
			return "destination-ipv6 " + value
		}
		return "destination-ipv4 " + value
	default: // pass through other criteria unchanged
		if value == "" {
			return criterion
		}
		// Unwrap single-element bracket lists: "[ !=0 ]" → "!=0"
		if strings.HasPrefix(value, "[ ") && strings.HasSuffix(value, " ]") {
			inner := strings.Fields(value[2 : len(value)-2])
			if len(inner) == 1 {
				return criterion + " " + inner[0]
			}
		}
		return criterion + " " + value
	}
}

// convertFlexToUpdate converts flex container entries to update blocks.
// Flex SAFIs (mcast-vpn, mup) store entire route lines as strings via AppendValue().
// Each string contains both NLRI fields and path attributes intermixed.
// This function separates them into attribute { } and nlri { } blocks.
func convertFlexToUpdate(afi, safi string, values []string, dst *config.Tree) {
	family := afi + "/" + safi

	for _, value := range values {
		attrs, nlriParts := splitFlexAttrs(value)

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

		// Build nlri block
		nlriEntry := config.NewTree()
		nlriEntry.Set("content", strings.Join(nlriParts, " "))
		update.AddListEntry("nlri", family, nlriEntry)

		dst.AddListEntry("update", "", update)
	}
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
			tokens = append(tokens, strings.Join(parts, " "))
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
			tokens = append(tokens, strings.Join(parts, " "))
		default: // plain token — no grouping
			tokens = append(tokens, f)
			i++
		}
	}
	return tokens
}
