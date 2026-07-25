package flowexport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func conntrackEnabledTree() *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("flow-export").GetOrCreateContainer("conntrack").Set("enabled", "true")
	return tree
}

// stubConntrackAvailable overrides the runtime probe for the test and restores it.
func stubConntrackAvailable(t *testing.T, avail bool) {
	t.Helper()
	prev := nfConntrackAvailable
	nfConntrackAvailable = func() bool { return avail }
	t.Cleanup(func() { nfConntrackAvailable = prev })
}

func TestCheckConntrackTrackingWarnsWhenUnavailable(t *testing.T) {
	// VALIDATES: conntrack export enabled + nf_conntrack unavailable ->
	// doctor-flowexport-conntrack-unavailable (the silent generic-flood gap).
	stubConntrackAvailable(t, false)
	diags := checkConntrackTracking(registry.DoctorCheckContext{Tree: conntrackEnabledTree()})
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-flowexport-conntrack-unavailable", diags[0].Code)
	assert.Equal(t, "warning", diags[0].Severity)
}

func TestCheckConntrackTrackingSilentWhenAvailable(t *testing.T) {
	stubConntrackAvailable(t, true)
	assert.Empty(t, checkConntrackTracking(registry.DoctorCheckContext{Tree: conntrackEnabledTree()}))
}

func TestCheckConntrackTrackingSilentWhenDisabledOrAbsent(t *testing.T) {
	stubConntrackAvailable(t, false)
	// conntrack export disabled -> no warning even if unavailable.
	off := config.NewTree()
	off.GetOrCreateContainer("flow-export").GetOrCreateContainer("conntrack").Set("enabled", "false")
	assert.Empty(t, checkConntrackTracking(registry.DoctorCheckContext{Tree: off}))
	// flow-export absent, empty tree, nil tree -> no warning.
	assert.Empty(t, checkConntrackTracking(registry.DoctorCheckContext{Tree: config.NewTree()}))
	assert.Empty(t, checkConntrackTracking(registry.DoctorCheckContext{Tree: nil}))
}

func TestConntrackDoctorCheckRegistered(t *testing.T) {
	found := false
	for _, c := range registry.PluginDoctorChecks() {
		if c.Name == "flow-export-conntrack-tracking" {
			found = true
			break
		}
	}
	assert.True(t, found, "doctor check flow-export-conntrack-tracking not registered")
}

func TestNfConntrackAvailableImplNoPanic(t *testing.T) {
	// Smoke: the real probe returns a bool without panicking (result is
	// environment-dependent: true iff nf_conntrack is loaded on the test host).
	_ = nfConntrackAvailableImpl()
}
