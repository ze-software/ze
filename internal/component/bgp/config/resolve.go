// Design: docs/architecture/config/syntax.md — BGP peer-group resolution and inheritance

package bgpconfig

import (
	"fmt"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

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

	peerMap := make(map[string]any)
	peerNames := make(map[string]string) // name -> addr (for uniqueness check)

	// Resolve grouped peers: bgp { group <name> { peer <ip> { } } }
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
			addr := peerEntry.Key
			peerTree := peerEntry.Value

			resolved := make(map[string]any)

			// Layer 2: Apply group defaults.
			deepMergeMaps(resolved, groupFields)

			// Layer 3: Apply peer's own values (highest precedence).
			deepMergeMaps(resolved, peerTree.ToMap())

			// Inject group name so PeersFromTree can populate PeerSettings.GroupName.
			resolved["group-name"] = groupName

			if err := validateAndTrackPeerName(resolved, groupName, addr, peerNames); err != nil {
				return nil, err
			}

			if _, exists := peerMap[addr]; exists {
				return nil, fmt.Errorf("bgp.group %s: duplicate peer IP %s (already defined in another group or as standalone)", groupName, addr)
			}
			peerMap[addr] = resolved
		}
	}

	// Resolve standalone peers: bgp { peer <ip> { } }
	for _, peerEntry := range bgp.GetListOrdered("peer") {
		addr := peerEntry.Key
		peerTree := peerEntry.Value

		resolved := peerTree.ToMap()

		if err := validateAndTrackPeerName(resolved, "", addr, peerNames); err != nil {
			return nil, err
		}

		if _, exists := peerMap[addr]; exists {
			return nil, fmt.Errorf("bgp.peer %s: duplicate peer IP (already defined in a group or as standalone)", addr)
		}
		peerMap[addr] = resolved
	}

	if len(peerMap) > 0 {
		result["peer"] = peerMap
	}

	return result, nil
}

// validateAndTrackPeerName validates and registers a peer name for uniqueness.
// groupName may be empty for standalone peers.
func validateAndTrackPeerName(resolved map[string]any, groupName, addr string, peerNames map[string]string) error {
	name, ok := resolved["name"].(string)
	if !ok || name == "" {
		return nil
	}
	if err := validatePeerName(name); err != nil {
		if groupName != "" {
			return fmt.Errorf("bgp.group %s peer %s: %w", groupName, addr, err)
		}
		return fmt.Errorf("bgp.peer %s: %w", addr, err)
	}
	if existingAddr, exists := peerNames[name]; exists {
		if groupName != "" {
			return fmt.Errorf("bgp.group %s peer %s: duplicate peer name %q (already used by %s)", groupName, addr, name, existingAddr)
		}
		return fmt.Errorf("bgp.peer %s: duplicate peer name %q (already used by %s)", addr, name, existingAddr)
	}
	peerNames[name] = addr
	return nil
}

// isASCIILetterOrDigit returns true if the character is an ASCII letter or digit.
func isASCIILetterOrDigit(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}

// isValidPeerNameChar returns true if the character is allowed in a peer name.
// Allowed: ASCII letters, ASCII digits, hyphens, underscores.
// Non-ASCII letters (unicode.IsLetter accepts CJK, accents, etc.) are rejected
// to avoid display issues and CLI ambiguity.
func isValidPeerNameChar(ch rune) bool {
	return isASCIILetterOrDigit(ch) || ch == '-' || ch == '_'
}

// maxPeerNameLen is the maximum length for peer names.
// Limits JSON response size and prevents DoS via long names.
const maxPeerNameLen = 255

// reservedPeerNames contains names that collide with "bgp peer <subcommand>"
// keywords. A peer named "list" would cause dispatch ambiguity: the dispatcher
// cannot tell if "bgp peer list detail" means "show detail for peer named list"
// or a syntax error. Reject these at config validation time.
var reservedPeerNames = map[string]bool{
	"list": true, "detail": true, "add": true, "remove": true,
	"pause": true, "resume": true, "save": true, "teardown": true,
	"capabilities": true, "statistics": true, "update": true,
	"raw": true, "refresh": true, "borr": true, "eorr": true,
	"clear": true, "plugin": true,
}

// validatePeerName checks that a peer name is valid for use as a CLI selector.
// Names must be ASCII alphanumeric with hyphens and underscores only.
// Names must not parse as IP addresses or look like glob patterns.
// Names must start with a letter or digit (not punctuation-only).
// Names must not collide with "bgp peer" subcommand keywords.
func validatePeerName(name string) error {
	if name == "*" {
		return fmt.Errorf("invalid peer name %q: reserved wildcard", name)
	}

	if len(name) > maxPeerNameLen {
		return fmt.Errorf("invalid peer name %q: exceeds maximum length %d", name, maxPeerNameLen)
	}

	// Reject names that collide with CLI subcommand keywords.
	if reservedPeerNames[name] {
		return fmt.Errorf("invalid peer name %q: conflicts with \"bgp peer\" subcommand", name)
	}

	// Reject names containing invalid characters.
	// Only ASCII letters, digits, hyphens, and underscores are allowed.
	for _, ch := range name {
		if !isValidPeerNameChar(ch) {
			return fmt.Errorf("invalid peer name %q: only ASCII alphanumeric, hyphens, and underscores allowed", name)
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
	for _, ch := range name {
		if !isValidPeerNameChar(ch) {
			return fmt.Errorf("invalid group name %q: only ASCII alphanumeric, hyphens, and underscores allowed", name)
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

// deepMergeMaps recursively merges src into dst.
// For leaf values (non-map), src overwrites dst.
// For map values, keys are merged recursively so both sides contribute.
func deepMergeMaps(dst, src map[string]any) {
	for k, srcVal := range src {
		srcMap, srcIsMap := srcVal.(map[string]any)
		if !srcIsMap {
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
		deepMergeMaps(dstMap, srcMap)
	}
}
