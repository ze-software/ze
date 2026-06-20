package iface

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	ifaceevents "codeberg.org/thomas-mangin/ze/internal/component/iface/events"
)

// emitBus is a minimal real in-process EventBus: Emit synchronously delivers to
// every Subscribe handler for the (namespace, eventType). It lets a test drive
// the resolver's full event path (bindEvents -> decodeLinkEvent -> onLinkEvent)
// the way the iface monitor does, without a live netlink monitor.
type emitBus struct {
	mu   sync.Mutex
	subs map[string][]func(any)
}

func newEmitBus() *emitBus { return &emitBus{subs: make(map[string][]func(any))} }

func (b *emitBus) Emit(ns, et string, payload any) (int, error) {
	b.mu.Lock()
	hs := append([]func(any){}, b.subs[ns+"/"+et]...)
	b.mu.Unlock()
	for _, h := range hs {
		h(payload)
	}
	return 0, nil
}

func (b *emitBus) Subscribe(ns, et string, handler func(any)) func() {
	b.mu.Lock()
	key := ns + "/" + et
	b.subs[key] = append(b.subs[key], handler)
	b.mu.Unlock()
	return func() {}
}

// VALIDATES: spec-iface-resolve-2-resolver AC-1..AC-7 -- the shared logical-name
// resolver (Resolve / Addresses / Subscribe), os-name translation, scope-tagged
// address classification, cache invalidation on link events, and value-only API.
// PREVENTS: a stale cached ifindex surviving a device delete; a netlink type
// leaking through the resolver's public Binding/LinkEvent/AddrInfo.

// resolveStubBackend is a minimal Backend that serves GetInterface/ListInterfaces
// from an in-memory map so the resolver's translate + cache + Binding-build path
// is host-testable without a netlink backend. The embedded Backend is nil; any
// other method panics if a test reaches it (none should).
type resolveStubBackend struct {
	Backend
	ifaces map[string]*InterfaceInfo
}

func (b *resolveStubBackend) GetInterface(name string) (*InterfaceInfo, error) {
	if info, ok := b.ifaces[name]; ok {
		return info, nil
	}
	return nil, fmt.Errorf("interface %s not found", name)
}

func (b *resolveStubBackend) ListInterfaces() ([]InterfaceInfo, error) {
	out := make([]InterfaceInfo, 0, len(b.ifaces))
	for _, v := range b.ifaces {
		out = append(out, *v)
	}
	return out, nil
}

// withResolveBackend installs b as the active backend for the test and restores
// the previous backend on cleanup.
func withResolveBackend(t *testing.T, b Backend) {
	t.Helper()
	backendsMu.Lock()
	prev := activeBackend
	activeBackend = b
	backendsMu.Unlock()
	t.Cleanup(func() {
		backendsMu.Lock()
		activeBackend = prev
		backendsMu.Unlock()
	})
}

// freshResolver returns an isolated resolver so a test does not race the package
// singleton's cached state.
func freshResolver() *resolver {
	return &resolver{
		cache: make(map[string]Binding),
		subs:  make(map[string]map[int]chan LinkEvent),
	}
}

// TestResolveExisting verifies Resolve populates every Binding field from the
// backend's InterfaceInfo (AC-1) and that the shape matches what the IS-IS /
// PPPoE ioctl wrappers produced -- ifindex, oper MAC, MTU (AC-2).
func TestResolveExisting(t *testing.T) {
	withResolveBackend(t, &resolveStubBackend{ifaces: map[string]*InterfaceInfo{
		"eth0": {
			Name: "eth0", OsName: "eth0", Index: 7, State: "up", MTU: 1500,
			MAC: "02:00:00:00:00:01", PermanentMAC: "aa:bb:cc:dd:ee:ff",
		},
	}})
	r := freshResolver()
	b, err := r.resolve("eth0")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if b.Ifindex != 7 || b.OperMAC != "02:00:00:00:00:01" || b.MTU != 1500 {
		t.Errorf("AC-2 wrapper shape wrong: %+v", b)
	}
	if b.OsName != "eth0" || b.PermMAC != "aa:bb:cc:dd:ee:ff" || b.State != "up" {
		t.Errorf("AC-1 binding fields wrong: %+v", b)
	}
}

