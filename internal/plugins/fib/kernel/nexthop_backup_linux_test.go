// VALIDATES: AC-11 / R-7 -- buildRichRoute emits a fast-reroute backup as a
// link-down-flagged multipath next-hop carrying the SR repair MPLS encap, so the
// kernel forwards to it only when the primary link is down. Linux-only (netlink);
// runs under QEMU per ai/rules/qemu-testing.md.
// PREVENTS: a backup that is never installed, or one the kernel always uses.

//go:build linux

package fibkernel

import (
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
	// A single primary + one backup becomes a two-way multipath.
	if len(route.MultiPath) != 2 {
		t.Fatalf("MultiPath len = %d, want 2 (primary + backup)", len(route.MultiPath))
	}
	backup := route.MultiPath[1]
	if backup.Gw.String() != "10.0.0.9" {
		t.Fatalf("backup next-hop = %v, want 10.0.0.9", backup.Gw)
	}
	if backup.Flags&int(unix.RTNH_F_LINKDOWN) == 0 {
		t.Fatal("backup next-hop is not RTNH_F_LINKDOWN flagged")
	}
	if backup.Encap == nil {
		t.Fatal("backup next-hop missing SR repair MPLS encap")
	}
	// The primary next-hop is NOT link-down flagged (used in steady state).
	if route.MultiPath[0].Flags&int(unix.RTNH_F_LINKDOWN) != 0 {
		t.Fatal("primary next-hop must not be link-down flagged")
	}
}
