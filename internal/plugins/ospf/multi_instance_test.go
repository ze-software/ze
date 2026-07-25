// VALIDATES: spec-ospf-ext-12 -- the OSPFv2 Multi-Instance (RFC 6549) engine-set manager:
// one full engine per configured Instance ID, isolated databases, shared-transport fan-out
// demux, and teardown of a removed instance.
package ospf

import (
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
)

// newTestInstanceManager builds a manager whose non-base engines run over fresh fake
// backends, mirroring register.go's production builder but without real sockets.
func newTestInstanceManager() *instanceManager {
	base := newEngine(transport.New(&fakeBackend{}))
	return newInstanceManager(base, func(uint8) *engine {
		return newEngine(transport.New(&fakeBackend{}))
	}, nil)
}

// TestConfigSpawnsInstanceEngine proves AC-6 / A-4: a per-interface `instance-id 5` yields
// exactly one extra OSPFv2 engine bound to Instance ID 5 (plus the base 0), each with its
// own dispatcher/LSDB/neighbor table, and `show ospf instance` lists both.
func TestConfigSpawnsInstanceEngine(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","instance-id":"5"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	mgr := newTestInstanceManager()
	mgr.setConfig(cfg)
	defer mgr.shutdownAll()

	if _, ok := mgr.engineFor(0); !ok {
		t.Fatal("base Instance 0 engine missing")
	}
	eng5, ok := mgr.engineFor(5)
	if !ok {
		t.Fatal("no engine spawned for Instance ID 5")
	}
	if eng5.dispatch.instanceID != 5 {
		t.Fatalf("Instance 5 engine dispatch.instanceID = %d, want 5", eng5.dispatch.instanceID)
	}
	// eth0 belongs to instance 5, not instance 0, so the base engine has no interfaces.
	eng0, _ := mgr.engineFor(0)
	if len(eng0.cfg.Interfaces) != 0 {
		t.Fatalf("base engine got %d interfaces, want 0 (eth0 is in instance 5)", len(eng0.cfg.Interfaces))
	}
	if len(eng5.cfg.Interfaces) != 1 || eng5.cfg.Interfaces[0].Name != "eth0" {
		t.Fatalf("instance 5 engine interfaces = %+v, want [eth0]", eng5.cfg.Interfaces)
	}

	rows := mgr.instanceSnapshot()
	if len(rows) != 2 {
		t.Fatalf("show ospf instance rows = %d, want 2 (instances 0 and 5)", len(rows))
	}
	got := map[uint8]instanceSummaryView{}
	for _, r := range rows {
		v, ok := r.(instanceSummaryView)
		if !ok {
			t.Fatalf("instance snapshot row type %T, want instanceSummaryView", r)
		}
		got[v.InstanceID] = v
	}
	if got[5].InterfaceCount != 1 {
		t.Fatalf("instance 5 interface-count = %d, want 1", got[5].InterfaceCount)
	}
	if got[0].InterfaceCount != 0 {
		t.Fatalf("instance 0 interface-count = %d, want 0", got[0].InterfaceCount)
	}
}

// TestTwoInstancesIsolatedLSDB proves AC-7 / A-4: two OSPFv2 instances (0 and 5) enrolled
// on the same physical interface run as fully separate engines -- distinct dispatchers,
// LSDBs, and neighbor tables -- so no LSA or neighbor state can cross instances.
func TestTwoInstancesIsolatedLSDB(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","instance-id":["0","5"]}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	mgr := newTestInstanceManager()
	mgr.setConfig(cfg)
	defer mgr.shutdownAll()

	eng0, ok0 := mgr.engineFor(0)
	eng5, ok5 := mgr.engineFor(5)
	if !ok0 || !ok5 {
		t.Fatalf("expected engines for instances 0 and 5, got 0=%v 5=%v", ok0, ok5)
	}
	// Both instances are enrolled on eth0 (the shared subnet), by design.
	if len(eng0.cfg.Interfaces) != 1 || len(eng5.cfg.Interfaces) != 1 {
		t.Fatalf("both instances must carry eth0: 0=%d 5=%d interfaces", len(eng0.cfg.Interfaces), len(eng5.cfg.Interfaces))
	}
	if eng0.dispatch.instanceID != 0 || eng5.dispatch.instanceID != 5 {
		t.Fatalf("dispatch instance IDs = %d/%d, want 0/5", eng0.dispatch.instanceID, eng5.dispatch.instanceID)
	}
	// State containers must be distinct objects (no shared LSDB/neighbor/dispatcher).
	if eng0 == eng5 {
		t.Fatal("both instances share one engine")
	}
	if eng0.lsdb == eng5.lsdb {
		t.Fatal("instances share one LSDB (isolation broken)")
	}
	if eng0.neighbors == eng5.neighbors {
		t.Fatal("instances share one neighbor table (isolation broken)")
	}
	if eng0.dispatch == eng5.dispatch {
		t.Fatal("instances share one dispatcher (isolation broken)")
	}
	if eng0.transport == eng5.transport {
		t.Fatal("instances share one transport (isolation broken)")
	}
}

