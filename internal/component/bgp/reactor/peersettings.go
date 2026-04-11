// Design: docs/architecture/core-design.md — peer configuration settings
// Related: config.go — config tree parsing produces PeerSettings
//
// Package reactor implements the BGP reactor - the main orchestrator
// that manages peer sessions, connections, and signal handling.
package reactor

import (
	"fmt"
	"net/netip"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
)

// NextHopMode values for PeerSettings.NextHopMode.
const (
	// NextHopAuto is the default: rewrite for eBGP, preserve for iBGP (RFC 4271 Section 5.1.3).
	NextHopAuto uint8 = iota
	// NextHopSelf always rewrites next-hop to local address.
	NextHopSelf
	// NextHopUnchanged never rewrites next-hop.
	NextHopUnchanged
	// NextHopExplicit sets next-hop to PeerSettings.NextHopAddress.
	NextHopExplicit
)

// DefaultBGPPort is the standard BGP port per RFC 4271.
// Source of truth: ze-bgp-conf.yang (environment > tcp > port, default 179).
// Used as fallback when YANG schema defaults are not applied (tests, PeerKey).
const DefaultBGPPort = 179

// ConnectionMode controls TCP connection establishment for a peer.
// Two independent booleans: Connect (dial out) and Accept (accept inbound).
// RFC 4271 Section 8.1.1: PassiveTcpEstablishment optional attribute.
type ConnectionMode struct {
	Connect bool // Initiate outbound TCP connections
	Accept  bool // Accept inbound TCP connections
}

// ConnectionBoth is the default: initiate and accept connections.
var ConnectionBoth = ConnectionMode{Connect: true, Accept: true}

// ConnectionActive initiates only.
var ConnectionActive = ConnectionMode{Connect: true, Accept: false}

// ConnectionPassive accepts only.
var ConnectionPassive = ConnectionMode{Connect: false, Accept: true}

// IsActive reports whether dialing out is enabled.
func (m ConnectionMode) IsActive() bool { return m.Connect }

// IsPassive reports whether accepting inbound is enabled.
func (m ConnectionMode) IsPassive() bool { return m.Accept }

// DefaultReceiveHoldTime is the default receive hold time per RFC 4271.
// Source of truth: ze-bgp-conf.yang (timer > receive-hold-time, default 90).
// Production config reads this from YANG via ApplyDefaults.
const DefaultReceiveHoldTime = 90 * time.Second

// DefaultConnectRetry is the default connect retry interval per RFC 4271.
// Source of truth: ze-bgp-conf.yang (timer > connect-retry, default 120).
// Production config reads this from YANG via ApplyDefaults.
const DefaultConnectRetry = 120 * time.Second

// StaticRoute represents a route to announce when session is established.
// Fields are stored in both serializable (string/uint) and wire-ready formats.
//
// IMMUTABILITY: StaticRoute and its slices (ASPath, Communities, etc.) must not
// be mutated after being stored. Peer settings store shallow
// copies for efficiency; mutation would corrupt internal state.
type StaticRoute struct {
	Prefix  netip.Prefix
	NextHop bgptypes.RouteNextHop // Encapsulates next-hop policy (explicit or self)

	Origin          uint8  // 0=IGP, 1=EGP, 2=INCOMPLETE
	LocalPreference uint32 // For iBGP
	MED             uint32 // Multi-Exit Discriminator

	// Communities (RFC 1997) - each uint32 is ASN:value (high 16 bits : low 16 bits)
	Communities []uint32

	// Large communities (RFC 8092) - each is [3]uint32: GlobalAdmin:LocalData1:LocalData2
	LargeCommunities [][3]uint32

	// Extended communities - both forms for serialization and wire encoding
	ExtCommunity      string // Original string (e.g., "target:72:1")
	ExtCommunityBytes []byte // Wire-format (8 bytes each, sorted)

	PathID uint32   // ADD-PATH path identifier
	Labels []uint32 // RFC 8277: MPLS label stack (20-bit values)

	// Route Distinguisher - both forms for serialization and wire encoding
	RD      string  // Original string (e.g., "100:100")
	RDBytes [8]byte // Wire-format (8 bytes)

	// AS_PATH - list of AS numbers in AS_SEQUENCE
	ASPath []uint32

	// Aggregator (RFC 4271) - 4-byte ASN + 4-byte IP
	AggregatorASN uint32
	AggregatorIP  [4]byte
	HasAggregator bool

	// ATOMIC_AGGREGATE flag
	AtomicAggregate bool

	// ORIGINATOR_ID and CLUSTER_LIST (RFC 4456)
	OriginatorID uint32
	ClusterList  []uint32

	// BGP Prefix-SID (RFC 8669) - wire-format bytes for attribute type 40
	PrefixSIDBytes []byte

	// Raw attributes (code, flags, value bytes)
	RawAttributes []RawAttribute
}

