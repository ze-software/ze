package ipc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ribschema "codeberg.org/thomas-mangin/ze/internal/plugins/bgp-rib/schema"
	bgpschema "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/schema"
	"codeberg.org/thomas-mangin/ze/internal/yang"

	ipcschema "codeberg.org/thomas-mangin/ze/internal/ipc/schema"
)

// loadAllAPIModules loads core + all API YANG modules for testing.
func loadAllAPIModules(t *testing.T) *yang.Loader {
	t.Helper()
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded(), "load core modules")
	require.NoError(t, loader.AddModuleFromText("ze-bgp-conf.yang", bgpschema.ZeBGPConfYANG), "load bgp conf")
	require.NoError(t, loader.AddModuleFromText("ze-bgp-api.yang", bgpschema.ZeBGPAPIYANG), "load bgp api")
	require.NoError(t, loader.AddModuleFromText("ze-system-api.yang", ipcschema.ZeSystemAPIYANG), "load system api")
	require.NoError(t, loader.AddModuleFromText("ze-plugin-api.yang", ipcschema.ZePluginAPIYANG), "load plugin api")
	require.NoError(t, loader.AddModuleFromText("ze-rib-api.yang", ribschema.ZeRibAPIYANG), "load rib api")
	require.NoError(t, loader.Resolve(), "resolve all modules")

	return loader
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
		"peer-list", "peer-show", "peer-add", "peer-remove", "peer-teardown",
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
		"daemon-shutdown", "daemon-status", "daemon-reload",
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
// VALIDATES: RIB RPCs (show-in, clear-in, show-out, clear-out) are defined.
// PREVENTS: Missing RPC definitions breaking RIB query dispatch.
func TestYANGRibAPIRPCs(t *testing.T) {
	loader := loadAllAPIModules(t)

	mod := loader.GetModule("ze-rib-api")
	require.NotNil(t, mod)

	expectedRPCs := []string{
		"help", "command-list", "command-help", "command-complete", "event-list",
		"show-in", "clear-in", "show-out", "clear-out",
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
		"session-ready", "peer-session-ready", "session-ping", "session-bye",
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
