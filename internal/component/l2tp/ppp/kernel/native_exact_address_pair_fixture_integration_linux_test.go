//go:build integration && linux

// Design: docs/architecture/appliance/kernel-profiles.md -- exact original address pairs
// Related: native_owner_guard_integration_linux_test.go -- actual creator/unit/ACK fixture
// Test-local0011c UAPI with complete Source13 masks. No production provider or name/procfs configuration write.

package kernel_test

import (
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"slices"
	"testing"
	"time"
	"unsafe"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

const nativeExactAddrPairBit = 1 << 10
const nativeExactAddrControlBit = 1 << 9
const nativeExactAddrUnitMask = 1791
const nativeExactAddrGlobalMask = 2047

type nativeExactAddrTuple struct {
	family int
	local  net.IP
	peer   net.IP
	prefix int
}

type nativeExactAddrSupport struct {
	abi       uint32
	features  uint32
	reserved0 uint32
	reserved1 uint32
}

func nativeExactAddrNew(family int, local, peer string, prefix int) nativeExactAddrTuple {
	left, right := net.ParseIP(local), net.ParseIP(peer)
	if family == unix.AF_INET {
		left, right = left.To4(), right.To4()
	} else {
		left, right = left.To16(), right.To16()
	}
	return nativeExactAddrTuple{family: family, local: left, peer: right, prefix: prefix}
}

func (tuple nativeExactAddrTuple) key() string {
	return fmt.Sprintf("%s|%s/%d", tuple.local, tuple.peer, tuple.prefix)
}

func nativeExactAddrProbe(t *testing.T, fixture *nativeOwnerFixture, family int) {
	t.Helper()
	familyBit := uint32(nativeOwnerAddr4)
	if family == unix.AF_INET6 {
		familyBit = nativeOwnerAddr6
	}
	want := familyBit | nativeExactAddrPairBit
	unit := fixture.expected()
	if err := nativeOwnerProbe(fixture.fd, &unit); err != nil {
		t.Fatal(err)
	}
	if unit.features&want != want {
		t.Fatalf("unit51 lacks exact-pair plus family receipt: got%#x, need%#x", unit.features, want)
	}
	if unit.features&nativeExactAddrControlBit == 0 {
		t.Fatal("unit51 lacks complete control-socket capability bit9")
	}
	if unit.features != nativeExactAddrUnitMask {
		t.Fatalf("unit51 mask=%#x, want current full mask%#x", unit.features, nativeExactAddrUnitMask)
	}
	fd, err := unix.Open("/dev/ppp", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer nativeOwnerClose(t, fd)
	support := nativeExactAddrSupport{abi: nativeOwnerABI}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), 0xc010742a, uintptr(unsafe.Pointer(&support))) //nolint:gosec // frozen global42 ABI.
	if errno != 0 {
		t.Fatalf("positive global42 exact-pair declaration: %v", errno)
	}
	if support.abi != nativeOwnerABI || support.reserved0 != 0 || support.reserved1 != 0 || support.features&want != want {
		t.Fatalf("global42 lacks exact-pair plus family declaration: %+v, need%#x", support, want)
	}
	if support.features&nativeExactAddrControlBit == 0 {
		t.Fatal("global42 lacks complete control-socket capability bit9")
	}
	if support.features != nativeExactAddrGlobalMask {
		t.Fatalf("global42 mask=%#x, want current full mask%#x", support.features, nativeExactAddrGlobalMask)
	}
	if family == unix.AF_INET6 {
		nativeExactAddrUp(t, fixture)
	}
}

func nativeExactAddrRequest(command, fd, index int, tuple nativeExactAddrTuple, guarded, includeLocal, includePeer bool) *nl.NetlinkRequest {
	flags := unix.NLM_F_ACK
	if command == unix.RTM_NEWADDR {
		flags |= unix.NLM_F_CREATE | unix.NLM_F_EXCL
	}
	request := nl.NewNetlinkRequest(command, flags)
	message := &nl.IfAddrmsg{Family: uint8(tuple.family), Prefixlen: uint8(tuple.prefix), Index: uint32(index)}
	if tuple.family == unix.AF_INET6 {
		message.Flags = unix.IFA_F_NODAD
	}
	request.AddData(message)
	if includeLocal {
		request.AddData(nl.NewRtAttr(unix.IFA_LOCAL, tuple.local))
	}
	if includePeer {
		request.AddData(nl.NewRtAttr(unix.IFA_ADDRESS, tuple.peer))
	}
	if guarded {
		request.AddData(nl.NewRtAttr(nativeOwnerAddrAttr, nl.Uint32Attr(uint32(int32(fd)))))
	}
	return request
}

