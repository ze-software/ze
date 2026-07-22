//go:build integration && linux

// Design: plan/spec-mpls-1-kernel.md -- BGP labeled-unicast -> kernel MPLS route,
// the F1 end-to-end path. The deep review found BGP best-paths flow through the
// unified Loc-RIB, whose Path dropped the label stack, so a labeled-unicast route
// installed a plain IP route instead of an MPLS label push. F1 carried the labels
// through locrib.Path + sysrib.changeToBatch into BestChangeEntry.Labels.
//
// This test certifies the LAST mile against a live kernel: a (system-rib,
// best-change) entry carrying Labels installs an IP route with an MPLS label encap
// (ingress imposition). It exercises the production processEvent path that BGP-LU
// takes, which is DISTINCT from the mplsfibevents.handleMPLSEntry path that LDP and
// RSVP-TE use (covered by TestMPLSIntegration_Push / _Swap). Both ultimately reach
// the rich-route backend, but only this path validates the BestChange.Labels ->
// changeToRichRoute -> encap wiring that F1 repaired.
package fibkernel

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/routeaction"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

// labeledChange builds a (system-rib, best-change) entry for a BGP labeled-unicast
// route: a prefix FEC, the imposed label stack, and the on-link next-hop 10.0.0.2
// (reachable via the ze-mpls0 /24 that setupDummyLink installs). A BGP-LU
// best-change always carries a next-hop, so it is fixed here rather than a param
// (the no-next-hop egress-pop case is the LDP/RSVP-TE mplsfibevents path).
func labeledChange(action routeaction.Action, prefix string, labels []uint32) incomingChange {
	return incomingChange{
		Action:   action,
		Prefix:   netip.MustParsePrefix(prefix),
		NextHop:  netip.MustParseAddr("10.0.0.2"),
		Protocol: "bgp",
		Labels:   labels,
	}
}

// VALIDATES: mpls-1 AC-1/AC-2/AC-3 -- a BGP labeled-unicast best-change carrying a
// label stack installs an IP route with an MPLS label encap (push), updates the
// encap on relabel, and removes it on withdraw, all through the production sysrib
// BestChange -> processEvent -> rich-route path. This is the F1 fix's last mile:
// the path BGP-LU takes, distinct from the LDP/RSVP-TE mplsfibevents path covered
// by TestMPLSIntegration_Push.
// PREVENTS: regression of F1, where labels were dropped before the kernel and a
// labeled-unicast route silently became a plain IP route.
func TestMPLSIntegration_BGPLabeledUnicastPush(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		enableNetnsMPLS(t)
		setupDummyLink(t, h)

		f := newFIBKernel(newTestBackend(h))

		// AC-1: BGP labeled-unicast route -- FEC 10.9.0.0/24, next-hop on the
		// connected /24, imposed label 300 -- installs a kernel encap (push).
		f.processEvent(makeSysribPayload([]incomingChange{
			labeledChange(routeaction.Add, "10.9.0.0/24", []uint32{300}),
		}))
		require.Equal(t, []int{300}, pushEncapLabels(t, h, "10.9.0.0/24"),
			"BGP labeled-unicast route must install an MPLS label encap (push), not a plain IP route")

		// AC-3: re-advertise the same FEC with a new label -- the kernel route is
		// replaced, not left with the stale label.
		f.processEvent(makeSysribPayload([]incomingChange{
			labeledChange(routeaction.Update, "10.9.0.0/24", []uint32{400}),
		}))
		require.Equal(t, []int{400}, pushEncapLabels(t, h, "10.9.0.0/24"),
			"relabel must update the kernel encap")

		// AC-2: withdraw removes the route from the kernel.
		f.processEvent(makeSysribPayload([]incomingChange{
			withdrawChange("10.9.0.0/24"),
		}))
		routes, err := h.RouteList(nil, netlink.FAMILY_V4)
		require.NoError(t, err)
		for i := range routes {
			if routes[i].Protocol == rtprotZE && routes[i].Dst != nil && routes[i].Dst.String() == "10.9.0.0/24" {
				t.Fatal("labeled-unicast route should be gone after withdraw")
			}
		}
	})
}

// VALIDATES: mpls-1 -- a BGP labeled-unicast push for a prefix that another FIB
// writer already owns must NOT clobber that route. The labeled BestChange takes the
// rich-route add path (RouteAdd, which fails EEXIST) so a foreign route keeps its
// owner and gains no MPLS encap.
// PREVENTS: a BGP-LU label install stomping a static/connected route for the same
// prefix.
func TestMPLSIntegration_BGPLabeledUnicastNoClobberForeign(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		enableNetnsMPLS(t)
		setupDummyLink(t, h)

		const foreignProto = 100 // not rtprotZE
		addProtocolRoute(t, h, "10.9.0.0/24", "10.0.0.2", foreignProto)

		f := newFIBKernel(newTestBackend(h))
		f.processEvent(makeSysribPayload([]incomingChange{
			labeledChange(routeaction.Add, "10.9.0.0/24", []uint32{300}),
		}))

		routes, err := h.RouteList(nil, netlink.FAMILY_V4)
		require.NoError(t, err)
		var found *netlink.Route
		for i := range routes {
			if routes[i].Dst != nil && routes[i].Dst.String() == "10.9.0.0/24" {
				found = &routes[i]
			}
		}
		require.NotNil(t, found, "foreign route disappeared")
		require.Equal(t, foreignProto, int(found.Protocol), "foreign route must keep its owner")
		require.Nil(t, found.Encap, "foreign route must not gain an MPLS label encap")
	})
}