// RawAttribute represents a raw BGP path attribute.
type RawAttribute struct {
	Code  uint8
	Flags uint8
	Value []byte
}

// IsVPN returns true if this is a VPN route (has RD).
func (r *StaticRoute) IsVPN() bool {
	return r.RD != ""
}

// IsLabeledUnicast returns true if this is a labeled unicast route (has labels but no RD).
// RFC 8277 Section 2: Labeled routes have MPLS label stack but no Route Distinguisher.
func (r *StaticRoute) IsLabeledUnicast() bool {
	return len(r.Labels) > 0 && r.RD == ""
}

// SingleLabel returns the first label from the label stack, or 0 if empty.
func (r *StaticRoute) SingleLabel() uint32 {
	if len(r.Labels) > 0 {
		return r.Labels[0]
	}
	return 0
}

// RouteKey returns a unique key for this route, suitable for use as a map key.
// Includes prefix, RD (for VPN), and PathID (for ADD-PATH).
// PathID is always included since 0 is a valid path identifier.
func (r *StaticRoute) RouteKey() string {
	key := r.Prefix.String()
	if r.RD != "" {
		key = r.RD + ":" + key
	}
	return fmt.Sprintf("%s#%d", key, r.PathID)
}

// MVPNRoute represents an MVPN route (RFC 6514).
type MVPNRoute struct {
	RouteType         uint8      // 5=source-ad, 6=shared-join, 7=source-join
	IsIPv6            bool       // IPv4 or IPv6 MVPN
	RD                [8]byte    // Route Distinguisher
	SourceAS          uint32     // Source AS
	Source            netip.Addr // Source IP or RP
	Group             netip.Addr // Multicast group
	NextHop           netip.Addr
	Origin            uint8
	LocalPreference   uint32
	MED               uint32
	ExtCommunityBytes []byte
	OriginatorID      uint32   // RFC 4456
	ClusterList       []uint32 // RFC 4456
}

// VPLSRoute represents a VPLS route.
type VPLSRoute struct {
	Name              string
	RD                [8]byte
	Endpoint          uint16
	Base              uint32
	Offset            uint16
	Size              uint16
	NextHop           netip.Addr
	Origin            uint8
	LocalPreference   uint32
	MED               uint32
	ASPath            []uint32
	Communities       []uint32
	ExtCommunityBytes []byte
	OriginatorID      uint32
	ClusterList       []uint32
}

// FlowSpecRoute represents a FlowSpec route (RFC 5575).
type FlowSpecRoute struct {
	Name                  string
	IsIPv6                bool
	RD                    [8]byte // For flow-vpn
	NLRI                  []byte  // Pre-built FlowSpec NLRI
	NextHop               netip.Addr
	CommunityBytes        []byte // Standard communities
	ExtCommunityBytes     []byte // Extended communities (attribute 16)
	IPv6ExtCommunityBytes []byte // IPv6 Extended communities (attribute 25, RFC 5701)
}

// MUPRoute represents a MUP route.
type MUPRoute struct {
	RouteType         uint8 // Route subtype
	IsIPv6            bool
	NLRI              []byte // Pre-built MUP NLRI
	NextHop           netip.Addr
	ExtCommunityBytes []byte
	PrefixSID         []byte
}

