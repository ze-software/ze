package ipc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"

	// Blank imports trigger init() registration of YANG modules.
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rib/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	_ "github.com/ze-software/ze/internal/component/hub/yang"
	_ "github.com/ze-software/ze/internal/core/ipc/yang"
)

// loadAllAPIModules loads core + all API YANG modules for testing.
func loadAllAPIModules(t *testing.T) *yang.Resolved {
	t.Helper()
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded(), "load core modules")
	require.NoError(t, loader.LoadRegistered(), "load registered modules")
	schema, err := loader.Resolve()
	require.NoError(t, err, "resolve all modules")

	return schema
}

// TestYANGAPIModuleLoad verifies all 4 API YANG modules load and resolve.
//
// VALIDATES: API modules parse and resolve imports (ze-types, ze-bgp-conf).
// PREVENTS: YANG syntax errors or missing imports blocking IPC dispatch.
func TestYANGAPIModuleLoad(t *testing.T) {
	loader := loadAllAPIModules(t)

	modules := []string{
		"ze-bgp-api",
		"ze-system-api",
		"ze-plugin-api",
		"ze-rib-api",
	}

	for _, name := range modules {
		t.Run(name, func(t *testing.T) {
			mod := loader.GetModule(name)
			require.NotNil(t, mod, "module %s should be loaded", name)
			assert.Equal(t, name, mod.Name)
		})
	}
}

// TestYANGIPCGroupings verifies IPC groupings are accessible in ze-types.
//
// VALIDATES: peer-info, command-info, event-type-info, cache-entry groupings exist.
// PREVENTS: API modules failing to use shared groupings from ze-types.
func TestYANGIPCGroupings(t *testing.T) {
	loader := loadAllAPIModules(t)

	typesMod := loader.GetModule("ze-types")
	require.NotNil(t, typesMod, "ze-types should be loaded")

	groupings := []string{
		"peer-info",
		"command-info",
		"event-type-info",
		"transaction-result",
		"cache-entry",
	}

	for _, name := range groupings {
		t.Run(name, func(t *testing.T) {
			found := false
			for _, g := range typesMod.Grouping {
				if g.Name == name {
					found = true
					break
				}
			}
			assert.True(t, found, "grouping %q should exist in ze-types", name)
		})
	}
}

// TestYANGIPCTypedefs verifies IPC typedefs are accessible in ze-types.
//
// VALIDATES: address-family, peer-selector, encoding-mode, wire-format, ack-mode typedefs.
// PREVENTS: API modules failing to reference shared types.
func TestYANGIPCTypedefs(t *testing.T) {
	loader := loadAllAPIModules(t)

	typesMod := loader.GetModule("ze-types")
	require.NotNil(t, typesMod, "ze-types should be loaded")

	typedefs := []string{
		"address-family",
		"peer-selector",
		"encoding-mode",
		"wire-format",
		"ack-mode",
	}

	for _, name := range typedefs {
		t.Run(name, func(t *testing.T) {
			found := false
			for _, td := range typesMod.Typedef {
				if td.Name == name {
					found = true
					break
				}
			}
			assert.True(t, found, "typedef %q should exist in ze-types", name)
		})
	}
}

// TestYANGBGPAPIRPCs verifies BGP API module contains expected RPCs.
//
// VALIDATES: Key RPCs (peer-list, peer-teardown, update, subscribe) are defined.
// PREVENTS: Missing RPC definitions breaking IPC method dispatch.
func TestYANGBGPAPIRPCs(t *testing.T) {
	loader := loadAllAPIModules(t)

	mod := loader.GetModule("ze-bgp-api")
	require.NotNil(t, mod)

	expectedRPCs := []string{
		"help", "command-list", "command-help", "command-complete",
		"plugin-encoding", "plugin-format", "plugin-ack",
		"overview", "peer-show-capabilities", "peer-show-statistics", "peer-clear-soft",
		"peer-list", "peer-show", "peer-create", "peer-delete", "peer-teardown",
		"peer-update",
		"peer-borr", "peer-eorr", "peer-raw",
		"cache", "commit",
		"subscribe", "unsubscribe", "event-list",
	}

	rpcNames := make(map[string]bool)
	for _, r := range mod.RPC {
		rpcNames[r.Name] = true
	}

	for _, name := range expectedRPCs {
		t.Run(name, func(t *testing.T) {
			assert.True(t, rpcNames[name], "RPC %q should exist in ze-bgp-api", name)
		})
	}
}

