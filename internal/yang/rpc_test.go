package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpschema "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/schema"
	ribschema "codeberg.org/thomas-mangin/ze/internal/plugin/rib/schema"

	ipcschema "codeberg.org/thomas-mangin/ze/internal/ipc/schema"
)

// loadAPIModules loads core + all API YANG modules for RPC extraction tests.
func loadAPIModules(t *testing.T) *Loader {
	t.Helper()
	loader := NewLoader()

	require.NoError(t, loader.LoadEmbedded(), "load core modules")
	require.NoError(t, loader.AddModuleFromText("ze-bgp-conf.yang", bgpschema.ZeBGPConfYANG), "load bgp conf")
	require.NoError(t, loader.AddModuleFromText("ze-bgp-api.yang", bgpschema.ZeBGPAPIYANG), "load bgp api")
	require.NoError(t, loader.AddModuleFromText("ze-system-api.yang", ipcschema.ZeSystemAPIYANG), "load system api")
	require.NoError(t, loader.AddModuleFromText("ze-plugin-api.yang", ipcschema.ZePluginAPIYANG), "load plugin api")
	require.NoError(t, loader.AddModuleFromText("ze-rib-api.yang", ribschema.ZeRibAPIYANG), "load rib api")
	require.NoError(t, loader.Resolve(), "resolve all modules")

	return loader
}

// TestExtractRPCs verifies RPC metadata extraction from YANG API modules.
//
// VALIDATES: All RPCs from each API module are extracted with correct names and descriptions.
// PREVENTS: Missing RPCs when building dispatch tables from YANG.
func TestExtractRPCs(t *testing.T) {
	loader := loadAPIModules(t)

	tests := []struct {
		name     string
		module   string
		wantRPCs []string
	}{
		{
			name:   "bgp-api",
			module: "ze-bgp-api",
			wantRPCs: []string{
				"help", "command-list", "command-help", "command-complete",
				"plugin-encoding", "plugin-format", "plugin-ack",
				"peer-list", "peer-show", "peer-add", "peer-remove", "peer-teardown",
				"peer-update", "watchdog-announce", "watchdog-withdraw",
				"peer-borr", "peer-eorr", "peer-raw",
				"cache", "commit",
				"subscribe", "unsubscribe", "event-list",
			},
		},
		{
			name:   "system-api",
			module: "ze-system-api",
			wantRPCs: []string{
				"help", "version-software", "version-api",
				"daemon-shutdown", "daemon-status", "daemon-reload",
				"subsystem-list",
				"command-list", "command-help", "command-complete",
			},
		},
		{
			name:   "rib-api",
			module: "ze-rib-api",
			wantRPCs: []string{
				"help", "command-list", "command-help", "command-complete", "event-list",
				"show-in", "clear-in", "show-out", "clear-out",
			},
		},
		{
			name:   "plugin-api",
			module: "ze-plugin-api",
			wantRPCs: []string{
				"help", "command-list", "command-help", "command-complete",
				"session-ready", "peer-session-ready", "session-ping", "session-bye",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rpcs := ExtractRPCs(loader, tt.module)
			rpcNames := make([]string, len(rpcs))
			for i, r := range rpcs {
				rpcNames[i] = r.Name
				assert.Equal(t, tt.module, r.Module, "module name for %s", r.Name)
				assert.NotEmpty(t, r.Description, "description for %s", r.Name)
			}
			assert.ElementsMatch(t, tt.wantRPCs, rpcNames)
		})
	}
}

// TestExtractRPCInputOutput verifies input/output leaves are extracted from RPCs.
//
// VALIDATES: RPC input parameters are extracted with types from YANG definitions.
// PREVENTS: Missing parameter metadata for dispatch validation.
func TestExtractRPCInputOutput(t *testing.T) {
	loader := loadAPIModules(t)

	rpcs := ExtractRPCs(loader, "ze-bgp-api")

	// Find peer-teardown - it has input params (selector, subcode)
	var teardown *RPCMeta
	for i := range rpcs {
		if rpcs[i].Name == "peer-teardown" {
			teardown = &rpcs[i]
			break
		}
	}
	require.NotNil(t, teardown, "peer-teardown RPC should exist")

	// peer-teardown should have input leaves
	assert.NotEmpty(t, teardown.Input, "peer-teardown should have input params")

	// Check that known input leaf exists with type
	inputNames := make(map[string]bool)
	for _, leaf := range teardown.Input {
		inputNames[leaf.Name] = true
		assert.NotEmpty(t, leaf.Type, "leaf %s should have type", leaf.Name)
	}
	assert.True(t, inputNames["selector"] || inputNames["subcode"],
		"peer-teardown should have selector or subcode input, got: %v", inputNames)
}

