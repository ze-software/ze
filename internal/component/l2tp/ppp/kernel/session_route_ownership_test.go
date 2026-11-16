// Design: docs/research/l2tpv2-ze-integration.md -- exact kernel PPP route ownership

package kernel

import (
	"errors"
	"fmt"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/rtproto"
)

type routeOwnershipTuple struct {
	name        string
	destination string
	gateway     string
	metric      int
	protocol    rtproto.Proto
}

type routeOwnershipBackend struct {
	sessionTestBackend
	routes    map[routeOwnershipTuple]bool
	adds      []routeOwnershipTuple
	removes   []routeOwnershipTuple
	addErr    error
	removeErr error
	checkLock func()
}

func (b *routeOwnershipBackend) AddRoute(name, destination, gateway string, metric int, protocol rtproto.Proto) error {
	if b.checkLock != nil {
		b.checkLock()
	}
	// The session must send its canonical network. The vendored parser keeps
	// host bits; gateway serialization alone normalizes IPv4-mapped addresses.
	route := routeOwnershipTuple{name, destination, gateway, metric, protocol}
	b.adds = append(b.adds, route)
	route.gateway = netip.MustParseAddr(gateway).Unmap().String()
	b.routes[route] = true
	return b.addErr
}

func (b *routeOwnershipBackend) RemoveRoute(name, destination, gateway string, metric int, protocol rtproto.Proto) error {
	if b.checkLock != nil {
		b.checkLock()
	}
	route := routeOwnershipTuple{name, destination, gateway, metric, protocol}
	b.removes = append(b.removes, route)
	if b.removeErr != nil {
		return b.removeErr
	}
	route.gateway = netip.MustParseAddr(gateway).Unmap().String()
	delete(b.routes, route)
	return nil
}

func newRouteOwnershipSession(t *testing.T) (ppp.SessionHandle, *routeOwnershipBackend) {
	t.Helper()
	streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
	var released []int
	backend := &routeOwnershipBackend{routes: make(map[routeOwnershipTuple]bool)}
	cfg := sessionTestConfig()
	cfg.InterfaceName = "created-ppp"
	cfg.Backend = backend
	_, handle, err := newSession(cfg, sessionTestOps(streams, &released))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	})
	return handle, backend
}

// A withdrawal must leave foreign tuples untouched, then remove only the exact
// installed tuple once. The backend models deletion, not merely a call count.
func TestKernelSessionRouteWithdrawalRequiresExactReceipt(t *testing.T) {
	handle, backend := newRouteOwnershipSession(t)
	destination := netip.MustParsePrefix("2001:db8:1::/64")
	gateway := netip.MustParseAddr("fe80::2")
	owned := routeOwnershipTuple{"created-ppp", destination.String(), gateway.String(), 7, rtproto.Static}
	foreign := []routeOwnershipTuple{
		{"created-ppp", "2001:db8:2::/64", gateway.String(), 7, rtproto.Static},
		{"created-ppp", destination.String(), "fe80::3", 7, rtproto.Static},
		{"created-ppp", destination.String(), gateway.String(), 8, rtproto.Static},
		{"created-ppp", destination.String(), gateway.String(), 7, rtproto.FIBKernel},
	}
	for _, route := range foreign {
		backend.routes[route] = true
		if err := handle.RemoveRoute(netip.MustParsePrefix(route.destination), netip.MustParseAddr(route.gateway), route.metric, route.protocol); err != nil {
			t.Fatal(err)
		}
	}
	if len(backend.removes) != 0 {
		t.Fatalf("unattempted route withdrawal reached native backend: %v", backend.removes)
	}
	if err := handle.AddRoute(destination, gateway, owned.metric, owned.protocol); err != nil {
		t.Fatal(err)
	}
	for _, route := range foreign {
		if err := handle.RemoveRoute(netip.MustParsePrefix(route.destination), netip.MustParseAddr(route.gateway), route.metric, route.protocol); err != nil {
			t.Fatal(err)
		}
		if !backend.routes[route] {
			t.Fatalf("foreign route removed: %+v", route)
		}
	}
	for range 2 {
		if err := handle.RemoveRoute(destination, gateway, owned.metric, owned.protocol); err != nil {
			t.Fatal(err)
		}
	}
	if backend.routes[owned] {
		t.Fatal("owned route survived successful withdrawal")
	}
	if len(backend.removes) != 1 {
		t.Fatalf("withdrawals = %v, want the owned tuple once", backend.removes)
	}
	if backend.removes[0] != owned {
		t.Fatalf("withdrawn route = %+v, want %+v", backend.removes[0], owned)
	}
}

