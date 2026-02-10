package config

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/message"
)

// parseAnnounceAFIRoutes parses routes from an AFI container (ipv4 or ipv6).
// Handles unicast, multicast, nlri-mpls, and mpls-vpn SAFIs.
func parseAnnounceAFIRoutes(afiTree *Tree) ([]StaticRouteConfig, error) {
	var routes []StaticRouteConfig
	safis := []string{"unicast", "multicast", "nlri-mpls", "mpls-vpn"}
	for _, safi := range safis {
		for _, entry := range afiTree.GetListOrdered(safi) {
			sr, err := parseRouteConfig(entry.Key, entry.Value)
			if err != nil {
				return nil, err
			}
			routes = append(routes, sr)
		}
	}
	return routes, nil
}

// extractRoutesFromTree extracts all routes from a neighbor or template tree.
// Handles both static { route ... } and announce { ipv4/ipv6 { unicast/multicast ... } } blocks.
// Uses GetListOrdered to preserve config order.
// Returns UpdateBlockRoutes containing all route types (static, flowspec, vpls, mvpn, mup).
func extractRoutesFromTree(tree *Tree) (*UpdateBlockRoutes, error) {
	result := &UpdateBlockRoutes{}

	// Static routes - use ordered iteration to preserve config order
	if static := tree.GetContainer("static"); static != nil {
		for _, entry := range static.GetListOrdered("route") {
			sr, err := parseRouteConfig(entry.Key, entry.Value)
			if err != nil {
				return nil, err
			}
			result.StaticRoutes = append(result.StaticRoutes, sr)
		}
	}

	// Announce routes - parse from announce { ipv4 { unicast ... } } structure
	if announce := tree.GetContainer("announce"); announce != nil {
		// Parse routes from IPv4 and IPv6 containers using shared helper
		for _, afiName := range []string{"ipv4", "ipv6"} {
			if afiTree := announce.GetContainer(afiName); afiTree != nil {
				routes, err := parseAnnounceAFIRoutes(afiTree)
				if err != nil {
					return nil, err
				}
				result.StaticRoutes = append(result.StaticRoutes, routes...)
			}
		}
	}

	// Native update blocks - parse from update { attribute { ... } nlri { ... } } structure
	for _, entry := range tree.GetListOrdered("update") {
		updateRoutes, err := extractRoutesFromUpdateBlock(entry.Value)
		if err != nil {
			return nil, fmt.Errorf("update block: %w", err)
		}
		// Aggregate all route types
		result.StaticRoutes = append(result.StaticRoutes, updateRoutes.StaticRoutes...)
		result.FlowSpecRoutes = append(result.FlowSpecRoutes, updateRoutes.FlowSpecRoutes...)
		result.VPLSRoutes = append(result.VPLSRoutes, updateRoutes.VPLSRoutes...)
		result.MVPNRoutes = append(result.MVPNRoutes, updateRoutes.MVPNRoutes...)
		result.MUPRoutes = append(result.MUPRoutes, updateRoutes.MUPRoutes...)
	}

	return result, nil
}

// UpdateBlockRoutes holds all route types extracted from an update { } block.
type UpdateBlockRoutes struct {
	StaticRoutes   []StaticRouteConfig
	FlowSpecRoutes []FlowSpecRouteConfig
	VPLSRoutes     []VPLSRouteConfig
	MVPNRoutes     []MVPNRouteConfig
	MUPRoutes      []MUPRouteConfig
}

