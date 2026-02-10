package config

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
)

const (
	configTrue    = "true"    // Config value for boolean true
	configFalse   = "false"   // Config value for boolean false
	configEnable  = "enable"  // Config value for enabled state
	configDisable = "disable" // Config value for disabled state
	configRequire = "require" // Config value for required state
	configSelf    = "self"    // Config value for next-hop self

	// DefaultHoldTime is the default hold time per RFC 4271 Section 10.
	DefaultHoldTime = 90

	// ADD-PATH mode strings.
	addPathSend        = "send"
	addPathReceive     = "receive"
	addPathSendReceive = "send/receive"
	addPathReceiveSend = "receive/send"

	// MUP route types for SRv6 Mobile User Plane.
	routeTypeMUPISD  = "mup-isd"
	routeTypeMUPDSD  = "mup-dsd"
	routeTypeMUPT1ST = "mup-t1st"
	routeTypeMUPT2ST = "mup-t2st"

	// Common field names.
	fieldSource = "source"

	// Encoder types.
	encoderText = "text"
)

// FamilyMode represents the negotiation mode for an address family.
type FamilyMode int

const (
	// FamilyModeEnable advertises the family, accepts if peer doesn't support.
	// Strict on UPDATE: error if peer sends NLRI for non-negotiated family.
	FamilyModeEnable FamilyMode = iota
	// FamilyModeDisable does not advertise the family.
	FamilyModeDisable
	// FamilyModeRequire advertises the family, refuses session if peer doesn't support.
	FamilyModeRequire
	// FamilyModeIgnore advertises the family, accepts if peer doesn't support.
	// Lenient on UPDATE: skip NLRI for non-negotiated family instead of error.
	FamilyModeIgnore
)

// String returns the string representation of FamilyMode.
func (m FamilyMode) String() string {
	switch m {
	case FamilyModeEnable:
		return configEnable
	case FamilyModeDisable:
		return configDisable
	case FamilyModeRequire:
		return configRequire
	case FamilyModeIgnore:
		return "ignore"
	default:
		return "unknown"
	}
}

// ParseFamilyMode parses a string into a FamilyMode.
// Returns FamilyModeEnable for empty string or "true"/"enable".
func ParseFamilyMode(s string) FamilyMode {
	switch strings.ToLower(s) {
	case "", configTrue, configEnable:
		return FamilyModeEnable
	case configFalse, configDisable:
		return FamilyModeDisable
	case configRequire:
		return FamilyModeRequire
	case "ignore":
		return FamilyModeIgnore
	default:
		return FamilyModeEnable
	}
}

// FamilyConfig holds configuration for a single address family.
type FamilyConfig struct {
	AFI  string     // "ipv4", "ipv6", "l2vpn", "bgp-ls"
	SAFI string     // "unicast", "multicast", "mpls-vpn", etc.
	Mode FamilyMode // enable, disable, require
}

// RIBOutConfig holds outgoing RIB configuration for route batching.
type RIBOutConfig struct {
	GroupUpdates    bool          // Group compatible routes in single UPDATE (default: true)
	AutoCommitDelay time.Duration // Delay before auto-flushing routes (default: 0 = immediate)
	MaxBatchSize    int           // Maximum routes per batch (default: 0 = unlimited)
}

// DefaultRIBOutConfig returns a RIBOutConfig with default values.
func DefaultRIBOutConfig() RIBOutConfig {
	return RIBOutConfig{
		GroupUpdates:    true,
		AutoCommitDelay: 0,
		MaxBatchSize:    0,
	}
}

// BGPConfig is the typed configuration structure.
type BGPConfig struct {
	RouterID   uint32
	LocalAS    uint32
	Listen     string
	Peers      []PeerConfig
	Plugins    []PluginConfig
	ConfigDir  string                       // Directory containing config file (set by LoadReactorFile)
	EnvValues  map[string]map[string]string // Environment block values (ze-specific)
	ConfigTree map[string]any               // Full config tree for plugin JSON delivery
}

