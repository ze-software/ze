// Design: docs/architecture/api/process-protocol.md — plugin process management
//
// Package plugin implements ze plugin infrastructure for external communication.
//
// This package provides:
//   - Unix socket server for CLI and external tool communication
//   - Command dispatch and handlers (peer detail, rib routes, announce/withdraw)
//   - JSON encoder for ze-bgp format output
//   - External process management for spawning and communicating with subprocesses
package plugin

import (
	"context"
	"fmt"
	"net/netip"
	"time"
)

// Encoding constants for process output formatting.
const (
	EncodingJSON = "json"
	EncodingText = "text"
)

// PeerInfo is a snapshot of peer state for API output.
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

	// PrefixUpdated is the ISO date (YYYY-MM-DD) when prefix maximums were
	// last updated from PeeringDB. Empty means manually configured.
	PrefixUpdated string

	// PrefixWarnings lists families where the received prefix count exceeds
	// the configured warning threshold. Updated in real-time by the session.
	// Empty when no family is in warning state (or session not established).
	PrefixWarnings []string

	// Statistics (engine-level counters; NLRI-level counters live in the RIB plugin)
	UpdatesReceived    uint32
	UpdatesSent        uint32
	KeepalivesReceived uint32
	KeepalivesSent     uint32
	EORReceived        uint32
	EORSent            uint32
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
	RouterID  uint32 // Local BGP router identifier (uint32 IP)
	LocalAS   uint32 // Local AS number
}

// PeerCapabilitiesInfo holds negotiated and configured capability data for API display.
type PeerCapabilitiesInfo struct {
	Families             []string          // Negotiated address families (e.g., "ipv4/unicast")
	ExtendedMessage      bool              // RFC 8654: Extended message support
	EnhancedRouteRefresh bool              // RFC 7313: Enhanced route refresh
	ASN4                 bool              // RFC 6793: 4-byte ASN support
	AddPath              map[string]string // RFC 7911: family → "send" for families with ADD-PATH (nil if none)
}

// ReactorIntrospector provides read-only access to peer and reactor state.
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

// ReactorPeerController manages peer lifecycle: shutdown, teardown, flow control,
// and dynamic peer add/remove.
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

// ReactorConfigurator handles configuration reload, verification, and application.
type ReactorConfigurator interface {
	// Reload reloads the configuration from the config file via reloadFunc.
	Reload() error

	// VerifyConfig validates protocol-specific settings from a config tree.
	VerifyConfig(configTree map[string]any) error

	// ApplyConfigDiff applies incremental changes from a protocol config tree.
	ApplyConfigDiff(configTree map[string]any) error

	// GetConfigTree returns the full config as a map for plugin config delivery.
	GetConfigTree() map[string]any

	// SetConfigTree replaces the running config tree after a successful reload.
	SetConfigTree(tree map[string]any)
}

// ReactorStartupCoordinator handles plugin startup protocol signaling.
type ReactorStartupCoordinator interface {
	// SignalAPIReady signals that an API process is ready.
	SignalAPIReady()

	// AddAPIProcessCount adds to the number of API processes to wait for.
	AddAPIProcessCount(count int)

	// SignalPluginStartupComplete signals that all plugin phases are done.
	SignalPluginStartupComplete()

	// SignalPeerAPIReady signals that a peer-specific API initialization is complete.
	SignalPeerAPIReady(peerAddr string)
}

// ReactorCacheCoordinator manages cache consumer registration and cleanup.
type ReactorCacheCoordinator interface {
	// RegisterCacheConsumer initializes tracking for a cache-consumer plugin.
	// unordered=false: FIFO consumer (cumulative ack — existing behavior).
	// unordered=true: per-entry ack only, no cumulative sweep. Required for
	// consumers like bgp-rs that process entries out of global message ID order.
	// Called when a plugin declares cache-consumer: true during Stage 1 registration.
	RegisterCacheConsumer(name string, unordered bool)

	// UnregisterCacheConsumer removes a cache-consumer plugin and adjusts pending counts.
	// Called when a cache-consumer plugin disconnects or exits.
	UnregisterCacheConsumer(name string)
}