// An attempted add can install state before returning an error. Failed cleanup
// must preserve its error and exact receipt until a later successful withdrawal.
func TestKernelSessionRouteUnknownAddRetainsCleanupReceipt(t *testing.T) {
	handle, backend := newRouteOwnershipSession(t)
	destination := netip.MustParsePrefix("2001:db8:1::/64")
	gateway := netip.MustParseAddr("fe80::2")
	addErr := errors.New("route add outcome unknown")
	removeErr := errors.New("route withdrawal outcome unknown")
	backend.addErr = addErr
	if err := handle.AddRoute(destination, gateway, 7, rtproto.Static); !errors.Is(err, addErr) {
		t.Fatalf("add failure lost: %v", err)
	}
	backend.removeErr = removeErr
	if err := handle.RemoveRoute(destination, gateway, 7, rtproto.Static); !errors.Is(err, removeErr) {
		t.Fatalf("withdrawal failure lost: %v", err)
	}
	if len(backend.routes) != 1 {
		t.Fatal("failed withdrawal lost native route")
	}
	backend.removeErr = nil
	for range 2 {
		if err := handle.RemoveRoute(destination, gateway, 7, rtproto.Static); err != nil {
			t.Fatal(err)
		}
	}
	if len(backend.routes) != 0 {
		t.Fatal("retry did not remove uncertain route")
	}
	if len(backend.removes) != 2 {
		t.Fatalf("withdrawals = %d, want failed attempt then successful retry", len(backend.removes))
	}
}

// Native route calls must retain the lease mutex, so Close cannot recycle the
// unit while an add or withdrawal is using its created interface identity.
func TestKernelSessionRouteOperationsHoldOwnerLock(t *testing.T) {
	handle, backend := newRouteOwnershipSession(t)
	owner, ok := handle.(*session)
	if !ok {
		t.Fatalf("kernel constructor returned %T, want *session", handle)
	}
	checks := 0
	backend.checkLock = func() {
		checks++
		if owner.mu.TryLock() {
			owner.mu.Unlock()
			t.Error("native route operation ran without the owner lease lock")
		}
	}
	destination := netip.MustParsePrefix("2001:db8:1::/64")
	gateway := netip.MustParseAddr("fe80::2")
	if err := handle.AddRoute(destination, gateway, 7, rtproto.Static); err != nil {
		t.Fatal(err)
	}
	if err := handle.RemoveRoute(destination, gateway, 7, rtproto.Static); err != nil {
		t.Fatal(err)
	}
	if checks != 2 {
		t.Fatalf("native lock checks = %d, want add and withdrawal", checks)
	}
}

// Refused input must acquire no receipt: cleanup of that same tuple must leave
// another producer's native state untouched rather than invent deletion rights.
func TestKernelSessionRouteRefusedAddAcquiresNoReceipt(t *testing.T) {
	handle, backend := newRouteOwnershipSession(t)
	destination := netip.MustParsePrefix("2001:db8:1::/64")
	gateway := netip.MustParseAddr("fe80::2")
	if err := handle.AddRoute(destination, netip.Addr{}, 7, rtproto.Static); err == nil {
		t.Fatal("invalid gateway accepted")
	}
	foreign := routeOwnershipTuple{"created-ppp", destination.String(), gateway.String(), 7, rtproto.Static}
	backend.routes[foreign] = true
	if err := handle.RemoveRoute(destination, gateway, 7, rtproto.Static); err != nil {
		t.Fatal(err)
	}
	if len(backend.adds) != 0 {
		t.Fatal("refused add reached native backend")
	}
	if len(backend.removes) != 0 {
		t.Fatal("refused add granted deletion rights")
	}
	if !backend.routes[foreign] {
		t.Fatal("foreign route removed after refused add")
	}
}

