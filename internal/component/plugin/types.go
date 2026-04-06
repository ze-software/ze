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
	"fmt"
	"time"
)

// Encoding constants for process output formatting.
const (
	EncodingJSON = "json"
	EncodingText = "text"
)

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
