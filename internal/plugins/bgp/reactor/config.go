package reactor

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// Config tree string constants (shared with reactor.go to satisfy goconst).
const (
	valTrue   = "true"
	valEnable = "enable"
)

// parsePeerFromTree parses a peer's settings from a flattened config tree.
// The tree is the peer subtree from a map[string]any config,
// already resolved with template inheritance.
// addr is the peer's IP address (the list key in the config).
// localAS and routerID are global defaults from the bgp block.
func parsePeerFromTree(addr string, tree map[string]any, localAS, routerID uint32) (*PeerSettings, error) {
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid peer address %q: %w", addr, err)
	}

	// Peer AS (required).
	peerAS, ok := mapUint32(tree, "peer-as")
	if !ok {
		return nil, fmt.Errorf("peer %s: missing required peer-as", addr)
	}

	// Local AS (peer-level overrides global; at least one must be set).
	peerLocalAS := localAS
	if v, ok := mapUint32(tree, "local-as"); ok {
		peerLocalAS = v
	}
	if peerLocalAS == 0 {
		return nil, fmt.Errorf("peer %s: missing required local-as (neither global nor peer-level)", addr)
	}

	// Router ID (peer-level overrides global).
	peerRouterID := routerID
	if v, ok := mapString(tree, "router-id"); ok {
		rid, err := netip.ParseAddr(v)
		if err != nil {
			return nil, fmt.Errorf("peer %s: invalid router-id: %w", addr, err)
		}
		peerRouterID = ipToUint32(rid)
	}

	ps := NewPeerSettings(ip, peerLocalAS, peerAS, peerRouterID)

	// Hold time (default 90s from NewPeerSettings).
	if v, ok := mapUint32(tree, "hold-time"); ok {
		ht := v
		// RFC 4271 Section 4.2: Hold Time MUST be either zero or at least three seconds.
		if ht >= 1 && ht <= 2 {
			return nil, fmt.Errorf("peer %s: invalid hold-time %d: RFC 4271 requires 0 or >= 3 seconds", addr, ht)
		}
		ps.HoldTime = time.Duration(ht) * time.Second
	}

	// Connection mode (both/passive/active).
	if v, ok := mapString(tree, "connection"); ok {
		mode, err := ParseConnectionMode(v)
		if err != nil {
			return nil, fmt.Errorf("peer %s: %w", addr, err)
		}
		ps.Connection = mode
	}

	// Per-peer listen port (overrides global tcp.port for this peer).
	if v, ok := mapUint32(tree, "port"); ok {
		if v < 1 || v > 65535 {
			return nil, fmt.Errorf("peer %s: port must be 1-65535, got %d", addr, v)
		}
		ps.Port = uint16(v)
	}

	// Group updates (default true from NewPeerSettings).
	if v, ok := mapBool(tree, "group-updates"); ok {
		ps.GroupUpdates = v
	}

	// Local address (required).
	v, hasLocalAddr := mapString(tree, "local-address")
	if !hasLocalAddr {
		return nil, fmt.Errorf("peer %s: local-address is required (use IP address or \"auto\")", addr)
	}
	if v != "auto" {
		la, err := netip.ParseAddr(v)
		if err != nil {
			return nil, fmt.Errorf("peer %s: invalid local-address: %w", addr, err)
		}
		ps.LocalAddress = la
	}

	// RFC 2545 Section 3: IPv6 link-local address for MP_REACH next-hop.
	if v, ok := mapString(tree, "link-local"); ok {
		ll, err := netip.ParseAddr(v)
		if err != nil {
			return nil, fmt.Errorf("peer %s: invalid link-local: %w", addr, err)
		}
		ps.LinkLocal = ll
	}

	// Parse families.
	if err := parseFamiliesFromTree(tree, ps); err != nil {
		return nil, fmt.Errorf("peer %s: %w", addr, err)
	}

	// Parse capabilities.
	parseCapabilitiesFromTree(tree, ps)

	// Parse process bindings.
	parseProcessBindingsFromTree(tree, ps)

	return ps, nil
}

