// Design: docs/architecture/debugging/plugin-testing.md -- the module list `ze plugin test` prints

package cli

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// pluginModulesFixtureYANG is the module text of the throwaway registration:
// a module whose file name is not "ze-<plugin>.yang", so a list that invents
// the file name from the plugin name cannot pass.
const pluginModulesFixtureYANG = `module ze-plugin-modules-fixture-conf {
	namespace "urn:ze:test:plugin-modules-fixture";
	prefix pmf;
	container plugin-modules-fixture {
		leaf value { type string; }
	}
}
`

// TestPluginModuleFilesNamesTheRegisteredModule proves the list `ze plugin
// test` prints names the file each named plugin's module is registered under.
//
// VALIDATES: the registered file name, not one built from the plugin name; the
// "ze." prefix names the same plugin; a plugin named twice is listed once; a
// name no internal plugin registers adds nothing; the order is the command
// line's.
// PREVENTS: an invented file name ("ze-<plugin>.yang") or a random order in the
// operator's output.
// Method: a throwaway registration and its module, both restored when the test
// ends, so the answer does not depend on the feature tags of this binary.
func TestPluginModuleFilesNamesTheRegisteredModule(t *testing.T) {
	const pluginName = "test-plugin-modules-fixture"
	const moduleFile = "ze-plugin-modules-fixture-conf.yang"
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	require.NoError(t, registry.Register(registry.Registration{
		Name:        pluginName,
		Description: "module list test fixture",
		YANG:        pluginModulesFixtureYANG,
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))
	t.Cleanup(configyang.RegisterModuleForTest(moduleFile, pluginModulesFixtureYANG))

	files, unregistered := pluginModuleFiles([]string{"ze." + pluginName, "no-such-plugin", pluginName})

	assert.Equal(t, []string{moduleFile}, files)
	assert.Empty(t, unregistered, "a plugin whose module init() registered is loaded")
}

// TestPluginModuleFilesSaysWhenNoModuleCarriesTheText proves a plugin whose
// YANG text no registered module carries is reported apart, never dropped and
// never counted as loaded: this command's schema is the registered set alone.
//
// VALIDATES: the plugin is answered in unregistered, once, under its name
// without the "ze." prefix, and no file is listed for it.
// PREVENTS: a count and a list that silently omit a plugin that declares YANG,
// or that call loaded a module the parse does not hold.
func TestPluginModuleFilesSaysWhenNoModuleCarriesTheText(t *testing.T) {
	const pluginName = "test-plugin-modules-unregistered"
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	require.NoError(t, registry.Register(registry.Registration{
		Name:        pluginName,
		Description: "module list test fixture with no registered module",
		YANG:        pluginModulesFixtureYANG,
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))

	files, unregistered := pluginModuleFiles([]string{pluginName, "ze." + pluginName})

	assert.Empty(t, files, "a module the schema does not hold is not listed as loaded")
	assert.Equal(t, []string{pluginName}, unregistered)
}
