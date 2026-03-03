// Design: docs/architecture/core-design.md — BGP reactor event loop

package reactor

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
)

// Config tree string constants (shared with reactor.go to satisfy goconst).
const (
	valTrue    = "true"
	valEnable  = "enable"
	valDisable = "disable"
	valRequire = "require"
	valRefuse  = "refuse"
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
			switch v := val.(type) {
			case string:
				ps.IgnoreFamilyMismatch = v == valTrue || v == valEnable
			case map[string]any:
				// List entry: mode may be set, or key-only means enable.
				if mode, ok := mapString(v, "mode"); ok {
					ps.IgnoreFamilyMismatch = mode == valTrue || mode == valEnable
				} else {
					ps.IgnoreFamilyMismatch = true
				}
			}
			continue
		}

		// Parse family: key is "afi/safi", val is mode string or list entry map.
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid family key %q: expected afi/safi format", key)
		}

		modeStr := ""
		switch v := val.(type) {
		case string:
			modeStr = v
		case map[string]any:
			// List entry: extract mode from map.
			if mode, ok := mapString(v, "mode"); ok {
				modeStr = mode
			}
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
	case "false", valDisable:
		return familyModeDisable
	case valRequire:
		return familyModeRequire
	case "ignore":
		return familyModeIgnore
	}
	return familyModeEnable
}

// capMode represents the negotiation mode for a non-family capability.
// Four modes: enable (advertise, no enforcement), disable (don't advertise),
// require (advertise + reject if peer lacks), refuse (don't advertise + reject if peer has).
type capMode int

const (
	capModeEnable capMode = iota
	capModeDisable
	capModeRequire
	capModeRefuse
)

// parseCapMode parses a capability mode string.
// Accepts enable/disable/require/refuse plus backwards-compat true/false.
// Empty string or unrecognized values default to enable (lenient parsing).
func parseCapMode(s string) capMode {
	switch strings.ToLower(s) {
	case "", valTrue, valEnable:
		return capModeEnable
	case "false", valDisable:
		return capModeDisable
	case valRequire:
		return capModeRequire
	case valRefuse:
		return capModeRefuse
	}
	return capModeEnable
}

// capModeAdvertise reports whether the mode means the capability should be advertised.
func (m capMode) advertise() bool { return m == capModeEnable || m == capModeRequire }

// applyCapMode records require/refuse enforcement for a capability code.
func applyCapMode(mode capMode, code capability.Code, ps *PeerSettings) {
	switch mode {
	case capModeRequire:
		ps.RequiredCapabilities = append(ps.RequiredCapabilities, code)
	case capModeRefuse:
		ps.RefusedCapabilities = append(ps.RefusedCapabilities, code)
	case capModeEnable, capModeDisable:
		// No enforcement action needed.
	}
}

