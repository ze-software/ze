package config

import (
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/zebgp/pkg/api"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/message"
)

const (
	configTrue    = "true"    // Config value for boolean true
	configFalse   = "false"   // Config value for boolean false
	configEnable  = "enable"  // Config value for enabled state
	configDisable = "disable" // Config value for disabled state
	configRequire = "require" // Config value for required state

	// ADD-PATH mode strings.
	addPathSend        = "send"
	addPathReceive     = "receive"
	addPathSendReceive = "send/receive"
	addPathReceiveSend = "receive/send"

	// MUP route types for SRv6 Mobile User Plane.
	routeTypeMUPISD  = "mup-isd"
	routeTypeMUPT1ST = "mup-t1st"

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

// flowRouteAttributes returns field definitions for FlowSpec routes.
// Format: route NAME { rd VALUE; match { ... } then { ... } }.
func flowRouteAttributes() []FieldDef {
	return []FieldDef{
		Field("rd", Leaf(TypeString)),
		Field("next-hop", Leaf(TypeString)),
		Field("extended-community", ValueOrArray(TypeString)),
		Field("match", Freeform()), // Match criteria
		Field("scope", Freeform()), // Scope criteria (interface-set, etc.)
		Field("then", Freeform()),  // Actions
	}
}

// mcastVpnAttributes returns field definitions for MCAST-VPN routes.
// Format: mcast-vpn <type> rp <ip> group <ip> rd <rd> source-as <asn> next-hop <ip> ...
func mcastVpnAttributes() []FieldDef {
	return []FieldDef{
		Field("rp", Leaf(TypeString)),
		Field("group", Leaf(TypeString)),
		Field("source", Leaf(TypeString)),
		Field("rd", Leaf(TypeString)),
		Field("source-as", Leaf(TypeUint32)),
		Field("next-hop", Leaf(TypeString)),
		Field("extended-community", ValueOrArray(TypeString)),
		Field("community", ValueOrArray(TypeString)),
		Field("origin", Leaf(TypeString)),
		Field("local-preference", Leaf(TypeUint32)),
		Field("med", Leaf(TypeUint32)),
		Field("originator-id", Leaf(TypeIPv4)),          // RFC 4456
		Field("cluster-list", ValueOrArray(TypeString)), // RFC 4456
	}
}

// vplsAttributes returns field definitions for VPLS routes.
func vplsAttributes() []FieldDef {
	return []FieldDef{
		Field("rd", Leaf(TypeString)),
		Field("endpoint", Leaf(TypeUint16)),
		Field("base", Leaf(TypeUint32)),
		Field("offset", Leaf(TypeUint16)),
		Field("size", Leaf(TypeUint16)),
		Field("next-hop", Leaf(TypeString)),
		Field("origin", Leaf(TypeString)),
		Field("local-preference", Leaf(TypeUint32)),
		Field("med", Leaf(TypeUint32)),
		Field("as-path", ValueOrArray(TypeString)),
		Field("community", ValueOrArray(TypeString)),
		Field("extended-community", ValueOrArray(TypeString)),
		Field("originator-id", Leaf(TypeIPv4)),
		Field("cluster-list", ValueOrArray(TypeString)),
	}
}

// routeAttributes returns the common route attribute field definitions.
// Used by both static and announce route schemas.
func routeAttributes() []FieldDef {
	return []FieldDef{
		Field("next-hop", Leaf(TypeString)),
		Field("origin", Leaf(TypeString)),
		Field("local-preference", Leaf(TypeUint32)),
		Field("med", Leaf(TypeUint32)),
		Field("community", ValueOrArray(TypeString)),
		Field("extended-community", ValueOrArray(TypeString)),
		Field("large-community", ValueOrArray(TypeString)),
		Field("as-path", ValueOrArray(TypeString)),
		Field("path-information", Leaf(TypeString)),
		Field("label", Leaf(TypeString)),
		Field("labels", ValueOrArray(TypeString)), // RFC 8277: Multi-label support
		Field("rd", Leaf(TypeString)),
		Field("aggregator", Leaf(TypeString)),
		Field("atomic-aggregate", Flex()), // Can be standalone or followed by aggregator
		Field("originator-id", Leaf(TypeIPv4)),
		Field("cluster-list", Leaf(TypeString)),
		Field("name", Leaf(TypeString)),
		Field("split", Leaf(TypeString)),
		Field("watchdog", Leaf(TypeString)),
		Field("withdraw", Flex()),                    // Withdraw route (flag)
		Field("attribute", ValueOrArray(TypeString)), // Generic attributes
		Field("bgp-prefix-sid", Flex()),              // Prefix SID - can use ( ... ) syntax
		Field("bgp-prefix-sid-srv6", Flex()),         // SRv6 Prefix SID - can use ( ... ) syntax
	}
}

// peerFields returns the common field definitions for neighbor and template schemas.
// Field order determines serialization order in generated configs.
func peerFields() []FieldDef {
	return []FieldDef{
		// Template inheritance (first, so it's clear what template is used)
		Field("inherit", Leaf(TypeString)),

		// Basic peer identification
		Field("description", Leaf(TypeString)),
		Field("router-id", Leaf(TypeIPv4)),
		Field("local-address", Leaf(TypeString)), // IP address or "auto"
		Field("local-as", Leaf(TypeUint32)),
		Field("peer-as", Leaf(TypeUint32)),
		Field("hold-time", LeafWithDefault(TypeUint16, "90")),
		Field("passive", LeafWithDefault(TypeBool, "false")),
		Field("group-updates", LeafWithDefault(TypeBool, configTrue)),

		// Address families (before capability)
		Field("family", FamilyBlock()), // { ipv4/unicast; ipv4 { unicast require; } }

		// Capabilities
		Field("capability", Container(
			Field("asn4", LeafWithDefault(TypeBool, configTrue)),
			Field("route-refresh", Flex()), // flag, value, or block
			Field("graceful-restart", Flex( // flag, value, or block
				Field("restart-time", LeafWithDefault(TypeUint16, "120")),
			)),
			Field("add-path", Flex( // flag, value (send/receive), or block
				Field("send", LeafWithDefault(TypeBool, "false")),
				Field("receive", LeafWithDefault(TypeBool, "false")),
			)),
			Field("extended-message", Flex()), // RFC 8654 Extended Message
			Field("nexthop", Flex()),
			Field("multi-session", Flex()),
			Field("operational", Flex()),
			Field("aigp", Flex()),
			Field("software-version", Flex()),
		)),

		// Additional peer settings
		Field("host-name", Leaf(TypeString)),
		Field("domain-name", Leaf(TypeString)),
		Field("md5-password", Leaf(TypeString)),
		Field("md5-ip", Leaf(TypeIP)),
		Field("ttl-security", Leaf(TypeUint16)),
		Field("outgoing-ttl", Leaf(TypeUint16)),
		Field("incoming-ttl", Leaf(TypeUint16)),
		Field("multi-session", LeafWithDefault(TypeBool, "false")),
		Field("nexthop", Freeform()),        // nexthop configuration
		Field("manual-eor", Leaf(TypeBool)), // manual end-of-RIB
		Field("auto-flush", Leaf(TypeBool)), // auto-flush routes
		Field("adj-rib-out", Leaf(TypeBool)),
		Field("adj-rib-in", Leaf(TypeBool)),

		// Announce routes - structured by AFI/SAFI
		// announce { ipv4 { unicast <prefix> <attrs>; } ipv6 { unicast <prefix> <attrs>; } }
		Field("announce", Container(
			Field("ipv4", Container(
				Field("unicast", InlineList(TypePrefix, routeAttributes()...)),
				Field("multicast", InlineList(TypePrefix, routeAttributes()...)),
				Field("nlri-mpls", InlineList(TypePrefix, routeAttributes()...)), // labeled-unicast (SAFI 4)
				Field("mpls-vpn", InlineList(TypePrefix, routeAttributes()...)),
				Field("mcast-vpn", InlineList(TypeString, mcastVpnAttributes()...)),
				Field("mup", Flex()),      // MUP - complex inline format with nested parens
				Field("flow", Freeform()), // FlowSpec - complex format
			)),
			Field("ipv6", Container(
				Field("unicast", InlineList(TypePrefix, routeAttributes()...)),
				Field("multicast", InlineList(TypePrefix, routeAttributes()...)),
				Field("nlri-mpls", InlineList(TypePrefix, routeAttributes()...)), // labeled-unicast (SAFI 4)
				Field("mpls-vpn", InlineList(TypePrefix, routeAttributes()...)),
				Field("mcast-vpn", InlineList(TypeString, mcastVpnAttributes()...)),
				Field("mup", Flex()),      // MUP - complex inline format with nested parens
				Field("flow", Freeform()), // FlowSpec - complex format
			)),
			Field("l2vpn", Container(
				Field("vpls", Flex(vplsAttributes()...)),
				Field("evpn", Freeform()), // EVPN - complex format
			)),
		)),

		// Static routes - InlineList supports "route PREFIX attr val attr val;"
		Field("static", Container(
			Field("route", InlineList(TypePrefix, routeAttributes()...)),
		)),

		// Flow routes - named list with match/then sub-blocks
		Field("flow", Container(
			Field("route", List(TypeString, flowRouteAttributes()...)),
		)),

		// L2VPN (VPLS, EVPN)
		Field("l2vpn", Container(
			Field("vpls", Flex(vplsAttributes()...)),
			Field("evpn", Freeform()), // EVPN - complex format
		)),

		// Add-path per-family configuration
		Field("add-path", Freeform()),

		// API configuration - supports both old and new syntax:
		// Old: api { processes [ foo ]; } or api <label> { processes [...]; receive {...}; }
		// New: api <process-name> { content { encoding json; }; receive { update; }; }
		Field("api", List(TypeString,
			// Old syntax fields (ExaBGP compatibility)
			Field("processes", ArrayLeaf(TypeString)),       // processes [ foo bar ]
			Field("processes-match", ArrayLeaf(TypeString)), // processes-match [ "^pattern" ]
			Field("neighbor-changes", Flex()),               // neighbor-changes; (flag)

			// New syntax fields
			Field("content", Container(
				Field("encoding", Leaf(TypeString)),  // json | text
				Field("format", Leaf(TypeString)),    // parsed | raw | full
				Field("attribute", Leaf(TypeString)), // all | none | "origin as-path ..."
				Field("nlri", Flex()),                // ipv4/unicast; ipv6/unicast; (repeated)
			)),
			Field("receive", Freeform()), // { update; open; notification; all; }
			Field("send", Freeform()),    // { update; refresh; all; }
		)),

		// Operational messages (ExaBGP-specific, not supported in ZeBGP)
		// We parse it to detect and warn, but don't process it
		Field("operational", Freeform()),

		// RIB configuration (per-neighbor)
		Field("rib", Container(
			Field("out", Container(
				Field("group-updates", LeafWithDefault(TypeBool, configTrue)),
				Field("auto-commit-delay", LeafWithDefault(TypeDuration, "0")),
				Field("max-batch-size", LeafWithDefault(TypeInt, "0")),
			)),
		)),
	}
}

// BGPSchema returns the schema for ZeBGP configuration (current syntax).
// Use LegacyBGPSchema for migration from old ExaBGP configs.
func BGPSchema() *Schema {
	schema := NewSchema()

	// Environment settings (ZeBGP-specific, not in ExaBGP)
	// Processed first, before template/peer/process blocks
	schema.Define("environment", environmentBlock())

	// Global settings
	schema.Define("router-id", Leaf(TypeIPv4))
	schema.Define("local-as", Leaf(TypeUint32))
	schema.Define("listen", MultiLeaf(TypeString)) // "address port"

	// Process definitions (API)
	schema.Define("process", List(TypeString,
		Field("run", MultiLeaf(TypeString)), // command with args
		Field("encoder", Leaf(TypeString)),  // json, text
		Field("respawn", Leaf(TypeBool)),    // respawn on exit
	))

	// Template definitions - named templates and glob patterns
	// template { group <name> { ... }; match <pattern> { ... } }
	schema.Define("template", Container(
		Field("group", List(TypeString, peerFields()...)), // Named templates (inherit <name>)
		Field("match", List(TypeString, peerFields()...)), // Glob patterns (auto-apply by IP)
	))

	// Peer definitions - actual BGP sessions (requires IP address)
	schema.Define("peer", List(TypeIP, peerFields()...))

	return schema
}

// environmentBlock returns the schema for the environment configuration block.
// Maps to Environment struct sections (daemon, log, tcp, bgp, cache, api, reactor, debug).
func environmentBlock() *ContainerNode {
	return Container(
		Field("daemon", Container(
			Field("pid", Leaf(TypeString)),
			Field("user", Leaf(TypeString)),
			Field("daemonize", Leaf(TypeBool)),
			Field("drop", Leaf(TypeBool)),
			Field("umask", Leaf(TypeString)), // Octal string
		)),
		Field("log", Container(
			Field("level", Leaf(TypeString)),
			Field("enable", Leaf(TypeBool)),
			Field("destination", Leaf(TypeString)),
			Field("all", Leaf(TypeBool)),
			Field("configuration", Leaf(TypeBool)),
			Field("reactor", Leaf(TypeBool)),
			Field("daemon", Leaf(TypeBool)),
			Field("processes", Leaf(TypeBool)),
			Field("network", Leaf(TypeBool)),
			Field("statistics", Leaf(TypeBool)),
			Field("packets", Leaf(TypeBool)),
			Field("rib", Leaf(TypeBool)),
			Field("message", Leaf(TypeBool)),
			Field("timers", Leaf(TypeBool)),
			Field("routes", Leaf(TypeBool)),
			Field("parser", Leaf(TypeBool)),
			Field("short", Leaf(TypeBool)),
		)),
		Field("tcp", Container(
			Field("port", Leaf(TypeUint16)),
			Field("attempts", Leaf(TypeInt)),
			Field("delay", Leaf(TypeInt)),
			Field("acl", Leaf(TypeBool)),
		)),
		Field("bgp", Container(
			Field("passive", Leaf(TypeBool)),
			Field("openwait", Leaf(TypeInt)),
		)),
		Field("cache", Container(
			Field("attributes", Leaf(TypeBool)),
		)),
		Field("api", Container(
			Field("ack", Leaf(TypeBool)),
			Field("chunk", Leaf(TypeInt)),
			Field("encoder", Leaf(TypeString)),
			Field("compact", Leaf(TypeBool)),
			Field("respawn", Leaf(TypeBool)),
			Field("terminate", Leaf(TypeBool)),
			Field("cli", Leaf(TypeBool)),
			Field("pipename", Leaf(TypeString)),
			Field("socketname", Leaf(TypeString)),
		)),
		Field("reactor", Container(
			Field("speed", Leaf(TypeString)), // Float as string
		)),
		Field("debug", Container(
			Field("pdb", Leaf(TypeBool)),
			Field("memory", Leaf(TypeBool)),
			Field("configuration", Leaf(TypeBool)),
			Field("selfcheck", Leaf(TypeBool)),
			Field("route", Leaf(TypeString)),
			Field("defensive", Leaf(TypeBool)),
			Field("rotate", Leaf(TypeBool)),
			Field("timing", Leaf(TypeBool)),
		)),
	)
}

// LegacyBGPSchema returns a schema that accepts both old and current syntax.
// Used by the migration tool to parse old configs before transformation.
func LegacyBGPSchema() *Schema {
	schema := NewSchema()

	// Environment settings (ZeBGP-specific)
	schema.Define("environment", environmentBlock())

	// Global settings
	schema.Define("router-id", Leaf(TypeIPv4))
	schema.Define("local-as", Leaf(TypeUint32))
	schema.Define("listen", MultiLeaf(TypeString))

	// Process definitions (API)
	schema.Define("process", List(TypeString,
		Field("run", MultiLeaf(TypeString)),
		Field("encoder", Leaf(TypeString)),
		Field("respawn", Leaf(TypeBool)),
	))

	// neighbor <IP> { ... } - deprecated ExaBGP syntax
	schema.Define("neighbor", List(TypeIP, peerFields()...))

	// peer - accepts both IP (current) and glob patterns (old)
	schema.Define("peer", List(TypeString, peerFields()...))

	// Template definitions - accepts both old and current syntax
	schema.Define("template", Container(
		Field("neighbor", List(TypeString, peerFields()...)), // old: named templates
		Field("group", List(TypeString, peerFields()...)),    // current: named templates
		Field("match", List(TypeString, peerFields()...)),    // current: glob patterns
	))

	return schema
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
	RouterID  uint32
	LocalAS   uint32
	Listen    string
	Peers     []PeerConfig
	Processes []ProcessConfig
	ConfigDir string                       // Directory containing config file (set by LoadReactorFile)
	EnvValues map[string]map[string]string // Environment block values (ZeBGP-specific)
}

// PeerConfig holds neighbor configuration.
type PeerConfig struct {
	Address              netip.Addr
	Description          string
	RouterID             uint32
	LocalAddress         netip.Addr
	LocalAddressAuto     bool // true when local-address is "auto"
	LocalAS              uint32
	PeerAS               uint32
	HoldTime             uint16
	Passive              bool
	GroupUpdates         bool           // DEPRECATED: Use RIBOut.GroupUpdates
	RIBOut               RIBOutConfig   // Per-neighbor outgoing RIB config
	Families             []string       // Legacy: "ipv4/unicast", "ipv6/unicast" etc.
	FamilyConfigs        []FamilyConfig // New: structured family config with mode
	IgnoreFamilyMismatch bool           // Ignore NLRI for non-negotiated AFI/SAFI instead of error
	Hostname             string
	DomainName           string
	Capabilities         CapabilityConfig
	AddPathFamilies      []AddPathFamilyConfig // Per-family add-path settings (RFC 7911)
	NexthopFamilies      []NexthopFamilyConfig // RFC 8950 Extended Next Hop families
	StaticRoutes         []StaticRouteConfig
	MVPNRoutes           []MVPNRouteConfig
	VPLSRoutes           []VPLSRouteConfig
	FlowSpecRoutes       []FlowSpecRouteConfig
	MUPRoutes            []MUPRouteConfig
	APIBindings          []PeerAPIBinding // Per-peer API process bindings
}

// PeerAPIBinding binds a peer to an API process with specific content and message config.
// This separates WHAT messages to send/receive from HOW they are formatted.
type PeerAPIBinding struct {
	ProcessName string            // Reference to process name (must exist)
	Content     PeerContentConfig // HOW: encoding and format
	Receive     PeerReceiveConfig // WHAT: which message types to receive
	Send        PeerSendConfig    // WHAT: which message types to send
}

// PeerContentConfig controls message formatting (encoding + format).
type PeerContentConfig struct {
	Encoding   string               // "json" | "text" (empty = inherit from process)
	Format     string               // "parsed" | "raw" | "full" (empty = "parsed")
	Attributes *api.AttributeFilter // Which attrs to include (nil = all)
	NLRI       *api.NLRIFilter      // Which families to include (nil = all)
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
	Nexthop         bool // RFC 8950 Extended Next Hop Encoding
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
	WatchdogWithdraw bool   // Start in withdrawn state (held until "announce watchdog")
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
type FlowSpecRouteConfig struct {
	Name              string
	IsIPv6            bool
	RD                string // for flow-vpn
	Match             map[string]string
	Then              map[string]string
	NextHop           string
	ExtendedCommunity string
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

// ProcessConfig holds process configuration.
type ProcessConfig struct {
	Name          string
	Run           string
	Encoder       string
	ReceiveUpdate bool // Forward received UPDATEs to process stdin
}

// TreeToConfig converts a parsed tree to a typed BGPConfig.
func TreeToConfig(tree *Tree) (*BGPConfig, error) {
	cfg := &BGPConfig{}

	// Global settings
	if v, ok := tree.Get("router-id"); ok {
		ip, err := netip.ParseAddr(v)
		if err != nil {
			return nil, fmt.Errorf("invalid router-id: %w", err)
		}
		cfg.RouterID = ipToUint32(ip)
	}

	if v, ok := tree.Get("local-as"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid local-as: %w", err)
		}
		cfg.LocalAS = uint32(n)
	}

	if v, ok := tree.Get("listen"); ok {
		cfg.Listen = v
	}

	// Processes
	for name, proc := range tree.GetList("process") {
		// Reject reserved names (underscore prefix used internally)
		if strings.HasPrefix(name, "_") {
			return nil, fmt.Errorf("process name %q: names starting with underscore are reserved", name)
		}
		pc := ProcessConfig{Name: name}
		if v, ok := proc.Get("run"); ok {
			pc.Run = v
		}
		if v, ok := proc.Get("encoder"); ok {
			pc.Encoder = v
		}
		// Default: text encoder processes receive updates
		// TODO: Parse api { receive { update; } } from peer/template for proper config
		if pc.Encoder == encoderText {
			pc.ReceiveUpdate = true
		}
		cfg.Processes = append(cfg.Processes, pc)
	}

	// Parse templates first (for inheritance)
	// Support both old (template.neighbor) and current (template.group) syntax
	templates := make(map[string]*Tree)
	if tmpl := tree.GetContainer("template"); tmpl != nil {
		// Old: template { neighbor <name> { ... } }
		for name, neighborTree := range tmpl.GetList("neighbor") {
			// Validate: inherit not allowed inside template
			if _, hasInherit := neighborTree.Get("inherit"); hasInherit {
				return nil, fmt.Errorf("inherit only valid inside peer { }, not in template.neighbor %s", name)
			}
			templates[name] = neighborTree
		}
		// Current: template { group <name> { ... } }
		for name, groupTree := range tmpl.GetList("group") {
			// Validate group name: must start with letter, contain letters/numbers/hyphens, not end with hyphen
			if !isValidGroupName(name) {
				return nil, fmt.Errorf("invalid group name %q: must start with letter, end with letter/number, contain only letters/numbers/hyphens", name)
			}
			// Validate: inherit not allowed inside template
			if _, hasInherit := groupTree.Get("inherit"); hasInherit {
				return nil, fmt.Errorf("inherit only valid inside peer { }, not in template.group %s", name)
			}
			templates[name] = groupTree
		}
		// Validate: inherit not allowed inside template.match
		for _, entry := range tmpl.GetListOrdered("match") {
			if _, hasInherit := entry.Value.Get("inherit"); hasInherit {
				return nil, fmt.Errorf("inherit only valid inside peer { }, not in template.match %s", entry.Key)
			}
		}
	}

	// Parse peer globs from root level (old: peer *) and template.match (current)
	// Old: peer * { ... } at root - sorted by specificity (backward compat)
	// Current: template { match * { ... } } - applied in CONFIG ORDER (not specificity!)
	//
	// Application order: old globs (sorted) → current matches (config order)

	// Old: Root-level peer globs (patterns only, not IPs) - SORTED by specificity
	oldPeerGlobs := make([]PeerGlob, 0)
	peerList := tree.GetList("peer")
	for pattern, peerTree := range peerList {
		if isGlobPattern(pattern) {
			oldPeerGlobs = append(oldPeerGlobs, PeerGlob{
				Pattern:     pattern,
				Specificity: peerGlobSpecificity(pattern),
				Tree:        peerTree,
			})
		}
	}
	// Sort old globs by specificity (least specific first)
	sortPeerGlobs(oldPeerGlobs)

	// Current: template { match ... } - CONFIG ORDER (not sorted!)
	matchBlocks := make([]PeerGlob, 0)
	if tmpl := tree.GetContainer("template"); tmpl != nil {
		for _, entry := range tmpl.GetListOrdered("match") {
			matchBlocks = append(matchBlocks, PeerGlob{
				Pattern:     entry.Key,
				Specificity: 0, // Not used for current syntax
				Tree:        entry.Value,
			})
		}
	}

	// Combined: old globs first (sorted), then current matches (config order)
	peerGlobs := make([]PeerGlob, 0, len(oldPeerGlobs)+len(matchBlocks))
	peerGlobs = append(peerGlobs, oldPeerGlobs...)
	peerGlobs = append(peerGlobs, matchBlocks...)

	// Neighbors from "neighbor" (old) and "peer" with IP address (current)
	for addr, n := range tree.GetList("neighbor") {
		nc, err := parsePeerConfig(addr, n, templates, peerGlobs)
		if err != nil {
			return nil, fmt.Errorf("neighbor %s: %w", addr, err)
		}
		cfg.Peers = append(cfg.Peers, nc)
	}

	// Build set of defined process names for validation
	processNames := make(map[string]bool)
	for _, p := range cfg.Processes {
		processNames[p.Name] = true
	}

	// V3: peer <IP> { ... } - parse IPs as neighbors
	for addr, n := range tree.GetList("peer") {
		if !isGlobPattern(addr) {
			// It's an IP address, treat as neighbor
			nc, err := parsePeerConfig(addr, n, templates, peerGlobs)
			if err != nil {
				return nil, fmt.Errorf("peer %s: %w", addr, err)
			}

			// Validate API binding process references
			for _, binding := range nc.APIBindings {
				if !processNames[binding.ProcessName] {
					return nil, fmt.Errorf("peer %s: undefined process %q in api binding", addr, binding.ProcessName)
				}
			}

			cfg.Peers = append(cfg.Peers, nc)
		}
	}

	return cfg, nil
}

// sortPeerGlobs sorts peer globs by specificity (ascending).
// Least specific first so more specific patterns override later.
func sortPeerGlobs(globs []PeerGlob) {
	for i := 0; i < len(globs)-1; i++ {
		for j := i + 1; j < len(globs); j++ {
			if globs[i].Specificity > globs[j].Specificity {
				globs[i], globs[j] = globs[j], globs[i]
			}
		}
	}
}

// applyTreeSettings applies settings from a tree (match block, template, or peer glob)
// to a PeerConfig. Only explicitly set values are applied.
func applyTreeSettings(nc *PeerConfig, tree *Tree) error {
	// Hold time
	if v, ok := tree.Get("hold-time"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return fmt.Errorf("invalid hold-time: %w", err)
		}
		// RFC 4271 Section 4.2: Hold Time MUST be either zero or at least three seconds
		if n >= 1 && n <= 2 {
			return fmt.Errorf("invalid hold-time %d: RFC 4271 requires 0 or >= 3 seconds", n)
		}
		nc.HoldTime = uint16(n)
	}

	// Peer AS
	if v, ok := tree.Get("peer-as"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid peer-as: %w", err)
		}
		nc.PeerAS = uint32(n)
	}

	// Local AS
	if v, ok := tree.Get("local-as"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid local-as: %w", err)
		}
		nc.LocalAS = uint32(n)
	}

	// Description
	if v, ok := tree.Get("description"); ok {
		nc.Description = v
	}

	// Router ID
	if v, ok := tree.Get("router-id"); ok {
		ip, err := netip.ParseAddr(v)
		if err != nil {
			return fmt.Errorf("invalid router-id: %w", err)
		}
		nc.RouterID = ipToUint32(ip)
	}

	// Local address
	if v, ok := tree.Get("local-address"); ok {
		if v == "auto" {
			nc.LocalAddressAuto = true
		} else {
			ip, err := netip.ParseAddr(v)
			if err != nil {
				return fmt.Errorf("invalid local-address: %w", err)
			}
			nc.LocalAddress = ip
		}
	}

	// Passive
	if v, ok := tree.Get("passive"); ok {
		nc.Passive = v == configTrue
	}

	// Group updates
	if v, ok := tree.Get("group-updates"); ok {
		nc.GroupUpdates = v == configTrue
		nc.RIBOut.GroupUpdates = v == configTrue
	}

	// RIBOut config
	if ribOut, err := parseRIBOutConfig(tree); err != nil {
		return fmt.Errorf("rib: %w", err)
	} else {
		applyRIBOutParseResult(&nc.RIBOut, ribOut)
	}

	// Capabilities
	if cap := tree.GetContainer("capability"); cap != nil {
		if v, ok := cap.Get("asn4"); ok {
			nc.Capabilities.ASN4 = v == configTrue
		}
		// route-refresh is Flex type, use GetFlex
		if v, ok := cap.GetFlex("route-refresh"); ok {
			nc.Capabilities.RouteRefresh = v == configTrue || v == configEnable
		}
		if gr := cap.GetContainer("graceful-restart"); gr != nil {
			nc.Capabilities.GracefulRestart = true
			if v, ok := gr.Get("restart-time"); ok {
				n, _ := strconv.ParseUint(v, 10, 16)
				nc.Capabilities.RestartTime = uint16(n)
			}
		}
		// Handle add-path as value (e.g., "add-path send/receive;")
		if v, ok := cap.GetFlex("add-path"); ok && v != "" {
			switch v {
			case addPathSendReceive, addPathReceiveSend:
				nc.Capabilities.AddPathSend = true
				nc.Capabilities.AddPathReceive = true
			case addPathSend:
				nc.Capabilities.AddPathSend = true
			case addPathReceive:
				nc.Capabilities.AddPathReceive = true
			}
		}
		// Handle add-path as block (e.g., "add-path { send; receive; }")
		if ap := cap.GetContainer("add-path"); ap != nil {
			if v, ok := ap.Get(addPathSend); ok {
				nc.Capabilities.AddPathSend = v == configTrue
			}
			if v, ok := ap.Get(addPathReceive); ok {
				nc.Capabilities.AddPathReceive = v == configTrue
			}
		}
		if v, ok := cap.GetFlex("extended-message"); ok {
			nc.Capabilities.ExtendedMessage = v == configTrue || v == configEnable
		}
		if v, ok := cap.GetFlex("software-version"); ok {
			nc.Capabilities.SoftwareVersion = v == configTrue || v == configEnable
		}
		// RFC 8950: Extended Next Hop Encoding capability
		if v, ok := cap.GetFlex("nexthop"); ok {
			nc.Capabilities.Nexthop = v == configTrue || v == configEnable
		}
	}

	// Parse nexthop { ... } block for extended next-hop families
	// Format: nexthop { ipv4/unicast ipv6; ipv4/mpls-vpn ipv6; }
	if nhBlock := tree.GetContainer("nexthop"); nhBlock != nil {
		nc.NexthopFamilies = parseNexthopFamilies(nhBlock)
	}

	return nil
}

