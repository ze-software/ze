// VALIDATES: spec-connected-static-reach-the-locrib -- a config with main-table
// static routes and no `fib { ... }` block loads the FIB plugin that programs
// the data plane `interface { backend }` selects (owner decision, 2026-09-06).
// PREVENTS: the static half of that spec leaving a route in the system RIB that
// nothing writes, and an auto-load overriding the backend an operator chose.

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// autoLoadNames runs the config-path auto-load for the given paths and data
// plane and returns the plugin names it would start.
func autoLoadNames(configuredPaths []string, dataPlane string) []string {
	s := &Server{
		config: &ServerConfig{
			ConfiguredPaths: configuredPaths,
			DataPlane:       dataPlane,
		},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}

	var names []string
	for _, p := range s.getConfigPathPlugins() {
		names = append(names, p.Name)
	}
	return names
}

// dataPlaneWriterNames returns the registered plugins that declare a data plane.
// The test asks the registry rather than naming fib-kernel and fib-vpp, for the
// reason the feature exists: a FIB plugin added later must need no edit here.
func dataPlaneWriterNames() []string {
	var names []string
	for _, reg := range registry.All() {
		if reg.DataPlane != "" {
			names = append(names, reg.Name)
		}
	}
	return names
}

// TestStaticWithNoFIBBlockAutoLoadsTheWriterForTheDataPlane proves the owner's
// decision: an operator writes static routes, writes no `fib { ... }` block, and
// the engine loads the writer for the data plane they already chose.
func TestStaticWithNoFIBBlockAutoLoadsTheWriterForTheDataPlane(t *testing.T) {
	writer, ok := registry.PluginForDataPlane("netlink")
	require.True(t, ok, "no plugin declares the netlink data plane")

	names := autoLoadNames([]string{"static"}, "netlink")

	assert.Contains(t, names, "static", "the static config path must still load static")
	assert.Contains(t, names, writer,
		"a static config with no fib block must load the writer for the netlink data plane")
}

// TestStaticNeedsNoWriterWhenNothingNeedsADataPlane proves the resolution is
// driven by the producer's declaration and not by the config path: a config with
// no plugin declaring NeedsDataPlane loads no FIB plugin.
func TestStaticNeedsNoWriterWhenNothingNeedsADataPlane(t *testing.T) {
	names := autoLoadNames([]string{"interface"}, "netlink")

	for _, writer := range dataPlaneWriterNames() {
		assert.NotContains(t, names, writer,
			"an interface-only config must not load a FIB plugin")
	}
}

// TestExplicitFIBBlockIsNotOverriddenByTheAutoLoad proves an operator who chose
// a backend gets exactly that one. fib-p4 programs no switch, so an auto-load
// that ignored the explicit block would be visible as a second FIB plugin
// beside it.
func TestExplicitFIBBlockIsNotOverriddenByTheAutoLoad(t *testing.T) {
	require.NotNil(t, registry.Lookup("fib-p4"), "fib-p4 is not registered")

	names := autoLoadNames([]string{"static", "fib/p4"}, "netlink")

	assert.Contains(t, names, "fib-p4", "the operator's explicit fib { p4 { } } block must load")
	writer, ok := registry.PluginForDataPlane("netlink")
	require.True(t, ok)
	assert.NotContains(t, names, writer,
		"the auto-load must not add a second FIB plugin beside the operator's choice")
}

// TestNoWriterForTheSelectedDataPlaneLoadsNoFIBPlugin proves the resolution
// fails closed. A data plane no plugin programs adds nothing, so the engine
// never starts a writer that would program the wrong data plane.
func TestNoWriterForTheSelectedDataPlaneLoadsNoFIBPlugin(t *testing.T) {
	_, ok := registry.PluginForDataPlane("p4-runtime")
	require.False(t, ok, "p4-runtime must declare no writer for this test to mean anything")

	names := autoLoadNames([]string{"static"}, "p4-runtime")

	assert.Contains(t, names, "static")
	for _, writer := range dataPlaneWriterNames() {
		assert.NotContains(t, names, writer,
			"no FIB plugin may load for a data plane none of them programs")
	}
}
