//go:build integration && linux

// Design: docs/architecture/appliance/kernel-profiles.md -- exact retained address deletion
// Related: native_exact_address_pair_fixture_integration_linux_test.go
// Native0011c only: all prior native tests/helpers remain unchanged.

package kernel_test

import (
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

func TestNativeExactAddressPairProbeAndMissingTuple(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	for _, family := range []int{unix.AF_INET, unix.AF_INET6} {
		t.Run(fmt.Sprint(family), func(t *testing.T) {
			fixture := nativeOwnerFixtureNew(t)
			nativeExactAddrProbe(t, fixture, family)
			pair := nativeExactAddrNew(family, "192.0.2.1", "198.51.100.1", 24)
			if family == unix.AF_INET6 {
				pair = nativeExactAddrNew(family, "2001:db8:1::1", "2001:db8:2::1", 64)
			}
			nativeExactAddrAdd(t, fixture, pair)
			before := nativeExactAddrTuples(t, fixture, family)
			routes := nativeExactAddrRoutes(t, fixture, family)
			for _, fields := range []struct{ local, peer bool }{{true, false}, {false, true}, {false, false}} {
				request := nativeExactAddrRequest(unix.RTM_DELADDR, fixture.fd, fixture.link.Attrs().Index, pair, true, fields.local, fields.peer)
				nativeOwnerErrno(t, fixture.send(request), unix.EINVAL)
				nativeExactAddrNoEffects(t, fixture, family, before, routes)
			}
			bad := pair
			bad.prefix = 33
			if family == unix.AF_INET6 {
				bad.prefix = 129
			}
			nativeOwnerErrno(t, fixture.send(nativeExactAddrRequest(unix.RTM_DELADDR, fixture.fd, fixture.link.Attrs().Index, bad, true, true, true)), unix.EINVAL)
			// An exact duplicate cannot create an ambiguous native population.
			nativeOwnerErrno(t, fixture.send(nativeExactAddrRequest(unix.RTM_NEWADDR, fixture.fd, fixture.link.Attrs().Index, pair, true, true, true)), unix.EEXIST)
			nativeExactAddrNoEffects(t, fixture, family, before, routes)
			nativeExactAddrDelete(t, fixture, fixture.fd, pair)
		})
	}
}

func TestNativeExactAddressPairIPv4WrongPeerSameSubnet(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	nativeExactAddrProbe(t, fixture, unix.AF_INET)
	pair := nativeExactAddrNew(unix.AF_INET, "192.0.2.1", "198.51.100.1", 24)
	nativeExactAddrAdd(t, fixture, pair)
	before := nativeExactAddrTuples(t, fixture, unix.AF_INET)
	routes := nativeExactAddrRoutes(t, fixture, unix.AF_INET)
	stale := pair
	stale.peer = net.ParseIP("198.51.100.99").To4()
	nativeOwnerErrno(t, fixture.send(nativeExactAddrRequest(unix.RTM_DELADDR, fixture.fd, fixture.link.Attrs().Index, stale, true, true, true)), unix.EADDRNOTAVAIL)
	nativeExactAddrNoEffects(t, fixture, unix.AF_INET, before, routes)
	stale = pair
	stale.prefix = 25
	nativeOwnerErrno(t, fixture.send(nativeExactAddrRequest(unix.RTM_DELADDR, fixture.fd, fixture.link.Attrs().Index, stale, true, true, true)), unix.EADDRNOTAVAIL)
	nativeExactAddrNoEffects(t, fixture, unix.AF_INET, before, routes)
	nativeExactAddrDelete(t, fixture, fixture.fd, pair)
}

func TestNativeExactAddressPairIPv6WrongPeer(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	nativeExactAddrProbe(t, fixture, unix.AF_INET6)
	pair := nativeExactAddrNew(unix.AF_INET6, "2001:db8:1::1", "2001:db8:2::1", 64)
	other := nativeExactAddrNew(unix.AF_INET6, "2001:db8:1::2", "2001:db8:2::2", 64)
	nativeExactAddrAdd(t, fixture, pair)
	nativeExactAddrAdd(t, fixture, other)
	nativeExactAddrRequireIPv6Routes(t, fixture, pair)
	nativeExactAddrRequireIPv6Routes(t, fixture, other)
	before := nativeExactAddrTuples(t, fixture, unix.AF_INET6)
	routes := nativeExactAddrRoutes(t, fixture, unix.AF_INET6)
	stale := pair
	stale.peer = net.ParseIP("2001:db8:2::99").To16()
	nativeOwnerErrno(t, fixture.send(nativeExactAddrRequest(unix.RTM_DELADDR, fixture.fd, fixture.link.Attrs().Index, stale, true, true, true)), unix.EADDRNOTAVAIL)
	nativeExactAddrNoEffects(t, fixture, unix.AF_INET6, before, routes)
	stale = pair
	stale.prefix = 63
	nativeOwnerErrno(t, fixture.send(nativeExactAddrRequest(unix.RTM_DELADDR, fixture.fd, fixture.link.Attrs().Index, stale, true, true, true)), unix.EADDRNOTAVAIL)
	nativeExactAddrNoEffects(t, fixture, unix.AF_INET6, before, routes)
	nativeExactAddrRequireIPv6Routes(t, fixture, other)
	nativeExactAddrDelete(t, fixture, fixture.fd, pair)
	nativeExactAddrPresent(t, fixture, other, true)
	nativeExactAddrRequireIPv6Routes(t, fixture, other)
	nativeExactAddrDelete(t, fixture, fixture.fd, other)
}

func TestNativeExactAddressPairIPv4PrimaryPreservesSiblingsAndConfig(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	nativeExactAddrProbe(t, fixture, unix.AF_INET)
	nativeExactAddrUp(t, fixture)
	primary := nativeExactAddrNew(unix.AF_INET, "192.0.2.1", "198.51.100.1", 24)
	second := nativeExactAddrNew(unix.AF_INET, "192.0.2.2", "198.51.100.2", 24)
	third := nativeExactAddrNew(unix.AF_INET, "192.0.2.3", "198.51.100.3", 24)
	unrelated := nativeExactAddrNew(unix.AF_INET, "203.0.113.5", "203.0.114.5", 24)
	for _, pair := range []nativeExactAddrTuple{primary, second, third, unrelated} {
		nativeExactAddrAdd(t, fixture, pair)
	}
	nativeExactAddrPromotionOff(t, fixture)
	before := nativeExactAddrConfig(t, fixture)
	flags := nativeExactAddrTuples(t, fixture, unix.AF_INET)
	if flags[primary.key()]&unix.IFA_F_SECONDARY != 0 || flags[second.key()]&unix.IFA_F_SECONDARY == 0 || flags[third.key()]&unix.IFA_F_SECONDARY == 0 {
		t.Fatalf("fixture lacks actual primary/secondary population: %+v", flags)
	}
	nativeExactAddrRequireRoute(t, fixture, unix.RT_TABLE_LOCAL, unix.RTN_LOCAL, second.local, nil, 32)
	nativeExactAddrRequireRoute(t, fixture, unix.RT_TABLE_LOCAL, unix.RTN_LOCAL, third.local, nil, 32)
	nativeExactAddrRequireRoute(t, fixture, unix.RT_TABLE_LOCAL, unix.RTN_LOCAL, unrelated.local, nil, 32)
	nativeExactAddrRequireRoute(t, fixture, unix.RT_TABLE_MAIN, unix.RTN_UNICAST, net.ParseIP("203.0.114.0"), unrelated.local, 24)
	nativeExactAddrRequireRoute(t, fixture, unix.RT_TABLE_MAIN, unix.RTN_UNICAST, net.ParseIP("198.51.100.0"), primary.local, 24)
	nativeExactAddrDelete(t, fixture, fixture.fd, primary)
	for _, pair := range []nativeExactAddrTuple{second, third, unrelated} {
		nativeExactAddrPresent(t, fixture, pair, true)
	}
	after := nativeExactAddrConfig(t, fixture)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("guarded delete changed actual per-device configuration: before%+v after%+v", before, after)
	}
	flags = nativeExactAddrTuples(t, fixture, unix.AF_INET)
	if flags[second.key()]&unix.IFA_F_SECONDARY != 0 {
		t.Fatalf("native survivor was not promoted: %+v", flags)
	}
	if flags[third.key()]&unix.IFA_F_SECONDARY == 0 {
		t.Fatalf("native later sibling lost secondary role: %+v", flags)
	}
	for _, pair := range []nativeExactAddrTuple{second, third, unrelated} {
		nativeExactAddrRequireRoute(t, fixture, unix.RT_TABLE_LOCAL, unix.RTN_LOCAL, pair.local, nil, 32)
	}
	nativeExactAddrRequireRoute(t, fixture, unix.RT_TABLE_MAIN, unix.RTN_UNICAST, net.ParseIP("198.51.100.0"), second.local, 24)
	nativeExactAddrRequireRoute(t, fixture, unix.RT_TABLE_MAIN, unix.RTN_UNICAST, net.ParseIP("203.0.114.0"), unrelated.local, 24)
	// A secondary exact delete also preserves the now-promoted original.
	nativeExactAddrDelete(t, fixture, fixture.fd, third)
	nativeExactAddrPresent(t, fixture, second, true)
	nativeExactAddrPresent(t, fixture, unrelated, true)
	for _, pair := range []nativeExactAddrTuple{second, unrelated} {
		nativeExactAddrRequireRoute(t, fixture, unix.RT_TABLE_LOCAL, unix.RTN_LOCAL, pair.local, nil, 32)
	}
	nativeExactAddrRequireRoute(t, fixture, unix.RT_TABLE_MAIN, unix.RTN_UNICAST, net.ParseIP("198.51.100.0"), second.local, 24)
	nativeExactAddrRequireRoute(t, fixture, unix.RT_TABLE_MAIN, unix.RTN_UNICAST, net.ParseIP("203.0.114.0"), unrelated.local, 24)
	if now := nativeExactAddrConfig(t, fixture); !reflect.DeepEqual(before, now) {
		t.Fatalf("secondary delete changed configuration: %+v", now)
	}
}