// PeersFromTree parses all peer settings from a bgp subtree (map[string]any).
// The tree should be the "bgp" block from a resolved config, with templates
// already applied and flattened via Tree.ToMap().
// Returns the list of PeerSettings and an error if any peer fails to parse.
//
// Global local-as and router-id are optional defaults. Each peer can override
// them or provide its own. If neither global nor peer-level local-as is set,
// parsePeerFromTree returns an error for that peer.
func PeersFromTree(bgpTree map[string]any) ([]*PeerSettings, error) {
	// Extract global defaults (both optional — peers can provide their own).
	localAS, _ := mapUint32(bgpTree, "local-as")

	var routerID uint32
	if v, ok := mapString(bgpTree, "router-id"); ok {
		rid, err := netip.ParseAddr(v)
		if err != nil {
			return nil, fmt.Errorf("bgp: invalid router-id: %w", err)
		}
		routerID = ipToUint32(rid)
	}

	// Parse peers.
	peerMap, ok := mapMap(bgpTree, "peer")
	if !ok {
		return nil, nil
	}

	var peers []*PeerSettings
	for addr, val := range peerMap {
		peerTree, ok := val.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("peer %s: invalid config (expected map)", addr)
		}
		ps, err := parsePeerFromTree(addr, peerTree, localAS, routerID)
		if err != nil {
			return nil, err
		}
		peers = append(peers, ps)
	}

	return peers, nil
}

// parseFamiliesFromTree parses address family configuration from the tree.
// Builds Multiprotocol capabilities and tracks required/ignore families.
func parseFamiliesFromTree(tree map[string]any, ps *PeerSettings) error {
	familyMap, ok := mapMap(tree, "family")
	if !ok {
		return nil
	}

	// Sort family keys for deterministic capability ordering in OPEN messages.
	// Go maps have random iteration order; without sorting, the OPEN message
	// would encode multiprotocol capabilities in non-deterministic order.
	familyKeys := make([]string, 0, len(familyMap))
	for key := range familyMap {
		familyKeys = append(familyKeys, key)
	}
	slices.Sort(familyKeys)

	for _, key := range familyKeys {
		val := familyMap[key]

		// Handle ignore-mismatch option.
		if key == "ignore-mismatch" {
			if vs, ok := val.(string); ok {
				ps.IgnoreFamilyMismatch = vs == valTrue || vs == valEnable
			}
			continue
		}

		// Parse family: key is "afi/safi", val is mode string.
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid family key %q: expected afi/safi format", key)
		}

		modeStr := ""
		if vs, ok := val.(string); ok {
			modeStr = vs
		}
		mode := parseFamilyMode(modeStr)

		// Skip disabled families.
		if mode == familyModeDisable {
			continue
		}

		// Parse to capability types.
		family, ok := nlri.ParseFamily(key)
		if !ok {
			return fmt.Errorf("unknown address family %q", key)
		}

		ps.Capabilities = append(ps.Capabilities, &capability.Multiprotocol{
			AFI:  family.AFI,
			SAFI: family.SAFI,
		})

		if mode == familyModeRequire {
			ps.RequiredFamilies = append(ps.RequiredFamilies, family)
		}
		if mode == familyModeIgnore {
			ps.IgnoreFamilies = append(ps.IgnoreFamilies, family)
		}
	}

	return nil
}

// familyMode represents the negotiation mode for an address family.
type familyMode int

const (
	familyModeEnable familyMode = iota
	familyModeDisable
	familyModeRequire
	familyModeIgnore
)

// parseFamilyMode parses a family mode string.
// Empty string or "true"/"enable" means enabled.
// Unrecognized values default to enable (lenient parsing).
func parseFamilyMode(s string) familyMode {
	switch strings.ToLower(s) {
	case "", valTrue, valEnable:
		return familyModeEnable
	case "false", "disable":
		return familyModeDisable
	case "require":
		return familyModeRequire
	case "ignore":
		return familyModeIgnore
	}
	return familyModeEnable
}

