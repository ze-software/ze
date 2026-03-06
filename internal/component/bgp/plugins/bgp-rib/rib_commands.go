// Design: docs/architecture/plugin/rib-storage-design.md — RIB command handlers
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_nlri.go — NLRI wire format helpers
// Related: rib_attr_format.go — attribute formatting for show enrichment
// Related: bestpath.go — best-path selection (extractCandidate, gatherCandidates, SelectBest)
package bgp_rib

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp-rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp-rib/storage"
)

// handleCommand processes command requests via SDK execute-command callback.
// Returns (status, data, error) for the SDK to send back to the engine.
// Supports both short names (rib show in) and legacy names (rib adjacent inbound show).
func (r *RIBManager) handleCommand(command, selector string, args []string) (string, string, error) {
	switch command {
	case "rib status", "rib adjacent status":
		return statusDone, r.statusJSON(), nil
	case "rib show in", "rib adjacent inbound show":
		return statusDone, r.inboundShowJSON(selector, args), nil
	case "rib clear in", "rib adjacent inbound empty":
		return statusDone, r.inboundEmptyJSON(selector), nil
	case "rib show out", "rib adjacent outbound show":
		return statusDone, r.outboundShowJSON(selector), nil
	case "rib clear out", "rib adjacent outbound resend":
		return statusDone, r.outboundResendJSON(selector), nil
	case "rib retain-routes":
		return statusDone, r.retainRoutesJSON(selector), nil
	case "rib release-routes":
		return statusDone, r.releaseRoutesJSON(selector), nil
	case "rib show best":
		return statusDone, r.bestPathShowJSON(selector, args), nil
	case "rib best status":
		return statusDone, r.bestPathStatusJSON(), nil
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
// Optional args filter by family (e.g., "ipv4/unicast") or prefix (e.g., "10.0.0.0/24").
func (r *RIBManager) inboundShowJSON(selector string, args []string) string {
	familyFilter, prefixFilter := parseShowFilters(args)

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]map[string]any)

	for peer, peerRIB := range r.ribInPool {
		if !matchesPeer(peer, selector) {
			continue
		}
		var routeList []map[string]any
		peerRIB.Iterate(func(family nlri.Family, nlriBytes []byte, entry *storage.RouteEntry) bool {
			familyStr := formatFamily(family)
			prefixStr := formatNLRIAsPrefix(family, nlriBytes)

			if familyFilter != "" && familyStr != familyFilter {
				return true
			}
			if prefixFilter != "" && prefixStr != prefixFilter {
				return true
			}

			routeMap := map[string]any{
				"family": familyStr,
				"prefix": prefixStr,
			}
			if entry != nil {
				enrichRouteMapFromEntry(routeMap, entry)
			}
			routeList = append(routeList, routeMap)
			return true
		})
		if len(routeList) > 0 {
			result[peer] = routeList
		}
	}

	data, _ := json.Marshal(map[string]any{"adj-rib-in": result})
	return string(data)
}

// parseShowFilters extracts family and prefix filters from command args.
// Family: "afi/safi" (e.g., "ipv4/unicast") — starts with letter, no colons.
// Prefix: IP/len (e.g., "10.0.0.0/24", "fc00::/7") — has digits or colons.
func parseShowFilters(args []string) (familyFilter, prefixFilter string) {
	for _, arg := range args {
		if !strings.Contains(arg, "/") {
			continue
		}
		// Family names never contain colons; IPv6 prefixes always do.
		if arg[0] >= 'a' && arg[0] <= 'z' && !strings.Contains(arg, ":") {
			familyFilter = arg
		} else {
			prefixFilter = arg
		}
	}
	return
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
		delete(r.peerMeta, peer)
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
			enrichRouteMapFromRoute(routeMap, rt)
			routeList = append(routeList, routeMap)
		}
		result[peer] = routeList
	}

	data, _ := json.Marshal(map[string]any{"adj-rib-out": result})
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

	return fmt.Sprintf(`{"running":true,"peers":%d,"routes-in":%d,"routes-out":%d}`,
		len(r.peerUp), routesIn, routesOut)
}

// retainRoutesJSON marks a peer's Adj-RIB-In for retention during GR.
// RFC 4724: Receiving speaker retains routes from restarting peer.
// Called by bgp-gr plugin via DispatchCommand("rib retain-routes <peer>").
func (r *RIBManager) retainRoutesJSON(selector string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	retained := 0
	for peer := range r.ribInPool {
		if !matchesPeer(peer, selector) {
			continue
		}
		r.retainedPeers[peer] = true
		retained++
	}

	data, _ := json.Marshal(map[string]any{"retained-peers": retained})
	return string(data)
}

// releaseRoutesJSON clears the retain flag and deletes Adj-RIB-In for matching peers.
// RFC 4724: Called when restart timer expires or GR completes.
// Called by bgp-gr plugin via DispatchCommand("rib release-routes <peer>").
func (r *RIBManager) releaseRoutesJSON(selector string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	released := 0
	for peer := range r.retainedPeers {
		if !matchesPeer(peer, selector) {
			continue
		}
		delete(r.retainedPeers, peer)
		if peerRIB := r.ribInPool[peer]; peerRIB != nil {
			peerRIB.Release()
			delete(r.ribInPool, peer)
		}
		delete(r.peerMeta, peer)
		released++
	}

	data, _ := json.Marshal(map[string]any{"released-peers": released})
	return string(data)
}

