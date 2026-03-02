package handler

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugin/process"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/plugin/server"
)

// TestRibMetaRPCsOnly verifies RibMetaRPCs() returns only meta-commands.
//
// VALIDATES: RibMetaRPCs returns exactly 5 meta-command registrations (help, command-list, command-help, command-complete, event-list).
// PREVENTS: Builtin data handlers lingering after unification.
func TestRibMetaRPCsOnly(t *testing.T) {
	rpcs := RibMetaRPCs()

	assert.Len(t, rpcs, 5, "RibMetaRPCs should return only 5 meta-commands")

	// Verify only meta wire methods remain
	wantMethods := map[string]bool{
		"ze-rib:help":             true,
		"ze-rib:command-list":     true,
		"ze-rib:command-help":     true,
		"ze-rib:command-complete": true,
		"ze-rib:event-list":       true,
	}

	for _, rpc := range rpcs {
		assert.True(t, wantMethods[rpc.WireMethod],
			"unexpected wire method in RibMetaRPCs: %s", rpc.WireMethod)
	}

	// Verify data handlers are NOT present
	for _, rpc := range rpcs {
		assert.NotEqual(t, "ze-rib:show-in", rpc.WireMethod, "show-in should be removed")
		assert.NotEqual(t, "ze-rib:clear-in", rpc.WireMethod, "clear-in should be removed")
		assert.NotEqual(t, "ze-rib:show-out", rpc.WireMethod, "show-out should be removed")
		assert.NotEqual(t, "ze-rib:clear-out", rpc.WireMethod, "clear-out should be removed")
	}
}

// TestRibHelpIncludesPlugin verifies handleRibHelp discovers plugin-registered rib subcommands.
//
// VALIDATES: handleRibHelp merges hardcoded subcommands with plugin-registered "rib *" commands.
// PREVENTS: Plugin subcommands silently missing from help output after unification.
func TestRibHelpIncludesPlugin(t *testing.T) {
	// Create server with rib meta RPCs injected
	server := pluginserver.NewServer(&pluginserver.ServerConfig{
		RPCProviders: []func() []pluginserver.RPCRegistration{RibMetaRPCs},
	}, nil)

	// Create a process for plugin command ownership
	proc := process.NewProcess(plugin.PluginConfig{Name: "rib"})

	// Register plugin commands that start with "rib "
	results := server.Dispatcher().Registry().Register(proc, []pluginserver.CommandDef{
		{Name: "rib status", Description: "RIB status"},
		{Name: "rib show in", Description: "Show inbound RIB"},
		{Name: "rib adjacent inbound show", Description: "Show adjacent inbound"},
	})
	for _, r := range results {
		require.True(t, r.OK, "failed to register %s: %s", r.Name, r.Error)
	}

	ctx := &pluginserver.CommandContext{Server: server}

	resp, err := handleRibHelp(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "response data should be map[string]any")

	subs, ok := data["subcommands"].([]string)
	require.True(t, ok, "subcommands should be []string")

	// Sort for deterministic comparison
	sort.Strings(subs)

	// Hardcoded: command, event (meta-commands backed by builtins)
	// Plugin adds: show, status, adjacent (discovered from registry)
	assert.Contains(t, subs, "command", "hardcoded subcommand")
	assert.Contains(t, subs, "event", "hardcoded subcommand")
	assert.Contains(t, subs, "show", "plugin subcommand from 'rib show in'")
	assert.Contains(t, subs, "status", "plugin subcommand from 'rib status'")
	assert.Contains(t, subs, "adjacent", "plugin subcommand from 'rib adjacent inbound show'")

	// No duplicates
	seen := make(map[string]bool)
	for _, s := range subs {
		assert.False(t, seen[s], "duplicate subcommand: %s", s)
		seen[s] = true
	}
}

// TestRibCommandListShowsPlugin verifies handleRibCommandList includes plugin-registered rib commands.
//
// VALIDATES: handleRibCommandList returns both builtin and plugin commands with "rib " prefix.
// PREVENTS: Plugin data commands missing from command-list output after unification.
func TestRibCommandListShowsPlugin(t *testing.T) {
	// Create server with rib meta RPCs injected (builtins loaded automatically)
	server := pluginserver.NewServer(&pluginserver.ServerConfig{
		RPCProviders: []func() []pluginserver.RPCRegistration{RibMetaRPCs},
	}, nil)

	// Create a process for plugin command ownership
	proc := process.NewProcess(plugin.PluginConfig{Name: "rib"})

	// Register plugin commands
	results := server.Dispatcher().Registry().Register(proc, []pluginserver.CommandDef{
		{Name: "rib status", Description: "RIB status"},
		{Name: "rib show in", Description: "Show inbound RIB"},
		{Name: "rib clear in", Description: "Clear inbound RIB"},
	})
	for _, r := range results {
		require.True(t, r.OK, "failed to register %s: %s", r.Name, r.Error)
	}

	ctx := &pluginserver.CommandContext{Server: server}

	resp, err := handleRibCommandList(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "response data should be map[string]any")

	commands, ok := data["commands"].([]pluginserver.Completion)
	require.True(t, ok, "commands should be []Completion")

	// Collect command names
	names := make(map[string]bool)
	for _, c := range commands {
		names[c.Value] = true
	}

	// Builtin rib commands should be present (meta-commands from RibMetaRPCs)
	assert.True(t, names["rib help"], "builtin 'rib help' should be listed")
	assert.True(t, names["rib command list"], "builtin 'rib command list' should be listed")
	assert.True(t, names["rib event list"], "builtin 'rib event list' should be listed")

	// Plugin commands should also be present
	assert.True(t, names["rib status"], "plugin 'rib status' should be listed")
	assert.True(t, names["rib show in"], "plugin 'rib show in' should be listed")
	assert.True(t, names["rib clear in"], "plugin 'rib clear in' should be listed")

	// Verify plugin commands have correct help text
	for _, c := range commands {
		if c.Value == "rib status" {
			assert.Equal(t, "RIB status", c.Help)
		}
		if c.Value == "rib show in" {
			assert.Equal(t, "Show inbound RIB", c.Help)
		}
	}
}