// PeerConfig holds neighbor configuration.
type PeerConfig struct {
	Address              netip.Addr
	Description          string
	RouterID             uint32
	LocalAddress         netip.Addr
	LocalAddressAuto     bool       // true when local-address is "auto"
	LinkLocal            netip.Addr // IPv6 link-local for MP_REACH next-hop (RFC 2545 Section 3)
	LocalAS              uint32
	PeerAS               uint32
	HoldTime             uint16 // RFC 4271: 0 (no keepalive) or >=3; default 90
	Passive              bool
	GroupUpdates         bool           // DEPRECATED: Use RIBOut.GroupUpdates
	RIBOut               RIBOutConfig   // Per-neighbor outgoing RIB config
	Families             []string       // Legacy: "ipv4/unicast", "ipv6/unicast" etc.
	FamilyConfigs        []FamilyConfig // New: structured family config with mode
	IgnoreFamilyMismatch bool           // Ignore NLRI for non-negotiated AFI/SAFI instead of error
	Capabilities         CapabilityConfig
	AddPathFamilies      []AddPathFamilyConfig // Per-family add-path settings (RFC 7911)
	NexthopFamilies      []NexthopFamilyConfig // RFC 8950 Extended Next Hop families
	StaticRoutes         []StaticRouteConfig
	MVPNRoutes           []MVPNRouteConfig
	VPLSRoutes           []VPLSRouteConfig
	FlowSpecRoutes       []FlowSpecRouteConfig
	MUPRoutes            []MUPRouteConfig
	ProcessBindings      []PeerProcessBinding         // Per-peer process bindings
	RawCapabilityConfig  map[string]map[string]string // Capability config for plugins
	CapabilityConfigJSON string                       // Full capability config as JSON for plugin delivery
}

// PeerProcessBinding binds a peer to a plugin process with specific content and message config.
// This separates WHAT messages to send/receive from HOW they are formatted.
type PeerProcessBinding struct {
	PluginName string            // Reference to plugin name (or inline name if Run is set)
	Run        string            // Inline run command (if set, plugin is defined inline)
	Content    PeerContentConfig // HOW: encoding and format
	Receive    PeerReceiveConfig // WHAT: which message types to receive
	Send       PeerSendConfig    // WHAT: which message types to send
}

// PeerContentConfig controls message formatting (encoding + format).
type PeerContentConfig struct {
	Encoding   string                  // "json" | "text" (empty = inherit from process)
	Format     string                  // "parsed" | "raw" | "full" (empty = "parsed")
	Attributes *plugin.AttributeFilter // Which attrs to include (nil = all)
	NLRI       *plugin.NLRIFilter      // Which families to include (nil = all)
}

// PeerReceiveConfig specifies which message types to forward to the process.
type PeerReceiveConfig struct {
	Update       bool // Forward UPDATE messages
	Open         bool // Forward OPEN messages
	Notification bool // Forward NOTIFICATION messages
	Keepalive    bool // Forward KEEPALIVE messages
	Refresh      bool // Forward ROUTE-REFRESH messages
	State        bool // Forward state change events
	Sent         bool // Forward sent UPDATE events
	Negotiated   bool // Forward negotiated capabilities after OPEN exchange
}

// PeerSendConfig specifies which message types the process can send.
type PeerSendConfig struct {
	Update  bool // Allow sending UPDATE messages
	Refresh bool // Allow sending ROUTE-REFRESH requests
}

// AddPathFamilyConfig holds per-family add-path settings per RFC 7911.
type AddPathFamilyConfig struct {
	Family  string // e.g., "ipv4/unicast"
	Send    bool   // Send additional paths
	Receive bool   // Receive additional paths
}

// CapabilityConfig holds capability settings.
type CapabilityConfig struct {
	ASN4            bool
	RouteRefresh    bool
	GracefulRestart bool
	RestartTime     uint16
	AddPathSend     bool
	AddPathReceive  bool
	ExtendedMessage bool // RFC 8654 Extended Message Support
	SoftwareVersion bool
	// RFC 8950 Extended Next Hop: capability is inferred from nexthop { } block presence.
	// No explicit capability flag needed - NexthopFamilyConfig entries determine the capability.
}

// NexthopFamilyConfig defines an extended next-hop family mapping.
// RFC 8950: Maps (NLRI AFI, NLRI SAFI) to the allowed Next Hop AFI.
type NexthopFamilyConfig struct {
	NLRIAFI    uint16 // AFI of the NLRI (1=IPv4, 2=IPv6)
	NLRISAFI   uint8  // SAFI of the NLRI
	NextHopAFI uint16 // AFI of the allowed next-hop (1=IPv4, 2=IPv6)
}

// StaticRouteConfig holds a static route.
type StaticRouteConfig struct {
	Prefix            netip.Prefix
	NextHop           string
	NextHopSelf       bool   // Use local address as next-hop
	Origin            string // igp, egp, incomplete
	LocalPreference   uint32
	MED               uint32
	Community         string
	ExtendedCommunity string
	LargeCommunity    string
	ASPath            string
	PathInformation   string   // path-id for add-path
	Label             string   // MPLS label (backward compat, single)
	Labels            []string // RFC 8277: MPLS label stack (multiple)
	RD                string   // Route Distinguisher
	Aggregator        string   // ASN:IP format
	AtomicAggregate   bool     // ATOMIC_AGGREGATE attribute
	Attribute         string   // Raw attribute hex: [ code flags value ]
	OriginatorID      string   // ORIGINATOR_ID (RFC 4456)
	ClusterList       string   // CLUSTER_LIST (RFC 4456)
	PrefixSID         string   // BGP Prefix-SID (RFC 8669) - can be number or "N, [(base,range),...]"

	// Split prefix into more-specific routes (e.g., "/25" splits /24 into two /25s)
	Split string

	// Watchdog support - routes can be grouped and controlled via API
	Watchdog         string // Watchdog group name (empty = no watchdog)
	WatchdogWithdraw bool   // Start in withdrawn state (held until "watchdog announce")
}

