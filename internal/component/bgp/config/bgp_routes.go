// Design: docs/architecture/config/syntax.md — BGP route extraction from config tree
// Detail: bgp_routes_flowspec.go — FlowSpec route parsing and extraction
// Detail: bgp_routes_vpls.go — VPLS route parsing and extraction
// Detail: bgp_routes_mvpn.go — MVPN route parsing and extraction
// Detail: bgp_routes_mup.go — MUP route parsing and extraction
// Detail: bgp_routes_inline.go — shared inline key-value tokenizer

package bgpconfig

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// NLRI operation keywords.
const (
	opAdd = "add"
	opDel = "del"
	opEor = "eor"
)

// MUP family string constants.
const (
	familyIPv4MUP = "ipv4/mup"
	familyIPv6MUP = "ipv6/mup"
)

// parseAnnounceAFIRoutes parses routes from an AFI container (ipv4 or ipv6).
// Handles unicast, multicast, nlri-mpls, and mpls-vpn SAFIs.
func parseAnnounceAFIRoutes(afiTree *config.Tree) ([]StaticRouteConfig, error) {
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
func extractRoutesFromTree(tree *config.Tree) (*UpdateBlockRoutes, error) {
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

		// All watchdog routes are owned by the bgp-watchdog plugin and must not
		// be added as engine static routes. The plugin sends them via UpdateRoute
		// on peer session establishment (for non-withdrawn routes) and on
		// explicit "watchdog announce" commands (for withdrawn routes).
		if hasWatchdogContainer(entry.Value) {
			continue
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

// hasWatchdogContainer returns true if an update block has a watchdog container.
// All watchdog routes are owned by the bgp-watchdog plugin and must not be
// added as engine static routes.
func hasWatchdogContainer(update *config.Tree) bool {
	return update.GetContainer("watchdog") != nil
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
func extractRoutesFromUpdateBlock(update *config.Tree) (*UpdateBlockRoutes, error) {
	result := &UpdateBlockRoutes{}

	// Parse attributes from attribute { } container
	attr := update.GetContainer("attribute")
	if attr == nil {
		attr = config.NewTree() // Empty attributes if not specified
	}

	// Parse nlri list entries - each has key=family and content=operation+payload
	nlriEntries := update.GetListOrdered("nlri")
	if len(nlriEntries) == 0 {
		return nil, fmt.Errorf("missing nlri block in update")
	}

	for _, nlriEntry := range nlriEntries {
		famName := config.StripListKeySuffix(nlriEntry.Key)
		content, _ := nlriEntry.Value.Get("content")
		line := famName
		if content != "" {
			line = famName + " " + content
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		// Handle complex NLRI families specially
		switch famName {
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

		case familyIPv4MUP, familyIPv6MUP:
			mr, err := parseMUPNLRILine(line, attr)
			if err != nil {
				return nil, fmt.Errorf("mup nlri: %w", err)
			}
			result.MUPRoutes = append(result.MUPRoutes, mr)
			continue
		}

		// Standard families with simple prefixes
		// Format: family [add|del|eor] [rd VALUE] [label VALUE] PREFIX...
		//   ipv4/unicast add 10.0.0.0/24;
		//   ipv4/mpls-vpn add rd 65000:100 label 100 10.0.0.0/24;
		remaining := parts[1:]

		// Validate family is registered
		if _, ok := family.LookupFamily(famName); !ok {
			return nil, fmt.Errorf("invalid family: %s", famName)
		}

		if len(remaining) == 0 {
			continue // Bare family declaration with no prefixes
		}

		// Operation keyword (add/del/eor) is mandatory
		op := remaining[0]
		if op != opAdd && op != opDel && op != opEor {
			return nil, fmt.Errorf("missing operation keyword (add/del/eor) for family %s, got %q", famName, op)
		}
		remaining = remaining[1:]

		if op == opEor || len(remaining) == 0 {
			continue // EOR or no prefixes after operation keyword
		}

		// Parse inline rd/label for VPN/labeled families (order doesn't matter)
		var inlineRD, inlineLabel string
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

		prefixes := remaining
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

			result.StaticRoutes = append(result.StaticRoutes, sr)
		}
	}

	return result, nil
}

// applyAttributesFromTree applies path attributes from a Tree to a StaticRouteConfig.
// Used by both parseRouteConfig (announce/static syntax) and extractRoutesFromUpdateBlock (update syntax).
func applyAttributesFromTree(tree *config.Tree, sr *StaticRouteConfig) error {
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
	if items := tree.GetSlice("community"); len(items) > 0 {
		sr.Community = strings.Join(items, " ")
	}
	if items := tree.GetSlice("extended-community"); len(items) > 0 {
		sr.ExtendedCommunity = strings.Join(items, " ")
	}
	if items := tree.GetSlice("large-community"); len(items) > 0 {
		sr.LargeCommunity = strings.Join(items, " ")
	}
	if items := tree.GetSlice("as-path"); len(items) > 0 {
		sr.ASPath = strings.Join(items, " ")
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
	if items := tree.GetSlice("labels"); len(items) > 0 {
		sr.Labels = items
	}
	// Note: rd is parsed inline with NLRI, not from attributes
	if v, ok := tree.Get("aggregator"); ok {
		sr.Aggregator = v
	}
	// atomic-aggregate can be a standalone flag or have a value
	if _, ok := tree.Get("atomic-aggregate"); ok {
		sr.AtomicAggregate = true
	}
	if items := tree.GetSlice("attribute"); len(items) > 0 {
		sr.Attribute = strings.Join(items, " ")
	}
	if v, ok := tree.Get("originator-id"); ok {
		sr.OriginatorID = v
	}
	if items := tree.GetSlice("cluster-list"); len(items) > 0 {
		sr.ClusterList = strings.Join(items, " ")
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
	return nil
}

// parseRouteConfig extracts a StaticRouteConfig from a parsed route tree.
// The prefix key may have a #N suffix for duplicate routes (ADD-PATH support).
func parseRouteConfig(prefix string, route *config.Tree) (StaticRouteConfig, error) {
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
