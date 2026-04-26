package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// TestAddressTableData_Build verifies address table construction from interfaces.
func TestAddressTableData_Build(t *testing.T) {
	infos := []iface.InterfaceInfo{
		{
			Name: "eth0",
			Addresses: []iface.AddrInfo{
				{Address: "192.168.1.1", PrefixLength: 24, Family: "ipv4"},
				{Address: "2001:db8::1", PrefixLength: 64, Family: "ipv6"},
			},
		},
		{
			Name: "eth1",
			Addresses: []iface.AddrInfo{
				{Address: "10.0.0.1", PrefixLength: 8, Family: "ipv4"},
			},
		},
	}

	data := BuildAddressTableData(infos, "", "")
	assert.Equal(t, "IP Addresses", data.Title)
	require.Len(t, data.Rows, 3)
	assert.Equal(t, "192.168.1.1/24", data.Rows[0].Cells[0])
	assert.Equal(t, "2001:db8::1/64", data.Rows[1].Cells[0])
	assert.Equal(t, "10.0.0.1/8", data.Rows[2].Cells[0])
}

// TestAddressTableData_Protocol verifies IPv4/IPv6 detection.
func TestAddressTableData_Protocol(t *testing.T) {
	infos := []iface.InterfaceInfo{
		{
			Name: "eth0",
			Addresses: []iface.AddrInfo{
				{Address: "192.168.1.1", PrefixLength: 24, Family: "ipv4"},
				{Address: "2001:db8::1", PrefixLength: 64, Family: "ipv6"},
			},
		},
	}

	data := BuildAddressTableData(infos, "", "")
	require.Len(t, data.Rows, 2)
	assert.Equal(t, "IPv4", data.Rows[0].Cells[3]) // family column
	assert.Equal(t, "IPv6", data.Rows[1].Cells[3])
}

// TestAddressTableData_FilterByInterface verifies interface filter.
func TestAddressTableData_FilterByInterface(t *testing.T) {
	infos := []iface.InterfaceInfo{
		{Name: "eth0", Addresses: []iface.AddrInfo{{Address: "192.168.1.1", PrefixLength: 24, Family: "ipv4"}}},
		{Name: "eth1", Addresses: []iface.AddrInfo{{Address: "10.0.0.1", PrefixLength: 8, Family: "ipv4"}}},
	}

	data := BuildAddressTableData(infos, "eth0", "")
	require.Len(t, data.Rows, 1)
	assert.Equal(t, "eth0", data.Rows[0].Cells[2]) // interface column
}

// TestAddressTableData_FilterByProtocol verifies protocol filter.
func TestAddressTableData_FilterByProtocol(t *testing.T) {
	infos := []iface.InterfaceInfo{
		{
			Name: "eth0",
			Addresses: []iface.AddrInfo{
				{Address: "192.168.1.1", PrefixLength: 24, Family: "ipv4"},
				{Address: "2001:db8::1", PrefixLength: 64, Family: "ipv6"},
			},
		},
	}

	data := BuildAddressTableData(infos, "", "IPv4")
	require.Len(t, data.Rows, 1)
	assert.Equal(t, "IPv4", data.Rows[0].Cells[3])

	data = BuildAddressTableData(infos, "", "IPv6")
	require.Len(t, data.Rows, 1)
	assert.Equal(t, "IPv6", data.Rows[0].Cells[3])
}

// TestAddressTableData_Empty verifies empty state.
func TestAddressTableData_Empty(t *testing.T) {
	data := BuildAddressTableData(nil, "", "")
	assert.Empty(t, data.Rows)
	assert.Contains(t, data.EmptyMessage, "No IP addresses")
}

// TestNetworkFromCIDR verifies network address computation from CIDR.
func TestNetworkFromCIDR(t *testing.T) {
	tests := []struct {
		cidr string
		want string
	}{
		{"10.0.0.5/24", "10.0.0.0/24"},
		{"192.168.1.100/16", "192.168.0.0/16"},
		{"10.0.0.1/32", "10.0.0.1/32"},
		{"2001:db8::1/48", "2001:db8::/48"},
		{"invalid", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.cidr, func(t *testing.T) {
			got := NetworkFromCIDR(tt.cidr)
			assert.Equal(t, tt.want, got)
		})
	}
}
