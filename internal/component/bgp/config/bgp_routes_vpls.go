// Design: docs/architecture/config/syntax.md — VPLS route extraction from config tree
// Overview: bgp_routes.go — route extraction orchestrator
// Related: bgp_routes_inline.go — shared inline key-value tokenizer

package bgpconfig

import (
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// parseVPLSNLRILine parses a VPLS NLRI line like:
// "l2vpn/vpls rd 192.168.201.1:123 ve-id 5 ve-block-offset 1 ve-block-size 8 label-base 10702".
func parseVPLSNLRILine(line string, attr *config.Tree) (VPLSRouteConfig, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return VPLSRouteConfig{}, fmt.Errorf("vpls nlri requires fields")
	}

	vr := VPLSRouteConfig{}

	// Operation keyword (add/del/eor) is mandatory
	remaining := parts[1:]
	// Parse rd before operation keyword: l2vpn/vpls rd X add ve-id ...
	if len(remaining) >= 2 && remaining[0] == "rd" {
		vr.RD = remaining[1]
		remaining = remaining[2:]
	}
	if len(remaining) == 0 {
		return VPLSRouteConfig{}, fmt.Errorf("missing operation keyword (add/del/eor) for family l2vpn/vpls")
	}
	op := remaining[0]
	if op != opAdd && op != opDel && op != opEor {
		return VPLSRouteConfig{}, fmt.Errorf("missing operation keyword (add/del/eor) for family l2vpn/vpls, got %q", op)
	}
	remaining = remaining[1:]
	if op == opEor {
		return vr, nil
	}

	// Parse key-value pairs from remaining (after operation keyword)
	for i := 0; i < len(remaining); i += 2 {
		if i+1 >= len(remaining) {
			break
		}
		key, val := remaining[i], remaining[i+1]
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
	if items := attr.GetSlice("as-path"); len(items) > 0 {
		vr.ASPath = strings.Join(items, " ")
	}
	if v, ok := attr.Get("local-preference"); ok {
		n, _ := strconv.ParseUint(v, 10, 32)
		vr.LocalPreference = uint32(n)
	}
	if v, ok := attr.Get("med"); ok {
		n, _ := strconv.ParseUint(v, 10, 32)
		vr.MED = uint32(n)
	}
	if items := attr.GetSlice("community"); len(items) > 0 {
		vr.Community = strings.Join(items, " ")
	}
	if items := attr.GetSlice("extended-community"); len(items) > 0 {
		vr.ExtendedCommunity = strings.Join(items, " ")
	}
	if v, ok := attr.Get("originator-id"); ok {
		vr.OriginatorID = v
	}
	if items := attr.GetSlice("cluster-list"); len(items) > 0 {
		vr.ClusterList = strings.Join(items, " ")
	}

	return vr, nil
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
func extractVPLSRoutes(tree *config.Tree) []VPLSRouteConfig {
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

func parseVPLSRoute(name string, route *config.Tree) VPLSRouteConfig {
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
	if items := route.GetSlice("as-path"); len(items) > 0 {
		r.ASPath = strings.Join(items, " ")
	}
	if items := route.GetSlice("community"); len(items) > 0 {
		r.Community = strings.Join(items, " ")
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
