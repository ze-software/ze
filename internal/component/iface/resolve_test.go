package iface

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
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
	r.setMapping(map[string]string{"uplink": "eth0"}, nil)

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
	r.setMapping(map[string]string{"uplink": "eth0"}, nil)

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
	r.setMapping(map[string]string{"uplink": "eth0"}, nil)

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
	globalResolver.setMapping(nil, nil)
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

// VALIDATES: the mac/match selector -- binding a logical interface to a kernel
// device by its hardware MAC rather than by name. The resolver matches the
// permanent (factory) MAC when the device reports one (so the binding survives
// an operational MAC override) and falls back to the current MAC for the
// virtual kinds that report none. Covers resolution (case-insensitive),
// precedence over os-name, the permanent-over-current preference, the
// absent/deferred case, and the event path (a freshly appeared device attaches;
// a down device invalidates via the last-known binding).
// PREVENTS: a mac/match binding that resolves by name, that an operational MAC
// override silently steals, or that never fires an event for the device it
// selected.

// macStub builds a stub backend whose ListInterfaces/GetInterface serve the
// given interfaces by name, so the mac-scan + match path is host-testable.
func macStub(ifaces ...*InterfaceInfo) *resolveStubBackend {
	m := make(map[string]*InterfaceInfo, len(ifaces))
	for _, i := range ifaces {
		m[i.Name] = i
	}
	return &resolveStubBackend{ifaces: m}
}

// TestResolveByPermMAC verifies a mac/match selector binds to the device that
// carries that permanent MAC, regardless of device name, case-insensitively.
func TestResolveByPermMAC(t *testing.T) {
	withResolveBackend(t, macStub(
		&InterfaceInfo{Name: "eth0", OsName: "eth0", Index: 9, MTU: 1500, State: "up", PermanentMAC: "aa:bb:cc:dd:ee:01"},
		&InterfaceInfo{Name: "eth1", OsName: "eth1", Index: 10, MTU: 1500, State: "up", PermanentMAC: "aa:bb:cc:dd:ee:02"},
	))
	r := freshResolver()
	// An uppercase config value must normalize and still match eth1.
	r.setMapping(nil, map[string]string{"uplink": "AA:BB:CC:DD:EE:02"})

	b, err := r.resolve("uplink")
	if err != nil {
		t.Fatalf("resolve uplink: %v", err)
	}
	if b.Ifindex != 10 || b.OsName != "eth1" {
		t.Errorf("mac/match bound wrong device: %+v (want eth1/10)", b)
	}
}

// TestResolveByPermMACPrefersPermanentOverCurrent verifies the selector matches
// the permanent MAC, not the (possibly overridden) current MAC, when a device
// reports a permanent address. Matching the overridden current value must NOT
// bind -- that is the whole point of keying on the factory address.
func TestResolveByPermMACPrefersPermanentOverCurrent(t *testing.T) {
	withResolveBackend(t, macStub(
		&InterfaceInfo{Name: "eth0", OsName: "eth0", Index: 7, MTU: 1500, State: "up",
			PermanentMAC: "aa:bb:cc:dd:ee:ff", MAC: "02:00:00:00:00:99"},
	))
	r := freshResolver()
	r.setMapping(nil, map[string]string{
		"perm": "aa:bb:cc:dd:ee:ff", // permanent -> binds
		"oper": "02:00:00:00:00:99", // overridden current -> must NOT bind
	})

	if b, err := r.resolve("perm"); err != nil || b.Ifindex != 7 {
		t.Errorf("match on permanent MAC must bind eth0: b=%+v err=%v", b, err)
	}
	if _, err := r.resolve("oper"); err == nil {
		t.Error("match on an overridden current MAC must not bind a device that has a permanent MAC")
	}
}

// TestResolveByPermMACFallsBackToCurrent verifies a virtual device (no permanent
// MAC) is matched by its current MAC -- the only hardware identity it has.
func TestResolveByPermMACFallsBackToCurrent(t *testing.T) {
	withResolveBackend(t, macStub(
		&InterfaceInfo{Name: "veth9", OsName: "veth9", Index: 12, MTU: 1500, State: "up", MAC: "02:00:00:00:00:0c"},
	))
	r := freshResolver()
	r.setMapping(nil, map[string]string{"lab": "02:00:00:00:00:0c"})

	b, err := r.resolve("lab")
	if err != nil {
		t.Fatalf("resolve lab: %v", err)
	}
	if b.Ifindex != 12 || b.OsName != "veth9" {
		t.Errorf("current-MAC fallback bound wrong device: %+v", b)
	}
}

