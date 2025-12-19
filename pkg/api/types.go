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

// ReactorInterface defines what the API needs from the reactor.
// This interface avoids import cycles between pkg/api and pkg/reactor.
type ReactorInterface interface {
	// Peers returns information about all configured peers.
	Peers() []PeerInfo

	// Stats returns reactor-level statistics.
	Stats() ReactorStats

	// Stop signals the reactor to shut down.
	Stop()

	// AnnounceRoute announces a route to peers matching the selector.
	// Selector can be "*" for all peers or a specific IP address.
	AnnounceRoute(peerSelector string, route RouteSpec) error

	// WithdrawRoute withdraws a route from peers matching the selector.
	WithdrawRoute(peerSelector string, prefix netip.Prefix) error
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
