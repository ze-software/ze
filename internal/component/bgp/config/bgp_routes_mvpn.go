// Design: docs/architecture/config/syntax.md — MVPN route extraction from config tree
// Overview: bgp_routes.go — route extraction orchestrator

package bgpconfig

import (
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// parseMVPNNLRILine parses an MVPN NLRI line like:
// "ipv4/mvpn add shared-join rp 10.99.199.1 group 239.251.255.228 rd 65000:99999 source-as 65000".
func parseMVPNNLRILine(line string, attr *config.Tree) (MVPNRouteConfig, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return MVPNRouteConfig{}, fmt.Errorf("mvpn nlri requires route type and fields")
	}

	fam := parts[0]
	mr := MVPNRouteConfig{
		IsIPv6: strings.HasPrefix(fam, "ipv6/"),
	}

	remaining := parts[1:]

	// Operation keyword (add/del/eor) is mandatory
	if len(remaining) == 0 {
		return MVPNRouteConfig{}, fmt.Errorf("missing operation keyword (add/del/eor) for family %s", fam)
	}
	op := remaining[0]
	if op != opAdd && op != opDel && op != opEor {
		return MVPNRouteConfig{}, fmt.Errorf("missing operation keyword (add/del/eor) for family %s, got %q", fam, op)
	}
	remaining = remaining[1:]
	if op == opEor {
		return mr, nil
	}

	// Route type is next field after operation
	if len(remaining) > 0 {
		mr.RouteType = remaining[0]
	}

	// Parse key-value pairs
	for i := 1; i < len(remaining); i += 2 {
		if i+1 >= len(remaining) {
			break
		}
		key, val := remaining[i], remaining[i+1]
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
	if items := attr.GetSlice("extended-community"); len(items) > 0 {
		mr.ExtendedCommunity = strings.Join(items, " ")
	}
	if v, ok := attr.Get("originator-id"); ok {
		mr.OriginatorID = v
	}
	if items := attr.GetSlice("cluster-list"); len(items) > 0 {
		mr.ClusterList = strings.Join(items, " ")
	}

	return mr, nil
}

// extractMVPNRoutes extracts MVPN routes from announce { ipv4/ipv6 { mcast-vpn ... } }.
func extractMVPNRoutes(tree *config.Tree) []MVPNRouteConfig {
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

func parseMVPNRoute(routeType string, route *config.Tree, isIPv6 bool) MVPNRouteConfig {
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
	if items := route.GetSlice("extended-community"); len(items) > 0 {
		r.ExtendedCommunity = strings.Join(items, " ")
	}
	if v, ok := route.Get("originator-id"); ok {
		r.OriginatorID = v
	}
	if items := route.GetSlice("cluster-list"); len(items) > 0 {
		r.ClusterList = strings.Join(items, " ")
	}

	return r
}