// parseCapabilitiesFromTree parses capability configuration from the tree.
func parseCapabilitiesFromTree(tree map[string]any, ps *PeerSettings) {
	capMap, ok := mapMap(tree, "capability")
	if !ok {
		// ASN4 is enabled by default (RFC 6793).
		return
	}

	// ASN4 — enabled by default, disable if explicitly set to false.
	asn4 := true
	if v, ok := mapString(capMap, "asn4"); ok {
		asn4 = v == valTrue
	}
	ps.DisableASN4 = !asn4

	// RFC 8654: Extended Message Support.
	if v := flexString(capMap, "extended-message"); v == valTrue || v == valEnable {
		ps.Capabilities = append(ps.Capabilities, &capability.ExtendedMessage{})
	}

	// Software version.
	if v := flexString(capMap, "software-version"); v == valTrue || v == valEnable {
		ps.Capabilities = append(ps.Capabilities, &capability.SoftwareVersion{
			Version: "ExaBGP/5.0.0-0+test",
		})
	}

	// RFC 2918/7313: Route Refresh.
	if v := flexString(capMap, "route-refresh"); v == valTrue || v == valEnable {
		ps.Capabilities = append(ps.Capabilities, &capability.RouteRefresh{})
		ps.Capabilities = append(ps.Capabilities, &capability.EnhancedRouteRefresh{})
	}

	// Graceful restart.
	if grMap, ok := mapMap(capMap, "graceful-restart"); ok {
		if ps.RawCapabilityConfig == nil {
			ps.RawCapabilityConfig = make(map[string]map[string]string)
		}
		ps.RawCapabilityConfig["graceful-restart"] = make(map[string]string)
		if v, ok := mapString(grMap, "restart-time"); ok {
			ps.RawCapabilityConfig["graceful-restart"]["restart-time"] = v
		}
	}

	// RFC 8950: Extended Next Hop.
	if nhMap, ok := mapMap(capMap, "nexthop"); ok {
		parseExtendedNextHopFromTree(nhMap, ps)
	}

	// ADD-PATH — global and per-family.
	parseAddPathFromTree(capMap, tree, ps)

	// Hostname — populate RawCapabilityConfig for plugin delivery.
	if hnMap, ok := mapMap(capMap, "hostname"); ok {
		if ps.RawCapabilityConfig == nil {
			ps.RawCapabilityConfig = make(map[string]map[string]string)
		}
		if ps.RawCapabilityConfig["hostname"] == nil {
			ps.RawCapabilityConfig["hostname"] = make(map[string]string)
		}
		if v, ok := mapString(hnMap, "host"); ok {
			ps.RawCapabilityConfig["hostname"]["host"] = v
		}
		if v, ok := mapString(hnMap, "domain"); ok {
			ps.RawCapabilityConfig["hostname"]["domain"] = v
		}
	}

	// Also check top-level host-name/domain-name (plugin YANG augmented fields).
	if v, ok := mapString(tree, "host-name"); ok {
		if ps.RawCapabilityConfig == nil {
			ps.RawCapabilityConfig = make(map[string]map[string]string)
		}
		if ps.RawCapabilityConfig["hostname"] == nil {
			ps.RawCapabilityConfig["hostname"] = make(map[string]string)
		}
		ps.RawCapabilityConfig["hostname"]["host"] = v
	}
	if v, ok := mapString(tree, "domain-name"); ok {
		if ps.RawCapabilityConfig == nil {
			ps.RawCapabilityConfig = make(map[string]map[string]string)
		}
		if ps.RawCapabilityConfig["hostname"] == nil {
			ps.RawCapabilityConfig["hostname"] = make(map[string]string)
		}
		ps.RawCapabilityConfig["hostname"]["domain"] = v
	}

	// Capability config JSON for plugin delivery.
	ps.CapabilityConfigJSON = mapToJSON(capMap)
}