// MVPNRouteConfig holds an MVPN route configuration.
type MVPNRouteConfig struct {
	RouteType         string // shared-join, source-join, source-ad
	IsIPv6            bool
	RD                string
	SourceAS          uint32
	Source            string // source IP or RP IP
	Group             string // multicast group
	NextHop           string
	Origin            string
	LocalPreference   uint32
	MED               uint32
	ExtendedCommunity string
	OriginatorID      string // RFC 4456 route reflector
	ClusterList       string // RFC 4456 route reflector
}

// VPLSRouteConfig holds a VPLS route configuration.
type VPLSRouteConfig struct {
	Name              string
	RD                string
	Endpoint          uint16
	Base              uint32
	Offset            uint16
	Size              uint16
	NextHop           string
	Origin            string
	LocalPreference   uint32
	MED               uint32
	ASPath            string
	Community         string
	ExtendedCommunity string
	OriginatorID      string
	ClusterList       string
}

// FlowSpecRouteConfig holds a FlowSpec route configuration.
// RFC 8955 Section 4: NLRI contains match criteria (destination, source, protocol, ports, etc.)
// RFC 8955 Section 7: Actions are encoded as Extended Communities (rate-limit, redirect, etc.)
type FlowSpecRouteConfig struct {
	Name              string
	IsIPv6            bool
	RD                string              // for flow-vpn (SAFI 134)
	NLRI              map[string][]string // Match criteria (RFC 8955 Section 4)
	NextHop           string
	Community         string
	ExtendedCommunity string // Actions as extended communities (RFC 8955 Section 7)
	Attribute         string // Raw attribute hex: [ code flags value ]
}

// MUPRouteConfig holds a MUP route configuration.
type MUPRouteConfig struct {
	RouteType         string // mup-isd, mup-dsd, mup-t1st, mup-t2st
	IsIPv6            bool
	Prefix            string
	Address           string
	RD                string
	TEID              string
	QFI               uint8
	Endpoint          string
	Source            string // T1ST source address
	NextHop           string
	ExtendedCommunity string
	PrefixSID         string
}

// PluginConfig holds plugin configuration.
type PluginConfig struct {
	Name          string
	Run           string // Command to run (empty for internal plugins)
	Encoder       string
	ReceiveUpdate bool          // Forward received UPDATEs to plugin stdin
	StageTimeout  time.Duration // Per-stage timeout (0 = use default 5s)
	Internal      bool          // If true, run in-process via goroutine (ze.X plugins)
}

