package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegistrySharingFormat verifies registry sharing message format.
//
// VALIDATES: Registry messages match spec format.
// PREVENTS: Plugin unable to parse registry messages.
func TestRegistrySharingFormat(t *testing.T) {
	t.Run("single_plugin_single_command", func(t *testing.T) {
		allCommands := map[string][]PluginCommandInfo{
			"rib-plugin": {
				{Command: "rib show", Encoding: "text"},
			},
		}

		lines := FormatRegistrySharing("rib-plugin", allCommands)

		require.Len(t, lines, 3)
		assert.Equal(t, "api name rib-plugin", lines[0])
		assert.Equal(t, "api rib-plugin text cmd rib show", lines[1])
		assert.Equal(t, "api done", lines[2])
	})

	t.Run("single_plugin_multiple_commands", func(t *testing.T) {
		allCommands := map[string][]PluginCommandInfo{
			"rib-plugin": {
				{Command: "rib adjacent in show", Encoding: "text"},
				{Command: "rib adjacent out show", Encoding: "text"},
				{Command: "rib loc show", Encoding: "b64"},
			},
		}

		lines := FormatRegistrySharing("rib-plugin", allCommands)

		require.Len(t, lines, 5) // name + 3 commands + done
		assert.Equal(t, "api name rib-plugin", lines[0])
		assert.Equal(t, "api done", lines[len(lines)-1])

		// Check commands are present (order may vary)
		found := make(map[string]bool)
		for _, line := range lines[1 : len(lines)-1] {
			found[line] = true
		}
		assert.True(t, found["api rib-plugin text cmd rib adjacent in show"])
		assert.True(t, found["api rib-plugin text cmd rib adjacent out show"])
		assert.True(t, found["api rib-plugin b64 cmd rib loc show"])
	})

	t.Run("multiple_plugins", func(t *testing.T) {
		allCommands := map[string][]PluginCommandInfo{
			"rib-plugin": {
				{Command: "rib show", Encoding: "text"},
			},
			"gr-plugin": {
				{Command: "peer * refresh", Encoding: "text"},
			},
		}

		// Each plugin gets its own name but sees all commands
		lines := FormatRegistrySharing("rib-plugin", allCommands)

		assert.Equal(t, "api name rib-plugin", lines[0])
		assert.Equal(t, "api done", lines[len(lines)-1])

		// Should see commands from both plugins
		allLines := make(map[string]bool)
		for _, line := range lines {
			allLines[line] = true
		}
		assert.True(t, allLines["api rib-plugin text cmd rib show"])
		assert.True(t, allLines["api gr-plugin text cmd peer * refresh"])
	})

	t.Run("no_commands", func(t *testing.T) {
		allCommands := map[string][]PluginCommandInfo{}

		lines := FormatRegistrySharing("passive-plugin", allCommands)

		require.Len(t, lines, 2)
		assert.Equal(t, "api name passive-plugin", lines[0])
		assert.Equal(t, "api done", lines[1])
	})
}

// TestRegistryBuildFromPlugins verifies registry is built from plugin registrations.
//
// VALIDATES: Command registry populated from plugin registrations.
// PREVENTS: Commands missing from registry after registration.
func TestRegistryBuildFromPlugins(t *testing.T) {
	t.Run("build_from_single_plugin", func(t *testing.T) {
		reg := &PluginRegistration{Name: "rib-plugin"}
		require.NoError(t, reg.ParseLine("encoding add text"))
		require.NoError(t, reg.ParseLine("cmd add rib show"))
		require.NoError(t, reg.ParseLine("cmd add rib clear"))
		require.NoError(t, reg.ParseLine("registration done"))

		registry := NewPluginRegistry()
		require.NoError(t, registry.Register(reg))

		// Build sharing info
		allCommands := registry.BuildCommandInfo()

		require.Contains(t, allCommands, "rib-plugin")
		cmds := allCommands["rib-plugin"]
		require.Len(t, cmds, 2)
	})

	t.Run("build_from_multiple_plugins", func(t *testing.T) {
		registry := NewPluginRegistry()

		reg1 := &PluginRegistration{Name: "plugin1"}
		require.NoError(t, reg1.ParseLine("encoding add text"))
		require.NoError(t, reg1.ParseLine("cmd add cmd1"))
		require.NoError(t, reg1.ParseLine("registration done"))
		require.NoError(t, registry.Register(reg1))

		reg2 := &PluginRegistration{Name: "plugin2"}
		require.NoError(t, reg2.ParseLine("encoding add b64"))
		require.NoError(t, reg2.ParseLine("cmd add cmd2"))
		require.NoError(t, reg2.ParseLine("registration done"))
		require.NoError(t, registry.Register(reg2))

		allCommands := registry.BuildCommandInfo()

		require.Contains(t, allCommands, "plugin1")
		require.Contains(t, allCommands, "plugin2")
	})
}

// TestRegistryCommandConflict verifies command conflicts are detected.
//
// VALIDATES: Same command from two plugins is rejected.
// PREVENTS: Command routing ambiguity.
func TestRegistryCommandConflict(t *testing.T) {
	registry := NewPluginRegistry()

	reg1 := &PluginRegistration{Name: "plugin1"}
	require.NoError(t, reg1.ParseLine("cmd add rib show"))
	require.NoError(t, reg1.ParseLine("registration done"))
	require.NoError(t, registry.Register(reg1))

	// Second plugin tries same command
	reg2 := &PluginRegistration{Name: "plugin2"}
	require.NoError(t, reg2.ParseLine("cmd add rib show"))
	require.NoError(t, reg2.ParseLine("registration done"))

	err := registry.Register(reg2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
	assert.Contains(t, err.Error(), "rib show")
}

// TestRegistryCommandLookup verifies commands can be looked up by name.
//
// VALIDATES: Registered commands can be found for routing.
// PREVENTS: Commands not routable to their plugin.
func TestRegistryCommandLookup(t *testing.T) {
	registry := NewPluginRegistry()

	reg := &PluginRegistration{Name: "rib-plugin"}
	require.NoError(t, reg.ParseLine("cmd add rib show"))
	require.NoError(t, reg.ParseLine("cmd add rib clear"))
	require.NoError(t, reg.ParseLine("registration done"))
	require.NoError(t, registry.Register(reg))

	plugin := registry.LookupCommand("rib show")
	assert.Equal(t, "rib-plugin", plugin)

	plugin = registry.LookupCommand("RIB SHOW") // case insensitive
	assert.Equal(t, "rib-plugin", plugin)

	plugin = registry.LookupCommand("unknown cmd")
	assert.Empty(t, plugin)
}
