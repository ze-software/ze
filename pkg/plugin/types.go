// Package api implements the ZeBGP API layer for external communication.
//
// This package provides:
//   - Unix socket server for CLI and external tool communication
//   - Command dispatch and handlers (peer show, rib show, announce/withdraw)
//   - JSON encoder for ExaBGP v6-compatible output
//   - External process management for spawning and communicating with subprocesses
package plugin

import (
	"errors"
	"fmt"
	"net/netip"
	"time"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/message"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
	"codeberg.org/thomas-mangin/zebgp/pkg/rib"
	"codeberg.org/thomas-mangin/zebgp/pkg/selector"
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
	SAFINameMUP       = "mup" // Mobile User Plane (SAFI 85)
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

// PeerCapabilityConfig holds capability configuration for a peer.
// Used by plugin protocol Stage 2 to deliver matching config.
// Values is a flexible map allowing any capability to be represented.
type PeerCapabilityConfig struct {
	Address string            // Peer IP address
	Values  map[string]string // capability-name → value (e.g., "hostname" → "router1.example.com")
}

// ReactorStats holds reactor-level statistics.
type ReactorStats struct {
	StartTime time.Time
	Uptime    time.Duration
	PeerCount int
}

// RouteSpec specifies a route for announcement.
// Supports optional BGP path attributes that override iBGP defaults.
//
// IMMUTABILITY: RouteSpec and Wire must not be mutated after being passed
// to any reactor method. The reactor stores shallow copies for efficiency;
// mutation would corrupt internal state.
type RouteSpec struct {
	Prefix  netip.Prefix
	NextHop RouteNextHop              // Encapsulates next-hop policy (explicit or self)
	Wire    *attribute.AttributesWire // Path attributes in wire format
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
	Prefix  netip.Prefix              // IP prefix
	NextHop netip.Addr                // Next-hop address
	RD      string                    // Route Distinguisher (e.g., "100:100" or "1.2.3.4:100")
	Labels  []uint32                  // MPLS label stack (supports multiple labels per RFC 3032)
	RT      string                    // Route Target (extended community, optional)
	Wire    *attribute.AttributesWire // Path attributes in wire format
}

// LabeledUnicastRoute specifies an MPLS labeled unicast route (SAFI 4).
// This is unicast routing with MPLS labels but without VPN semantics (no RD/RT).
// RFC 8277: Using BGP to Bind MPLS Labels to Address Prefixes.
// RFC 7911: ADD-PATH support via PathID field.
type LabeledUnicastRoute struct {
	Prefix  netip.Prefix              // IP prefix
	NextHop netip.Addr                // Next-hop address
	Labels  []uint32                  // MPLS label stack
	PathID  uint32                    // ADD-PATH path identifier (RFC 7911), 0 means not set
	Wire    *attribute.AttributesWire // Path attributes in wire format
}

// MUPRouteSpec specifies a MUP route for announcement (SAFI 85).
// Per draft-mpmz-bess-mup-safi for Mobile User Plane.
type MUPRouteSpec struct {
	RouteType    string                    // mup-isd, mup-dsd, mup-t1st, mup-t2st
	IsIPv6       bool                      // AFI: false=IPv4, true=IPv6
	Prefix       string                    // For ISD, T1ST (e.g., "10.0.1.0/24")
	Address      string                    // For DSD, T2ST (e.g., "10.0.0.1")
	RD           string                    // Route Distinguisher
	TEID         string                    // Tunnel Endpoint ID (for T1ST/T2ST)
	QFI          uint8                     // QoS Flow Identifier
	Endpoint     string                    // GTP endpoint address
	Source       string                    // Source address (optional)
	NextHop      string                    // Next-hop address (IPv6 for SRv6)
	ExtCommunity string                    // Extended communities (e.g., "[target:10:10]")
	PrefixSID    string                    // SRv6 Prefix SID (e.g., "l3-service 2001:db8::1 0x48 [64,24,16,0,0,0]")
	Wire         *attribute.AttributesWire // Path attributes in wire format
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

	// AnnounceMUPRoute announces a MUP route (SAFI 85) to peers.
	AnnounceMUPRoute(peerSelector string, route MUPRouteSpec) error

	// WithdrawMUPRoute withdraws a MUP route from peers.
	WithdrawMUPRoute(peerSelector string, route MUPRouteSpec) error

	// AnnounceNLRIBatch announces a batch of NLRIs with shared attributes.
	// Builds wire-format UPDATE(s), splits if exceeding peer's max message size.
	// RFC 4271 Section 4.3: UPDATE Message Format
	// RFC 4760: MP_REACH_NLRI for non-IPv4-unicast families
	// RFC 8654: Respects peer's max message size (4096 or 65535)
	AnnounceNLRIBatch(peerSelector string, batch NLRIBatch) error

	// WithdrawNLRIBatch withdraws a batch of NLRIs.
	// Builds wire-format UPDATE(s), splits if exceeding peer's max message size.
	// RFC 4271 Section 4.3: Withdrawn Routes field
	// RFC 4760: MP_UNREACH_NLRI for non-IPv4-unicast families
	WithdrawNLRIBatch(peerSelector string, batch NLRIBatch) error

	// TeardownPeer gracefully closes a peer session with NOTIFICATION.
	// Sends Cease (6) with the specified subcode per RFC 4486.
	// Common subcodes: 2=Admin Shutdown, 3=Peer De-configured, 4=Admin Reset.
	TeardownPeer(addr netip.Addr, subcode uint8) error

	// AnnounceEOR sends an End-of-RIB marker for the given address family.
	AnnounceEOR(peerSelector string, afi uint16, safi uint8) error

	// SendBoRR sends a Beginning of Route Refresh marker to matching peers.
	// RFC 7313 Section 4: "Before the speaker starts a route refresh...
	// the speaker MUST send a BoRR message."
	SendBoRR(peerSelector string, afi uint16, safi uint8) error

	// SendEoRR sends an End of Route Refresh marker to matching peers.
	// RFC 7313 Section 4: "After the speaker completes the re-advertisement
	// of the entire Adj-RIB-Out to the peer, it MUST send an EoRR message."
	SendEoRR(peerSelector string, afi uint16, safi uint8) error

	// RIBInRoutes returns routes from Adj-RIB-In for the given peer.
	// If peerID is empty, returns routes from all peers.
	// Returns rib.RouteJSON which implements json.Marshaler for efficient output.
	RIBInRoutes(peerID string) []rib.RouteJSON

	// RIBOutRoutes returns routes from Adj-RIB-Out.
	//
	// Deprecated: Adj-RIB-Out tracking removed. Always returns nil.
	RIBOutRoutes() []rib.RouteJSON

	// RIBStats returns RIB statistics.
	// Note: OutPending/OutWithdrawls/OutSent always 0 (Adj-RIB-Out removed).
	RIBStats() RIBStatsInfo

	// Transaction support for commit-based batching.
	// Use CommitManager via "commit <name> start/end/rollback" instead.
	//
	// Deprecated: Per-peer Adj-RIB-Out transactions removed.

	// BeginTransaction starts a new transaction with optional label.
	//
	// Deprecated: Use "commit <name> start" instead.
	BeginTransaction(peerSelector, label string) error

	// CommitTransaction commits the current transaction.
	//
	// Deprecated: Use "commit <name> end" instead.
	CommitTransaction(peerSelector string) (TransactionResult, error)

	// CommitTransactionWithLabel commits, verifying the label matches.
	//
	// Deprecated: Use "commit <name> end" instead.
	CommitTransactionWithLabel(peerSelector, label string) (TransactionResult, error)

	// RollbackTransaction discards all queued routes in the transaction.
	//
	// Deprecated: Use "commit <name> rollback" instead.
	RollbackTransaction(peerSelector string) (TransactionResult, error)

	// InTransaction returns true if a transaction is active.
	//
	// Deprecated: Always returns false (per-peer transactions removed).
	InTransaction(peerSelector string) bool

	// TransactionID returns the current transaction label.
	//
	// Deprecated: Always returns empty string (per-peer transactions removed).
	TransactionID(peerSelector string) string

	// SendRoutes sends routes directly to matching peers using CommitService.
	// Used by named commits to bypass OutgoingRIB transaction.
	SendRoutes(peerSelector string, routes []*rib.Route, withdrawals []nlri.NLRI, sendEOR bool) (TransactionResult, error)

	// AnnounceWatchdog announces all routes in the named watchdog group.
	// Routes are moved from withdrawn (-) to announced (+) state.
	AnnounceWatchdog(peerSelector, name string) error

	// WithdrawWatchdog withdraws all routes in the named watchdog group.
	// Routes are moved from announced (+) to withdrawn (-) state.
	WithdrawWatchdog(peerSelector, name string) error

	// AddWatchdogRoute adds a route to a global watchdog pool.
	// The route will be announced when "watchdog announce <name>" is called.
	AddWatchdogRoute(route RouteSpec, poolName string) error

	// RemoveWatchdogRoute removes a route from a global watchdog pool.
	// Returns error if pool or route doesn't exist.
	RemoveWatchdogRoute(routeKey, poolName string) error

	// ClearRIBIn clears all routes in Adj-RIB-In.
	// Returns count of routes cleared.
	ClearRIBIn() int

	// ClearRIBOut withdraws all routes from Adj-RIB-Out.
	//
	// Deprecated: Adj-RIB-Out tracking removed. Always returns 0.
	ClearRIBOut() int

	// FlushRIBOut re-queues all sent routes for re-announcement.
	//
	// Deprecated: Adj-RIB-Out tracking removed. Always returns 0.
	FlushRIBOut() int

	// GetPeerProcessBindings returns process bindings for a specific peer.
	// Used to determine which plugins receive messages from this peer.
	GetPeerProcessBindings(peerAddr netip.Addr) []PeerProcessBinding

	// ForwardUpdate forwards a cached UPDATE to peers matching the selector.
	// One-shot: deletes from cache after forwarding.
	// Returns error if update-id is expired or not found.
	ForwardUpdate(sel *selector.Selector, updateID uint64) error

	// DeleteUpdate removes an update from the cache without forwarding.
	// Used when controller decides not to forward (filtering).
	DeleteUpdate(updateID uint64) error

	// RetainUpdate prevents eviction of a cached UPDATE.
	// Used by API for graceful restart - retain routes for replay.
	// Returns error if update-id is not found.
	RetainUpdate(updateID uint64) error

	// ReleaseUpdate allows eviction of a previously retained UPDATE.
	// Resets TTL to default expiration time.
	// Returns error if update-id is not found.
	ReleaseUpdate(updateID uint64) error

	// ListUpdates returns all cached msg-ids (retained or non-expired).
	ListUpdates() []uint64

	// SignalAPIReady signals that an API process is ready.
	// When all processes have signaled, WaitForAPIReady returns.
	SignalAPIReady()

	// SignalPeerAPIReady signals that a peer-specific API initialization is complete.
	// Called when "peer <addr> session api ready" is received (e.g., after route replay).
	SignalPeerAPIReady(peerAddr string)

	// SendRawMessage sends raw bytes to a peer.
	// If msgType is 0, payload is a full BGP packet (user provides marker+header).
	// If msgType is non-zero, payload is message body (ZeBGP adds header).
	// No validation - bytes sent exactly as provided.
	SendRawMessage(peerAddr netip.Addr, msgType uint8, payload []byte) error

	// GetPeerCapabilityConfigs returns capability configurations for all peers.
	// Used by plugin protocol Stage 2 to deliver matching config.
	GetPeerCapabilityConfigs() []PeerCapabilityConfig
}

// PeerProcessBinding describes which plugin receives messages from a peer.
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
	ReceiveSent         bool // Forward sent UPDATE events

	// Send settings (WHAT message types plugin can send)
	SendUpdate  bool
	SendRefresh bool
}

