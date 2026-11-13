// VALIDATES: AC-11 / R-7 -- buildRichRoute sends a route that carries a
// fast-reroute backup with its primary next-hop alone: no multipath list, the
// primary gateway, no RTNH_F_LINKDOWN on the route or any member, and no member
// for the backup gateway. Linux has no standby multipath member and refuses
// RTNH_F_LINKDOWN from userspace (net/ipv4/fib_semantics.c, fib_get_nhs), so a
// link-down backup member would make the kernel refuse the whole route.
// Linux-only (netlink); runs under QEMU per ai/rules/platform-linux.md.
// PREVENTS: the backup costing the protected prefix its kernel route, or the
// backup load-sharing as an ordinary multipath member.

//go:build linux

package fibkernel

import (
	"net"
	"net/netip"
	"testing"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"

	"golang.org/x/sys/unix"
)

func TestBuildRichRouteBackupLinkDown(t *testing.T) {
	r := RichRoute{
		Prefix:  netip.MustParsePrefix("10.20.0.0/24"),
		NextHop: netip.MustParseAddr("10.0.0.2"),
		Metric:  110,
		Backup: []sysribevents.ECMPPath{{
			NextHop: netip.MustParseAddr("10.0.0.9"),
			Labels:  []uint32{16010, 24003},
		}},
	}
	route, err := buildRichRoute(r)
	if err != nil {
		t.Fatalf("buildRichRoute: %v", err)
	}
	if len(route.MultiPath) != 0 {
		t.Fatalf("MultiPath len = %d, want 0 (the primary alone): %+v", len(route.MultiPath), route.MultiPath)
	}
	if !route.Gw.Equal(net.ParseIP("10.0.0.2")) {
		t.Fatalf("route gateway = %v, want the primary 10.0.0.2", route.Gw)
	}
	if route.Flags&int(unix.RTNH_F_LINKDOWN) != 0 {
		t.Fatal("route carries RTNH_F_LINKDOWN")
	}
	backup := net.ParseIP("10.0.0.9")
	for index, member := range route.MultiPath {
		if member.Flags&int(unix.RTNH_F_LINKDOWN) != 0 {
			t.Fatalf("member %d carries RTNH_F_LINKDOWN: %+v", index, member)
		}
		if member.Gw.Equal(backup) {
			t.Fatalf("member %d is the backup gateway 10.0.0.9: %+v", index, member)
		}
	}
	if route.Gw.Equal(backup) {
		t.Fatal("route gateway is the backup 10.0.0.9")
	}
}
