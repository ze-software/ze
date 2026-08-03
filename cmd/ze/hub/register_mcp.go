// Design: ai/rules/plugins.md -- compile-out-able services (feature-gate)
//
// Build-tag-gated registration of the MCP service factory. Compiled only under
// //go:build ze_mcp; absent the tag this init() does not exist, so the hub
// builds no MCP service and the mcp package is dropped from the binary.

//go:build ze_mcp

package hub

func init() {
	registerService("mcp", buildMCPService, func(lm *ListenerMigrator, svc Service) {
		lm.SetMCP(svc)
	})
}
