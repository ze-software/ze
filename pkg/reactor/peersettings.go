// Package reactor implements the BGP reactor - the main orchestrator
// that manages peer sessions, connections, and signal handling.
package reactor

import (
	"fmt"
	"net/netip"
	"time"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/capability"
	"codeberg.org/thomas-mangin/zebgp/pkg/plugin"
)

// DefaultBGPPort is the standard BGP port per RFC 4271.
const DefaultBGPPort = 179

// DefaultHoldTime is the default hold time per RFC 4271.
const DefaultHoldTime = 90 * time.Second

// StaticRoute represents a route to announce when session is established.
// Fields are stored in both serializable (string/uint) and wire-ready formats.
//
// IMMUTABILITY: StaticRoute and its slices (ASPath, Communities, etc.) must not
// be mutated after being stored. Watchdog pools and peer settings store shallow
// copies for efficiency; mutation would corrupt internal state.
type StaticRoute struct {
	Prefix  netip.Prefix
	NextHop plugin.RouteNextHop // Encapsulates next-hop policy (explicit or self)

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
func (r StaticRoute) IsVPN() bool {
	return r.RD != ""
}

// IsLabeledUnicast returns true if this is a labeled unicast route (has labels but no RD).
// RFC 8277 Section 2: Labeled routes have MPLS label stack but no Route Distinguisher.
func (r StaticRoute) IsLabeledUnicast() bool {
	return len(r.Labels) > 0 && r.RD == ""
}

// SingleLabel returns the first label from the label stack for backward compatibility.
// Returns 0 if no labels are present.
func (r StaticRoute) SingleLabel() uint32 {
	if len(r.Labels) > 0 {
		return r.Labels[0]
	}
	return 0
}

// RouteKey returns a unique key for this route, suitable for use as a map key.
// Includes prefix, RD (for VPN), and PathID (for ADD-PATH).
// PathID is always included since 0 is a valid path identifier.
func (r StaticRoute) RouteKey() string {
	key := r.Prefix.String()
	if r.RD != "" {
		key = r.RD + ":" + key
	}
	return fmt.Sprintf("%s#%d", key, r.PathID)
}

// WatchdogRoute wraps a static route with watchdog metadata.
// Stored separately from StaticRoutes to avoid bloating that struct.
type WatchdogRoute struct {
	StaticRoute             // Embed existing type
	InitiallyWithdrawn bool // Start in '-' state (held until "watchdog announce")
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
	// Address is the peer's IP address.
	Address netip.Addr

	// LocalAddress is our local IP for this session.
	LocalAddress netip.Addr

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

	// Passive indicates listen-only mode (no outgoing connections).
	Passive bool

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

	// StaticRoutes are announced when session is established.
	StaticRoutes []StaticRoute

	// WatchdogGroups holds routes controlled by watchdog API.
	// Key is watchdog name, value is list of routes in that group.
	// Routes here are NOT in StaticRoutes - they're stored separately.
	WatchdogGroups map[string][]WatchdogRoute

	// Exotic route types
	MVPNRoutes     []MVPNRoute
	VPLSRoutes     []VPLSRoute
	FlowSpecRoutes []FlowSpecRoute
	MUPRoutes      []MUPRoute

	// Process bindings - which plugins receive messages from this peer.
	ProcessBindings []ProcessBinding
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
		GroupUpdates: true,
	}
}

// IsIBGP returns true if this is an internal BGP session (same AS).
func (n *PeerSettings) IsIBGP() bool {
	return n.LocalAS == n.PeerAS
}

// IsEBGP returns true if this is an external BGP session (different AS).
func (n *PeerSettings) IsEBGP() bool {
	return n.LocalAS != n.PeerAS
}
