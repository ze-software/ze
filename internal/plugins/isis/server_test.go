// Design: plan/spec-isis-4-component-config.md -- engine/server + PDU dispatcher tests
//
// VALIDATES: the PDU-type receive dispatcher routes each 5-bit PDU type to its
// registered handler and drops unknown types without panicking
// (TestISISPDUDispatch); the engine opens a circuit per enabled interface over a
// fake transport backend, registration is present in the plugin inventory, and
// OnConfigApply reconciles only the changed circuit
// (TestISISComponentStart / TestISISConfigApplyReconcile).
package isis

import (
	"slices"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
)

// minimalPDU builds an 8-octet common header carrying pduType so the dispatcher
// can read the 5-bit type field. It is not a full valid PDU; the dispatcher only
// reads the type octet.
func minimalPDU(pduType packet.PDUType) []byte {
	b := make([]byte, packet.CommonHeaderLen)
	b[0] = packet.ProtocolDiscriminator
	b[2] = 0x01 // version/proto-id ext
	b[3] = 0x00 // id length (default 6)
	b[4] = byte(pduType)
	b[5] = 0x01 // version
	return b
}

func TestISISPDUDispatch(t *testing.T) {
	d := newDispatcher()

	var mu sync.Mutex
	seen := map[packet.PDUType]int{}
	record := func(pt packet.PDUType) pduHandler {
		return func(transport.RawFrame) {
			mu.Lock()
			seen[pt]++
			mu.Unlock()
		}
	}

	all := []packet.PDUType{
		packet.PDUTypeL1LANHello, packet.PDUTypeL2LANHello, packet.PDUTypeP2PHello,
		packet.PDUTypeL1LSP, packet.PDUTypeL2LSP,
		packet.PDUTypeL1CSNP, packet.PDUTypeL2CSNP,
		packet.PDUTypeL1PSNP, packet.PDUTypeL2PSNP,
	}
	for _, pt := range all {
		d.register(pt, record(pt))
	}

	for _, pt := range all {
		d.dispatch(transport.RawFrame{IfIndex: 7, PDU: minimalPDU(pt)})
	}

	mu.Lock()
	defer mu.Unlock()
	for _, pt := range all {
		if seen[pt] != 1 {
			t.Errorf("PDU type %v dispatched %d times, want 1", pt, seen[pt])
		}
	}

	// An unknown 5-bit type (0x01) must be dropped, not panic, and not reach any
	// handler.
	d.dispatch(transport.RawFrame{IfIndex: 7, PDU: minimalPDU(packet.PDUType(0x01))})
	// A short PDU (no type octet) must be dropped, not panic.
	d.dispatch(transport.RawFrame{IfIndex: 7, PDU: []byte{0x83}})
	d.dispatch(transport.RawFrame{IfIndex: 7, PDU: nil})

	if got := d.dropped(); got < 3 {
		t.Errorf("dropped = %d, want >= 3 (unknown + 2 short)", got)
	}
}

// fakeBackend is a transport.Backend that hands out fake circuits without a real
// socket, so the engine's circuit-open path can be exercised on darwin.
type fakeBackend struct {
	mu     sync.Mutex
	opened []string
}

func (b *fakeBackend) OpenCircuit(name string) (transport.CircuitHandle, error) {
	b.mu.Lock()
	b.opened = append(b.opened, name)
	b.mu.Unlock()
	return &fakeCircuit{name: name, recv: make(chan transport.RawFrame)}, nil
}

type fakeCircuit struct {
	name string
	recv chan transport.RawFrame
	once sync.Once
}

func (c *fakeCircuit) IfIndex() int                   { return 1 }
func (c *fakeCircuit) HWAddr() [transport.MACLen]byte { return [transport.MACLen]byte{} }
func (c *fakeCircuit) MTU() int                       { return 1500 }
func (c *fakeCircuit) Send(_, _ [transport.MACLen]byte, _ []byte) error {
	return nil
}
func (c *fakeCircuit) Recv() <-chan transport.RawFrame { return c.recv }
func (c *fakeCircuit) Close() error {
	c.once.Do(func() { close(c.recv) })
	return nil
}