// TestResolveAbsent verifies an absent interface returns an error and leaves no
// cache entry (AC-1).
func TestResolveAbsent(t *testing.T) {
	withResolveBackend(t, &resolveStubBackend{ifaces: map[string]*InterfaceInfo{}})
	r := freshResolver()
	if _, err := r.resolve("ghost"); err == nil {
		t.Fatal("expected not-found error for absent interface")
	}
	r.mu.Lock()
	_, cached := r.cache["ghost"]
	r.mu.Unlock()
	if cached {
		t.Error("absent interface must not be cached")
	}
}

// TestResolveByOsName verifies the os-name selector: a logical name resolves to
// a different kernel device, and the Binding reports the OS name it bound to.
func TestResolveByOsName(t *testing.T) {
	withResolveBackend(t, &resolveStubBackend{ifaces: map[string]*InterfaceInfo{
		"eth0": {Name: "eth0", OsName: "eth0", Index: 9, MTU: 1500, State: "up"},
	}})
	r := freshResolver()
	r.setMapping(map[string]string{"uplink": "eth0"})

	b, err := r.resolve("uplink")
	if err != nil {
		t.Fatalf("resolve uplink: %v", err)
	}
	if b.Ifindex != 9 || b.OsName != "eth0" {
		t.Errorf("logical->os mapping wrong: %+v", b)
	}
	// A name with no override resolves to itself.
	if _, err := r.resolve("eth0"); err != nil {
		t.Errorf("default mapping (name==os) must still resolve: %v", err)
	}
}

// TestAddressesScopeSplit verifies the link-local classifier reproduces IS-IS's
// v4 / v6-link-local / v6-global split (AC-3, A-2).
func TestAddressesScopeSplit(t *testing.T) {
	in := []AddrInfo{
		{Address: "192.0.2.1", PrefixLength: 24, Family: "ipv4"},
		{Address: "fe80::1", PrefixLength: 64, Family: "ipv6"},
		{Address: "2001:db8::1", PrefixLength: 64, Family: "ipv6"},
	}
	got := classifyAddresses(in)
	if got[0].LinkLocal {
		t.Error("IPv4 must never be link-local")
	}
	if !got[1].LinkLocal {
		t.Error("fe80::1 must be link-local")
	}
	if got[2].LinkLocal {
		t.Error("2001:db8::1 (global) must not be link-local")
	}
}