func TestNativeExactAddressPairUnguardedIPv4KeepsLegacyCascade(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	nativeExactAddrProbe(t, fixture, unix.AF_INET)
	nativeExactAddrUp(t, fixture)
	primary := nativeExactAddrNew(unix.AF_INET, "192.0.2.1", "198.51.100.1", 24)
	secondary := nativeExactAddrNew(unix.AF_INET, "192.0.2.2", "198.51.100.2", 24)
	nativeExactAddrAdd(t, fixture, primary)
	nativeExactAddrAdd(t, fixture, secondary)
	nativeExactAddrPromotionOff(t, fixture)
	before := nativeExactAddrConfig(t, fixture)
	if nativeExactAddrTuples(t, fixture, unix.AF_INET)[secondary.key()]&unix.IFA_F_SECONDARY == 0 {
		t.Fatal("legacy control lacks a real secondary")
	}
	staleSubnetPeer := primary
	staleSubnetPeer.peer = net.ParseIP("198.51.100.99").To4()
	request := nativeExactAddrRequest(unix.RTM_DELADDR, -1, fixture.link.Attrs().Index, staleSubnetPeer, false, true, true)
	if err := fixture.send(request); err != nil {
		t.Fatalf("unchanged unguarded subnet selector/cascade: %v", err)
	}
	nativeExactAddrPresent(t, fixture, primary, false)
	nativeExactAddrPresent(t, fixture, secondary, false)
	if now := nativeExactAddrConfig(t, fixture); !reflect.DeepEqual(before, now) {
		t.Fatalf("legacy delete changed configuration: %+v", now)
	}
}