// extractRoutesFromUpdateBlock parses a single update { attribute { } nlri { } } block.
// Returns all route types (static, flowspec, vpls, mvpn, mup) for each NLRI in the block.
func extractRoutesFromUpdateBlock(update *Tree) (*UpdateBlockRoutes, error) {
	result := &UpdateBlockRoutes{}

	// Parse attributes from attribute { } container
	attr := update.GetContainer("attribute")
	if attr == nil {
		attr = NewTree() // Empty attributes if not specified
	}

	// Parse watchdog container from update block level
	// Routes with watchdog { name ...; withdraw true; } are held until "bgp watchdog announce <name>"
	var watchdog string
	var watchdogWithdraw bool
	if wdContainer := update.GetContainer("watchdog"); wdContainer != nil {
		watchdog, _ = wdContainer.Get("name")
		_, watchdogWithdraw = wdContainer.Get("withdraw")
	}

	// Parse nlri { } container - freeform content like "ipv4/unicast 1.0.0.0/24 2.0.0.0/24;"
	nlriContainer := update.GetContainer("nlri")
	if nlriContainer == nil {
		return nil, fmt.Errorf("missing nlri block in update")
	}

	// Parse each family line from the freeform nlri block
	// Freeform stores content in two ways:
	// 1. Without brackets: "ipv4/unicast 1.0.0.0/24" as key -> "true"
	// 2. With brackets: "ipv4/flow" as key -> "packet-length >200&<300 >400&<500" (brackets stripped)
	// We need to combine key+value to get the full line.
	for _, key := range nlriContainer.Values() {
		value, _ := nlriContainer.Get(key)
		var line string
		if value == configTrue || value == "" {
			line = key // Simple case: entire line is the key
		} else {
			line = key + " " + value // Bracketed case: combine key and value
		}
		// Parse the line: first word is family, rest depends on family type
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		family := parts[0]

		// Handle complex NLRI families specially
		switch family {
		case "ipv4/flow", "ipv6/flow", "ipv4/flow-vpn", "ipv6/flow-vpn":
			fr, err := parseFlowSpecNLRILine(line, attr)
			if err != nil {
				return nil, fmt.Errorf("flowspec nlri: %w", err)
			}
			result.FlowSpecRoutes = append(result.FlowSpecRoutes, fr)
			continue

		case "l2vpn/vpls":
			vr, err := parseVPLSNLRILine(line, attr)
			if err != nil {
				return nil, fmt.Errorf("vpls nlri: %w", err)
			}
			result.VPLSRoutes = append(result.VPLSRoutes, vr)
			continue

		case "ipv4/mcast-vpn", "ipv6/mcast-vpn":
			mr, err := parseMVPNNLRILine(line, attr)
			if err != nil {
				return nil, fmt.Errorf("mvpn nlri: %w", err)
			}
			result.MVPNRoutes = append(result.MVPNRoutes, mr)
			continue

		case "ipv4/mup", "ipv6/mup":
			mr, err := parseMUPNLRILine(line, attr)
			if err != nil {
				return nil, fmt.Errorf("mup nlri: %w", err)
			}
			result.MUPRoutes = append(result.MUPRoutes, mr)
			continue
		}

		// Standard families with simple prefixes
		// Filter out action keywords (add/del) - config routes are always announcements
		// Parse inline rd/label for VPN/labeled families (order doesn't matter):
		//   ipv4/mpls-vpn rd 65000:100 label 100 10.0.0.0/24
		//   ipv4/mpls-vpn label 100 rd 65000:100 10.0.0.0/24
		remaining := parts[1:]
		var inlineRD, inlineLabel string

		// Parse rd/label in any order (both optional)
		for len(remaining) >= 2 {
			switch remaining[0] {
			case "rd":
				inlineRD = remaining[1]
				remaining = remaining[2:]
				continue
			case "label":
				inlineLabel = remaining[1]
				remaining = remaining[2:]
				continue
			}
			break
		}

		var prefixes []string
		for _, p := range remaining {
			if p == "add" || p == "del" || p == "eor" {
				continue // Skip action keywords
			}
			prefixes = append(prefixes, p)
		}

		// Validate family
		if _, ok := message.FamilyConfigNames[family]; !ok {
			return nil, fmt.Errorf("invalid family: %s", family)
		}

		if len(prefixes) == 0 {
			continue // No prefixes for this family
		}
		for _, prefix := range prefixes {
			sr := StaticRouteConfig{}

			// Parse prefix
			p, err := netip.ParsePrefix(prefix)
			if err != nil {
				// Try as bare IP, convert to /32 or /128
				ip, err2 := netip.ParseAddr(prefix)
				if err2 != nil {
					return nil, fmt.Errorf("invalid prefix %s: %w", prefix, err)
				}
				bits := 32
				if ip.Is6() {
					bits = 128
				}
				p = netip.PrefixFrom(ip, bits)
			}
			sr.Prefix = p

			// Apply inline rd/label (parsed from NLRI line)
			if inlineRD != "" {
				sr.RD = inlineRD
			}
			if inlineLabel != "" {
				sr.Label = inlineLabel
			}

			// Apply attributes using shared helper
			if err := applyAttributesFromTree(attr, &sr); err != nil {
				return nil, err
			}

			// Apply watchdog from update block level
			if watchdog != "" {
				sr.Watchdog = watchdog
				sr.WatchdogWithdraw = watchdogWithdraw
			}

			result.StaticRoutes = append(result.StaticRoutes, sr)
		}
	}

	return result, nil
}