// TestYANGBGPAPINotifications verifies BGP API module contains expected notifications.
//
// VALIDATES: Key notifications (peer-state-change, session-established) are defined.
// PREVENTS: Missing notification definitions breaking event subscriptions.
func TestYANGBGPAPINotifications(t *testing.T) {
	loader := loadAllAPIModules(t)

	mod := loader.GetModule("ze-bgp-api")
	require.NotNil(t, mod)

	expectedNotifs := []string{
		"peer-state-change",
		"route-received",
		"route-update-sent",
		"eor-received",
		"graceful-restart-state",
		"session-established",
		"session-closed",
	}

	notifNames := make(map[string]bool)
	for _, n := range mod.Notification {
		notifNames[n.Name] = true
	}

	for _, name := range expectedNotifs {
		t.Run(name, func(t *testing.T) {
			assert.True(t, notifNames[name], "notification %q should exist in ze-bgp-api", name)
		})
	}
}

// TestYANGSystemAPIRPCs verifies system API module contains expected RPCs.
//
// VALIDATES: System RPCs (version, shutdown, command-list, command-complete) exist.
// PREVENTS: Missing system introspection RPCs.
func TestYANGSystemAPIRPCs(t *testing.T) {
	loader := loadAllAPIModules(t)

	mod := loader.GetModule("ze-system-api")
	require.NotNil(t, mod)

	expectedRPCs := []string{
		"help", "version-software", "version-api",
		"daemon-shutdown", "daemon-quit", "daemon-status", "daemon-reload",
		"subsystem-list",
		"command-list", "command-help", "command-complete",
	}

	rpcNames := make(map[string]bool)
	for _, r := range mod.RPC {
		rpcNames[r.Name] = true
	}

	for _, name := range expectedRPCs {
		t.Run(name, func(t *testing.T) {
			assert.True(t, rpcNames[name], "RPC %q should exist in ze-system-api", name)
		})
	}
}

// TestYANGRibAPIRPCs verifies RIB API module contains expected RPCs.
//
// VALIDATES: RIB RPCs (show, clear-in, clear-out) are defined.
// PREVENTS: Missing RPC definitions breaking RIB query dispatch.
func TestYANGRibAPIRPCs(t *testing.T) {
	loader := loadAllAPIModules(t)

	mod := loader.GetModule("ze-rib-api")
	require.NotNil(t, mod)

	expectedRPCs := []string{
		"show", "best", "best-status", "clear-in", "clear-out", "status",
	}

	rpcNames := make(map[string]bool)
	for _, r := range mod.RPC {
		rpcNames[r.Name] = true
	}

	for _, name := range expectedRPCs {
		t.Run(name, func(t *testing.T) {
			assert.True(t, rpcNames[name], "RPC %q should exist in ze-rib-api", name)
		})
	}
}

// TestYANGRibAPINotifications verifies RIB API module contains expected notifications.
//
// VALIDATES: RIB notification (rib-change) is defined.
// PREVENTS: Missing notification definitions breaking RIB event subscriptions.
func TestYANGRibAPINotifications(t *testing.T) {
	loader := loadAllAPIModules(t)

	mod := loader.GetModule("ze-rib-api")
	require.NotNil(t, mod)

	expectedNotifs := []string{
		"rib-change",
	}

	notifNames := make(map[string]bool)
	for _, n := range mod.Notification {
		notifNames[n.Name] = true
	}

	for _, name := range expectedNotifs {
		t.Run(name, func(t *testing.T) {
			assert.True(t, notifNames[name], "notification %q should exist in ze-rib-api", name)
		})
	}
}

// TestYANGPluginAPIRPCs verifies plugin API module contains lifecycle RPCs.
//
// VALIDATES: Plugin lifecycle RPCs (session-ready, ping, bye) exist.
// PREVENTS: Missing plugin lifecycle RPCs breaking plugin protocol.
func TestYANGPluginAPIRPCs(t *testing.T) {
	loader := loadAllAPIModules(t)

	mod := loader.GetModule("ze-plugin-api")
	require.NotNil(t, mod)

	expectedRPCs := []string{
		"help", "command-list", "command-help", "command-complete",
		"session-ready", "session-peer-ready", "session-ping", "session-bye",
	}

	rpcNames := make(map[string]bool)
	for _, r := range mod.RPC {
		rpcNames[r.Name] = true
	}

	for _, name := range expectedRPCs {
		t.Run(name, func(t *testing.T) {
			assert.True(t, rpcNames[name], "RPC %q should exist in ze-plugin-api", name)
		})
	}
}

