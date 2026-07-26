// Design: plan/learned/710-gap-2-static-route-enhancements.md -- config parsing

package static

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/routingtable"
)

var (
	errRouteMissingPrefix = errors.New("route missing prefix")
)

// maxNetlinkInt is the largest value this build can carry in one of the
// netlink bindings' int-typed route fields (netlink.Route.Table, .Priority).
// The encoder emits RTA_TABLE only when Table > 0 and RTA_PRIORITY only when
// Priority > 0 (vendor/github.com/vishvananda/netlink/route_linux.go:1058,1069),
// so on a 32-bit build a uint32 above MaxInt32 turns negative and the attribute
// is dropped without an error: the route lands in RT_TABLE_MAIN (the RtMsg
// default, nl/route_linux.go:16) at the kernel's default metric instead of
// where the operator put it. On the 64-bit targets Ze ships this bound is above
// every uint32 and never bites.
const maxNetlinkInt = uint64(math.MaxInt)

// validateRouteMetric rejects a metric this build cannot program without
// truncation. maxEncodable is a parameter, not the constant, so the 32-bit
// rejection stays testable on a 64-bit host where it can never be hit.
func validateRouteMetric(metric uint32, maxEncodable uint64) error {
	if uint64(metric) > maxEncodable {
		return fmt.Errorf("metric: value %d exceeds %d, the largest this build can program through netlink", metric, maxEncodable)
	}
	return nil
}

// parseStaticConfig parses the static config JSON (map format from Tree.ToMap).
// The tree shape is:
//
//	{"static": {"table": {"<name>": {"route": {"<prefix>": {...}}}}}}
//
// Lists are keyed maps (list key = map key), not arrays.
func parseStaticConfig(jsonData string, reg *routingtable.Registry) ([]staticRoute, error) {
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, fmt.Errorf("unmarshal static config: %w", err)
	}

	staticTree, ok := data["static"].(map[string]any)
	if !ok {
		return nil, nil
	}

	tableMap, ok := staticTree["table"].(map[string]any)
	if !ok {
		return nil, nil
	}

	var routes []staticRoute
	for tableName, tableValue := range tableMap {
		tableTree, ok := tableValue.(map[string]any)
		if !ok {
			continue
		}

		tableID, err := reg.Resolve(tableName)
		if err != nil {
			return nil, fmt.Errorf("static table %q: %w", tableName, err)
		}

		routeMap, ok := tableTree["route"].(map[string]any)
		if !ok {
			continue
		}

		for prefixStr, routeValue := range routeMap {
			routeTree, ok := routeValue.(map[string]any)
			if !ok {
				continue
			}

			r, err := parseRoute(prefixStr, routeTree)
			if err != nil {
				return nil, err
			}
			r.Table = tableID
			routes = append(routes, r)
		}
	}

	return routes, nil
}

func parseRoute(prefixStr string, entry map[string]any) (staticRoute, error) {
	var r staticRoute

	if prefixStr == "" {
		return r, errRouteMissingPrefix
	}
	pfx, err := netip.ParsePrefix(prefixStr)
	if err != nil {
		return r, fmt.Errorf("invalid prefix %q: %w", prefixStr, err)
	}
	r.Prefix = pfx.Masked()

	r.Description, _ = entry["description"].(string)

	metric, err := mapUint32(entry, "metric")
	if err != nil {
		return r, fmt.Errorf("route %s: %w", prefixStr, err)
	}
	if err := validateRouteMetric(metric, maxNetlinkInt); err != nil {
		return r, fmt.Errorf("route %s: %w", prefixStr, err)
	}
	r.Metric = metric

	tag, err := mapUint32(entry, "tag")
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

	nextMap, _ := entry["next"].(map[string]any)
	if len(nextMap) == 0 {
		return r, fmt.Errorf("route %s: must have next, blackhole, or reject", prefixStr)
	}

	r.Action = actionForward

	hopMap, _ := nextMap["hop"].(map[string]any)
	for addr, nhValue := range hopMap {
		nhTree, ok := nhValue.(map[string]any)
		if !ok {
			nhTree = map[string]any{}
		}
		nh, err := parseNextHop(addr, nhTree)
		if err != nil {
			return r, fmt.Errorf("route %s: %w", prefixStr, err)
		}
		r.NextHops = append(r.NextHops, nh)
	}

	ifMap, _ := nextMap["interface"].(map[string]any)
	for ifName, ifValue := range ifMap {
		ifTree, ok := ifValue.(map[string]any)
		if !ok {
			ifTree = map[string]any{}
		}
		nh, err := parseInterfaceNextHop(ifName, ifTree)
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

func parseNextHop(addrStr string, entry map[string]any) (nextHop, error) {
	var nh nextHop

	if addrStr == "" {
		return nh, errors.New("next-hop missing address")
	}
	addr, err := netip.ParseAddr(addrStr)
	if err != nil {
		return nh, fmt.Errorf("invalid next-hop address %q: %w", addrStr, err)
	}
	nh.Address = addr

	nh.Interface, _ = entry["interface"].(string)
	nh.BFDProfile, _ = entry["bfd-profile"].(string)

	w, err := mapUint32(entry, "weight")
	if err != nil {
		return nh, err
	}
	switch {
	case w == 0:
		nh.Weight = 1
	case w > 65535:
		return nh, fmt.Errorf("weight %d exceeds maximum 65535", w)
	default:
		nh.Weight = uint16(w)
	}

	return nh, nil
}

func parseInterfaceNextHop(ifName string, entry map[string]any) (nextHop, error) {
	var nh nextHop

	if ifName == "" {
		return nh, errors.New("next interface missing name")
	}
	nh.Interface = ifName

	if bfd, _ := entry["bfd-profile"].(string); bfd != "" {
		return nh, fmt.Errorf("next interface %q: BFD profile not allowed (BFD requires a peer address)", nh.Interface)
	}

	w, err := mapUint32(entry, "weight")
	if err != nil {
		return nh, err
	}
	switch {
	case w == 0:
		nh.Weight = 1
	case w > 65535:
		return nh, fmt.Errorf("weight %d exceeds maximum 65535", w)
	default:
		nh.Weight = uint16(w)
	}

	return nh, nil
}

func mapUint32(m map[string]any, key string) (uint32, error) {
	v, ok := m[key]
	if !ok {
		return 0, nil
	}
	n, ok := cfgFloat(v)
	if !ok {
		return 0, nil
	}
	if n < 0 || n > math.MaxUint32 {
		return 0, fmt.Errorf("%s: value %v out of uint32 range", key, n)
	}
	return uint32(n), nil
}

// cfgFloat coerces a config value to float64. The plugin config framework
// delivers YANG leaf values as JSON strings (e.g. "200"), so the string form is
// accepted alongside the native JSON number. Without it string-valued metric,
// tag, and weight leaves would silently fall back to zero.
func cfgFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}
