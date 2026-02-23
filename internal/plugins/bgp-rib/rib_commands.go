// Design: docs/architecture/plugin/rib-storage-design.md — RIB command handlers
// Related: rib.go — RIB plugin core types and event handlers
// Related: rib_nlri.go — NLRI wire format helpers
package bgp_rib

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp-rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp-rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// handleCommand processes command requests via SDK execute-command callback.
// Returns (status, data, error) for the SDK to send back to the engine.
// Supports both short names (rib show in) and legacy names (rib adjacent inbound show).
func (r *RIBManager) handleCommand(command, selector string) (string, string, error) {
	switch command {
	case "rib status", "rib adjacent status":
		return statusDone, r.statusJSON(), nil
	case "rib show in", "rib adjacent inbound show":
		return statusDone, r.inboundShowJSON(selector), nil
	case "rib clear in", "rib adjacent inbound empty":
		return statusDone, r.inboundEmptyJSON(selector), nil
	case "rib show out", "rib adjacent outbound show":
		return statusDone, r.outboundShowJSON(selector), nil
	case "rib clear out", "rib adjacent outbound resend":
		return statusDone, r.outboundResendJSON(selector), nil
	default: // fail on unknown command
		return "error", "", fmt.Errorf("unknown command: %s", command)
	}
}

// matchesPeer returns true if peerAddr matches the selector string.
// Supports: *, IP, !IP (negation), IP,IP,IP (multi-IP).
func matchesPeer(peerAddr, selector string) bool {
	selector = strings.TrimSpace(selector)

	if selector == "" || selector == "*" {
		return true
	}

	// Negation: !IP matches all except that IP
	if strings.HasPrefix(selector, "!") {
		excludeIP := strings.TrimSpace(selector[1:])
		return peerAddr != excludeIP
	}

	// Multi-IP: IP,IP,IP matches any in list
	if strings.Contains(selector, ",") {
		for s := range strings.SplitSeq(selector, ",") {
			if strings.TrimSpace(s) == peerAddr {
				return true
			}
		}
		return false
	}

	// Single IP
	return peerAddr == selector
}

// inboundShowJSON returns Adj-RIB-In routes filtered by selector as JSON.
func (r *RIBManager) inboundShowJSON(selector string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]map[string]any)

	for peer, peerRIB := range r.ribInPool {
		if !matchesPeer(peer, selector) {
			continue
		}
		var routeList []map[string]any
		peerRIB.Iterate(func(family nlri.Family, nlriBytes []byte, entry *storage.RouteEntry) bool {
			routeMap := map[string]any{
				"family": formatFamily(family),
				"prefix": formatNLRIAsPrefix(family, nlriBytes),
			}
			// Add next-hop if available from RouteEntry.
			if entry != nil && entry.HasNextHop() {
				if nhData, err := pool.NextHop.Get(entry.NextHop); err == nil {
					routeMap["next-hop"] = formatNextHop(nhData)
				}
			}
			routeList = append(routeList, routeMap)
			return true
		})
		if len(routeList) > 0 {
			result[peer] = routeList
		}
	}

	data, _ := json.Marshal(map[string]any{"adj_rib_in": result})
	return string(data)
}

// inboundEmptyJSON clears Adj-RIB-In routes for matching peers, returns JSON result.
func (r *RIBManager) inboundEmptyJSON(selector string) string {
	r.mu.Lock()
	cleared := 0

	for peer, peerRIB := range r.ribInPool {
		if !matchesPeer(peer, selector) {
			continue
		}
		cleared += peerRIB.Len()
		peerRIB.Release()
		delete(r.ribInPool, peer)
	}
	r.mu.Unlock()

	data, _ := json.Marshal(map[string]any{"cleared": cleared})
	return string(data)
}

// outboundShowJSON returns Adj-RIB-Out routes filtered by selector as JSON.
func (r *RIBManager) outboundShowJSON(selector string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]map[string]any)
	for peer, routes := range r.ribOut {
		if !matchesPeer(peer, selector) {
			continue
		}
		routeList := make([]map[string]any, 0, len(routes))
		for _, rt := range routes {
			routeMap := map[string]any{
				"family":   rt.Family,
				"prefix":   rt.Prefix,
				"next-hop": rt.NextHop,
			}
			if rt.PathID != 0 {
				routeMap["path-id"] = rt.PathID
			}
			routeList = append(routeList, routeMap)
		}
		result[peer] = routeList
	}

	data, _ := json.Marshal(map[string]any{"adj_rib_out": result})
	return string(data)
}