// ProtocolReactor is the minimal interface any protocol reactor must implement.
// It provides lifecycle management and configuration access that the engine
// and plugin infrastructure use without knowledge of the specific protocol.
//
// Protocol-specific extensions (BGP peers, OSPF neighbors, IS-IS adjacencies)
// are expressed as separate interfaces. Consumers type-assert when they need
// protocol-specific operations.
type ProtocolReactor interface {
	// Stop signals the reactor to shut down.
	Stop()

	// Reload reloads the configuration.
	Reload() error

	// GetConfigTree returns the full config as a map for plugin config delivery.
	GetConfigTree() map[string]any

	// SetConfigTree replaces the running config tree after a successful reload.
	SetConfigTree(tree map[string]any)

	// VerifyConfig validates protocol-specific settings from a config tree.
	VerifyConfig(configTree map[string]any) error

	// ApplyConfigDiff applies incremental changes from a protocol config tree.
	ApplyConfigDiff(configTree map[string]any) error

	// SignalPluginStartupComplete signals that all plugin phases are done.
	SignalPluginStartupComplete()
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
	ReceiveNegotiated   bool            // Forward negotiated capabilities after OPEN exchange
	ReceiveSent         bool            // Forward sent UPDATE events
	ReceiveCustom       map[string]bool // Plugin-registered event types (e.g., "update-rpki")

	// Send settings (WHAT message types plugin can send)
	SendUpdate  bool
	SendRefresh bool
	SendCustom  map[string]bool // Plugin-registered send types (e.g., "enhanced-refresh")
}

// StateChangeReceiver receives peer state change notifications.
// State events are separate from BGP protocol messages.
type StateChangeReceiver interface {
	OnPeerStateChange(peer PeerInfo, state string)
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

// ProcessSpawner is the interface for plugin process lifecycle management.
// Implemented by PluginManager. Used by Server to delegate process creation
// instead of creating ProcessManager directly.
type ProcessSpawner interface {
	// SpawnMore spawns additional plugin processes (for auto-load).
	SpawnMore(configs []PluginConfig) error

	// GetProcessManager returns the most recent ProcessManager.
	// Returns nil if no processes have been spawned.
	GetProcessManager() any
}

// HubServerConfig holds a named hub server block.
// Extracted from: plugin { hub { server <name> { host; port; secret; } } }.
type HubServerConfig struct {
	Name    string            // Server block name (e.g., "local", "central")
	Host    string            // Listen address
	Port    uint16            // Listen port
	Secret  string            `json:"-"` // Auth token for plugin connections
	Clients map[string]string `json:"-"` // Per-client secrets: name -> secret
}

// Address returns "host:port" for net.Listen.
func (s HubServerConfig) Address() string {
	return s.Host + ":" + fmt.Sprintf("%d", s.Port)
}

// HubClientConfig holds a hub-level client block (outbound connection).
// Extracted from: plugin { hub { client <name> { host; port; secret; } } }.
type HubClientConfig struct {
	Name   string // Client identity name
	Host   string // Remote hub address
	Port   uint16 // Remote hub port
	Secret string `json:"-"` // Auth token
}

// Address returns "host:port" for net.Dial.
func (c HubClientConfig) Address() string {
	return c.Host + ":" + fmt.Sprintf("%d", c.Port)
}

// HubConfig holds plugin transport configuration.
// Extracted from: plugin { hub { server ...; client ...; } }.
type HubConfig struct {
	Servers []HubServerConfig // Named server blocks (listeners)
	Clients []HubClientConfig // Hub-level client blocks (outbound)
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

// Format constants for process output formatting.
const (
	FormatHex     = "hex"     // Wire bytes as hex string
	FormatBase64  = "base64"  // Wire bytes as base64
	FormatParsed  = "parsed"  // Decoded/interpreted fields only (default)
	FormatRaw     = "raw"     // Wire bytes only (hex) - alias for FormatHex
	FormatFull    = "full"    // Both parsed content AND raw bytes
	FormatSummary = "summary" // NLRI metadata only (families + announce/withdraw presence)
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
	StatusOK    = "ok"
)

// cmdPlugin is the "plugin" token in command strings like "ze plugin <name>".
const cmdPlugin = "plugin"