// parseFlowSpecNLRILine parses a FlowSpec NLRI line like:
// "ipv4/flow source-ipv4 10.0.0.1/32 destination-port =80 protocol =tcp".
// RFC 8955 Section 4 defines the FlowSpec NLRI format.
func parseFlowSpecNLRILine(line string, attr *Tree) (FlowSpecRouteConfig, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return FlowSpecRouteConfig{}, fmt.Errorf("flowspec nlri requires match criteria")
	}

	family := parts[0]
	fr := FlowSpecRouteConfig{
		IsIPv6: strings.HasPrefix(family, "ipv6/"),
		NLRI:   make(map[string][]string),
	}

	// Parse inline rd for VPN variant: ipv4/flow-vpn rd 65000:100 destination ...
	// RD is part of NLRI (RFC 8955), not a path attribute
	criteria := parts[1:]
	if strings.HasSuffix(family, "-vpn") {
		if len(criteria) >= 2 && criteria[0] == "rd" {
			fr.RD = criteria[1]
			criteria = criteria[2:] // consume rd <value>
		}
	}

	// Get next-hop from attributes
	if v, ok := attr.Get("next-hop"); ok {
		fr.NextHop = v
	}

	// Get community from attributes
	if v, ok := attr.Get("community"); ok {
		fr.Community = v
	}

	// Get extended-community from attributes (actions per RFC 8955 Section 7)
	if v, ok := attr.Get("extended-community"); ok {
		fr.ExtendedCommunity = v
	}

	// Get raw attribute (e.g., for IPv6 Extended Community attr 25)
	if v, ok := attr.Get("attribute"); ok {
		fr.Attribute = v
	}

	// Parse NLRI match criteria from remaining parts
	// Format: <criterion> <value> [<criterion> <value>]...
	// Values are stored as slices to support multi-value criteria like "protocol [ =tcp =udp ]"
	for i := 0; i < len(criteria); i++ {
		criterion := normalizeFlowSpecCriterion(criteria[i])
		// Handle bracketed lists like [ >200&<300 >400&<500 ]
		if i+1 < len(criteria) && criteria[i+1] == "[" {
			// Find closing bracket and collect all values
			j := i + 2
			for ; j < len(criteria) && criteria[j] != "]"; j++ {
				fr.NLRI[criterion] = append(fr.NLRI[criterion], criteria[j])
			}
			i = j
			continue
		}
		// Regular key-value pair (single value)
		if i+1 < len(criteria) {
			fr.NLRI[criterion] = append(fr.NLRI[criterion], criteria[i+1])
			i++
		}
	}

	return fr, nil
}

// normalizeFlowSpecCriterion normalizes FlowSpec criterion names to canonical form.
// Maps "source-ipv4", "source-ipv6" -> "source"; "destination-ipv4", "destination-ipv6" -> "destination".
// This ensures the NLRI map uses keys that buildFlowSpecNLRI expects.
func normalizeFlowSpecCriterion(criterion string) string {
	switch criterion {
	case "source-ipv4", "source-ipv6":
		return "source"
	case "destination-ipv4", "destination-ipv6":
		return "destination"
	default:
		return criterion
	}
}

// parseVPLSNLRILine parses a VPLS NLRI line like:
// "l2vpn/vpls rd 192.168.201.1:123 ve-id 5 ve-block-offset 1 ve-block-size 8 label-base 10702".
func parseVPLSNLRILine(line string, attr *Tree) (VPLSRouteConfig, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return VPLSRouteConfig{}, fmt.Errorf("vpls nlri requires fields")
	}

	vr := VPLSRouteConfig{}

	// Parse key-value pairs
	for i := 1; i < len(parts); i += 2 {
		if i+1 >= len(parts) {
			break
		}
		key, val := parts[i], parts[i+1]
		switch key {
		case "rd":
			vr.RD = val
		case "ve-id", "endpoint":
			v, _ := strconv.ParseUint(val, 10, 16)
			vr.Endpoint = uint16(v)
		case "ve-block-offset", "offset":
			v, _ := strconv.ParseUint(val, 10, 16)
			vr.Offset = uint16(v)
		case "ve-block-size", "size":
			v, _ := strconv.ParseUint(val, 10, 16)
			vr.Size = uint16(v)
		case "label-base", "base":
			v, _ := strconv.ParseUint(val, 10, 32)
			vr.Base = uint32(v)
		}
	}

	// Apply attributes
	if v, ok := attr.Get("next-hop"); ok {
		vr.NextHop = v
	}
	if v, ok := attr.Get("origin"); ok {
		vr.Origin = v
	}
	if v, ok := attr.Get("as-path"); ok {
		vr.ASPath = v
	}
	if v, ok := attr.Get("local-preference"); ok {
		n, _ := strconv.ParseUint(v, 10, 32)
		vr.LocalPreference = uint32(n)
	}
	if v, ok := attr.Get("med"); ok {
		n, _ := strconv.ParseUint(v, 10, 32)
		vr.MED = uint32(n)
	}
	if v, ok := attr.Get("community"); ok {
		vr.Community = v
	}
	if v, ok := attr.Get("extended-community"); ok {
		vr.ExtendedCommunity = v
	}
	if v, ok := attr.Get("originator-id"); ok {
		vr.OriginatorID = v
	}
	if v, ok := attr.Get("cluster-list"); ok {
		vr.ClusterList = v
	}

	return vr, nil
}

