// Design: docs/architecture/api/process-protocol.md — plugin process management

package server

// version is ze application version string, set by main at startup via SetVersion.
var version = "dev"

// buildDate is the build date string, set by main at startup via SetVersion.
var buildDate = "unknown"

// SetVersion sets the application version and build date (called from main).
func SetVersion(v, d string) {
	version = v
	buildDate = d
}

// GetVersion returns the current version and build date.
func GetVersion() (string, string) {
	return version, buildDate
}

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
	WireMethod       string  // "module:rpc-name" format (e.g., "ze-bgp:peer-list")
	CLICommand       string  // CLI text command (e.g., "bgp peer list")
	Handler          Handler // Handler function
	Help             string  // Human-readable description
	ReadOnly         bool    // True if command only reads state (safe for "ze show")
	RequiresSelector bool    // True if peer commands must have explicit selector (not default "*")
}
