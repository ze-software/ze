// Design: docs/architecture/core-design.md -- FIB Kernel (section 16): rich route programming
// Related: richroute.go -- RichRoute and the richRouteBackend interface
// Related: nexthop_linux.go -- the netlink backend that implements it
//
// The rich path is what Linux actually runs: the netlink backend implements
// richRouteBackend, so any change carrying rich fields (a metric is enough)
// programs the kernel through addRichRoute/delRichRoute. Before this file the
// richMockBackend in fibkernel_test.go was only ever asserted on richAdded --
// nothing asserted that a Withdraw reaches delRichRoute at all.

package fibkernel

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
)

// VALIDATES: on a richRouteBackend, a metric-carrying Add programs the route via
// addRichRoute and the matching Withdraw removes it via delRichRoute for the SAME
// prefix and the same (main, id 0) table -- the install/withdraw pair
// test/plugin/forked-route-install-kernel.ci drives through the real kernel.
// PREVENTS: a withdraw that never reaches the backend's delete, or one aimed at a
// different prefix or table than the add -- the .ci failure
// "route-remove: 10.99.0.0/24 still in kernel after withdrawal".
func TestRichBackendWithdrawDeletesInstalledPrefix(t *testing.T) {
	backend := newRichMockBackend()
	f := newFIBKernel(backend)

	require.NotNil(t, f.asRichBackend(), "rich mock must be seen as a richRouteBackend")

	pfx := netip.MustParsePrefix("10.99.0.0/24")
	nh := netip.MustParseAddr("192.0.2.1")

	// The forked route-install .ci values: admin-distance 110, metric 10. A
	// non-zero metric alone makes hasRichFields() true, so this takes addRichRoute.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: pfx, NextHop: nh, Protocol: "bgp", Metric: 10},
	}))

	require.Len(t, backend.richAdded, 1, "metric-carrying Add must go through addRichRoute")
	assert.Equal(t, pfx, backend.richAdded[0].Prefix)
	assert.Equal(t, uint32(10), backend.richAdded[0].Metric)
	assert.Empty(t, backend.added, "rich path must not also use the plain addRoute")
	assert.Equal(t, nh.String(), f.installed[pfx.String()])

	// sysrib's Withdraw carries only Action+Prefix (recomputeBest's
	// len(protocols)==0 branch), so the delete must key on the prefix alone.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Withdraw, Prefix: pfx},
	}))

	require.Len(t, backend.richDeleted, 1, "Withdraw must issue exactly one rich delete")
	assert.Equal(t, pfx, backend.richDeleted[0], "delete must target the installed prefix")
	require.Len(t, backend.richDelTables, 1)
	assert.Equal(t, uint32(0), backend.richDelTables[0],
		"delete must target the same (main) table the add used")
	assert.NotContains(t, f.installed, pfx.String(),
		"installed tracking must drop the withdrawn prefix")
}
