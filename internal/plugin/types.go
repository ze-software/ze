// Design: docs/architecture/api/process-protocol.md — plugin process management
//
// Package plugin implements ze plugin infrastructure for external communication.
//
// This package provides:
//   - Unix socket server for CLI and external tool communication
//   - Command dispatch and handlers (peer show, rib show, announce/withdraw)
//   - JSON encoder for ze-bgp format output
//   - External process management for spawning and communicating with subprocesses
package plugin

import (
	"encoding/json"
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
	Connection   string        // Connection mode: "both" (default), "passive", "active"
}

// ReactorLifecycle defines generic lifecycle operations for the reactor.
// These methods are protocol-agnostic and handle peer management,
// configuration, introspection, and startup coordination.
type ReactorLifecycle interface {
	// --- Introspection (4 methods) ---

	// Peers returns information about all configured peers.
	Peers() []PeerInfo

	// Stats returns reactor-level statistics.
	Stats() ReactorStats

	// GetPeerProcessBindings returns process bindings for a specific peer.
	GetPeerProcessBindings(peerAddr netip.Addr) []PeerProcessBinding

	// GetPeerCapabilityConfigs returns capability configurations for all peers.
	GetPeerCapabilityConfigs() []PeerCapabilityConfig

	// --- Lifecycle (2 methods) ---

	// Stop signals the reactor to shut down.
	Stop()

	// TeardownPeer gracefully closes a peer session with NOTIFICATION.
	// RFC 4486: Cease subcodes (2=Admin Shutdown, 3=Peer De-configured, 4=Admin Reset).
	TeardownPeer(addr netip.Addr, subcode uint8) error

	// PausePeer pauses reading from a specific peer's session.
	// Used by flow control to apply backpressure when a plugin's worker pool saturates.
	PausePeer(addr netip.Addr) error

	// ResumePeer resumes reading from a specific peer's session.
	// Used by flow control to release backpressure when a plugin's worker pool drains.
	ResumePeer(addr netip.Addr) error

	// --- Configuration (6 methods) ---

	// Reload reloads the configuration from the config file via reloadFunc.
	Reload() error

	// VerifyConfig validates peer settings from a BGP config tree.
	VerifyConfig(bgpTree map[string]any) error

	// ApplyConfigDiff applies peer changes from a BGP config tree.
	ApplyConfigDiff(bgpTree map[string]any) error

	// AddDynamicPeer adds a peer with the given configuration.
	AddDynamicPeer(config DynamicPeerConfig) error

	// RemovePeer removes a peer by address.
	RemovePeer(addr netip.Addr) error

	// GetConfigTree returns the full config as a map for plugin config delivery.
	GetConfigTree() map[string]any

	// SetConfigTree replaces the running config tree after a successful reload.
	SetConfigTree(tree map[string]any)

	// --- Startup coordination (4 methods) ---

	// SignalAPIReady signals that an API process is ready.
	SignalAPIReady()

	// AddAPIProcessCount adds to the number of API processes to wait for.
	AddAPIProcessCount(count int)

	// SignalPluginStartupComplete signals that all plugin phases are done.
	SignalPluginStartupComplete()

	// SignalPeerAPIReady signals that a peer-specific API initialization is complete.
	SignalPeerAPIReady(peerAddr string)

	// --- Cache consumer lifecycle (2 methods) ---

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
	SocketPath         string                                          // Path to Unix socket
	Plugins            []PluginConfig                                  // External plugins to spawn
	ConfiguredFamilies []string                                        // Families configured on peers (for deferred auto-load)
	RPCProviders       []func() []RPCRegistration                      // Additional RPC sources (e.g., BGP handler RPCs)
	RPCFallback        func(string) func(json.RawMessage) (any, error) // Resolves RPC methods not in core dispatch
	CommitManager      any                                             // Commit manager instance (injected by reactor, type-asserted by handlers)
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
)

// cmdPlugin is the "plugin" token in command strings like "ze plugin <name>".
const cmdPlugin = "plugin"
