//go:build linux

// Design: docs/guide/ospf.md -- fast reroute: the kernel carries the primary path only
// Related: nexthop_linux.go -- buildRichRoute, the netlink message under test

package fibkernel

import (
	"net"
	"net/netip"
	"reflect"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
)

// assertNoBackupMember fails when the built route names the backup gateway
// anywhere, or carries RTNH_F_LINKDOWN on the route or on any member. Linux
// refuses RTNH_F_LINKDOWN from userspace (net/ipv4/fib_semantics.c,
// fib_get_nhs: "Invalid flags for nexthop - can not contain DEAD or
// LINKDOWN"), and an unflagged backup member would load-share in steady state.
func assertNoBackupMember(t *testing.T, route *netlink.Route, backup netip.Addr) {
	t.Helper()
	backupIP := net.IP(backup.AsSlice())
	if route.Flags&unix.RTNH_F_LINKDOWN != 0 {
		t.Errorf("route carries RTNH_F_LINKDOWN: %+v", route)
	}
	if route.Gw.Equal(backupIP) {
		t.Errorf("route gateway is the backup %s", backup)
	}
	if via, ok := route.Via.(*netlink.Via); ok && via.Addr.Equal(backupIP) {
		t.Errorf("route reaches the backup %s through RTA_VIA", backup)
	}
	for index, member := range route.MultiPath {
		if member.Flags&unix.RTNH_F_LINKDOWN != 0 {
			t.Errorf("member %d carries RTNH_F_LINKDOWN: %+v", index, member)
		}
		if member.Gw.Equal(backupIP) {
			t.Errorf("member %d is the backup next hop %s: %+v", index, backup, member)
		}
		if via, ok := member.Via.(*netlink.Via); ok && via.Addr.Equal(backupIP) {
			t.Errorf("member %d reaches the backup %s through RTA_VIA: %+v", index, backup, member)
		}
	}
}

// Goal: a single-path route that carries a fast-reroute backup is sent to the
// kernel exactly as the same route without the backup: one next hop, its own
// gateway, on-link flag and route-level encapsulation, and no multipath list.
// The backup stays in Ze's state and never reaches the netlink message.
// Method: build each route twice, with its backup and with Backup cleared, and
// compare the two netlink routes field by field (reflect.DeepEqual follows the
// encapsulation pointer), then check that no field names the backup.
func TestBuildRichRouteBackupSinglePathMatchesPrimaryAlone(t *testing.T) {
	backupGateway := netip.MustParseAddr("198.18.248.1")
	cases := []struct {
		name   string
		route  RichRoute
		backup netip.Addr
	}{
		{
			name: "on-link gateway, labeled backup",
			route: RichRoute{
				Prefix:  netip.MustParsePrefix("198.18.253.0/24"),
				NextHop: netip.MustParseAddr("198.18.247.1"), OnLink: true, Metric: 20,
				Backup: []sysribevents.ECMPPath{{
					NextHop: backupGateway, OnLink: true, Weight: 1, Labels: []uint32{17001, 17002},
				}},
			},
			backup: backupGateway,
		},
		{
			name: "labeled primary with path MTU",
			route: RichRoute{
				Prefix:  netip.MustParsePrefix("10.20.0.0/24"),
				NextHop: netip.MustParseAddr("10.0.0.2"), Metric: 110, Labels: []uint32{16003}, PathMTU: 1400,
				Backup: []sysribevents.ECMPPath{{
					NextHop: netip.MustParseAddr("10.0.0.9"), Labels: []uint32{16010, 24003},
				}},
			},
			backup: netip.MustParseAddr("10.0.0.9"),
		},
		{
			name: "IPv4 through an IPv6 gateway",
			route: RichRoute{
				Prefix:  netip.MustParsePrefix("198.51.100.0/24"),
				NextHop: netip.MustParseAddr("fe80::1"), OnLink: true, Weight: 3,
				Backup: []sysribevents.ECMPPath{{
					NextHop: netip.MustParseAddr("fe80::2"), OnLink: true, Weight: 2, Labels: []uint32{16000},
				}},
			},
			backup: netip.MustParseAddr("fe80::2"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildRichRoute(tc.route)
			if err != nil {
				t.Fatalf("build route with backup: %v", err)
			}
			alone := tc.route
			alone.Backup = nil
			want, err := buildRichRoute(alone)
			if err != nil {
				t.Fatalf("build route without backup: %v", err)
			}
			if len(got.MultiPath) != 0 {
				t.Fatalf("single-path route promoted to %d multipath members: %+v", len(got.MultiPath), got.MultiPath)
			}
			if !reflect.DeepEqual(*got, *want) {
				t.Fatalf("backup changed the primary encoding:\n with backup: %+v\n without:     %+v", *got, *want)
			}
			assertNoBackupMember(t, got, tc.backup)
		})
	}
}

// Goal: an ECMP route that carries a fast-reroute backup is sent to the kernel
// with exactly its equal-cost members, each with its own gateway, weight,
// on-link flag and label stack, and no member for the backup.
// Method: build the ECMP route with and without its backup, compare the two
// netlink routes field by field, and check that no member names the backup.
func TestBuildRichRouteBackupECMPKeepsOnlyItsMembers(t *testing.T) {
	backupGateway := netip.MustParseAddr("192.0.2.9")
	route := RichRoute{
		Prefix:  netip.MustParsePrefix("203.0.113.0/24"),
		NextHop: netip.MustParseAddr("192.0.2.1"), OnLink: true, Weight: 2, Labels: []uint32{16001},
		ECMPPaths: []sysribevents.ECMPPath{
			{NextHop: netip.MustParseAddr("192.0.2.2"), Weight: 1, Labels: []uint32{16002, 24002}},
		},
		Backup: []sysribevents.ECMPPath{
			{NextHop: backupGateway, OnLink: true, Labels: []uint32{16010, 24003}},
		},
	}
	got, err := buildRichRoute(route)
	if err != nil {
		t.Fatalf("build ECMP route with backup: %v", err)
	}
	alone := route
	alone.Backup = nil
	want, err := buildRichRoute(alone)
	if err != nil {
		t.Fatalf("build ECMP route without backup: %v", err)
	}
	if len(got.MultiPath) != 2 {
		t.Fatalf("ECMP route has %d members, want its 2 equal-cost members: %+v", len(got.MultiPath), got.MultiPath)
	}
	if !reflect.DeepEqual(*got, *want) {
		t.Fatalf("backup changed the ECMP encoding:\n with backup: %+v\n without:     %+v", *got, *want)
	}
	assertNoBackupMember(t, got, backupGateway)
}
