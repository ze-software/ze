package plugin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/ipc"
)

// TestRPCRegistrationTable verifies the builtin RPC registration table.
//
// VALIDATES: All handlers mapped to valid wire methods and unique CLI commands.
// PREVENTS: Lost handlers during migration from RegisterBuiltin pattern.
func TestRPCRegistrationTable(t *testing.T) {
	rpcs := AllBuiltinRPCs()

	// Verify count matches expected (BGP handler RPCs + RIB meta RPCs moved to handler/ package,
	// injected via RPCProviders — not counted here)
	assert.Len(t, rpcs, 21, "expected 21 builtin RPCs (update+watchdog+rib moved to handler/)")

	// Track uniqueness
	wireMethodsSeen := make(map[string]bool)
	cliCommandsSeen := make(map[string]bool)

	for _, reg := range rpcs {
		t.Run(reg.WireMethod, func(t *testing.T) {
			// Valid wire method format
			_, _, err := ipc.ParseMethod(reg.WireMethod)
			require.NoError(t, err, "invalid wire method format")

			// Non-empty CLI command
			assert.NotEmpty(t, reg.CLICommand, "missing CLI command")

			// Non-nil handler
			assert.NotNil(t, reg.Handler, "missing handler")

			// Non-empty help
			assert.NotEmpty(t, reg.Help, "missing help text")

			// Unique wire method
			assert.False(t, wireMethodsSeen[reg.WireMethod], "duplicate wire method: %s", reg.WireMethod)
			wireMethodsSeen[reg.WireMethod] = true

			// Unique CLI command
			assert.False(t, cliCommandsSeen[reg.CLICommand], "duplicate CLI command: %s", reg.CLICommand)
			cliCommandsSeen[reg.CLICommand] = true
		})
	}
}

// TestRPCRegistrationPerModule verifies each module registers the expected RPCs.
//
// VALIDATES: Per-module registration functions return correct counts.
// PREVENTS: Commands accidentally placed in wrong module.
func TestRPCRegistrationPerModule(t *testing.T) {
	bgp := BgpPluginRPCs()
	system := SystemPluginRPCs()
	lifecycle := PluginLifecycleRPCs()

	// Verify per-module counts
	// RIB meta RPCs moved to handler/ package, injected via RPCProviders
	assert.Len(t, bgp, 2, "BGP plugin RPCs (update+watchdog moved to handler/)")
	assert.Len(t, system, 11, "System RPCs")
	assert.Len(t, lifecycle, 8, "Plugin lifecycle RPCs")

	// Verify BGP RPCs all have ze-bgp: prefix
	for _, reg := range bgp {
		module, _, err := ipc.ParseMethod(reg.WireMethod)
		require.NoError(t, err)
		assert.Equal(t, "ze-bgp", module, "BGP RPC %s has wrong module", reg.WireMethod)
	}

	// Verify system RPCs all have ze-system: prefix
	for _, reg := range system {
		module, _, err := ipc.ParseMethod(reg.WireMethod)
		require.NoError(t, err)
		assert.Equal(t, "ze-system", module, "system RPC %s has wrong module", reg.WireMethod)
	}

	// Verify plugin RPCs all have ze-plugin: prefix
	for _, reg := range lifecycle {
		module, _, err := ipc.ParseMethod(reg.WireMethod)
		require.NoError(t, err)
		assert.Equal(t, "ze-plugin", module, "plugin RPC %s has wrong module", reg.WireMethod)
	}
}

// TestRPCRegistrationExpectedMethods verifies specific critical wire methods are present.
//
// VALIDATES: Essential commands are not accidentally removed.
// PREVENTS: Missing critical handlers after refactoring.
func TestRPCRegistrationExpectedMethods(t *testing.T) {
	rpcs := AllBuiltinRPCs()

	methodMap := make(map[string]*RPCRegistration, len(rpcs))
	for i := range rpcs {
		methodMap[rpcs[i].WireMethod] = &rpcs[i]
	}

	// BGP handler methods + RIB meta RPCs moved to handler/ package,
	// injected via RPCProviders — not in AllBuiltinRPCs.
	expectedMethods := []string{
		"ze-system:daemon-shutdown",
		"ze-system:daemon-status",
		"ze-system:daemon-reload",
		"ze-bgp:subscribe",
		"ze-bgp:unsubscribe",
		"ze-system:help",
		"ze-system:command-list",
		"ze-plugin:session-ready",
		"ze-plugin:session-bye",
	}

	for _, method := range expectedMethods {
		_, exists := methodMap[method]
		assert.True(t, exists, "missing expected wire method: %s", method)
	}
}

// TestRPCRegistrationLoadDispatcher verifies handlers load into RPCDispatcher.
//
// VALIDATES: AllBuiltinRPCs register with RPCDispatcher successfully.
// PREVENTS: Registration errors from invalid wire methods.
func TestRPCRegistrationLoadDispatcher(t *testing.T) {
	dispatcher := ipc.NewRPCDispatcher()

	for _, reg := range AllBuiltinRPCs() {
		err := dispatcher.Register(reg.WireMethod, func(_ string, _ json.RawMessage) (any, error) {
			return struct{}{}, nil
		})
		require.NoError(t, err, "failed to register %s", reg.WireMethod)
	}

	// Verify specific methods are registered (peer-list moved to handler/ RPCProviders)
	assert.True(t, dispatcher.HasMethod("ze-bgp:subscribe"))
	assert.True(t, dispatcher.HasMethod("ze-system:help"))
	// ze-rib:help moved to handler/ RPCProviders
	assert.True(t, dispatcher.HasMethod("ze-plugin:session-ready"))
}
