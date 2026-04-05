// Design: docs/architecture/config/syntax.md — BGP peer-group resolution and inheritance

package bgpconfig

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// cumulativePaths lists config paths where leaf-list values should accumulate across
// config inheritance levels (bgp -> group -> peer) instead of the most-specific
// level replacing less-specific ones. Derived from ze:cumulative YANG extension.
var cumulativePaths = map[string]bool{
	"filter/ingress/community/tag":   true,
	"filter/ingress/community/strip": true,
	"filter/egress/community/tag":    true,
	"filter/egress/community/strip":  true,
}

// ResolveBGPTree resolves peer-group inheritance and returns the bgp block as map[string]any.
// Resolution applies 3 layers per peer (in precedence order):
//  1. BGP-level globals (local-as, router-id from the bgp block)
//  2. Group-level defaults (fields set on the group, shared by all member peers)
//  3. The peer's own values (highest precedence)
//
// Each layer deep-merges into the previous, so containers like capability are merged
// at the key level, not replaced wholesale.
func ResolveBGPTree(tree *config.Tree) (map[string]any, error) {
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil, fmt.Errorf("missing required bgp { } block")
	}

	// Build result map with global bgp values.
	result := bgp.ToMap()

	// Remove raw group and peer lists -- we'll rebuild as a flat peer map.
	delete(result, "group")
	delete(result, "peer")

	// BGP-level defaults (Layer 1: lowest precedence for every peer).
	// After removing group/peer lists, result contains all BGP-level fields.
	// These serve as defaults for every peer -- groups and peer-level values
	// override them via deepMergeMaps. No field whitelist needed: unknown
	// fields are harmlessly ignored by PeersFromTree downstream.
	bgpDefaults := deepCopyMap(result)

	peerMap := make(map[string]any)
	peerNames := make(map[string]string) // name -> addr (for uniqueness check)

	// Resolve grouped peers: bgp { group <name> { peer <name> { } } }
	for _, groupEntry := range bgp.GetListOrdered("group") {
		groupName := groupEntry.Key
		groupTree := groupEntry.Value

		// Validate group name using the same rules as peer names.
		if err := validateGroupName(groupName); err != nil {
			return nil, err
		}

		// Extract group-level fields (everything except the nested peer list).
		groupFields := groupTree.ToMap()
		delete(groupFields, "peer") // Peer list is not a group-level field.

		// Resolve each peer in this group.
		for _, peerEntry := range groupTree.GetListOrdered("peer") {
			peerName := peerEntry.Key
			peerTree := peerEntry.Value

			// Validate peer name (the list key).
			if err := validatePeerName(peerName); err != nil {
				return nil, fmt.Errorf("bgp/group %s peer %s: %w", groupName, peerName, err)
			}
			if existingAddr, exists := peerNames[peerName]; exists {
				return nil, fmt.Errorf("bgp/group %s peer %s: duplicate peer name (already used by %s)", groupName, peerName, existingAddr)
			}
			peerNames[peerName] = peerName

			resolved := make(map[string]any)

			// Layer 1: BGP-level defaults (lowest precedence).
			deepMergeMaps(resolved, deepCopyMap(bgpDefaults), cumulativePaths)

			// Layer 2: Apply group defaults.
			deepMergeMaps(resolved, groupFields, cumulativePaths)

			// Layer 3: Apply peer's own values (highest precedence).
			deepMergeMaps(resolved, peerTree.ToMap(), cumulativePaths)

			// Inject group name so PeersFromTree can populate PeerSettings.GroupName.
			resolved["group-name"] = groupName

			if _, exists := peerMap[peerName]; exists {
				return nil, fmt.Errorf("bgp/group %s: duplicate peer name %s (already defined in another group or as standalone)", groupName, peerName)
			}
			peerMap[peerName] = resolved
		}
	}

	// Resolve standalone peers: bgp { peer <name> { } }
	for _, peerEntry := range bgp.GetListOrdered("peer") {
		peerName := peerEntry.Key
		peerTree := peerEntry.Value

		resolved := make(map[string]any)

		// Layer 1: BGP-level defaults (lowest precedence).
		deepMergeMaps(resolved, deepCopyMap(bgpDefaults), cumulativePaths)

		// Layer 3: Apply peer's own values (highest precedence).
		deepMergeMaps(resolved, peerTree.ToMap(), cumulativePaths)

		// Validate peer name (the list key).
		if err := validatePeerName(peerName); err != nil {
			return nil, fmt.Errorf("bgp/peer %s: %w", peerName, err)
		}
		if existingAddr, exists := peerNames[peerName]; exists {
			return nil, fmt.Errorf("bgp/peer %s: duplicate peer name (already used by %s)", peerName, existingAddr)
		}
		peerNames[peerName] = peerName

		if _, exists := peerMap[peerName]; exists {
			return nil, fmt.Errorf("bgp/peer %s: duplicate peer name (already defined in a group or as standalone)", peerName)
		}
		peerMap[peerName] = resolved
	}

	// Check for duplicate remote > ip across all peers.
	if err := checkDuplicateRemoteIPs(peerMap); err != nil {
		return nil, err
	}

	if len(peerMap) > 0 {
		result["peer"] = peerMap
	}

	return result, nil
}