func nativeExactAddrAdd(t *testing.T, fixture *nativeOwnerFixture, tuple nativeExactAddrTuple) {
	t.Helper()
	if err := fixture.send(nativeExactAddrRequest(unix.RTM_NEWADDR, fixture.fd, fixture.link.Attrs().Index, tuple, true, true, true)); err != nil {
		t.Fatalf("actual exact pair add %s: %v", tuple.key(), err)
	}
	nativeExactAddrPresent(t, fixture, tuple, true)
	if tuple.family == unix.AF_INET6 {
		nativeExactAddrWaitIPv6Routes(t, fixture, tuple)
	}
}

func nativeExactAddrDelete(t *testing.T, fixture *nativeOwnerFixture, fd int, tuple nativeExactAddrTuple) {
	t.Helper()
	if err := fixture.send(nativeExactAddrRequest(unix.RTM_DELADDR, fd, fixture.link.Attrs().Index, tuple, true, true, true)); err != nil {
		t.Fatalf("actual exact pair delete %s: %v", tuple.key(), err)
	}
	nativeExactAddrPresent(t, fixture, tuple, false)
}

func nativeExactAddrTuples(t *testing.T, fixture *nativeOwnerFixture, family int) map[string]int {
	t.Helper()
	addresses, err := netlink.AddrList(fixture.link, family)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]int, len(addresses))
	for _, address := range addresses {
		local := address.IP
		peer := local
		prefix, _ := address.Mask.Size()
		if address.Peer != nil {
			peer = address.Peer.IP
			// netlinkv1.3.1 parseAddr places a P2P mask on Peer, and
			// deliberately represents LOCAL with a host-length mask.
			prefix, _ = address.Peer.Mask.Size()
		}
		result[nativeExactAddrTuple{family: family, local: local, peer: peer, prefix: prefix}.key()] = address.Flags
	}
	return result
}

func nativeExactAddrPresent(t *testing.T, fixture *nativeOwnerFixture, tuple nativeExactAddrTuple, present bool) {
	t.Helper()
	got := nativeExactAddrTuples(t, fixture, tuple.family)
	_, exists := got[tuple.key()]
	if exists != present {
		t.Fatalf("native tuple %s present%t,want%t; all=%+v", tuple.key(), exists, present, got)
	}
}

func nativeExactAddrRoutes(t *testing.T, fixture *nativeOwnerFixture, family int) []string {
	t.Helper()
	routes, err := netlink.RouteListFiltered(family, &netlink.Route{Table: unix.RT_TABLE_UNSPEC, LinkIndex: fixture.link.Attrs().Index}, netlink.RT_FILTER_TABLE|netlink.RT_FILTER_OIF)
	if err != nil {
		t.Fatal(err)
	}
	result := make([]string, 0, len(routes))
	for index := range routes {
		route := &routes[index]
		result = append(result, fmt.Sprintf("%d|%d|%d|%d|%d|%s|%s|%s", route.Table, route.Type, route.Scope, route.Priority, route.LinkIndex, route.Dst, route.Src, route.Gw))
	}
	slices.Sort(result)
	return result
}

func nativeExactAddrNoEffects(t *testing.T, fixture *nativeOwnerFixture, family int, before map[string]int, routes []string) {
	t.Helper()
	if after := nativeExactAddrTuples(t, fixture, family); !reflect.DeepEqual(before, after) {
		t.Fatalf("refused tuple changed actual addresses: before%+v after%+v", before, after)
	}
	if after := nativeExactAddrRoutes(t, fixture, family); !reflect.DeepEqual(routes, after) {
		t.Fatalf("refused tuple changed actual FIB: before%+v after%+v", routes, after)
	}
}