func TestNativeExactAddressPairAllIPv4Prefixes(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	nativeExactAddrProbe(t, fixture, unix.AF_INET)
	for prefix := range 33 {
		t.Run(fmt.Sprint(prefix), func(t *testing.T) {
			pair := nativeExactAddrNew(unix.AF_INET, "192.0.2.10", "198.51.100.10", prefix)
			nativeExactAddrAdd(t, fixture, pair)
			nativeExactAddrDelete(t, fixture, fixture.fd, pair)
		})
	}
}

func TestNativeExactAddressPairAllIPv6Prefixes(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	nativeExactAddrProbe(t, fixture, unix.AF_INET6)
	for prefix := range 129 {
		t.Run(fmt.Sprint(prefix), func(t *testing.T) {
			pair := nativeExactAddrNew(unix.AF_INET6, "2001:db8:1::10", "2001:db8:2::10", prefix)
			nativeExactAddrAdd(t, fixture, pair)
			nativeExactAddrDelete(t, fixture, fixture.fd, pair)
		})
	}
}

func TestNativeExactAddressPairLocalOnlyExplicitTuple(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	for _, family := range []int{unix.AF_INET, unix.AF_INET6} {
		t.Run(fmt.Sprint(family), func(t *testing.T) {
			fixture := nativeOwnerFixtureNew(t)
			nativeExactAddrProbe(t, fixture, family)
			pair := nativeExactAddrNew(family, "192.0.2.8", "192.0.2.8", 24)
			if family == unix.AF_INET6 {
				pair = nativeExactAddrNew(family, "2001:db8:1::8", "2001:db8:1::8", 64)
			}
			nativeExactAddrAdd(t, fixture, pair)
			stale := pair
			stale.peer = net.ParseIP("192.0.2.99").To4()
			if family == unix.AF_INET6 {
				stale.peer = net.ParseIP("2001:db8:1::99").To16()
			}
			before := nativeExactAddrTuples(t, fixture, family)
			routes := nativeExactAddrRoutes(t, fixture, family)
			nativeOwnerErrno(t, fixture.send(nativeExactAddrRequest(unix.RTM_DELADDR, fixture.fd, fixture.link.Attrs().Index, stale, true, true, true)), unix.EADDRNOTAVAIL)
			nativeExactAddrNoEffects(t, fixture, family, before, routes)
			nativeExactAddrDelete(t, fixture, fixture.fd, pair)
		})
	}
}

