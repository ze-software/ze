// VALIDATES: the armed-without-firewall readiness check warns only when armed and
// no firewall is configured, and that the check is registered so `ze doctor` runs it.
// PREVENTS: silently enabling autonomous enforcement with no backend to apply it.

package shape

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func armedShapeTree() *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("anomaly").GetOrCreateContainer("shape").Set("mode", "armed")
	return tree
}

func TestCheckFirewallWarnsWhenArmedNoFirewall(t *testing.T) {
	diags := checkFirewall(registry.DoctorCheckContext{Tree: armedShapeTree()})
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-anomaly-shape-armed-no-firewall", diags[0].Code)
	assert.Equal(t, "warning", diags[0].Severity)
}

func TestCheckFirewallSilentWhenShadowOrFirewallPresent(t *testing.T) {
	// firewall configured -> no warning even in armed mode.
	withFw := armedShapeTree()
	withFw.GetOrCreateContainer("firewall")
	assert.Empty(t, checkFirewall(registry.DoctorCheckContext{Tree: withFw}))

	// shadow mode -> nothing to enforce, no warning.
	shadow := config.NewTree()
	shadow.GetOrCreateContainer("anomaly").GetOrCreateContainer("shape").Set("mode", "shadow")
	assert.Empty(t, checkFirewall(registry.DoctorCheckContext{Tree: shadow}))

	// absent / nil trees.
	assert.Empty(t, checkFirewall(registry.DoctorCheckContext{Tree: config.NewTree()}))
	assert.Empty(t, checkFirewall(registry.DoctorCheckContext{Tree: nil}))
}

func TestShapeDoctorCheckRegistered(t *testing.T) {
	found := false
	for _, c := range registry.PluginDoctorChecks() {
		if c.Name == "anomaly-shape-firewall" {
			found = true
			break
		}
	}
	assert.True(t, found, "doctor check anomaly-shape-firewall not registered")
}