func TestISISComponentStart(t *testing.T) {
	// AC-7: the component is registered in the plugin inventory.
	if !registry.Has("isis") {
		t.Fatal("isis is not in the plugin registry (make generate must wire all.go)")
	}
	r := registry.Lookup("isis")
	if r == nil {
		t.Fatal("registry.Lookup(isis) returned nil")
	}
	if r.RunEngine == nil {
		t.Error("isis registration has no RunEngine")
	}
	if len(r.ConfigRoots) == 0 || r.ConfigRoots[0] != "isis" {
		t.Errorf("ConfigRoots = %v, want [isis]", r.ConfigRoots)
	}
	if r.YANG == "" {
		t.Error("isis registration has no YANG")
	}

	// AC-1: the engine opens a circuit per enabled interface over a fake backend.
	cfg, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{},"eth1":{}}}}}`))
	if err != nil {
		t.Fatalf("parseISISConfig: %v", err)
	}
	fb := &fakeBackend{}
	eng := newEngine(transport.New(fb))
	eng.setConfig(cfg)
	if err := eng.openCircuits(); err != nil {
		t.Fatalf("openCircuits: %v", err)
	}
	defer eng.shutdown()

	if !eng.transport.CircuitOpen("eth0") || !eng.transport.CircuitOpen("eth1") {
		t.Errorf("expected circuits open for eth0 and eth1, open count = %d", eng.transport.OpenCircuitCount())
	}
}

func TestISISConfigApplyReconcile(t *testing.T) {
	cfg, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"metric":"10"},"eth1":{"metric":"10"}}}}}`))
	if err != nil {
		t.Fatalf("parseISISConfig: %v", err)
	}
	fb := &fakeBackend{}
	eng := newEngine(transport.New(fb))
	eng.setConfig(cfg)
	if err := eng.openCircuits(); err != nil {
		t.Fatalf("openCircuits: %v", err)
	}
	defer eng.shutdown()

	// Reload changes only eth1's metric. eth0's circuit must NOT be torn down
	// (AC-8: reconcile, not restart-all).
	newCfg, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"metric":"10"},"eth1":{"metric":"20"}}}}}`))
	if err != nil {
		t.Fatalf("parseISISConfig(reload): %v", err)
	}
	res := eng.reconcile(newCfg)

	if !eng.transport.CircuitOpen("eth0") {
		t.Error("eth0 circuit was torn down on a metric-only change to eth1")
	}
	if !eng.transport.CircuitOpen("eth1") {
		t.Error("eth1 circuit missing after reconcile")
	}
	// The journal reports eth1 changed and eth0 unchanged.
	if res.changed["eth1"] != true {
		t.Errorf("reconcile result did not mark eth1 changed: %+v", res)
	}
	if res.changed["eth0"] == true {
		t.Errorf("reconcile result wrongly marked eth0 changed: %+v", res)
	}
	if len(res.opened) != 0 || len(res.closed) != 0 {
		t.Errorf("metric-only change should open/close no circuits, got opened=%v closed=%v", res.opened, res.closed)
	}

	// Removing eth1 closes its circuit; adding eth2 opens one.
	addRemove, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"metric":"10"},"eth2":{"metric":"10"}}}}}`))
	if err != nil {
		t.Fatalf("parseISISConfig(add/remove): %v", err)
	}
	res2 := eng.reconcile(addRemove)
	if eng.transport.CircuitOpen("eth1") {
		t.Error("eth1 circuit still open after removal")
	}
	if !eng.transport.CircuitOpen("eth2") {
		t.Error("eth2 circuit not opened after addition")
	}
	if !slices.Contains(res2.closed, "eth1") {
		t.Errorf("reconcile result missing eth1 in closed: %v", res2.closed)
	}
	if !slices.Contains(res2.opened, "eth2") {
		t.Errorf("reconcile result missing eth2 in opened: %v", res2.opened)
	}
}