func parsePeerConfig(addr string, tree *Tree, templates map[string]*Tree, peerGlobs []PeerGlob) (PeerConfig, error) {
	nc := PeerConfig{}

	// Set default capability values (ASN4 enabled by default per RFC 6793).
	nc.Capabilities.ASN4 = true

	// RIBOut defaults - set early so peer globs can override
	nc.RIBOut = DefaultRIBOutConfig()

	// Address
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return nc, fmt.Errorf("invalid address: %w", err)
	}
	nc.Address = ip

	// Build precedence chain: [match blocks] -> [inherited templates] -> [peer config]
	// Each layer can override settings from previous layers

	// Layer 1: Apply matching peer globs / template.match blocks (in order)
	// This sets defaults that can be overridden by templates and neighbor config
	// Collect matching trees for reuse when processing API bindings later
	matchingTrees := make([]*Tree, 0, len(peerGlobs))
	for _, glob := range peerGlobs {
		if IPGlobMatch(glob.Pattern, addr) {
			matchingTrees = append(matchingTrees, glob.Tree)
			// Apply all settings from this match block
			if err := applyTreeSettings(&nc, glob.Tree); err != nil {
				return nc, fmt.Errorf("match %s: %w", glob.Pattern, err)
			}
			// Extract routes from match block
			routes, err := extractRoutesFromTree(glob.Tree)
			if err != nil {
				return nc, fmt.Errorf("match %s routes: %w", glob.Pattern, err)
			}
			nc.StaticRoutes = append(nc.StaticRoutes, routes...)
		}
	}

	// Layer 2: Handle template inheritance (multiple inherit supported)
	// V3 supports multiple inherit statements, applied in order
	inheritedTemplates := make([]*Tree, 0)
	for _, entry := range tree.GetListOrdered("inherit") {
		// inherit is stored as a leaf value, key is the template name
		inheritName := entry.Key
		if t, exists := templates[inheritName]; exists {
			inheritedTemplates = append(inheritedTemplates, t)
		}
	}
	// Also check single inherit value (ExaBGP compatibility)
	if len(inheritedTemplates) == 0 {
		if inheritName, ok := tree.Get("inherit"); ok {
			if t, exists := templates[inheritName]; exists {
				inheritedTemplates = append(inheritedTemplates, t)
			}
		}
	}

	// Apply inherited templates in order
	for _, tmpl := range inheritedTemplates {
		if err := applyTreeSettings(&nc, tmpl); err != nil {
			return nc, fmt.Errorf("template: %w", err)
		}
		routes, err := extractRoutesFromTree(tmpl)
		if err != nil {
			return nc, fmt.Errorf("template routes: %w", err)
		}
		nc.StaticRoutes = append(nc.StaticRoutes, routes...)
	}

	// Get last inherited template for getValue fallback (backward compat)
	var tmpl *Tree
	if len(inheritedTemplates) > 0 {
		tmpl = inheritedTemplates[len(inheritedTemplates)-1]
	}

	// Helper to get value from neighbor tree, falling back to template.
	getValue := func(key string) (string, bool) {
		if v, ok := tree.Get(key); ok {
			return v, true
		}
		if tmpl != nil {
			return tmpl.Get(key)
		}
		return "", false
	}

	// Simple fields (check template fallback for each)
	if v, ok := getValue("description"); ok {
		nc.Description = v
	}

	if v, ok := getValue("router-id"); ok {
		ip, err := netip.ParseAddr(v)
		if err != nil {
			return nc, fmt.Errorf("invalid router-id: %w", err)
		}
		nc.RouterID = ipToUint32(ip)
	}

	if v, ok := getValue("local-address"); ok {
		if v == "auto" {
			nc.LocalAddressAuto = true
		} else {
			ip, err := netip.ParseAddr(v)
			if err != nil {
				return nc, fmt.Errorf("invalid local-address: %w", err)
			}
			nc.LocalAddress = ip
		}
	}

	if v, ok := getValue("local-as"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nc, fmt.Errorf("invalid local-as: %w", err)
		}
		nc.LocalAS = uint32(n)
	}

	if v, ok := getValue("peer-as"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nc, fmt.Errorf("invalid peer-as: %w", err)
		}
		nc.PeerAS = uint32(n)
	}

	if v, ok := getValue("hold-time"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return nc, fmt.Errorf("invalid hold-time: %w", err)
		}
		// RFC 4271 Section 4.2: Hold Time MUST be either zero or at least three seconds
		if n >= 1 && n <= 2 {
			return nc, fmt.Errorf("invalid hold-time %d: RFC 4271 requires 0 or >= 3 seconds", n)
		}
		nc.HoldTime = uint16(n)
	}

	if v, ok := getValue("passive"); ok {
		nc.Passive = v == configTrue
	}

	// group-updates defaults to true, check template first then neighbor
	nc.GroupUpdates = true
	if tmpl != nil {
		if v, ok := tmpl.Get("group-updates"); ok {
			nc.GroupUpdates = v == configTrue
		}
	}
	if v, ok := tree.Get("group-updates"); ok {
		nc.GroupUpdates = v == configTrue
	}

	if v, ok := tree.Get("host-name"); ok {
		nc.Hostname = v
	}

	if v, ok := tree.Get("domain-name"); ok {
		nc.DomainName = v
	}

	// Families - FamilyBlock stores "ipv4/unicast" as key with mode as value
	// Also parse ignore-mismatch option from family block
	// Check template first, then override with neighbor values
	familyTree := tree.GetContainer("family")
	if familyTree == nil && tmpl != nil {
		familyTree = tmpl.GetContainer("family")
	}
	if familyTree != nil {
		for _, key := range familyTree.Values() {
			if strings.HasPrefix(key, "ignore-mismatch") {
				// Parse ignore-mismatch option (not a family)
				// Format: "ignore-mismatch [enable|true|disable|false]"
				parts := strings.Fields(key)
				if len(parts) == 2 {
					nc.IgnoreFamilyMismatch = parts[1] == configTrue || parts[1] == configEnable
				} else if len(parts) == 1 {
					// Just "ignore-mismatch" alone means enable
					nc.IgnoreFamilyMismatch = true
				}
			} else {
				// Regular address family - key is "AFI/SAFI", value is mode
				modeStr, _ := familyTree.Get(key)
				mode := ParseFamilyMode(modeStr)

				// Parse AFI and SAFI from key (format: afi/safi)
				parts := strings.SplitN(key, "/", 2)
				if len(parts) == 2 {
					fc := FamilyConfig{
						AFI:  parts[0],
						SAFI: parts[1],
						Mode: mode,
					}
					nc.FamilyConfigs = append(nc.FamilyConfigs, fc)

					// Also populate legacy Families for backward compatibility
					// (only for enabled families)
					if mode != FamilyModeDisable {
						nc.Families = append(nc.Families, key)
					}
				}
			}
		}
	}

	// Capabilities - check template first, then override with neighbor values
	cap := tree.GetContainer("capability")
	if cap == nil && tmpl != nil {
		cap = tmpl.GetContainer("capability")
	}
	if cap != nil {
		if v, ok := cap.Get("asn4"); ok {
			nc.Capabilities.ASN4 = v == configTrue
		}
		if v, ok := cap.Get("route-refresh"); ok {
			nc.Capabilities.RouteRefresh = v == configTrue
		}
		if gr := cap.GetContainer("graceful-restart"); gr != nil {
			nc.Capabilities.GracefulRestart = true
			if v, ok := gr.Get("restart-time"); ok {
				n, _ := strconv.ParseUint(v, 10, 16)
				nc.Capabilities.RestartTime = uint16(n)
			}
		}
		// Handle add-path as value (e.g., "add-path send/receive;")
		if v, ok := cap.GetFlex("add-path"); ok && v != "" {
			switch v {
			case addPathSendReceive, addPathReceiveSend:
				nc.Capabilities.AddPathSend = true
				nc.Capabilities.AddPathReceive = true
			case addPathSend:
				nc.Capabilities.AddPathSend = true
			case addPathReceive:
				nc.Capabilities.AddPathReceive = true
			}
		}
		// Handle add-path as block (e.g., "add-path { send; receive; }")
		if ap := cap.GetContainer("add-path"); ap != nil {
			if v, ok := ap.Get(addPathSend); ok {
				nc.Capabilities.AddPathSend = v == configTrue
			}
			if v, ok := ap.Get(addPathReceive); ok {
				nc.Capabilities.AddPathReceive = v == configTrue
			}
		}
		// RFC 8654 Extended Message capability
		if v, ok := cap.GetFlex("extended-message"); ok {
			nc.Capabilities.ExtendedMessage = v == configTrue || v == configEnable
		}
		if v, ok := cap.GetFlex("software-version"); ok {
			nc.Capabilities.SoftwareVersion = v == configTrue || v == configEnable
		}
		// RFC 8950: Extended Next Hop Encoding capability
		if v, ok := cap.GetFlex("nexthop"); ok {
			nc.Capabilities.Nexthop = v == configTrue || v == configEnable
		}
	}

	// Parse nexthop { ... } block for extended next-hop families
	// Format: nexthop { ipv4/unicast ipv6; ipv4/mpls-vpn ipv6; }
	if nhBlock := tree.GetContainer("nexthop"); nhBlock != nil {
		nc.NexthopFamilies = parseNexthopFamilies(nhBlock)
	}

	// Per-family add-path configuration (RFC 7911)
	// Format: add-path { ipv4/unicast send; ipv6/unicast receive; ipv4/multicast send/receive; }
	if addPath := tree.GetContainer("add-path"); addPath != nil {
		for _, key := range addPath.Values() {
			apf := parseAddPathFamily(key)
			if apf.Family != "" {
				nc.AddPathFamilies = append(nc.AddPathFamilies, apf)
			}
		}
	}

	// Extract routes from this neighbor's static and announce blocks
	routes, err := extractRoutesFromTree(tree)
	if err != nil {
		return nc, err
	}
	nc.StaticRoutes = append(nc.StaticRoutes, routes...)

	// Extract exotic routes
	nc.MVPNRoutes = extractMVPNRoutes(tree)
	nc.VPLSRoutes = extractVPLSRoutes(tree)
	nc.FlowSpecRoutes = extractFlowSpecRoutes(tree)
	nc.MUPRoutes = extractMUPRoutes(tree)

	// Apply template rib.out if present (peer globs already applied above)
	if tmpl != nil {
		if ribOut, err := parseRIBOutConfig(tmpl); err != nil {
			return nc, fmt.Errorf("template rib: %w", err)
		} else {
			applyRIBOutParseResult(&nc.RIBOut, ribOut)
		}
	}

	// Apply neighbor rib.out (overrides template)
	if ribOut, err := parseRIBOutConfig(tree); err != nil {
		return nc, fmt.Errorf("rib: %w", err)
	} else {
		applyRIBOutParseResult(&nc.RIBOut, ribOut)
	}

	// Sync legacy group-updates to RIBOut (if explicit)
	// Legacy neighbor group-updates takes final precedence for backward compat
	if v, ok := tree.Get("group-updates"); ok {
		nc.RIBOut.GroupUpdates = v == configTrue
	} else if tmpl != nil {
		if v, ok := tmpl.Get("group-updates"); ok {
			nc.RIBOut.GroupUpdates = v == configTrue
		}
	}

	// Parse API bindings - supports both old and new syntax:
	// Old: api { processes [ foo bar ]; receive { ... } }
	// New: api <process-name> { content { encoding json; } receive { update; } }
	//
	// Precedence: match templates → inherited templates → peer config
	// Each layer can override the previous.

	// Layer 1: Match templates (collected earlier in matchingTrees)
	for _, matchTree := range matchingTrees {
		matchBindings, err := parseAPIBindings(matchTree)
		if err != nil {
			return PeerConfig{}, fmt.Errorf("peer %s: %w", addr, err)
		}
		nc.APIBindings = mergeAPIBindings(nc.APIBindings, matchBindings)
	}

	// Layer 2: Inherited templates (later templates override earlier ones)
	for _, tmpl := range inheritedTemplates {
		tmplBindings, err := parseAPIBindings(tmpl)
		if err != nil {
			return PeerConfig{}, fmt.Errorf("peer %s: %w", addr, err)
		}
		nc.APIBindings = mergeAPIBindings(nc.APIBindings, tmplBindings)
	}

	// Layer 3: Peer bindings override all templates
	peerBindings, err := parseAPIBindings(tree)
	if err != nil {
		return PeerConfig{}, fmt.Errorf("peer %s: %w", addr, err)
	}
	nc.APIBindings = mergeAPIBindings(nc.APIBindings, peerBindings)

	return nc, nil
}

