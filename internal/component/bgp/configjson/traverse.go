// Design: docs/architecture/config/syntax.md -- BGP config JSON traversal helpers

// Package configjson provides shared helpers for traversing BGP config JSON
// delivered to plugins at Stage 2 (configure callback). Handles both standalone
// peers (bgp.peer) and grouped peers (bgp.group.<name>.peer).
package configjson

import (
	"encoding/json"
	"net/netip"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// DynamicPeerIP is the group-level placeholder that ze-bgp-conf.yang allows in
// connection > remote > ip to mean "accept sessions from any address in the
// group's range". It is never a peer address: reactor/config.go rejects it at
// peer level, and a dynamically created peer carries its real address.
//
// It is also the one marker of a dynamic group, matching config.isDynamicGroup
// (internal/component/bgp/config/resolve.go). Both readers must agree on which
// groups are dynamic, or the reactor would build peers for a group the plugin
// traversal delivered no config for.
const DynamicPeerIP = "dynamic"

// PeerOrigin says where a visit's config came from, so a caller can store it
// under a key a runtime lookup can reach.
//
// Template is the field that matters. A dynamic group's members do not exist in
// the operator's config document -- they are created from the group's template
// when a connection arrives (reactor.tryCreateDynamicPeer) -- so no traversal of
// that document can ever yield them. What it CAN yield is the template, and the
// one identity the template shares with the peers built from it is the group's
// NAME: reactor.buildDynamicPeerSettings writes it to PeerSettings.GroupName, and
// every filterapi.PeerFilterInfo carries it.
type PeerOrigin struct {
	// Group is the enclosing group's name, empty for a standalone peer.
	Group string
	// Template reports that this visit IS a dynamic group's template rather than
	// a configured peer. peerMap is nil for such a visit, which peerMap alone
	// cannot say: a configured peer with no fields also has a nil map.
	Template bool
}

// PeerVisitor is called for each peer found in the config JSON, and once for each
// dynamic group's template.
// name is the peer's config-map key (the peer NAME), or the group's name on a
// template visit.
// peerMap is the peer's config (nil when the peer entry has no fields, and always
// nil on a template visit).
// groupMap is the enclosing group's config (nil for standalone peers).
// origin says whether the visit is a configured peer or a group's template.
type PeerVisitor func(name string, peerMap, groupMap map[string]any, origin PeerOrigin)

// PeerConfigKey identifies the configuration a peer resolves to at runtime.
//
// It is a typed pair rather than a bare string because the two namespaces can
// collide: config.ResolveBGPTree collects every peer name into one uniqueness map
// and refuses a duplicate, but a group name only goes through validateGroupName and
// is never compared against it. `bgp { peer ix {...} group ix {...} }` is therefore
// accepted, and a bare key would let a group's template answer a lookup meant for
// the peer. Template separates the two namespaces by construction.
type PeerConfigKey struct {
	// ID is a peer's remote IP (or its name when the name IS its address), or a
	// dynamic group's name when Template is set.
	ID string
	// Template reports that ID names a dynamic group rather than a peer.
	Template bool
}

// GroupKey returns the key a dynamic group's template config is stored under.
func GroupKey(group string) PeerConfigKey {
	return PeerConfigKey{ID: group, Template: true}
}

// PeerKey returns the key a configured peer's config is stored under, and whether
// the peer has a key any runtime reader can produce.
//
// The key is the peer's remote IP, because that is what every runtime reader holds:
// the filter chains pass filterapi.PeerFilterInfo.Address. When the peer states no
// remote IP anywhere its config-map key is used instead, but only when that name
// parses as an address -- operators very commonly name a peer by its own address,
// in which case the name IS the key readers produce. A name that is not an address
// can never be looked up, so ok is false rather than a key nothing finds.
//
// The "dynamic" placeholder is refused for the same reason: it is non-empty, so it
// would sail past an emptiness check and be stored under a literal key no reader
// can produce.
func PeerKey(name string, peerMap, groupMap map[string]any) (PeerConfigKey, bool) {
	ip := PeerRemoteIP(peerMap, groupMap)
	if ip == DynamicPeerIP {
		return PeerConfigKey{}, false
	}
	if ip != "" {
		return PeerConfigKey{ID: ip}, true
	}
	if _, err := netip.ParseAddr(name); err != nil {
		return PeerConfigKey{}, false
	}
	return PeerConfigKey{ID: name}, true
}

// KeyFor returns the key one visit's config is stored under. It is PeerKey for a
// configured peer and GroupKey for a dynamic group's template.
func KeyFor(name string, peerMap, groupMap map[string]any, origin PeerOrigin) (PeerConfigKey, bool) {
	if origin.Template {
		return GroupKey(origin.Group), true
	}
	return PeerKey(name, peerMap, groupMap)
}

// CapabilityGroupKey returns the peer-selector a capability declared for a dynamic
// group's template is published under.
//
// The capability path cannot use PeerConfigKey: rpc.CapabilityDecl.Peers is a
// []string that crosses the plugin process boundary, so the two namespaces have to
// share one string space. The ":" separator is what keeps them apart, and it holds
// by construction rather than by convention: naming.ValidateNodeName accepts only
// alphanumerics, "_", "-" and ".", so neither a peer name nor a group name can ever
// contain one.
//
// Without the separator a peer named "ix" and a group named "ix" would publish the
// same selector for the same capability code, and plugin.AddPluginCapabilities
// would refuse the whole config as a capability conflict -- a config that loads
// today.
func CapabilityGroupKey(group string) string {
	var tb textbuf.Buffer
	return tb.Str(GroupKeyPrefix).Str(group).String()
}

// GroupKeyPrefix is what CapabilityGroupKey puts in front of a group's name. It
// is exported so a reader that must tell a group's key from a peer's -- to name
// the right object in an error, say -- reads the prefix back rather than
// spelling it a second time.
const GroupKeyPrefix = "group:"

// CapabilitySelector returns the CapabilityDecl.Peers entry for one visit: the
// peer's own name for a configured peer, and the group selector for a dynamic
// group's template.
func CapabilitySelector(name string, origin PeerOrigin) string {
	if origin.Template {
		return CapabilityGroupKey(origin.Group)
	}
	return name
}

// LookupPeerConfig resolves a peer's config from an index keyed by KeyFor.
//
// The order is the config's own precedence: what a peer states beats what its group
// states. addr and name identify a configured peer; group is consulted last and
// answers for a peer created from a dynamic group's template, whose own address and
// name appear nowhere in the config document.
//
// It reports whether a config was found. A caller MUST branch on that rather than
// on the zero value: for every plugin here the zero value reads as "this peer has
// nothing configured", which is the permissive answer, and a miss that cannot be
// told from a deliberate absence is the trap ai/rules/evidence.md names.
func LookupPeerConfig[T any](m map[PeerConfigKey]T, addr, name, group string) (T, bool) {
	if addr != "" {
		if v, ok := m[PeerConfigKey{ID: addr}]; ok {
			return v, true
		}
	}
	if name != "" {
		if v, ok := m[PeerConfigKey{ID: name}]; ok {
			return v, true
		}
	}
	if group != "" {
		if v, ok := m[GroupKey(group)]; ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

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

// IsDynamicGroup reports whether a group's config map is a dynamic group: one
// whose connection > remote > ip is the "dynamic" placeholder, so its members are
// created from its template when a connection arrives rather than being named in
// the config.
//
// This is the same test config.isDynamicGroup applies when it builds the reactor's
// DynamicGroupTemplate, and the two must not diverge.
func IsDynamicGroup(groupMap map[string]any) bool {
	if groupMap == nil {
		return false
	}
	conn, ok := groupMap["connection"].(map[string]any)
	if !ok {
		return false
	}
	remote, ok := conn["remote"].(map[string]any)
	if !ok {
		return false
	}
	ip, _ := remote["ip"].(string)
	return ip == DynamicPeerIP
}

// ForEachPeer iterates over every peer a BGP config subtree states, and over every
// dynamic group's template.
//
// Visits standalone peers (bgpTree["peer"]), grouped peers
// (bgpTree["group"]["<name>"]["peer"]), and, once per dynamic group, the group
// itself with a nil peer map and origin.Template set.
//
// The template visit is what reaches a dynamic group's members. They are not in the
// config document and cannot be: reactor.tryCreateDynamicPeer builds them from the
// template when a connection arrives, long after config.BuildPluginConfigSections
// serialized the document a plugin reads. Without this visit an operator who states
// a role, an RPKI action or a blackhole block on a listen-range group -- the
// canonical IXP route-server shape -- got no error and no enforcement.
//
// A dynamic group that ALSO lists static peers yields both: each static peer, and
// the template. The two are separate configurations and both are in force.
func ForEachPeer(bgpTree map[string]any, visit PeerVisitor) {
	// Standalone peers.
	if peersMap, ok := bgpTree["peer"].(map[string]any); ok {
		for name, peerData := range peersMap {
			peerMap, _ := peerData.(map[string]any)
			visit(name, peerMap, nil, PeerOrigin{})
		}
	}

	// Grouped peers, and each dynamic group's template.
	groupsMap, ok := bgpTree["group"].(map[string]any)
	if !ok {
		return
	}
	for groupName, groupData := range groupsMap {
		groupMap, ok := groupData.(map[string]any)
		if !ok {
			continue
		}

		// The template first, so a caller that logs its visits reads the group
		// before the peers it encloses.
		if IsDynamicGroup(groupMap) {
			visit(groupName, nil, groupMap, PeerOrigin{Group: groupName, Template: true})
		}

		peersMap, ok := groupMap["peer"].(map[string]any)
		if !ok {
			continue
		}
		for name, peerData := range peersMap {
			peerMap, _ := peerData.(map[string]any)
			visit(name, peerMap, groupMap, PeerOrigin{Group: groupName})
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
