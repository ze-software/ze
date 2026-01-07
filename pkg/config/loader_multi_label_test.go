package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigLoader_MultiLabel verifies config parsing for multi-label syntax.
//
// VALIDATES: New `labels [100 200]` array syntax is correctly parsed.
// PREVENTS: Config loading failure for multi-label routes.
func TestConfigLoader_MultiLabel(t *testing.T) {
	configInput := `
router-id 1.2.3.4;
local-as 65001;
peer 192.168.1.1 {
    peer-as 65002;
    family { ipv4/nlri-mpls; }
    announce {
        ipv4 { nlri-mpls 10.0.0.0/8 labels [1000 2000] next-hop self; }
    }
}
`
	_, r, err := LoadReactorWithConfig(configInput)
	require.NoError(t, err, "config should parse")
	require.NotNil(t, r)

	// Get peer settings
	peers := r.Peers()
	require.Len(t, peers, 1)

	settings := peers[0].Settings()
	require.Len(t, settings.StaticRoutes, 1)

	route := settings.StaticRoutes[0]
	assert.Equal(t, []uint32{1000, 2000}, route.Labels, "labels should be [1000, 2000]")
	assert.True(t, route.IsLabeledUnicast(), "should be labeled unicast")
}

// TestConfigLoader_LabelSingleValue verifies single `label` syntax works.
//
// VALIDATES: `label 1000` syntax is correctly parsed.
// PREVENTS: Regression in single-label config parsing.
func TestConfigLoader_LabelSingleValue(t *testing.T) {
	configInput := `
router-id 1.2.3.4;
local-as 65001;
peer 192.168.1.1 {
    peer-as 65002;
    family { ipv4/nlri-mpls; }
    announce {
        ipv4 { nlri-mpls 10.0.0.0/8 label 1000 next-hop self; }
    }
}
`
	_, r, err := LoadReactorWithConfig(configInput)
	require.NoError(t, err, "config should parse")
	require.NotNil(t, r)

	// Get peer settings
	peers := r.Peers()
	require.Len(t, peers, 1)

	settings := peers[0].Settings()
	require.Len(t, settings.StaticRoutes, 1)

	route := settings.StaticRoutes[0]
	assert.Equal(t, []uint32{1000}, route.Labels, "labels should be [1000]")
	assert.Equal(t, uint32(1000), route.SingleLabel(), "SingleLabel should return 1000")
}

// TestConfigLoader_VPNMultiLabel verifies VPN routes with multi-label stack.
//
// VALIDATES: VPN routes can have multi-label stacks.
// PREVENTS: VPN route parsing failure with multiple labels.
func TestConfigLoader_VPNMultiLabel(t *testing.T) {
	configInput := `
router-id 1.2.3.4;
local-as 65001;
peer 192.168.1.1 {
    peer-as 65002;
    family { ipv4/mpls-vpn; }
    announce {
        ipv4 { mpls-vpn 10.0.0.0/8 rd 100:100 labels [1000 2000] next-hop self; }
    }
}
`
	_, r, err := LoadReactorWithConfig(configInput)
	require.NoError(t, err, "config should parse")
	require.NotNil(t, r)

	// Get peer settings
	peers := r.Peers()
	require.Len(t, peers, 1)

	settings := peers[0].Settings()
	require.Len(t, settings.StaticRoutes, 1)

	route := settings.StaticRoutes[0]
	assert.Equal(t, []uint32{1000, 2000}, route.Labels, "labels should be [1000, 2000]")
	assert.True(t, route.IsVPN(), "should be VPN route")
	assert.False(t, route.IsLabeledUnicast(), "should not be labeled unicast")
}

// TestConfigLoader_VPNRequiresLabel verifies VPN route without label is rejected.
//
// VALIDATES: VPN routes require at least one label (fail-early).
// PREVENTS: Invalid VPN routes producing corrupt wire format.
func TestConfigLoader_VPNRequiresLabel(t *testing.T) {
	configInput := `
router-id 1.2.3.4;
local-as 65001;
peer 192.168.1.1 {
    peer-as 65002;
    family { ipv4/mpls-vpn; }
    announce {
        ipv4 { mpls-vpn 10.0.0.0/8 rd 100:100 next-hop self; }
    }
}
`
	_, _, err := LoadReactorWithConfig(configInput)
	require.Error(t, err, "VPN route without label should fail")
	assert.Contains(t, err.Error(), "label", "error should mention label requirement")
}
