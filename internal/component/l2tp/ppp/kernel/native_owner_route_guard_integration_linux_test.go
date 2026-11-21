//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- affected native PPP route legs
// Related: native_owner_guard_integration_linux_test.go -- actual creator ABI fixture
// UAPI: frozen 0008-ppp-original-file-route-guard.patch after frozen 0007.
// These direct kernel tests do not prove SessionHandle/RouteReplace wiring.

package kernel_test

import (
	"net"
	"reflect"
	"slices"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

const nativeOwnerRouteAttr = 33 // Linux v7.3 RTA_DEL_REASON=32, then frozen 0008.

// Linux v7.3 include/uapi/linux/rtnetlink.h; this x/sys version omits RTA_NH_ID.
const nativeOwnerNexthopIDAttr = 30

// TestNativeOwnerGuardRoute4 proves useful add, replacement and withdrawal on
// the original device, and refusal of a different live creator for both writers.
func TestNativeOwnerGuardRoute4(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeOwnerRouteCase(t, unix.AF_INET)
}

// TestNativeOwnerGuardRoute6 proves the same contract through IPv6's own route
// writer, with a separate feature probe before its first guarded write.
func TestNativeOwnerGuardRoute6(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeOwnerRouteCase(t, unix.AF_INET6)
}

// TestNativeOwnerGuardRoute4ForeignReplace refuses replacement of an actual old
// foreign-device alias even though the requested new OIF has the right creator.
func TestNativeOwnerGuardRoute4ForeignReplace(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeOwnerRouteForeignReplace(t, unix.AF_INET)
}

// TestNativeOwnerGuardRoute6ForeignReplace requires refusal before the IPv6
// replacement candidate can remove another unit's actual old path.
func TestNativeOwnerGuardRoute6ForeignReplace(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeOwnerRouteForeignReplace(t, unix.AF_INET6)
}

// TestNativeOwnerGuardRoute4MultipathDelete refuses deleting an old shared
// alias selected by a single matching first-leg OIF and preserves both legs.
func TestNativeOwnerGuardRoute4MultipathDelete(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	first := nativeOwnerFixtureNew(t)
	second := nativeOwnerFixtureNew(t)
	nativeOwnerRouteReady(t, first, unix.AF_INET)
	nativeOwnerRouteReady(t, second, unix.AF_INET)
	route := nativeOwnerRoute(t, unix.AF_INET, first.link.Attrs().Index)
	route.LinkIndex = 0
	route.MultiPath = []*netlink.NexthopInfo{{LinkIndex: first.link.Attrs().Index}, {LinkIndex: second.link.Attrs().Index}}
	if err := netlink.RouteAdd(route); err != nil {
		t.Fatal(err)
	}
	before := nativeOwnerRoutes(t, unix.AF_INET)
	nativeOwnerRequireLegs(t, before, first.link.Attrs().Index, second.link.Attrs().Index)
	request := nativeOwnerRouteRequest(unix.RTM_DELROUTE, 0, unix.AF_INET, first.fd, nativeOwnerRoute(t, unix.AF_INET, first.link.Attrs().Index))
	nativeOwnerErrno(t, first.send(request), unix.EOPNOTSUPP)
	nativeOwnerRoutesEqual(t, unix.AF_INET, before)
	if err := netlink.RouteDel(route); err != nil {
		t.Fatalf("unguarded shared-alias deletion control: %v", err)
	}
	if len(nativeOwnerRoutes(t, unix.AF_INET)) != 0 {
		t.Fatal("unguarded shared-alias deletion did not remove both legs")
	}
}

// TestNativeOwnerGuardRoute6SiblingDeleteAll refuses the gatewayless writer
// that would otherwise remove every member of a mixed-device sibling group.
func TestNativeOwnerGuardRoute6SiblingDeleteAll(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeOwnerRoute6Siblings(t, false, false)
}

// TestNativeOwnerGuardRoute6SiblingDeleteGateway refuses even gateway-qualified
// deletion: fib6_del also updates the surviving foreign sibling's metadata.
func TestNativeOwnerGuardRoute6SiblingDeleteGateway(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeOwnerRoute6Siblings(t, true, false)
}

// TestNativeOwnerGuardRoute6SameOwnerSiblings proves refusal is ownership-bound:
// a gatewayless deletion of two real siblings on the same original is useful.
func TestNativeOwnerGuardRoute6SameOwnerSiblings(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeOwnerRoute6Siblings(t, false, true)
}

// TestNativeOwnerGuardRoute4UnsupportedShapes verifies the guarded route parser
// refuses unsupported input before the useful original route changes.
func TestNativeOwnerGuardRoute4UnsupportedShapes(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeOwnerRouteUnsupported(t, unix.AF_INET)
}

// TestNativeOwnerGuardRoute6UnsupportedShapes repeats that question at the IPv6
// family boundary rather than infer coverage from the IPv4 parser.
func TestNativeOwnerGuardRoute6UnsupportedShapes(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeOwnerRouteUnsupported(t, unix.AF_INET6)
}

func nativeOwnerRouteReady(t *testing.T, fixture *nativeOwnerFixture, family int) {
	t.Helper()
	reply := fixture.expected()
	if err := nativeOwnerProbe(fixture.fd, &reply); err != nil {
		t.Fatal(err)
	}
	feature := uint32(1 << 3)
	if family == unix.AF_INET6 {
		feature = 1 << 4
	}
	if reply.features&feature == 0 {
		t.Fatalf("PPP original-file guard ABI lacks route family %d feature %#x before guarded writes", family, feature)
	}
	if err := netlink.LinkSetUp(fixture.link); err != nil {
		t.Fatal(err)
	}
}

func nativeOwnerRoute(t *testing.T, family, index int) *netlink.Route {
	t.Helper()
	prefix := "203.0.113.0/24"
	if family == unix.AF_INET6 {
		prefix = "2001:db8:900::/64"
	}
	_, destination, err := net.ParseCIDR(prefix)
	if err != nil {
		t.Fatal(err)
	}
	return &netlink.Route{LinkIndex: index, Dst: destination, Table: 4300, Priority: 90, Protocol: unix.RTPROT_STATIC, Type: unix.RTN_UNICAST}
}

func nativeOwnerRouteRequest(command, flags, family, fd int, route *netlink.Route) *nl.NetlinkRequest {
	request := nl.NewNetlinkRequest(command, unix.NLM_F_ACK|flags)
	message := nl.NewRtMsg()
	message.Family = uint8(family)
	message.Table = unix.RT_TABLE_UNSPEC
	message.Protocol = unix.RTPROT_STATIC
	prefix, _ := route.Dst.Mask.Size()
	message.Dst_len = uint8(prefix)
	request.AddData(message)
	destination := route.Dst.IP.To4()
	if family == unix.AF_INET6 {
		destination = route.Dst.IP.To16()
	}
	request.AddData(nl.NewRtAttr(unix.RTA_DST, destination))
	request.AddData(nl.NewRtAttr(unix.RTA_TABLE, nl.Uint32Attr(uint32(route.Table))))
	request.AddData(nl.NewRtAttr(unix.RTA_PRIORITY, nl.Uint32Attr(uint32(route.Priority))))
	request.AddData(nl.NewRtAttr(unix.RTA_OIF, nl.Uint32Attr(uint32(route.LinkIndex))))
	if route.Gw != nil {
		gateway := route.Gw.To4()
		if family == unix.AF_INET6 {
			gateway = route.Gw.To16()
		}
		request.AddData(nl.NewRtAttr(unix.RTA_GATEWAY, gateway))
	}
	if route.MTU > 0 {
		metrics := nl.NewRtAttr(unix.RTA_METRICS, nil)
		metrics.AddRtAttr(unix.RTAX_MTU, nl.Uint32Attr(uint32(route.MTU)))
		request.AddData(metrics)
	}
	request.AddData(nl.NewRtAttr(nativeOwnerRouteAttr, nl.Uint32Attr(uint32(int32(fd)))))
	return request
}

func nativeOwnerRoutes(t *testing.T, family int) []netlink.Route {
	t.Helper()
	routes, err := netlink.RouteListFiltered(family, &netlink.Route{Table: 4300}, netlink.RT_FILTER_TABLE)
	if err != nil {
		t.Fatal(err)
	}
	return routes
}

func nativeOwnerRoutesEqual(t *testing.T, family int, before []netlink.Route) {
	t.Helper()
	if after := nativeOwnerRoutes(t, family); !reflect.DeepEqual(after, before) {
		t.Fatalf("refused guarded route write changed full native survivors: before=%+v after=%+v", before, after)
	}
}

func nativeOwnerRouteCase(t *testing.T, family int) {
	t.Helper()
	first := nativeOwnerFixtureNew(t)
	second := nativeOwnerFixtureNew(t)
	nativeOwnerRouteReady(t, first, family)
	route := nativeOwnerRoute(t, family, first.link.Attrs().Index)
	route.MTU = 1400
	add := nativeOwnerRouteRequest(unix.RTM_NEWROUTE, unix.NLM_F_CREATE|unix.NLM_F_EXCL, family, first.fd, route)
	if err := first.send(add); err != nil {
		t.Fatalf("guarded original route add: %v", err)
	}
	before := nativeOwnerRoutes(t, family)
	if len(before) != 1 {
		t.Fatalf("guarded route add installed %d routes, want 1", len(before))
	}
	if before[0].LinkIndex != route.LinkIndex {
		t.Fatal("guarded route selected another native device")
	}
	if before[0].MTU != 1400 {
		t.Fatalf("guarded route MTU=%d, want 1400", before[0].MTU)
	}
	for _, command := range []int{unix.RTM_NEWROUTE, unix.RTM_DELROUTE} {
		flags := 0
		if command == unix.RTM_NEWROUTE {
			flags = unix.NLM_F_CREATE | unix.NLM_F_REPLACE
		}
		request := nativeOwnerRouteRequest(command, flags, family, second.fd, route)
		nativeOwnerErrno(t, first.send(request), unix.ESTALE)
		nativeOwnerRoutesEqual(t, family, before)
	}
	route.MTU = 1300
	replace := nativeOwnerRouteRequest(unix.RTM_NEWROUTE, unix.NLM_F_CREATE|unix.NLM_F_REPLACE, family, first.fd, route)
	if err := first.send(replace); err != nil {
		t.Fatalf("guarded original RouteReplace: %v", err)
	}
	after := nativeOwnerRoutes(t, family)
	if len(after) != 1 {
		t.Fatalf("guarded replacement has %d routes, want 1", len(after))
	}
	if after[0].MTU != 1300 {
		t.Fatalf("guarded original RouteReplace left MTU=%d", after[0].MTU)
	}
	remove := nativeOwnerRouteRequest(unix.RTM_DELROUTE, 0, family, first.fd, route)
	if err := first.send(remove); err != nil {
		t.Fatalf("guarded original route withdrawal: %v", err)
	}
	if len(nativeOwnerRoutes(t, family)) != 0 {
		t.Fatal("guarded original route withdrawal left state")
	}
}

func nativeOwnerRouteForeignReplace(t *testing.T, family int) {
	t.Helper()
	first := nativeOwnerFixtureNew(t)
	second := nativeOwnerFixtureNew(t)
	nativeOwnerRouteReady(t, first, family)
	if err := netlink.LinkSetUp(second.link); err != nil {
		t.Fatal(err)
	}
	foreign := nativeOwnerRoute(t, family, second.link.Attrs().Index)
	foreign.MTU = 1450
	if err := netlink.RouteAdd(foreign); err != nil {
		t.Fatal(err)
	}
	before := nativeOwnerRoutes(t, family)
	if len(before) != 1 {
		t.Fatalf("foreign route control has %d routes, want 1", len(before))
	}
	if before[0].LinkIndex != second.link.Attrs().Index {
		t.Fatal("foreign route control is not on the second unit")
	}
	replacement := nativeOwnerRoute(t, family, first.link.Attrs().Index)
	replacement.MTU = 1350
	request := nativeOwnerRouteRequest(unix.RTM_NEWROUTE, unix.NLM_F_CREATE|unix.NLM_F_REPLACE, family, first.fd, replacement)
	nativeOwnerErrno(t, first.send(request), unix.ESTALE)
	nativeOwnerRoutesEqual(t, family, before)
	if err := netlink.RouteReplace(replacement); err != nil {
		t.Fatalf("unguarded foreign replacement positive control: %v", err)
	}
	after := nativeOwnerRoutes(t, family)
	if len(after) != 1 {
		t.Fatal("unguarded replacement did not leave exactly one route")
	}
	if after[0].LinkIndex != first.link.Attrs().Index {
		t.Fatal("unguarded replacement failed to select the first unit")
	}
	if after[0].MTU != 1350 {
		t.Fatal("unguarded replacement failed to change the route metric")
	}
}

func nativeOwnerRequireLegs(t *testing.T, routes []netlink.Route, first, second int) {
	t.Helper()
	var indices []int
	for index := range routes {
		route := &routes[index]
		if len(route.MultiPath) == 0 {
			indices = append(indices, route.LinkIndex)
		}
		for _, hop := range route.MultiPath {
			indices = append(indices, hop.LinkIndex)
		}
	}
	if len(indices) != 2 {
		t.Fatalf("native shared-route control has %d legs, want exactly two: %+v", len(indices), routes)
	}
	want := []int{first, second}
	slices.Sort(indices)
	slices.Sort(want)
	if !slices.Equal(indices, want) {
		t.Fatalf("actual native legs=%v, want %v", indices, want)
	}
}

func nativeOwnerRoute6Siblings(t *testing.T, qualified, sameOwner bool) {
	t.Helper()
	first := nativeOwnerFixtureNew(t)
	second := nativeOwnerFixtureNew(t)
	nativeOwnerRouteReady(t, first, unix.AF_INET6)
	if err := netlink.LinkSetUp(second.link); err != nil {
		t.Fatal(err)
	}
	for index, item := range []*nativeOwnerFixture{first, second} {
		local := "2001:db8:10::1/64"
		if index == 1 {
			local = "2001:db8:20::1/64"
		}
		address, err := netlink.ParseAddr(local)
		if err != nil {
			t.Fatal(err)
		}
		address.Flags = unix.IFA_F_NODAD
		if err := netlink.AddrAdd(item.link, address); err != nil {
			t.Fatal(err)
		}
	}
	otherIndex := second.link.Attrs().Index
	otherGateway := net.ParseIP("2001:db8:20::2")
	if sameOwner {
		otherIndex = first.link.Attrs().Index
		otherGateway = net.ParseIP("2001:db8:10::3")
	}
	route := nativeOwnerRoute(t, unix.AF_INET6, 0)
	route.MultiPath = []*netlink.NexthopInfo{
		{LinkIndex: first.link.Attrs().Index, Gw: net.ParseIP("2001:db8:10::2")},
		{LinkIndex: otherIndex, Gw: otherGateway},
	}
	if err := netlink.RouteAdd(route); err != nil {
		t.Fatalf("install actual IPv6 sibling control: %v", err)
	}
	before := nativeOwnerRoutes(t, unix.AF_INET6)
	nativeOwnerRequireLegs(t, before, first.link.Attrs().Index, otherIndex)
	remove := nativeOwnerRoute(t, unix.AF_INET6, first.link.Attrs().Index)
	if qualified {
		remove.Gw = net.ParseIP("2001:db8:10::2")
	}
	request := nativeOwnerRouteRequest(unix.RTM_DELROUTE, 0, unix.AF_INET6, first.fd, remove)
	if sameOwner {
		if err := first.send(request); err != nil {
			t.Fatalf("guarded same-original sibling deletion: %v", err)
		}
		if len(nativeOwnerRoutes(t, unix.AF_INET6)) != 0 {
			t.Fatal("same-original sibling deletion retained routes")
		}
		return
	}
	nativeOwnerErrno(t, first.send(request), unix.ESTALE)
	nativeOwnerRoutesEqual(t, unix.AF_INET6, before)
	if err := netlink.RouteDel(remove); err != nil {
		t.Fatalf("unguarded IPv6 sibling deletion control: %v", err)
	}
	after := nativeOwnerRoutes(t, unix.AF_INET6)
	if qualified {
		if len(after) != 1 {
			t.Fatalf("qualified unguarded deletion left %d routes, want one survivor", len(after))
		}
		if after[0].LinkIndex != otherIndex {
			t.Fatal("qualified unguarded deletion removed wrong native sibling")
		}
		return
	}
	if len(after) != 0 {
		t.Fatalf("gatewayless unguarded deletion retained native siblings: %+v", after)
	}
}

func nativeOwnerRouteUnsupported(t *testing.T, family int) {
	t.Helper()
	fixture := nativeOwnerFixtureNew(t)
	nativeOwnerRouteReady(t, fixture, family)
	route := nativeOwnerRoute(t, family, fixture.link.Attrs().Index)
	if err := fixture.send(nativeOwnerRouteRequest(unix.RTM_NEWROUTE, unix.NLM_F_CREATE|unix.NLM_F_EXCL, family, fixture.fd, route)); err != nil {
		t.Fatal(err)
	}
	before := nativeOwnerRoutes(t, family)
	if len(before) != 1 {
		t.Fatal("unsupported-input control did not install one useful route")
	}
	if before[0].LinkIndex != fixture.link.Attrs().Index {
		t.Fatal("unsupported-input control selected another device")
	}
	for _, attr := range []int{nativeOwnerNexthopIDAttr, unix.RTA_MULTIPATH, unix.RTA_ENCAP_TYPE} {
		request := nativeOwnerRouteRequest(unix.RTM_NEWROUTE, unix.NLM_F_CREATE|unix.NLM_F_REPLACE, family, fixture.fd, route)
		request.AddData(nl.NewRtAttr(attr, nl.Uint32Attr(123)))
		nativeOwnerErrno(t, fixture.send(request), unix.EOPNOTSUPP)
		nativeOwnerRoutesEqual(t, family, before)
	}
}