// parseCapabilitiesFromTree parses capability configuration from the tree.
func parseCapabilitiesFromTree(tree map[string]any, ps *PeerSettings) {
	capMap, ok := mapMap(tree, "capability")
	if !ok {
		// ASN4 is enabled by default (RFC 6793).
		return
	}

	// ASN4 — enabled by default (RFC 6793), supports all four modes.
	asn4Mode := capModeEnable
	if v, ok := mapString(capMap, "asn4"); ok {
		asn4Mode = parseCapMode(v)
	}
	ps.DisableASN4 = !asn4Mode.advertise()
	applyCapMode(asn4Mode, capability.CodeASN4, ps)

	// RFC 8654: Extended Message Support (opt-in, absent = disabled).
	if v := flexString(capMap, "extended-message"); v != "" {
		extMsgMode := parseCapMode(v)
		if extMsgMode.advertise() {
			ps.Capabilities = append(ps.Capabilities, &capability.ExtendedMessage{})
		}
		applyCapMode(extMsgMode, capability.CodeExtendedMessage, ps)
	}

	// RFC 2918/7313: Route Refresh (opt-in, absent = disabled).
	// Enforcement targets basic route-refresh (code 2) only.
	// Enhanced route-refresh (code 70) is a separate capability not independently configurable.
	if v := flexString(capMap, "route-refresh"); v != "" {
		rrMode := parseCapMode(v)
		if rrMode.advertise() {
			ps.Capabilities = append(ps.Capabilities, &capability.RouteRefresh{}, &capability.EnhancedRouteRefresh{})
		}
		applyCapMode(rrMode, capability.CodeRouteRefresh, ps)
	}

	// Graceful restart — block capability with optional mode key.
	if grMap, ok := mapMap(capMap, "graceful-restart"); ok {
		grMode := capModeEnable
		if v, ok := mapString(grMap, "mode"); ok {
			grMode = parseCapMode(v)
		}
		applyCapMode(grMode, capability.CodeGracefulRestart, ps)

		if ps.RawCapabilityConfig == nil {
			ps.RawCapabilityConfig = make(map[string]map[string]string)
		}
		ps.RawCapabilityConfig["graceful-restart"] = make(map[string]string)
		if v, ok := mapString(grMap, "restart-time"); ok {
			ps.RawCapabilityConfig["graceful-restart"]["restart-time"] = v
		}
	}

	// RFC 8950: Extended Next Hop — mode is inline on each family line.
	// e.g., "ipv4/unicast ipv6 require;" — last token is a mode if it matches a mode keyword.
	if nhMap, ok := mapMap(capMap, "nexthop"); ok {
		nhMode := parseExtendedNextHopFromTree(nhMap, ps)
		applyCapMode(nhMode, capability.CodeExtendedNextHop, ps)
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

// extractNextHopEntry extracts nhAFI name and mode from a nexthop entry value.
// Handles both string format ("ipv6 require") and list entry map ({"nhafi": "ipv6", "mode": "require"}).
func extractNextHopEntry(rawVal any) (string, capMode) {
	if vs, ok := rawVal.(string); ok {
		tokens := strings.Fields(vs)
		if len(tokens) == 0 {
			return "", capModeEnable
		}
		mode := capModeEnable
		if len(tokens) > 1 {
			mode = parseCapMode(tokens[1])
		}
		return tokens[0], mode
	}
	if m, ok := rawVal.(map[string]any); ok {
		nh, nhOK := mapString(m, "nhafi")
		if !nhOK {
			return "", capModeEnable
		}
		mode := capModeEnable
		if modeStr, ok := mapString(m, "mode"); ok {
			mode = parseCapMode(modeStr)
		}
		return nh, mode
	}
	return "", capModeEnable
}

// extractAddPathEntry extracts family, direction, and mode from an add-path entry.
// Handles both old format (key="ipv4/unicast send require" or key="ipv4/unicast", val="send require")
// and new list format (key="ipv4/unicast", val=map{"direction":"send","mode":"require"}).
func extractAddPathEntry(key string, val any) (familyKey, direction, mode string) {
	// New list entry format: val is map[string]any.
	if m, ok := val.(map[string]any); ok {
		dir, _ := mapString(m, "direction")
		md, _ := mapString(m, "mode")
		return key, dir, md
	}

	// Old format: key may contain space-separated tokens, or val is a string.
	parts := strings.Fields(key)
	if len(parts) < 2 {
		if vs, ok := val.(string); ok {
			parts = append(parts, strings.Fields(vs)...)
		}
	}
	// Check for trailing mode token.
	if len(parts) >= 3 && isCapModeToken(parts[len(parts)-1]) {
		return parts[0], parts[1], parts[len(parts)-1]
	}
	if len(parts) >= 2 {
		return parts[0], parts[1], ""
	}
	return key, "", ""
}

// parseExtendedNextHopFromTree parses RFC 8950 extended next-hop families.
// Map key is the NLRI family (e.g., "ipv4/unicast"), value is "nhAFI [mode]":
//
//	"ipv4/unicast" → "ipv6"          (enable, default)
//	"ipv4/unicast" → "ipv6 require"  (require mode)
//
// Returns the most restrictive mode seen across all entries (require > refuse > enable).
func parseExtendedNextHopFromTree(nhMap map[string]any, ps *PeerSettings) capMode {
	afiMap := map[string]uint16{"ipv4": 1, "ipv6": 2}
	safiMap := map[string]uint8{
		"unicast": 1, "multicast": 2, "mpls-vpn": 128, "mpls-label": 4,
	}

	var families []capability.ExtendedNextHopFamily
	mode := capModeEnable

	for familyKey, rawVal := range nhMap {
		// Parse family key: "ipv4/unicast" → afi="ipv4", safi="unicast"
		parts := strings.SplitN(familyKey, "/", 2)
		if len(parts) != 2 {
			continue
		}
		nlriAFI, afiOK := afiMap[parts[0]]
		nlriSAFI, safiOK := safiMap[parts[1]]
		if !afiOK || !safiOK {
			continue
		}

		// Parse value: string "ipv6 [require]" or list entry map {nhafi, mode}.
		nhAFIName, entryMode := extractNextHopEntry(rawVal)
		if nhAFIName == "" {
			continue
		}

		nhAFI, ok := afiMap[nhAFIName]
		if !ok {
			continue
		}

		// Only include family if mode advertises (enable/require).
		if entryMode.advertise() {
			families = append(families, capability.ExtendedNextHopFamily{
				NLRIAFI:    capability.AFI(nlriAFI),
				NLRISAFI:   capability.SAFI(nlriSAFI),
				NextHopAFI: capability.AFI(nhAFI),
			})
		}
		// Precedence: require > refuse > enable/disable.
		if entryMode == capModeRequire {
			mode = capModeRequire
		} else if entryMode == capModeRefuse && mode != capModeRequire {
			mode = capModeRefuse
		}
	}

	if len(families) > 0 {
		ps.Capabilities = append(ps.Capabilities, &capability.ExtendedNextHop{
			Families: families,
		})
	}
	return mode
}

// capModeTokens is the single source of truth for capability mode keywords.
var capModeTokens = []string{valRequire, valRefuse, valEnable, valDisable}

// isCapModeToken reports whether s is a capability mode keyword.
func isCapModeToken(s string) bool {
	return slices.Contains(capModeTokens, strings.ToLower(s))
}

// parseAddPathFromTree parses ADD-PATH capability from the capability map and peer tree.
// RFC 7911: Supports both global add-path mode and per-family overrides.
// An optional trailing mode token (require/refuse/enable/disable) sets enforcement.
func parseAddPathFromTree(capMap, peerTree map[string]any, ps *PeerSettings) {
	var globalSend, globalReceive bool
	addPathMode := capModeEnable

	// Check capability block for add-path (flex: string or map).
	if val, ok := capMap["add-path"]; ok {
		switch v := val.(type) {
		case string:
			// Global mode: "send/receive [require]" — last token may be a mode.
			parts := strings.Fields(v)
			if len(parts) >= 2 && isCapModeToken(parts[len(parts)-1]) {
				addPathMode = parseCapMode(parts[len(parts)-1])
				parts = parts[:len(parts)-1]
			}
			dir := strings.Join(parts, "/")
			switch dir {
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
	perFamilyHasEnforcement := false // true when any per-family entry sets require/refuse
	if addPathMap, ok := mapMap(peerTree, "add-path"); ok {
		for key, val := range addPathMap {
			familyKey, dirStr, modeStr := extractAddPathEntry(key, val)

			// Apply enforcement mode.
			if modeStr != "" {
				entryMode := parseCapMode(modeStr)
				if entryMode == capModeRequire {
					addPathMode = capModeRequire
					perFamilyHasEnforcement = true
				} else if entryMode == capModeRefuse && addPathMode != capModeRequire {
					addPathMode = capModeRefuse
					perFamilyHasEnforcement = true
				}
			}

			if familyKey == "" || dirStr == "" {
				continue
			}
			family, ok := nlri.ParseFamily(familyKey)
			if !ok {
				continue
			}
			var mode capability.AddPathMode
			switch dirStr {
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

	// Apply add-path enforcement mode.
	if globalSend || globalReceive || perFamilyHasEnforcement {
		applyCapMode(addPathMode, capability.CodeAddPath, ps)
	}

	if !globalSend && !globalReceive && len(perFamily) == 0 {
		return
	}

	// Don't advertise the capability if mode suppresses it (disable/refuse).
	if !addPathMode.advertise() {
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

		// Receive settings — leaf-list: "update state negotiated".
		if v, ok := mapStringJoined(pMap, "receive"); ok {
			parseReceiveFlags(v, &binding)
		}

		// Send settings — leaf-list: "update refresh".
		if v, ok := mapStringJoined(pMap, "send"); ok {
			parseSendFlags(v, &binding)
		}

		ps.ProcessBindings = append(ps.ProcessBindings, binding)
	}
}

// parseReceiveFlags sets receive flags on a ProcessBinding from a space-separated enum list.
func parseReceiveFlags(s string, b *ProcessBinding) {
	for token := range strings.FieldsSeq(s) {
		switch token {
		case "all":
			b.ReceiveUpdate = true
			b.ReceiveOpen = true
			b.ReceiveNotification = true
			b.ReceiveKeepalive = true
			b.ReceiveRefresh = true
			b.ReceiveState = true
			b.ReceiveSent = true
			b.ReceiveNegotiated = true
			return
		case "update":
			b.ReceiveUpdate = true
		case "open":
			b.ReceiveOpen = true
		case "notification":
			b.ReceiveNotification = true
		case "keepalive":
			b.ReceiveKeepalive = true
		case "refresh":
			b.ReceiveRefresh = true
		case "state":
			b.ReceiveState = true
		case "sent":
			b.ReceiveSent = true
		case "negotiated":
			b.ReceiveNegotiated = true
		}
	}
}

// parseSendFlags sets send flags on a ProcessBinding from a space-separated enum list.
func parseSendFlags(s string, b *ProcessBinding) {
	for token := range strings.FieldsSeq(s) {
		switch token {
		case "all":
			b.SendUpdate = true
			b.SendRefresh = true
			return
		case "update":
			b.SendUpdate = true
		case "refresh":
			b.SendRefresh = true
		}
	}
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

// mapStringJoined extracts a string or []string value from a map, joining slices with spaces.
// Used for leaf-list fields where ToMap() produces []string for multi-value entries.
func mapStringJoined(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	if s, ok := v.(string); ok {
		return s, true
	}
	if ss, ok := v.([]string); ok && len(ss) > 0 {
		return strings.Join(ss, " "), true
	}
	return "", false
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
