// Design: plan/spec-mpls-2-ldp.md -- LDP port-646 readiness doctor check tests

package ldp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestCheckLDPPortUnavailable(t *testing.T) {
	// VALIDATES: ldp configured + port 646 unbindable -> doctor-ldp-port-unavailable.
	// PREVENTS: LDP silently failing to start because port 646 is privileged/in use.
	old := ldpPortProbe
	ldpPortProbe = func() bool { return false }
	t.Cleanup(func() { ldpPortProbe = old })

	tree := config.NewTree()
	tree.GetOrCreateContainer("ldp")

	diags := checkLDPPort(registry.DoctorCheckContext{Tree: tree})
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-ldp-port-unavailable", diags[0].Code)
	assert.Equal(t, "warning", diags[0].Severity)
}

func TestCheckLDPPortAvailable(t *testing.T) {
	// VALIDATES: when port 646 binds, no warning is emitted.
	old := ldpPortProbe
	ldpPortProbe = func() bool { return true }
	t.Cleanup(func() { ldpPortProbe = old })

	tree := config.NewTree()
	tree.GetOrCreateContainer("ldp")
	assert.Empty(t, checkLDPPort(registry.DoctorCheckContext{Tree: tree}))
}

func TestCheckLDPPortAbsentConfig(t *testing.T) {
	// VALIDATES: the check fires only when ldp is configured.
	// PREVENTS: doctor warning about LDP port 646 on boxes that do not use it.
	old := ldpPortProbe
	ldpPortProbe = func() bool { return false }
	t.Cleanup(func() { ldpPortProbe = old })

	assert.Empty(t, checkLDPPort(registry.DoctorCheckContext{Tree: config.NewTree()}))
	assert.Empty(t, checkLDPPort(registry.DoctorCheckContext{Tree: nil}))
}

func TestProbePortBindableEphemeral(t *testing.T) {
	// VALIDATES: the bind probe succeeds for an ephemeral port (both UDP and TCP).
	// PREVENTS: a probe that always fails (constant false doctor warning) regressing in.
	if !probePortBindable("127.0.0.1:0") {
		t.Fatal("ephemeral port must be bindable by the probe")
	}
}

func TestLDPDoctorCheckRegistered(t *testing.T) {
	// VALIDATES: ldp registers the ldp-port doctor check via the plugin registry,
	// so `ze doctor` runs it and removing ldp removes the check.
	checks := registry.PluginDoctorChecks()
	found := false
	for _, c := range checks {
		if c.Name == "ldp-port" {
			found = true
			break
		}
	}
	assert.True(t, found, "doctor check ldp-port not registered via Registration.DoctorChecks")
}
