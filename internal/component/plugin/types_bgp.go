// Design: docs/architecture/api/process-protocol.md -- BGP reactor types
//
// BGP-specific types and interfaces for the reactor lifecycle. These types
// are used by the BGP reactor, Coordinator, plugin server, and BGP command
// handlers. They live in the plugin package (not a BGP-specific package)
// because the Coordinator and plugin server need them without importing
// BGP packages.
//
// Other protocols (OSPF, IS-IS) would add their own types_<protocol>.go
// file in this package following the same pattern.

package plugin

import (
	"context"
	"net/netip"
	"time"
)

// PeerInfo is a snapshot of BGP peer state for API output.
type PeerInfo struct {
	Address         netip.Addr
	LocalAddress    netip.Addr
	Name            string // Human-readable peer name for CLI selector
	GroupName       string // Peer-group this peer belongs to
	LocalAS         uint32
	PeerAS          uint32
	RouterID        uint32
	ReceiveHoldTime time.Duration // Configured receive hold time (RFC 4271)
	SendHoldTime    time.Duration // Configured send hold time (RFC 9687, 0=auto)
	ConnectRetry    time.Duration // Connect retry interval
	Connect         bool          // Initiate outbound connections
	Accept          bool          // Accept inbound connections
	State           string
	Uptime          time.Duration

	// Route reflection (RFC 4456).
	RouteReflectorClient bool   // Peer is an RR client
	ClusterID            uint32 // Explicit cluster-id (0 = use router-id)

	// Next-hop mode for forwarded UPDATEs (RFC 4271 Section 5.1.3).
	// Values: 0=auto, 1=self, 2=unchanged, 3=explicit.
	NextHopMode    uint8
	NextHopAddress netip.Addr // Only when NextHopMode == 3 (explicit)

	// PrefixUpdated is the ISO date (YYYY-MM-DD) when prefix maximums were
	// last updated from PeeringDB. Empty means manually configured.
	// Active prefix-threshold and prefix-stale warnings live on the report
	// bus (internal/core/report), not on this struct.
	PrefixUpdated string

	// Policy filter chains (after group inheritance + canonicalization).
	ImportFilters []string
	ExportFilters []string

	// Statistics (engine-level counters; NLRI-level counters live in the RIB plugin)
	UpdatesReceived    uint32
	UpdatesSent        uint32
	KeepalivesReceived uint32
	KeepalivesSent     uint32
	EORReceived        uint32
	EORSent            uint32
}

// PeerCapabilityConfig holds BGP capability configuration for a peer.
// Used by plugin protocol Stage 2 to deliver matching config.
// Values is a flexible map allowing any capability to be represented.
type PeerCapabilityConfig struct {
	Address        string            // Peer IP address
	Values         map[string]string // capability-name -> value (e.g., "hostname" -> "router1.example.com")
	CapabilityJSON string            // Full capability block as JSON - plugins extract what they need
}

// ReactorStats holds BGP reactor-level statistics.
type ReactorStats struct {
	StartTime time.Time
	Uptime    time.Duration
	PeerCount int
	RouterID  uint32 // Local BGP router identifier (uint32 IP)
	LocalAS   uint32 // Local AS number
}

// PeerCapabilitiesInfo holds negotiated and configured BGP capability data for API display.
type PeerCapabilitiesInfo struct {
	Families             []string          // Negotiated address families (e.g., "ipv4/unicast")
	ExtendedMessage      bool              // RFC 8654: Extended message support
	EnhancedRouteRefresh bool              // RFC 7313: Enhanced route refresh
	ASN4                 bool              // RFC 6793: 4-byte ASN support
	AddPath              map[string]string // RFC 7911: family -> "send" for families with ADD-PATH (nil if none)
}

// ReactorIntrospector provides read-only access to BGP peer and reactor state.
type ReactorIntrospector interface {
	// Peers returns information about all configured peers.
	Peers() []PeerInfo

	// Stats returns reactor-level statistics.
	Stats() ReactorStats

	// PeerNegotiatedCapabilities returns negotiated capabilities for a peer.
	// Returns nil if peer not found or negotiation not complete.
	PeerNegotiatedCapabilities(addr netip.Addr) *PeerCapabilitiesInfo

	// GetPeerProcessBindings returns process bindings for a specific peer.
	GetPeerProcessBindings(peerAddr netip.Addr) []PeerProcessBinding

	// GetPeerCapabilityConfigs returns capability configurations for all peers.
	GetPeerCapabilityConfigs() []PeerCapabilityConfig
}

