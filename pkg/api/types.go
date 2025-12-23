// Package api implements the ZeBGP API layer for external communication.
//
// This package provides:
//   - Unix socket server for CLI and external tool communication
//   - Command dispatch and handlers (peer show, rib show, announce/withdraw)
//   - JSON encoder for ExaBGP v6-compatible output
//   - External process management for spawning and communicating with subprocesses
package api

import (
	"errors"
	"net/netip"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
	"github.com/exa-networks/zebgp/pkg/rib"
)

// AFI name constants for API use.
// These match the string representations used in commands and JSON output.
const (
	AFINameIPv4  = "ipv4"
	AFINameIPv6  = "ipv6"
	AFINameL2VPN = "l2vpn"
)

// SAFI name constants for API use.
// These match the string representations used in commands and JSON output.
const (
	SAFINameUnicast   = "unicast"
	SAFINameMulticast = "multicast"
	SAFINameMPLSVPN   = "mpls-vpn"
	SAFINameNLRIMPLS  = "nlri-mpls" // ExaBGP name for labeled-unicast
	SAFINameFlowSpec  = "flowspec"
	SAFINameEVPN      = "evpn"
)

// Transaction errors.
var (
	ErrAlreadyInTransaction = errors.New("already in transaction")
	ErrNoTransaction        = errors.New("no transaction in progress")
	ErrLabelMismatch        = errors.New("transaction label mismatch")
)

// TransactionResult holds the result of a commit or rollback operation.
type TransactionResult struct {
	RoutesAnnounced int      // Routes announced (on commit)
	RoutesWithdrawn int      // Routes withdrawn (on commit)
	RoutesDiscarded int      // Routes discarded (on rollback)
	UpdatesSent     int      // Number of UPDATE messages sent
	Families        []string // Address families with EOR sent
	TransactionID   string   // Transaction label
}

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

// PathAttributes holds BGP path attributes common to all route types.
// These attributes are optional - nil values use protocol defaults.
// Embedding this struct in route types ensures consistency and reduces duplication.
type PathAttributes struct {
	Origin           *uint8           // 0=IGP, 1=EGP, 2=INCOMPLETE (nil = use default)
	LocalPreference  *uint32          // LOCAL_PREF (nil = use default 100 for iBGP)
	MED              *uint32          // MULTI_EXIT_DISC (nil = not sent)
	ASPath           []uint32         // AS_PATH segments (nil = empty for iBGP)
	Communities      []uint32         // Standard communities (2-byte ASN:2-byte value)
	LargeCommunities []LargeCommunity // RFC 8092 large communities
}

// RouteSpec specifies a route for announcement.
// Supports optional BGP path attributes that override iBGP defaults.
type RouteSpec struct {
	Prefix  netip.Prefix
	NextHop netip.Addr
	PathAttributes
}

// LargeCommunity is an alias for attribute.LargeCommunity (RFC 8092).
// Using alias to avoid duplication between api and attribute packages.
type LargeCommunity = attribute.LargeCommunity

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

// L3VPNRoute specifies an L3VPN (MPLS VPN) route for announcement.
// Supports VPNv4 (AFI=1, SAFI=128) and VPNv6 (AFI=2, SAFI=128) per RFC 4364.
type L3VPNRoute struct {
	Prefix  netip.Prefix // IP prefix
	NextHop netip.Addr   // Next-hop address
	RD      string       // Route Distinguisher (e.g., "100:100" or "1.2.3.4:100")
	Labels  []uint32     // MPLS label stack (supports multiple labels per RFC 3032)
	RT      string       // Route Target (extended community, optional)
	PathAttributes
}

// LabeledUnicastRoute specifies an MPLS labeled unicast route (SAFI 4).
// This is unicast routing with MPLS labels but without VPN semantics (no RD/RT).
// RFC 8277: Using BGP to Bind MPLS Labels to Address Prefixes.
type LabeledUnicastRoute struct {
	Prefix  netip.Prefix // IP prefix
	NextHop netip.Addr   // Next-hop address
	Labels  []uint32     // MPLS label stack
	PathAttributes
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

	// AnnounceL3VPN announces an L3VPN (MPLS VPN) route to peers.
	AnnounceL3VPN(peerSelector string, route L3VPNRoute) error

	// WithdrawL3VPN withdraws an L3VPN route from peers.
	WithdrawL3VPN(peerSelector string, route L3VPNRoute) error

	// AnnounceLabeledUnicast announces an MPLS labeled unicast route (SAFI 4).
	AnnounceLabeledUnicast(peerSelector string, route LabeledUnicastRoute) error

	// WithdrawLabeledUnicast withdraws an MPLS labeled unicast route.
	WithdrawLabeledUnicast(peerSelector string, route LabeledUnicastRoute) error

	// TeardownPeer gracefully closes a peer session.
	// If reason is provided, it's sent in the NOTIFICATION message.
	TeardownPeer(addr netip.Addr, reason string) error

	// AnnounceEOR sends an End-of-RIB marker for the given address family.
	AnnounceEOR(peerSelector string, afi uint16, safi uint8) error

	// RIBInRoutes returns routes from Adj-RIB-In for the given peer.
	// If peerID is empty, returns routes from all peers.
	RIBInRoutes(peerID string) []RIBRoute

	// RIBOutRoutes returns routes from Adj-RIB-Out.
	RIBOutRoutes() []RIBRoute

	// RIBStats returns RIB statistics.
	RIBStats() RIBStatsInfo

	// Transaction support for commit-based batching.
	// peerSelector is "*" for all peers or a specific peer address.

	// BeginTransaction starts a new transaction with optional label.
	BeginTransaction(peerSelector, label string) error

	// CommitTransaction commits the current transaction.
	CommitTransaction(peerSelector string) (TransactionResult, error)

	// CommitTransactionWithLabel commits, verifying the label matches.
	CommitTransactionWithLabel(peerSelector, label string) (TransactionResult, error)

	// RollbackTransaction discards all queued routes in the transaction.
	RollbackTransaction(peerSelector string) (TransactionResult, error)

	// InTransaction returns true if a transaction is active.
	InTransaction(peerSelector string) bool

	// TransactionID returns the current transaction label.
	TransactionID(peerSelector string) string

	// SendRoutes sends routes directly to matching peers using CommitService.
	// Used by named commits to bypass OutgoingRIB transaction.
	SendRoutes(peerSelector string, routes []*rib.Route, withdrawals []nlri.NLRI, sendEOR bool) (TransactionResult, error)
}

// RIBRoute is an API-friendly representation of a route.
type RIBRoute struct {
	Peer    string `json:"peer,omitempty"`
	Prefix  string `json:"prefix"`
	NextHop string `json:"next_hop"`
	ASPath  string `json:"as_path,omitempty"`
}

// RIBStatsInfo holds RIB statistics.
type RIBStatsInfo struct {
	InPeerCount   int `json:"in_peer_count"`
	InRouteCount  int `json:"in_route_count"`
	OutPending    int `json:"out_pending"`
	OutWithdrawls int `json:"out_withdrawals"`
	OutSent       int `json:"out_sent"`
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
	WorkDir string // Working directory for process execution
}

// ServerConfig holds API server configuration.
type ServerConfig struct {
	SocketPath string          // Path to Unix socket
	Processes  []ProcessConfig // External processes to spawn
}
