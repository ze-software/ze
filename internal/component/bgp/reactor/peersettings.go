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

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
)

// DefaultBGPPort is the standard BGP port per RFC 4271.
const DefaultBGPPort = 179

// ConnectionMode controls TCP connection establishment for a peer.
// Bitmask: Active (bit 0) = dial out, Passive (bit 1) = accept inbound.
// RFC 4271 Section 8.1.1: PassiveTcpEstablishment optional attribute.
type ConnectionMode int

const (
	// ConnectionActive initiates only — does not bind/listen for inbound connections.
	ConnectionActive ConnectionMode = 1 << iota
	// ConnectionPassive accepts only — does not initiate outbound connections.
	ConnectionPassive
	// ConnectionBoth initiates and accepts connections (default).
	ConnectionBoth = ConnectionActive | ConnectionPassive
)

const (
	connBoth    = "both"
	connPassive = "passive"
	connActive  = "active"
)

// IsActive reports whether dialing out is enabled (active bit set).
func (m ConnectionMode) IsActive() bool { return m&ConnectionActive != 0 }

// IsPassive reports whether accepting inbound is enabled (passive bit set).
func (m ConnectionMode) IsPassive() bool { return m&ConnectionPassive != 0 }

// String returns the config-level name for the connection mode.
func (m ConnectionMode) String() string {
	switch m {
	case ConnectionBoth:
		return connBoth
	case ConnectionPassive:
		return connPassive
	case ConnectionActive:
		return connActive
	}
	return connBoth
}

// ParseConnectionMode parses a connection mode string from config.
func ParseConnectionMode(s string) (ConnectionMode, error) {
	switch s {
	case connBoth, "":
		return ConnectionBoth, nil
	case connPassive:
		return ConnectionPassive, nil
	case connActive:
		return ConnectionActive, nil
	}
	return 0, fmt.Errorf("invalid connection mode %q: must be %s, %s, or %s", s, connBoth, connPassive, connActive)
}

// DefaultHoldTime is the default hold time per RFC 4271.
const DefaultHoldTime = 90 * time.Second

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

	// LocalAS is our AS number.
	LocalAS uint32

	// PeerAS is the peer's AS number.
	PeerAS uint32

	// RouterID is our BGP router identifier (IPv4 format).
	RouterID uint32

	// HoldTime is the proposed hold time (default 90s).
	HoldTime time.Duration

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

	// Process bindings - which plugins receive messages from this peer.
	ProcessBindings []ProcessBinding

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
	ReceiveSent         bool // Forward sent UPDATE events
	ReceiveNegotiated   bool // Forward negotiated capabilities after OPEN exchange

	// Send settings (WHAT message types plugin can send)
	SendUpdate  bool
	SendRefresh bool
}

// NewPeerSettings creates a peer settings with default values.
func NewPeerSettings(address netip.Addr, localAS, peerAS, routerID uint32) *PeerSettings {
	return &PeerSettings{
		Address:      address,
		Port:         DefaultBGPPort,
		LocalAS:      localAS,
		PeerAS:       peerAS,
		RouterID:     routerID,
		HoldTime:     DefaultHoldTime,
		Connection:   ConnectionBoth,
		GroupUpdates: true,
	}
}

// PeerKey returns the map key for this peer: "addr:port".
// This uniquely identifies a peer even when multiple peers share the same IP.
// Uses DefaultBGPPort when Port is zero (unset).
func (n *PeerSettings) PeerKey() string {
	port := n.Port
	if port == 0 {
		port = DefaultBGPPort
	}
	return PeerKeyFromAddrPort(n.Address, port)
}

// PeerKeyFromAddrPort builds a peer map key from address and port.
func PeerKeyFromAddrPort(addr netip.Addr, port uint16) string {
	return fmt.Sprintf("%s:%d", addr.String(), port)
}

// IsIBGP returns true if this is an internal BGP session (same AS).
func (n *PeerSettings) IsIBGP() bool {
	return n.LocalAS == n.PeerAS
}

// IsEBGP returns true if this is an external BGP session (different AS).
func (n *PeerSettings) IsEBGP() bool {
	return n.LocalAS != n.PeerAS
}