// parseExtendedNextHopFromTree parses RFC 8950 extended next-hop families.
func parseExtendedNextHopFromTree(nhMap map[string]any, ps *PeerSettings) {
	afiMap := map[string]uint16{"ipv4": 1, "ipv6": 2}
	safiMap := map[string]uint8{
		"unicast": 1, "multicast": 2, "mpls-vpn": 128, "mpls-label": 4,
	}

	var families []capability.ExtendedNextHopFamily

	for _, nlriAFIName := range []string{"ipv4", "ipv6"} {
		nlriAFI := afiMap[nlriAFIName]
		for _, safiName := range []string{"unicast", "multicast", "mpls-vpn", "mpls-label"} {
			nlriSAFI := safiMap[safiName]
			for _, nhAFIName := range []string{"ipv4", "ipv6"} {
				nhAFI := afiMap[nhAFIName]
				key := nlriAFIName + "/" + safiName + " " + nhAFIName
				if _, ok := nhMap[key]; ok {
					families = append(families, capability.ExtendedNextHopFamily{
						NLRIAFI:    capability.AFI(nlriAFI),
						NLRISAFI:   capability.SAFI(nlriSAFI),
						NextHopAFI: capability.AFI(nhAFI),
					})
				}
			}
		}
	}

	if len(families) > 0 {
		ps.Capabilities = append(ps.Capabilities, &capability.ExtendedNextHop{
			Families: families,
		})
	}
}

// parseAddPathFromTree parses ADD-PATH capability from the capability map and peer tree.
// RFC 7911: Supports both global add-path mode and per-family overrides.
func parseAddPathFromTree(capMap, peerTree map[string]any, ps *PeerSettings) {
	var globalSend, globalReceive bool

	// Check capability block for add-path (flex: string or map).
	if val, ok := capMap["add-path"]; ok {
		switch v := val.(type) {
		case string:
			// Global mode: "send", "receive", "send/receive", "receive/send".
			switch v {
			case "send/receive", "receive/send":
				globalSend = true
				globalReceive = true
			case "send":
				globalSend = true
			case "receive":
				globalReceive = true
			}
		case map[string]any:
			// Block mode: add-path { send true; receive true; }
			if s, ok := mapString(v, "send"); ok {
				globalSend = s == valTrue
			}
			if r, ok := mapString(v, "receive"); ok {
				globalReceive = r == valTrue
			}
		}
	}

	// Per-family add-path from peer tree.
	var perFamily []capability.AddPathFamily
	if addPathMap, ok := mapMap(peerTree, "add-path"); ok {
		for key, val := range addPathMap {
			// Key format: "ipv4/unicast send" stored as Freeform value.
			parts := strings.Fields(key)
			if len(parts) < 2 {
				// Try key as family, val as mode.
				if vs, ok := val.(string); ok {
					parts = []string{key, vs}
				}
			}
			if len(parts) >= 2 {
				family, ok := nlri.ParseFamily(parts[0])
				if !ok {
					continue
				}
				var mode capability.AddPathMode
				switch parts[1] {
				case "send":
					mode = capability.AddPathSend
				case "receive":
					mode = capability.AddPathReceive
				case "send/receive", "receive/send":
					mode = capability.AddPathBoth
				}
				if mode != capability.AddPathNone {
					perFamily = append(perFamily, capability.AddPathFamily{
						AFI:  family.AFI,
						SAFI: family.SAFI,
						Mode: mode,
					})
				}
			}
		}
	}

	if !globalSend && !globalReceive && len(perFamily) == 0 {
		return
	}

	addPath := &capability.AddPath{
		Families: make([]capability.AddPathFamily, 0),
	}

	// Apply global mode to all configured families.
	if globalSend || globalReceive {
		var globalMode capability.AddPathMode
		switch {
		case globalSend && globalReceive:
			globalMode = capability.AddPathBoth
		case globalSend:
			globalMode = capability.AddPathSend
		case globalReceive:
			globalMode = capability.AddPathReceive
		}
		for _, cap := range ps.Capabilities {
			if mp, ok := cap.(*capability.Multiprotocol); ok {
				addPath.Families = append(addPath.Families, capability.AddPathFamily{
					AFI:  mp.AFI,
					SAFI: mp.SAFI,
					Mode: globalMode,
				})
			}
		}
	}

	// Override with per-family settings.
	addPath.Families = append(addPath.Families, perFamily...)

	if len(addPath.Families) > 0 {
		ps.Capabilities = append(ps.Capabilities, addPath)
	}
}

