// Package plugin implements ze plugin infrastructure for external communication.
//
// This package provides:
//   - Unix socket server for CLI and external tool communication
//   - Command dispatch and handlers (peer show, rib show, announce/withdraw)
//   - JSON encoder for ze-bgp format output
//   - External process management for spawning and communicating with subprocesses
package plugin

import (
	"fmt"
	"net/netip"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	bgpfilter "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/filter"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/rib"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/selector"
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

// PeerCapabilityConfig holds capability configuration for a peer.
// Used by plugin protocol Stage 2 to deliver matching config.
// Values is a flexible map allowing any capability to be represented.
type PeerCapabilityConfig struct {
	Address        string            // Peer IP address
	Values         map[string]string // capability-name → value (e.g., "hostname" → "router1.example.com")
	CapabilityJSON string            // Full capability block as JSON - plugins extract what they need
}

// ReactorStats holds reactor-level statistics.
type ReactorStats struct {
	StartTime time.Time
	Uptime    time.Duration
	PeerCount int
}

// DynamicPeerConfig contains minimal configuration for adding a peer dynamically.
// Used by "bgp peer <ip> add" command to add peers at runtime.
type DynamicPeerConfig struct {
	Address      netip.Addr    // Peer IP address (required)
	PeerAS       uint32        // Peer AS number (required)
	LocalAS      uint32        // Local AS number (optional, use reactor default if 0)
	LocalAddress netip.Addr    // Local IP for this session (optional)
	RouterID     uint32        // Router ID (optional, use reactor default if 0)
	HoldTime     time.Duration // Hold time (optional, use default if 0)
	Passive      bool          // Passive mode (listen-only)
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

	// AddDynamicPeer adds a peer with the given configuration.
	// Used by "bgp peer <ip> add" command for runtime peer management.
	AddDynamicPeer(config DynamicPeerConfig) error

	// RemovePeer removes a peer by address.
	// Used by "bgp peer <ip> remove" command for runtime peer management.
	RemovePeer(addr netip.Addr) error

	// Reload reloads the configuration from the config file via reloadFunc.
	Reload() error

	// VerifyConfig validates peer settings from a BGP config tree without modifying state.
	// Called by the reload coordinator during the verify phase.
	VerifyConfig(bgpTree map[string]any) error

	// ApplyConfigDiff applies peer changes from a BGP config tree.
	// Computes diff between current peers and new config, removes/adds as needed.
	// Called by the reload coordinator during the apply phase.
	ApplyConfigDiff(bgpTree map[string]any) error

	// AnnounceRoute announces a route to peers matching the selector.
	// Selector can be "*" for all peers or a specific IP address.
	AnnounceRoute(peerSelector string, route bgptypes.RouteSpec) error

	// WithdrawRoute withdraws a route from peers matching the selector.
	WithdrawRoute(peerSelector string, prefix netip.Prefix) error

	// AnnounceFlowSpec announces a FlowSpec route to peers.
	AnnounceFlowSpec(peerSelector string, route bgptypes.FlowSpecRoute) error

	// WithdrawFlowSpec withdraws a FlowSpec route from peers.
	WithdrawFlowSpec(peerSelector string, route bgptypes.FlowSpecRoute) error

	// AnnounceVPLS announces a VPLS route to peers.
	AnnounceVPLS(peerSelector string, route bgptypes.VPLSRoute) error

	// WithdrawVPLS withdraws a VPLS route from peers.
	WithdrawVPLS(peerSelector string, route bgptypes.VPLSRoute) error

	// AnnounceL2VPN announces an L2VPN/EVPN route to peers.
	AnnounceL2VPN(peerSelector string, route bgptypes.L2VPNRoute) error

	// WithdrawL2VPN withdraws an L2VPN/EVPN route from peers.
	WithdrawL2VPN(peerSelector string, route bgptypes.L2VPNRoute) error

	// AnnounceL3VPN announces an L3VPN (MPLS VPN) route to peers.
	AnnounceL3VPN(peerSelector string, route bgptypes.L3VPNRoute) error

	// WithdrawL3VPN withdraws an L3VPN route from peers.
	WithdrawL3VPN(peerSelector string, route bgptypes.L3VPNRoute) error

	// AnnounceLabeledUnicast announces an MPLS labeled unicast route (SAFI 4).
	AnnounceLabeledUnicast(peerSelector string, route bgptypes.LabeledUnicastRoute) error

	// WithdrawLabeledUnicast withdraws an MPLS labeled unicast route.
	WithdrawLabeledUnicast(peerSelector string, route bgptypes.LabeledUnicastRoute) error

	// AnnounceMUPRoute announces a MUP route (SAFI 85) to peers.
	AnnounceMUPRoute(peerSelector string, route bgptypes.MUPRouteSpec) error

	// WithdrawMUPRoute withdraws a MUP route from peers.
	WithdrawMUPRoute(peerSelector string, route bgptypes.MUPRouteSpec) error

	// AnnounceNLRIBatch announces a batch of NLRIs with shared attributes.
	// Builds wire-format UPDATE(s), splits if exceeding peer's max message size.
	// RFC 4271 Section 4.3: UPDATE Message Format
	// RFC 4760: MP_REACH_NLRI for non-IPv4-unicast families
	// RFC 8654: Respects peer's max message size (4096 or 65535)
	AnnounceNLRIBatch(peerSelector string, batch bgptypes.NLRIBatch) error

	// WithdrawNLRIBatch withdraws a batch of NLRIs.
	// Builds wire-format UPDATE(s), splits if exceeding peer's max message size.
	// RFC 4271 Section 4.3: Withdrawn Routes field
	// RFC 4760: MP_UNREACH_NLRI for non-IPv4-unicast families
	WithdrawNLRIBatch(peerSelector string, batch bgptypes.NLRIBatch) error

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
	CommitTransaction(peerSelector string) (bgptypes.TransactionResult, error)

	// CommitTransactionWithLabel commits, verifying the label matches.
	//
	// Deprecated: Use "commit <name> end" instead.
	CommitTransactionWithLabel(peerSelector, label string) (bgptypes.TransactionResult, error)

	// RollbackTransaction discards all queued routes in the transaction.
	//
	// Deprecated: Use "commit <name> rollback" instead.
	RollbackTransaction(peerSelector string) (bgptypes.TransactionResult, error)

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
	SendRoutes(peerSelector string, routes []*rib.Route, withdrawals []nlri.NLRI, sendEOR bool) (bgptypes.TransactionResult, error)

	// AnnounceWatchdog announces all routes in the named watchdog group.
	// Routes are moved from withdrawn (-) to announced (+) state.
	AnnounceWatchdog(peerSelector, name string) error

	// WithdrawWatchdog withdraws all routes in the named watchdog group.
	// Routes are moved from announced (+) to withdrawn (-) state.
	WithdrawWatchdog(peerSelector, name string) error

	// AddWatchdogRoute adds a route to a global watchdog pool.
	// The route will be announced when "watchdog announce <name>" is called.
	AddWatchdogRoute(route bgptypes.RouteSpec, poolName string) error

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

	// AddAPIProcessCount adds to the number of API processes to wait for.
	// Used for two-phase plugin startup: Phase 1 (explicit) + Phase 2 (auto-load).
	AddAPIProcessCount(count int)

	// SignalPluginStartupComplete signals that all plugin phases are done.
	// Called by Server after Phase 1 (explicit) + Phase 2 (auto-load) complete.
	SignalPluginStartupComplete()

	// SignalPeerAPIReady signals that a peer-specific API initialization is complete.
	// Called when "peer <addr> plugin session ready" is received (e.g., after route replay).
	SignalPeerAPIReady(peerAddr string)

	// SendRawMessage sends raw bytes to a peer.
	// If msgType is 0, payload is a full BGP packet (user provides marker+header).
	// If msgType is non-zero, payload is message body (ze adds header).
	// No validation - bytes sent exactly as provided.
	SendRawMessage(peerAddr netip.Addr, msgType uint8, payload []byte) error

	// GetPeerCapabilityConfigs returns capability configurations for all peers.
	// Used by plugin protocol Stage 2 to deliver matching config.
	GetPeerCapabilityConfigs() []PeerCapabilityConfig

	// GetConfigTree returns the full config as a map for plugin config delivery.
	// Plugins request specific roots (e.g., "bgp", "environment") and receive JSON.
	GetConfigTree() map[string]any

	// SetConfigTree replaces the running config tree after a successful reload.
	// Called by the reload coordinator after verify→apply completes.
	SetConfigTree(tree map[string]any)
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
	ReceiveNegotiated   bool // Forward negotiated capabilities after OPEN exchange
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

// ResponseWrapper wraps a Response with type field for ze-bgp JSON.
// All responses are wrapped: {"type":"response","response":{...}}.
type ResponseWrapper struct {
	Type     string    `json:"type"`     // Always "response"
	Response *Response `json:"response"` // Payload
}

// WrapResponse wraps a Response in a ResponseWrapper for ze-bgp JSON.
func WrapResponse(r *Response) *ResponseWrapper {
	return &ResponseWrapper{
		Type:     "response",
		Response: r,
	}
}

// NewResponse creates a new Response with the given status and data.
func NewResponse(status string, data any) *Response {
	return &Response{
		Status: status,
		Data:   data,
	}
}

// NewErrorResponse creates an error Response with the given message.
func NewErrorResponse(message string) *Response {
	return &Response{
		Status: StatusError,
		Data:   message,
	}
}

// PluginConfig holds plugin configuration.
type PluginConfig struct {
	Name           string        // Plugin identifier
	Run            string        // Command to execute (empty for internal plugins)
	Encoder        string        // "json" or "text"
	Respawn        bool          // ExaBGP compat (prefer RespawnEnabled)
	RespawnEnabled bool          // Respawn with limit enforcement (5/60s)
	WorkDir        string        // Working directory for plugin execution
	ReceiveUpdate  bool          // Forward received UPDATEs to plugin stdin
	StageTimeout   time.Duration // Per-stage timeout (0 = use default 5s)
	Internal       bool          // If true, run in-process via goroutine (ze.X plugins)
}

// ServerConfig holds API server configuration.
type ServerConfig struct {
	SocketPath         string         // Path to Unix socket
	Plugins            []PluginConfig // External plugins to spawn
	ConfiguredFamilies []string       // Families configured on peers (for deferred auto-load)
}

// Format constants for process output formatting.
const (
	FormatHex    = "hex"    // Wire bytes as hex string
	FormatBase64 = "base64" // Wire bytes as base64
	FormatParsed = "parsed" // Decoded/interpreted fields only (default)
	FormatRaw    = "raw"    // Wire bytes only (hex) - alias for FormatHex
	FormatFull   = "full"   // Both parsed content AND raw bytes
)

// WireEncoding specifies how wire bytes are encoded in API messages.
// Controls encoding for both inbound (events to process) and outbound (commands from process).
type WireEncoding uint8

// Wire encoding constants.
const (
	WireEncodingHex  WireEncoding = iota // Hex string (default, human-readable)
	WireEncodingB64                      // Base64 (33% overhead, compact)
	WireEncodingText                     // Parsed text (no wire bytes)
)

// Wire encoding name constants.
const (
	wireEncHex = "hex"
	wireEncB64 = "b64"
)

// String returns the encoding name.
func (e WireEncoding) String() string {
	switch e {
	case WireEncodingHex:
		return wireEncHex
	case WireEncodingB64:
		return wireEncB64
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
	case EncodingText:
		return WireEncodingText, nil
	default:
		return WireEncodingHex, fmt.Errorf("invalid wire encoding: %q (valid: hex, b64, text)", s)
	}
}

// Status constants for API responses.
const (
	StatusDone  = "done"
	StatusError = "error"
)

// cmdPlugin is the "plugin" token in command strings like "ze bgp plugin <name>".
const cmdPlugin = "plugin"

// ContentConfig controls HOW messages are formatted (encoding + format).
// Separated from message type subscriptions (WHAT) per new API design.
type ContentConfig struct {
	Encoding   string                     // "json" | "text" (default: "text")
	Format     string                     // "parsed" | "raw" | "full" (default: "parsed")
	Attributes *bgpfilter.AttributeFilter // Which attrs to include (nil = all)
	NLRI       *bgpfilter.NLRIFilter      // Which address families to include (nil = all)
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
	WireUpdate *wireu.WireUpdate         // UPDATE wire wrapper (nil if not UPDATE)
	Direction  string                    // "sent" or "received"
	ParseError error                     // Non-nil if lazy parsing failed
}

// IsAsyncSafe reports whether this message's RawBytes can be safely used after
// the callback returns. Returns false for zero-copy received UPDATEs where
// RawBytes points to a buffer that may be reused.
func (m *RawMessage) IsAsyncSafe() bool {
	return m.WireUpdate == nil
}
