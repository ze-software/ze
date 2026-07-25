// Design: docs/architecture/api/architecture.md -- API engine shared types
// Related: engine.go -- engine that uses these types
// Related: schema.go -- OpenAPI generation from CommandMeta
// Related: config_session.go -- config session manager

package api

import (
	"net"

	"github.com/ze-software/ze/internal/component/plugin"
)

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
	Name        string      // Dispatch path, e.g. "show bgp rib status"
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

// ExecResult is the standard API response envelope. It is an alias for the
// unified command-result envelope plugin.Response; the API transports render
// the same {status, data, error} shape every other surface produces. See
// internal/component/plugin/dispatch.go.
type ExecResult = plugin.Response

// CallerIdentity carries trusted caller metadata for an API request. It is an
// alias for the unified plugin.CallerIdentity, relocated to the plugin
// infrastructure package so the shared CommandDispatcher type can reference it.
type CallerIdentity = plugin.CallerIdentity

// Status constants for ExecResult. Sourced from the single definition in the
// plugin package so the "done"/"error" literals live in exactly one place.
const (
	StatusDone  = plugin.StatusDone
	StatusError = plugin.StatusError
)

// Config session authorization command strings shared by REST and gRPC.
const (
	ConfigAuthEdit    = "config edit"
	ConfigAuthSet     = "config set"
	ConfigAuthDelete  = "config delete"
	ConfigAuthCommit  = "config commit"
	ConfigAuthDiscard = "config discard"
)