// parseMVPNNLRILine parses an MVPN NLRI line like:
// "ipv4/mcast-vpn shared-join rp 10.99.199.1 group 239.251.255.228 rd 65000:99999 source-as 65000".
func parseMVPNNLRILine(line string, attr *Tree) (MVPNRouteConfig, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return MVPNRouteConfig{}, fmt.Errorf("mvpn nlri requires route type and fields")
	}

	family := parts[0]
	mr := MVPNRouteConfig{
		IsIPv6: strings.HasPrefix(family, "ipv6/"),
	}

	// Route type is second field
	if len(parts) > 1 {
		mr.RouteType = parts[1]
	}

	// Parse key-value pairs
	for i := 2; i < len(parts); i += 2 {
		if i+1 >= len(parts) {
			break
		}
		key, val := parts[i], parts[i+1]
		switch key {
		case "rp":
			mr.Source = val
		case fieldSource:
			mr.Source = val
		case "group":
			mr.Group = val
		case "rd":
			mr.RD = val
		case "source-as":
			n, _ := strconv.ParseUint(val, 10, 32)
			mr.SourceAS = uint32(n)
		}
	}

	// Apply attributes
	if v, ok := attr.Get("next-hop"); ok {
		mr.NextHop = v
	}
	if v, ok := attr.Get("origin"); ok {
		mr.Origin = v
	}
	if v, ok := attr.Get("local-preference"); ok {
		n, _ := strconv.ParseUint(v, 10, 32)
		mr.LocalPreference = uint32(n)
	}
	if v, ok := attr.Get("med"); ok {
		n, _ := strconv.ParseUint(v, 10, 32)
		mr.MED = uint32(n)
	}
	if v, ok := attr.Get("extended-community"); ok {
		mr.ExtendedCommunity = v
	}
	if v, ok := attr.Get("originator-id"); ok {
		mr.OriginatorID = v
	}
	if v, ok := attr.Get("cluster-list"); ok {
		mr.ClusterList = v
	}

	return mr, nil
}

// parseMUPNLRILine parses a MUP NLRI line like:
// "ipv4/mup mup-isd 10.0.1.0/24 rd 100:100".
func parseMUPNLRILine(line string, attr *Tree) (MUPRouteConfig, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return MUPRouteConfig{}, fmt.Errorf("mup nlri requires route type and fields")
	}

	family := parts[0]
	mr := MUPRouteConfig{
		IsIPv6: strings.HasPrefix(family, "ipv6/"),
	}

	// Route type is second field (mup-isd, mup-dsd, mup-t1st, mup-t2st)
	if len(parts) > 1 {
		mr.RouteType = parts[1]
	}

	// Third field is typically the prefix/address
	if len(parts) > 2 {
		switch mr.RouteType {
		case routeTypeMUPISD:
			mr.Prefix = parts[2]
		case routeTypeMUPDSD:
			mr.Address = parts[2]
		case routeTypeMUPT1ST:
			mr.Prefix = parts[2]
		case routeTypeMUPT2ST:
			mr.Address = parts[2]
		}
	}

	// Parse remaining key-value pairs
	for i := 3; i < len(parts); i += 2 {
		if i+1 >= len(parts) {
			break
		}
		key, val := parts[i], parts[i+1]
		switch key {
		case "rd":
			mr.RD = val
		case "teid":
			mr.TEID = val
		case "qfi":
			n, _ := strconv.ParseUint(val, 10, 8)
			mr.QFI = uint8(n)
		case "endpoint":
			mr.Endpoint = val
		case fieldSource:
			mr.Source = val
		}
	}

	// Apply attributes
	if v, ok := attr.Get("next-hop"); ok {
		mr.NextHop = v
	}
	if v, ok := attr.Get("extended-community"); ok {
		mr.ExtendedCommunity = v
	}
	if v, ok := attr.GetFlex("bgp-prefix-sid-srv6"); ok {
		mr.PrefixSID = v
	}

	return mr, nil
}

// applyAttributesFromTree applies path attributes from a Tree to a StaticRouteConfig.
// Used by both parseRouteConfig (announce/static syntax) and extractRoutesFromUpdateBlock (update syntax).
func applyAttributesFromTree(tree *Tree, sr *StaticRouteConfig) error {
	if v, ok := tree.Get("next-hop"); ok {
		if v == configSelf {
			sr.NextHopSelf = true
		} else {
			sr.NextHop = v
		}
	}
	if v, ok := tree.Get("local-preference"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid local-preference %q: %w", v, err)
		}
		sr.LocalPreference = uint32(n)
	}
	if v, ok := tree.Get("med"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid med %q: %w", v, err)
		}
		sr.MED = uint32(n)
	}
	if v, ok := tree.Get("community"); ok {
		sr.Community = v
	}
	if v, ok := tree.Get("extended-community"); ok {
		sr.ExtendedCommunity = v
	}
	if v, ok := tree.Get("large-community"); ok {
		sr.LargeCommunity = v
	}
	if v, ok := tree.Get("as-path"); ok {
		sr.ASPath = v
	}
	if v, ok := tree.Get("origin"); ok {
		sr.Origin = v
	}
	if v, ok := tree.Get("path-information"); ok {
		sr.PathInformation = v
	}
	if v, ok := tree.Get("label"); ok {
		sr.Label = v
	}
	// RFC 8277: Multi-label support via `labels [100 200 300]` syntax
	if v, ok := tree.Get("labels"); ok {
		sr.Labels = parseLabelsArray(v)
	}
	// Note: rd is parsed inline with NLRI, not from attributes
	if v, ok := tree.Get("aggregator"); ok {
		sr.Aggregator = v
	}
	// atomic-aggregate can be a standalone flag or have a value
	if _, ok := tree.Get("atomic-aggregate"); ok {
		sr.AtomicAggregate = true
	}
	if v, ok := tree.Get("attribute"); ok {
		sr.Attribute = v
	}
	if v, ok := tree.Get("originator-id"); ok {
		sr.OriginatorID = v
	}
	if v, ok := tree.Get("cluster-list"); ok {
		sr.ClusterList = v
	}
	// Flex syntax stores in multiValues, so use GetFlex
	if v, ok := tree.GetFlex("bgp-prefix-sid"); ok {
		sr.PrefixSID = v
	}
	// SRv6 Prefix-SID overrides label-index Prefix-SID if both are specified
	if v, ok := tree.GetFlex("bgp-prefix-sid-srv6"); ok {
		sr.PrefixSID = v
	}
	if v, ok := tree.Get("split"); ok {
		sr.Split = v
	}
	// Watchdog support
	if v, ok := tree.Get("watchdog"); ok {
		sr.Watchdog = v
	}
	if _, ok := tree.Get("withdraw"); ok {
		sr.WatchdogWithdraw = true
	}
	return nil
}

