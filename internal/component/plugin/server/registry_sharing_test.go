package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// TestRegistryBuildFromPlugins verifies registry is built from plugin registrations.
//
// VALIDATES: Command registry populated from plugin registrations.
// PREVENTS: Commands missing from registry after registration.
func TestRegistryBuildFromPlugins(t *testing.T) {
	t.Run("build_from_single_plugin", func(t *testing.T) {
		reg := &plugin.PluginRegistration{
			Name:      "rib-plugin",
			Encodings: []string{"text"},
			Commands:  []string{"rib show", "rib clear"},
			Done:      true,
		}

		registry := plugin.NewPluginRegistry()
		require.NoError(t, registry.Register(reg))

		// Build sharing info
		allCommands := registry.BuildCommandInfo()

		require.Contains(t, allCommands, "rib-plugin")
		cmds := allCommands["rib-plugin"]
		require.Len(t, cmds, 2)
	})

	t.Run("build_from_multiple_plugins", func(t *testing.T) {
		registry := plugin.NewPluginRegistry()

		reg1 := &plugin.PluginRegistration{
			Name:      "plugin1",
			Encodings: []string{"text"},
			Commands:  []string{"cmd1"},
			Done:      true,
		}
		require.NoError(t, registry.Register(reg1))

		reg2 := &plugin.PluginRegistration{
			Name:      "plugin2",
			Encodings: []string{"b64"},
			Commands:  []string{"cmd2"},
			Done:      true,
		}
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
	registry := plugin.NewPluginRegistry()

	reg1 := &plugin.PluginRegistration{
		Name:     "plugin1",
		Commands: []string{"rib show"},
		Done:     true,
	}
	require.NoError(t, registry.Register(reg1))

	// Second plugin tries same command
	reg2 := &plugin.PluginRegistration{
		Name:     "plugin2",
		Commands: []string{"rib show"},
		Done:     true,
	}

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
	registry := plugin.NewPluginRegistry()

	reg := &plugin.PluginRegistration{
		Name:     "rib-plugin",
		Commands: []string{"rib show", "rib clear"},
		Done:     true,
	}
	require.NoError(t, registry.Register(reg))

	plugin := registry.LookupCommand("rib show")
	assert.Equal(t, "rib-plugin", plugin)

	plugin = registry.LookupCommand("RIB SHOW") // case insensitive
	assert.Equal(t, "rib-plugin", plugin)

	plugin = registry.LookupCommand("unknown cmd")
	assert.Empty(t, plugin)
}