// PeerSettings contains configuration for a BGP peer.
type PeerSettings struct {
	// Name is an optional human-readable peer name for CLI selector.
	Name string

	// GroupName is the peer-group this peer belongs to.
	GroupName string

	// Address is the peer's IP address.
	Address netip.Addr

	// LocalAddress is our local IP for this session.
	LocalAddress netip.Addr

	// LinkLocal is the IPv6 link-local address for MP_REACH next-hop (RFC 2545 Section 3).
	// When set, IPv6 unicast MP_REACH_NLRI includes 32-byte next-hop (global + link-local).
	LinkLocal netip.Addr

	// Port is the peer's BGP port (default 179).
	Port uint16

	// LocalAS is our effective AS number for this session.
	// Equals the per-peer local-as override when set, otherwise the global local-as.
	LocalAS uint32

	// GlobalLocalAS is the router's global local-as (bgp/session/asn/local),
	// preserved even when LocalAS is overridden per-peer. Used by local-as
	// modifiers to know the "real" AS for dual-prepend semantics.
	// Equals LocalAS when no override is active.
	GlobalLocalAS uint32

	// PeerAS is the peer's AS number.
	PeerAS uint32

	// RouterID is our BGP router identifier (IPv4 format).
	RouterID uint32

	// ReceiveHoldTime is the proposed receive hold time (default 90s, RFC 4271).
	// Advertised in OPEN; negotiated value is min(local, remote).
	ReceiveHoldTime time.Duration

	// SendHoldTime is the send hold timer duration (RFC 9687).
	// 0 = automatic: max(8 minutes, 2x ReceiveHoldTime).
	// Explicit value >= 480s overrides the formula.
	SendHoldTime time.Duration

	// ConnectRetry is the initial connect retry interval (default 5s).
	// Used as the base for exponential backoff in peer.run().
	ConnectRetry time.Duration

	// Connection controls TCP connection establishment mode.
	// ConnectionBoth (default): initiate and accept.
	// ConnectionPassive: accept only (no dial out).
	// ConnectionActive: dial only (no bind/listen).
	Connection ConnectionMode

	// MD5Key is the TCP MD5 authentication key (RFC 2385).
	// When non-empty, TCP_MD5SIG is applied on both dialer and listener sockets.
	// The MD5IP field specifies which address to authenticate (defaults to Address).
	MD5Key string

	// MD5IP overrides the peer address used for TCP_MD5SIG setsockopt.
	// Useful for multihop BGP where the MD5 key is bound to a different address.
	// Defaults to Address when empty.
	MD5IP netip.Addr

	// GroupUpdates indicates whether to group compatible routes in single UPDATE.
	// Default: true (reduces UPDATE count from O(routes) to O(routes/capacity)).
	GroupUpdates bool

	// IgnoreFamilyMismatch ignores NLRI for non-negotiated AFI/SAFI instead of error.
	// RFC 4760 Section 6: speaker MAY treat non-negotiated AFI/SAFI as error.
	// Default false = error (RFC-correct), true = log warning and skip.
	IgnoreFamilyMismatch bool

	// DisableASN4 prevents advertising 4-byte ASN capability.
	DisableASN4 bool

	// Capabilities to advertise in OPEN message.
	Capabilities []capability.Capability

	// RequiredFamilies are address families that must be negotiated.
	// Session will be rejected with NOTIFICATION if peer doesn't support these.
	RequiredFamilies []capability.Family

	// IgnoreFamilies are address families with lenient UPDATE validation.
	// NLRI for these families will be skipped (not error) if not negotiated.
	IgnoreFamilies []capability.Family

	// RequiredCapabilities are non-family capability codes that must be negotiated.
	// Session will be rejected with NOTIFICATION if peer doesn't support these.
	// RFC 5492 Section 3: Unsupported Capability subcode.
	RequiredCapabilities []capability.Code

	// RefusedCapabilities are capability codes that must NOT be present in peer's OPEN.
	// Session will be rejected with NOTIFICATION if peer advertises any of these.
	// Unlike require, refuse checks against peer's raw capabilities, not negotiated intersection.
	RefusedCapabilities []capability.Code

	// StaticRoutes are announced when session is established.
	StaticRoutes []StaticRoute

	// Exotic route types
	MVPNRoutes     []MVPNRoute
	VPLSRoutes     []VPLSRoute
	FlowSpecRoutes []FlowSpecRoute
	MUPRoutes      []MUPRoute

	// PrefixMaximum is the hard maximum number of prefixes accepted per family.
	// Key is "afi/safi" string (e.g., "ipv4/unicast"). Mandatory for every negotiated family.
	// RFC 4486 Section 4: exceeding triggers Cease/MaxPrefixes NOTIFICATION.
	PrefixMaximum map[string]uint32

	// PrefixWarning is the warning threshold per family.
	// Defaults to 90% of PrefixMaximum when not explicitly configured.
	PrefixWarning map[string]uint32

	// PrefixTeardown controls whether exceeding the maximum tears down the session.
	// Default: true. When false, excess prefixes are rejected but session stays up.
	PrefixTeardown bool

	// PrefixIdleTimeout is seconds to wait before auto-reconnect after prefix teardown.
	// 0 means no auto-reconnect. Uses exponential backoff on repeated teardowns.
	PrefixIdleTimeout uint16

	// PrefixUpdated is the ISO date (YYYY-MM-DD) when prefix maximums were last
	// updated from PeeringDB. Empty means manually configured (no staleness tracking).
	// Hidden leaf -- not shown in config output.
	PrefixUpdated string

	// Process bindings - which plugins receive messages from this peer.
	ProcessBindings []ProcessBinding

	// ImportFilters is the ordered import filter chain for this peer.
	// Cumulative: bgp-level + group-level + peer-level, in order.
	// Each entry is a "<plugin>:<filter>" string.
	ImportFilters []string

	// ExportFilters is the ordered export filter chain for this peer.
	// Cumulative: bgp-level + group-level + peer-level, in order.
	ExportFilters []string

	// LoopAllowOwnAS is the number of own-AS occurrences to tolerate in AS_PATH.
	// From loop-detection filter config. 0 = reject on first (RFC 4271 Section 9 default).
	LoopAllowOwnAS uint8

	// LoopClusterID is the explicit cluster-id for CLUSTER_LIST loop detection.
	// From loop-detection filter config. 0 = use RouterID (RFC 4456 Section 8 default).
	LoopClusterID uint32

	// LoopDisabled disables loop detection for this peer.
	// Set when the peer's import chain has inactive: on its loop-detection filter.
	LoopDisabled bool

	// RouteReflectorClient marks this peer as a route reflector client (RFC 4456).
	// When true, routes from this peer are forwarded to all other clients and non-clients.
	// When false (non-client), routes from this peer are forwarded to clients only.
	RouteReflectorClient bool

	// ClusterID is the cluster identifier for route reflection (RFC 4456 Section 7).
	// Prepended to CLUSTER_LIST on reflected routes.
	// 0 means use RouterID (default per RFC 4456).
	ClusterID uint32

	// NextHopMode controls next-hop rewriting for forwarded UPDATEs.
	// RFC 4271 Section 5.1.3.
	//   NextHopAuto (0): rewrite for eBGP, preserve for iBGP (default)
	//   NextHopSelf (1): always rewrite to local address
	//   NextHopUnchanged (2): never rewrite
	//   NextHopExplicit (3): set to NextHopAddress
	NextHopMode uint8

	// NextHopAddress is the explicit next-hop IP when NextHopMode == NextHopExplicit.
	NextHopAddress netip.Addr

	// ASOverride replaces the peer's ASN with local ASN in outbound AS_PATH.
	// Used in VPN/multi-site scenarios.
	ASOverride bool

	// LocalASNoPrepend prevents prepending the real ASN before the local-as override.
	// Only relevant when session/asn/local is set (local-as override).
	LocalASNoPrepend bool

	// LocalASReplaceAS replaces the real ASN entirely with the local-as override.
	// Only relevant when session/asn/local is set.
	LocalASReplaceAS bool

	// SendCommunity controls which community types to include in outbound UPDATEs.
	// nil/empty means send all (default). "none" means suppress all.
	// Individual types: "standard", "large", "extended".
	SendCommunity []string

	// DefaultOriginate tracks per-family default route origination.
	// Key is "afi/safi" string (e.g., "ipv4/unicast").
	DefaultOriginate map[string]bool

	// DefaultOriginateFilter tracks per-family conditional origination filters.
	// Key is "afi/safi" string. Empty value means unconditional.
	DefaultOriginateFilter map[string]string

	// RawCapabilityConfig stores parsed capability config values for plugin delivery.
	// Maps capability name → field name → value (e.g., "graceful-restart" → "restart-time" → "120").
	// Populated from config blocks like: capability { graceful-restart { restart-time 120; } }
	// Used for plugin-declared capabilities that don't have Go capability types.
	RawCapabilityConfig map[string]map[string]string

	// CapabilityConfigJSON is the entire capability block as JSON for plugin delivery.
	// Plugins receive this and extract what they need based on their YANG knowledge.
	// This replaces the need for per-plugin extraction code in the config loader.
	CapabilityConfigJSON string
}