// parseRouteConfig extracts a StaticRouteConfig from a parsed route tree.
// The prefix key may have a #N suffix for duplicate routes (ADD-PATH support).
func parseRouteConfig(prefix string, route *Tree) (StaticRouteConfig, error) {
	sr := StaticRouteConfig{}

	// Strip #N suffix added by AddListEntry for duplicate keys
	// e.g., "10.0.0.10#1" → "10.0.0.10"
	actualPrefix := prefix
	if idx := strings.LastIndex(prefix, "#"); idx > 0 {
		// Verify suffix is numeric (not part of IPv6 address)
		suffix := prefix[idx+1:]
		if _, err := strconv.Atoi(suffix); err == nil {
			actualPrefix = prefix[:idx]
		}
	}

	// Try as prefix first, then as bare IP (host route)
	p, err := netip.ParsePrefix(actualPrefix)
	if err != nil {
		// Try as bare IP, convert to /32 or /128
		ip, err2 := netip.ParseAddr(actualPrefix)
		if err2 != nil {
			return sr, fmt.Errorf("invalid prefix %s: %w", actualPrefix, err)
		}
		bits := 32
		if ip.Is6() {
			bits = 128
		}
		p = netip.PrefixFrom(ip, bits)
	}
	sr.Prefix = p

	if err := applyAttributesFromTree(route, &sr); err != nil {
		return sr, err
	}

	return sr, nil
}

// parseLabelsArray parses labels from schema.
// RFC 8277: Multi-label support.
// Input can be:
//   - "[100 200 300]" (from parseKeyValuesFromTokens, inline parsing)
//   - "100 200 300" (from ValueOrArray schema node, space-separated)
//   - "100" (single label)
func parseLabelsArray(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Strip brackets if present (from inline parsing)
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		s = strings.TrimPrefix(s, "[")
		s = strings.TrimSuffix(s, "]")
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
	}

	// Split by whitespace (handles both "100" and "100 200 300")
	return strings.Fields(s)
}

// extractMVPNRoutes extracts MVPN routes from announce { ipv4/ipv6 { mcast-vpn ... } }.
func extractMVPNRoutes(tree *Tree) []MVPNRouteConfig {
	var routes []MVPNRouteConfig

	announce := tree.GetContainer("announce")
	if announce == nil {
		return routes
	}

	// IPv4 MVPN - use GetListOrdered to preserve config order
	if ipv4 := announce.GetContainer("ipv4"); ipv4 != nil {
		for _, entry := range ipv4.GetListOrdered("mcast-vpn") {
			r := parseMVPNRoute(entry.Key, entry.Value, false)
			routes = append(routes, r)
		}
	}

	// IPv6 MVPN - use GetListOrdered to preserve config order
	if ipv6 := announce.GetContainer("ipv6"); ipv6 != nil {
		for _, entry := range ipv6.GetListOrdered("mcast-vpn") {
			r := parseMVPNRoute(entry.Key, entry.Value, true)
			routes = append(routes, r)
		}
	}

	return routes
}

func parseMVPNRoute(routeType string, route *Tree, isIPv6 bool) MVPNRouteConfig {
	r := MVPNRouteConfig{
		RouteType: routeType,
		IsIPv6:    isIPv6,
	}

	if v, ok := route.Get("rd"); ok {
		r.RD = v
	}
	if v, ok := route.Get("source-as"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.SourceAS = uint32(n)
		}
	}
	// Source can be "source" or "rp" depending on route type
	if v, ok := route.Get("source"); ok {
		r.Source = v
	} else if v, ok := route.Get("rp"); ok {
		r.Source = v
	}
	if v, ok := route.Get("group"); ok {
		r.Group = v
	}
	if v, ok := route.Get("next-hop"); ok {
		r.NextHop = v
	}
	if v, ok := route.Get("origin"); ok {
		r.Origin = v
	}
	if v, ok := route.Get("local-preference"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.LocalPreference = uint32(n)
		}
	}
	if v, ok := route.Get("med"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.MED = uint32(n)
		}
	}
	if v, ok := route.Get("extended-community"); ok {
		r.ExtendedCommunity = v
	}
	if v, ok := route.Get("originator-id"); ok {
		r.OriginatorID = v
	}
	if v, ok := route.Get("cluster-list"); ok {
		r.ClusterList = v
	}

	return r
}