// ribOutParseResult holds parsed values with explicit "was set" tracking.
type ribOutParseResult struct {
	GroupUpdates       bool
	GroupUpdatesSet    bool
	AutoCommitDelay    time.Duration
	AutoCommitDelaySet bool
	MaxBatchSize       int
	MaxBatchSizeSet    bool
}

// parseRIBOutConfig extracts RIBOut settings from a tree's rib.out block.
// Returns a parse result that tracks which fields were explicitly set.
func parseRIBOutConfig(tree *Tree) (ribOutParseResult, error) {
	result := ribOutParseResult{}

	rib := tree.GetContainer("rib")
	if rib == nil {
		return result, nil
	}

	ribOut := rib.GetContainer("out")
	if ribOut == nil {
		return result, nil
	}

	if v, ok := ribOut.Get("group-updates"); ok {
		result.GroupUpdates = v == configTrue
		result.GroupUpdatesSet = true
	}
	if v, ok := ribOut.Get("auto-commit-delay"); ok {
		d, err := parseDurationValue(v)
		if err != nil {
			return result, fmt.Errorf("auto-commit-delay: %w", err)
		}
		result.AutoCommitDelay = d
		result.AutoCommitDelaySet = true
	}
	if v, ok := ribOut.Get("max-batch-size"); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return result, fmt.Errorf("max-batch-size: %w", err)
		}
		result.MaxBatchSize = n
		result.MaxBatchSizeSet = true
	}

	return result, nil
}

