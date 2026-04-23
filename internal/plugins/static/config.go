// Design: plan/spec-static-routes.md -- config parsing

package static

import (
	"encoding/json"
	"fmt"
	"math"
	"net/netip"
)

func parseStaticConfig(jsonData string) ([]staticRoute, error) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(jsonData), &tree); err != nil {
		return nil, fmt.Errorf("unmarshal static config: %w", err)
	}

	staticTree, ok := tree["static"].(map[string]any)
	if !ok {
		return nil, nil
	}

	routeList, ok := staticTree["route"].([]any)
	if !ok {
		return nil, nil
	}

	seen := make(map[netip.Prefix]bool, len(routeList))
	var routes []staticRoute
	for _, item := range routeList {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}

		r, err := parseRoute(entry)
		if err != nil {
			return nil, err
		}
		if seen[r.Prefix] {
			return nil, fmt.Errorf("duplicate route prefix %s", r.Prefix)
		}
		seen[r.Prefix] = true
		routes = append(routes, r)
	}

	return routes, nil
}

func parseRoute(entry map[string]any) (staticRoute, error) {
	var r staticRoute

	prefixStr, _ := entry["prefix"].(string)
	if prefixStr == "" {
		return r, fmt.Errorf("route missing prefix")
	}
	pfx, err := netip.ParsePrefix(prefixStr)
	if err != nil {
		return r, fmt.Errorf("invalid prefix %q: %w", prefixStr, err)
	}
	r.Prefix = pfx.Masked()

	r.Description, _ = entry["description"].(string)

	metric, err := jsonUint32(entry, "metric")
	if err != nil {
		return r, fmt.Errorf("route %s: %w", prefixStr, err)
	}
	r.Metric = metric

	tag, err := jsonUint32(entry, "tag")
	if err != nil {
		return r, fmt.Errorf("route %s: %w", prefixStr, err)
	}
	r.Tag = tag

	if _, ok := entry["blackhole"]; ok {
		r.Action = actionBlackhole
		return r, nil
	}
	if _, ok := entry["reject"]; ok {
		r.Action = actionReject
		return r, nil
	}

	nhList, ok := entry["next-hop"].([]any)
	if !ok || len(nhList) == 0 {
		return r, fmt.Errorf("route %s: must have next-hop, blackhole, or reject", prefixStr)
	}

	r.Action = actionForward
	for _, nhItem := range nhList {
		nhMap, ok := nhItem.(map[string]any)
		if !ok {
			continue
		}
		nh, err := parseNextHop(nhMap)
		if err != nil {
			return r, fmt.Errorf("route %s: %w", prefixStr, err)
		}
		r.NextHops = append(r.NextHops, nh)
	}

	if len(r.NextHops) == 0 {
		return r, fmt.Errorf("route %s: no valid next-hops", prefixStr)
	}

	return r, nil
}

func parseNextHop(entry map[string]any) (nextHop, error) {
	var nh nextHop

	addrStr, _ := entry["address"].(string)
	if addrStr == "" {
		return nh, fmt.Errorf("next-hop missing address")
	}
	addr, err := netip.ParseAddr(addrStr)
	if err != nil {
		return nh, fmt.Errorf("invalid next-hop address %q: %w", addrStr, err)
	}
	nh.Address = addr

	nh.Interface, _ = entry["interface"].(string)
	nh.BFDProfile, _ = entry["bfd-profile"].(string)

	w, err := jsonUint32(entry, "weight")
	if err != nil {
		return nh, err
	}
	if w == 0 {
		nh.Weight = 1
	} else if w > 65535 {
		return nh, fmt.Errorf("weight %d exceeds maximum 65535", w)
	} else {
		nh.Weight = uint16(w)
	}

	return nh, nil
}

func jsonUint32(m map[string]any, key string) (uint32, error) {
	v, ok := m[key]
	if !ok {
		return 0, nil
	}
	switch n := v.(type) {
	case float64:
		if n < 0 || n > math.MaxUint32 {
			return 0, fmt.Errorf("%s: value %v out of uint32 range", key, n)
		}
		return uint32(n), nil
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, fmt.Errorf("%s: %w", key, err)
		}
		if i < 0 || i > math.MaxUint32 {
			return 0, fmt.Errorf("%s: value %d out of uint32 range", key, i)
		}
		return uint32(i), nil
	}
	return 0, nil
}