// The route owner permits 1024 retained paths, matching the bounded session
// policy. Refusal must not reach native I/O or acquire a receipt; withdrawing
// one owned path must free a slot for the previously refused path.
func TestKernelSessionRouteCapacityReleasesOnlyOwnedSlots(t *testing.T) {
	const retainedPathsMax = 1024
	handle, backend := newRouteOwnershipSession(t)
	gateway := netip.MustParseAddr("192.0.2.1")
	for index := range retainedPathsMax {
		address := netip.AddrFrom4([4]byte{10, 0, byte(index >> 8), byte(index)})
		if err := handle.AddRoute(netip.PrefixFrom(address, 32), gateway, 7, rtproto.Static); err != nil {
			t.Fatalf("route %d refused before capacity: %v", index, err)
		}
	}
	destination := netip.MustParsePrefix("198.51.100.0/24")
	foreign := routeOwnershipTuple{"created-ppp", destination.String(), gateway.String(), 7, rtproto.Static}
	backend.routes[foreign] = true
	if err := handle.AddRoute(destination, gateway, 7, rtproto.Static); err == nil {
		t.Fatal("route ownership limit accepted another path")
	}
	if len(backend.adds) != retainedPathsMax {
		t.Fatal("capacity refusal reached native backend")
	}
	if err := handle.RemoveRoute(destination, gateway, 7, rtproto.Static); err != nil {
		t.Fatal(err)
	}
	if len(backend.removes) != 0 {
		t.Fatal("capacity refusal acquired deletion rights")
	}
	if !backend.routes[foreign] {
		t.Fatal("capacity refusal deleted foreign route")
	}
	first := netip.MustParsePrefix("10.0.0.0/32")
	if err := handle.AddRoute(first, gateway, 7, rtproto.Static); err != nil {
		t.Fatalf("existing receipt refused at capacity: %v", err)
	}
	if err := handle.RemoveRoute(first, gateway, 7, rtproto.Static); err != nil {
		t.Fatal(err)
	}
	if err := handle.AddRoute(destination, gateway, 7, rtproto.Static); err != nil {
		t.Fatalf("withdrawal failed to release its slot: %v", err)
	}
	if err := handle.RemoveRoute(destination, gateway, 7, rtproto.Static); err != nil {
		t.Fatal(err)
	}
	if backend.routes[foreign] {
		t.Fatal("newly owned path survived withdrawal")
	}
}

