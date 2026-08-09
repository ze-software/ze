// Design: docs/architecture/ospf/ospf-4-component-config.md -- engine and dispatcher tests
//
// VALIDATES: plugin inventory wiring, raw-transport interface enrolment over a
// fake backend, journal reconcile without restart-all, and packet dispatcher
// drops for short, wrong-version, bad-checksum, unknown-type, and interface/area mismatches.
package ospf

import (
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	ospfiface "github.com/ze-software/ze/internal/plugins/ospf/iface"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

type fakeBackend struct {
	mu      sync.Mutex
	nextIdx int
	opened  []string
	// attempted records every OpenInterface call, including the ones failFor
	// rejects, so a test can tell "never tried" from "tried and failed".
	attempted []string
	// failFor makes OpenInterface fail for the named interfaces, modeling a link
	// the kernel cannot give a usable source address (a loopback has no IPv6
	// link-local, an IPv6-disabled link has none, a link in DAD not yet).
	failFor map[string]error
	handles map[string]*fakeHandle
}

func (b *fakeBackend) OpenInterface(name string, _ transport.DropRecorder) (transport.InterfaceHandle, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.attempted = append(b.attempted, name)
	if err, bad := b.failFor[name]; bad {
		return nil, err
	}
	if b.handles == nil {
		b.handles = make(map[string]*fakeHandle)
	}
	b.nextIdx++
	h := &fakeHandle{ifindex: b.nextIdx, recv: make(chan transport.RawPacket, 8)}
	b.opened = append(b.opened, name)
	b.handles[name] = h
	return h, nil
}

type fakeHandle struct {
	ifindex int
	recv    chan transport.RawPacket
	once    sync.Once
	joined  bool
}

func (h *fakeHandle) IfIndex() int                      { return h.ifindex }
func (h *fakeHandle) Send(_ netip.Addr, _ []byte) error { return nil }
func (h *fakeHandle) Recv() <-chan transport.RawPacket  { return h.recv }
func (h *fakeHandle) JoinAllSPFRouters() error          { h.joined = true; return nil }
func (h *fakeHandle) JoinAllDRouters() error            { return nil }
func (h *fakeHandle) LeaveAllDRouters() error           { return nil }
func (h *fakeHandle) Close() error {
	h.once.Do(func() { close(h.recv) })
	return nil
}

func TestOSPFComponentStart(t *testing.T) {
	if !registry.Has("ospf") {
		t.Fatal("ospf is not in the plugin registry (make generate must wire all.go)")
	}
	r := registry.Lookup("ospf")
	if r == nil {
		t.Fatal("registry.Lookup(ospf) returned nil")
	}
	if r.RunEngine == nil {
		t.Error("ospf registration has no RunEngine")
	}
	if !slices.Equal(r.ConfigRoots, []string{"ospf"}) {
		t.Errorf("ConfigRoots = %v, want [ospf]", r.ConfigRoots)
	}
	if !slices.Contains(r.Dependencies, "interface") {
		t.Errorf("Dependencies = %v, want interface for router-id derivation", r.Dependencies)
	}
	if r.YANG == "" {
		t.Error("ospf registration has no YANG")
	}
	if r.InProcessConfigVerifier == nil {
		t.Error("ospf registration has no InProcessConfigVerifier")
	}

	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"},"eth1":{"area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	fb := &fakeBackend{}
	eng := newEngine(transport.New(fb))
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()
	if !eng.transport.InterfaceOpen("eth0") || !eng.transport.InterfaceOpen("eth1") {
		t.Fatalf("expected eth0 and eth1 open, count=%d", eng.transport.OpenInterfaceCount())
	}
	if len(fb.opened) != 2 {
		t.Fatalf("fake backend opened %v, want 2 interfaces", fb.opened)
	}
}

func TestOSPFPassiveAndLoopbackRecordsDoNotOpenTransport(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"},"passive0":{"area":"0","passive":"true"},"lo":{"area":"0","network-type":"loopback"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	eng := newEngine(transport.New(&fakeBackend{}))
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()
	if !eng.transport.InterfaceOpen("eth0") {
		t.Fatal("active interface eth0 not opened")
	}
	if eng.transport.InterfaceOpen("passive0") || eng.transport.InterfaceOpen("lo") {
		t.Fatal("passive or loopback interface opened a raw transport socket")
	}
	if got := snapshotByName(t, eng.interfaceSnapshot(), "passive0").State; got != "down" {
		t.Fatalf("passive state = %s, want down", got)
	}
	if got := snapshotByName(t, eng.interfaceSnapshot(), "lo").State; got != "loopback" {
		t.Fatalf("loopback state = %s, want loopback", got)
	}
}

// VALIDATES: an interface the backend cannot open leaves the engine STARTED --
// openInterfaces logs that interface and keeps going, and every other enrolled
// interface is open. The failing link is attempted (not silently skipped) so the
// transport can retry it from RescanInterfaces.
// PREVENTS: the ospfv3-vlink QEMU failure. `interface lo` under `address-family
// ipv6` can never have an IPv6 link-local, so its open failed; openInterfaces
// returned that error, v6EngineSet.start propagated it, and the plugin's
// post-startup callback exited the WHOLE ospf plugin ("internal plugin exited
// with non-zero code plugin=ospf code=1"), after which every `show ospf`
// answered "plugin process not running". One unopenable link must never take
// down every instance and address family.
func TestOpenInterfacesSurvivesOneFailingInterface(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"},"lo":{"area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	fb := &fakeBackend{failFor: map[string]error{"lo": errNoSourceAddress}}
	eng := newEngine(transport.New(fb))
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces = %v, want nil: one unopenable interface must not fail engine start", err)
	}
	defer eng.shutdown()
	if !eng.transport.InterfaceOpen("eth0") {
		t.Error("eth0 is not open: a sibling interface's failure stopped the enrolment loop")
	}
	if eng.transport.InterfaceOpen("lo") {
		t.Error("lo reported open although the backend refused it")
	}
	if !slices.Contains(fb.attempted, "lo") {
		t.Errorf("backend attempts = %v, want an attempt for lo: it must stay pending for RescanInterfaces to retry", fb.attempted)
	}
}