func nativeExactAddrUp(t *testing.T, fixture *nativeOwnerFixture) {
	t.Helper()
	request := nl.NewNetlinkRequest(unix.RTM_SETLINK, unix.NLM_F_ACK)
	message := nl.NewIfInfomsg(unix.AF_UNSPEC)
	message.Index = int32(fixture.link.Attrs().Index)
	message.Change = unix.IFF_UP
	message.Flags = unix.IFF_UP
	request.AddData(message)
	request.AddData(nl.NewRtAttr(nativeOwnerLinkAttr, nl.Uint32Attr(uint32(fixture.fd))))
	if err := fixture.send(request); err != nil {
		t.Fatalf("original-file guarded link-up: %v", err)
	}
}

// Configuration is set ONCE on this test's exclusive freshly created device,
// by native index before the operation under test. No global/default cnf,
// procfs/name write, temporary toggle or later restoration is involved.
func nativeExactAddrPromotionOff(t *testing.T, fixture *nativeOwnerFixture) {
	t.Helper()
	request := nl.NewNetlinkRequest(unix.RTM_SETLINK, unix.NLM_F_ACK)
	message := nl.NewIfInfomsg(unix.AF_UNSPEC)
	message.Index = int32(fixture.link.Attrs().Index)
	request.AddData(message)
	spec := nl.NewRtAttr(unix.IFLA_AF_SPEC|unix.NLA_F_NESTED, nil)
	family := spec.AddRtAttr(unix.AF_INET|unix.NLA_F_NESTED, nil)
	conf := family.AddRtAttr(1|unix.NLA_F_NESTED, nil) // pinned IFLA_INET_CONF=1.
	conf.AddRtAttr(20, nl.Uint32Attr(0))               // pinned IPV4_DEVCONF_PROMOTE_SECONDARIES=20.
	request.AddData(spec)
	if err := fixture.send(request); err != nil {
		t.Fatalf("owned native fixture promotion setting: %v", err)
	}
	if values := nativeExactAddrConfig(t, fixture); values[19] != 0 {
		t.Fatalf("actual per-device promote_secondaries=%d,want0", values[19])
	}
}

// Read the actual retained index's IFLA_AF_SPEC/AF_INET/IFLA_INET_CONF array.
// Linuxv7.3 inet_fill_link_af emits native u32 values; none is changed here.
func nativeExactAddrConfig(t *testing.T, fixture *nativeOwnerFixture) []uint32 {
	t.Helper()
	request := nl.NewNetlinkRequest(unix.RTM_GETLINK, 0)
	message := nl.NewIfInfomsg(unix.AF_UNSPEC)
	message.Index = int32(fixture.link.Attrs().Index)
	request.AddData(message)
	messages, err := request.Execute(unix.NETLINK_ROUTE, unix.RTM_NEWLINK)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range messages {
		if len(data) < unix.SizeofIfInfomsg {
			t.Fatal("short native GETLINK message")
		}
		info := nl.DeserializeIfInfomsg(data)
		if int(info.Index) != fixture.link.Attrs().Index {
			continue
		}
		attrs, err := nl.ParseRouteAttr(data[info.Len():])
		if err != nil {
			t.Fatal(err)
		}
		for _, attr := range attrs {
			if attr.Attr.Type&0x3fff != unix.IFLA_AF_SPEC {
				continue
			}
			families, err := nl.ParseRouteAttr(attr.Value)
			if err != nil {
				t.Fatal(err)
			}
			for _, af := range families {
				if af.Attr.Type&0x3fff != unix.AF_INET {
					continue
				}
				entries, err := nl.ParseRouteAttr(af.Value)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if entry.Attr.Type&0x3fff != 1 {
						continue
					}
					if len(entry.Value) < 80 || len(entry.Value)%4 != 0 {
						t.Fatalf("invalid native IPv4 cnf array length%d", len(entry.Value))
					}
					result := make([]uint32, len(entry.Value)/4)
					for i := range result {
						result[i] = binary.NativeEndian.Uint32(entry.Value[i*4 : i*4+4])
					}
					return result
				}
			}
		}
	}
	t.Fatal("actual device has no native IPv4 configuration array")
	return nil
}