// TestResolveByPermMACPrecedence verifies a name with BOTH os-name and mac/match
// binds by MAC: the selector takes precedence over os-name.
func TestResolveByPermMACPrecedence(t *testing.T) {
	withResolveBackend(t, macStub(
		&InterfaceInfo{Name: "eth0", OsName: "eth0", Index: 3, MTU: 1500, State: "up", PermanentMAC: "aa:bb:cc:00:00:01"},
		&InterfaceInfo{Name: "eth1", OsName: "eth1", Index: 4, MTU: 1500, State: "up", PermanentMAC: "aa:bb:cc:00:00:02"},
	))
	r := freshResolver()
	r.setMapping(
		map[string]string{"uplink": "eth0"},              // os-name -> eth0
		map[string]string{"uplink": "aa:bb:cc:00:00:02"}, // mac/match -> eth1 (wins)
	)

	b, err := r.resolve("uplink")
	if err != nil {
		t.Fatalf("resolve uplink: %v", err)
	}
	if b.Ifindex != 4 || b.OsName != "eth1" {
		t.Errorf("mac/match must take precedence over os-name: %+v (want eth1/4)", b)
	}
}

// TestResolveByPermMACAbsent verifies an unmatched MAC is a deferred binding:
// Resolve returns not-found and caches nothing.
func TestResolveByPermMACAbsent(t *testing.T) {
	withResolveBackend(t, macStub(
		&InterfaceInfo{Name: "eth0", OsName: "eth0", Index: 1, MTU: 1500, State: "up", PermanentMAC: "aa:bb:cc:00:00:01"},
	))
	r := freshResolver()
	r.setMapping(nil, map[string]string{"uplink": "de:ad:be:ef:00:01"})

	if _, err := r.resolve("uplink"); err == nil {
		t.Fatal("absent mac/match device must return not-found, not a binding")
	}
	r.mu.Lock()
	_, cached := r.cache["uplink"]
	r.mu.Unlock()
	if cached {
		t.Error("absent mac/match binding must not be cached")
	}
}

