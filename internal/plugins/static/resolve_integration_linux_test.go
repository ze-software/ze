//go:build linux

// VALIDATES: a static route's nexthop interface is resolved through the shared
// iface resolver (iface.Resolve), not the static backend's own netlink handle,
// so a logical interface name maps to the right kernel device against the real
// netlink backend. The os-name / mac-match remapping itself is proven centrally
// in the iface package (TestResolveRemapsLogicalNameToOSDevice,
// TestResolveByMACBindsToDevice); this proves the static consumer is wired to it
// and returns the correct ifindex (and a clean error for an absent interface).
// PREVENTS: a regression to b.handle.LinkByName(name), which would fail for a
// logical name aliased via os-name/mac-match, or a "no backend loaded" surprise.

package static

import (
	"net"
	"net/netip"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/component/iface"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink" // register the netlink backend for iface.LoadBackend
	"github.com/ze-software/ze/internal/test/userns"
)

func TestResolveNexthopIndexUsesResolver(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withStaticNetNS(t, func() {
		if iface.GetBackend() == nil {
			if err := iface.LoadBackend("netlink"); err != nil {
				t.Fatalf("load iface backend: %v", err)
			}
		}
		const dev = "zestatic0"
		if err := iface.CreateDummy(dev); err != nil {
			t.Fatalf("create dummy %s: %v", dev, err)
		}
		t.Cleanup(func() { _ = iface.DeleteInterface(dev) })
		if err := iface.SetAdminUp(dev); err != nil {
			t.Fatalf("set %s up: %v", dev, err)
		}

		link, err := netlink.LinkByName(dev)
		if err != nil {
			t.Fatalf("lookup %s: %v", dev, err)
		}
		want := link.Attrs().Index

		idx, err := resolveNexthopIndex(dev)
		if err != nil {
			t.Fatalf("resolveNexthopIndex(%s): %v", dev, err)
		}
		if idx != want {
			t.Errorf("ifindex = %d, want real %d", idx, want)
		}

		if _, err := resolveNexthopIndex("zeghost0"); err == nil {
			t.Error("resolveNexthopIndex must error for an absent interface")
		}
	})
}

// withStaticNetNS runs fn in a fresh network namespace. The caller has entered
// a user namespace (userns.Enter), where root holds CAP_NET_ADMIN, so a refusal
// fails the test. Mirrors the per-package netns helper used by the isis / fib /
// traffic integration tests (there is no shared helper).
func withStaticNetNS(t *testing.T, fn func()) {
	t.Helper()
	runtime.LockOSThread()
	orig, err := netns.Get()
	if err != nil {
		t.Fatalf("read the current network namespace: %v", err)
	}
	ns, err := netns.NewNamed("zestaticrt")
	if err != nil {
		orig.Close() //nolint:errcheck // best-effort cleanup
		t.Fatalf("create network namespace zestaticrt inside the user namespace (CAP_NET_ADMIN): %v", err)
	}
	t.Cleanup(func() {
		if rerr := netns.Set(orig); rerr != nil {
			t.Errorf("restore netns: %v", rerr)
		}
		orig.Close()                    //nolint:errcheck // best-effort cleanup
		ns.Close()                      //nolint:errcheck // best-effort cleanup
		netns.DeleteNamed("zestaticrt") //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
	})
	fn()
}

// followBus is an EventBus that delivers to its subscribers on the emitter's
// goroutine, as EmitEngineEvent does, so the netlink monitor drives the
// resolver, and the resolver drives static's link watch, through the real
// chain.
type followBus struct {
	mu   sync.Mutex
	subs map[string][]func(any)
}

func (b *followBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	handlers := slices.Clone(b.subs[namespace+"/"+eventType])
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return len(handlers), nil
}

func (b *followBus) Subscribe(namespace, eventType string, fn func(any)) func() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs == nil {
		b.subs = map[string][]func(any){}
	}
	key := namespace + "/" + eventType
	b.subs[key] = append(b.subs[key], fn)
	return func() {}
}

// followDeviceRig is one namespace's real chain: the netlink iface backend,
// its monitor on a followBus, the resolver bound to that bus, and a route
// manager writing named tables through static's netlink backend with the
// production linkSource (ifaceLinks).
type followDeviceRig struct {
	h   *netlink.Handle
	mgr *routeManager
}