// TestExtractRPCs extracts metadata from isolated, resolved YANG modules.
//
// VALIDATES: RPC names and modules, exact independent help texts, and a module without RPCs.
// PREVENTS: Cross-module leakage, invented help text, and truncated explanations.
func TestExtractRPCs(t *testing.T) {
	tests := []struct {
		module string
		source string
		want   []yang.RPCMeta
	}{
		{
			module: "ze-extract-api",
			source: `module ze-extract-api {
    namespace "urn:test:extract:api";
    prefix extract;
    import ze-extensions { prefix ze; }

    rpc inspect {
        ze:help "Inspect the selected item.";
        description "Return the selected item's state.\n\nInclude its last transition.";
    }
    rpc reset {
        ze:help "Reset the selected item.";
    }
    rpc explain {
        description "Return the explanation without inventing a summary.";
    }
}`,
			want: []yang.RPCMeta{
				{
					Module:      "ze-extract-api",
					Name:        "inspect",
					ShortHelp:   "Inspect the selected item.",
					Description: "Return the selected item's state.\n\nInclude its last transition.",
				},
				{
					Module:    "ze-extract-api",
					Name:      "reset",
					ShortHelp: "Reset the selected item.",
				},
				{
					Module:      "ze-extract-api",
					Name:        "explain",
					Description: "Return the explanation without inventing a summary.",
				},
			},
		},
		{
			module: "ze-other-api",
			source: `module ze-other-api {
    namespace "urn:test:other:api";
    prefix other;
    import ze-extensions { prefix ze; }

    rpc inspect {
        ze:help "Inspect another module's item.";
        description "This explanation belongs to the other module.";
    }
}`,
			want: []yang.RPCMeta{
				{
					Module:      "ze-other-api",
					Name:        "inspect",
					ShortHelp:   "Inspect another module's item.",
					Description: "This explanation belongs to the other module.",
				},
			},
		},
		{
			module: "ze-no-rpcs",
			source: `module ze-no-rpcs {
    namespace "urn:test:no-rpcs";
    prefix no-rpcs;

    container state {
        config false;
        leaf count { type uint32; }
    }
    notification changed {
        leaf count { type uint32; }
    }
}`,
		},
	}

	loader := yang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	for _, tt := range tests {
		require.NoError(t, loader.AddModuleFromText(tt.module+".yang", tt.source))
	}
	schema, err := loader.Resolve()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			rpcs := yang.ExtractRPCs(schema, tt.module)
			if tt.want == nil {
				assert.Nil(t, rpcs, "a loaded module without RPCs returns nil")
				return
			}
			assert.ElementsMatch(t, tt.want, rpcs)
		})
	}
}

// TestExtractRPCInputOutput verifies input/output leaves are extracted from RPCs.
//
// VALIDATES: RPC input parameters are extracted with types from YANG definitions.
// PREVENTS: Missing parameter metadata for dispatch validation.
func TestExtractRPCInputOutput(t *testing.T) {
	loader := loadAllAPIModules(t)

	rpcs := yang.ExtractRPCs(loader, "ze-bgp-api")

	// Find peer-teardown - it has input params (selector, subcode)
	var teardown *yang.RPCMeta
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
	loader := loadAllAPIModules(t)

	rpcs := yang.ExtractRPCs(loader, "ze-system-api")

	// Find version-software - it has output params
	var versionSW *yang.RPCMeta
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
	loader := loadAllAPIModules(t)

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
			notifs := yang.ExtractNotifications(loader, tt.module)
			notifNames := make([]string, len(notifs))
			for i, n := range notifs {
				notifNames[i] = n.Name
				assert.Equal(t, tt.module, n.Module, "module name for %s", n.Name)
			}
			assert.ElementsMatch(t, tt.wantNotifs, notifNames)
		})
	}
}
