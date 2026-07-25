package detect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func enabledDetectTree() *config.Tree {
	tree := config.NewTree()
	dd := tree.GetOrCreateContainer("ddos").GetOrCreateContainer("detect")
	dd.Set("enabled", "true")
	return tree
}

func TestCheckFlowSourceWarnsWhenNoSource(t *testing.T) {
	// VALIDATES: D-6 -- detector enabled + characterization on + no flow source
	// -> doctor-ddos-detect-no-flow-source.
	diags := checkFlowSource(registry.DoctorCheckContext{Tree: enabledDetectTree()})
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-ddos-detect-no-flow-source", diags[0].Code)
	assert.Equal(t, "warning", diags[0].Severity)
}

func TestCheckFlowSourceSilentWithSource(t *testing.T) {
	// traffic-usage present -> no warning.
	tu := enabledDetectTree()
	tu.GetOrCreateContainer("traffic").GetOrCreateContainer("usage")
	assert.Empty(t, checkFlowSource(registry.DoctorCheckContext{Tree: tu}))

	// flow-export present -> no warning.
	fe := enabledDetectTree()
	fe.GetOrCreateContainer("flow-export")
	assert.Empty(t, checkFlowSource(registry.DoctorCheckContext{Tree: fe}))
}

func TestCheckFlowSourceSilentWhenDisabled(t *testing.T) {
	// characterize-enable=false -> no warning even with no source.
	off := enabledDetectTree()
	off.GetContainerPath(configRoot).Set("characterize-enable", "false")
	assert.Empty(t, checkFlowSource(registry.DoctorCheckContext{Tree: off}))

	// detector not enabled -> no warning.
	notEnabled := config.NewTree()
	notEnabled.GetOrCreateContainer("ddos").GetOrCreateContainer("detect").Set("enabled", "false")
	assert.Empty(t, checkFlowSource(registry.DoctorCheckContext{Tree: notEnabled}))

	// ddos-detect absent, and nil tree -> no warning.
	assert.Empty(t, checkFlowSource(registry.DoctorCheckContext{Tree: config.NewTree()}))
	assert.Empty(t, checkFlowSource(registry.DoctorCheckContext{Tree: nil}))
}

func TestDetectDoctorCheckRegistered(t *testing.T) {
	// VALIDATES: ddos-detect registers the flow-source doctor check so `ze doctor`
	// runs it and removing ddos-detect removes the check.
	found := false
	for _, c := range registry.PluginDoctorChecks() {
		if c.Name == "ddos-detect-flow-source" {
			found = true
			break
		}
	}
	assert.True(t, found, "doctor check ddos-detect-flow-source not registered via Registration.DoctorChecks")
}
