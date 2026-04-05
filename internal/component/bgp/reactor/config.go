// Design: docs/architecture/core-design.md — config tree parsing (PeersFromTree)
// Overview: reactor.go — BGP reactor event loop and peer management
// Detail: config_capabilities.go — BGP capability parsing from config tree
// Related: peersettings.go — PeerSettings type produced by config parsing

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
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
)

// Config tree string constants (shared with reactor.go to satisfy goconst).
const (
	valTrue    = "true"
	valFalse   = "false"
	valEnable  = "enable"
	valDisable = "disable"
	valRequire = "require"
	valRefuse  = "refuse"
	valAuto    = "auto" // local ip "auto" = use system default
)

// parsePeerFromTree parses a peer's settings from a flattened config tree.
// The tree is the peer subtree from a map[string]any config,
// already resolved with template inheritance.
// name is the peer's name (the list key in the config).
// localAS and routerID are global defaults from the bgp block.
func parsePeerFromTree(name string, tree map[string]any, localAS, routerID uint32) (*PeerSettings, error) {
	// Navigate nested containers.
	connMap, _ := mapMap(tree, "connection")
	sessionMap, _ := mapMap(tree, "session")
	behaviorMap, _ := mapMap(tree, "behavior")

	// Remote AS from session > asn > remote (required).
	var peerAS uint32
	if sessionMap != nil {
		if asnMap, ok := mapMap(sessionMap, "asn"); ok {
			if v, ok := mapUint32(asnMap, "remote"); ok {
				peerAS = v
			}
		}
	}
	if peerAS == 0 {
		return nil, fmt.Errorf("peer %s: missing required session > asn > remote", name)
	}

	// Remote IP from connection > remote > ip (required).
	var remoteIPStr string
	var connRemoteMap map[string]any
	if connMap != nil {
		connRemoteMap, _ = mapMap(connMap, "remote")
		if connRemoteMap != nil {
			remoteIPStr, _ = mapString(connRemoteMap, "ip")
		}
	}
	if remoteIPStr == "" {
		return nil, fmt.Errorf("peer %s: missing required connection > remote > ip", name)
	}
	ip, err := netip.ParseAddr(remoteIPStr)
	if err != nil {
		return nil, fmt.Errorf("peer %s: invalid remote ip %q: %w", name, remoteIPStr, err)
	}

	// Local AS from session > asn > local (optional per-peer override).
	peerLocalAS := localAS
	if sessionMap != nil {
		if asnMap, ok := mapMap(sessionMap, "asn"); ok {
			if v, ok := mapUint32(asnMap, "local"); ok {
				peerLocalAS = v
			}
		}
	}
	if peerLocalAS == 0 {
		return nil, fmt.Errorf("peer %s: missing required local as (neither global nor peer-level)", name)
	}

	// Router ID from session > router-id (peer-level overrides global).
	peerRouterID := routerID
	if sessionMap != nil {
		if v, ok := mapString(sessionMap, "router-id"); ok {
			rid, err := netip.ParseAddr(v)
			if err != nil {
				return nil, fmt.Errorf("peer %s: invalid router-id: %w", name, err)
			}
			peerRouterID = ipToUint32(rid)
		}
	}

	ps := NewPeerSettings(ip, peerLocalAS, peerAS, peerRouterID)

	// Timer container (receive-hold-time, send-hold-time, connect-retry).
	timerMap, _ := mapMap(tree, "timer")
	if timerMap != nil {
		if v, ok := mapUint32(timerMap, "receive-hold-time"); ok {
			// RFC 4271 Section 4.2: Hold Time MUST be either zero or at least three seconds.
			if v >= 1 && v <= 2 {
				return nil, fmt.Errorf("peer %s: invalid receive-hold-time %d: RFC 4271 requires 0 or >= 3 seconds", name, v)
			}
			ps.ReceiveHoldTime = time.Duration(v) * time.Second
		}
		if v, ok := mapUint32(timerMap, "send-hold-time"); ok {
			// RFC 9687: Send Hold Timer. 0 = auto (max(8min, 2x receive-hold-time)).
			if v != 0 && v < 480 {
				return nil, fmt.Errorf("peer %s: invalid send-hold-time %d: RFC 9687 requires 0 (auto) or >= 480 seconds", name, v)
			}
			ps.SendHoldTime = time.Duration(v) * time.Second
		}
		if v, ok := mapUint32(timerMap, "connect-retry"); ok {
			ps.ConnectRetry = time.Duration(v) * time.Second
		}
	}

	// Connection mode from connection > local > connect and connection > remote > accept.
	if connMap != nil {
		if connLocalMap, ok := mapMap(connMap, "local"); ok {
			if v, ok := mapBool(connLocalMap, "connect"); ok {
				ps.Connection.Connect = v
			}
		}
		if connRemoteMap != nil {
			if v, ok := mapBool(connRemoteMap, "accept"); ok {
				ps.Connection.Accept = v
			}
		}
	}
	if !ps.Connection.Connect && !ps.Connection.Accept {
		return nil, fmt.Errorf("peer %s: connect and accept cannot both be false", name)
	}

	// Per-peer listen port from connection > local > port or connection > remote > port.
	if connMap != nil {
		if connLocalMap, ok := mapMap(connMap, "local"); ok {
			if v, ok := mapUint32(connLocalMap, "port"); ok {
				if v < 1 || v > 65535 {
					return nil, fmt.Errorf("peer %s: port must be 1-65535, got %d", name, v)
				}
				ps.Port = uint16(v)
			}
		}
		if connRemoteMap, ok := mapMap(connMap, "remote"); ok {
			if v, ok := mapUint32(connRemoteMap, "port"); ok {
				if v < 1 || v > 65535 {
					return nil, fmt.Errorf("peer %s: remote port must be 1-65535, got %d", name, v)
				}
				// Use remote port for connection target if set.
				ps.Port = uint16(v)
			}
		}
	}

	// Group updates from behavior > group-updates (default true from NewPeerSettings).
	if behaviorMap != nil {
		if v, ok := mapBool(behaviorMap, "group-updates"); ok {
			ps.GroupUpdates = v
		}
	}

	// Local address from connection > local > ip (required).
	var localAddrStr string
	if connMap != nil {
		if connLocalMap, ok := mapMap(connMap, "local"); ok {
			localAddrStr, _ = mapString(connLocalMap, "ip")
		}
	}
	if localAddrStr == "" {
		return nil, fmt.Errorf("peer %s: local ip is required (use IP address or \"auto\")", name)
	}
	if localAddrStr != valAuto {
		la, err := netip.ParseAddr(localAddrStr)
		if err != nil {
			return nil, fmt.Errorf("peer %s: invalid local ip: %w", name, err)
		}
		ps.LocalAddress = la
	}

	// RFC 2545 Section 3: IPv6 link-local address for MP_REACH next-hop from session > link-local.
	if sessionMap != nil {
		if v, ok := mapString(sessionMap, "link-local"); ok {
			ll, err := netip.ParseAddr(v)
			if err != nil {
				return nil, fmt.Errorf("peer %s: invalid link-local: %w", name, err)
			}
			ps.LinkLocal = ll
		}
	}

	// Parse families from session > family (includes per-family prefix limits).
	if sessionMap != nil {
		if err := parseFamiliesFromTree(sessionMap, ps); err != nil {
			return nil, fmt.Errorf("peer %s: %w", name, err)
		}
	}

	// Parse capabilities from session > capability and session > add-path.
	if sessionMap != nil {
		parseCapabilitiesFromTree(sessionMap, ps)
	}

	// Parse process bindings.
	if err := parseProcessBindingsFromTree(tree, ps); err != nil {
		return nil, fmt.Errorf("peer %s: %w", name, err)
	}

	// TCP MD5 authentication (RFC 2385) from connection > md5.
	if connMap != nil {
		if md5Map, ok := mapMap(connMap, "md5"); ok {
			if md5, ok := mapString(md5Map, "password"); ok {
				if !network.TCPMD5Supported() {
					reactorLogger().Warn("md5-password configured but TCP MD5 is not supported on this platform; connections will fail",
						"peer", name)
				}
				ps.MD5Key = md5
				if md5ip, ok := mapString(md5Map, "ip"); ok {
					a, err := netip.ParseAddr(md5ip)
					if err != nil {
						return nil, fmt.Errorf("peer %s: invalid md5 ip: %w", name, err)
					}
					ps.MD5IP = a
				}
			} else if _, hasMD5IP := mapString(md5Map, "ip"); hasMD5IP {
				return nil, fmt.Errorf("peer %s: md5 ip requires md5 password", name)
			}
		}
	}

	return ps, nil
}

