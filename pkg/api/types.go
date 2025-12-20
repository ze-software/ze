// Package api implements the ZeBGP API layer for external communication.
//
// This package provides:
//   - Unix socket server for CLI and external tool communication
//   - Command dispatch and handlers (peer show, rib show, announce/withdraw)
//   - JSON encoder for ExaBGP v6-compatible output
//   - External process management for spawning and communicating with subprocesses
package api

import (
	"net/netip"
	"time"
)

// PeerInfo is a snapshot of peer state for API output.
type PeerInfo struct {
	Address      netip.Addr
	LocalAddress netip.Addr
	LocalAS      uint32
	PeerAS       uint32
	RouterID     uint32
	State        string
	Uptime       time.Duration

	// Statistics
	MessagesReceived uint64
	MessagesSent     uint64
	RoutesReceived   uint32
	RoutesSent       uint32
}

// ReactorStats holds reactor-level statistics.
type ReactorStats struct {
	StartTime time.Time
	Uptime    time.Duration
	PeerCount int
}

// RouteSpec specifies a route for announcement.
type RouteSpec struct {
	Prefix  netip.Prefix
	NextHop netip.Addr
	// TODO: Add attributes (origin, as-path, communities, etc.)
}

// FlowSpecRoute specifies a FlowSpec route for announcement.
type FlowSpecRoute struct {
	Family       string          // "ipv4" or "ipv6"
	DestPrefix   *netip.Prefix   // Destination prefix match
	SourcePrefix *netip.Prefix   // Source prefix match
	Protocols    []uint8         // IP protocol numbers
	Ports        []uint16        // Port numbers (src or dst)
	DestPorts    []uint16        // Destination ports
	SourcePorts  []uint16        // Source ports
	Actions      FlowSpecActions // Traffic actions
}

// FlowSpecActions specifies what to do with matching traffic.
type FlowSpecActions struct {
	Accept    bool   // Accept traffic (default)
	Discard   bool   // Drop traffic
	RateLimit uint32 // Rate limit in bps (0 = no limit)
	Redirect  string // Redirect target (RT or IP)
	MarkDSCP  uint8  // DSCP marking value
}

// VPLSRoute specifies a VPLS route for announcement.
type VPLSRoute struct {
	RD            string // Route distinguisher (e.g., "65000:100")
	VEBlockOffset uint16 // VE block offset
	VEBlockSize   uint16 // VE block size
	LabelBase     uint32 // Base MPLS label
	NextHop       netip.Addr
}

// L2VPNRoute specifies an L2VPN/EVPN route for announcement.
type L2VPNRoute struct {
	RouteType   string // "mac-ip", "ip-prefix", "multicast", "ethernet-segment", "ethernet-ad"
	RD          string // Route distinguisher
	EthernetTag uint32 // Ethernet Tag ID

	// For mac-ip (Type 2)
	MAC string     // MAC address (e.g., "00:11:22:33:44:55")
	IP  netip.Addr // Optional IP address
	ESI string     // Ethernet Segment Identifier

	// For ip-prefix (Type 5)
	Prefix  netip.Prefix // IP prefix
	Gateway netip.Addr   // Gateway IP

	// Labels
	Label1 uint32 // First MPLS label
	Label2 uint32 // Second MPLS label (optional)

	// Next-hop
	NextHop netip.Addr
}

// ReactorInterface defines what the API needs from the reactor.
// This interface avoids import cycles between pkg/api and pkg/reactor.
type ReactorInterface interface {
	// Peers returns information about all configured peers.
	Peers() []PeerInfo

	// Stats returns reactor-level statistics.
	Stats() ReactorStats

	// Stop signals the reactor to shut down.
	Stop()

	// Reload reloads the configuration.
	Reload() error

	// AnnounceRoute announces a route to peers matching the selector.
	// Selector can be "*" for all peers or a specific IP address.
	AnnounceRoute(peerSelector string, route RouteSpec) error

	// WithdrawRoute withdraws a route from peers matching the selector.
	WithdrawRoute(peerSelector string, prefix netip.Prefix) error

	// AnnounceFlowSpec announces a FlowSpec route to peers.
	AnnounceFlowSpec(peerSelector string, route FlowSpecRoute) error

	// WithdrawFlowSpec withdraws a FlowSpec route from peers.
	WithdrawFlowSpec(peerSelector string, route FlowSpecRoute) error

	// AnnounceVPLS announces a VPLS route to peers.
	AnnounceVPLS(peerSelector string, route VPLSRoute) error

	// WithdrawVPLS withdraws a VPLS route from peers.
	WithdrawVPLS(peerSelector string, route VPLSRoute) error

	// AnnounceL2VPN announces an L2VPN/EVPN route to peers.
	AnnounceL2VPN(peerSelector string, route L2VPNRoute) error

	// WithdrawL2VPN withdraws an L2VPN/EVPN route from peers.
	WithdrawL2VPN(peerSelector string, route L2VPNRoute) error

	// TeardownPeer gracefully closes a peer session.
	// If reason is provided, it's sent in the NOTIFICATION message.
	TeardownPeer(addr netip.Addr, reason string) error
}

// Response represents an API command response.
type Response struct {
	Status string `json:"status"`          // "done" or "error"
	Error  string `json:"error,omitempty"` // Error message if status="error"
	Data   any    `json:"data,omitempty"`  // Result data if applicable
}

// ProcessConfig holds external process configuration.
type ProcessConfig struct {
	Name    string // Process identifier
	Run     string // Command to execute
	Encoder string // "json" (only v6 supported)
	Respawn bool   // Respawn on exit
}

// ServerConfig holds API server configuration.
type ServerConfig struct {
	SocketPath string          // Path to Unix socket
	Processes  []ProcessConfig // External processes to spawn
}