// parseInlineKeyValues parses an inline "key value key value ..." string into a map.
// Handles arrays like "[ a b c ]" and parenthesized content like "( ... )".
func parseInlineKeyValues(inline string) map[string]string {
	tokens := tokenizeInline(inline)
	return parseKeyValuesFromTokens(tokens, 0)
}

// parseKeyValuesFromTokens parses "key value key value ..." from a token slice.
// Handles arrays like "[ a b c ]" and parenthesized content like "( ... )".
// Start specifies the index to begin parsing from.
func parseKeyValuesFromTokens(tokens []string, start int) map[string]string {
	result := make(map[string]string)
	i := start
	for i < len(tokens) {
		key := tokens[i]
		i++
		if i >= len(tokens) {
			break
		}

		// Collect value (might be array or parenthesized)
		switch tokens[i] {
		case "[":
			// Array: collect until ]
			var arr []string
			i++ // skip [
			for i < len(tokens) && tokens[i] != "]" {
				arr = append(arr, tokens[i])
				i++
			}
			if i < len(tokens) {
				i++ // skip ]
			}
			result[key] = "[" + strings.Join(arr, " ") + "]"
		case "(":
			// Parenthesized: collect until )
			depth := 1
			var paren []string
			i++ // skip (
		parenLoop:
			for i < len(tokens) && depth > 0 {
				switch tokens[i] {
				case "(":
					depth++
				case ")":
					depth--
					if depth == 0 {
						break parenLoop
					}
				}
				paren = append(paren, tokens[i])
				i++
			}
			if i < len(tokens) {
				i++ // skip )
			}
			result[key] = "(" + strings.Join(paren, " ") + ")"
		default:
			// Simple value
			result[key] = tokens[i]
			i++
		}
	}

	return result
}

// tokenizeInline splits an inline string into tokens, preserving brackets and parens.
func tokenizeInline(s string) []string {
	var tokens []string
	var current strings.Builder

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case ' ', '\t', '\n', '\r':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		case '[', ']', '(', ')':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(c))
		case '\\':
			// Skip backslash continuations - they're artifacts from multiline parsing
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// parseVPLSFromInline creates a VPLSRouteConfig from an inline string.
func parseVPLSFromInline(inline string) VPLSRouteConfig {
	kv := parseInlineKeyValues(inline)
	r := VPLSRouteConfig{}

	if v, ok := kv["rd"]; ok {
		r.RD = v
	}
	if v, ok := kv["endpoint"]; ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Endpoint = uint16(n)
		}
	}
	if v, ok := kv["base"]; ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.Base = uint32(n)
		}
	}
	if v, ok := kv["offset"]; ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Offset = uint16(n)
		}
	}
	if v, ok := kv["size"]; ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Size = uint16(n)
		}
	}
	if v, ok := kv["next-hop"]; ok {
		r.NextHop = v
	}
	if v, ok := kv["origin"]; ok {
		r.Origin = v
	}
	if v, ok := kv["local-preference"]; ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.LocalPreference = uint32(n)
		}
	}
	if v, ok := kv["med"]; ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.MED = uint32(n)
		}
	}
	if v, ok := kv["as-path"]; ok {
		r.ASPath = v
	}
	if v, ok := kv["community"]; ok {
		r.Community = v
	}
	if v, ok := kv["extended-community"]; ok {
		r.ExtendedCommunity = v
	}
	if v, ok := kv["originator-id"]; ok {
		r.OriginatorID = v
	}
	if v, ok := kv["cluster-list"]; ok {
		r.ClusterList = v
	}

	return r
}

// extractVPLSRoutes extracts VPLS routes from l2vpn { vpls ... } and announce { l2vpn { vpls ... } }.
// Order: announce inline first, then l2vpn named, then l2vpn inline (to match ExaBGP behavior).
func extractVPLSRoutes(tree *Tree) []VPLSRouteConfig {
	var routes []VPLSRouteConfig

	// From announce { l2vpn { vpls ... } } - inline routes first
	if announce := tree.GetContainer("announce"); announce != nil {
		if l2vpn := announce.GetContainer("l2vpn"); l2vpn != nil {
			// Inline first
			for _, inline := range l2vpn.GetMultiValues("vpls") {
				if inline != "" && inline != configTrue {
					r := parseVPLSFromInline(inline)
					routes = append(routes, r)
				}
			}
			// Named blocks from announce - use GetListOrdered to preserve config order
			for _, entry := range l2vpn.GetListOrdered("vpls") {
				r := parseVPLSRoute(entry.Key, entry.Value)
				routes = append(routes, r)
			}
		}
	}

	// From l2vpn block - named blocks then inline
	if l2vpn := tree.GetContainer("l2vpn"); l2vpn != nil {
		// Named blocks: vpls site5 { ... } - use GetListOrdered to preserve config order
		for _, entry := range l2vpn.GetListOrdered("vpls") {
			r := parseVPLSRoute(entry.Key, entry.Value)
			routes = append(routes, r)
		}
		// Inline: vpls rd X endpoint Y ...;
		for _, inline := range l2vpn.GetMultiValues("vpls") {
			if inline != "" && inline != configTrue {
				r := parseVPLSFromInline(inline)
				routes = append(routes, r)
			}
		}
	}

	return routes
}

