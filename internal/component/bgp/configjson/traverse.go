// Design: docs/architecture/config/syntax.md -- BGP config JSON traversal helpers

// Package configjson provides shared helpers for traversing BGP config JSON
// delivered to plugins at Stage 2 (configure callback). Handles both standalone
// peers (bgp.peer) and grouped peers (bgp.group.<name>.peer).
package configjson

import "encoding/json"

// PeerVisitor is called for each peer found in the config JSON.
// peerAddr is the peer's IP address string.
// peerMap is the peer's config (may be nil if the peer entry has no fields).
// groupMap is the enclosing group's config (nil for standalone peers).
type PeerVisitor func(peerAddr string, peerMap, groupMap map[string]any)

// ParseBGPSubtree extracts the bgp subtree from a config JSON string.
// Handles both {"bgp": {...}} and bare {...} formats.
func ParseBGPSubtree(jsonStr string) (map[string]any, bool) {
	var root map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &root); err != nil {
		return nil, false
	}
	if bgp, ok := root["bgp"].(map[string]any); ok {
		return bgp, true
	}
	return root, true
}

// ForEachPeer iterates over all peers in a BGP config subtree.
// Visits standalone peers (bgpTree["peer"]) and grouped peers
// (bgpTree["group"]["<name>"]["peer"]).
// For grouped peers, groupMap is the group's config map.
// For standalone peers, groupMap is nil.
func ForEachPeer(bgpTree map[string]any, visit PeerVisitor) {
	// Standalone peers.
	if peersMap, ok := bgpTree["peer"].(map[string]any); ok {
		for addr, peerData := range peersMap {
			peerMap, _ := peerData.(map[string]any)
			visit(addr, peerMap, nil)
		}
	}

	// Grouped peers.
	groupsMap, ok := bgpTree["group"].(map[string]any)
	if !ok {
		return
	}
	for _, groupData := range groupsMap {
		groupMap, ok := groupData.(map[string]any)
		if !ok {
			continue
		}
		peersMap, ok := groupMap["peer"].(map[string]any)
		if !ok {
			continue
		}
		for addr, peerData := range peersMap {
			peerMap, _ := peerData.(map[string]any)
			visit(addr, peerMap, groupMap)
		}
	}
}

// PeerRemoteIP returns a peer's remote IP address from its delivered config map,
// reading connection > remote > ip. peerMap takes precedence over the enclosing
// groupMap (a peer's own remote IP always wins); returns "" when neither has it.
//
// This is the single correct reader for a peer's remote IP. The config delivered
// to plugins keys peers by NAME (Tree.ToMap emits a keyed YANG list keyed by the
// entry name), with the address nested at connection > remote > ip. Plugins that
// identify peers by IP at runtime (RPKI, watchdog, role) MUST key on this value,
// not on the ForEachPeer map key (which is the peer name).
func PeerRemoteIP(peerMap, groupMap map[string]any) string {
	for _, m := range []map[string]any{peerMap, groupMap} {
		if m == nil {
			continue
		}
		conn, ok := m["connection"].(map[string]any)
		if !ok {
			continue
		}
		remote, ok := conn["remote"].(map[string]any)
		if !ok {
			continue
		}
		if ip, ok := remote["ip"].(string); ok && ip != "" {
			return ip
		}
	}
	return ""
}

// GetCapability returns the capability map for a peer or group config map.
// Capabilities live under session > capability in the YANG peer config structure.
// Returns nil if no capability container exists.
func GetCapability(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	session, ok := m["session"].(map[string]any)
	if !ok {
		return nil
	}
	capMap, _ := session["capability"].(map[string]any)
	return capMap
}