// StateChangeReceiver receives peer state change notifications.
// State events are separate from BGP protocol messages.
type StateChangeReceiver interface {
	OnPeerStateChange(peer PeerInfo, state string)
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
// Serial is included only if command had #N prefix.
type Response struct {
	Serial  string `json:"serial,omitempty"`  // Correlation ID (omitted if no prefix)
	Status  string `json:"status"`            // "done", "error", or "partial"
	Partial bool   `json:"partial,omitempty"` // True for streaming chunks, false for final
	Data    any    `json:"data,omitempty"`    // Payload (success data or error message)
}

// PluginConfig holds external plugin configuration.
type PluginConfig struct {
	Name           string // Plugin identifier
	Run            string // Command to execute
	Encoder        string // "json" or "text"
	Respawn        bool   // ExaBGP compat (prefer RespawnEnabled)
	RespawnEnabled bool   // Respawn with limit enforcement (5/60s)
	WorkDir        string // Working directory for plugin execution
	ReceiveUpdate  bool   // Forward received UPDATEs to plugin stdin
}

// ServerConfig holds API server configuration.
type ServerConfig struct {
	SocketPath string         // Path to Unix socket
	Plugins    []PluginConfig // External plugins to spawn
}

// Format constants for process output formatting.
const (
	FormatParsed = "parsed" // Decoded/interpreted fields only (default)
	FormatRaw    = "raw"    // Wire bytes only (hex)
	FormatFull   = "full"   // Both parsed content AND raw bytes
)

// WireEncoding specifies how wire bytes are encoded in API messages.
// Controls encoding for both inbound (events to process) and outbound (commands from process).
type WireEncoding uint8

// Wire encoding constants.
const (
	WireEncodingHex  WireEncoding = iota // Hex string (default, human-readable)
	WireEncodingB64                      // Base64 (33% overhead, compact)
	WireEncodingCBOR                     // CBOR binary (0% overhead, native)
	WireEncodingText                     // Parsed text (no wire bytes)
)

// Wire encoding name constants.
const (
	wireEncHex  = "hex"
	wireEncB64  = "b64"
	wireEncCBOR = "cbor"
)

// String returns the encoding name.
func (e WireEncoding) String() string {
	switch e {
	case WireEncodingHex:
		return wireEncHex
	case WireEncodingB64:
		return wireEncB64
	case WireEncodingCBOR:
		return wireEncCBOR
	case WireEncodingText:
		return EncodingText
	default:
		return wireEncHex
	}
}

// ParseWireEncoding converts a string to WireEncoding.
// Returns error for unknown encodings.
func ParseWireEncoding(s string) (WireEncoding, error) {
	switch s {
	case wireEncHex:
		return WireEncodingHex, nil
	case wireEncB64, "base64":
		return WireEncodingB64, nil
	case wireEncCBOR:
		return WireEncodingCBOR, nil
	case EncodingText:
		return WireEncodingText, nil
	default:
		return WireEncodingHex, fmt.Errorf("invalid wire encoding: %q (valid: hex, b64, cbor, text)", s)
	}
}

// Status constants for API responses.
const (
	statusDone  = "done"
	statusError = "error"
)

// ContentConfig controls HOW messages are formatted (encoding + format).
// Separated from message type subscriptions (WHAT) per new API design.
type ContentConfig struct {
	Encoding   string           // "json" | "text" (default: "text")
	Format     string           // "parsed" | "raw" | "full" (default: "parsed")
	Attributes *AttributeFilter // Which attrs to include (nil = all)
	NLRI       *NLRIFilter      // Which address families to include (nil = all)
}

// WithDefaults returns a ContentConfig with default values applied.
func (c ContentConfig) WithDefaults() ContentConfig {
	if c.Encoding == "" {
		c.Encoding = EncodingText
	}
	if c.Format == "" {
		c.Format = FormatParsed
	}
	return c
}

// RawMessage represents a BGP message sent or received.
// Contains raw wire bytes for on-demand parsing based on format config.
type RawMessage struct {
	Type       message.MessageType // UPDATE, OPEN, NOTIFICATION, etc.
	RawBytes   []byte              // Original wire bytes (without marker/header)
	Timestamp  time.Time
	MessageID  uint64                    // Unique ID for all message types
	AttrsWire  *attribute.AttributesWire // Lazy attribute parsing (nil if not UPDATE or parse failed)
	WireUpdate *WireUpdate               // UPDATE wire wrapper (nil if not UPDATE)
	Direction  string                    // "sent" or "received"
	ParseError error                     // Non-nil if lazy parsing failed
}

// IsAsyncSafe reports whether this message's RawBytes can be safely used after
// the callback returns. Returns false for zero-copy received UPDATEs where
// RawBytes points to a buffer that may be reused.
func (m *RawMessage) IsAsyncSafe() bool {
	return m.WireUpdate == nil
}

// NLRIGroup represents a group of NLRIs sharing the same attributes.
// Used by ParseUpdateText to capture attribute snapshots per NLRI section.
type NLRIGroup struct {
	Family       nlri.Family               // Address family (AFI/SAFI)
	Announce     []nlri.NLRI               // NLRIs to announce
	Withdraw     []nlri.NLRI               // NLRIs to withdraw
	Wire         *attribute.AttributesWire // Path attributes in wire format
	NextHop      RouteNextHop              // Encapsulates next-hop policy (explicit or self)
	WatchdogName string                    // Watchdog pool name for announce routes (empty = none)
}

// UpdateTextResult is the parsed result of an update text command.
type UpdateTextResult struct {
	Groups       []NLRIGroup
	WatchdogName string
}

// NLRIBatch represents a batch of NLRIs with shared attributes.
// Used for efficient UPDATE message generation - reactor builds wire format
// and splits into multiple messages if exceeding peer's max size.
// RFC 4271 Section 4.3: UPDATE Message Format.
// RFC 4760: MP_REACH_NLRI/MP_UNREACH_NLRI for non-IPv4-unicast families.
type NLRIBatch struct {
	Family  nlri.Family               // AFI/SAFI for all NLRIs
	NLRIs   []nlri.NLRI               // NLRIs to announce or withdraw
	NextHop RouteNextHop              // Next-hop policy (announce only)
	Attrs   *attribute.Builder        // Attribute builder (for new routes)
	Wire    *attribute.AttributesWire // Wire passthrough (for forwarding)
}