// ProcessBinding represents a binding between this peer and a plugin.
// Controls what messages are forwarded and in what format.
type ProcessBinding struct {
	PluginName string // Reference to plugin name

	// Content settings (HOW messages are formatted)
	Encoding string // "json" | "text" (empty = inherit from plugin)
	Format   string // "parsed" | "raw" | "full" (empty = "parsed")

	// Receive settings (WHAT message types to forward)
	ReceiveUpdate       bool
	ReceiveOpen         bool
	ReceiveNotification bool
	ReceiveKeepalive    bool
	ReceiveRefresh      bool
	ReceiveState        bool
	ReceiveSent         bool            // Forward sent UPDATE events
	ReceiveNegotiated   bool            // Forward negotiated capabilities after OPEN exchange
	ReceiveCustom       map[string]bool // Plugin-registered event types (e.g., "update-rpki")

	// Send settings (WHAT message types plugin can send)
	SendUpdate  bool
	SendRefresh bool
	SendCustom  map[string]bool // Plugin-registered send types (e.g., "enhanced-refresh")
}

// NewPeerSettings creates a peer settings with default values.
// In production, YANG schema defaults are applied to the config tree before parsing,
// so these values serve as fallbacks for direct callers (tests, API).
// They MUST match the YANG defaults in ze-bgp-conf.yang.
func NewPeerSettings(address netip.Addr, localAS, peerAS, routerID uint32) *PeerSettings {
	return &PeerSettings{
		Address:         address,
		Port:            DefaultBGPPort,
		LocalAS:         localAS,
		GlobalLocalAS:   localAS, // default: no override, global == effective
		PeerAS:          peerAS,
		RouterID:        routerID,
		ReceiveHoldTime: DefaultReceiveHoldTime,
		ConnectRetry:    DefaultConnectRetry,
		Connection:      ConnectionBoth,
		GroupUpdates:    true,
		PrefixTeardown:  true,
	}
}