// parseProcessBindingsFromTree parses process bindings from the peer tree.
func parseProcessBindingsFromTree(tree map[string]any, ps *PeerSettings) {
	procMap, ok := mapMap(tree, "process")
	if !ok {
		return
	}

	for name, val := range procMap {
		pMap, ok := val.(map[string]any)
		if !ok {
			continue
		}

		binding := ProcessBinding{PluginName: name}

		// Content settings.
		if contentMap, ok := mapMap(pMap, "content"); ok {
			if v, ok := mapString(contentMap, "encoding"); ok {
				binding.Encoding = strings.ToLower(v)
			}
			if v, ok := mapString(contentMap, "format"); ok {
				binding.Format = strings.ToLower(v)
			}
		}

		// Receive settings.
		if recvMap, ok := mapMap(pMap, "receive"); ok {
			parseReceiveFlags(recvMap, &binding)
		}

		// Send settings.
		if sendMap, ok := mapMap(pMap, "send"); ok {
			parseSendFlags(sendMap, &binding)
		}

		ps.ProcessBindings = append(ps.ProcessBindings, binding)
	}
}

// parseReceiveFlags sets receive flags on a ProcessBinding from a map.
func parseReceiveFlags(m map[string]any, b *ProcessBinding) {
	// Check for "all" shorthand.
	if _, ok := m["all"]; ok {
		b.ReceiveUpdate = true
		b.ReceiveOpen = true
		b.ReceiveNotification = true
		b.ReceiveKeepalive = true
		b.ReceiveRefresh = true
		b.ReceiveState = true
		b.ReceiveSent = true
		b.ReceiveNegotiated = true
		return
	}

	_, b.ReceiveUpdate = m["update"]
	_, b.ReceiveOpen = m["open"]
	_, b.ReceiveNotification = m["notification"]
	_, b.ReceiveKeepalive = m["keepalive"]
	_, b.ReceiveRefresh = m["refresh"]
	_, b.ReceiveState = m["state"]
	_, b.ReceiveSent = m["sent"]
	_, b.ReceiveNegotiated = m["negotiated"]
}

// parseSendFlags sets send flags on a ProcessBinding from a map.
func parseSendFlags(m map[string]any, b *ProcessBinding) {
	// Check for "all" shorthand.
	if _, ok := m["all"]; ok {
		b.SendUpdate = true
		b.SendRefresh = true
		return
	}

	_, b.SendUpdate = m["update"]
	_, b.SendRefresh = m["refresh"]
}

// --- Map navigation helpers ---

// mapString extracts a string value from a map.
func mapString(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// mapUint32 extracts a uint32 value from a map (stored as string).
func mapUint32(m map[string]any, key string) (uint32, bool) {
	s, ok := mapString(m, key)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}

// mapBool extracts a boolean value from a map (stored as string "true"/"false").
func mapBool(m map[string]any, key string) (bool, bool) {
	s, ok := mapString(m, key)
	if !ok {
		return false, false
	}
	return s == valTrue, true
}

// mapMap extracts a nested map from a map.
func mapMap(m map[string]any, key string) (map[string]any, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	sub, ok := v.(map[string]any)
	return sub, ok
}

// flexString extracts a string from a flex field (can be string or map).
// Returns the string value, or empty string if it's a map or not present.
func flexString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// ipToUint32 converts an IPv4 address to uint32.
func ipToUint32(ip netip.Addr) uint32 {
	if !ip.Is4() {
		return 0
	}
	b := ip.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// mapToJSON converts a map to a JSON string.
// Returns empty string if the map is nil or marshaling fails.
func mapToJSON(m map[string]any) string {
	if len(m) == 0 {
		return ""
	}
	data, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(data)
}
