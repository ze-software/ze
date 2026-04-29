// Design: docs/architecture/api/architecture.md -- API engine shared types
// Related: engine.go -- engine that uses these types
// Related: schema.go -- OpenAPI generation from CommandMeta
// Related: config_session.go -- config session manager

package api

import "net"

// IsLoopbackAddr returns true if the host portion of addr resolves to a
// loopback address or is literally "localhost". Used by REST and gRPC
// transports to enforce loopback-only policies for plaintext listeners.
func IsLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// CommandMeta describes a registered command for API consumers.
type CommandMeta struct {
	Name        string      // Dispatch path, e.g. "bgp rib status"
	Description string      // From YANG or registration
	ReadOnly    bool        // True if read-only command
	Params      []ParamMeta // Input parameters from YANG RPC (nil = no typed params)
}

// ParamMeta describes a single input parameter from YANG RPC metadata.
type ParamMeta struct {
	Name        string // Parameter name (kebab-case from YANG)
	Type        string // YANG type: "string", "uint32", "boolean", etc.
	Description string // From YANG description
	Required    bool   // Mandatory in YANG
}

// ExecResult is the standard API response envelope.
type ExecResult struct {
	Status string `json:"status"`          // "done", "error", or "partial"
	Data   any    `json:"data,omitempty"`  // Payload
	Error  string `json:"error,omitempty"` // Error message (when status is "error")
}

// CallerIdentity carries trusted caller metadata for an API request.
// Populated by the transport after authentication; not an auth credential.
type CallerIdentity struct {
	Username   string
	RemoteAddr string
}

// Status constants for ExecResult.
const (
	StatusDone  = "done"
	StatusError = "error"
)