// The session must send one canonical network for native prefix aliases and
// retain one receipt and capacity slot; the backend preserves request host bits.
func TestKernelSessionRoutePrefixAliasesShareReceipt(t *testing.T) {
	const retainedPathsMax = 1024
	handle, backend := newRouteOwnershipSession(t)
	gateway := netip.MustParseAddr("192.0.2.1")
	network := netip.MustParsePrefix("198.51.100.0/24")
	alias := netip.MustParsePrefix("198.51.100.1/24")
	for _, destination := range []netip.Prefix{network, alias} {
		if err := handle.AddRoute(destination, gateway, 7, rtproto.Static); err != nil {
			t.Fatal(err)
		}
	}
	if len(backend.routes) != 1 {
		t.Fatalf("native prefix aliases produced %d paths, want one", len(backend.routes))
	}
	for index := range retainedPathsMax - 1 {
		address := netip.AddrFrom4([4]byte{10, 0, byte(index >> 8), byte(index)})
		if err := handle.AddRoute(netip.PrefixFrom(address, 32), gateway, 7, rtproto.Static); err != nil {
			t.Fatalf("alias consumed another ownership slot at %d: %v", index, err)
		}
	}
	overflow := netip.MustParsePrefix("203.0.113.0/24")
	if err := handle.AddRoute(overflow, gateway, 7, rtproto.Static); err == nil {
		t.Fatal("alias test failed to reach ownership capacity")
	}
	if err := handle.RemoveRoute(alias, gateway, 7, rtproto.Static); err != nil {
		t.Fatal(err)
	}
	if err := handle.RemoveRoute(network, gateway, 7, rtproto.Static); err != nil {
		t.Fatal(err)
	}
	if len(backend.removes) != 1 {
		t.Fatalf("prefix aliases retained separate receipts: %v", backend.removes)
	}
	if backend.removes[0].destination != network.String() {
		t.Fatalf("native withdrawal destination = %q, want canonical %q", backend.removes[0].destination, network.String())
	}
	canonical := routeOwnershipTuple{"created-ppp", network.String(), gateway.String(), 7, rtproto.Static}
	if backend.routes[canonical] {
		t.Fatal("canonical native route survived alias withdrawal")
	}
	if err := handle.AddRoute(overflow, gateway, 7, rtproto.Static); err != nil {
		t.Fatalf("canonical withdrawal failed to free the aliased slot: %v", err)
	}
}

// The backend refuses Any as a producer before any native write. The session
// must refuse it before creating a receipt that could permit wildcard deletion.
func TestKernelSessionRouteWildcardProducerAcquiresNoReceipt(t *testing.T) {
	handle, backend := newRouteOwnershipSession(t)
	destination := netip.MustParsePrefix("198.51.100.0/24")
	gateway := netip.MustParseAddr("192.0.2.1")
	foreign := routeOwnershipTuple{"created-ppp", destination.String(), gateway.String(), 7, rtproto.Static}
	backend.routes[foreign] = true
	if err := handle.AddRoute(destination, gateway, 7, rtproto.Any); err == nil {
		t.Fatal("wildcard route protocol accepted as a producer")
	}
	if len(backend.adds) != 0 {
		t.Fatal("wildcard producer reached backend instead of being refused before a receipt")
	}
	if err := handle.RemoveRoute(destination, gateway, 7, rtproto.Any); err != nil {
		t.Fatal(err)
	}
	if len(backend.removes) != 0 {
		t.Fatal("refused wildcard producer acquired native deletion rights")
	}
	if !backend.routes[foreign] {
		t.Fatal("wildcard cleanup removed foreign route")
	}
	control := netip.MustParsePrefix("203.0.113.0/24")
	if err := handle.AddRoute(control, gateway, 7, rtproto.Iface); err != nil {
		t.Fatalf("concrete interface producer refused: %v", err)
	}
	if err := handle.RemoveRoute(control, gateway, 7, rtproto.Iface); err != nil {
		t.Fatalf("concrete interface producer could not withdraw its path: %v", err)
	}
	if len(backend.adds) != 1 {
		t.Fatal("concrete interface producer did not reach native add")
	}
	if len(backend.removes) != 1 {
		t.Fatal("concrete interface producer did not reach native withdrawal")
	}
	if !backend.routes[foreign] {
		t.Fatal("concrete interface producer removed the foreign static path")
	}
}