// CheckRequiredFields validates that all ze:required fields on the peer list schema
// have non-empty values in every resolved peer map. Called after ResolveBGPTree by
// callers that need config validation (cmd_validate, PeersFromConfigTree).
func CheckRequiredFields(schema *config.Schema, bgpTree map[string]any) error {
	peerMap, ok := bgpTree["peer"].(map[string]any)
	if !ok {
		return nil // No peers to validate.
	}

	// Look up the peer list schema to get Required fields.
	bgpNode := schema.Get("bgp")
	if bgpNode == nil {
		return nil
	}
	bgpContainer, ok := bgpNode.(*config.ContainerNode)
	if !ok {
		return nil
	}
	peerNode := bgpContainer.Get("peer")
	if peerNode == nil {
		return nil
	}
	peerListNode, ok := peerNode.(*config.ListNode)
	if !ok {
		return nil
	}
	if len(peerListNode.Required) == 0 {
		return nil
	}

	// Sort peer names for deterministic error reporting.
	peerNames := make([]string, 0, len(peerMap))
	for name := range peerMap {
		peerNames = append(peerNames, name)
	}
	sort.Strings(peerNames)

	for _, peerName := range peerNames {
		peer, ok := peerMap[peerName].(map[string]any)
		if !ok {
			continue
		}
		for _, reqPath := range peerListNode.Required {
			if !hasNestedValue(peer, reqPath) {
				configLogger().Warn("incomplete peer definition",
					"peer", peerName,
					"missing", strings.Join(reqPath, "/"))
			}
		}
	}
	return nil
}

// hasNestedValue checks if a map has a non-empty value at the given path.
func hasNestedValue(m map[string]any, path []string) bool {
	current := m
	for i, key := range path {
		val, exists := current[key]
		if !exists {
			return false
		}
		if i == len(path)-1 {
			s, ok := val.(string)
			return !ok || s != ""
		}
		next, ok := val.(map[string]any)
		if !ok {
			return false
		}
		current = next
	}
	return false
}

// checkDuplicateRemoteIPs checks that no two peers share the same connection > remote > ip value.
// Peers without a connection > remote > ip are skipped (they will fail mandatory field validation elsewhere).
func checkDuplicateRemoteIPs(peerMap map[string]any) error {
	seen := make(map[string]string) // remote IP -> first peer name
	for peerName, v := range peerMap {
		peer, ok := v.(map[string]any)
		if !ok {
			continue
		}
		connMap, ok := peer["connection"].(map[string]any)
		if !ok {
			continue
		}
		remoteMap, ok := connMap["remote"].(map[string]any)
		if !ok {
			continue
		}
		ip, ok := remoteMap["ip"].(string)
		if !ok || ip == "" {
			continue
		}
		if firstPeer, exists := seen[ip]; exists {
			return fmt.Errorf("duplicate remote IP %s in peer %s (already used by peer %s)", ip, peerName, firstPeer)
		}
		seen[ip] = peerName
	}
	return nil
}