// bestPathShowJSON computes and returns the best route per prefix across all peers.
// RFC 4271 §9.1.2: Decision Process Phase 2 — on-demand computation.
// Optional args filter by family or prefix (same as inboundShowJSON).
func (r *RIBManager) bestPathShowJSON(selector string, args []string) string {
	familyFilter, prefixFilter := parseShowFilters(args)

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect all unique (family, nlriKey) across matching peers.
	type routeKey struct {
		family  nlri.Family
		nlriKey string
		familyS string
		prefixS string
	}
	seen := make(map[string]routeKey) // "familyStr|nlriKey" → routeKey

	for peer, peerRIB := range r.ribInPool {
		if !matchesPeer(peer, selector) {
			continue
		}
		peerRIB.Iterate(func(family nlri.Family, nlriBytes []byte, _ *storage.RouteEntry) bool {
			fStr := formatFamily(family)
			pStr := formatNLRIAsPrefix(family, nlriBytes)

			if familyFilter != "" && fStr != familyFilter {
				return true
			}
			if prefixFilter != "" && pStr != prefixFilter {
				return true
			}

			key := fStr + "|" + string(nlriBytes)
			if _, ok := seen[key]; !ok {
				seen[key] = routeKey{family: family, nlriKey: string(nlriBytes), familyS: fStr, prefixS: pStr}
			}
			return true
		})
	}

	// For each unique prefix, gather candidates and select best.
	type bestResult struct {
		Family   string         `json:"family"`
		Prefix   string         `json:"prefix"`
		BestPeer string         `json:"best-peer"`
		Attrs    map[string]any `json:"attributes,omitempty"`
	}

	var results []bestResult
	for _, rk := range seen {
		candidates := r.gatherCandidates(rk.family, []byte(rk.nlriKey))
		best := SelectBest(candidates)
		if best == nil {
			continue
		}

		br := bestResult{
			Family:   rk.familyS,
			Prefix:   rk.prefixS,
			BestPeer: best.PeerAddr,
		}

		// Enrich with attributes from the best route's pool entry.
		if peerRIB := r.ribInPool[best.PeerAddr]; peerRIB != nil {
			if entry, ok := peerRIB.Lookup(rk.family, []byte(rk.nlriKey)); ok && entry != nil {
				attrs := make(map[string]any)
				enrichRouteMapFromEntry(attrs, entry)
				if len(attrs) > 0 {
					br.Attrs = attrs
				}
			}
		}

		results = append(results, br)
	}

	// Sort by family then prefix for stable output.
	sort.Slice(results, func(i, j int) bool {
		if results[i].Family != results[j].Family {
			return results[i].Family < results[j].Family
		}
		return results[i].Prefix < results[j].Prefix
	})

	data, _ := json.Marshal(map[string]any{"best-path": results})
	return string(data)
}

// bestPathStatusJSON returns summary statistics about the best-path computation.
func (r *RIBManager) bestPathStatusJSON() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	totalPeers := len(r.ribInPool)
	totalRoutes := 0
	for _, peerRIB := range r.ribInPool {
		totalRoutes += peerRIB.Len()
	}

	data, _ := json.Marshal(map[string]any{
		"running":        true,
		"peers-with-rib": totalPeers,
		"total-routes":   totalRoutes,
	})
	return string(data)
}

// gatherCandidates collects best-path candidates for a given (family, nlri) across all peers.
// Returns extracted Candidate structs ready for SelectBest.
// Caller must hold at least read lock.
func (r *RIBManager) gatherCandidates(family nlri.Family, nlriBytes []byte) []*Candidate {
	var candidates []*Candidate
	for peer, peerRIB := range r.ribInPool {
		entry, ok := peerRIB.Lookup(family, nlriBytes)
		if !ok || entry == nil {
			continue
		}
		c := r.extractCandidate(peer, entry)
		candidates = append(candidates, c)
	}
	return candidates
}

// extractCandidate builds a Candidate from a RouteEntry by reading pool handles.
// Extracts attribute values needed for RFC 4271 §9.1.2 comparison.
func (r *RIBManager) extractCandidate(peerAddr string, entry *storage.RouteEntry) *Candidate {
	c := &Candidate{
		PeerAddr:  peerAddr,
		LocalPref: 100, // RFC 4271 default
	}

	// Peer metadata for eBGP/iBGP detection.
	if meta := r.peerMeta[peerAddr]; meta != nil {
		c.PeerASN = meta.PeerASN
		c.LocalASN = meta.LocalASN
	}

	// LOCAL_PREF (type 5): 4 bytes, higher wins.
	if entry.HasLocalPref() {
		if data, err := pool.LocalPref.Get(entry.LocalPref); err == nil {
			if v, ok := formatUint32Attr(data); ok {
				c.LocalPref = v
			}
		}
	}

	// AS_PATH (type 2): wire bytes, count length and extract first AS.
	if entry.HasASPath() {
		if data, err := pool.ASPath.Get(entry.ASPath); err == nil {
			c.ASPathLen = asPathLength(data)
			c.FirstAS = firstASInPath(data)
		}
	}

	// ORIGIN (type 1): 1 byte (0=IGP, 1=EGP, 2=INCOMPLETE).
	if entry.HasOrigin() {
		if data, err := pool.Origin.Get(entry.Origin); err == nil && len(data) > 0 {
			c.Origin = data[0]
		}
	}

	// MED (type 4): 4 bytes, lower wins.
	if entry.HasMED() {
		if data, err := pool.MED.Get(entry.MED); err == nil {
			if v, ok := formatUint32Attr(data); ok {
				c.MED = v
			}
		}
	}

	// ORIGINATOR_ID (type 9): 4 bytes, used as Router ID tiebreak (RFC 4456).
	if entry.HasOriginatorID() {
		if data, err := pool.OriginatorID.Get(entry.OriginatorID); err == nil {
			c.OriginatorID = formatNextHop(data) // same 4-byte IP format
		}
	}

	return c
}
