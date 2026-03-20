// Design: docs/architecture/plugin/rib-storage-design.md — attribute formatting for show commands
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_commands.go — command handling and JSON responses
// Related: rib_nlri.go — NLRI wire format helpers
// Related: bestpath.go — best-path selection (asPathLength, firstASInPath shared concern)
// Related: rib_pipeline.go — iterator pipeline for show commands
package rib

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
)

// enrichRouteMapFromEntry adds path attributes from a pool-based RouteEntry to a route map.
// Only adds attributes that are present (valid handle) — missing attributes are omitted.
func enrichRouteMapFromEntry(routeMap map[string]any, entry *storage.RouteEntry) {
	if entry.StaleLevel > storage.StaleLevelFresh {
		routeMap["stale"] = true
		routeMap["stale-level"] = entry.StaleLevel
	}
	if entry.HasNextHop() {
		if data, err := pool.NextHop.Get(entry.NextHop); err == nil {
			routeMap["next-hop"] = formatNextHop(data)
		}
	}
	if entry.HasOrigin() {
		if data, err := pool.Origin.Get(entry.Origin); err == nil {
			if origin := formatOrigin(data); origin != "" {
				routeMap["origin"] = origin
			}
		}
	}
	if entry.HasASPath() {
		if data, err := pool.ASPath.Get(entry.ASPath); err == nil {
			if asPath := formatASPath(data); asPath != nil {
				routeMap["as-path"] = asPath
			}
		}
	}
	if entry.HasMED() {
		if data, err := pool.MED.Get(entry.MED); err == nil {
			if v, ok := formatUint32Attr(data); ok {
				routeMap["med"] = v
			}
		}
	}
	if entry.HasLocalPref() {
		if data, err := pool.LocalPref.Get(entry.LocalPref); err == nil {
			if v, ok := formatUint32Attr(data); ok {
				routeMap["local-preference"] = v
			}
		}
	}
	if entry.HasCommunities() {
		if data, err := pool.Communities.Get(entry.Communities); err == nil {
			if communities := formatCommunities(data); communities != nil {
				routeMap["community"] = communities
			}
		}
	}
}

// enrichRouteMapFromRoute adds path attributes from a Route (Adj-RIB-Out) to a route map.
// Only non-empty/non-nil attributes are added.
func enrichRouteMapFromRoute(routeMap map[string]any, rt *Route) {
	if rt.Origin != "" {
		routeMap["origin"] = rt.Origin
	}
	if len(rt.ASPath) > 0 {
		routeMap["as-path"] = rt.ASPath
	}
	if rt.MED != nil {
		routeMap["med"] = *rt.MED
	}
	if rt.LocalPreference != nil {
		routeMap["local-preference"] = *rt.LocalPreference
	}
	if len(rt.Communities) > 0 {
		routeMap["community"] = rt.Communities
	}
	if len(rt.LargeCommunities) > 0 {
		routeMap["large-community"] = rt.LargeCommunities
	}
	if len(rt.ExtendedCommunities) > 0 {
		routeMap["extended-community"] = rt.ExtendedCommunities
	}
}

// originNames maps ORIGIN values to RFC 4271 names.
var originNames = map[byte]string{
	0: "igp",
	1: "egp",
	2: "incomplete",
}

// formatOrigin converts raw ORIGIN pool bytes to RFC 4271 name.
// ORIGIN is 1 byte: 0=IGP, 1=EGP, 2=INCOMPLETE.
func formatOrigin(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	if name, ok := originNames[data[0]]; ok {
		return name
	}
	return fmt.Sprintf("unknown(%d)", data[0])
}

// formatASPath converts raw AS_PATH pool bytes to a flat ASN slice.
// RFC 4271 Section 4.3b: segments are [type(1)][count(1)][ASN(4)*count].
// AS_SEQUENCE (type 2) and AS_SET (type 1) are both flattened.
func formatASPath(data []byte) []uint32 {
	if len(data) == 0 {
		return nil
	}
	var result []uint32
	offset := 0
	for offset+2 <= len(data) {
		// segType := data[offset] — not needed for flat list
		count := int(data[offset+1])
		offset += 2
		for range count {
			if offset+4 > len(data) {
				return nil // truncated data — don't return partial results
			}
			asn := uint32(data[offset])<<24 | uint32(data[offset+1])<<16 |
				uint32(data[offset+2])<<8 | uint32(data[offset+3])
			result = append(result, asn)
			offset += 4
		}
	}
	return result
}

// formatUint32Attr converts 4 big-endian bytes to uint32.
// Used for MED (type 4) and LOCAL_PREF (type 5).
func formatUint32Attr(data []byte) (uint32, bool) {
	if len(data) < 4 {
		return 0, false
	}
	v := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
	return v, true
}

// formatCommunities converts raw COMMUNITIES pool bytes to "high:low" strings.
// RFC 1997: each community is 4 bytes — 2-byte high : 2-byte low.
func formatCommunities(data []byte) []string {
	if len(data) == 0 || len(data)%4 != 0 {
		return nil
	}
	result := make([]string, 0, len(data)/4)
	for i := 0; i+4 <= len(data); i += 4 {
		high := uint16(data[i])<<8 | uint16(data[i+1])
		low := uint16(data[i+2])<<8 | uint16(data[i+3])
		result = append(result, fmt.Sprintf("%d:%d", high, low))
	}
	return result
}