func TestNativeExactAddressPairOriginalFileRenameAndDuplicate(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	original := nativeOwnerFixtureNew(t)
	nativeExactAddrProbe(t, original, unix.AF_INET)
	oldName := original.link.Attrs().Name
	if err := netlink.LinkSetName(original.link, "ep-original"); err != nil {
		t.Fatal(err)
	}
	victim := nativeOwnerFixtureNew(t)
	nativeExactAddrProbe(t, victim, unix.AF_INET)
	if err := netlink.LinkSetName(victim.link, oldName); err != nil {
		t.Fatal(err)
	}
	pair := nativeExactAddrNew(unix.AF_INET, "192.0.2.9", "198.51.100.9", 24)
	nativeExactAddrAdd(t, original, pair)
	nativeExactAddrAdd(t, victim, pair)
	nativeOwnerErrno(t, original.send(nativeExactAddrRequest(unix.RTM_DELADDR, victim.fd, original.link.Attrs().Index, pair, true, true, true)), unix.ESTALE)
	nativeOwnerErrno(t, victim.send(nativeExactAddrRequest(unix.RTM_DELADDR, original.fd, victim.link.Attrs().Index, pair, true, true, true)), unix.ESTALE)
	nativeExactAddrPresent(t, original, pair, true)
	nativeExactAddrPresent(t, victim, pair, true)
	duplicate, err := unix.FcntlInt(uintptr(original.fd), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer nativeOwnerClose(t, duplicate)
	nativeExactAddrDelete(t, original, duplicate, pair)
	nativeExactAddrPresent(t, victim, pair, true)
	nativeExactAddrDelete(t, victim, victim.fd, pair)
}

func TestNativeExactAddressPairUnguardedIPv6KeepsLocalSelector(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	nativeExactAddrProbe(t, fixture, unix.AF_INET6)
	pair := nativeExactAddrNew(unix.AF_INET6, "2001:db8:1::1", "2001:db8:2::1", 64)
	sibling := nativeExactAddrNew(unix.AF_INET6, "2001:db8:1::2", "2001:db8:2::2", 64)
	nativeExactAddrAdd(t, fixture, pair)
	nativeExactAddrAdd(t, fixture, sibling)
	nativeExactAddrRequireIPv6Routes(t, fixture, pair)
	nativeExactAddrRequireIPv6Routes(t, fixture, sibling)
	stale := pair
	stale.peer = net.ParseIP("2001:db8:2::99").To16()
	request := nativeExactAddrRequest(unix.RTM_DELADDR, -1, fixture.link.Attrs().Index, stale, false, true, true)
	if err := fixture.send(request); err != nil {
		t.Fatalf("unchanged unguarded IPv6 local selector: %v", err)
	}
	nativeExactAddrPresent(t, fixture, pair, false)
	nativeExactAddrPresent(t, fixture, sibling, true)
	nativeExactAddrRequireIPv6Routes(t, fixture, sibling)
	nativeExactAddrDelete(t, fixture, fixture.fd, sibling)
}
