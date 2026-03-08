// Design: docs/architecture/api/process-protocol.md — init()-based RPC registration

package server

// registeredRPCs holds RPCs added via RegisterRPCs from init() in register_*.go files.
var registeredRPCs []RPCRegistration

// RegisterRPCs adds RPCs to the package-level registry.
// Called from init() in register.go files.
func RegisterRPCs(rpcs ...RPCRegistration) {
	registeredRPCs = append(registeredRPCs, rpcs...)
}