// Note: validateAndTrackPeerName was removed. Peer name validation is now done
// directly in the resolve loops since the name IS the list key, not a field in resolved.

// isASCIILetterOrDigit returns true if the character is an ASCII letter or digit.
func isASCIILetterOrDigit(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}

// isValidPeerNameFirstChar returns true if the character is allowed as the first
// character of a peer name. Only ASCII letters, digits, and underscores.
// Dots and hyphens are not allowed as the first character to avoid ambiguity
// with IP addresses (dot) and CLI flags (hyphen).
func isValidPeerNameFirstChar(ch rune) bool {
	return isASCIILetterOrDigit(ch) || ch == '_'
}

// isValidPeerNameChar returns true if the character is allowed in a peer name
// at the second position or later. Allowed: ASCII letters, digits, hyphens,
// underscores, and dots. Non-ASCII letters (unicode.IsLetter accepts CJK,
// accents, etc.) are rejected to avoid display issues and CLI ambiguity.
func isValidPeerNameChar(ch rune) bool {
	return isASCIILetterOrDigit(ch) || ch == '-' || ch == '_' || ch == '.'
}

// maxPeerNameLen is the maximum length for peer names.
// Limits JSON response size and prevents DoS via long names.
const maxPeerNameLen = 255

// reservedPeerNames contains names that collide with "peer <subcommand>"
// keywords. A peer named "list" would cause dispatch ambiguity: the dispatcher
// cannot tell if "peer list detail" means "show detail for peer named list"
// or a syntax error. Reject these at config validation time.
var reservedPeerNames = map[string]bool{
	"list": true, "detail": true,
	"pause": true, "resume": true, "flush": true, "teardown": true,
	"capabilities": true, "statistics": true, "update": true,
	"raw": true, "refresh": true, "borr": true, "eorr": true,
	"clear": true, "plugin": true,
}

// validatePeerName checks that a peer name is valid for use as a CLI selector.
// First character must be ASCII alphanumeric or underscore.
// Subsequent characters may also include hyphens and dots.
// Names must not parse as IP addresses or look like glob patterns.
// Names must contain at least one letter or digit.
// Names must not collide with "peer" subcommand keywords.
func validatePeerName(name string) error {
	if name == "*" {
		return fmt.Errorf("invalid peer name %q: reserved wildcard", name)
	}

	if len(name) > maxPeerNameLen {
		return fmt.Errorf("invalid peer name %q: exceeds maximum length %d", name, maxPeerNameLen)
	}

	// Reject names that collide with CLI subcommand keywords.
	if reservedPeerNames[name] {
		return fmt.Errorf("invalid peer name %q: conflicts with \"peer\" subcommand", name)
	}

	// Reject names containing invalid characters.
	// First character: ASCII letters, digits, underscores only.
	// Subsequent characters: also allow hyphens and dots.
	for i, ch := range name {
		if i == 0 {
			if !isValidPeerNameFirstChar(ch) {
				return fmt.Errorf("invalid peer name %q: first character must be alphanumeric or underscore", name)
			}
		} else if !isValidPeerNameChar(ch) {
			return fmt.Errorf("invalid peer name %q: only alphanumeric, hyphens, underscores, and dots allowed", name)
		}
	}

	// Reject names that are only punctuation (hyphens/underscores).
	// Such names are confusing as CLI selectors and provide no useful identification.
	hasAlphanumeric := false
	for _, ch := range name {
		if isASCIILetterOrDigit(ch) {
			hasAlphanumeric = true
			break
		}
	}
	if !hasAlphanumeric {
		return fmt.Errorf("invalid peer name %q: must contain at least one letter or digit", name)
	}

	// Reject names that parse as valid IP addresses.
	if _, err := netip.ParseAddr(name); err == nil {
		return fmt.Errorf("invalid peer name %q: must not be a valid IP address", name)
	}

	return nil
}

