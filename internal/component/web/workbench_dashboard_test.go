package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestBuildDashboardData verifies the dashboard builder returns populated
// panels when the config tree has BGP peers and interfaces.
func TestBuildDashboardData(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	tree := config.NewTree()

	// Add system hostname.
	sys := config.NewTree()
	sys.Set("host", "router-lab-1")
	tree.SetContainer("system", sys)

	// Add BGP peers: 2 direct peers.
	bgp := config.NewTree()
	peer1 := config.NewTree()
	peer1.Set("neighbor-address", "10.0.0.1")
	bgp.AddListEntry("peer", "peer-1", peer1)

	peer2 := config.NewTree()
	peer2.Set("neighbor-address", "10.0.0.2")
	bgp.AddListEntry("peer", "peer-2", peer2)

	// Add 1 peer inside a group.
	group1 := config.NewTree()
	groupPeer := config.NewTree()
	groupPeer.Set("neighbor-address", "10.0.0.3")
	group1.AddListEntry("peer", "group-peer-1", groupPeer)
	bgp.AddListEntry("group", "transit", group1)

	tree.SetContainer("bgp", bgp)

	// Add interfaces.
	iface1 := config.NewTree()
	iface1.Set("type", "ethernet")
	tree.AddListEntry("iface", "eth0", iface1)

	iface2 := config.NewTree()
	iface2.Set("type", "bridge")
	tree.AddListEntry("iface", "br0", iface2)

	data := BuildDashboardData(tree, schema)

	// System panel.
	assert.Equal(t, "router-lab-1", data.System.Hostname)
	assert.Greater(t, data.System.CPUCount, 0, "CPU count must be positive")
	assert.NotEmpty(t, data.System.Version)
	assert.NotEmpty(t, data.System.MemAlloc)

	// BGP panel: 2 direct + 1 group = 3 total.
	assert.False(t, data.BGP.Empty, "BGP panel must not be empty")
	assert.Equal(t, 3, data.BGP.Total)

	// Interfaces panel: 2 interfaces.
	assert.False(t, data.Interfaces.Empty, "Interfaces panel must not be empty")
	assert.Equal(t, 2, data.Interfaces.Total)
}

// TestBuildDashboardData_EmptyState verifies that with no config, BGP and
// Interfaces panels have Empty=true with hint URLs.
func TestBuildDashboardData_EmptyState(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	tree := config.NewTree()
	data := BuildDashboardData(tree, schema)

	// System panel should still work (runtime data).
	assert.Greater(t, data.System.CPUCount, 0)

	// BGP empty.
	assert.True(t, data.BGP.Empty, "BGP panel must be empty with no peers")
	assert.Equal(t, "/show/bgp/peer/", data.BGP.HintURL)

	// Interfaces empty.
	assert.True(t, data.Interfaces.Empty, "Interfaces panel must be empty with no interfaces")
	assert.Equal(t, "/show/iface/", data.Interfaces.HintURL)

	// No warnings or errors.
	assert.Empty(t, data.Warnings)
	assert.Empty(t, data.Errors)
}

// TestBuildDashboardData_NilTree verifies the builder handles a nil tree
// gracefully.
func TestBuildDashboardData_NilTree(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	data := BuildDashboardData(nil, schema)
	assert.True(t, data.BGP.Empty)
	assert.True(t, data.Interfaces.Empty)
	assert.Empty(t, data.System.Hostname)
}

// TestRenderDashboard verifies the dashboard template renders all panels.
func TestRenderDashboard(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	data := DashboardData{
		System: DashboardSystemPanel{
			Hostname: "test-router",
			Uptime:   "3d 12h",
			Version:  "ze dev (built unknown)",
			CPUCount: 4,
			MemAlloc: "12.3 MiB",
		},
		BGP: DashboardBGPPanel{
			Established: 5,
			Active:      1,
			Idle:        2,
			Down:        0,
			Total:       8,
		},
		Interfaces: DashboardIfacePanel{
			Up:        3,
			Down:      1,
			AdminDown: 0,
			Total:     4,
		},
	}

	html := string(r.RenderFragment("workbench_dashboard", data))
	require.NotEmpty(t, html, "dashboard fragment must render")

	// Panel titles.
	assert.Contains(t, html, `System`)
	assert.Contains(t, html, `BGP Summary`)
	assert.Contains(t, html, `Interfaces`)
	assert.Contains(t, html, `Recent Warnings`)
	assert.Contains(t, html, `Recent Errors`)

	// System stats.
	assert.Contains(t, html, `test-router`)
	assert.Contains(t, html, `3d 12h`)
	assert.Contains(t, html, `12.3 MiB`)

	// BGP stats.
	assert.Contains(t, html, `wb-dashboard-stat--green`)
	assert.Contains(t, html, `wb-dashboard-stat--yellow`)

	// Dashboard grid.
	assert.Contains(t, html, `wb-dashboard-grid`)
	assert.Contains(t, html, `wb-dashboard-panel`)
}

// TestRenderDashboard_EmptyState verifies the dashboard template renders
// empty hints when panels have no data.
func TestRenderDashboard_EmptyState(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	data := DashboardData{
		System: DashboardSystemPanel{
			Version:  "ze dev",
			CPUCount: 1,
			MemAlloc: "1.0 MiB",
		},
		BGP:        DashboardBGPPanel{Empty: true, HintURL: "/show/bgp/peer/"},
		Interfaces: DashboardIfacePanel{Empty: true, HintURL: "/show/iface/"},
	}

	html := string(r.RenderFragment("workbench_dashboard", data))
	require.NotEmpty(t, html, "dashboard fragment must render")

	assert.Contains(t, html, `No BGP peers configured`)
	assert.Contains(t, html, `Add a peer`)
	assert.Contains(t, html, `No interfaces configured`)
	assert.Contains(t, html, `Configure interfaces`)
	assert.Contains(t, html, `No recent warnings`)
	assert.Contains(t, html, `No recent errors`)
}

// TestFormatBytes verifies the byte formatting helper.
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1048576, "1.0 MiB"},
		{1073741824, "1.0 GiB"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatBytes(tt.input))
	}
}