// outboundResendJSON replays Adj-RIB-Out routes for matching peers, returns JSON result.
// Does NOT send "plugin session ready" - that's only for initial reconnect.
func (r *RIBManager) outboundResendJSON(selector string) string {
	r.mu.RLock()
	var peersToResend []string
	routesToResend := make(map[string][]*Route)

	for peer, routes := range r.ribOut {
		if !matchesPeer(peer, selector) {
			continue
		}
		if !r.peerUp[peer] {
			continue // Only resend to up peers
		}
		peersToResend = append(peersToResend, peer)
		routesCopy := make([]*Route, 0, len(routes))
		for _, rt := range routes {
			routesCopy = append(routesCopy, rt)
		}
		routesToResend[peer] = routesCopy
	}
	r.mu.RUnlock()

	// Replay routes outside lock - use sendRoutes, not replayRoutes
	resent := 0
	for _, peer := range peersToResend {
		routes := routesToResend[peer]
		r.sendRoutes(peer, routes)
		resent += len(routes)
	}

	data, _ := json.Marshal(map[string]any{"resent": resent, "peers": len(peersToResend)})
	return string(data)
}

// sendRoutes sends routes to a peer without the "plugin session ready" signal.
// Used for manual resend operations. Includes full path attributes.
func (r *RIBManager) sendRoutes(peerAddr string, routes []*Route) {
	// Sort by MsgID to send in original announcement order
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].MsgID < routes[j].MsgID
	})

	for _, route := range routes {
		cmd := formatRouteCommand(route)
		r.updateRoute(peerAddr, cmd)
	}
}

// formatRouteCommand builds the update text command with full attributes.
// Format: update text [attrs...] nhop set <nh> nlri <family> add <prefix>.
// The peer selector is passed separately to updateRoute.
func formatRouteCommand(route *Route) string {
	var sb strings.Builder

	// Base command (peer selector is handled by updateRoute)
	sb.WriteString("update text")

	// Path-ID (RFC 7911) - must come before nlri
	if route.PathID != 0 {
		fmt.Fprintf(&sb, " path-information set %d", route.PathID)
	}

	// Origin
	if route.Origin != "" {
		sb.WriteString(" origin set ")
		sb.WriteString(route.Origin)
	}

	// AS-Path (use [] for list)
	if len(route.ASPath) > 0 {
		sb.WriteString(" as-path set ")
		sb.WriteString(attribute.FormatASPath(route.ASPath))
	}

	// MED
	if route.MED != nil {
		fmt.Fprintf(&sb, " med set %d", *route.MED)
	}

	// Local-Preference
	if route.LocalPreference != nil {
		fmt.Fprintf(&sb, " local-preference set %d", *route.LocalPreference)
	}

	// Communities (use [] for list)
	if len(route.Communities) > 0 {
		sb.WriteString(" community set [")
		sb.WriteString(strings.Join(route.Communities, " "))
		sb.WriteString("]")
	}

	// Large Communities (use [] for list)
	if len(route.LargeCommunities) > 0 {
		sb.WriteString(" large-community set [")
		sb.WriteString(strings.Join(route.LargeCommunities, " "))
		sb.WriteString("]")
	}

	// Extended Communities (use [] for list)
	if len(route.ExtendedCommunities) > 0 {
		sb.WriteString(" extended-community set [")
		sb.WriteString(strings.Join(route.ExtendedCommunities, " "))
		sb.WriteString("]")
	}

	// Next-hop (required)
	sb.WriteString(" nhop set ")
	sb.WriteString(route.NextHop)

	// NLRI with family
	sb.WriteString(" nlri ")
	sb.WriteString(route.Family)
	sb.WriteString(" add ")
	sb.WriteString(route.Prefix)

	return sb.String()
}

// statusJSON returns status as JSON.
func (r *RIBManager) statusJSON() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routesIn := 0
	for _, peerRIB := range r.ribInPool {
		routesIn += peerRIB.Len()
	}

	routesOut := 0
	for _, routes := range r.ribOut {
		routesOut += len(routes)
	}

	return fmt.Sprintf(`{"running":true,"peers":%d,"routes_in":%d,"routes_out":%d}`,
		len(r.peerUp), routesIn, routesOut)
}
