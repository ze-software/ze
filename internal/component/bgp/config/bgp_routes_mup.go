// Design: docs/architecture/config/syntax.md — MUP route extraction from config tree
// Overview: bgp_routes.go — route extraction orchestrator
// Related: bgp_routes_inline.go — shared inline key-value tokenizer

package bgpconfig

import (
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// parseMUPNLRILine parses a MUP NLRI line like:
// "ipv4/mup mup-isd 10.0.1.0/24 rd 100:100".
func parseMUPNLRILine(line string, attr *config.Tree) (MUPRouteConfig, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return MUPRouteConfig{}, fmt.Errorf("mup nlri requires route type and fields")
	}

	family := parts[0]
	mr := MUPRouteConfig{
		IsIPv6: strings.HasPrefix(family, "ipv6/"),
	}

	remaining := parts[1:]

	// Operation keyword (add/del/eor) is mandatory
	if len(remaining) == 0 {
		return MUPRouteConfig{}, fmt.Errorf("missing operation keyword (add/del/eor) for family %s", family)
	}
	op := remaining[0]
	if op != opAdd && op != opDel && op != opEor {
		return MUPRouteConfig{}, fmt.Errorf("missing operation keyword (add/del/eor) for family %s, got %q", family, op)
	}
	remaining = remaining[1:]
	if op == opEor {
		return mr, nil
	}

	// Route type is next field after operation (mup-isd, mup-dsd, mup-t1st, mup-t2st)
	if len(remaining) > 0 {
		mr.RouteType = remaining[0]
	}

	// Next field is typically the prefix/address
	if len(remaining) > 1 {
		switch mr.RouteType {
		case routeTypeMUPISD:
			mr.Prefix = remaining[1]
		case routeTypeMUPDSD:
			mr.Address = remaining[1]
		case routeTypeMUPT1ST:
			mr.Prefix = remaining[1]
		case routeTypeMUPT2ST:
			mr.Address = remaining[1]
		}
	}

	// Parse remaining key-value pairs
	for i := 2; i < len(remaining); i += 2 {
		if i+1 >= len(remaining) {
			break
		}
		key, val := remaining[i], remaining[i+1]
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
	if items := attr.GetSlice("extended-community"); len(items) > 0 {
		mr.ExtendedCommunity = strings.Join(items, " ")
	}
	if v, ok := attr.GetFlex("bgp-prefix-sid-srv6"); ok {
		mr.PrefixSID = v
	}

	return mr, nil
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
func extractMUPRoutes(tree *config.Tree) []MUPRouteConfig {
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

func parseMUPRoute(routeType string, route *config.Tree, isIPv6 bool) MUPRouteConfig {
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
	if items := route.GetSlice("extended-community"); len(items) > 0 {
		r.ExtendedCommunity = strings.Join(items, " ")
	}
	if v, ok := route.Get("bgp-prefix-sid-srv6"); ok {
		r.PrefixSID = v
	}

	return r
}