// applyRIBOutParseResult applies parsed values to a config, only overriding
// fields that were explicitly set.
func applyRIBOutParseResult(cfg *RIBOutConfig, parsed ribOutParseResult) {
	if parsed.GroupUpdatesSet {
		cfg.GroupUpdates = parsed.GroupUpdates
	}
	if parsed.AutoCommitDelaySet {
		cfg.AutoCommitDelay = parsed.AutoCommitDelay
	}
	if parsed.MaxBatchSizeSet {
		cfg.MaxBatchSize = parsed.MaxBatchSize
	}
}

// parseAPIBindings parses API bindings from a peer tree.
// Supports both old and new syntax:
//   - Old: api { processes [ foo bar ]; } - uses KeyDefault
//   - New: api <process-name> { content { encoding json; } receive { update; } }
func parseAPIBindings(tree *Tree) ([]PeerAPIBinding, error) {
	var bindings []PeerAPIBinding

	// Schema defines api as List(TypeString, ...) - use GetList
	apiList := tree.GetList("api")
	if len(apiList) == 0 {
		return nil, nil
	}

	// Sort keys for deterministic order (maps iterate randomly)
	keys := make([]string, 0, len(apiList))
	for k := range apiList {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		apiTree := apiList[key]
		if key == KeyDefault {
			// Old syntax: api { processes [ foo bar ]; }
			bindings = append(bindings, parseOldAPIBindings(apiTree)...)
		} else {
			// New syntax: api <process-name> { content {...} receive {...} send {...} }
			binding, err := parseNewAPIBinding(key, apiTree)
			if err != nil {
				return nil, err
			}
			bindings = append(bindings, binding)
		}
	}

	return bindings, nil
}