// TestExtractRPCOutputLeaves verifies output leaves are extracted from RPCs.
//
// VALIDATES: RPC output leaves are extracted with types from YANG definitions.
// PREVENTS: Missing output metadata for response formatting.
func TestExtractRPCOutputLeaves(t *testing.T) {
	loader := loadAPIModules(t)

	rpcs := ExtractRPCs(loader, "ze-system-api")

	// Find version-software - it has output params
	var versionSW *RPCMeta
	for i := range rpcs {
		if rpcs[i].Name == "version-software" {
			versionSW = &rpcs[i]
			break
		}
	}
	require.NotNil(t, versionSW, "version-software RPC should exist")
	assert.NotEmpty(t, versionSW.Output, "version-software should have output params")
}

// TestExtractNotifications verifies notification metadata extraction from YANG.
//
// VALIDATES: All notifications from API modules are extracted.
// PREVENTS: Missing notification definitions breaking event subscriptions.
func TestExtractNotifications(t *testing.T) {
	loader := loadAPIModules(t)

	tests := []struct {
		name       string
		module     string
		wantNotifs []string
	}{
		{
			name:   "bgp-api",
			module: "ze-bgp-api",
			wantNotifs: []string{
				"peer-state-change", "route-received", "route-update-sent",
				"eor-received", "graceful-restart-state",
				"session-established", "session-closed",
			},
		},
		{
			name:   "rib-api",
			module: "ze-rib-api",
			wantNotifs: []string{
				"rib-change",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifs := ExtractNotifications(loader, tt.module)
			notifNames := make([]string, len(notifs))
			for i, n := range notifs {
				notifNames[i] = n.Name
				assert.Equal(t, tt.module, n.Module, "module name for %s", n.Name)
			}
			assert.ElementsMatch(t, tt.wantNotifs, notifNames)
		})
	}
}

// TestWireModule verifies YANG module name to wire method prefix conversion.
//
// VALIDATES: Module names are correctly stripped of -api/-conf suffixes.
// PREVENTS: Wrong method prefixes on the wire (e.g., "ze-bgp-api:peer-list" instead of "ze-bgp:peer-list").
func TestWireModule(t *testing.T) {
	tests := []struct {
		name   string
		module string
		want   string
	}{
		{"bgp-api", "ze-bgp-api", "ze-bgp"},
		{"system-api", "ze-system-api", "ze-system"},
		{"rib-api", "ze-rib-api", "ze-rib"},
		{"plugin-api", "ze-plugin-api", "ze-plugin"},
		{"bgp-conf", "ze-bgp-conf", "ze-bgp"},
		{"no-suffix", "ze-types", "ze-types"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, WireModule(tt.module))
		})
	}
}

// TestExtractRPCsNonexistentModule verifies graceful handling of missing modules.
//
// VALIDATES: Returns empty slice for nonexistent module.
// PREVENTS: Nil pointer panic when module doesn't exist.
func TestExtractRPCsNonexistentModule(t *testing.T) {
	loader := loadAPIModules(t)
	rpcs := ExtractRPCs(loader, "nonexistent-module")
	assert.Empty(t, rpcs, "should return empty for nonexistent module")
}

// TestExtractNotificationsNonexistentModule verifies graceful handling of missing modules.
//
// VALIDATES: Returns empty slice for nonexistent module.
// PREVENTS: Nil pointer panic when module doesn't exist.
func TestExtractNotificationsNonexistentModule(t *testing.T) {
	loader := loadAPIModules(t)
	notifs := ExtractNotifications(loader, "nonexistent-module")
	assert.Empty(t, notifs, "should return empty for nonexistent module")
}