// TreeToConfig converts a parsed tree to a typed BGPConfig.
func TreeToConfig(tree *Tree) (*BGPConfig, error) {
	cfg := &BGPConfig{}

	// BGP block is required - contains router-id, local-as, listen, peer
	bgpContainer := tree.GetContainer("bgp")
	if bgpContainer == nil {
		return nil, fmt.Errorf("missing required bgp { } block")
	}

	// BGP settings from bgp { } block
	if v, ok := bgpContainer.Get("router-id"); ok {
		ip, err := netip.ParseAddr(v)
		if err != nil {
			return nil, fmt.Errorf("invalid bgp.router-id: %w", err)
		}
		cfg.RouterID = ipToUint32(ip)
	}

	if v, ok := bgpContainer.Get("local-as"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid bgp.local-as: %w", err)
		}
		cfg.LocalAS = uint32(n)
	}

	if v, ok := bgpContainer.Get("listen"); ok {
		cfg.Listen = v
	}

	// Plugins - new syntax: plugin { external <name> { ... } }
	if pluginContainer := tree.GetContainer("plugin"); pluginContainer != nil {
		for name, proc := range pluginContainer.GetList("external") {
			// Reject reserved names (underscore prefix used internally)
			if strings.HasPrefix(name, "_") {
				return nil, fmt.Errorf("plugin name %q: names starting with underscore are reserved", name)
			}
			pc := PluginConfig{Name: name}
			if v, ok := proc.Get("run"); ok {
				pc.Run = v
			}
			if v, ok := proc.Get("encoder"); ok {
				pc.Encoder = v
			}
			if v, ok := proc.Get("timeout"); ok {
				d, err := time.ParseDuration(v)
				if err != nil {
					return nil, fmt.Errorf("plugin %q: invalid timeout %q: %w", name, v, err)
				}
				if d < 0 {
					return nil, fmt.Errorf("plugin %q: timeout must be positive, got %q", name, v)
				}
				pc.StageTimeout = d
			}
			// Default: text encoder plugins receive updates
			// TODO: Parse process { receive { update; } } from peer/template for proper config
			if pc.Encoder == encoderText {
				pc.ReceiveUpdate = true
			}
			cfg.Plugins = append(cfg.Plugins, pc)
		}
	}

	// Parse templates for inheritance
	// New syntax: template { bgp { peer <pattern> { inherit-name <name>; ... } } }
	// Legacy syntax: template { group <name> { ... } }
	templates := make(map[string]*Tree)         // Named templates (for 'inherit <name>')
	templatePatterns := make(map[string]string) // Pattern for each named template
	peerGlobs := make([]PeerGlob, 0)            // Auto-apply patterns (no inherit-name)

	if tmpl := tree.GetContainer("template"); tmpl != nil {
		// New syntax: template { bgp { peer <pattern> { inherit-name <name>; ... } } }
		if bgpTmpl := tmpl.GetContainer("bgp"); bgpTmpl != nil {
			for _, entry := range bgpTmpl.GetListOrdered("peer") {
				pattern := entry.Key
				peerTree := entry.Value

				// Check if this is a named template (has inherit-name)
				if inheritName, hasName := peerTree.Get("inherit-name"); hasName {
					templates[inheritName] = peerTree
					templatePatterns[inheritName] = pattern
				} else {
					// No inherit-name: auto-apply to matching peers
					peerGlobs = append(peerGlobs, PeerGlob{
						Pattern:     pattern,
						Specificity: 0, // Config order, not specificity
						Tree:        peerTree,
					})
				}
			}
		}

		// Legacy syntax: template { group <name> { ... } }
		for name, groupTree := range tmpl.GetList("group") {
			if !isValidGroupName(name) {
				return nil, fmt.Errorf("invalid group name %q: must start with letter, end with letter/number, contain only letters/numbers/hyphens", name)
			}
			if _, hasInherit := groupTree.Get("inherit"); hasInherit {
				return nil, fmt.Errorf("inherit only valid inside peer { }, not in template.group %s", name)
			}
			templates[name] = groupTree
			// Legacy templates have no pattern restriction
		}

		// Legacy syntax: template { match <pattern> { ... } } - auto-apply
		for _, entry := range tmpl.GetListOrdered("match") {
			if _, hasInherit := entry.Value.Get("inherit"); hasInherit {
				return nil, fmt.Errorf("inherit only valid inside peer { }, not in template.match %s", entry.Key)
			}
			peerGlobs = append(peerGlobs, PeerGlob{
				Pattern:     entry.Key,
				Specificity: 0,
				Tree:        entry.Value,
			})
		}
	}

	// Build set of defined plugin names for validation
	pluginNames := make(map[string]bool)
	for _, p := range cfg.Plugins {
		pluginNames[p.Name] = true
	}

	// Peers from bgp { peer <IP> { ... } }
	for addr, n := range bgpContainer.GetList("peer") {
		// It's an IP address, treat as neighbor
		nc, err := parsePeerConfig(addr, n, templates, templatePatterns, peerGlobs)
		if err != nil {
			return nil, fmt.Errorf("bgp.peer %s: %w", addr, err)
		}

		// Validate process binding plugin references
		// Skip validation if binding has inline Run (defines plugin inline)
		for _, binding := range nc.ProcessBindings {
			if binding.Run == "" && !pluginNames[binding.PluginName] {
				return nil, fmt.Errorf("bgp.peer %s: undefined plugin %q in process binding", addr, binding.PluginName)
			}
		}

		cfg.Peers = append(cfg.Peers, nc)
	}

	// Register inline plugins (process bindings with Run defined inline)
	inlinePlugins := make(map[string]bool)
	for _, peer := range cfg.Peers {
		for _, binding := range peer.ProcessBindings {
			if binding.Run != "" && !inlinePlugins[binding.PluginName] {
				inlinePlugins[binding.PluginName] = true
				cfg.Plugins = append(cfg.Plugins, PluginConfig{
					Name:          binding.PluginName,
					Run:           binding.Run,
					Encoder:       encoderText, // Default to text encoder
					ReceiveUpdate: true,        // Default: receive updates
				})
			}
		}
	}

	// Validate process-dependent capabilities
	if err := validateProcessCapabilities(cfg.Peers); err != nil {
		return nil, err
	}

	return cfg, nil
}
