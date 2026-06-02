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

// PeerSubcommandKeywords returns the set of words that immediately follow
// `peer` in BGP peer command paths. Used by config validation to reject peer
// names that would collide with subcommand dispatch.
// The wireToPath map is typically built via yang.WireMethodToPath(loader).
func PeerSubcommandKeywords(wireToPath map[string]string) map[string]bool {
	keywords := make(map[string]bool)
	for _, path := range wireToPath {
		words := strings.Fields(strings.ToLower(path))
		for i := 0; i+1 < len(words); i++ {
			if words[i] != "peer" || !isBGPPeerPath(words, i) || words[i+1] == "" {
				continue
			}
			keywords[words[i+1]] = true
		}
	}
	return keywords
}

func isBGPPeerPath(words []string, peerIdx int) bool {
	if peerIdx == 0 {
		return true
	}
	if peerIdx >= 2 && words[peerIdx-1] == bgpParticipantName {
		switch words[peerIdx-2] {
		case "show", "set", "del", "update":
			return true
		}
	}
	return false
}