// TestSubscribePermMACAppeared verifies a subscriber on a mac/match logical name
// is notified when the matching kernel device appears, even though the device
// name differs. The resolver learns the device MAC from the up event and routes
// by it.
func TestSubscribePermMACAppeared(t *testing.T) {
	withResolveBackend(t, macStub(
		&InterfaceInfo{Name: "eth5", OsName: "eth5", Index: 5, MTU: 1500, State: "up", PermanentMAC: "aa:bb:cc:dd:ee:05"},
	))
	r := freshResolver()
	r.setMapping(nil, map[string]string{"uplink": "aa:bb:cc:dd:ee:05"})

	ch, cancel := r.subscribe("uplink")
	defer cancel()

	r.onLinkEvent("eth5", LinkAppeared, 5)

	select {
	case ev := <-ch:
		if ev.Name != "uplink" || ev.Kind != LinkAppeared || ev.Index != 5 {
			t.Errorf("unexpected event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no appeared event delivered to mac/match subscriber")
	}
}

// TestPermMACDownInvalidatesBoundDevice verifies that once a mac/match name has
// resolved to a device, a down event for that device invalidates the cache and
// notifies the subscriber via the last-known binding -- the device's MAC is no
// longer readable once it is gone.
func TestPermMACDownInvalidatesBoundDevice(t *testing.T) {
	stub := macStub(
		&InterfaceInfo{Name: "eth5", OsName: "eth5", Index: 5, MTU: 1500, State: "up", PermanentMAC: "aa:bb:cc:dd:ee:05"},
	)
	withResolveBackend(t, stub)
	r := freshResolver()
	r.setMapping(nil, map[string]string{"uplink": "aa:bb:cc:dd:ee:05"})

	if _, err := r.resolve("uplink"); err != nil {
		t.Fatalf("initial resolve: %v", err)
	}
	ch, cancel := r.subscribe("uplink")
	defer cancel()

	// Device removed: the monitor reports a down event (RTM_DELLINK -> down).
	delete(stub.ifaces, "eth5")
	r.onLinkEvent("eth5", LinkDown, 5)

	select {
	case ev := <-ch:
		if ev.Name != "uplink" || ev.Kind != LinkDown {
			t.Errorf("unexpected event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("down event for the bound device must reach the mac/match subscriber")
	}
	if _, err := r.resolve("uplink"); err == nil {
		t.Fatal("after the device is gone, resolve must not serve a stale binding")
	}
}

// countingBackend records how many times a link event handler reached the
// backend. Every other method comes from the embedded stub.
type countingBackend struct {
	*resolveStubBackend
	mu    sync.Mutex
	calls int
}

func (b *countingBackend) GetInterface(name string) (*InterfaceInfo, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	return b.resolveStubBackend.GetInterface(name)
}

func (b *countingBackend) ListInterfaces() ([]InterfaceInfo, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	return b.resolveStubBackend.ListInterfaces()
}

func (b *countingBackend) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

// TestLinkEventHandlerMakesNoBackendCall holds the event bus handler contract on
// the one handler that broke it.
//
// VALIDATES: AC-4 -- no handler calls the backend on the emitter's goroutine.
// EmitEngineEvent (internal/component/plugin/server/engine_event.go) runs a
// subscriber synchronously on the caller's goroutine, and for a link event that
// caller is the netlink monitor's read loop. pkg/ze/eventbus.go states that such
// a handler "MUST NOT block on I/O".
// PREVENTS: the stall that overflows the kernel-side queue. onLinkEvent called
// GetInterface for every up or appeared event whenever ANY mac/match selector
// was configured, to learn the device's MAC. The backend is pluggable, so that
// call is a plugin round-trip on a VPP box, taken on the read loop.
//
// The mapping below is what made the old code call: it configures a mac/match
// selector, so `hasPermMACMatches` was true and the lookup ran. The second half
// of the assertion is why the fix is not a deletion -- the deferred binding must
// still be reached, which logicalsForLocked now does without knowing the MAC.
func TestLinkEventHandlerMakesNoBackendCall(t *testing.T) {
	backend := &countingBackend{resolveStubBackend: &resolveStubBackend{ifaces: map[string]*InterfaceInfo{
		"eth5": {Name: "eth5", OsName: "eth5", Index: 5, State: "up", PermanentMAC: "aa:bb:cc:dd:ee:05"},
	}}}
	withResolveBackend(t, backend)

	r := freshResolver()
	r.setMapping(nil, map[string]string{"uplink": "aa:bb:cc:dd:ee:05"})
	bus := newEmitBus()
	r.bindEvents(bus)

	ch, cancel := r.subscribe("uplink")
	defer cancel()

	if _, err := bus.Emit(ifaceevents.Namespace, ifaceevents.EventUp, `{"name":"eth5","index":5}`); err != nil {
		t.Fatalf("emit: %v", err)
	}

	if got := backend.count(); got != 0 {
		t.Errorf("the link event handler made %d backend calls on the emitter goroutine, want 0", got)
	}
	select {
	case ev := <-ch:
		if ev.Name != "uplink" || ev.Kind != LinkUp {
			t.Errorf("unexpected event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("the deferred mac/match binding must still be reached without the backend call")
	}
}

// TestResolverDropIsCounted proves the one place that still discards a link
// event says so.
//
// VALIDATES: AC-3 -- a discarded event is counted and named. The resolver
// fan-out never blocks the netlink monitor's read loop, so a subscriber that
// falls behind loses an event. What it must not do is lose one silently.
// PREVENTS: the invisibility this spec exists for, met a second time. The
// discard carried a Debug line and no counter, so an operator whose IS-IS
// circuit came back a rescan late had nothing to read.
//
// WHICH event is discarded is the separate guarantee, held by
// TestResolverKeepsTheFinalStateWhenTheChannelFills below.
func TestResolverDropIsCounted(t *testing.T) {
	reg := bindCapturingMetrics(t)

	r := freshResolver()
	ch, cancel := r.subscribe("eth0")
	defer cancel()

	// subscribe hands out a channel 8 deep and nothing is reading it, so the
	// ninth event onwards has nowhere to go.
	const capacity = 8
	const overflow = 3
	for range capacity + overflow {
		r.onLinkEvent("eth0", LinkUp, 1)
	}

	counter := reg.counterVecs["ze_iface_resolver_events_dropped_total"]
	if counter == nil {
		t.Fatal("bindMetricsRegistry must create the resolver drop counter")
	}
	if got := counter.value("eth0"); got != float64(overflow) {
		t.Errorf("dropped counter for eth0 is %v, want %d", got, overflow)
	}
	if len(ch) != capacity {
		t.Errorf("subscriber holds %d events, want the full %d", len(ch), capacity)
	}
}

// TestResolverKeepsTheFinalStateWhenTheChannelFills holds the fan-out to the
// guarantee its consumers depend on.
//
// VALIDATES: a subscriber that cannot keep up loses an EARLIER event, never the
// state the interface ended in. sendLatest (resolve.go) discards the oldest
// buffered event to make room; a bare non-blocking send discards the newest.
// PREVENTS: stranding a consumer that accumulates rather than recomputes.
// Sender.onLinkEvent (internal/plugins/iface/ra/sender_linux.go) sets
// state.linkDown on a down, and its timer branch returns without rearming while
// that flag is set, so the next up is the only thing that can restart router
// advertisements. Discarding THAT event stopped advertisements on the interface
// for the life of the process, and no timer anywhere repaired it.
func TestResolverKeepsTheFinalStateWhenTheChannelFills(t *testing.T) {
	r := freshResolver()
	ch, cancel := r.subscribe("eth0")
	defer cancel()

	// subscribe hands out a channel 8 deep and nothing is reading it, so the
	// burst below overruns it well before the final transition arrives.
	const capacity = 8
	for range capacity + 4 {
		r.onLinkEvent("eth0", LinkUp, 1)
	}
	r.onLinkEvent("eth0", LinkDown, 1)

	var got []LinkEvent
	for len(ch) > 0 {
		got = append(got, <-ch)
	}
	if len(got) != capacity {
		t.Fatalf("subscriber holds %d events, want the full %d", len(got), capacity)
	}
	if last := got[len(got)-1]; last.Kind != LinkDown {
		t.Errorf("the last event delivered is %q, want %q: a consumer that pauses on down and resumes only on up is stranded when the final state is the one discarded",
			string(last.Kind), string(LinkDown))
	}
}