// parseOldAPIBindings parses the old api { processes [...] } syntax.
// Also handles neighbor-changes flag which maps to receive.State.
func parseOldAPIBindings(apiTree *Tree) []PeerAPIBinding {
	var bindings []PeerAPIBinding

	// Check for neighbor-changes flag (maps to receive.State)
	// Flex stores "neighbor-changes;" as GetFlex returning "true" or "enable"
	neighborChanges := false
	if v, ok := apiTree.GetFlex("neighbor-changes"); ok {
		neighborChanges = v == configTrue || v == configEnable || v == ""
	}

	// Look for "processes" key with array value like "[ foo bar ]"
	if processesValue, ok := apiTree.Get("processes"); ok {
		// Parse process names from "[ foo bar ]" or "foo bar" format
		processesValue = strings.Trim(processesValue, "[]")
		for _, processName := range strings.Fields(processesValue) {
			binding := PeerAPIBinding{ProcessName: processName}
			if neighborChanges {
				binding.Receive.State = true
			}
			bindings = append(bindings, binding)
		}
	}

	return bindings
}

// parseNewAPIBinding parses a single api <name> { ... } binding.
func parseNewAPIBinding(processName string, apiTree *Tree) (PeerAPIBinding, error) {
	binding := PeerAPIBinding{ProcessName: processName}

	// Parse content block: content { encoding json; format full; attribute ...; nlri ...; }
	if content := apiTree.GetContainer("content"); content != nil {
		if v, ok := content.Get("encoding"); ok {
			binding.Content.Encoding = strings.ToLower(v) // Normalize case
		}
		if v, ok := content.Get("format"); ok {
			binding.Content.Format = strings.ToLower(v) // Normalize case
		}
		if v, ok := content.Get("attribute"); ok {
			filter, err := api.ParseAttributeFilter(v)
			if err != nil {
				return PeerAPIBinding{}, fmt.Errorf("api %s: invalid attribute filter: %w", processName, err)
			}
			binding.Content.Attributes = &filter
		}
		// Parse nlri entries: nlri ipv4/unicast; nlri ipv6/unicast;
		if nlriEntries := content.GetMultiValues("nlri"); len(nlriEntries) > 0 {
			filter, err := parseNLRIEntries(nlriEntries)
			if err != nil {
				return PeerAPIBinding{}, fmt.Errorf("api %s: invalid nlri filter: %w", processName, err)
			}
			binding.Content.NLRI = &filter
		}
	}

	// Parse receive block: receive { update; notification; all; }
	if recv := apiTree.GetContainer("receive"); recv != nil {
		binding.Receive = parseReceiveConfig(recv)
	}

	// Parse send block: send { update; refresh; all; }
	if send := apiTree.GetContainer("send"); send != nil {
		binding.Send = parseSendConfig(send)
	}

	return binding, nil
}

// parseNLRIEntries parses multiple "nlri <afi> <safi>;" entries into NLRIFilter.
// Each entry is a space-separated string like "ipv4/unicast" or "ipv6/unicast".
// Special values: "all" includes all families, "none" excludes all.
func parseNLRIEntries(entries []string) (api.NLRIFilter, error) {
	if len(entries) == 0 {
		return api.NewNLRIFilterAll(), nil
	}

	// Check for special keywords
	if len(entries) == 1 {
		entry := strings.TrimSpace(strings.ToLower(entries[0]))
		if entry == "all" {
			return api.NewNLRIFilterAll(), nil
		}
		if entry == "none" {
			return api.NewNLRIFilterNone(), nil
		}
	}

	// Parse each entry as "<afi> <safi>" and convert to hyphenated form
	families := make(map[string]bool, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(strings.ToLower(entry))
		if entry == "" {
			continue
		}

		// Validate against known families (format: afi/safi)
		canonical, ok := message.FamilyConfigNames[strings.ToLower(entry)]
		if !ok {
			return api.NLRIFilter{}, fmt.Errorf("unknown family %q, valid: %s",
				entry, message.ValidFamilyConfigNames())
		}
		families[canonical] = true
	}

	return api.NewNLRIFilterSelective(families), nil
}

// parseReceiveConfig parses a Freeform receive block.
// Freeform stores "update;" as key "update" -> value "true".
func parseReceiveConfig(tree *Tree) PeerReceiveConfig {
	cfg := PeerReceiveConfig{}

	// Check for "all" shorthand - sets all flags
	if _, ok := tree.Get("all"); ok {
		cfg.Update = true
		cfg.Open = true
		cfg.Notification = true
		cfg.Keepalive = true
		cfg.Refresh = true
		cfg.State = true
		cfg.Sent = true
		return cfg
	}

	// Individual flags
	_, cfg.Update = tree.Get("update")
	_, cfg.Open = tree.Get("open")
	_, cfg.Notification = tree.Get("notification")
	_, cfg.Keepalive = tree.Get("keepalive")
	_, cfg.Refresh = tree.Get("refresh")
	_, cfg.State = tree.Get("state")
	_, cfg.Sent = tree.Get("sent")

	return cfg
}

// parseSendConfig parses a Freeform send block.
func parseSendConfig(tree *Tree) PeerSendConfig {
	cfg := PeerSendConfig{}

	// Check for "all" shorthand
	if _, ok := tree.Get("all"); ok {
		cfg.Update = true
		cfg.Refresh = true
		return cfg
	}

	// Individual flags
	_, cfg.Update = tree.Get("update")
	_, cfg.Refresh = tree.Get("refresh")

	return cfg
}

// mergeAPIBindings merges new bindings into existing bindings.
// Bindings with the same process name are replaced (new overrides existing).
// Bindings with different process names are appended.
func mergeAPIBindings(existing, new []PeerAPIBinding) []PeerAPIBinding {
	if len(new) == 0 {
		return existing
	}
	if len(existing) == 0 {
		return new
	}

	// Build map of existing bindings by process name
	result := make([]PeerAPIBinding, 0, len(existing)+len(new))
	seen := make(map[string]int) // process name -> index in result

	// Add existing bindings
	for _, b := range existing {
		seen[b.ProcessName] = len(result)
		result = append(result, b)
	}

	// Merge new bindings (replace or append)
	for _, b := range new {
		if idx, exists := seen[b.ProcessName]; exists {
			// Replace existing binding
			result[idx] = b
		} else {
			// Append new binding
			seen[b.ProcessName] = len(result)
			result = append(result, b)
		}
	}

	return result
}

// parseNexthopFamilies parses the nexthop { ... } block for RFC 8950 extended next-hop.
// Format: nexthop { ipv4/unicast ipv6; ipv4/mpls-vpn ipv6; ipv6/unicast ipv4; }
// Each entry maps (NLRI AFI, NLRI SAFI) -> NextHop AFI.
// Freeform stores ALL words as a single key with value "true".
// So "ipv4/unicast ipv6;" becomes key="ipv4/unicast ipv6", value="true".
func parseNexthopFamilies(tree *Tree) []NexthopFamilyConfig {
	var families []NexthopFamilyConfig

	afiMap := map[string]uint16{
		"ipv4": 1,
		"ipv6": 2,
	}
	safiMap := map[string]uint8{
		"unicast":    1,
		"multicast":  2,
		"mpls-vpn":   128,
		"mpls-label": 4,
	}

	// Iterate over all possible combinations: "<afi>/<safi> <nexthop-afi>"
	// Freeform stores the entire line as the key with value "true"
	for _, nlriAFIName := range []string{"ipv4", "ipv6"} {
		nlriAFI := afiMap[nlriAFIName]
		for _, safiName := range []string{"unicast", "multicast", "mpls-vpn", "mpls-label"} {
			nlriSAFI := safiMap[safiName]
			for _, nhAFIName := range []string{"ipv4", "ipv6"} {
				nhAFI := afiMap[nhAFIName]
				key := nlriAFIName + "/" + safiName + " " + nhAFIName
				if _, ok := tree.Get(key); ok {
					families = append(families, NexthopFamilyConfig{
						NLRIAFI:    nlriAFI,
						NLRISAFI:   nlriSAFI,
						NextHopAFI: nhAFI,
					})
				}
			}
		}
	}

	return families
}

