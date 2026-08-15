// Design: docs/architecture/config/syntax.md — BGP peer-group resolution and inheritance
// Related: variables.go — config variable substitution for dynamic peers

package bgpconfig

import (
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/naming"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errMissingRequiredBgpBlock   = errors.New("missing required bgp { } block")
	errInvalidGroupNameMustNotBe = errors.New("invalid group name: must not be empty")
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
		return nil, errMissingRequiredBgpBlock
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

	var dynamicGroups []DynamicGroupTemplate

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

		// Detect dynamic group: connection > remote > ip == "dynamic".
		if isDynamicGroup(groupFields) {
			tmpl, err := resolveDynamicGroup(groupName, groupFields, bgpDefaults)
			if err != nil {
				return nil, err
			}
			dynamicGroups = append(dynamicGroups, tmpl)
		}

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

	if len(dynamicGroups) > 0 {
		result["dynamic-groups"] = dynamicGroups
	}

	return result, nil
}

// DynamicGroupTemplate holds the resolved config for a dynamic peer group.
// Exported by ResolveBGPTree for the reactor to build PeerSettings at connection time.
type DynamicGroupTemplate struct {
	GroupName string
	Ranges    []netip.Prefix
	MaxPeers  uint32
	RSClient  bool
	Template  map[string]any
}

// isDynamicGroup returns true when the group's connection > remote > ip is "dynamic".
func isDynamicGroup(groupFields map[string]any) bool {
	connMap, ok := groupFields["connection"].(map[string]any)
	if !ok {
		return false
	}
	remoteMap, ok := connMap["remote"].(map[string]any)
	if !ok {
		return false
	}
	ip, ok := remoteMap["ip"].(string)
	return ok && ip == "dynamic"
}

// resolveDynamicGroup validates and resolves a dynamic group into a template.
func resolveDynamicGroup(groupName string, groupFields, bgpDefaults map[string]any) (DynamicGroupTemplate, error) {
	connMap, ok := groupFields["connection"].(map[string]any)
	if !ok {
		return DynamicGroupTemplate{}, fmt.Errorf("bgp/group %s: missing connection block", groupName)
	}
	remoteMap, ok := connMap["remote"].(map[string]any)
	if !ok {
		return DynamicGroupTemplate{}, fmt.Errorf("bgp/group %s: missing connection/remote block", groupName)
	}

	// Validate: range is required when ip is dynamic.
	rangeVal, hasRange := remoteMap["range"]
	if !hasRange {
		return DynamicGroupTemplate{}, fmt.Errorf("bgp/group %s: ip dynamic requires at least one range", groupName)
	}

	// Validate: connect false is required for dynamic groups.
	// YANG default for connect is true, so absent means true.
	connectVal, hasConnect := remoteMap["connect"]
	if !hasConnect {
		return DynamicGroupTemplate{}, fmt.Errorf("bgp/group %s: dynamic group requires explicit connect false", groupName)
	}
	if s, ok := connectVal.(string); ok && s != "false" {
		return DynamicGroupTemplate{}, fmt.Errorf("bgp/group %s: dynamic group requires connect false", groupName)
	}

	// Parse ranges.
	var ranges []netip.Prefix
	switch v := rangeVal.(type) {
	case string:
		p, err := netip.ParsePrefix(v)
		if err != nil {
			return DynamicGroupTemplate{}, fmt.Errorf("bgp/group %s: invalid range %q: %w", groupName, v, err)
		}
		ranges = append(ranges, p)
	case []string:
		for _, s := range v {
			p, err := netip.ParsePrefix(s)
			if err != nil {
				return DynamicGroupTemplate{}, fmt.Errorf("bgp/group %s: invalid range %q: %w", groupName, s, err)
			}
			ranges = append(ranges, p)
		}
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			p, err := netip.ParsePrefix(s)
			if err != nil {
				return DynamicGroupTemplate{}, fmt.Errorf("bgp/group %s: invalid range %q: %w", groupName, s, err)
			}
			ranges = append(ranges, p)
		}
	}

	if len(ranges) == 0 {
		return DynamicGroupTemplate{}, fmt.Errorf("bgp/group %s: ip dynamic requires at least one valid range", groupName)
	}

	// Parse max-peers (default 1000 from YANG).
	var maxPeers uint32 = 1000
	if mp, ok := remoteMap["max-peers"].(string); ok {
		if n, err := strconv.ParseUint(mp, 10, 32); err == nil {
			maxPeers = uint32(n)
		}
	}

	// Check rs-client on the session block.
	var rsClient bool
	if sessionMap, ok := groupFields["session"].(map[string]any); ok {
		if v, ok := sessionMap["rs-client"].(string); ok && v == "true" {
			rsClient = true
		}
	}

	// Build resolved template (merge bgp defaults + group fields).
	resolved := make(map[string]any)
	deepMergeMaps(resolved, deepCopyMap(bgpDefaults), cumulativePaths)
	deepMergeMaps(resolved, groupFields, cumulativePaths)
	resolved["group-name"] = groupName
	if err := validateDynamicGroupTTL(groupName, resolved); err != nil {
		return DynamicGroupTemplate{}, err
	}

	return DynamicGroupTemplate{
		GroupName: groupName,
		Ranges:    ranges,
		MaxPeers:  maxPeers,
		RSClient:  rsClient,
		Template:  resolved,
	}, nil
}