// TestSubscribeAppeared verifies a subscriber receives an appeared/up event for
// a late-appearing interface, including across an os-name mapping (AC-4).
func TestSubscribeAppeared(t *testing.T) {
	r := freshResolver()
	r.setMapping(map[string]string{"uplink": "eth0"})

	ch, cancel := r.subscribe("uplink")
	defer cancel()

	// Kernel device eth0 appears; the resolver must reach the logical name.
	r.onLinkEvent("eth0", LinkAppeared, 3)

	select {
	case ev := <-ch:
		if ev.Name != "uplink" || ev.Kind != LinkAppeared || ev.Index != 3 {
			t.Errorf("unexpected event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no appeared event delivered to subscriber")
	}
}

// TestCacheInvalidatedOnDelete verifies a down/delete event invalidates the
// cache so a subsequent Resolve never serves a stale ifindex (AC-5).
func TestCacheInvalidatedOnDelete(t *testing.T) {
	stub := &resolveStubBackend{ifaces: map[string]*InterfaceInfo{
		"eth0": {Name: "eth0", OsName: "eth0", Index: 4, MTU: 1500, State: "up"},
	}}
	withResolveBackend(t, stub)
	r := freshResolver()

	if _, err := r.resolve("eth0"); err != nil {
		t.Fatalf("initial resolve: %v", err)
	}
	// Device removed: monitor reports a down event (RTM_DELLINK -> down).
	delete(stub.ifaces, "eth0")
	r.onLinkEvent("eth0", LinkDown, 4)

	if _, err := r.resolve("eth0"); err == nil {
		t.Fatal("after delete, resolve must return not-found, not a stale ifindex")
	}
}

// TestSubscribeNoLeak verifies cancel releases the subscription with no leaked
// goroutine (fan-out is synchronous; only a channel + map entry are held), and
// is safe to call twice (AC-7).
func TestSubscribeNoLeak(t *testing.T) {
	r := freshResolver()
	ch, cancel := r.subscribe("x")

	r.mu.Lock()
	n := len(r.subs["x"])
	r.mu.Unlock()
	if n != 1 {
		t.Fatalf("expected 1 subscriber registered, got %d", n)
	}

	cancel()
	r.mu.Lock()
	_, present := r.subs["x"]
	r.mu.Unlock()
	if present {
		t.Error("cancel must remove the per-name subscriber map when empty")
	}
	if _, open := <-ch; open {
		t.Error("cancel must close the channel")
	}
	cancel() // idempotent: must not panic / double-close
}

// TestNoNetlinkLeakInAPI verifies the resolver's public value types expose no
// netlink / net.Interface fields, so consumers do not couple to the backend
// (AC-6).
func TestNoNetlinkLeakInAPI(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeFor[Binding](),
		reflect.TypeFor[LinkEvent](),
		reflect.TypeFor[AddrInfo](),
	} {
		for _, f := range reflect.VisibleFields(typ) {
			pkg := f.Type.PkgPath()
			if strings.Contains(pkg, "netlink") || strings.Contains(pkg, "vishvananda") || pkg == "net" {
				t.Errorf("%s.%s leaks backend type from %q", typ.Name(), f.Name, pkg)
			}
		}
	}
}

// TestBindEventsDeliversRemappedEvent verifies the full resolver event path: a
// monitor-shaped JSON-string event for a kernel device, emitted on the bus
// bindEvents subscribed to, is decoded and delivered to a subscriber registered
// under the LOGICAL name that the os-name selector maps to that device. It also
// confirms the same event invalidates the cache entry for the logical name.
func TestBindEventsDeliversRemappedEvent(t *testing.T) {
	r := freshResolver()
	r.setMapping(map[string]string{"uplink": "eth0"})

	bus := newEmitBus()
	r.bindEvents(bus)

	// Pre-seed a cache entry for the logical name so we can prove invalidation.
	r.mu.Lock()
	r.cache["uplink"] = Binding{Ifindex: 99, OsName: "eth0"}
	r.mu.Unlock()

	ch, cancel := r.subscribe("uplink")
	defer cancel()

	// The monitor emits the payload as a JSON string keyed by the KERNEL name.
	if _, err := bus.Emit(ifaceevents.Namespace, ifaceevents.EventUp, `{"name":"eth0","index":5}`); err != nil {
		t.Fatalf("emit: %v", err)
	}

	select {
	case ev := <-ch:
		if ev.Name != "uplink" || ev.Kind != LinkUp || ev.Index != 5 {
			t.Errorf("got %+v, want {uplink up 5}", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no event delivered through bindEvents -> decode -> remap")
	}

	r.mu.Lock()
	_, cached := r.cache["uplink"]
	r.mu.Unlock()
	if cached {
		t.Error("link event must invalidate the cached binding for the logical name")
	}
}

// TestPublicResolveWiring smoke-tests the exported Resolve func against the
// package singleton so the public entry point itself is exercised.
func TestPublicResolveWiring(t *testing.T) {
	withResolveBackend(t, &resolveStubBackend{ifaces: map[string]*InterfaceInfo{
		"lo": {Name: "lo", OsName: "lo", Index: 1, MTU: 65536, State: "up"},
	}})
	globalResolver.setMapping(nil)
	globalResolver.mu.Lock()
	globalResolver.cache = make(map[string]Binding)
	globalResolver.mu.Unlock()

	b, err := Resolve("lo")
	if err != nil {
		t.Fatalf("public Resolve: %v", err)
	}
	if b.Ifindex != 1 {
		t.Errorf("public Resolve returned wrong binding: %+v", b)
	}
}