func TestOSPFTopologyRetainsDownActiveArea(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"},"passive0":{"area":"0","passive":"true"},"lo":{"area":"0","network-type":"loopback"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	eng := newEngine(transport.New(&fakeBackend{}))
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()

	if err := eng.transport.HandleLinkDown("eth0"); err != nil {
		t.Fatalf("HandleLinkDown: %v", err)
	}
	var names []string
	downActive := false
	for _, iface := range eng.lsdbTopology() {
		names = append(names, iface.Name)
		if iface.Name == "eth0" && iface.State == "down" && !iface.Passive {
			downActive = true
		}
	}
	if !downActive {
		t.Fatalf("topology names = %v, want down active eth0 retained for empty-area re-origination", names)
	}
	if !slices.Contains(names, "passive0") || !slices.Contains(names, "lo") {
		t.Fatalf("topology names = %v, want passive and loopback stubs retained", names)
	}
}

func TestOSPFConfigApplyReconcile(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","cost":"10"},"eth1":{"area":"0","cost":"10"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	fb := &fakeBackend{}
	eng := newEngine(transport.New(fb))
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()

	changed, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","cost":"10"},"eth1":{"area":"0","cost":"20"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(changed): %v", err)
	}
	res := eng.reconcile(changed)
	if !eng.transport.InterfaceOpen("eth0") || !eng.transport.InterfaceOpen("eth1") {
		t.Fatalf("metric-only change closed an interface")
	}
	if !res.changed["eth1"] || res.changed["eth0"] {
		t.Fatalf("changed journal = %+v, want only eth1", res)
	}
	if len(res.opened) != 0 || len(res.closed) != 0 {
		t.Fatalf("metric-only reconcile opened=%v closed=%v, want none", res.opened, res.closed)
	}
	if got := snapshotByName(t, eng.interfaceSnapshot(), "eth1").Cost; got != 20 {
		t.Fatalf("eth1 runtime cost = %d, want 20 after reconcile", got)
	}

	addRemove, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","cost":"10"},"eth2":{"area":"0","cost":"10"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(add/remove): %v", err)
	}
	res2 := eng.reconcile(addRemove)
	if eng.transport.InterfaceOpen("eth1") {
		t.Fatal("eth1 still open after removal")
	}
	if !eng.transport.InterfaceOpen("eth2") {
		t.Fatal("eth2 not opened after addition")
	}
	if !slices.Contains(res2.closed, "eth1") || !slices.Contains(res2.opened, "eth2") {
		t.Fatalf("reconcile journal = %+v, want close eth1 open eth2", res2)
	}
}

