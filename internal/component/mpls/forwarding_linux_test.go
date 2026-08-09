//go:build linux

// Design: docs/architecture/mpls/mpls-kernel.md -- MPLS operation classifier tests
package mpls

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

func TestMPLSOperationClassifier(t *testing.T) {
	assert.Equal(t, "pop", mplsOperation(nil), "no outgoing label is disposition")
	assert.Equal(t, "pop", mplsOperation([]int{mplsImplicitNull}), "implicit-null only is pop")
	assert.Equal(t, "swap", mplsOperation([]int{16001}), "single real label is swap")
	assert.Equal(t, "swap", mplsOperation([]int{16001, 16002}), "label stack is swap")
}

// VALIDATES: F14 -- `show mpls forwarding` includes label-imposition (push)
// routes, which are ze-owned IP routes carrying an MPLS encap, not just AF_MPLS
// swap/pop entries.
func TestPushEntryFromRoute(t *testing.T) {
	_, ipnet, err := net.ParseCIDR("10.9.0.0/24")
	require.NoError(t, err)

	entry, ok := pushEntryFromRoute(&netlink.Route{
		Protocol:  rtprotZE,
		Dst:       ipnet,
		Gw:        net.ParseIP("10.0.0.2"),
		LinkIndex: 5,
		Encap:     &netlink.MPLSEncap{Labels: []int{300}},
	}, map[int]string{5: "eth0"})
	require.True(t, ok, "a ze IP route with MPLS encap is a push entry")
	assert.Equal(t, "push", entry.Operation)
	assert.Equal(t, "10.9.0.0/24", entry.FEC)
	assert.Equal(t, []int{300}, entry.OutLabels)
	assert.Equal(t, "10.0.0.2", entry.NextHop)
	assert.Equal(t, "eth0", entry.Device)
	assert.Zero(t, entry.InLabel, "a push entry has no incoming label")

	// A foreign-protocol route is not a ze MPLS push.
	_, ok = pushEntryFromRoute(&netlink.Route{
		Protocol: 100, Dst: ipnet, Encap: &netlink.MPLSEncap{Labels: []int{1}},
	}, nil)
	assert.False(t, ok, "foreign-protocol route is not a ze push")

	// A ze route without an MPLS encap is a plain IP route, not a push.
	_, ok = pushEntryFromRoute(&netlink.Route{Protocol: rtprotZE, Dst: ipnet}, nil)
	assert.False(t, ok, "ze route without MPLS encap is not a push")
}