// PeersFromTree parses all peer settings from a bgp subtree (map[string]any).
// The tree should be the "bgp" block from a resolved config, with templates
// already applied and flattened via Tree.ToMap().
// Returns the list of PeerSettings and an error if any peer fails to parse.
//
// Global local > as and router-id are optional defaults. Each peer can override
// them or provide its own. If neither global nor peer-level local as is set,
// parsePeerFromTree returns an error for that peer.
func PeersFromTree(bgpTree map[string]any) ([]*PeerSettings, error) {
	// Ensure plugin-registered event and send types are available for validation.
	// Idempotent -- safe to call multiple times. Needed here because config parsing
	// happens before NewServer, and dynamic types must be valid.
	plugin.RegisterPluginEventTypes()
	plugin.RegisterPluginSendTypes()

	// Extract global defaults (both optional -- peers can provide their own).
	// Global local AS is under bgp > session > asn > local.
	var localAS uint32
	if sessionMap, ok := mapMap(bgpTree, "session"); ok {
		if asnMap, ok := mapMap(sessionMap, "asn"); ok {
			localAS, _ = mapUint32(asnMap, "local")
		}
	}

	var routerID uint32
	if v, ok := mapString(bgpTree, "router-id"); ok {
		rid, err := netip.ParseAddr(v)
		if err != nil {
			return nil, fmt.Errorf("bgp: invalid router-id: %w", err)
		}
		routerID = ipToUint32(rid)
	}

	// Parse peers. Key is now peer name (not IP address).
	peerMap, ok := mapMap(bgpTree, "peer")
	if !ok {
		return nil, nil
	}

	var peers []*PeerSettings
	for peerName, val := range peerMap {
		peerTree, ok := val.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("peer %s: invalid config (expected map)", peerName)
		}
		ps, err := parsePeerFromTree(peerName, peerTree, localAS, routerID)
		if err != nil {
			// Skip incomplete peers (missing required fields like remote IP/ASN).
			// This allows the daemon to start for config editing with partial configs.
			reactorLogger().Warn("skipping peer with invalid config", "peer", peerName, "error", err)
			continue
		}

		// The peer name is the list key itself.
		ps.Name = peerName

		// Extract group name (set by ResolveBGPTree).
		if groupName, ok := mapString(peerTree, "group-name"); ok {
			ps.GroupName = groupName
		}

		peers = append(peers, ps)
	}

	return peers, nil
}