func validateDynamicGroupTTL(groupName string, resolved map[string]any) error {
	conn, ok := resolved["connection"].(map[string]any)
	if !ok {
		return nil
	}
	ttl, ok := conn["ttl"].(map[string]any)
	if !ok {
		return nil
	}
	hasMax, err := ttlValueSet(ttl, "max")
	if err != nil {
		return fmt.Errorf("bgp/group %s: %w", groupName, err)
	}
	hasSet, err := ttlValueSet(ttl, "set")
	if err != nil {
		return fmt.Errorf("bgp/group %s: %w", groupName, err)
	}
	hasMin, err := ttlValueSet(ttl, "min")
	if err != nil {
		return fmt.Errorf("bgp/group %s: %w", groupName, err)
	}
	if hasMax && (hasSet || hasMin) {
		return fmt.Errorf("bgp/group %s: ttl max cannot be combined with ttl set or ttl min", groupName)
	}
	return nil
}

func ttlValueSet(ttl map[string]any, key string) (bool, error) {
	raw, ok := ttl[key]
	if !ok {
		return false, nil
	}
	s, ok := raw.(string)
	if !ok {
		return false, nil
	}
	n, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		return false, fmt.Errorf("ttl %s must be in range 0..255", key)
	}
	return n != 0, nil
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
					"missing", textbuf.Join(reqPath, "/"))
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

// maxPeerNameLen is the maximum length for peer names.
const maxPeerNameLen = 255

// reservedPeerNames contains names that collide with "peer <subcommand>"
// keywords. A peer named "list" would cause dispatch ambiguity: the dispatcher
// cannot tell if "peer list detail" means "show detail for peer named list"
// or a syntax error. Reject these at config validation time.
var reservedPeerNames = map[string]bool{
	"list": true, "detail": true, "capabilities": true, "statistics": true,
	"history": true, "rib": true,
	"pause": true, "resume": true, "flush": true, "teardown": true,
	"update": true, "raw": true, "refresh": true, "borr": true, "eorr": true,
	"clear": true, "plugin": true, "prefix": true,
}

func validatePeerName(name string) error {
	if name == "*" {
		return fmt.Errorf("invalid peer name %q: reserved wildcard", name)
	}
	if err := naming.ValidateNodeName("peer", name, maxPeerNameLen); err != nil {
		return err
	}
	if !format.IsJSONSafe(name) {
		return fmt.Errorf("invalid peer name %q: contains characters requiring JSON escaping", name)
	}
	if reservedPeerNames[name] {
		return fmt.Errorf("invalid peer name %q: conflicts with \"peer\" subcommand", name)
	}
	if _, err := netip.ParseAddr(name); err == nil {
		return fmt.Errorf("invalid peer name %q: must not be a valid IP address", name)
	}
	if strings.HasPrefix(name, "dyn-") {
		return fmt.Errorf("invalid peer name %q: dyn- prefix is reserved for dynamic peers", name)
	}
	return nil
}

func validateGroupName(name string) error {
	if name == "" {
		return errInvalidGroupNameMustNotBe
	}
	if err := naming.ValidateNodeName("group", name, maxPeerNameLen); err != nil {
		return err
	}
	if !format.IsJSONSafe(name) {
		return fmt.Errorf("invalid group name %q: contains characters requiring JSON escaping", name)
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
					// A fresh slice, never an append in place: dst's slice can
					// share its backing array with a sibling's, and an append
					// with spare capacity overwrites what the sibling holds.
					merged := make([]any, 0, len(dstSlice)+len(srcSlice))
					merged = append(merged, dstSlice...)
					dst[k] = append(merged, srcSlice...)
					continue
				}
			}
			dst[k] = srcVal
			continue
		}
		dstMap, dstIsMap := dst[k].(map[string]any)
		if !dstIsMap {
			// dst holds no map here, so it takes a COPY of src's. Assigning
			// src's map would leave dst ALIASING it, and the next merge into
			// dst would write through into src. That is what let one group
			// member's own `attach process` block rewrite the group's block,
			// which every sibling then inherited: the group map reaches each
			// member as layer 2 (ResolveBGPTree), so it is shared by
			// construction (AC-6b, spec-fixit-peer-process-event-filter).
			dst[k] = deepCopyMap(srcMap)
			continue
		}
		// Both are maps -- recurse.
		deepMergeAt(dstMap, srcMap, cumulative, path)
	}
}
