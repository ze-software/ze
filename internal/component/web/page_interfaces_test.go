package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

func testInterfaces() []iface.InterfaceInfo {
	return []iface.InterfaceInfo{
		{
			Name:  "eth0",
			Index: 1,
			Type:  "ethernet",
			State: "up",
			MTU:   1500,
			MAC:   "00:11:22:33:44:55",
			Addresses: []iface.AddrInfo{
				{Address: "192.168.1.1", PrefixLength: 24, Family: "ipv4"},
			},
			Stats: &iface.InterfaceStats{
				RxBytes: 1000, RxPackets: 10, RxErrors: 0, RxDropped: 0,
				TxBytes: 2000, TxPackets: 20, TxErrors: 0, TxDropped: 0,
			},
		},
		{
			Name:  "br0",
			Index: 2,
			Type:  "bridge",
			State: "up",
			MTU:   1500,
			MAC:   "aa:bb:cc:dd:ee:ff",
		},
		{
			Name:   "gre0",
			Index:  3,
			Type:   "gre",
			State:  "down",
			MTU:    1476,
			VlanID: 0,
		},
		{
			Name:   "eth0.100",
			Index:  4,
			Type:   "vlan",
			State:  "up",
			MTU:    1500,
			VlanID: 100,
		},
	}
}

// TestInterfaceTableData_Build verifies that BuildInterfaceTableData
// produces correct rows from a list of InterfaceInfo.
func TestInterfaceTableData_Build(t *testing.T) {
	infos := testInterfaces()
	data := BuildInterfaceTableData(infos, "")

	assert.Equal(t, "All Interfaces", data.Title)
	assert.Len(t, data.Rows, 4)
	assert.Equal(t, "eth0", data.Rows[0].Key)
	assert.Equal(t, "br0", data.Rows[1].Key)
	assert.Equal(t, "gre0", data.Rows[2].Key)
	assert.Equal(t, "eth0.100", data.Rows[3].Key)
}

// TestInterfaceTableData_Flags verifies that R/. flags are computed from state.
func TestInterfaceTableData_Flags(t *testing.T) {
	infos := testInterfaces()
	data := BuildInterfaceTableData(infos, "")

	// eth0 is up -> R green
	assert.Equal(t, "R", data.Rows[0].Flags)
	assert.Equal(t, flagClassGreen, data.Rows[0].FlagClass)

	// gre0 is down -> . red
	assert.Equal(t, ".", data.Rows[2].Flags)
	assert.Equal(t, flagClassRed, data.Rows[2].FlagClass)
}

// TestInterfaceTableData_FilterByType verifies type filtering.
func TestInterfaceTableData_FilterByType(t *testing.T) {
	infos := testInterfaces()

	// Filter ethernet.
	data := BuildInterfaceTableData(infos, "ethernet")
	require.Len(t, data.Rows, 1)
	assert.Equal(t, "eth0", data.Rows[0].Key)

	// Filter bridge.
	data = BuildInterfaceTableData(infos, "bridge")
	require.Len(t, data.Rows, 1)
	assert.Equal(t, "br0", data.Rows[0].Key)

	// Filter tunnel (matches gre type).
	data = BuildInterfaceTableData(infos, "tunnel")
	require.Len(t, data.Rows, 1)
	assert.Equal(t, "gre0", data.Rows[0].Key)

	// Filter vlan (matches VlanID > 0).
	data = BuildInterfaceTableData(infos, "vlan")
	require.Len(t, data.Rows, 1)
	assert.Equal(t, "eth0.100", data.Rows[0].Key)
}

// TestInterfaceTableData_EmptyState verifies empty table renders correctly.
func TestInterfaceTableData_EmptyState(t *testing.T) {
	data := BuildInterfaceTableData(nil, "")
	assert.Empty(t, data.Rows)
	assert.Equal(t, "No interfaces found.", data.EmptyMessage)
	assert.NotEmpty(t, data.AddURL)
}

// TestInterfaceTableData_EmptyWithFilter verifies empty state with filter.
func TestInterfaceTableData_EmptyWithFilter(t *testing.T) {
	data := BuildInterfaceTableData(nil, "ethernet")
	assert.Empty(t, data.Rows)
	assert.Contains(t, data.EmptyMessage, "ethernet")
}

