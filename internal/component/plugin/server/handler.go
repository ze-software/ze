// Design: docs/architecture/api/process-protocol.md — plugin process management

package server

// Version is ze application version string.
const Version = "0.1.0"

// APIVersion is the IPC protocol version.
const APIVersion = "0.1.0"

// Command source constants.
const (
	sourceBuiltin = "builtin"
	argVerbose    = "verbose"
	cmdPlugin     = "plugin" // "plugin" token in command strings like "ze plugin <name>"
)

// RPCRegistration maps a YANG RPC wire method to its handler function and CLI command.
type RPCRegistration struct {
	WireMethod string  // "module:rpc-name" format (e.g., "ze-bgp:peer-list")
	CLICommand string  // CLI text command (e.g., "bgp peer list")
	Handler    Handler // Handler function
	Help       string  // Human-readable description
	ReadOnly   bool    // True if command only reads state (safe for "ze show")
}