// parseFamiliesFromTree parses address family configuration from the tree.
// Builds Multiprotocol capabilities, tracks required/ignore families,
// and extracts per-family prefix limits (RFC 4486).
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
		var familyEntryMap map[string]any
		switch v := val.(type) {
		case string:
			modeStr = v
		case map[string]any:
			familyEntryMap = v
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

		// RFC 4486: Extract per-family prefix limits.
		if err := parsePrefixLimitFromFamily(key, familyEntryMap, ps); err != nil {
			return err
		}
	}

	return nil
}

// parsePrefixLimitFromFamily extracts prefix maximum, warning, teardown, idle-timeout,
// and updated from a family entry's prefix block.
// RFC 4486 Section 4: Maximum Number of Prefixes Reached.
// Every non-disabled family MUST have a prefix maximum configured.
func parsePrefixLimitFromFamily(familyKey string, entryMap map[string]any, ps *PeerSettings) error {
	prefixMap, hasPrefixBlock := mapMap(entryMap, "prefix")
	if !hasPrefixBlock {
		return fmt.Errorf("family %s: prefix maximum is mandatory (add prefix { maximum N; })", familyKey)
	}

	maximum, ok := mapUint32(prefixMap, "maximum")
	if !ok || maximum == 0 {
		return fmt.Errorf("family %s: prefix maximum is mandatory and must be > 0", familyKey)
	}

	// Initialize maps lazily.
	if ps.PrefixMaximum == nil {
		ps.PrefixMaximum = make(map[string]uint32)
	}
	if ps.PrefixWarning == nil {
		ps.PrefixWarning = make(map[string]uint32)
	}

	ps.PrefixMaximum[familyKey] = maximum

	// Warning defaults to 90% of maximum.
	warning, hasWarning := mapUint32(prefixMap, "warning")
	if hasWarning {
		if warning >= maximum {
			return fmt.Errorf("family %s: prefix warning (%d) must be less than maximum (%d)", familyKey, warning, maximum)
		}
		ps.PrefixWarning[familyKey] = warning
	} else {
		ps.PrefixWarning[familyKey] = maximum * 9 / 10
	}

	// Per-family prefix enforcement settings (teardown, idle-timeout, updated).
	if v, ok := mapString(prefixMap, "teardown"); ok {
		ps.PrefixTeardown = v != valFalse
	}

	if v, ok := mapUint32(prefixMap, "idle-timeout"); ok {
		ps.PrefixIdleTimeout = uint16(v) //nolint:gosec // Bounded by YANG uint16 range
	}

	if v, ok := mapString(prefixMap, "updated"); ok {
		ps.PrefixUpdated = v
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
	case valFalse, valDisable:
		return familyModeDisable
	case valRequire:
		return familyModeRequire
	case "ignore":
		return familyModeIgnore
	}
	return familyModeEnable
}

