package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/iface"
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

// TestInterfaceTableData_FilterEthernetMatchesKernelDevice verifies Linux
// netlink physical links are classified as Ethernet for display and filtering.
// VALIDATES: an interface with kernel type "device" appears on the Ethernet page.
// PREVENTS: physical appliance NICs disappearing from /show/iface/?type=ethernet.
func TestInterfaceTableData_FilterEthernetMatchesKernelDevice(t *testing.T) {
	infos := []iface.InterfaceInfo{
		{Name: "enp1s0", Type: "device", State: "up", MTU: 1500, MAC: "00:11:22:33:44:55"},
	}

	data := BuildInterfaceTableData(infos, "ethernet")
	require.Len(t, data.Rows, 1)
	assert.Equal(t, "enp1s0", data.Rows[0].Key)
	assert.Equal(t, "ethernet", data.Rows[0].Cells[1])
}

// TestBuildInterfaceTableDataForView_IncludesConfiguredOnlyTypes verifies the
// workbench source includes configuration entries even when the OS has no link.
// VALIDATES: every configured interface type remains visible after navigation.
// PREVENTS: pending or unapplied interface config disappearing from tables.
func TestBuildInterfaceTableDataForView_IncludesConfiguredOnlyTypes(t *testing.T) {
	tree := config.NewTree()
	ifaceTree := config.NewTree()
	for _, typ := range []string{"ethernet", "bridge", "dummy", "veth", "tunnel", "wireguard", "xfrm"} {
		entry := config.NewTree()
		if typ == "ethernet" {
			mac := config.NewTree()
			mac.Set("address", "11:22:33:44:55:66")
			entry.SetContainer("mac", mac)
		}
		ifaceTree.AddListEntry(typ, typ+"-test", entry)
	}
	tree.SetContainer("interface", ifaceTree)

	data := buildInterfaceTableDataForView(nil, tree, "")
	require.Len(t, data.Rows, 7)
	for _, typ := range []string{"ethernet", "bridge", "dummy", "veth", "tunnel", "wireguard", "xfrm"} {
		row := requireInterfaceRow(t, data, typ+"-test")
		assert.Equal(t, typ, row.Cells[1])
		assert.Equal(t, "configured", row.Cells[2])
	}

	ethernet := buildInterfaceTableDataForView(nil, tree, "ethernet")
	require.Len(t, ethernet.Rows, 1)
	assert.Equal(t, "ethernet-test", ethernet.Rows[0].Key)
	assert.Equal(t, "11:22:33:44:55:66", ethernet.Rows[0].Cells[4])
}

// TestBuildInterfaceTableDataForView_IncludesConfiguredVLANUnits verifies VLAN
// units render as VLAN interface rows before their OS subinterface exists.
// VALIDATES: interface ethernet <name> unit <n> vlan-id <id> appears on VLAN page.
// PREVENTS: configured VLANs disappearing until the config has been committed.
func TestBuildInterfaceTableDataForView_IncludesConfiguredVLANUnits(t *testing.T) {
	tree := config.NewTree()
	ifaceTree := config.NewTree()
	eth := config.NewTree()
	mac := config.NewTree()
	mac.Set("address", "aa:bb:cc:dd:ee:ff")
	eth.SetContainer("mac", mac)
	unit := config.NewTree()
	unit.Set("vlan-id", "100")
	ipv4 := config.NewTree()
	ipv4.SetSlice("address", []string{"192.0.2.1/24"})
	unit.SetContainer("ipv4", ipv4)
	eth.AddListEntry("unit", "default", unit)
	ifaceTree.AddListEntry("ethernet", "uplink", eth)
	tree.SetContainer("interface", ifaceTree)

	data := buildInterfaceTableDataForView(nil, tree, "vlan")
	require.Len(t, data.Rows, 1)
	assert.Equal(t, "uplink.100", data.Rows[0].Key)
	assert.Equal(t, "vlan", data.Rows[0].Cells[1])
	assert.Equal(t, "192.0.2.1/24", data.Rows[0].Cells[5])
}

