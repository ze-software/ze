//go:build integration && linux

// Design: docs/guide/ospf.md -- fast reroute: the kernel carries the primary path only
// Related: nexthop_linux.go -- buildRichRoute, the netlink message the kernel judges
// Related: backend_scope_integration_linux_test.go -- withScopeBackend, the namespace and devices used here

package fibkernel

import (
	"net"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/test/userns"
)

// Goal: a route that carries a fast-reroute backup with repair labels is
// installed in a real kernel with its primary path, so the protected prefix
// keeps forwarding. Linux holds no standby member in RTA_MULTIPATH and refuses
// RTNH_F_LINKDOWN from userspace with EINVAL (fib_get_nhs), which refused the
// whole route, primary included.
// Method: drive the daemon's own path (fibKernel.processEvent over the netlink
// backend) with one best-change entry whose primary is an on-link gateway on
// ze-scope0 and whose backup is an on-link gateway on ze-scope1 with labels
// 17001/17002, the shape observed failing in Ze's guest. Then read the route
// back from the kernel: it must exist, name the primary gateway and device, and
// carry no multipath member and no backup gateway.
func TestFIBBackupRouteInstallsItsPrimary(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	loadMPLSModules(t)
	report.ResetForTest()
	defer report.ResetForTest()
	withScopeBackend(t, func(backend *netlinkBackend, h *netlink.Handle, indices [2]int) {
		primary := netip.MustParseAddr("198.18.247.1")
		backup := netip.MustParseAddr("198.18.248.1")
		entry := sysribevents.BestChangeEntry{
			Action: routeaction.Add, Prefix: netip.MustParsePrefix("198.18.253.0/24"),
			NextHop: primary, Interface: "ze-scope0", OnLink: true, Metric: 20,
			Backup: []sysribevents.ECMPPath{{
				NextHop: backup, Interface: "ze-scope1", OnLink: true, Weight: 1, Labels: []uint32{17001, 17002},
			}},
		}
		writer := newFIBKernel(backend)
		writer.processEvent(&sysribevents.BestChangeBatch{
			Family: family.IPv4Unicast, Changes: []sysribevents.BestChangeEntry{entry},
		})

		routes, err := h.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Protocol: rtprotZE}, netlink.RT_FILTER_PROTOCOL)
		if err != nil {
			t.Fatalf("list Ze routes: %v", err)
		}
		var installed *netlink.Route
		for i := range routes {
			if routes[i].Dst != nil && routes[i].Dst.String() == entry.Prefix.String() {
				installed = &routes[i]
			}
		}
		if installed == nil {
			t.Fatalf("kernel holds no route for %s; FIB errors: %+v", entry.Prefix, report.Errors(0))
		}
		if len(installed.MultiPath) != 0 {
			t.Fatalf("kernel route has %d multipath members, want the primary alone: %+v", len(installed.MultiPath), installed.MultiPath)
		}
		if !installed.Gw.Equal(net.IP(primary.AsSlice())) {
			t.Errorf("kernel gateway = %s, want primary %s", installed.Gw, primary)
		}
		if installed.Gw.Equal(net.IP(backup.AsSlice())) {
			t.Errorf("kernel route forwards through the backup %s", backup)
		}
		if installed.LinkIndex != indices[0] {
			t.Errorf("kernel device = %d, want ze-scope0 (%d)", installed.LinkIndex, indices[0])
		}
		if installed.Encap != nil {
			t.Errorf("unlabelled primary acquired an encapsulation: %+v", installed.Encap)
		}
	})
}
