//go:build integration && linux

// Design: docs/guide/mpls.md -- the label space ze writes before it programs a label
// Related: nexthop_linux.go -- addRichRoute, replaceRichRoute, needsLabelSpace
// Related: labelspace_linux.go -- ensureLabelSpace, repairLabelSpace

package fibkernel

import (
	"net/netip"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/test/userns"
)

// readNetnsLabelSpace answers net.mpls.platform_labels of the namespace the
// calling thread is in. withNetNS locks the thread, so this is the test's own
// namespace, which the kernel creates with a label space of 0 (mpls_net_init).
func readNetnsLabelSpace(t *testing.T) string {
	t.Helper()
	value, err := os.ReadFile(mplsPlatformLabels)
	require.NoError(t, err, "read net.mpls.platform_labels in the test namespace")
	return strings.TrimSpace(string(value))
}

// labeledRoute finds the Ze-owned IPv4 route for dst in the kernel.
func labeledRoute(t *testing.T, h *netlink.Handle, dst string) netlink.Route {
	t.Helper()
	routes, err := h.RouteList(nil, netlink.FAMILY_V4)
	require.NoError(t, err)
	for i := range routes {
		if routes[i].Protocol == rtprotZE && routes[i].Dst != nil && routes[i].Dst.String() == dst {
			return routes[i]
		}
	}
	t.Fatalf("the kernel holds no Ze route for %s", dst)
	return netlink.Route{}
}

// Goal: the first labeled install through the writer enables the namespace's
// MPLS label space before the push route goes in.
// Method: a fresh namespace, whose label space the kernel starts at 0, gets a
// labeled BGP route through processEvent, which takes addRichRoute. The test
// does not call enableNetnsMPLS: that is the state an appliance boots in.
// userns.Enter runs this test alone in a child process, so labelSpaceOnce has
// not fired before it.
// Expected: the kernel holds the route with the label stack [300], and the
// label space reads 1048575 (labelSpaceMax) afterwards.
func TestFIBLabeledAddEnablesTheLabelSpace(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		setupDummyLink(t, h)
		require.Equal(t, "0", readNetnsLabelSpace(t), "precondition: a new namespace starts with no label space")

		f := newFIBKernel(newTestBackend(h))
		f.processEvent(makeSysribPayload([]incomingChange{
			labeledChange(routeaction.Add, "10.9.1.0/24", []uint32{300}),
		}))

		require.Equal(t, []int{300}, pushEncapLabels(t, h, "10.9.1.0/24"),
			"the labeled route must reach the kernel with its label stack")
		require.Equal(t, "1048575", readNetnsLabelSpace(t),
			"a labeled install must leave the label space enabled")
	})
}

// Goal: a replace that first brings a label (here on an ECMP member only)
// enables the label space, and an unlabeled install before it does not.
// Method: a fresh namespace gets an unlabeled rich route (a metric makes it
// rich, so it takes addRichRoute), then an update of the same prefix whose
// second member carries label 500, which takes replaceRichRoute.
// Expected: the label space still reads 0 after the unlabeled install, reads
// 1048575 after the replace, and the kernel holds two members, the labeled one
// through 10.0.0.3 with stack [500] and the primary through 10.0.0.2 with none.
func TestFIBLabeledReplaceEnablesTheLabelSpace(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		setupDummyLink(t, h)
		require.Equal(t, "0", readNetnsLabelSpace(t), "precondition: a new namespace starts with no label space")

		const dst = "10.9.2.0/24"
		change := incomingChange{
			Action:   routeaction.Add,
			Prefix:   netip.MustParsePrefix(dst),
			NextHop:  netip.MustParseAddr("10.0.0.2"),
			Protocol: "bgp",
			Metric:   100,
		}
		f := newFIBKernel(newTestBackend(h))
		f.processEvent(makeSysribPayload([]incomingChange{change}))
		require.Equal(t, 100, labeledRoute(t, h, dst).Priority, "control: the unlabeled rich route must be installed")
		require.Equal(t, "0", readNetnsLabelSpace(t), "an install with no label must leave the label space alone")

		change.Action = routeaction.Update
		change.ECMPPaths = []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("10.0.0.3"), Labels: []uint32{500}}}
		f.processEvent(makeSysribPayload([]incomingChange{change}))

		require.Equal(t, "1048575", readNetnsLabelSpace(t),
			"a replace that brings the first label must leave the label space enabled")
		route := labeledRoute(t, h, dst)
		require.Len(t, route.MultiPath, 2, "the replace must install both members: %+v", route)
		stacks := map[string][]int{}
		for _, member := range route.MultiPath {
			var labels []int
			if encap, ok := member.Encap.(*netlink.MPLSEncap); ok {
				labels = encap.Labels
			}
			stacks[member.Gw.String()] = labels
		}
		require.Equal(t, map[string][]int{"10.0.0.2": nil, "10.0.0.3": {500}}, stacks,
			"each member must carry its own label stack")
	})
}
