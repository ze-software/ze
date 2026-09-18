package yang

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestAPIRPCInputMatchesCommandTree proves that every `ze-iface:*` rpc in
// ze-iface-api.yang declares exactly the leaves the command tree binds for the
// path that reaches it, under the same names.
//
// VALIDATES: the parameters the API, `ze help ai --json` and the MCP tool
// publish for a command are the ones the dispatcher accepts. The API reads the
// rpc input (buildParamMeta, cmd/ze/hub/command_meta.go) and the dispatcher
// reads the command tree (PathToArgDefs), so a leaf present on one side only is
// a published argument nobody can pass, or an accepted argument nobody is told
// about. The MCP tool forwards each typed parameter as `<name> <value>`
// (dispatchGenerated, internal/component/mcp/tools.go), so the NAME has to be
// the one validateCommandArgs binds.
// PREVENTS: `rpc interface-unit-del` publishing one argument, `name`, for the
// two-argument `delete interface <name> unit <vid>` (journal row of 2026-09-15
// in plan/journal/command-takes-an-untyped-positional-value.md).
func TestAPIRPCInputMatchesCommandTree(t *testing.T) {
	loader, err := configyang.DefaultLoader()
	require.NoError(t, err)

	rpcs := configyang.ExtractRPCs(loader, "ze-iface-api")
	require.NotEmpty(t, rpcs, "ze-iface-api declares rpcs")
	inputByRPC := make(map[string][]string, len(rpcs))
	for _, rpc := range rpcs {
		names := make([]string, 0, len(rpc.Input))
		for _, leaf := range rpc.Input {
			names = append(names, leaf.Name)
		}
		slices.Sort(names)
		inputByRPC[rpc.Name] = names
	}

	// The walk starts from the rpcs the API module declares, so an rpc the
	// command tree binds and the API module does not declare is not this test's
	// finding: it is a missing declaration, not a wrong one.
	argDefs := configyang.PathToArgDefs(loader)
	pathsByWire := configyang.WireMethodToPaths(loader)
	checked := 0
	for rpcName, want := range inputByRPC {
		wire := "ze-iface:" + rpcName
		paths := pathsByWire[wire]
		require.NotEmpty(t, paths, "%s is declared in ze-iface-api.yang and bound to no command path", wire)
		for _, path := range paths {
			defs := argDefs[path]
			// A command modeled with modifier containers, `request interface
			// migrate from <x> to <y> address <z>`, carries its leaves BELOW
			// the path, so PathToArgDefs answers nothing for it and there is
			// nothing to compare against the rpc here.
			if len(defs) == 0 {
				continue
			}
			got := make([]string, 0, len(defs))
			for i := range defs {
				got = append(got, defs[i].Name)
			}
			slices.Sort(got)
			assert.Equal(t, want, got, "%s: rpc %s input leaves versus the leaves the command tree binds on %q", wire, rpcName, path)
			checked++
		}
	}
	require.NotZero(t, checked, "no ze-iface command path was checked")
}