// ReactorPeerController manages BGP peer lifecycle: shutdown, teardown,
// flow control, and dynamic peer add/remove.
type ReactorPeerController interface {
	// Stop signals the reactor to shut down.
	Stop()

	// TeardownPeer gracefully closes a peer session with NOTIFICATION.
	// RFC 4486: Cease subcodes (2=Admin Shutdown, 3=Peer De-configured, 4=Admin Reset).
	// RFC 8203: shutdownMsg is included for subcodes 2/4 (empty = default message).
	TeardownPeer(addr netip.Addr, subcode uint8, shutdownMsg string) error

	// PausePeer pauses reading from a specific peer's session.
	// Used by flow control to apply backpressure when a plugin's worker pool saturates.
	PausePeer(addr netip.Addr) error

	// ResumePeer resumes reading from a specific peer's session.
	// Used by flow control to release backpressure when a plugin's worker pool drains.
	ResumePeer(addr netip.Addr) error

	// AddDynamicPeer adds a peer from a YANG-parsed config tree.
	// The addr is from the peer selector; tree is the peer-fields config.
	// Calls parsePeerFromTree directly (not the reload pipeline).
	AddDynamicPeer(addr netip.Addr, tree map[string]any) error

	// RemovePeer removes a peer by address.
	RemovePeer(addr netip.Addr) error

	// FlushForwardPool blocks until all forward pool workers have drained their
	// queued items to peer sockets. Used by plugins to ensure route delivery
	// before proceeding with dependent operations (e.g., teardown, withdraw).
	FlushForwardPool(ctx context.Context) error

	// FlushForwardPoolPeer blocks until the forward pool worker for a specific
	// peer address has drained its queued items. Returns nil immediately if no
	// worker exists for that peer.
	FlushForwardPoolPeer(ctx context.Context, addr string) error
}

// ReactorCacheCoordinator manages BGP cache consumer registration and cleanup.
type ReactorCacheCoordinator interface {
	// RegisterCacheConsumer initializes tracking for a cache-consumer plugin.
	// unordered=false: FIFO consumer (cumulative ack -- existing behavior).
	// unordered=true: per-entry ack only, no cumulative sweep. Required for
	// consumers like bgp-rs that process entries out of global message ID order.
	// Called when a plugin declares cache-consumer: true during Stage 1 registration.
	RegisterCacheConsumer(name string, unordered bool)

	// UnregisterCacheConsumer removes a cache-consumer plugin and adjusts pending counts.
	// Called when a cache-consumer plugin disconnects or exits.
	UnregisterCacheConsumer(name string)
}

// ReactorLifecycle is the full BGP reactor interface composed from focused
// sub-interfaces. It extends ProtocolReactor with BGP-specific peer management,
// introspection, and cache coordination.
//
// Consumers should prefer the narrowest sub-interface that satisfies their needs.
// Non-BGP code should use ProtocolReactor instead.
type ReactorLifecycle interface {
	ReactorIntrospector
	ReactorPeerController
	ReactorConfigurator
	ReactorStartupCoordinator
	ReactorCacheCoordinator
}

// PeerProcessBinding describes which plugin receives messages from a BGP peer.
type PeerProcessBinding struct {
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
	ReceiveNegotiated   bool            // Forward negotiated capabilities after OPEN exchange
	ReceiveSent         bool            // Forward sent UPDATE events
	ReceiveCustom       map[string]bool // Plugin-registered event types (e.g., "update-rpki")

	// Send settings (WHAT message types plugin can send)
	SendUpdate  bool
	SendRefresh bool
	SendCustom  map[string]bool // Plugin-registered send types (e.g., "enhanced-refresh")
}

// StateChangeReceiver receives BGP peer state change notifications.
// State events are separate from BGP protocol messages.
type StateChangeReceiver interface {
	OnPeerStateChange(peer PeerInfo, state string)
}
