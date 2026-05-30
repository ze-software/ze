// Design: docs/architecture/api/process-protocol.md — init()-based RPC registration

package server

import "strings"

// registeredRPCs holds RPCs added via RegisterRPCs from init() in register_*.go files.
var registeredRPCs []RPCRegistration

// RegisterRPCs adds RPCs to the package-level registry.
// Called from init() in register.go files.
func RegisterRPCs(rpcs ...RPCRegistration) {
	registeredRPCs = append(registeredRPCs, rpcs...)
}

// ProcessCleanupFunc is called when a plugin process exits.
// Receives the process name for scoped cleanup.
type ProcessCleanupFunc func(processName string)

var processCleanupHooks []ProcessCleanupFunc

// RegisterProcessCleanup registers a callback invoked during cleanupProcess.
// Called from init() to avoid import cycles between server and command packages.
func RegisterProcessCleanup(fn ProcessCleanupFunc) {
	processCleanupHooks = append(processCleanupHooks, fn)
}

// runProcessCleanupHooks calls all registered cleanup hooks for a process.
func runProcessCleanupHooks(processName string) {
	for _, fn := range processCleanupHooks {
		fn(processName)
	}
}

// PeerSubcommandKeywords returns the set of first words that follow "peer"
// in CLI command paths. Used by config validation to reject peer names
// that would collide with subcommand dispatch.
// The wireToPath map is typically built via yang.WireMethodToPath(loader).
func PeerSubcommandKeywords(wireToPath map[string]string) map[string]bool {
	const prefix = "peer "
	keywords := make(map[string]bool)
	for _, path := range wireToPath {
		cmd := strings.ToLower(path)
		if !strings.HasPrefix(cmd, prefix) {
			continue
		}
		rest := cmd[len(prefix):]
		word, _, _ := strings.Cut(rest, " ")
		if word != "" {
			keywords[word] = true
		}
	}
	return keywords
}