// extractRoutesFromTree extracts all routes from a neighbor or template tree.
// Handles both static { route ... } and announce { ipv4/ipv6 { unicast/multicast ... } } blocks.
// Uses GetListOrdered to preserve config order.
func extractRoutesFromTree(tree *Tree) ([]StaticRouteConfig, error) {
	var routes []StaticRouteConfig

	// Static routes - use ordered iteration to preserve config order
	if static := tree.GetContainer("static"); static != nil {
		for _, entry := range static.GetListOrdered("route") {
			sr, err := parseRouteConfig(entry.Key, entry.Value)
			if err != nil {
				return nil, err
			}
			routes = append(routes, sr)
		}
	}

	// Announce routes - parse from announce { ipv4 { unicast ... } } structure
	if announce := tree.GetContainer("announce"); announce != nil {
		// Parse IPv4 routes
		if ipv4 := announce.GetContainer("ipv4"); ipv4 != nil {
			for _, entry := range ipv4.GetListOrdered("unicast") {
				sr, err := parseRouteConfig(entry.Key, entry.Value)
				if err != nil {
					return nil, err
				}
				routes = append(routes, sr)
			}
			for _, entry := range ipv4.GetListOrdered("multicast") {
				sr, err := parseRouteConfig(entry.Key, entry.Value)
				if err != nil {
					return nil, err
				}
				routes = append(routes, sr)
			}
			for _, entry := range ipv4.GetListOrdered("nlri-mpls") {
				sr, err := parseRouteConfig(entry.Key, entry.Value)
				if err != nil {
					return nil, err
				}
				routes = append(routes, sr)
			}
			for _, entry := range ipv4.GetListOrdered("mpls-vpn") {
				sr, err := parseRouteConfig(entry.Key, entry.Value)
				if err != nil {
					return nil, err
				}
				routes = append(routes, sr)
			}
		}
		// Parse IPv6 routes
		if ipv6 := announce.GetContainer("ipv6"); ipv6 != nil {
			for _, entry := range ipv6.GetListOrdered("unicast") {
				sr, err := parseRouteConfig(entry.Key, entry.Value)
				if err != nil {
					return nil, err
				}
				routes = append(routes, sr)
			}
			for _, entry := range ipv6.GetListOrdered("multicast") {
				sr, err := parseRouteConfig(entry.Key, entry.Value)
				if err != nil {
					return nil, err
				}
				routes = append(routes, sr)
			}
			for _, entry := range ipv6.GetListOrdered("nlri-mpls") {
				sr, err := parseRouteConfig(entry.Key, entry.Value)
				if err != nil {
					return nil, err
				}
				routes = append(routes, sr)
			}
			for _, entry := range ipv6.GetListOrdered("mpls-vpn") {
				sr, err := parseRouteConfig(entry.Key, entry.Value)
				if err != nil {
					return nil, err
				}
				routes = append(routes, sr)
			}
		}
	}

	return routes, nil
}

// parseRouteConfig extracts a StaticRouteConfig from a parsed route tree.
// The prefix key may have a #N suffix for duplicate routes (ADD-PATH support).
func parseRouteConfig(prefix string, route *Tree) (StaticRouteConfig, error) {
	sr := StaticRouteConfig{}

	// Strip #N suffix added by AddListEntry for duplicate keys
	// e.g., "10.0.0.10#1" → "10.0.0.10"
	actualPrefix := prefix
	if idx := strings.LastIndex(prefix, "#"); idx > 0 {
		// Verify suffix is numeric (not part of IPv6 address)
		suffix := prefix[idx+1:]
		if _, err := strconv.Atoi(suffix); err == nil {
			actualPrefix = prefix[:idx]
		}
	}

	// Try as prefix first, then as bare IP (host route)
	p, err := netip.ParsePrefix(actualPrefix)
	if err != nil {
		// Try as bare IP, convert to /32 or /128
		ip, err2 := netip.ParseAddr(actualPrefix)
		if err2 != nil {
			return sr, fmt.Errorf("invalid prefix %s: %w", actualPrefix, err)
		}
		bits := 32
		if ip.Is6() {
			bits = 128
		}
		p = netip.PrefixFrom(ip, bits)
	}
	sr.Prefix = p

	if v, ok := route.Get("next-hop"); ok {
		if v == "self" {
			sr.NextHopSelf = true
		} else {
			sr.NextHop = v
		}
	}
	if v, ok := route.Get("local-preference"); ok {
		n, _ := strconv.ParseUint(v, 10, 32)
		sr.LocalPreference = uint32(n)
	}
	if v, ok := route.Get("med"); ok {
		n, _ := strconv.ParseUint(v, 10, 32)
		sr.MED = uint32(n)
	}
	if v, ok := route.Get("community"); ok {
		sr.Community = v
	}
	if v, ok := route.Get("extended-community"); ok {
		sr.ExtendedCommunity = v
	}
	if v, ok := route.Get("large-community"); ok {
		sr.LargeCommunity = v
	}
	if v, ok := route.Get("as-path"); ok {
		sr.ASPath = v
	}
	if v, ok := route.Get("origin"); ok {
		sr.Origin = v
	}
	if v, ok := route.Get("path-information"); ok {
		sr.PathInformation = v
	}
	if v, ok := route.Get("label"); ok {
		sr.Label = v
	}
	// RFC 8277: Multi-label support via `labels [100 200 300]` syntax
	if v, ok := route.Get("labels"); ok {
		sr.Labels = parseLabelsArray(v)
	}
	if v, ok := route.Get("rd"); ok {
		sr.RD = v
	}
	if v, ok := route.Get("aggregator"); ok {
		sr.Aggregator = v
	}
	// atomic-aggregate can be a standalone flag or have a value
	if _, ok := route.Get("atomic-aggregate"); ok {
		sr.AtomicAggregate = true
	}
	if v, ok := route.Get("attribute"); ok {
		sr.Attribute = v
	}
	if v, ok := route.Get("originator-id"); ok {
		sr.OriginatorID = v
	}
	if v, ok := route.Get("cluster-list"); ok {
		sr.ClusterList = v
	}
	if v, ok := route.Get("bgp-prefix-sid"); ok {
		sr.PrefixSID = v
	}
	if v, ok := route.Get("split"); ok {
		sr.Split = v
	}

	// Watchdog support
	if v, ok := route.Get("watchdog"); ok {
		sr.Watchdog = v
	}
	if _, ok := route.Get("withdraw"); ok {
		sr.WatchdogWithdraw = true
	}

	return sr, nil
}

// parseLabelsArray parses labels from schema.
// RFC 8277: Multi-label support.
// Input can be:
//   - "[100 200 300]" (from parseKeyValuesFromTokens, inline parsing)
//   - "100 200 300" (from ValueOrArray schema node, space-separated)
//   - "100" (single label)
func parseLabelsArray(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Strip brackets if present (from inline parsing)
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		s = strings.TrimPrefix(s, "[")
		s = strings.TrimSuffix(s, "]")
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
	}

	// Split by whitespace (handles both "100" and "100 200 300")
	return strings.Fields(s)
}

// extractMVPNRoutes extracts MVPN routes from announce { ipv4/ipv6 { mcast-vpn ... } }.
func extractMVPNRoutes(tree *Tree) []MVPNRouteConfig {
	var routes []MVPNRouteConfig

	announce := tree.GetContainer("announce")
	if announce == nil {
		return routes
	}

	// IPv4 MVPN - use GetListOrdered to preserve config order
	if ipv4 := announce.GetContainer("ipv4"); ipv4 != nil {
		for _, entry := range ipv4.GetListOrdered("mcast-vpn") {
			r := parseMVPNRoute(entry.Key, entry.Value, false)
			routes = append(routes, r)
		}
	}

	// IPv6 MVPN - use GetListOrdered to preserve config order
	if ipv6 := announce.GetContainer("ipv6"); ipv6 != nil {
		for _, entry := range ipv6.GetListOrdered("mcast-vpn") {
			r := parseMVPNRoute(entry.Key, entry.Value, true)
			routes = append(routes, r)
		}
	}

	return routes
}

func parseMVPNRoute(routeType string, route *Tree, isIPv6 bool) MVPNRouteConfig {
	r := MVPNRouteConfig{
		RouteType: routeType,
		IsIPv6:    isIPv6,
	}

	if v, ok := route.Get("rd"); ok {
		r.RD = v
	}
	if v, ok := route.Get("source-as"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.SourceAS = uint32(n)
		}
	}
	// Source can be "source" or "rp" depending on route type
	if v, ok := route.Get("source"); ok {
		r.Source = v
	} else if v, ok := route.Get("rp"); ok {
		r.Source = v
	}
	if v, ok := route.Get("group"); ok {
		r.Group = v
	}
	if v, ok := route.Get("next-hop"); ok {
		r.NextHop = v
	}
	if v, ok := route.Get("origin"); ok {
		r.Origin = v
	}
	if v, ok := route.Get("local-preference"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.LocalPreference = uint32(n)
		}
	}
	if v, ok := route.Get("med"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.MED = uint32(n)
		}
	}
	if v, ok := route.Get("extended-community"); ok {
		r.ExtendedCommunity = v
	}
	if v, ok := route.Get("originator-id"); ok {
		r.OriginatorID = v
	}
	if v, ok := route.Get("cluster-list"); ok {
		r.ClusterList = v
	}

	return r
}

// parseInlineKeyValues parses an inline "key value key value ..." string into a map.
// Handles arrays like "[ a b c ]" and parenthesized content like "( ... )".
func parseInlineKeyValues(inline string) map[string]string {
	tokens := tokenizeInline(inline)
	return parseKeyValuesFromTokens(tokens, 0)
}

// parseKeyValuesFromTokens parses "key value key value ..." from a token slice.
// Handles arrays like "[ a b c ]" and parenthesized content like "( ... )".
// Start specifies the index to begin parsing from.
func parseKeyValuesFromTokens(tokens []string, start int) map[string]string {
	result := make(map[string]string)
	i := start
	for i < len(tokens) {
		key := tokens[i]
		i++
		if i >= len(tokens) {
			break
		}

		// Collect value (might be array or parenthesized)
		switch tokens[i] {
		case "[":
			// Array: collect until ]
			var arr []string
			i++ // skip [
			for i < len(tokens) && tokens[i] != "]" {
				arr = append(arr, tokens[i])
				i++
			}
			if i < len(tokens) {
				i++ // skip ]
			}
			result[key] = "[" + strings.Join(arr, " ") + "]"
		case "(":
			// Parenthesized: collect until )
			depth := 1
			var paren []string
			i++ // skip (
		parenLoop:
			for i < len(tokens) && depth > 0 {
				switch tokens[i] {
				case "(":
					depth++
				case ")":
					depth--
					if depth == 0 {
						break parenLoop
					}
				}
				paren = append(paren, tokens[i])
				i++
			}
			if i < len(tokens) {
				i++ // skip )
			}
			result[key] = "(" + strings.Join(paren, " ") + ")"
		default:
			// Simple value
			result[key] = tokens[i]
			i++
		}
	}

	return result
}

