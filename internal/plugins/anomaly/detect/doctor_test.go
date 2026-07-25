package detect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func enabledAnomalyTree() *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("anomaly").GetOrCreateContainer("detect").Set("enabled", "true")
	return tree
}

func TestCheckFeatureSourceWarnsWhenNoSource(t *testing.T) {
	// VALIDATES: anomaly-detect enabled + no flow source -> a warning diagnostic.
	diags := checkFeatureSource(registry.DoctorCheckContext{Tree: enabledAnomalyTree()})
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-anomaly-detect-no-feature-source", diags[0].Code)
	assert.Equal(t, "warning", diags[0].Severity)
}

func TestCheckFeatureSourceSilentWithSource(t *testing.T) {
	tu := enabledAnomalyTree()
	tu.GetOrCreateContainer("traffic").GetOrCreateContainer("usage")
	assert.Empty(t, checkFeatureSource(registry.DoctorCheckContext{Tree: tu}))

	fe := enabledAnomalyTree()
	fe.GetOrCreateContainer("flow-export")
	assert.Empty(t, checkFeatureSource(registry.DoctorCheckContext{Tree: fe}))
}

func TestCheckFeatureSourceSilentWhenDisabled(t *testing.T) {
	notEnabled := config.NewTree()
	notEnabled.GetOrCreateContainer("anomaly").GetOrCreateContainer("detect").Set("enabled", "false")
	assert.Empty(t, checkFeatureSource(registry.DoctorCheckContext{Tree: notEnabled}))

	assert.Empty(t, checkFeatureSource(registry.DoctorCheckContext{Tree: config.NewTree()}))
	assert.Empty(t, checkFeatureSource(registry.DoctorCheckContext{Tree: nil}))
}

func TestDetectDoctorCheckRegistered(t *testing.T) {
	// VALIDATES: the plugin registers the feature-source doctor check so `ze doctor`
	// runs it and removing anomaly-detect removes the check.
	found := false
	for _, c := range registry.PluginDoctorChecks() {
		if c.Name == "anomaly-detect-feature-source" {
			found = true
			break
		}
	}
	assert.True(t, found, "doctor check anomaly-detect-feature-source not registered via Registration.DoctorChecks")
}
