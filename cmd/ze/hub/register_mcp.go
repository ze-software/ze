// Design: ai/rules/plugins.md -- compile-out-able services (feature-gate)
//
// Build-tag-gated registration of the MCP service factory. Compiled only under
// //go:build ze_mcp; absent the tag this init() does not exist, so the hub
// builds no MCP service and the mcp package is dropped from the binary.

//go:build ze_mcp

package hub

func init() {
	registerService("mcp", buildMCPService, func(lm *listenerMigrator, svc Service) {
		lm.setMCP(svc)
	})
}

// setMCP updates the MCP server reference. It carries this file's build
// constraint because the registration above is its only caller: see the note
// where its setWeb and setLG siblings live, in listener_migrate.go.
func (m *listenerMigrator) setMCP(s Reconfigurable) {
	m.mcp = s
	m.registerAuthReporter(svcMCP, s)
}