// TestBuildInterfaceTableDataForView_VLANWithMACNotOnEthernetPage verifies a
// VLAN entry whose parent has a MAC does not leak onto the Ethernet filter page.
// PREVENTS: CanonicalInterfaceType MAC fallback reclassifying VLANs as ethernet.
func TestBuildInterfaceTableDataForView_VLANWithMACNotOnEthernetPage(t *testing.T) {
	tree := config.NewTree()
	ifaceTree := config.NewTree()
	eth := config.NewTree()
	mac := config.NewTree()
	mac.Set("address", "aa:bb:cc:dd:ee:ff")
	eth.SetContainer("mac", mac)
	unit := config.NewTree()
	unit.Set("vlan-id", "200")
	eth.AddListEntry("unit", "default", unit)
	ifaceTree.AddListEntry("ethernet", "wan0", eth)
	tree.SetContainer("interface", ifaceTree)

	vlanData := buildInterfaceTableDataForView(nil, tree, "vlan")
	require.Len(t, vlanData.Rows, 1)
	assert.Equal(t, "wan0.200", vlanData.Rows[0].Key)

	ethernetData := buildInterfaceTableDataForView(nil, tree, "ethernet")
	for _, row := range ethernetData.Rows {
		assert.NotEqual(t, "wan0.200", row.Key, "VLAN row must not appear on ethernet page")
	}
}

// TestConfiguredUnitVLANID_Boundaries verifies VLAN ID parsing edge cases.
func TestConfiguredUnitVLANID_Boundaries(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*config.Tree)
		expect int
	}{
		{"nil tree", func(_ *config.Tree) {}, 0},
		{"missing key", func(tree *config.Tree) {}, 0},
		{"zero", func(tree *config.Tree) { tree.Set("vlan-id", "0") }, 0},
		{"negative", func(tree *config.Tree) { tree.Set("vlan-id", "-1") }, 0},
		{"non-numeric", func(tree *config.Tree) { tree.Set("vlan-id", "abc") }, 0},
		{"valid", func(tree *config.Tree) { tree.Set("vlan-id", "100") }, 100},
		{"max 4094", func(tree *config.Tree) { tree.Set("vlan-id", "4094") }, 4094},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree := config.NewTree()
			tc.setup(tree)
			got := configuredUnitVLANID(tree)
			assert.Equal(t, tc.expect, got)
		})
	}
	assert.Equal(t, 0, configuredUnitVLANID(nil))
}

// TestConfiguredInterfaceInfo_MTUBoundaries verifies MTU parsing edge cases.
func TestConfiguredInterfaceInfo_MTUBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		mtu    string
		expect int
	}{
		{"zero", "0", defaultInterfaceMTU},
		{"negative", "-1", defaultInterfaceMTU},
		{"non-numeric", "abc", defaultInterfaceMTU},
		{"valid", "9000", 9000},
		{"minimum", "1", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := config.NewTree()
			entry.Set("mtu", tc.mtu)
			info := configuredInterfaceInfo("eth0", "ethernet", entry)
			assert.Equal(t, tc.expect, info.MTU)
		})
	}
	info := configuredInterfaceInfo("eth0", "ethernet", nil)
	assert.Equal(t, defaultInterfaceMTU, info.MTU)
}

// TestConfiguredAddrInfo_Boundaries verifies CIDR address parsing edge cases.
func TestConfiguredAddrInfo_Boundaries(t *testing.T) {
	tests := []struct {
		name   string
		cidr   string
		addr   string
		prefix int
		family string
	}{
		{"ipv4 with prefix", "192.0.2.1/24", "192.0.2.1", 24, "ipv4"},
		{"ipv6 with prefix", "2001:db8::1/64", "2001:db8::1", 64, "ipv6"},
		{"no slash", "10.0.0.1", "10.0.0.1", 0, "ipv4"},
		{"non-numeric prefix", "10.0.0.1/abc", "10.0.0.1", 0, "ipv4"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := configuredAddrInfo(tc.cidr, tc.family)
			assert.Equal(t, tc.addr, info.Address)
			assert.Equal(t, tc.prefix, info.PrefixLength)
			assert.Equal(t, tc.family, info.Family)
		})
	}
}

func requireInterfaceRow(t *testing.T, data WorkbenchTableData, key string) WorkbenchTableRow {
	t.Helper()
	for _, row := range data.Rows {
		if row.Key == key {
			return row
		}
	}
	require.Failf(t, "missing interface row", "key %q in %#v", key, data.Rows)
	return WorkbenchTableRow{}
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