// newFollowDeviceRig MUST run inside withStaticNetNS: the backend, the monitor
// and the handle bind to the namespace of the calling thread.
func newFollowDeviceRig(t *testing.T) *followDeviceRig {
	t.Helper()
	if err := iface.LoadBackend("netlink"); err != nil {
		t.Fatalf("load iface backend: %v", err)
	}
	t.Cleanup(func() { _ = iface.CloseBackend() })
	bus := &followBus{}
	t.Cleanup(iface.BindResolverEventsForConsumerTest(bus))
	b := iface.GetBackend()
	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("start iface monitor: %v", err)
	}
	t.Cleanup(b.StopMonitor)

	h, err := netlink.NewHandle()
	if err != nil {
		t.Fatalf("netlink handle: %v", err)
	}
	t.Cleanup(h.Close)
	mgr := newRouteManager(&netlinkStaticBackend{handle: h})
	t.Cleanup(mgr.shutdown)
	return &followDeviceRig{h: h, mgr: mgr}
}

// addDummy creates the dummy device name, sets it up and returns its ifindex.
func (r *followDeviceRig) addDummy(t *testing.T, name string) int {
	t.Helper()
	if err := r.h.LinkAdd(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: name}}); err != nil {
		t.Fatalf("create dummy %s: %v", name, err)
	}
	link, err := r.h.LinkByName(name)
	if err != nil {
		t.Fatalf("lookup %s: %v", name, err)
	}
	if err := r.h.LinkSetUp(link); err != nil {
		t.Fatalf("set %s up: %v", name, err)
	}
	return link.Attrs().Index
}

func (r *followDeviceRig) delDummy(t *testing.T, name string) {
	t.Helper()
	link, err := r.h.LinkByName(name)
	if err != nil {
		t.Fatalf("lookup %s: %v", name, err)
	}
	if err := r.h.LinkDel(link); err != nil {
		t.Fatalf("delete %s: %v", name, err)
	}
}

// staticRoutes reads the proto-static routes the kernel holds for prefix in
// table.
func (r *followDeviceRig) staticRoutes(t *testing.T, table int, prefix netip.Prefix) []netlink.Route {
	t.Helper()
	filter := &netlink.Route{Table: table, Dst: prefixToIPNet(prefix)}
	routes, err := r.h.RouteListFiltered(netlink.FAMILY_V4, filter, netlink.RT_FILTER_TABLE|netlink.RT_FILTER_DST)
	if err != nil {
		t.Fatalf("list table %d: %v", table, err)
	}
	out := routes[:0]
	for i := range routes {
		if routes[i].Protocol == rtprotStatic {
			out = append(out, routes[i])
		}
	}
	return out
}