// TestSharedTransportFanOut proves A-7 / R-4 (run under -race): the same received datagram
// fanned to every OSPFv2 instance is processed only by the engine whose Instance ID
// matches, and dropped by the others, with no data race across the concurrent demux.
func TestSharedTransportFanOut(t *testing.T) {
	const deliveries = 200
	// Two independent per-instance dispatchers, modeling the two engines each raw socket
	// (or a fan-out multiplexer) delivers every multicast datagram to.
	var mu sync.Mutex
	handled := map[uint8]int{}
	makeDispatch := func(id uint8) *dispatcher {
		d := newDispatcher(v4Codec{})
		d.instanceID = id
		d.register(PacketTypeHello, func(transport.RawPacket, Header) {
			mu.Lock()
			handled[id]++
			mu.Unlock()
		})
		return d
	}
	d0 := makeDispatch(0)
	d5 := makeDispatch(5)

	// Every datagram carries Instance ID 5.
	payload := minimalOSPFInstancePacket(t, packet.PacketTypeHello, "0", 5)

	var wg sync.WaitGroup
	for range deliveries {
		wg.Add(2)
		go func() { defer wg.Done(); d0.dispatch(transport.RawPacket{IfIndex: 1, Payload: payload}) }()
		go func() { defer wg.Done(); d5.dispatch(transport.RawPacket{IfIndex: 1, Payload: payload}) }()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if handled[0] != 0 {
		t.Fatalf("Instance 0 engine processed %d Instance-5 datagrams, want 0 (all dropped)", handled[0])
	}
	if handled[5] != deliveries {
		t.Fatalf("Instance 5 engine processed %d datagrams, want %d", handled[5], deliveries)
	}
	if d0.dropped() != deliveries {
		t.Fatalf("Instance 0 dispatcher dropped %d, want %d", d0.dropped(), deliveries)
	}
}

// TestInstanceRemovedTearsDown proves AC-8 / R-5: removing an Instance ID from the running
// config and reconciling tears down that instance's engine (shutdown cancels its context)
// and leaves the remaining instances untouched.
func TestInstanceRemovedTearsDown(t *testing.T) {
	withInstance, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"},"eth1":{"area":"0","instance-id":"5"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(with): %v", err)
	}
	withoutInstance, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(without): %v", err)
	}

	mgr := newTestInstanceManager()
	defer mgr.shutdownAll()
	mgr.reconcile(withInstance)

	eng5, ok := mgr.engineFor(5)
	if !ok {
		t.Fatal("instance 5 engine not created")
	}
	if eng5.ctx.Err() != nil {
		t.Fatal("instance 5 engine already shut down before removal")
	}

	// Remove instance 5 from config and reconcile: its engine must be torn down.
	mgr.reconcile(withoutInstance)
	if _, ok := mgr.engineFor(5); ok {
		t.Fatal("instance 5 engine still present after its Instance ID was removed")
	}
	if eng5.ctx.Err() == nil {
		t.Fatal("removed instance 5 engine was not shut down (context still live)")
	}
	// The base instance is unaffected.
	if _, ok := mgr.engineFor(0); !ok {
		t.Fatal("base instance 0 engine removed by unrelated reconcile")
	}
}
