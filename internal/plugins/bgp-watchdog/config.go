// Design: docs/architecture/rib-transition.md — watchdog plugin extraction
// Related: pool.go — route pool management
// Related: command.go — text command builder

package bgp_watchdog

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// parseConfig extracts per-peer watchdog route pools from a BGP config JSON tree.
// The JSON is delivered via OnConfigure and has the structure produced by
// ResolveBGPTree + Tree.ToMap():
//
//	{"peer": {"10.0.0.1": {"update": {"default": {"attribute": {...}, "nlri": {...}, "watchdog": {...}}}}}}
//
// Returns a map from peer address to PoolSet containing watchdog route definitions.
// Update blocks without a watchdog container are skipped.
func parseConfig(jsonData string) (map[string]*PoolSet, error) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(jsonData), &tree); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// Navigate the "bgp" wrapper — ExtractConfigSubtree wraps data as {"bgp": {...}}
	bgpTree, ok := getMap(tree, "bgp")
	if !ok {
		return make(map[string]*PoolSet), nil
	}

	peerMap, ok := getMap(bgpTree, "peer")
	if !ok {
		return make(map[string]*PoolSet), nil
	}

	result := make(map[string]*PoolSet)

	for peerAddr, peerData := range peerMap {
		peerTree, ok := peerData.(map[string]any)
		if !ok {
			continue
		}

		updateMap, ok := getMap(peerTree, "update")
		if !ok {
			continue
		}

		var pools *PoolSet

		for _, updateData := range updateMap {
			updateTree, ok := updateData.(map[string]any)
			if !ok {
				continue
			}

			// Only process update blocks with a watchdog container
			wdMap, ok := getMap(updateTree, "watchdog")
			if !ok {
				continue
			}

			wdName := getString(wdMap, "name")
			if wdName == "" {
				continue
			}

			_, wdWithdraw := wdMap["withdraw"]

			// Parse attributes
			attrMap, _ := getMap(updateTree, "attribute")
			b := buildCmdFromAttrs(attrMap)

			// Parse NLRI entries
			nlriMap, ok := getMap(updateTree, "nlri")
			if !ok {
				continue
			}

			entries := parseNLRIEntries(nlriMap, b, wdWithdraw)
			if len(entries) == 0 {
				continue
			}

			if pools == nil {
				pools = NewPoolSet()
			}

			for _, entry := range entries {
				if err := pools.AddRoute(wdName, entry); err != nil {
					logger().Warn("duplicate watchdog route", "peer", peerAddr, "pool", wdName, "key", entry.Key, "error", err)
				}
			}
		}

		if pools != nil {
			result[peerAddr] = pools
		}
	}

	return result, nil
}

// buildCmdFromAttrs creates a cmdBuilder from an attribute map.
func buildCmdFromAttrs(attrMap map[string]any) cmdBuilder {
	var b cmdBuilder

	b.origin = getStringAny(attrMap, "origin")
	b.nextHop = getStringAny(attrMap, "next-hop")

	if v := getStringAny(attrMap, "local-preference"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			b.localPref = uint32(n)
		}
	}

	if v := getStringAny(attrMap, "med"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			b.med = uint32(n)
		}
	}

	if v := getStringAny(attrMap, "as-path"); v != "" {
		b.asPath = parseASPath(v)
	}

	if v := getStringAny(attrMap, "community"); v != "" {
		b.communities = splitCommaOrSpace(v)
	}

	if v := getStringAny(attrMap, "large-community"); v != "" {
		b.largeCommunities = splitCommaOrSpace(v)
	}

	if v := getStringAny(attrMap, "extended-community"); v != "" {
		b.extCommunities = splitCommaOrSpace(v)
	}

	return b
}

// parseNLRIEntries parses NLRI map entries into PoolEntry objects.
// Each NLRI entry has family as key and a map with "content" field.
func parseNLRIEntries(nlriMap map[string]any, base cmdBuilder, initiallyWithdrawn bool) []*PoolEntry {
	var entries []*PoolEntry

	for familyKey, nlriData := range nlriMap {
		// Strip #N suffix from duplicate family keys
		family := stripKeySuffix(familyKey)

		nlriTree, ok := nlriData.(map[string]any)
		if !ok {
			continue
		}

		content := getStringAny(nlriTree, "content")
		if content == "" {
			continue
		}

		// Parse content: "add PREFIX1 PREFIX2 ..." or "del PREFIX1 ..."
		parts := strings.Fields(content)
		if len(parts) < 2 {
			continue
		}

		op := parts[0]
		if op != "add" {
			continue // Only announce routes for watchdog
		}

		// Parse inline rd/label modifiers before prefixes
		remaining := parts[1:]
		var rd string
		var labels []uint32

		for len(remaining) >= 2 {
			switch remaining[0] {
			case "rd":
				rd = remaining[1]
				remaining = remaining[2:]
				continue
			case "label":
				if n, err := strconv.ParseUint(remaining[1], 10, 32); err == nil {
					labels = append(labels, uint32(n))
				}
				remaining = remaining[2:]
				continue
			}
			break
		}

		// Remaining items are prefixes
		for _, prefix := range remaining {
			b := base
			b.family = family
			b.prefix = normalizePrefix(prefix)
			b.rd = rd
			b.labels = labels

			entry := NewPoolEntry(b.routeKey(), b.announce(), b.withdraw())
			entry.initiallyAnnounced = !initiallyWithdrawn

			entries = append(entries, entry)
		}
	}

	return entries
}

// parseASPath parses space or comma-separated AS numbers.
func parseASPath(s string) []uint32 {
	parts := splitCommaOrSpace(s)
	var asns []uint32
	for _, p := range parts {
		if n, err := strconv.ParseUint(p, 10, 32); err == nil {
			asns = append(asns, uint32(n))
		}
	}
	return asns
}

// splitCommaOrSpace splits a string by commas or spaces.
func splitCommaOrSpace(s string) []string {
	s = strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(s)
	return parts
}

// stripKeySuffix removes the #N suffix from list keys (e.g., "ipv4/unicast#1" → "ipv4/unicast").
func stripKeySuffix(key string) string {
	if idx := strings.LastIndex(key, "#"); idx > 0 {
		suffix := key[idx+1:]
		if _, err := strconv.Atoi(suffix); err == nil {
			return key[:idx]
		}
	}
	return key
}

// normalizePrefix ensures a prefix has CIDR notation.
// Bare IPs like "77.77.77.77" become "77.77.77.77/32" (IPv4) or "/128" (IPv6).
// Already-valid prefixes pass through unchanged.
func normalizePrefix(s string) string {
	if strings.Contains(s, "/") {
		return s
	}
	ip, err := netip.ParseAddr(s)
	if err != nil {
		return s // Let downstream parser report the error
	}
	bits := 32
	if ip.Is6() {
		bits = 128
	}
	return netip.PrefixFrom(ip, bits).String()
}

// getMap extracts a map[string]any from a parent map by key.
func getMap(m map[string]any, key string) (map[string]any, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	sub, ok := v.(map[string]any)
	return sub, ok
}

// getString extracts a string value from a map.
func getString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// getStringAny extracts a string from a map, handling nil maps gracefully.
func getStringAny(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	return getString(m, key)
}
