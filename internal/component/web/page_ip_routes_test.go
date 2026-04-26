package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// TestRouteTableData_Build verifies route table construction.
func TestRouteTableData_Build(t *testing.T) {
	routes := []iface.KernelRoute{
		{Destination: "0.0.0.0/0", NextHop: "192.168.1.1", Device: "eth0", Protocol: "static", Metric: 100, Family: "ipv4"},
		{Destination: "10.0.0.0/8", NextHop: "", Device: "eth1", Protocol: "kernel", Metric: 0, Family: "ipv4"},
		{Destination: "172.16.0.0/12", NextHop: "10.0.0.1", Device: "eth1", Protocol: "bgp", Metric: 20, Family: "ipv4"},
	}

	data := BuildRouteTableData(routes, "")
	assert.Equal(t, "Routes", data.Title)
	require.Len(t, data.Rows, 3)
	assert.Equal(t, "0.0.0.0/0", data.Rows[0].Key)
}

// TestRouteTableData_Flags verifies route flag computation.
func TestRouteTableData_Flags(t *testing.T) {
	tests := []struct {
		protocol  string
		wantFlags string
	}{
		{"static", "A S"},
		{"bgp", "A B"},
		{"kernel", "A C"},
		{"ze", "A D"},
		{"dhcp", "A D"},
	}

	for _, tt := range tests {
		t.Run(tt.protocol, func(t *testing.T) {
			route := iface.KernelRoute{Protocol: tt.protocol}
			flags, _ := routeFlag(route)
			assert.Equal(t, tt.wantFlags, flags)
		})
	}
}

// TestRouteTableData_FilterByProtocol verifies protocol filter.
func TestRouteTableData_FilterByProtocol(t *testing.T) {
	routes := []iface.KernelRoute{
		{Destination: "0.0.0.0/0", Protocol: "static", Family: "ipv4"},
		{Destination: "10.0.0.0/8", Protocol: "kernel", Family: "ipv4"},
		{Destination: "172.16.0.0/12", Protocol: "bgp", Family: "ipv4"},
	}

	data := BuildRouteTableData(routes, "static")
	require.Len(t, data.Rows, 1)
	assert.Equal(t, "0.0.0.0/0", data.Rows[0].Key)

	data = BuildRouteTableData(routes, "bgp")
	require.Len(t, data.Rows, 1)
	assert.Equal(t, "172.16.0.0/12", data.Rows[0].Key)
}

// TestRouteTableData_EmptyGateway verifies "-" placeholder for empty gateway.
func TestRouteTableData_EmptyGateway(t *testing.T) {
	routes := []iface.KernelRoute{
		{Destination: "10.0.0.0/8", NextHop: "", Device: "eth0", Protocol: "kernel", Family: "ipv4"},
	}

	data := BuildRouteTableData(routes, "")
	require.Len(t, data.Rows, 1)
	assert.Equal(t, "-", data.Rows[0].Cells[1]) // gateway column
}

// TestRouteTableData_Empty verifies empty state.
func TestRouteTableData_Empty(t *testing.T) {
	data := BuildRouteTableData(nil, "")
	assert.Empty(t, data.Rows)
	assert.Contains(t, data.EmptyMessage, "No routes")
}