func TestOSPFReconcileAreaTypeRefreshesRuntime(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0","area-type":"normal"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	eng := newEngine(transport.New(&fakeBackend{}))
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()

	stub, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0","area-type":"stub"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(stub): %v", err)
	}
	res := eng.reconcile(stub)
	if !res.changed["eth0"] {
		t.Fatalf("changed journal = %+v, want eth0 refreshed for area type", res)
	}
	peer, err := types.ParseRouterID("10.0.0.2")
	if err != nil {
		t.Fatal(err)
	}
	h := packet.Hello{
		HelloInterval: DefaultHelloInterval,
		Options:       types.OptionE,
		Priority:      1,
		DeadInterval:  uint32(DefaultDeadInterval),
	}
	eng.mu.Lock()
	ifc := eng.interfaces["eth0"]
	eng.mu.Unlock()
	if got := ifc.ReceiveHello(peer, h, time.Now()); got != "options-e" {
		t.Fatalf("stub runtime ReceiveHello = %q, want options-e", got)
	}
}

func TestOSPFReconcileAddedInterfaceStartsReceiveLoop(t *testing.T) {
	empty, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(empty): %v", err)
	}
	fb := &fakeBackend{}
	eng := newEngine(transport.New(fb))
	eng.setConfig(empty)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces(empty): %v", err)
	}
	defer eng.shutdown()

	added, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(added): %v", err)
	}
	res := eng.reconcile(added)
	if !slices.Contains(res.opened, "eth0") {
		t.Fatalf("reconcile opened=%v, want eth0", res.opened)
	}

	seen := make(chan struct{})
	eng.dispatch.register(PacketTypeHello, func(transport.RawPacket, Header) {
		close(seen)
	})
	fb.mu.Lock()
	h := fb.handles["eth0"]
	fb.mu.Unlock()
	h.recv <- transport.RawPacket{IfIndex: h.ifindex, Payload: minimalOSPFPacket(t, packet.PacketTypeHello, "0")}
	select {
	case <-seen:
	case <-t.Context().Done():
		t.Fatal("added interface packet was not dispatched")
	}
}

func TestOSPFCarrierFlapRestoresRunningInterface(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	eng := newEngine(transport.New(&fakeBackend{}))
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()

	if err := eng.transport.HandleLinkDown("eth0"); err != nil {
		t.Fatalf("HandleLinkDown: %v", err)
	}
	eng.mu.Lock()
	_, stillConfigured := eng.running["eth0"]
	eng.mu.Unlock()
	if !stillConfigured {
		t.Fatal("eth0 removed from engine configured map after link down")
	}
	if got := snapshotByName(t, eng.interfaceSnapshot(), "eth0").State; got != "down" {
		t.Fatalf("eth0 state after link down = %s, want down", got)
	}

	if err := eng.transport.HandleLinkUp("eth0"); err != nil {
		t.Fatalf("HandleLinkUp: %v", err)
	}
	if got := snapshotByName(t, eng.interfaceSnapshot(), "eth0").State; got != "waiting" {
		t.Fatalf("eth0 state after link up = %s, want waiting", got)
	}
}

func TestOSPFPacketDispatch(t *testing.T) {
	d := newDispatcher(v4Codec{})
	var mu sync.Mutex
	seen := map[packet.PacketType]int{}
	for _, pt := range []packet.PacketType{packet.PacketTypeHello, packet.PacketTypeDBDesc, packet.PacketTypeLSReq, packet.PacketTypeLSUpdate, packet.PacketTypeLSAck} {
		d.register(PacketType(pt), func(transport.RawPacket, Header) {
			mu.Lock()
			seen[pt]++
			mu.Unlock()
		})
	}
	for _, pt := range []packet.PacketType{packet.PacketTypeHello, packet.PacketTypeDBDesc, packet.PacketTypeLSReq, packet.PacketTypeLSUpdate, packet.PacketTypeLSAck} {
		d.dispatch(transport.RawPacket{IfIndex: 1, Payload: minimalOSPFPacket(t, pt, "0")})
	}
	mu.Lock()
	for _, pt := range []packet.PacketType{packet.PacketTypeHello, packet.PacketTypeDBDesc, packet.PacketTypeLSReq, packet.PacketTypeLSUpdate, packet.PacketTypeLSAck} {
		if seen[pt] != 1 {
			t.Errorf("packet type %v dispatched %d times, want 1", pt, seen[pt])
		}
	}
	mu.Unlock()

	d.dispatch(transport.RawPacket{Payload: nil})
	wrongVersion := minimalOSPFPacket(t, packet.PacketTypeHello, "0")
	wrongVersion[0] = 3
	d.dispatch(transport.RawPacket{Payload: wrongVersion})
	badChecksum := minimalOSPFPacket(t, packet.PacketTypeHello, "0")
	badChecksum[4] ^= 0xff
	d.dispatch(transport.RawPacket{Payload: badChecksum})
	d.dispatch(transport.RawPacket{Payload: []byte{packet.Version, 99}})
	if got := d.dropped(); got < 4 {
		t.Fatalf("dropped = %d, want at least 4", got)
	}
}