// IPv4-mapped and plain gateways have identical native bytes. Both calls must
// send the canonical gateway and share one retained receipt and capacity slot.
func TestKernelSessionRouteMappedGatewayAliasesShareReceipt(t *testing.T) {
	const retainedPathsMax = 1024
	handle, backend := newRouteOwnershipSession(t)
	destination := netip.MustParsePrefix("198.51.100.0/24")
	gateway := netip.MustParseAddr("192.0.2.1")
	mapped := netip.MustParseAddr("::ffff:192.0.2.1")
	for _, peer := range []netip.Addr{gateway, mapped} {
		if err := handle.AddRoute(destination, peer, 0, rtproto.Iface); err != nil {
			t.Fatal(err)
		}
	}
	for _, added := range backend.adds {
		if added.gateway != gateway.String() {
			t.Fatalf("native add gateway = %q, want canonical %q", added.gateway, gateway.String())
		}
	}
	if len(backend.routes) != 1 {
		t.Fatalf("gateway aliases produced %d native paths, want one", len(backend.routes))
	}
	for index := range retainedPathsMax - 1 {
		address := netip.AddrFrom4([4]byte{10, 0, byte(index >> 8), byte(index)})
		if err := handle.AddRoute(netip.PrefixFrom(address, 32), gateway, 0, rtproto.Iface); err != nil {
			t.Fatalf("gateway alias consumed a second slot at %d: %v", index, err)
		}
	}
	if err := handle.AddRoute(netip.MustParsePrefix("203.0.113.0/24"), gateway, 0, rtproto.Iface); err == nil {
		t.Fatal("gateway alias test failed to reach ownership capacity")
	}
	if err := handle.RemoveRoute(destination, mapped, 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	if err := handle.RemoveRoute(destination, gateway, 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	if len(backend.removes) != 1 {
		t.Fatalf("gateway aliases retained separate receipts: %v", backend.removes)
	}
	if backend.removes[0].gateway != gateway.String() {
		t.Fatalf("native withdrawal gateway = %q, want canonical %q", backend.removes[0].gateway, gateway.String())
	}
}

// Native priority and protocol widths must not truncate distinct input into a
// valid or wildcard route. Refusals acquire no receipt and perform no native I/O.
func TestKernelSessionRouteNativeWidthsRefusedBeforeReceipt(t *testing.T) {
	cases := []struct {
		name     string
		metric   int
		protocol rtproto.Proto
	}{
		{"negative metric", -1, rtproto.Iface},
		{"negative protocol", 0, rtproto.Proto(-1)},
		{"protocol truncates to wildcard", 0, rtproto.Proto(256)},
		{"protocol truncates to interface", 0, rtproto.Proto(256) + rtproto.Iface},
	}
	const nativePriorityMax = uint64(1<<32 - 1)
	intMax := int(^uint(0) >> 1)
	if uint64(intMax) > nativePriorityMax {
		overflow := nativePriorityMax + 1
		cases = append(cases, struct {
			name     string
			metric   int
			protocol rtproto.Proto
		}{"metric truncates to zero", int(overflow), rtproto.Iface})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handle, backend := newRouteOwnershipSession(t)
			destination := netip.MustParsePrefix("198.51.100.0/24")
			gateway := netip.MustParseAddr("192.0.2.1")
			foreign := routeOwnershipTuple{"created-ppp", destination.String(), gateway.String(), 0, rtproto.Iface}
			backend.routes[foreign] = true
			if err := handle.AddRoute(destination, gateway, tc.metric, tc.protocol); err == nil {
				t.Fatal("nonrepresentable native route accepted")
			}
			if len(backend.adds) != 0 {
				t.Fatal("native-width refusal reached backend add")
			}
			if err := handle.RemoveRoute(destination, gateway, tc.metric, tc.protocol); err == nil {
				t.Fatal("nonrepresentable native withdrawal accepted")
			}
			if len(backend.removes) != 0 {
				t.Fatal("native-width refusal reached backend withdrawal")
			}
			if !backend.routes[foreign] {
				t.Fatal("native-width alias removed the foreign route")
			}
		})
	}
	// The production caller's metric-zero Iface route and maximum encodable
	// values remain valid; a blanket refusal must fail these positive controls.
	for _, metric := range []int{0, int(min(uint64(intMax), nativePriorityMax))} {
		handle, backend := newRouteOwnershipSession(t)
		destination := netip.MustParsePrefix("203.0.113.0/24")
		gateway := netip.MustParseAddr("192.0.2.1")
		protocol := rtproto.Iface
		if metric != 0 {
			protocol = rtproto.Proto(255)
		}
		if err := handle.AddRoute(destination, gateway, metric, protocol); err != nil {
			t.Fatalf("representable route refused: %v", err)
		}
		if err := handle.RemoveRoute(destination, gateway, metric, protocol); err != nil {
			t.Fatalf("representable withdrawal refused: %v", err)
		}
		if len(backend.adds) != 1 {
			t.Fatal("representable route did not reach native add")
		}
		if len(backend.removes) != 1 {
			t.Fatal("representable route did not reach native withdrawal")
		}
		if len(backend.routes) != 0 {
			t.Fatal("representable route survived withdrawal")
		}
	}
}