// tokenizeInline splits an inline string into tokens, preserving brackets and parens.
func tokenizeInline(s string) []string {
	var tokens []string
	var current strings.Builder

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case ' ', '\t', '\n', '\r':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		case '[', ']', '(', ')':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(c))
		case '\\':
			// Skip backslash continuations - they're artifacts from multiline parsing
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// parseVPLSFromInline creates a VPLSRouteConfig from an inline string.
func parseVPLSFromInline(inline string) VPLSRouteConfig {
	kv := parseInlineKeyValues(inline)
	r := VPLSRouteConfig{}

	if v, ok := kv["rd"]; ok {
		r.RD = v
	}
	if v, ok := kv["endpoint"]; ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Endpoint = uint16(n)
		}
	}
	if v, ok := kv["base"]; ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.Base = uint32(n)
		}
	}
	if v, ok := kv["offset"]; ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Offset = uint16(n)
		}
	}
	if v, ok := kv["size"]; ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Size = uint16(n)
		}
	}
	if v, ok := kv["next-hop"]; ok {
		r.NextHop = v
	}
	if v, ok := kv["origin"]; ok {
		r.Origin = v
	}
	if v, ok := kv["local-preference"]; ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.LocalPreference = uint32(n)
		}
	}
	if v, ok := kv["med"]; ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.MED = uint32(n)
		}
	}
	if v, ok := kv["as-path"]; ok {
		r.ASPath = v
	}
	if v, ok := kv["community"]; ok {
		r.Community = v
	}
	if v, ok := kv["extended-community"]; ok {
		r.ExtendedCommunity = v
	}
	if v, ok := kv["originator-id"]; ok {
		r.OriginatorID = v
	}
	if v, ok := kv["cluster-list"]; ok {
		r.ClusterList = v
	}

	return r
}

// extractVPLSRoutes extracts VPLS routes from l2vpn { vpls ... } and announce { l2vpn { vpls ... } }.
// Order: announce inline first, then l2vpn named, then l2vpn inline (to match ExaBGP behavior).
func extractVPLSRoutes(tree *Tree) []VPLSRouteConfig {
	var routes []VPLSRouteConfig

	// From announce { l2vpn { vpls ... } } - inline routes first
	if announce := tree.GetContainer("announce"); announce != nil {
		if l2vpn := announce.GetContainer("l2vpn"); l2vpn != nil {
			// Inline first
			for _, inline := range l2vpn.GetMultiValues("vpls") {
				if inline != "" && inline != configTrue {
					r := parseVPLSFromInline(inline)
					routes = append(routes, r)
				}
			}
			// Named blocks from announce - use GetListOrdered to preserve config order
			for _, entry := range l2vpn.GetListOrdered("vpls") {
				r := parseVPLSRoute(entry.Key, entry.Value)
				routes = append(routes, r)
			}
		}
	}

	// From l2vpn block - named blocks then inline
	if l2vpn := tree.GetContainer("l2vpn"); l2vpn != nil {
		// Named blocks: vpls site5 { ... } - use GetListOrdered to preserve config order
		for _, entry := range l2vpn.GetListOrdered("vpls") {
			r := parseVPLSRoute(entry.Key, entry.Value)
			routes = append(routes, r)
		}
		// Inline: vpls rd X endpoint Y ...;
		for _, inline := range l2vpn.GetMultiValues("vpls") {
			if inline != "" && inline != configTrue {
				r := parseVPLSFromInline(inline)
				routes = append(routes, r)
			}
		}
	}

	return routes
}

func parseVPLSRoute(name string, route *Tree) VPLSRouteConfig {
	r := VPLSRouteConfig{Name: name}

	if v, ok := route.Get("rd"); ok {
		r.RD = v
	}
	if v, ok := route.Get("endpoint"); ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Endpoint = uint16(n)
		}
	}
	if v, ok := route.Get("base"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.Base = uint32(n)
		}
	}
	if v, ok := route.Get("offset"); ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Offset = uint16(n)
		}
	}
	if v, ok := route.Get("size"); ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Size = uint16(n)
		}
	}
	if v, ok := route.Get("next-hop"); ok {
		r.NextHop = v
	}
	if v, ok := route.Get("origin"); ok {
		r.Origin = v
	}
	if v, ok := route.Get("local-preference"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.LocalPreference = uint32(n)
		}
	}
	if v, ok := route.Get("med"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.MED = uint32(n)
		}
	}
	if v, ok := route.Get("as-path"); ok {
		r.ASPath = v
	}
	if v, ok := route.Get("community"); ok {
		r.Community = v
	}
	if v, ok := route.Get("extended-community"); ok {
		r.ExtendedCommunity = v
	}
	if v, ok := route.Get("originator-id"); ok {
		r.OriginatorID = v
	}
	if v, ok := route.Get("cluster-list"); ok {
		r.ClusterList = v
	}

	return r
}

// extractFlowSpecRoutes extracts FlowSpec routes from flow { route ... }.
func extractFlowSpecRoutes(tree *Tree) []FlowSpecRouteConfig {
	flow := tree.GetContainer("flow")
	if flow == nil {
		return nil
	}

	// Use ordered iteration to preserve config order.
	entries := flow.GetListOrdered("route")
	routes := make([]FlowSpecRouteConfig, 0, len(entries))
	for _, entry := range entries {
		r := parseFlowSpecRoute(entry.Key, entry.Value)
		routes = append(routes, r)
	}

	return routes
}

func parseFlowSpecRoute(name string, route *Tree) FlowSpecRouteConfig {
	r := FlowSpecRouteConfig{
		Name:  name,
		Match: make(map[string]string),
		Then:  make(map[string]string),
	}

	if v, ok := route.Get("rd"); ok {
		r.RD = v
	}
	if v, ok := route.Get("next-hop"); ok {
		r.NextHop = v
	}

	// Parse match block - Freeform stores:
	// - "keyword value" -> "true" for simple values like "source 10.0.0.1/32"
	// - "keyword" -> "value" for arrays like "fragment [ last-fragment ]"
	if match := route.GetContainer("match"); match != nil {
		for _, key := range match.Values() {
			val, _ := match.Get(key)
			if val == configTrue || val == "" {
				// Legacy format: key might be "keyword value"
				parts := strings.SplitN(key, " ", 2)
				if len(parts) == 2 {
					r.Match[parts[0]] = parts[1]
				} else {
					r.Match[key] = ""
				}
			} else {
				// Array format: key is keyword, val has the values
				r.Match[key] = val
			}
		}
	}

	// Parse then block - Freeform stores:
	// - "keyword" -> "true" for flags like "discard"
	// - "keyword" -> "value" for arrays like "community [30740:0 30740:30740]"
	// - "keyword value" -> "true" for simple key-value (older format)
	if then := route.GetContainer("then"); then != nil {
		for _, key := range then.Values() {
			val, _ := then.Get(key)
			if val == "true" || val == "" {
				// Flag or legacy format: key might be "keyword value"
				parts := strings.SplitN(key, " ", 2)
				if len(parts) == 2 {
					r.Then[parts[0]] = parts[1]
				} else {
					r.Then[key] = ""
				}
			} else {
				// Array format: key is keyword, val has the values
				r.Then[key] = val
			}
		}
	}

	// Determine if IPv6 based on match criteria
	for key, val := range r.Match {
		if key == "source" || key == "destination" {
			if strings.Contains(val, ":") {
				r.IsIPv6 = true
				break
			}
		}
	}

	// Extract extended-community from "then" block into dedicated field
	// This is needed because extended-community can appear either in "then" block
	// or at route level, but processing always uses the ExtendedCommunity field.
	if ec, ok := r.Then["extended-community"]; ok && ec != "" {
		// Append to any existing extended-community
		if r.ExtendedCommunity != "" {
			r.ExtendedCommunity += " " + ec
		} else {
			r.ExtendedCommunity = ec
		}
		delete(r.Then, "extended-community") // Avoid double processing
	}

	return r
}

// parseMUPFromInline creates a MUPRouteConfig from an inline string.
// Format: "mup-isd PREFIX rd RD next-hop NH ..." or "mup-dsd ADDR rd RD ...".
func parseMUPFromInline(inline string, isIPv6 bool) MUPRouteConfig {
	tokens := tokenizeInline(inline)
	if len(tokens) == 0 {
		return MUPRouteConfig{}
	}

	r := MUPRouteConfig{
		IsIPv6: isIPv6,
	}

	// First token is route type
	r.RouteType = tokens[0]

	// Second token is prefix or address
	if len(tokens) > 1 {
		if r.RouteType == routeTypeMUPISD || r.RouteType == routeTypeMUPT1ST {
			r.Prefix = tokens[1]
		} else {
			r.Address = tokens[1]
		}
	}

	// Parse remaining as key-value pairs starting from index 2
	kv := parseKeyValuesFromTokens(tokens, 2)

	if v, ok := kv["rd"]; ok {
		r.RD = v
	}
	if v, ok := kv["teid"]; ok {
		r.TEID = v
	}
	if v, ok := kv["qfi"]; ok {
		if n, err := strconv.ParseUint(v, 10, 8); err == nil {
			r.QFI = uint8(n)
		}
	}
	if v, ok := kv["endpoint"]; ok {
		r.Endpoint = v
	}
	if v, ok := kv["source"]; ok {
		r.Source = v
	}
	if v, ok := kv["next-hop"]; ok {
		r.NextHop = v
	}
	if v, ok := kv["extended-community"]; ok {
		r.ExtendedCommunity = v
	}
	if v, ok := kv["bgp-prefix-sid-srv6"]; ok {
		r.PrefixSID = v
	}

	return r
}

// extractMUPRoutes extracts MUP routes from announce { ipv4/ipv6 { mup ... } }.
func extractMUPRoutes(tree *Tree) []MUPRouteConfig {
	var routes []MUPRouteConfig

	announce := tree.GetContainer("announce")
	if announce == nil {
		return routes
	}

	// IPv4 MUP - use GetListOrdered to preserve config order
	if ipv4 := announce.GetContainer("ipv4"); ipv4 != nil {
		// Named blocks (if any)
		for _, entry := range ipv4.GetListOrdered("mup") {
			r := parseMUPRoute(entry.Key, entry.Value, false)
			routes = append(routes, r)
		}
		// Inline: mup mup-isd PREFIX rd RD ...;
		for _, inline := range ipv4.GetMultiValues("mup") {
			if inline != "" && inline != configTrue {
				r := parseMUPFromInline(inline, false)
				routes = append(routes, r)
			}
		}
	}

	// IPv6 MUP - use GetListOrdered to preserve config order
	if ipv6 := announce.GetContainer("ipv6"); ipv6 != nil {
		// Named blocks (if any)
		for _, entry := range ipv6.GetListOrdered("mup") {
			r := parseMUPRoute(entry.Key, entry.Value, true)
			routes = append(routes, r)
		}
		// Inline
		for _, inline := range ipv6.GetMultiValues("mup") {
			if inline != "" && inline != configTrue {
				r := parseMUPFromInline(inline, true)
				routes = append(routes, r)
			}
		}
	}

	return routes
}

