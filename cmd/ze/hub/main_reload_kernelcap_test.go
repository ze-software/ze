package hub

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// reloadCapabilityMarker is the container the test capability below treats as
// "this configuration uses the subsystem". A marker rather than a real config
// block keeps the capability inert for every other test in this binary, and it
// keeps this test independent of which subsystems the build enrolled.
const reloadCapabilityMarker = "kernelcap-reload-probe"

func init() {
	kernelcap.MustRegister(kernelcap.Capability{
		Subsystem:   "reload-probe",
		Component:   "hub",
		Kernel:      "CONFIG_RELOAD_PROBE",
		ConfigLeaf:  reloadCapabilityMarker,
		CodeAbsent:  "doctor-reload-probe-unavailable",
		CodeUnknown: "doctor-reload-probe-unknown",
		Order:       999,
		InUse: func(tree *zeconfig.Tree) bool {
			return tree != nil && tree.GetContainer(reloadCapabilityMarker) != nil
		},
		Probe: func() kernelcap.Result { return kernelcap.Result{State: kernelcap.StateAbsent} },
	})
}

// TestReloadRefusalKeepsRunningConfig proves AC-14.
//
// VALIDATES: a reload into a configuration that uses a subsystem this host
// cannot carry is refused, the error names the subsystem and the kernel feature,
// and the daemon keeps serving the configuration it already had.
// PREVENTS: a SIGHUP turning a working router into a dead one. The gate runs
// before ReloadConfig, before the provider refresh and before engine.Reload, so
// nothing the running daemon depends on has been touched when it refuses.
func TestReloadRefusalKeepsRunningConfig(t *testing.T) {
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	cp := zeconfig.NewProvider()
	priorRoot := map[string]any{"kept": "yes"}
	cp.SetRoot("running", priorRoot)

	incoming := map[string]any{reloadCapabilityMarker: map[string]any{}}
	tree := zeconfig.NewTree()
	tree.GetOrCreateContainer(reloadCapabilityMarker)
	load := func() (map[string]any, *zeconfig.Tree, error) { return incoming, tree, nil }

	err = runReload(srv, cp, load, nil)
	require.Error(t, err, "a reload into an ungated subsystem must be refused")
	require.Contains(t, err.Error(), "kernel capability")
	require.Contains(t, err.Error(), "reload-probe")
	require.Contains(t, err.Error(), "CONFIG_RELOAD_PROBE")

	running, ok := cp.Root("running")
	require.True(t, ok, "the refused reload dropped the running configuration")
	require.Equal(t, "yes", running["kept"], "the refused reload replaced the running configuration")
	_, replaced := cp.Root(reloadCapabilityMarker)
	require.False(t, replaced, "the refused reload published the configuration it rejected")
}

// VALIDATES: a reload whose configuration does not use the ungated subsystem is
// not refused by the gate.
// PREVENTS: a gate that refuses every reload, which is the failure a positive
// test alone would not catch (R-3).
func TestReloadUnaffectedByAnUnusedCapability(t *testing.T) {
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	cp := zeconfig.NewProvider()
	incoming := map[string]any{}
	load := func() (map[string]any, *zeconfig.Tree, error) { return incoming, zeconfig.NewTree(), nil }

	if err := runReload(srv, cp, load, nil); err != nil {
		require.False(t, strings.Contains(err.Error(), "kernel capability"),
			"a configuration that uses no gated subsystem was refused by the gate: %v", err)
	}
}