// PeerKey returns the map key for this peer as a netip.AddrPort value type.
// This uniquely identifies a peer even when multiple peers share the same IP.
// Uses DefaultBGPPort when Port is zero (unset).
func (n *PeerSettings) PeerKey() netip.AddrPort {
	port := n.Port
	if port == 0 {
		port = DefaultBGPPort
	}
	return PeerKeyFromAddrPort(n.Address, port)
}

// PeerKeyFromAddrPort builds a peer map key from address and port.
// Returns a netip.AddrPort value type (20 bytes, comparable, zero allocation).
func PeerKeyFromAddrPort(addr netip.Addr, port uint16) netip.AddrPort {
	return netip.AddrPortFrom(addr, port)
}

// IsIBGP returns true if this is an internal BGP session (same AS).
func (n *PeerSettings) IsIBGP() bool {
	return n.LocalAS == n.PeerAS
}

// IsEBGP returns true if this is an external BGP session (different AS).
func (n *PeerSettings) IsEBGP() bool {
	return n.LocalAS != n.PeerAS
}

// EffectiveClusterID returns the cluster-id for route reflection.
// RFC 4456 Section 7: defaults to router-id when not explicitly configured.
func (n *PeerSettings) EffectiveClusterID() uint32 {
	if n.ClusterID != 0 {
		return n.ClusterID
	}
	return n.RouterID
}