func parseMUPRoute(routeType string, route *Tree, isIPv6 bool) MUPRouteConfig {
	r := MUPRouteConfig{
		RouteType: routeType,
		IsIPv6:    isIPv6,
	}

	// Route type determines which field to use for prefix/address
	if strings.HasSuffix(routeType, "-isd") || strings.HasSuffix(routeType, "-t1st") {
		// These have prefix
		for _, key := range route.Values() {
			if strings.Contains(key, "/") || strings.Contains(key, ":") {
				r.Prefix = key
				break
			}
		}
	} else {
		// mup-dsd, mup-t2st have address
		for _, key := range route.Values() {
			if !strings.Contains(key, "/") && (strings.Contains(key, ".") || strings.Contains(key, ":")) {
				r.Address = key
				break
			}
		}
	}

	if v, ok := route.Get("rd"); ok {
		r.RD = v
	}
	if v, ok := route.Get("teid"); ok {
		r.TEID = v
	}
	if v, ok := route.Get("qfi"); ok {
		if n, err := strconv.ParseUint(v, 10, 8); err == nil {
			r.QFI = uint8(n)
		}
	}
	if v, ok := route.Get("endpoint"); ok {
		r.Endpoint = v
	}
	if v, ok := route.Get("source"); ok {
		r.Source = v
	}
	if v, ok := route.Get("next-hop"); ok {
		r.NextHop = v
	}
	if v, ok := route.Get("extended-community"); ok {
		r.ExtendedCommunity = v
	}
	if v, ok := route.Get("bgp-prefix-sid-srv6"); ok {
		r.PrefixSID = v
	}

	return r
}

// parseAddPathFamily parses a per-family add-path configuration string.
// Format: "ipv4/unicast send" or "ipv6/unicast receive" or "ipv4/multicast send/receive"
// Returns AddPathFamilyConfig with Family, Send, and Receive populated.
func parseAddPathFamily(s string) AddPathFamilyConfig {
	parts := strings.Fields(s)
	if len(parts) < 2 {
		return AddPathFamilyConfig{}
	}

	// Family is first token (e.g., "ipv4/unicast")
	family := parts[0]
	mode := parts[1]

	apf := AddPathFamilyConfig{Family: family}

	switch mode {
	case addPathSend:
		apf.Send = true
	case addPathReceive:
		apf.Receive = true
	case addPathSendReceive, addPathReceiveSend:
		apf.Send = true
		apf.Receive = true
	}

	return apf
}

// ipToUint32 converts an IPv4 address to uint32.
func ipToUint32(ip netip.Addr) uint32 {
	if !ip.Is4() {
		return 0
	}
	b := ip.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// ToReactorConfig converts BGPConfig to reactor configuration.
func (c *BGPConfig) ToReactorPeers() []*PeerReactor {
	neighbors := make([]*PeerReactor, 0, len(c.Peers))

	for _, nc := range c.Peers {
		n := &PeerReactor{
			Address:  nc.Address,
			Port:     179,
			LocalAS:  nc.LocalAS,
			PeerAS:   nc.PeerAS,
			RouterID: nc.RouterID,
			HoldTime: time.Duration(nc.HoldTime) * time.Second,
			Passive:  nc.Passive,
		}

		// Use global LocalAS if not set per-neighbor
		if n.LocalAS == 0 {
			n.LocalAS = c.LocalAS
		}

		// Use global RouterID if not set per-neighbor
		if n.RouterID == 0 {
			n.RouterID = c.RouterID
		}

		neighbors = append(neighbors, n)
	}

	return neighbors
}

// PeerReactor is the reactor-compatible neighbor config.
type PeerReactor struct {
	Address  netip.Addr
	Port     uint16
	LocalAS  uint32
	PeerAS   uint32
	RouterID uint32
	HoldTime time.Duration
	Passive  bool
}

// parseDurationValue parses a duration string like "100ms", "5s", "0.5s".
func parseDurationValue(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "0" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

// isValidGroupName validates group names per naming rules:
// - Must start with a letter (a-z, A-Z).
// - May contain letters, numbers, hyphens.
// - Must NOT end with a hyphen.
// - Minimum length: 1 character.
func isValidGroupName(name string) bool {
	if len(name) == 0 {
		return false
	}

	// Must start with letter.
	first := name[0]
	isLetter := (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')
	if !isLetter {
		return false
	}

	// Single character is valid.
	if len(name) == 1 {
		return true
	}

	// Must not end with hyphen.
	last := name[len(name)-1]
	if last == '-' {
		return false
	}

	// Middle characters: letters, numbers, hyphens only.
	for i := 1; i < len(name); i++ {
		c := name[i]
		isAlphaNum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !isAlphaNum && c != '-' {
			return false
		}
	}

	return true
}

// isGlobPattern returns true if the pattern is a glob (contains * or /) rather than an IP.
func isGlobPattern(pattern string) bool {
	// Contains wildcard
	if strings.Contains(pattern, "*") {
		return true
	}
	// Contains CIDR notation
	if strings.Contains(pattern, "/") {
		return true
	}
	// Try to parse as IP - if it fails, it might be a pattern
	_, err := netip.ParseAddr(pattern)
	return err != nil
}

// IPGlobMatch checks if an IP address matches a glob pattern.
// Pattern "*" matches any IP (IPv4 or IPv6).
// For IPv4, each octet can be "*" to match any value 0-255.
// For IPv6, supports trailing wildcard (2001:db8::*).
// CIDR notation is also supported (10.0.0.0/8, 2001:db8::/32).
// Examples: "192.168.*.*", "10.*.0.1", "*.*.*.1", "2001:db8::*", "10.0.0.0/8".
func IPGlobMatch(pattern, ip string) bool {
	// "*" matches everything
	if pattern == "*" {
		return true
	}

	// CIDR notation
	if strings.Contains(pattern, "/") {
		return cidrMatch(pattern, ip)
	}

	// IPv4 glob pattern (contains dots and wildcard)
	if strings.Contains(pattern, ".") && strings.Contains(ip, ".") {
		if strings.Contains(pattern, "*") {
			return ipv4GlobMatch(pattern, ip)
		}
		return pattern == ip
	}

	// IPv6 glob pattern (contains colons and wildcard)
	if strings.Contains(pattern, ":") && strings.Contains(ip, ":") {
		if strings.Contains(pattern, "*") {
			return ipv6GlobMatch(pattern, ip)
		}
		return pattern == ip
	}

	// Exact match fallback
	return pattern == ip
}

// cidrMatch checks if an IP is within a CIDR range.
func cidrMatch(cidr, ip string) bool {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return false
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	return prefix.Contains(addr)
}

// ipv6GlobMatch matches IPv6 addresses against trailing wildcard patterns.
// Supports patterns like "2001:db8::*" matching "2001:db8::1".
func ipv6GlobMatch(pattern, ip string) bool {
	// Handle trailing wildcard: 2001:db8::*
	if strings.HasSuffix(pattern, "::*") {
		prefix := strings.TrimSuffix(pattern, "::*")
		// The IP should start with the prefix followed by ::
		if strings.HasPrefix(ip, prefix+"::") {
			return true
		}
		// Or it could be an expanded form
		if strings.HasPrefix(ip, prefix+":") {
			return true
		}
		return false
	}

	// Handle mid-pattern wildcards (less common but supported)
	if strings.Contains(pattern, "*") {
		// Split on :: and handle each part
		patternParts := strings.Split(pattern, ":")
		ipParts := strings.Split(ip, ":")

		// Normalize both to full 8 groups for comparison
		patternParts = normalizeIPv6Parts(patternParts)
		ipParts = normalizeIPv6Parts(ipParts)

		if len(patternParts) != len(ipParts) {
			return false
		}

		for i := range patternParts {
			if patternParts[i] == "*" {
				continue
			}
			if patternParts[i] != ipParts[i] {
				return false
			}
		}
		return true
	}

	return pattern == ip
}

// normalizeIPv6Parts expands :: notation to full 8 groups.
func normalizeIPv6Parts(parts []string) []string {
	// Count empty strings (from :: split).
	emptyCount := 0
	for _, p := range parts {
		if p == "" {
			emptyCount++
		}
	}

	if emptyCount == 0 && len(parts) == 8 {
		return parts
	}

	// Need to expand :: to fill 8 groups.
	result := make([]string, 0, 8)
	for i, p := range parts {
		switch {
		case p == "" && i > 0 && i < len(parts)-1:
			// This is the :: expansion point.
			zerosNeeded := 8 - len(parts) + emptyCount
			for j := 0; j < zerosNeeded; j++ {
				result = append(result, "0")
			}
		case p != "":
			result = append(result, p)
		case i == 0 || i == len(parts)-1:
			// Leading or trailing empty from :: at start/end.
			result = append(result, "0")
		}
	}

	// Pad to 8 if needed.
	for len(result) < 8 {
		result = append(result, "0")
	}

	return result[:8]
}

// ipv4GlobMatch matches IPv4 addresses against glob patterns.
func ipv4GlobMatch(pattern, ip string) bool {
	patternParts := strings.Split(pattern, ".")
	ipParts := strings.Split(ip, ".")

	if len(patternParts) != 4 || len(ipParts) != 4 {
		return false
	}

	for i := 0; i < 4; i++ {
		if patternParts[i] == "*" {
			continue // wildcard matches any octet
		}
		if patternParts[i] != ipParts[i] {
			return false
		}
	}
	return true
}

// peerGlobSpecificity returns how specific a peer glob pattern is.
// More specific patterns have higher scores. Used for ordering.
// "*" = 0, "192.*.*.*" = 1, "192.168.*.*" = 2, etc.
func peerGlobSpecificity(pattern string) int {
	if pattern == "*" {
		return 0
	}
	if !strings.Contains(pattern, ".") {
		return 4 // exact IPv6 or exact match
	}
	parts := strings.Split(pattern, ".")
	count := 0
	for _, p := range parts {
		if p != "*" {
			count++
		}
	}
	return count
}

// PeerGlob holds a parsed peer glob pattern and its settings.
type PeerGlob struct {
	Pattern     string
	Specificity int
	Tree        *Tree
}

// ExtractEnvironment extracts environment configuration values from a parsed Tree.
// Returns a map suitable for passing to LoadEnvironmentWithConfig.
// The environment block is optional - returns empty map if not present.
func ExtractEnvironment(tree *Tree) map[string]map[string]string {
	envContainer := tree.GetContainer("environment")
	if envContainer == nil {
		return nil
	}

	result := make(map[string]map[string]string)

	// Extract each section (daemon, log, tcp, bgp, cache, api, reactor, debug)
	sections := []string{"daemon", "log", "tcp", "bgp", "cache", "api", "reactor", "debug"}
	for _, section := range sections {
		sectionContainer := envContainer.GetContainer(section)
		if sectionContainer == nil {
			continue
		}

		sectionValues := make(map[string]string)
		for _, option := range sectionContainer.Values() {
			value, _ := sectionContainer.Get(option)
			sectionValues[option] = value
		}

		if len(sectionValues) > 0 {
			result[section] = sectionValues
		}
	}

	return result
}