// waitKernel waits, within a bound far above the monitor's latency, until
// check accepts what the kernel holds for prefix in table. It asserts state,
// never elapsed time: the loop ends the moment the state is right.
func (r *followDeviceRig) waitKernel(t *testing.T, step string, table int, prefix netip.Prefix, check func([]netlink.Route) bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		routes := r.staticRoutes(t, table, prefix)
		if check(routes) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: table %d holds %+v for %v", step, table, routes, prefix)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// waitWithdrawn waits until static has handled the device's loss for key: the
// route is held, and nothing of it is recorded as installed. A removal that
// failed leaves programmed set (programRouteLocked clears it only on
// success), so this is the proof that no removal error was met.
func (r *followDeviceRig) waitWithdrawn(t *testing.T, step string, key routeKey) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		r.mgr.mu.Lock()
		rs, held := r.mgr.routes[key]
		withdrawn := held && rs.programmed == nil
		r.mgr.mu.Unlock()
		if withdrawn {
			return
		}
		if !held {
			t.Fatalf("%s: route %v table %d is no longer held", step, key.prefix, key.table)
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: static still records route %v table %d as installed: its removal failed or never ran", step, key.prefix, key.table)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func throughDevice(index int) func([]netlink.Route) bool {
	return func(routes []netlink.Route) bool {
		return len(routes) == 1 && routes[0].LinkIndex == index && len(routes[0].MultiPath) == 0
	}
}

func noRoute(routes []netlink.Route) bool { return len(routes) == 0 }

func throughGatewayAlone(gateway net.IP) func([]netlink.Route) bool {
	return func(routes []netlink.Route) bool {
		return len(routes) == 1 && routes[0].Gw.Equal(gateway) && len(routes[0].MultiPath) == 0
	}
}

// throughBoth accepts the mixed group: the gateway at weight 1 and the device
// at weight 3 (Hops is weight - 1).
func throughBoth(gateway net.IP, index int) func([]netlink.Route) bool {
	return func(routes []netlink.Route) bool {
		if len(routes) != 1 || len(routes[0].MultiPath) != 2 {
			return false
		}
		var viaGateway, viaDevice bool
		for _, nh := range routes[0].MultiPath {
			if nh.Gw.Equal(gateway) && nh.Hops == 0 {
				viaGateway = true
			}
			if nh.LinkIndex == index && nh.Gw == nil && nh.Hops == 2 {
				viaDevice = true
			}
		}
		return viaGateway && viaDevice
	}
}

// TestStaticInterfaceRouteFollowsDevice is AC-13, A-3 and A-7 against a real
// kernel. Method, in a user and network namespace, through the real chain (the
// netlink monitor, the bus, the bound resolver, static's link watch, static's
// netlink backend): a named-table route through a declared on-demand name is
// committed while its device is absent, then a dummy device under that name is
// created and set up, deleted, and recreated; the named table is read back
// after each step.
// PREVENTS: a waiting route that never installs (A-3: the monitor's up event
// must reach static); a route still recorded after the kernel flushed it, or a
// removal that fails on the device that is gone (A-7); a reconnect that does
// not reinstall, or reinstalls through the stale ifindex.
func TestStaticInterfaceRouteFollowsDevice(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withStaticNetNS(t, func() {
		rig := newFollowDeviceRig(t)
		const dev = "zefollow0"
		const table = 100
		prefix := netip.MustParsePrefix("10.99.80.0/24")
		route := staticRoute{
			Prefix: prefix, Table: table, Action: actionForward,
			NextHops: []nextHop{{Interface: dev, Weight: 1, Dynamic: true}},
		}
		if err := rig.mgr.applyRoutes([]staticRoute{route}); err != nil {
			t.Fatalf("applyRoutes: %v", err)
		}
		if got := rig.staticRoutes(t, table, prefix); len(got) != 0 {
			t.Fatalf("committed with the device absent: table %d holds %+v, want nothing", table, got)
		}

		first := rig.addDummy(t, dev)
		rig.waitKernel(t, "device up", table, prefix, throughDevice(first))

		rig.delDummy(t, dev)
		rig.waitKernel(t, "device deleted", table, prefix, noRoute)
		rig.waitWithdrawn(t, "device deleted", routeKey{table: table, prefix: prefix})

		second := rig.addDummy(t, dev)
		rig.waitKernel(t, "device recreated", table, prefix, throughDevice(second))
	})
}

// TestStaticMixedECMPFollowsDevice is AC-15 and A-8 against a real kernel, in a
// named table: a gateway member (192.0.2.1, on a second dummy device that stays
// up) and a member through a declared on-demand name. Method as in
// TestStaticInterfaceRouteFollowsDevice, reading the table back after each
// step. Right after the deletion the kernel is read once and what it holds is
// logged: A-8 assumes the kernel flushes the whole multipath route, and the
// design holds whether it flushes or shrinks it, because static replaces the
// route with the survivors either way.
// PREVENTS: the mixed route withdrawn while its gateway is active, a session
// drop that leaves the dead member in the group, and a reconnect that does not
// bring the device back into the group.
func TestStaticMixedECMPFollowsDevice(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withStaticNetNS(t, func() {
		rig := newFollowDeviceRig(t)
		const dev = "zefollow1"
		const gwDev = "zefollowgw0"
		const table = 101
		rig.addDummy(t, gwDev)
		gwLink, err := rig.h.LinkByName(gwDev)
		if err != nil {
			t.Fatalf("lookup %s: %v", gwDev, err)
		}
		if err := rig.h.AddrAdd(gwLink, &netlink.Addr{
			IPNet: &net.IPNet{IP: net.ParseIP("192.0.2.2").To4(), Mask: net.CIDRMask(24, 32)},
		}); err != nil {
			t.Fatalf("address %s: %v", gwDev, err)
		}
		gateway := netip.MustParseAddr("192.0.2.1")
		gw := net.IP(gateway.AsSlice())
		prefix := netip.MustParsePrefix("10.99.81.0/24")
		route := staticRoute{
			Prefix: prefix, Table: table, Action: actionForward,
			NextHops: []nextHop{
				{Address: gateway, Weight: 1},
				{Interface: dev, Weight: 3, Dynamic: true},
			},
		}
		if err := rig.mgr.applyRoutes([]staticRoute{route}); err != nil {
			t.Fatalf("applyRoutes: %v", err)
		}
		rig.waitKernel(t, "committed, device absent", table, prefix, throughGatewayAlone(gw))

		first := rig.addDummy(t, dev)
		rig.waitKernel(t, "device up", table, prefix, throughBoth(gw, first))

		rig.delDummy(t, dev)
		t.Logf("A-8: right after deleting %s, table %d holds %+v", dev, table, rig.staticRoutes(t, table, prefix))
		rig.waitKernel(t, "device deleted", table, prefix, throughGatewayAlone(gw))

		second := rig.addDummy(t, dev)
		rig.waitKernel(t, "device recreated", table, prefix, throughBoth(gw, second))
	})
}
