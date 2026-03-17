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
