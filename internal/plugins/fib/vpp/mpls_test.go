package fibvpp

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	sysribevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysrib/events"
)

func TestMPLSPush(t *testing.T) {
	mb := &mockMPLSBackend{}
	pfx := netip.MustParsePrefix("10.0.0.0/24")
	nh := netip.MustParseAddr("192.168.1.1")

	err := mb.addMPLSRoute(pfx, nh, []uint32{100})
	require.NoError(t, err)
	require.Len(t, mb.pushes, 1)
	assert.Equal(t, pfx, mb.pushes[0].prefix)
	assert.Equal(t, nh, mb.pushes[0].nextHop)
	assert.Equal(t, []uint32{100}, mb.pushes[0].labels)
}

func TestMPLSSwap(t *testing.T) {
	mb := &mockMPLSBackend{}
	nh := netip.MustParseAddr("192.168.1.1")

	err := mb.addMPLSSwap(100, 200, nh)
	require.NoError(t, err)
	require.Len(t, mb.swaps, 1)
	assert.Equal(t, uint32(100), mb.swaps[0].inLabel)
	assert.Equal(t, uint32(200), mb.swaps[0].outLabel)
	assert.Equal(t, nh, mb.swaps[0].nextHop)
}

func TestMPLSPop(t *testing.T) {
	mb := &mockMPLSBackend{}
	nh := netip.MustParseAddr("192.168.1.1")

	err := mb.addMPLSSwap(100, 3, nh)
	require.NoError(t, err)
	require.Len(t, mb.swaps, 1)
	assert.Equal(t, uint32(3), mb.swaps[0].outLabel)
}

func TestMPLSDelete(t *testing.T) {
	mb := &mockMPLSBackend{}
	pfx := netip.MustParsePrefix("10.0.0.0/24")

	err := mb.delMPLSRoute(pfx, nil)
	require.NoError(t, err)
	require.Len(t, mb.delPushes, 1)
	assert.Equal(t, pfx, mb.delPushes[0])

	err = mb.delMPLSSwap(100)
	require.NoError(t, err)
	require.Len(t, mb.delSwaps, 1)
	assert.Equal(t, uint32(100), mb.delSwaps[0])
}

func TestMPLSInterfaceEnable(t *testing.T) {
	mb := &mockMPLSBackend{}

	err := mb.enableMPLS(1)
	require.NoError(t, err)
	require.Len(t, mb.enables, 1)
	assert.Equal(t, uint32(1), mb.enables[0])

	err = mb.disableMPLS(1)
	require.NoError(t, err)
	require.Len(t, mb.disables, 1)
	assert.Equal(t, uint32(1), mb.disables[0])
}

func TestMPLSLabelRange(t *testing.T) {
	mb := &mockMPLSBackend{}
	pfx := netip.MustParsePrefix("10.0.0.0/24")
	nh := netip.MustParseAddr("192.168.1.1")

	t.Run("max-valid", func(t *testing.T) {
		err := mb.addMPLSRoute(pfx, nh, []uint32{1048575})
		assert.NoError(t, err)
	})

	t.Run("over-max", func(t *testing.T) {
		err := mb.addMPLSRoute(pfx, nh, []uint32{1048576})
		assert.Error(t, err)
	})

	t.Run("zero-valid", func(t *testing.T) {
		err := mb.addMPLSRoute(pfx, nh, []uint32{0})
		assert.NoError(t, err)
	})

	t.Run("empty-stack", func(t *testing.T) {
		err := mb.addMPLSRoute(pfx, nh, []uint32{})
		assert.Error(t, err)
	})

	t.Run("stack-depth-16", func(t *testing.T) {
		labels := make([]uint32, 16)
		for i := range labels {
			labels[i] = uint32(i + 100)
		}
		err := mb.addMPLSRoute(pfx, nh, labels)
		assert.NoError(t, err)
	})

	t.Run("stack-depth-17", func(t *testing.T) {
		labels := make([]uint32, 17)
		err := mb.addMPLSRoute(pfx, nh, labels)
		assert.Error(t, err)
	})
}

func TestProcessEventWithLabels(t *testing.T) {
	ipBackend := &mockBackend{}
	mplsBack := &mockMPLSBackend{}
	f := newFibVPPWithMPLS(ipBackend, mplsBack)

	batch := &incomingBatch{
		Changes: []incomingChange{
			{
				Action:  bgptypes.RouteActionAdd,
				Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
				NextHop: netip.MustParseAddr("192.168.1.1"),
				Labels:  []uint32{100},
			},
		},
	}
	f.processEvent(batch)

	assert.Len(t, ipBackend.adds, 0, "labeled route should not go to IP backend")
	require.Len(t, mplsBack.pushes, 1, "labeled route should go to MPLS backend")
	assert.Equal(t, []uint32{100}, mplsBack.pushes[0].labels)
}

func TestProcessEventWithoutLabels(t *testing.T) {
	ipBackend := &mockBackend{}
	mplsBack := &mockMPLSBackend{}
	f := newFibVPPWithMPLS(ipBackend, mplsBack)

	batch := &incomingBatch{
		Changes: []incomingChange{
			{
				Action:  bgptypes.RouteActionAdd,
				Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
				NextHop: netip.MustParseAddr("192.168.1.1"),
			},
		},
	}
	f.processEvent(batch)

	require.Len(t, ipBackend.adds, 1, "unlabeled route should go to IP backend")
	assert.Len(t, mplsBack.pushes, 0, "unlabeled route should not go to MPLS backend")
}

func TestProcessEventWithdrawLabeled(t *testing.T) {
	ipBackend := &mockBackend{}
	mplsBack := &mockMPLSBackend{}
	f := newFibVPPWithMPLS(ipBackend, mplsBack)

	pfx := netip.MustParsePrefix("10.0.0.0/24")
	nh := netip.MustParseAddr("192.168.1.1")

	f.processEvent(&incomingBatch{
		Changes: []incomingChange{
			{Action: bgptypes.RouteActionAdd, Prefix: pfx, NextHop: nh, Labels: []uint32{100}},
		},
	})
	require.Len(t, mplsBack.pushes, 1)

	f.processEvent(&incomingBatch{
		Changes: []incomingChange{
			{Action: bgptypes.RouteActionWithdraw, Prefix: pfx},
		},
	})
	require.Len(t, mplsBack.delPushes, 1)
	assert.Equal(t, pfx, mplsBack.delPushes[0])
}

// newFibVPPWithMPLS creates a fibVPP with both IP and MPLS backends for testing.
func newFibVPPWithMPLS(ip vppBackend, mpls mplsBackend) *fibVPP {
	return &fibVPP{
		installed:     make(map[string]string),
		mplsInstalled: make(map[string]bool),
		backend:       ip,
		mplsBackend:   mpls,
	}
}

// verify the sysrib event type includes Labels field.
func TestSysribEventLabelsField(t *testing.T) {
	entry := sysribevents.BestChangeEntry{
		Action: bgptypes.RouteActionAdd,
		Prefix: netip.MustParsePrefix("10.0.0.0/24"),
		Labels: []uint32{100, 200},
	}
	assert.Equal(t, []uint32{100, 200}, entry.Labels)

	entryNoLabels := sysribevents.BestChangeEntry{
		Action: bgptypes.RouteActionAdd,
		Prefix: netip.MustParsePrefix("10.0.0.0/24"),
	}
	assert.Nil(t, entryNoLabels.Labels)
}