// validateGroupName checks that a group name is valid.
// Group names follow the same character and length rules as peer names.
func validateGroupName(name string) error {
	if name == "" {
		return fmt.Errorf("invalid group name: must not be empty")
	}
	if len(name) > maxPeerNameLen {
		return fmt.Errorf("invalid group name %q: exceeds maximum length %d", name, maxPeerNameLen)
	}
	for i, ch := range name {
		if i == 0 {
			if !isValidPeerNameFirstChar(ch) {
				return fmt.Errorf("invalid group name %q: first character must be alphanumeric or underscore", name)
			}
		} else if !isValidPeerNameChar(ch) {
			return fmt.Errorf("invalid group name %q: only alphanumeric, hyphens, underscores, and dots allowed", name)
		}
	}
	hasAlphanumeric := false
	for _, ch := range name {
		if isASCIILetterOrDigit(ch) {
			hasAlphanumeric = true
			break
		}
	}
	if !hasAlphanumeric {
		return fmt.Errorf("invalid group name %q: must contain at least one letter or digit", name)
	}
	return nil
}

// deepCopyMap returns a deep copy of a map, recursively copying nested maps.
// Non-map values are shared (strings, ints are immutable so this is safe).
func deepCopyMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		if m, ok := v.(map[string]any); ok {
			dst[k] = deepCopyMap(m)
		} else {
			dst[k] = v
		}
	}
	return dst
}

// deepMergeMaps recursively merges src into dst.
// For leaf values (non-map), src overwrites dst.
// For map values, keys are merged recursively so both sides contribute.
// If cumulative is non-nil, keys whose dot-path is in the set accumulate
// slice values (append) instead of replacing them. Used for ze:cumulative
// leaf-lists like filter tag/strip that must gather values from all config levels.
func deepMergeMaps(dst, src map[string]any, cumulative map[string]bool) {
	deepMergeAt(dst, src, cumulative, "")
}

// toAnySlice converts []any, []string, or a bare string to []any.
// Returns nil if the value is not a slice or string type.
func toAnySlice(v any) []any {
	switch s := v.(type) {
	case []any:
		return s
	case []string:
		result := make([]any, len(s))
		for i, val := range s {
			result[i] = val
		}
		return result
	case string:
		return []any{s}
	}
	return nil
}

// deepMergeAt is the recursive worker for deepMergeMaps, tracking the dot-path
// prefix for cumulative key lookups.
func deepMergeAt(dst, src map[string]any, cumulative map[string]bool, prefix string) {
	for k, srcVal := range src {
		path := k
		if prefix != "" {
			path = config.AppendPath(prefix, k)
		}

		srcMap, srcIsMap := srcVal.(map[string]any)
		if !srcIsMap {
			// Cumulative: append slices instead of replacing.
			// ToMap() produces []string for multiValues, JSON round-trip produces []any.
			// Handle both types uniformly by converting to []any for accumulation.
			if cumulative[path] {
				srcSlice := toAnySlice(srcVal)
				dstSlice := toAnySlice(dst[k])
				if srcSlice != nil && dstSlice != nil {
					dst[k] = append(dstSlice, srcSlice...)
					continue
				}
			}
			dst[k] = srcVal
			continue
		}
		dstMap, dstIsMap := dst[k].(map[string]any)
		if !dstIsMap {
			// dst doesn't have a map here -- copy src map.
			dst[k] = srcVal
			continue
		}
		// Both are maps -- recurse.
		deepMergeAt(dstMap, srcMap, cumulative, path)
	}
}
