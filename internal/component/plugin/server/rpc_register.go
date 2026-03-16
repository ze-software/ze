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

// PeerSubcommandKeywords returns the set of first words that follow "bgp peer"
// in registered CLI commands. Used by config validation to reject peer names
// that would collide with subcommand dispatch.
// Derived dynamically from registeredRPCs so the set stays in sync automatically.
func PeerSubcommandKeywords() map[string]bool {
	const prefix = "bgp peer "
	keywords := make(map[string]bool)
	for _, rpc := range registeredRPCs {
		cmd := strings.ToLower(rpc.CLICommand)
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