// Native deletion treats priority zero as a wildcard, even when explicitly
// encoded. One session must refuse a zero/nonzero pair sharing every other
// native match field until its retained first receipt is successfully removed.
func TestKernelSessionRouteZeroMetricAmbiguityRefusedBeforeReceipt(t *testing.T) {
	families := []struct {
		name        string
		destination netip.Prefix
		gateway     netip.Addr
	}{
		{"IPv4", netip.MustParsePrefix("198.51.100.0/24"), netip.MustParseAddr("192.0.2.1")},
		{"IPv6", netip.MustParsePrefix("2001:db8:1::/64"), netip.MustParseAddr("fe80::2")},
	}
	for _, family := range families {
		for _, first := range []int{0, 7} {
			t.Run(family.name+"/first="+fmt.Sprint(first), func(t *testing.T) {
				handle, backend := newRouteOwnershipSession(t)
				second := 0
				if first == 0 {
					second = 7
				}
				if err := handle.AddRoute(family.destination, family.gateway, first, rtproto.Iface); err != nil {
					t.Fatal(err)
				}
				if err := handle.AddRoute(family.destination, family.gateway, second, rtproto.Iface); err == nil {
					t.Fatal("ambiguous zero/nonzero route pair accepted")
				}
				if len(backend.adds) != 1 {
					t.Fatal("ambiguous route reached native add")
				}
				if err := handle.RemoveRoute(family.destination, family.gateway, second, rtproto.Iface); err != nil {
					t.Fatal(err)
				}
				if len(backend.removes) != 0 {
					t.Fatal("refused ambiguous route acquired native deletion rights")
				}
				failure := errors.New("native withdrawal unknown")
				backend.removeErr = failure
				if err := handle.RemoveRoute(family.destination, family.gateway, first, rtproto.Iface); !errors.Is(err, failure) {
					t.Fatalf("withdrawal failure lost: %v", err)
				}
				if err := handle.AddRoute(family.destination, family.gateway, second, rtproto.Iface); err == nil {
					t.Fatal("failed withdrawal released its ambiguity reservation")
				}
				backend.removeErr = nil
				if err := handle.RemoveRoute(family.destination, family.gateway, first, rtproto.Iface); err != nil {
					t.Fatal(err)
				}
				if err := handle.AddRoute(family.destination, family.gateway, second, rtproto.Iface); err != nil {
					t.Fatalf("successful withdrawal retained its ambiguity reservation: %v", err)
				}
				if err := handle.RemoveRoute(family.destination, family.gateway, second, rtproto.Iface); err != nil {
					t.Fatal(err)
				}
				if len(backend.routes) != 0 {
					t.Fatal("owned route survived final withdrawal")
				}
			})
		}
	}
	// Nonzero priorities remain independently withdrawable, and a distinct
	// producer stamp keeps another route outside the wildcard's match scope.
	handle, backend := newRouteOwnershipSession(t)
	destination := netip.MustParsePrefix("198.51.100.0/24")
	gateway := netip.MustParseAddr("192.0.2.1")
	for _, metric := range []int{7, 8} {
		if err := handle.AddRoute(destination, gateway, metric, rtproto.Iface); err != nil {
			t.Fatalf("distinct nonzero priority refused: %v", err)
		}
	}
	if err := handle.AddRoute(destination, gateway, 0, rtproto.Static); err != nil {
		t.Fatalf("different producer priority-zero route refused: %v", err)
	}
	if err := handle.RemoveRoute(destination, gateway, 0, rtproto.Static); err != nil {
		t.Fatal(err)
	}
	independent := []struct {
		destination netip.Prefix
		gateway     netip.Addr
	}{
		{destination, netip.MustParseAddr("192.0.2.2")},
		{netip.MustParsePrefix("203.0.113.0/24"), gateway},
	}
	for _, route := range independent {
		if err := handle.AddRoute(route.destination, route.gateway, 0, rtproto.Iface); err != nil {
			t.Fatalf("distinct match tuple refused: %v", err)
		}
	}
	for _, route := range independent {
		if err := handle.RemoveRoute(route.destination, route.gateway, 0, rtproto.Iface); err != nil {
			t.Fatal(err)
		}
	}
	for _, metric := range []int{7, 8} {
		route := routeOwnershipTuple{"created-ppp", destination.String(), gateway.String(), metric, rtproto.Iface}
		if !backend.routes[route] {
			t.Fatalf("permitted nonzero route disappeared: %+v", route)
		}
		if err := handle.RemoveRoute(destination, gateway, metric, rtproto.Iface); err != nil {
			t.Fatal(err)
		}
	}
	if len(backend.routes) != 0 {
		t.Fatal("permitted routes survived successful cleanup")
	}
	uncertain, pending := newRouteOwnershipSession(t)
	failure := errors.New("native add outcome unknown")
	pending.addErr = failure
	if err := uncertain.AddRoute(destination, gateway, 0, rtproto.Iface); !errors.Is(err, failure) {
		t.Fatalf("uncertain add error lost: %v", err)
	}
	if err := uncertain.AddRoute(destination, gateway, 7, rtproto.Iface); err == nil {
		t.Fatal("uncertain add released its ambiguity reservation")
	}
	if len(pending.adds) != 1 {
		t.Fatal("ambiguous route reached native add after an uncertain result")
	}
	if err := uncertain.RemoveRoute(destination, gateway, 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	pending.addErr = nil
	if err := uncertain.AddRoute(destination, gateway, 7, rtproto.Iface); err != nil {
		t.Fatalf("uncertain route cleanup did not release its reservation: %v", err)
	}
	if err := uncertain.RemoveRoute(destination, gateway, 7, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
}

// IPv4's unspecified gateway removes a native deletion selector, while mapped
// zero is the same native value. Unspecified peers must acquire no receipt.
func TestKernelSessionRouteUnspecifiedGatewayAcquiresNoReceipt(t *testing.T) {
	cases := []struct {
		destination netip.Prefix
		gateway     netip.Addr
	}{
		{netip.MustParsePrefix("198.51.100.0/24"), netip.MustParseAddr("0.0.0.0")},
		{netip.MustParsePrefix("198.51.100.0/24"), netip.MustParseAddr("::ffff:0.0.0.0")},
		{netip.MustParsePrefix("2001:db8:1::/64"), netip.MustParseAddr("::")},
	}
	for _, tc := range cases {
		t.Run(tc.gateway.String(), func(t *testing.T) {
			handle, backend := newRouteOwnershipSession(t)
			if err := handle.AddRoute(tc.destination, tc.gateway, 0, rtproto.Iface); err == nil {
				t.Fatal("unspecified peer accepted for native route")
			}
			if len(backend.adds) != 0 {
				t.Fatal("unspecified peer reached native add")
			}
			if err := handle.RemoveRoute(tc.destination, tc.gateway, 0, rtproto.Iface); err == nil {
				t.Fatal("unspecified peer accepted for native withdrawal")
			}
			if len(backend.removes) != 0 {
				t.Fatal("unspecified peer reached native withdrawal")
			}
		})
	}
}