func parseVPLSRoute(name string, route *Tree) VPLSRouteConfig {
	r := VPLSRouteConfig{Name: name}

	if v, ok := route.Get("rd"); ok {
		r.RD = v
	}
	if v, ok := route.Get("endpoint"); ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Endpoint = uint16(n)
		}
	}
	if v, ok := route.Get("base"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.Base = uint32(n)
		}
	}
	if v, ok := route.Get("offset"); ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Offset = uint16(n)
		}
	}
	if v, ok := route.Get("size"); ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Size = uint16(n)
		}
	}
	if v, ok := route.Get("next-hop"); ok {
		r.NextHop = v
	}
	if v, ok := route.Get("origin"); ok {
		r.Origin = v
	}
	if v, ok := route.Get("local-preference"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.LocalPreference = uint32(n)
		}
	}
	if v, ok := route.Get("med"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.MED = uint32(n)
		}
	}
	if v, ok := route.Get("as-path"); ok {
		r.ASPath = v
	}
	if v, ok := route.Get("community"); ok {
		r.Community = v
	}
	if v, ok := route.Get("extended-community"); ok {
		r.ExtendedCommunity = v
	}
	if v, ok := route.Get("originator-id"); ok {
		r.OriginatorID = v
	}
	if v, ok := route.Get("cluster-list"); ok {
		r.ClusterList = v
	}

	return r
}

// extractFlowSpecRoutes extracts FlowSpec routes from flow { route ... }.
func extractFlowSpecRoutes(tree *Tree) []FlowSpecRouteConfig {
	flow := tree.GetContainer("flow")
	if flow == nil {
		return nil
	}

	// Use ordered iteration to preserve config order.
	entries := flow.GetListOrdered("route")
	routes := make([]FlowSpecRouteConfig, 0, len(entries))
	for _, entry := range entries {
		r := parseFlowSpecRoute(entry.Key, entry.Value)
		routes = append(routes, r)
	}

	return routes
}

func parseFlowSpecRoute(name string, route *Tree) FlowSpecRouteConfig {
	r := FlowSpecRouteConfig{
		Name: name,
		NLRI: make(map[string][]string),
	}

	if v, ok := route.Get("rd"); ok {
		r.RD = v
	}
	if v, ok := route.Get("next-hop"); ok {
		r.NextHop = v
	}

	// Parse match block into NLRI criteria (RFC 8955 Section 4)
	// Freeform stores:
	// - "keyword value" -> "true" for simple values like "source 10.0.0.1/32"
	// - "keyword" -> "value" for arrays like "fragment [ last-fragment ]"
	if match := route.GetContainer("match"); match != nil {
		for _, key := range match.Values() {
			val, _ := match.Get(key)
			if val == configTrue || val == "" {
				// Legacy format: key might be "keyword value"
				parts := strings.SplitN(key, " ", 2)
				if len(parts) == 2 {
					r.NLRI[parts[0]] = []string{parts[1]}
				}
				// Skip empty keys
			} else {
				// Array format: key is keyword, val has the values
				r.NLRI[key] = strings.Fields(strings.Trim(val, "[]"))
			}
		}
	}

	// Parse then block into ExtendedCommunity (RFC 8955 Section 7)
	// Actions are encoded as Traffic Filtering Action Extended Communities
	var extComms []string
	if then := route.GetContainer("then"); then != nil {
		for _, key := range then.Values() {
			val, _ := then.Get(key)
			action, value := key, val

			// Handle legacy "keyword value" format stored as key
			if val == configTrue || val == "" {
				parts := strings.SplitN(key, " ", 2)
				if len(parts) == 2 {
					action, value = parts[0], parts[1]
				} else {
					action, value = key, ""
				}
			}

			// Convert actions to extended community format
			switch action {
			case "discard":
				extComms = append(extComms, "discard")
			case "rate-limit":
				extComms = append(extComms, "rate-limit:"+value)
			case "redirect":
				extComms = append(extComms, "redirect:"+value)
			case "redirect-to-nexthop-draft":
				extComms = append(extComms, "redirect-to-nexthop-draft")
			case "copy-to-nexthop":
				extComms = append(extComms, "copy-to-nexthop")
			case "mark":
				extComms = append(extComms, "mark "+value)
			case "action":
				extComms = append(extComms, "action "+value)
			case "community":
				r.Community = strings.Trim(value, "[]")
			case "extended-community":
				extComms = append(extComms, strings.Trim(value, "[]"))
			}
		}
	}

	// Combine explicit extended-community with action-based ones
	if len(extComms) > 0 {
		if r.ExtendedCommunity != "" {
			r.ExtendedCommunity += " " + strings.Join(extComms, " ")
		} else {
			r.ExtendedCommunity = strings.Join(extComms, " ")
		}
	}

	// Determine if IPv6 based on NLRI criteria
	for key, vals := range r.NLRI {
		if key == "source" || key == "destination" {
			for _, val := range vals {
				if strings.Contains(val, ":") {
					r.IsIPv6 = true
					break
				}
			}
		}
	}

	return r
}