func nativeExactAddrRequireRoute(t *testing.T, fixture *nativeOwnerFixture, table, kind int, destination, source net.IP, prefix int) {
	t.Helper()
	routes, err := netlink.RouteListFiltered(unix.AF_INET, &netlink.Route{Table: table, LinkIndex: fixture.link.Attrs().Index}, netlink.RT_FILTER_TABLE|netlink.RT_FILTER_OIF)
	if err != nil {
		t.Fatal(err)
	}
	for index := range routes {
		route := &routes[index]
		if route.Dst == nil || route.Type != kind {
			continue
		}
		size, bits := route.Dst.Mask.Size()
		if bits != 32 || size != prefix || !route.Dst.IP.Equal(destination) {
			continue
		}
		if source != nil && !route.Src.Equal(source) {
			continue
		}
		return
	}
	t.Fatalf("surviving exact pair lost native route table%d/type%d/%s/%d/src%s: %+v", table, kind, destination, prefix, source, routes)
}

// NODAD still schedules addrconf_dad_work. Wait for the actual local and peer
// route publication before callers freeze refusal snapshots or delete a pair.
// The connected prefix route is also required. Success is native state only;
// time and attempt limits are failure bounds, never evidence of settlement.
func nativeExactAddrWaitIPv6Routes(t *testing.T, fixture *nativeOwnerFixture, tuple nativeExactAddrTuple) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var last []netlink.Route
	for range 300 {
		last = nativeExactAddrIPv6RouteList(t, fixture)
		if nativeExactAddrIPv6RoutesReady(last, tuple) {
			return
		}
		if !time.Now().Before(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("native IPv6 local/peer/connected routes did not settle for %s: %+v", tuple.key(), last)
}

func nativeExactAddrIPv6RouteList(t *testing.T, fixture *nativeOwnerFixture) []netlink.Route {
	t.Helper()
	routes, err := netlink.RouteListFiltered(unix.AF_INET6, &netlink.Route{Table: unix.RT_TABLE_UNSPEC, LinkIndex: fixture.link.Attrs().Index}, netlink.RT_FILTER_TABLE|netlink.RT_FILTER_OIF)
	if err != nil {
		t.Fatal(err)
	}
	return routes
}

func nativeExactAddrIPv6RouteMatches(route *netlink.Route, table, kind int, address net.IP, prefix int) bool {
	if route.Table != table || route.Type != kind {
		return false
	}
	if route.Dst == nil {
		return prefix == 0
	}
	size, bits := route.Dst.Mask.Size()
	return bits == 128 && size == prefix && route.Dst.IP.Equal(address)
}

func nativeExactAddrIPv6RoutesReady(routes []netlink.Route, tuple nativeExactAddrTuple) bool {
	local, connected := false, false
	// A local-only tuple stores no distinct peer route. Its local and
	// connected routes are still required, including a /128 connected route.
	peer := tuple.local.Equal(tuple.peer)
	network := tuple.local.Mask(net.CIDRMask(tuple.prefix, 128))
	for index := range routes {
		route := &routes[index]
		if nativeExactAddrIPv6RouteMatches(route, unix.RT_TABLE_LOCAL, unix.RTN_LOCAL, tuple.local, 128) {
			local = true
		}
		if nativeExactAddrIPv6RouteMatches(route, unix.RT_TABLE_MAIN, unix.RTN_UNICAST, network, tuple.prefix) {
			connected = true
		}
		if !peer && nativeExactAddrIPv6RouteMatches(route, unix.RT_TABLE_MAIN, unix.RTN_UNICAST, tuple.peer, 128) {
			peer = true
		}
	}
	return local && peer && connected
}

// Post-delete survival is an immediate assertion on the exact previously
// published sibling routes. It does not wait for a lost route to reappear.
func nativeExactAddrRequireIPv6Routes(t *testing.T, fixture *nativeOwnerFixture, tuple nativeExactAddrTuple) {
	t.Helper()
	routes := nativeExactAddrIPv6RouteList(t, fixture)
	if !nativeExactAddrIPv6RoutesReady(routes, tuple) {
		t.Fatalf("exact sibling lost local/peer/connected IPv6 routes for %s: %+v", tuple.key(), routes)
	}
}