func TestOSPFPacketDispatchAreaFilter(t *testing.T) {
	backbone, _ := types.ParseAreaID("0")
	d := newDispatcher(v4Codec{})
	d.areaOK = func(ifindex int, h Header) bool { return ifindex == 7 && h.AreaID == backbone }
	called := false
	d.register(PacketTypeHello, func(transport.RawPacket, Header) { called = true })
	d.dispatch(transport.RawPacket{IfIndex: 8, Payload: minimalOSPFPacket(t, packet.PacketTypeHello, "0")})
	d.dispatch(transport.RawPacket{IfIndex: 7, Payload: minimalOSPFPacket(t, packet.PacketTypeHello, "1")})
	if called {
		t.Fatal("handler called for area not bound to receiving interface")
	}
	if d.dropped() != 2 {
		t.Fatalf("dropped = %d, want 2", d.dropped())
	}
}

func snapshotByName(t *testing.T, rows []any, name string) ospfiface.Snapshot {
	t.Helper()
	for _, row := range rows {
		snap, ok := row.(ospfiface.Snapshot)
		if !ok {
			t.Fatalf("snapshot row has type %T, want ospfiface.Snapshot", row)
		}
		if snap.Name == name {
			return snap
		}
	}
	t.Fatalf("snapshot %q not found in %+v", name, rows)
	return ospfiface.Snapshot{}
}

func minimalOSPFPacket(t *testing.T, pt packet.PacketType, areaText string) []byte {
	t.Helper()
	return minimalOSPFInstancePacket(t, pt, areaText, 0)
}

func minimalOSPFInstancePacket(t *testing.T, pt packet.PacketType, areaText string, instanceID uint8) []byte {
	t.Helper()
	rid, err := types.ParseRouterID("10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	areaID, err := types.ParseAreaID(areaText)
	if err != nil {
		t.Fatal(err)
	}
	p := packet.Packet{Header: packet.Header{Type: pt, RouterID: rid, AreaID: areaID, InstanceID: instanceID}}
	buf := make([]byte, packet.CommonHeaderLen)
	p.WriteTo(buf, 0)
	return buf
}

// TestDispatchDropsMismatchedInstance proves AC-4 / A-3 / R-6 (RFC 6549 §2/§3.1): an
// engine configured for Instance ID 5 drops a received packet carrying a different Instance
// ID before any handler runs, and accepts one carrying its own Instance ID.
func TestDispatchDropsMismatchedInstance(t *testing.T) {
	d := newDispatcher(v4Codec{})
	d.instanceID = 5
	var handled int
	d.register(PacketTypeHello, func(transport.RawPacket, Header) { handled++ })
	mismatches := 0
	d.onInstanceMismatch = func(transport.RawPacket) { mismatches++ }

	// RFC requirement: RFC6549-2-1 positive -- a received packet whose Instance ID does not
	// match one configured for the receiving interface MUST be discarded (§2, §3.1): each of
	// {0,1,4,6,255} != the engine's Instance 5 is dropped by dispatch() before any handler runs.
	for _, id := range []uint8{0, 1, 4, 6, 255} {
		d.dispatch(transport.RawPacket{IfIndex: 1, Payload: minimalOSPFInstancePacket(t, packet.PacketTypeHello, "0", id)})
	}
	if handled != 0 {
		t.Fatalf("handler ran %d times for mismatched Instance IDs, want 0", handled)
	}
	if mismatches != 5 {
		t.Fatalf("instance-mismatch hook fired %d times, want 5", mismatches)
	}

	// RFC requirement: RFC6549-2-1 negative -- the discard is confined to mismatches: a packet
	// carrying the engine's own Instance ID (5) is NOT discarded but delivered to the handler,
	// so the demux is not a blanket drop of every packet.
	d.dispatch(transport.RawPacket{IfIndex: 1, Payload: minimalOSPFInstancePacket(t, packet.PacketTypeHello, "0", 5)})
	if handled != 1 {
		t.Fatalf("handler ran %d times for the matching Instance ID, want 1", handled)
	}
	if mismatches != 5 {
		t.Fatalf("instance-mismatch hook fired %d times after a match, want 5", mismatches)
	}
}