// parseMUPFromInline creates a MUPRouteConfig from an inline string.
// Format: "mup-isd PREFIX rd RD next-hop NH ..." or "mup-dsd ADDR rd RD ...".
func parseMUPFromInline(inline string, isIPv6 bool) MUPRouteConfig {
	tokens := tokenizeInline(inline)
	if len(tokens) == 0 {
		return MUPRouteConfig{}
	}

	r := MUPRouteConfig{
		IsIPv6: isIPv6,
	}

	// First token is route type
	r.RouteType = tokens[0]

	// Second token is prefix or address
	if len(tokens) > 1 {
		if r.RouteType == routeTypeMUPISD || r.RouteType == routeTypeMUPT1ST {
			r.Prefix = tokens[1]
		} else {
			r.Address = tokens[1]
		}
	}

	// Parse remaining as key-value pairs starting from index 2
	kv := parseKeyValuesFromTokens(tokens, 2)

	if v, ok := kv["rd"]; ok {
		r.RD = v
	}
	if v, ok := kv["teid"]; ok {
		r.TEID = v
	}
	if v, ok := kv["qfi"]; ok {
		if n, err := strconv.ParseUint(v, 10, 8); err == nil {
			r.QFI = uint8(n)
		}
	}
	if v, ok := kv["endpoint"]; ok {
		r.Endpoint = v
	}
	if v, ok := kv["source"]; ok {
		r.Source = v
	}
	if v, ok := kv["next-hop"]; ok {
		r.NextHop = v
	}
	if v, ok := kv["extended-community"]; ok {
		r.ExtendedCommunity = v
	}
	if v, ok := kv["bgp-prefix-sid-srv6"]; ok {
		r.PrefixSID = v
	}

	return r
}

// extractMUPRoutes extracts MUP routes from announce { ipv4/ipv6 { mup ... } }.
func extractMUPRoutes(tree *Tree) []MUPRouteConfig {
	var routes []MUPRouteConfig

	announce := tree.GetContainer("announce")
	if announce == nil {
		return routes
	}

	// IPv4 MUP - use GetListOrdered to preserve config order
	if ipv4 := announce.GetContainer("ipv4"); ipv4 != nil {
		// Named blocks (if any)
		for _, entry := range ipv4.GetListOrdered("mup") {
			r := parseMUPRoute(entry.Key, entry.Value, false)
			routes = append(routes, r)
		}
		// Inline: mup mup-isd PREFIX rd RD ...;
		for _, inline := range ipv4.GetMultiValues("mup") {
			if inline != "" && inline != configTrue {
				r := parseMUPFromInline(inline, false)
				routes = append(routes, r)
			}
		}
	}

	// IPv6 MUP - use GetListOrdered to preserve config order
	if ipv6 := announce.GetContainer("ipv6"); ipv6 != nil {
		// Named blocks (if any)
		for _, entry := range ipv6.GetListOrdered("mup") {
			r := parseMUPRoute(entry.Key, entry.Value, true)
			routes = append(routes, r)
		}
		// Inline
		for _, inline := range ipv6.GetMultiValues("mup") {
			if inline != "" && inline != configTrue {
				r := parseMUPFromInline(inline, true)
				routes = append(routes, r)
			}
		}
	}

	return routes
}

func parseMUPRoute(routeType string, route *Tree, isIPv6 bool) MUPRouteConfig {
	r := MUPRouteConfig{
		RouteType: routeType,
		IsIPv6:    isIPv6,
	}

	// Route type determines which field to use for prefix/address
	if strings.HasSuffix(routeType, "-isd") || strings.HasSuffix(routeType, "-t1st") {
		// These have prefix
		for _, key := range route.Values() {
			if strings.Contains(key, "/") || strings.Contains(key, ":") {
				r.Prefix = key
				break
			}
		}
	} else {
		// mup-dsd, mup-t2st have address
		for _, key := range route.Values() {
			if !strings.Contains(key, "/") && (strings.Contains(key, ".") || strings.Contains(key, ":")) {
				r.Address = key
				break
			}
		}
	}

	if v, ok := route.Get("rd"); ok {
		r.RD = v
	}
	if v, ok := route.Get("teid"); ok {
		r.TEID = v
	}
	if v, ok := route.Get("qfi"); ok {
		if n, err := strconv.ParseUint(v, 10, 8); err == nil {
			r.QFI = uint8(n)
		}
	}
	if v, ok := route.Get("endpoint"); ok {
		r.Endpoint = v
	}
	if v, ok := route.Get("source"); ok {
		r.Source = v
	}
	if v, ok := route.Get("next-hop"); ok {
		r.NextHop = v
	}
	if v, ok := route.Get("extended-community"); ok {
		r.ExtendedCommunity = v
	}
	if v, ok := route.Get("bgp-prefix-sid-srv6"); ok {
		r.PrefixSID = v
	}

	return r
}
