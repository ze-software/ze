//go:build integration && linux

// Design: docs/architecture/core-design.md -- FIB Linux kernel backend
// Related: backend_linux.go -- listZeRoutes, replaceRoute, replaceOwnedRoute

package fibkernel

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/test/userns"
)

// Goal: the startup listing reports the gateway each Ze route forwards
// through. TestNetlinkIntegration_ListZeRoutes checks only the prefixes.
// Method: install two routes through the writer with different gateways, then
// list them through the backend.
// Expected: each listed route carries its own gateway, in the main table.
func TestNetlinkIntegration_ListZeRoutesReportsEachGateway(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		addLoopback(t, h)

		backend := newTestBackend(h)
		f := newFIBKernel(backend)
		f.processEvent(makeSysribPayload([]incomingChange{
			addChange("10.99.30.0/24", "127.0.0.1"),
			addChange("10.99.31.0/24", "127.0.0.2"),
		}))

		listed, err := backend.listZeRoutes()
		require.NoError(t, err)
		gateways := map[string]string{}
		for _, r := range listed {
			assert.Equal(t, uint32(0), r.table, "%s listed outside the main table", r.prefix)
			gateways[r.prefix] = r.nextHop
		}
		assert.Equal(t, map[string]string{"10.99.30.0/24": "127.0.0.1", "10.99.31.0/24": "127.0.0.2"}, gateways)
	})
}

// Goal: a plain IPv6 update replaces Ze's route in its own metric slot. IPv6
// gives a route installed with no metric the user default 1024, so the
// ownership lookup must look for 1024, not 0.
// Method: install 2001:db8:30::/64 through a gateway on a dummy link, then
// update it to a second gateway on the same link, both through the writer,
// which takes replaceRoute and replaceOwnedRoute.
// Expected: one Ze route for the prefix, at metric 1024, through the second
// gateway, and no fib-sync-failure.
func TestNetlinkIntegration_ReplaceIPv6DefaultMetric(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withNetNS(t, func() {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		addLoopback(t, h)
		link := &netlink.Dummy{}
		link.Name = "ze-v6rep0"
		require.NoError(t, h.LinkAdd(link))
		require.NoError(t, h.LinkSetUp(link))
		address, err := netlink.ParseAddr("2001:db8:31::1/64")
		require.NoError(t, err)
		address.Flags = unix.IFA_F_NODAD
		err = h.AddrAdd(link, address)
		if errors.Is(err, unix.EAFNOSUPPORT) || errors.Is(err, unix.EACCES) {
			t.Skipf("requires enabled IPv6 on ze-v6rep0: %v", err)
		}
		require.NoError(t, err)

		f := newFIBKernel(newTestBackend(h))
		f.processEvent(makeSysribPayload([]incomingChange{
			addChange("2001:db8:30::/64", "2001:db8:31::2"),
		}))
		f.processEvent(makeSysribPayload([]incomingChange{
			updateChange("2001:db8:30::/64", "2001:db8:31::3", "bgp"),
		}))

		var owned []netlink.Route
		routes, err := h.RouteList(nil, netlink.FAMILY_V6)
		require.NoError(t, err)
		for i := range routes {
			if routes[i].Protocol == rtprotZE && routes[i].Dst != nil && routes[i].Dst.String() == "2001:db8:30::/64" {
				owned = append(owned, routes[i])
			}
		}
		require.Len(t, owned, 1, "the update must leave exactly one Ze route: %+v", owned)
		assert.Equal(t, "2001:db8:31::3", owned[0].Gw.String(), "the update must move the route to its new gateway")
		assert.Equal(t, 1024, owned[0].Priority, "the route must stay in the IPv6 default metric slot")
		for _, e := range report.Errors(0) {
			if e.Code == reportCodeFIBSyncFailure {
				t.Errorf("the IPv6 update raised %s for %s: %s", e.Code, e.Subject, e.Message)
			}
		}
	})
}

// Goal: a replace the kernel refuses is reported, and the route it was to
// replace keeps forwarding.
// Method: install 10.99.32.0/24 through 127.0.0.1, then update it to
// 192.0.2.77, a gateway no route in the namespace reaches, so the kernel
// refuses the RTM_NEWROUTE replace.
// Expected: one fib-sync-failure for the prefix with operation replace, and the
// kernel still forwards the prefix through 127.0.0.1.
func TestNetlinkIntegration_RefusedReplaceKeepsTheInstalledRoute(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withNetNS(t, func() {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		addLoopback(t, h)

		f := newFIBKernel(newTestBackend(h))
		f.processEvent(makeSysribPayload([]incomingChange{
			addChange("10.99.32.0/24", "127.0.0.1"),
		}))
		requireForwards(t, h, "10.99.32.1", "10.99.32.0/24", "127.0.0.1", rtprotZE)

		f.processEvent(makeSysribPayload([]incomingChange{
			updateChange("10.99.32.0/24", "192.0.2.77", "bgp"),
		}))

		raised := 0
		for _, e := range report.Errors(0) {
			if e.Code != reportCodeFIBSyncFailure {
				continue
			}
			if e.Subject != "10.99.32.0/24" {
				continue
			}
			raised++
			assert.Equal(t, "replace", e.Detail[keyOperation])
		}
		assert.Equal(t, 1, raised, "the refused replace must raise one fib-sync-failure")
		requireForwards(t, h, "10.99.32.1", "10.99.32.0/24", "127.0.0.1", rtprotZE)
	})
}