// parseProcessBindingsFromTree parses process bindings from the peer tree.
func parseProcessBindingsFromTree(tree map[string]any, ps *PeerSettings) error {
	procMap, ok := mapMap(tree, "process")
	if !ok {
		return nil
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
			if err := parseReceiveFlags(v, &binding); err != nil {
				return fmt.Errorf("process %q: %w", name, err)
			}
		}

		// Send settings — leaf-list: "update refresh".
		if v, ok := mapStringJoined(pMap, "send"); ok {
			if err := parseSendFlags(v, &binding); err != nil {
				return fmt.Errorf("process %q: %w", name, err)
			}
		}

		ps.ProcessBindings = append(ps.ProcessBindings, binding)
	}
	return nil
}

// parseReceiveFlags sets receive flags on a ProcessBinding from a space-separated list.
// Base event types are mapped to bool fields. Plugin-registered event types (validated
// against plugin.IsValidEvent) are stored in ReceiveCustom.
// Unknown event types cause a config parse error (fail-on-unknown per config-design.md).
func parseReceiveFlags(s string, b *ProcessBinding) error {
	for token := range strings.FieldsSeq(s) {
		if err := parseOneReceiveFlag(token, b); err != nil {
			return err
		}
	}
	return nil
}

// parseOneReceiveFlag handles a single receive token.
// "all" is not accepted: list event types explicitly to avoid silently receiving
// new event types when plugins register them (e.g., "rpki", "update-rpki").
func parseOneReceiveFlag(token string, b *ProcessBinding) error {
	switch token {
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
	default: // Plugin-registered event types (e.g., "rpki", "update-rpki"). Fail on truly unknown.
		if !plugin.IsValidEvent(plugin.NamespaceBGP, token) {
			return fmt.Errorf("invalid value for receive: %q (valid: %s)",
				token, plugin.ValidEventNames(plugin.NamespaceBGP))
		}
		if b.ReceiveCustom == nil {
			b.ReceiveCustom = make(map[string]bool)
		}
		b.ReceiveCustom[token] = true
	}
	return nil
}

// parseSendFlags sets send flags on a ProcessBinding from a space-separated enum list.
// "all" is not accepted: list send types explicitly.
func parseSendFlags(s string, b *ProcessBinding) error {
	for token := range strings.FieldsSeq(s) {
		if err := parseOneSendFlag(token, b); err != nil {
			return err
		}
	}
	return nil
}

// parseOneSendFlag handles a single send token.
// Base types (update, refresh) have dedicated bool fields.
// Plugin-registered types (e.g., enhanced-refresh) are validated against
// the dynamic ValidSendTypes registry and stored in the SendCustom map.
func parseOneSendFlag(token string, b *ProcessBinding) error {
	switch token {
	case "update":
		b.SendUpdate = true
		return nil
	case "refresh":
		b.SendRefresh = true
		return nil
	}
	// Plugin-registered send types: validate against dynamic registry.
	if plugin.IsValidSendType(token) {
		if b.SendCustom == nil {
			b.SendCustom = make(map[string]bool)
		}
		b.SendCustom[token] = true
		return nil
	}
	valid := "update, refresh"
	if extra := plugin.ValidSendTypeNames(); extra != "" {
		valid += ", " + extra
	}
	return fmt.Errorf("invalid value for send: %q (valid: %s)", token, valid)
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

// mapBool extracts a boolean value from a map (stored as string "true"/"false" or "1"/"0").
func mapBool(m map[string]any, key string) (bool, bool) {
	s, ok := mapString(m, key)
	if !ok {
		return false, false
	}
	return s == valTrue || s == "1", true
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
