// Design: ai/rules/plugins.md -- ze_mcp present build validation
//
//go:build ze_mcp

package hub

// VALIDATES: with the ze_mcp build tag (the default ze / ze-appliance feature
// set), the MCP service factory is registered in the construction registry.
// PREVENTS: a regression where ze_mcp is set but mcp is not wired (e.g. the
// register_mcp.go init() is dropped or the tag stops reaching the generator).

import "testing"

func TestBuildTag_MCP_Present(t *testing.T) {
	if !registeredServiceName("mcp") {
		t.Fatal("ze_mcp build: mcp service factory not registered")
	}
}