// TestInterfaceDetailData_Build verifies detail panel construction.
func TestInterfaceDetailData_Build(t *testing.T) {
	info := &iface.InterfaceInfo{
		Name:  "eth0",
		Index: 1,
		Type:  "ethernet",
		State: "up",
		MTU:   1500,
		MAC:   "00:11:22:33:44:55",
		Stats: &iface.InterfaceStats{
			RxBytes: 1000, TxBytes: 2000,
		},
	}

	detail := BuildInterfaceDetailData(info)
	assert.Equal(t, "eth0", detail.Title)
	assert.Equal(t, "/show/iface/", detail.CloseURL)
	require.Len(t, detail.Tabs, 3)
	assert.Equal(t, "config", detail.Tabs[0].Key)
	assert.True(t, detail.Tabs[0].Active)
	assert.Equal(t, "status", detail.Tabs[1].Key)
	assert.Equal(t, "counters", detail.Tabs[2].Key)

	// Config tab should contain interface name and type.
	assert.Contains(t, string(detail.Tabs[0].Content), "eth0")
	assert.Contains(t, string(detail.Tabs[0].Content), "ethernet")

	// Status tab should contain link state.
	assert.Contains(t, string(detail.Tabs[1].Content), "up")

	// Counters tab should contain stats.
	assert.Contains(t, string(detail.Tabs[2].Content), "1000")
	assert.Contains(t, string(detail.Tabs[2].Content), "2000")

	// Tools should include Clear Counters.
	require.Len(t, detail.Tools, 1)
	assert.Equal(t, "Clear Counters", detail.Tools[0].Label)
	assert.Equal(t, "danger", detail.Tools[0].Class)
}

// TestInterfaceDetailData_NilStats verifies detail handles nil stats.
func TestInterfaceDetailData_NilStats(t *testing.T) {
	info := &iface.InterfaceInfo{
		Name:  "dum0",
		Type:  "dummy",
		State: "down",
		MTU:   1500,
	}

	detail := BuildInterfaceDetailData(info)
	assert.Contains(t, string(detail.Tabs[2].Content), "not available")
}

// TestInterfaceTypeDropdown verifies interface type list is derived from iface
// package and excludes loopback.
func TestInterfaceTypeDropdown(t *testing.T) {
	types := InterfaceTypes()
	assert.NotEmpty(t, types)
	assert.Contains(t, types, "ethernet")
	assert.Contains(t, types, "bridge")
	assert.Contains(t, types, "dummy")
	assert.Contains(t, types, "tunnel")
	assert.Contains(t, types, "wireguard")
	assert.NotContains(t, types, "loopback")
}

// TestMatchesTypeFilter_TunnelIncludesKernelTypes verifies tunnel filter
// matches all tunnel kernel link types.
func TestMatchesTypeFilter_TunnelIncludesKernelTypes(t *testing.T) {
	tunnelTypes := []string{"tunnel", "wireguard", "gre", "gretap", "ip6gre", "ip6gretap", "ipip", "sit", "ip6tnl"}
	for _, tt := range tunnelTypes {
		info := iface.InterfaceInfo{Type: tt}
		assert.True(t, matchesTypeFilter(info, "tunnel"), "type %q should match tunnel filter", tt)
	}

	// Non-tunnel types should not match.
	for _, tt := range []string{"ethernet", "bridge", "dummy", "veth"} {
		info := iface.InterfaceInfo{Type: tt}
		assert.False(t, matchesTypeFilter(info, "tunnel"), "type %q should not match tunnel filter", tt)
	}
}

// TestMatchesTypeFilter_VlanByVlanID verifies VLAN filter checks VlanID field.
func TestMatchesTypeFilter_VlanByVlanID(t *testing.T) {
	assert.True(t, matchesTypeFilter(iface.InterfaceInfo{VlanID: 100}, "vlan"))
	assert.False(t, matchesTypeFilter(iface.InterfaceInfo{VlanID: 0}, "vlan"))
}
